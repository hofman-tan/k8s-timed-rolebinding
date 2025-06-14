package controller

import (
	"context"
	"fmt"
	"time"

	rbacv1alpha1 "github.com/hofman-tan/k8s-timed-rolebinding/api/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// Reconciler is an interface for the reconciler that handles both TimedRoleBinding and TimedClusterRoleBinding
type Reconciler[T rbacv1alpha1.TimedRoleBinding | rbacv1alpha1.TimedClusterRoleBinding] interface {
	GetObject(ctx context.Context, req ctrl.Request) (*T, error)
	CreateRoleBinding(ctx context.Context, trb *T) error
	DeleteRoleBinding(ctx context.Context, trb *T) error
	CreateHookJob(ctx context.Context, trb *T, name string, templateSpec rbacv1alpha1.JobTemplateSpec) error
	GetObjectName(trb *T) string
	GetObjectNamespace(trb *T) string
	GetObjectSpec(trb *T) rbacv1alpha1.TimedRoleBindingSpec
	GetObjectStatus(trb *T) *rbacv1alpha1.TimedRoleBindingStatus
	SetObjectStatus(trb *T, phase rbacv1alpha1.TimedRoleBindingPhase, msg string)
	UpdateObjectStatus(ctx context.Context, trb *T) error
}

func reconciles[T rbacv1alpha1.TimedRoleBinding | rbacv1alpha1.TimedClusterRoleBinding](
	ctx context.Context,
	req ctrl.Request,
	r Reconciler[T],
) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	trb, err := r.GetObject(ctx, req)
	if err != nil {
		if errors.IsNotFound(err) {
			// TimedRoleBinding not found. No need to requeue.
			log.Info("TimedRoleBinding not found. Ignoring since it must have been deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get TimedRoleBinding")
		return ctrl.Result{}, err
	}

	spec := r.GetObjectSpec(trb)
	now := time.Now()
	startTime := spec.StartTime.Time
	endTime := spec.EndTime.Time

	// Start time is in the future. Requeue after the time difference.
	if startTime.After(now) {
		timeDiff := startTime.Sub(now)

		r.SetObjectStatus(
			trb,
			rbacv1alpha1.TimedRoleBindingPhasePending,
			"TimedRoleBinding is queued for activation",
		)

		// Make sure the associated RoleBinding is deleted
		if err := r.DeleteRoleBinding(ctx, trb); err != nil {
			log.Error(err, "Failed to delete RoleBinding")
			r.SetObjectStatus(
				trb,
				rbacv1alpha1.TimedRoleBindingPhaseFailed,
				fmt.Sprintf("Failed to delete RoleBinding: %v", err),
			)
		}

		if err := r.UpdateObjectStatus(ctx, trb); err != nil {
			log.Error(err, "Failed to update TimedRoleBinding status")
			return ctrl.Result{}, err
		}

		log.Info("TimedRoleBinding is queued for activation", "name", r.GetObjectName(trb), "namespace", r.GetObjectNamespace(trb))

		// Requeue after the time difference
		return ctrl.Result{RequeueAfter: timeDiff}, nil
	}

	// Start time is in the past/now. Create the role binding.
	if !startTime.After(now) {

		// If the end time is in the past/now, the role binding has expired.
		if !endTime.After(now) {
			r.SetObjectStatus(
				trb,
				rbacv1alpha1.TimedRoleBindingPhaseExpired,
				"TimedRoleBinding has expired",
			)

			// Make sure the associated RoleBinding is deleted
			if err := r.DeleteRoleBinding(ctx, trb); err != nil {
				log.Error(err, "Failed to delete RoleBinding")
				r.SetObjectStatus(
					trb,
					rbacv1alpha1.TimedRoleBindingPhaseFailed,
					fmt.Sprintf("Failed to delete RoleBinding: %v", err),
				)
			}

			if err := r.UpdateObjectStatus(ctx, trb); err != nil {
				log.Error(err, "Failed to update TimedRoleBinding status")
				return ctrl.Result{}, err
			}

			if spec.KeepExpiredFor != nil {
				keepDeadline := endTime.Add(spec.KeepExpiredFor.Duration)
				// Keep the CRD if it's within the keepDeadline.
				if now.Before(keepDeadline) {
					// Create the postExpire job (if specified)
					if isPostExpireJobEnabled(&spec) {
						log.Info("Creating postExpire job")
						if err := r.CreateHookJob(
							ctx,
							trb,
							r.GetObjectName(trb)+"-post-expire",
							*spec.PostExpire.JobTemplate.DeepCopy(),
						); err != nil {
							log.Error(err, "Failed to create postExpire job")
							r.SetObjectStatus(
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
			if err := r.DeleteRoleBinding(ctx, trb); client.IgnoreNotFound(err) != nil {
				log.Error(err, "Failed to delete TimedRoleBinding")
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}

		// If the end time is in the future, the role binding is active.
		r.SetObjectStatus(
			trb,
			rbacv1alpha1.TimedRoleBindingPhaseActive,
			"TimedRoleBinding is active",
		)

		if err := r.CreateRoleBinding(ctx, trb); err != nil {
			log.Error(err, "Failed to create RoleBinding")
			r.SetObjectStatus(
				trb,
				rbacv1alpha1.TimedRoleBindingPhaseFailed,
				fmt.Sprintf("Failed to create RoleBinding: %v", err),
			)
		}

		// Create the postActivate job (if specified)
		if isPostActivateJobEnabled(&spec) {
			log.Info("Creating postActivate job")
			if err := r.CreateHookJob(
				ctx,
				trb,
				r.GetObjectName(trb)+"-post-activate",
				*spec.PostActivate.JobTemplate.DeepCopy(),
			); err != nil {
				log.Error(err, "Failed to create postActivate job")
				r.SetObjectStatus(
					trb,
					rbacv1alpha1.TimedRoleBindingPhaseFailed,
					fmt.Sprintf("Failed to create postActivate job: %v", err),
				)
			}
		}

		if err := r.UpdateObjectStatus(ctx, trb); err != nil {
			log.Error(err, "Failed to update TimedRoleBinding status")
			return ctrl.Result{}, err
		}

		// Requeue after the end time.
		return ctrl.Result{RequeueAfter: endTime.Sub(now)}, nil
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
