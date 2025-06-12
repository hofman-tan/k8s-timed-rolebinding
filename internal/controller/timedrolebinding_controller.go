/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	rbacv1alpha1 "github.com/hofman-tan/k8s-timed-rolebinding/api/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
)

// TimedRoleBindingReconciler reconciles a TimedRoleBinding object
type TimedRoleBindingReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=rbac.hhh.github.io,resources=timedrolebindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.hhh.github.io,resources=timedrolebindings/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=rbac.hhh.github.io,resources=timedrolebindings/finalizers,verbs=update
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles,verbs=bind
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles,verbs=bind
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the TimedRoleBinding object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.20.4/pkg/reconcile
func (r *TimedRoleBindingReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	trb := &rbacv1alpha1.TimedRoleBinding{}
	if err := r.Get(ctx, req.NamespacedName, trb); err != nil {
		if errors.IsNotFound(err) {
			// TimedRoleBinding not found. No need to requeue.
			log.Info("TimedRoleBinding not found. Ignoring since it must have been deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get TimedRoleBinding")
		return ctrl.Result{}, err
	}

	now := time.Now()
	startTime := trb.Spec.StartTime.Time
	endTime := trb.Spec.EndTime.Time

	// Start time is in the future. Requeue after the time difference.
	if startTime.After(now) {
		timeDiff := startTime.Sub(now)

		setStatus(
			trb,
			rbacv1alpha1.TimedRoleBindingPhasePending,
			"TimedRoleBinding is queued for activation",
		)

		// Make sure the associated RoleBinding is deleted
		if err := r.deleteRoleBinding(ctx, trb); err != nil {
			log.Error(err, "Failed to delete RoleBinding")
			setStatus(
				trb,
				rbacv1alpha1.TimedRoleBindingPhaseFailed,
				fmt.Sprintf("Failed to delete RoleBinding: %v", err),
			)
		}

		if err := r.Status().Update(ctx, trb); err != nil {
			log.Error(err, "Failed to update TimedRoleBinding status")
			return ctrl.Result{}, err // TODO: handle error
		}

		log.Info("TimedRoleBinding is queued for activation", "name", trb.Name, "namespace", trb.Namespace)

		// Requeue after the time difference
		return ctrl.Result{RequeueAfter: timeDiff}, nil
	}

	// Start time is in the past/now. Create the role binding.
	if !startTime.After(now) {

		// If the end time is in the past/now, the role binding has expired.
		if !endTime.After(now) {
			setStatus(
				trb,
				rbacv1alpha1.TimedRoleBindingPhaseExpired,
				"TimedRoleBinding has expired",
			)

			// Make sure the associated RoleBinding is deleted
			if err := r.deleteRoleBinding(ctx, trb); err != nil {
				log.Error(err, "Failed to delete RoleBinding")
				setStatus(
					trb,
					rbacv1alpha1.TimedRoleBindingPhaseFailed,
					fmt.Sprintf("Failed to delete RoleBinding: %v", err),
				)
			}

			if err := r.Status().Update(ctx, trb); err != nil {
				log.Error(err, "Failed to update TimedRoleBinding status")
				return ctrl.Result{}, err // TODO: handle error
			}

			if trb.Spec.KeepExpiredFor != nil {
				keepDeadline := endTime.Add(trb.Spec.KeepExpiredFor.Duration)
				// Keep the CRD if it's within the keepDeadline.
				if now.Before(keepDeadline) {
					// Create the postExpire job (if specified)
					if isPostExpireJobEnabled(trb) {
						log.Info("Creating postExpire job")
						if err := r.createHookJob(
							ctx,
							trb,
							trb.GetName()+"-post-expire",
							*trb.Spec.PostExpire.JobTemplate.Spec.DeepCopy(),
						); err != nil {
							log.Error(err, "Failed to create postExpire job")
							setStatus(
								trb,
								rbacv1alpha1.TimedRoleBindingPhaseFailed,
								fmt.Sprintf("Failed to create postExpire job: %v", err),
							)
						}
					}

					// Requeue for deletion later.
					diff := keepDeadline.Sub(now)
					return ctrl.Result{RequeueAfter: diff}, nil
				}
			}

			// Remove the CRD.
			if err := r.Delete(ctx, trb); client.IgnoreNotFound(err) != nil {
				log.Error(err, "Failed to delete TimedRoleBinding")
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}

		// If the end time is in the future, the role binding is active.
		setStatus(
			trb,
			rbacv1alpha1.TimedRoleBindingPhaseActive,
			"TimedRoleBinding is active",
		)

		if err := r.createRoleBinding(ctx, trb); err != nil {
			log.Error(err, "Failed to create RoleBinding")
			setStatus(
				trb,
				rbacv1alpha1.TimedRoleBindingPhaseFailed,
				fmt.Sprintf("Failed to create RoleBinding: %v", err),
			)
		}

		// Create the postActivate job (if specified)
		if isPostActivateJobEnabled(trb) {
			log.Info("Creating postActivate job")
			if err := r.createHookJob(
				ctx,
				trb,
				trb.GetName()+"-post-activate",
				*trb.Spec.PostActivate.JobTemplate.Spec.DeepCopy(),
			); err != nil {
				log.Error(err, "Failed to create postActivate job")
				setStatus(
					trb,
					rbacv1alpha1.TimedRoleBindingPhaseFailed,
					fmt.Sprintf("Failed to create postActivate job: %v", err),
				)
			}
		}

		if err := r.Status().Update(ctx, trb); err != nil {
			log.Error(err, "Failed to update TimedRoleBinding status")
			return ctrl.Result{}, err // TODO: handle error
		}

		// Requeue after the end time.
		return ctrl.Result{RequeueAfter: endTime.Sub(now)}, nil
	}

	return ctrl.Result{}, nil
}

func (r *TimedRoleBindingReconciler) createRoleBinding(ctx context.Context, trb *rbacv1alpha1.TimedRoleBinding) error {
	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      trb.Name,
			Namespace: trb.Namespace,
		},
		Subjects: trb.Spec.Subjects,
		RoleRef:  trb.Spec.RoleRef,
	}

	// Sets the owner reference to the TimedRoleBinding.
	// This ensures that the RoleBinding will be deleted when the TimedRoleBinding is deleted.
	controllerutil.SetControllerReference(trb, roleBinding, r.Scheme)

	if err := r.Create(ctx, roleBinding); client.IgnoreAlreadyExists(err) != nil {
		return err
	}
	return nil
}

