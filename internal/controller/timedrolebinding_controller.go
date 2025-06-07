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
	"time"

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
	// Start time is in the future. Requeue after the time difference.
	if trb.Spec.StartTime.Time.After(now) {
		timeDiff := trb.Spec.StartTime.Time.Sub(now)

		// Update the status phase to Pending
		trb.Status.Phase = rbacv1alpha1.TimedRoleBindingPhasePending
		trb.Status.LastTransitionTime = metav1.Now()
		if err := r.Status().Update(ctx, trb); err != nil {
			log.Error(err, "Failed to update TimedRoleBinding status")
			return ctrl.Result{}, err // TODO: handle error
		}

		// Make sure the associated RoleBinding is deleted
		if err := r.deleteRoleBinding(ctx, trb); err != nil {
			log.Error(err, "Failed to delete RoleBinding")
			return ctrl.Result{}, err
		}

		// Requeue after the time difference
		return ctrl.Result{RequeueAfter: timeDiff}, nil

	}

	// Start time is in the past/now. Create the role binding.
	if trb.Spec.StartTime.Time.Before(now) || trb.Spec.StartTime.Time.Equal(now) {

		// If the end time is in the past/now, the role binding is expired.
		if trb.Spec.EndTime.Time.Before(now) || trb.Spec.EndTime.Time.Equal(now) {
			trb.Status.Phase = rbacv1alpha1.TimedRoleBindingPhaseExpired
			trb.Status.LastTransitionTime = metav1.Now()
			if err := r.Status().Update(ctx, trb); err != nil {
				log.Error(err, "Failed to update TimedRoleBinding status")
				return ctrl.Result{}, err // TODO: handle error
			}

			// Make sure the associated RoleBinding is deleted
			if err := r.deleteRoleBinding(ctx, trb); err != nil {
				log.Error(err, "Failed to delete RoleBinding")
				return ctrl.Result{}, err
			}

			keepExpiry := trb.Spec.EndTime.Time.Add(trb.Spec.KeepExpiredFor.Duration)
			// Keep the CRD if its KeepExpiredFor duration has not passed.
			if now.Before(keepExpiry) {
				// Requeue after the KeepExpiredFor duration.
				diff := keepExpiry.Sub(now)
				return ctrl.Result{RequeueAfter: diff}, nil
			}

			// Remove the CRD.
			if err := r.Delete(ctx, trb); client.IgnoreNotFound(err) != nil {
				log.Error(err, "Failed to delete TimedRoleBinding")
				return ctrl.Result{}, err // TODO: handle error
			}

			return ctrl.Result{}, nil
		}

		// If the end time is in the future, the role binding is active.
		trb.Status.Phase = rbacv1alpha1.TimedRoleBindingPhaseActive
		trb.Status.LastTransitionTime = metav1.Now()
		if err := r.Status().Update(ctx, trb); err != nil {
			log.Error(err, "Failed to update TimedRoleBinding status")
			return ctrl.Result{}, err // TODO: handle error
		}

		if err := r.createRoleBinding(ctx, trb); err != nil {
			log.Error(err, "Failed to create RoleBinding")
			return ctrl.Result{}, err // TODO: handle error
		}

		// Requeue after the end time.
		return ctrl.Result{RequeueAfter: trb.Spec.EndTime.Time.Sub(now)}, nil
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

// SetupWithManager sets up the controller with the Manager.
func (r *TimedRoleBindingReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&rbacv1alpha1.TimedRoleBinding{}).
		Owns(&rbacv1.RoleBinding{}).
		Named("timedrolebinding").
		Complete(r)
}
