package controller

import (
	"context"
	"fmt"
	"time"

	rbacv1alpha1 "github.com/hofman-tan/k8s-timed-rolebinding/api/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

type TimedRoleBindingObject interface {
	client.Object
	GetSpec() *rbacv1alpha1.TimedRoleBindingSpec
	GetStatus() *rbacv1alpha1.TimedRoleBindingStatus
	BuildObjectForRoleBinding() client.Object
	BuildJobObject(name string, jobTemplateSpec *batchv1.JobTemplateSpec) batchv1.Job
}

func setStatus(
	trb TimedRoleBindingObject,
	phase rbacv1alpha1.TimedRoleBindingPhase,
	msg string,
) {
	trb.GetStatus().Phase = phase
	trb.GetStatus().Message = msg
	trb.GetStatus().LastTransitionTime = metav1.Now()
}

type Reconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func (r *Reconciler) reconcileObject(
	ctx context.Context,
	req ctrl.Request,
	trb TimedRoleBindingObject,
) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	if err := r.Get(ctx, req.NamespacedName, trb); err != nil {
		if errors.IsNotFound(err) {
			// TimedRoleBinding not found. No need to requeue.
			log.Info(fmt.Sprintf("%s not found. Ignoring since it must have been deleted", trb.GetName()))
			return ctrl.Result{}, nil
		}
		log.Error(err, fmt.Sprintf("Failed to get %s", trb.GetName()))
		return ctrl.Result{}, err
	}

	log.Info(fmt.Sprintf("Reconciling %s", trb.GetName()), "namespace", trb.GetNamespace())

	spec := trb.GetSpec()
	now := time.Now()
	startTime := spec.StartTime.Time
	endTime := spec.EndTime.Time

	// Pending activation
	if startTime.After(now) {
		setStatus(
			trb,
			rbacv1alpha1.TimedRoleBindingPhasePending,
			fmt.Sprintf("%s is queued for activation", trb.GetName()),
		)

		// Make sure the associated RoleBinding is deleted
		if err := r.Delete(ctx, trb.BuildObjectForRoleBinding()); client.IgnoreNotFound(err) != nil {
			log.Error(err, "Failed to delete RoleBinding")
			setStatus(
				trb,
				rbacv1alpha1.TimedRoleBindingPhaseFailed,
				fmt.Sprintf("Failed to delete RoleBinding: %v", err),
			)
		}

		if err := r.Status().Update(ctx, trb); err != nil {
			log.Error(err, fmt.Sprintf("Failed to update %s status", trb.GetName()))
			return ctrl.Result{}, err
		}

		log.Info(
			fmt.Sprintf("%s is queued for activation", trb.GetName()),
			"name", trb.GetName(),
			"namespace", trb.GetNamespace(),
		)

		// Requeue for activation
		return ctrl.Result{RequeueAfter: startTime.Sub(now)}, nil
	}

	// Active
	if !startTime.After(now) && endTime.After(now) {
		setStatus(
			trb,
			rbacv1alpha1.TimedRoleBindingPhaseActive,
			fmt.Sprintf("%s is activated", trb.GetName()),
		)

		// Create the RoleBinding
		rb := trb.BuildObjectForRoleBinding()
		if err := controllerutil.SetControllerReference(trb, rb, r.Scheme); err != nil {
			log.Error(err, "Failed to set controller reference")
			setStatus(
				trb,
				rbacv1alpha1.TimedRoleBindingPhaseFailed,
				fmt.Sprintf("Failed to set controller reference: %v", err),
			)
		}
		if err := r.Create(ctx, rb); client.IgnoreAlreadyExists(err) != nil {
			log.Error(err, fmt.Sprintf("Failed to create %s", rb.GetName()))
			setStatus(
				trb,
				rbacv1alpha1.TimedRoleBindingPhaseFailed,
				fmt.Sprintf("Failed to create %s: %v", rb.GetName(), err),
			)
		}

		if err := r.Status().Update(ctx, trb); err != nil {
			log.Error(err, fmt.Sprintf("Failed to update %s status", trb.GetName()))
			return ctrl.Result{}, err
		}

		// Create the postActivate job (if configured)
		if isPostActivateJobEnabled(spec) {
			log.Info("Creating postActivate job")
			// TODO: user-specified name for the job
			job := trb.BuildJobObject(trb.GetName()+"-post-activate", spec.PostActivate.JobTemplate.DeepCopy())
			if err := controllerutil.SetControllerReference(trb, &job, r.Scheme); err != nil {
				log.Error(err, "Failed to set controller reference")
				return ctrl.Result{}, err
			}
			if err := r.Create(ctx, &job); client.IgnoreAlreadyExists(err) != nil {
				log.Error(err, fmt.Sprintf("Failed to create %s", job.GetName()))
				return ctrl.Result{}, err
			}
		}

		// Requeue for expiration
		return ctrl.Result{RequeueAfter: endTime.Sub(now)}, nil
	}

	// Expired
	setStatus(
		trb,
		rbacv1alpha1.TimedRoleBindingPhaseExpired,
		fmt.Sprintf("%s has expired", trb.GetName()),
	)

	// Make sure the role binding is deleted
	if err := r.Delete(ctx, trb.BuildObjectForRoleBinding()); client.IgnoreNotFound(err) != nil {
		log.Error(err, "Failed to delete RoleBinding")
		setStatus(
			trb,
			rbacv1alpha1.TimedRoleBindingPhaseFailed,
			fmt.Sprintf("Failed to delete RoleBinding: %v", err),
		)
	}

	if err := r.Status().Update(ctx, trb); err != nil {
		log.Error(err, fmt.Sprintf("Failed to update %s status", trb.GetName()))
		return ctrl.Result{}, err
	}

	// Create the postExpire job (if configured)
	if isPostExpireJobEnabled(spec) {
		log.Info("Creating postExpire job")
		job := trb.BuildJobObject(trb.GetName()+"-post-expire", spec.PostExpire.JobTemplate.DeepCopy())
		if err := controllerutil.SetControllerReference(trb, &job, r.Scheme); err != nil {
			log.Error(err, "Failed to set controller reference")
			return ctrl.Result{}, err
		}
		if err := r.Create(ctx, &job); client.IgnoreAlreadyExists(err) != nil {
			log.Error(err, fmt.Sprintf("Failed to create %s", job.GetName()))
			return ctrl.Result{}, err
		}
	}

	if spec.KeepExpiredFor != nil {
		keepDeadline := endTime.Add(spec.KeepExpiredFor.Duration)
		if now.Before(keepDeadline) {
			// Requeue for deletion later.
			diff := keepDeadline.Sub(now)
			return ctrl.Result{RequeueAfter: diff}, nil
		}
	}

	// Remove the CR
	if err := r.Delete(ctx, trb); err != nil {
		log.Error(err, fmt.Sprintf("Failed to delete %s", trb.GetName()))
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func isPostActivateJobEnabled(spec *rbacv1alpha1.TimedRoleBindingSpec) bool {
	return spec.PostActivate != nil &&
		spec.PostActivate.JobTemplate != nil
}

func isPostExpireJobEnabled(spec *rbacv1alpha1.TimedRoleBindingSpec) bool {
	return spec.PostExpire != nil &&
		spec.PostExpire.JobTemplate != nil
}