func (r *TimedRoleBindingReconciler) deleteRoleBinding(ctx context.Context, trb *rbacv1alpha1.TimedRoleBinding) error {
	if err := r.Delete(ctx, &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      trb.Name,
			Namespace: trb.Namespace,
		},
	}); client.IgnoreNotFound(err) != nil {
		return err
	}
	return nil
}

func setStatus(trb *rbacv1alpha1.TimedRoleBinding, phase rbacv1alpha1.TimedRoleBindingPhase, msg string) {
	trb.Status.Phase = phase
	trb.Status.Message = msg
	trb.Status.LastTransitionTime = metav1.Now()
}

func isPostActivateJobEnabled(trb *rbacv1alpha1.TimedRoleBinding) bool {
	return trb.Spec.PostActivate != nil &&
		trb.Spec.PostActivate.JobTemplate != nil
}

func isPostExpireJobEnabled(trb *rbacv1alpha1.TimedRoleBinding) bool {
	return trb.Spec.PostExpire != nil &&
		trb.Spec.PostExpire.JobTemplate != nil
}

func (r *TimedRoleBindingReconciler) createHookJob(ctx context.Context, trb *rbacv1alpha1.TimedRoleBinding, name string, spec batchv1.JobSpec) error {
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: trb.Namespace, // Same namespace as the TimedRoleBinding resource
		},
		Spec: spec,
	}

	// Inject the TimedRoleBinding name as env variable into the job's containers.
	for i := range job.Spec.Template.Spec.Containers {
		job.Spec.Template.Spec.Containers[i].Env = append(job.Spec.Template.Spec.Containers[i].Env, corev1.EnvVar{
			Name:  "TIMED_ROLE_BINDING_NAME",
			Value: trb.Name,
		})
	}

	// Sets the owner reference to the TimedRoleBinding.
	// This ensures that the Job will be deleted when the TimedRoleBinding is deleted.
	controllerutil.SetControllerReference(trb, job, r.Scheme)

	if err := r.Create(ctx, job); client.IgnoreAlreadyExists(err) != nil {
		return err
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *TimedRoleBindingReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&rbacv1alpha1.TimedRoleBinding{}).
		Owns(&rbacv1.RoleBinding{}).
		Named("timedrolebinding").
		Complete(r)
}
