package controller

import (
	"context"
	"fmt"
	"strings"
	"time"

	rbacv1alpha1 "github.com/hofman-tan/k8s-timed-rolebinding/api/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// Reconciler is an interface for the reconciler that handles both TimedRoleBinding and TimedClusterRoleBinding
type Reconciler[T rbacv1alpha1.TimedRoleBinding | rbacv1alpha1.TimedClusterRoleBinding] interface {
	GetObject(ctx context.Context, req ctrl.Request) (*T, error)
	DeleteObject(ctx context.Context, trb *T) error
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
	name string,
) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	trb, err := r.GetObject(ctx, req)
	if err != nil {
		if errors.IsNotFound(err) {
			// TimedRoleBinding not found. No need to requeue.
			log.Info(fmt.Sprintf("%s not found. Ignoring since it must have been deleted", name))
			return ctrl.Result{}, nil
		}
		log.Error(err, fmt.Sprintf("Failed to get %s", name))
		return ctrl.Result{}, err
	}

	spec := r.GetObjectSpec(trb)
	now := time.Now()
	startTime := spec.StartTime.Time
	endTime := spec.EndTime.Time

	// Pending activation
	if startTime.After(now) {
		r.SetObjectStatus(
			trb,
			rbacv1alpha1.TimedRoleBindingPhasePending,
			fmt.Sprintf("%s is queued for activation", name),
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
			log.Error(err, fmt.Sprintf("Failed to update %s status", name))
			return ctrl.Result{}, err
		}

		log.Info(fmt.Sprintf("%s is queued for activation", name), "name", r.GetObjectName(trb), "namespace", r.GetObjectNamespace(trb))

		// Requeue for activation
		return ctrl.Result{RequeueAfter: startTime.Sub(now)}, nil
	}

	// Active
	if !startTime.After(now) && endTime.After(now) {
		r.SetObjectStatus(
			trb,
			rbacv1alpha1.TimedRoleBindingPhaseActive,
			fmt.Sprintf("%s is active", name),
		)

		errs := []string{}

		if err := r.CreateRoleBinding(ctx, trb); err != nil {
			log.Error(err, "Failed to create RoleBinding")
			errs = append(errs, fmt.Sprintf("Failed to create RoleBinding: %v", err))
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
				errs = append(errs, fmt.Sprintf("Failed to create postActivate job: %v", err))
			}
		}

		if len(errs) > 0 {
			r.SetObjectStatus(
				trb,
				rbacv1alpha1.TimedRoleBindingPhaseFailed,
				fmt.Sprintf("Failed to create RoleBinding: %s", strings.Join(errs, ". ")),
			)
		}

		if err := r.UpdateObjectStatus(ctx, trb); err != nil {
			log.Error(err, fmt.Sprintf("Failed to update %s status", name))
			return ctrl.Result{}, err
		}

		// Requeue for expiration
		return ctrl.Result{RequeueAfter: endTime.Sub(now)}, nil
	}

	// Expired
	r.SetObjectStatus(
		trb,
		rbacv1alpha1.TimedRoleBindingPhaseExpired,
		fmt.Sprintf("%s has expired", name),
	)

	errs := []string{}

	// Make sure role binding is deleted
	if err := r.DeleteRoleBinding(ctx, trb); err != nil {
		log.Error(err, "Failed to delete RoleBinding")
		errs = append(errs, fmt.Sprintf("Failed to delete RoleBinding: %v", err))
	}

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
			errs = append(errs, fmt.Sprintf("Failed to create postExpire job: %v", err))
		}
	}

	if len(errs) > 0 {
		r.SetObjectStatus(
			trb,
			rbacv1alpha1.TimedRoleBindingPhaseFailed,
			fmt.Sprintf("Failed to create RoleBinding: %s", strings.Join(errs, ". ")),
		)
	}

	if err := r.UpdateObjectStatus(ctx, trb); err != nil {
		log.Error(err, fmt.Sprintf("Failed to update %s status", name))
		return ctrl.Result{}, err
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
	if err := r.DeleteObject(ctx, trb); err != nil {
		log.Error(err, fmt.Sprintf("Failed to delete %s", name))
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
