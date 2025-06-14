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

package v1alpha1

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	rbacv1alpha1 "github.com/hofman-tan/k8s-timed-rolebinding/api/v1alpha1"
)

// nolint:unused
// log is for logging in this package.
var timedclusterrolebindinglog = logf.Log.WithName("timedclusterrolebinding-resource")

// SetupTimedClusterRoleBindingWebhookWithManager registers the webhook for TimedClusterRoleBinding in the manager.
func SetupTimedClusterRoleBindingWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&rbacv1alpha1.TimedClusterRoleBinding{}).
		WithValidator(&TimedClusterRoleBindingCustomValidator{}).
		WithDefaulter(&TimedClusterRoleBindingCustomDefaulter{
			DefaultKeepExpiredFor: metav1.Duration{
				Duration: 24 * time.Hour, // 1 day
			},
		}).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// +kubebuilder:webhook:path=/mutate-rbac-hhh-github-io-v1alpha1-timedclusterrolebinding,mutating=true,failurePolicy=fail,sideEffects=None,groups=rbac.hhh.github.io,resources=timedclusterrolebindings,verbs=create;update,versions=v1alpha1,name=mtimedclusterrolebinding-v1alpha1.kb.io,admissionReviewVersions=v1

// TimedClusterRoleBindingCustomDefaulter struct is responsible for setting default values on the custom resource of the
// Kind TimedClusterRoleBinding when those are created or updated.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as it is used only for temporary operations and does not need to be deeply copied.
type TimedClusterRoleBindingCustomDefaulter struct {
	DefaultKeepExpiredFor metav1.Duration
}

var _ webhook.CustomDefaulter = &TimedClusterRoleBindingCustomDefaulter{}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the Kind TimedClusterRoleBinding.
func (d *TimedClusterRoleBindingCustomDefaulter) Default(ctx context.Context, obj runtime.Object) error {
	timedclusterrolebinding, ok := obj.(*rbacv1alpha1.TimedClusterRoleBinding)

	if !ok {
		return fmt.Errorf("expected an TimedClusterRoleBinding object but got %T", obj)
	}
	timedclusterrolebindinglog.Info("Defaulting for TimedClusterRoleBinding", "name", timedclusterrolebinding.GetName())

	// Apply defaults
	if timedclusterrolebinding.Spec.KeepExpiredFor == nil {
		timedclusterrolebindinglog.Info("Defaulting keepExpiredFor", "name", timedclusterrolebinding.GetName())
		timedclusterrolebinding.Spec.KeepExpiredFor = &d.DefaultKeepExpiredFor
	}

	return nil
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
// NOTE: The 'path' attribute must follow a specific pattern and should not be modified directly here.
// Modifying the path for an invalid path can cause API server errors; failing to locate the webhook.
// +kubebuilder:webhook:path=/validate-rbac-hhh-github-io-v1alpha1-timedclusterrolebinding,mutating=false,failurePolicy=fail,sideEffects=None,groups=rbac.hhh.github.io,resources=timedclusterrolebindings,verbs=create;update,versions=v1alpha1,name=vtimedclusterrolebinding-v1alpha1.kb.io,admissionReviewVersions=v1

// TimedClusterRoleBindingCustomValidator struct is responsible for validating the TimedClusterRoleBinding resource
// when it is created, updated, or deleted.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
type TimedClusterRoleBindingCustomValidator struct {
	// TODO(user): Add more fields as needed for validation
}

var _ webhook.CustomValidator = &TimedClusterRoleBindingCustomValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type TimedClusterRoleBinding.
func (v *TimedClusterRoleBindingCustomValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	timedclusterrolebinding, ok := obj.(*rbacv1alpha1.TimedClusterRoleBinding)
	if !ok {
		return nil, fmt.Errorf("expected a TimedClusterRoleBinding object but got %T", obj)
	}
	timedclusterrolebindinglog.Info("Validation for TimedClusterRoleBinding upon creation", "name", timedclusterrolebinding.GetName())

	// TODO(user): fill in your validation logic upon object creation.

	return nil, validateTimedClusterRoleBinding(timedclusterrolebinding)
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type TimedClusterRoleBinding.
func (v *TimedClusterRoleBindingCustomValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	timedclusterrolebinding, ok := newObj.(*rbacv1alpha1.TimedClusterRoleBinding)
	if !ok {
		return nil, fmt.Errorf("expected a TimedClusterRoleBinding object for the newObj but got %T", newObj)
	}
	timedclusterrolebindinglog.Info("Validation for TimedClusterRoleBinding upon update", "name", timedclusterrolebinding.GetName())

	// TODO(user): fill in your validation logic upon object update.

	return nil, validateTimedClusterRoleBinding(timedclusterrolebinding)
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type TimedClusterRoleBinding.
func (v *TimedClusterRoleBindingCustomValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	timedclusterrolebinding, ok := obj.(*rbacv1alpha1.TimedClusterRoleBinding)
	if !ok {
		return nil, fmt.Errorf("expected a TimedClusterRoleBinding object but got %T", obj)
	}
	timedclusterrolebindinglog.Info("Validation for TimedClusterRoleBinding upon deletion", "name", timedclusterrolebinding.GetName())

	// TODO(user): fill in your validation logic upon object deletion.

	return nil, nil
}

// TODO: consolidate with validateTimedRoleBinding
func validateTimedClusterRoleBinding(trb *rbacv1alpha1.TimedClusterRoleBinding) error {
	var allErrs field.ErrorList

	// Start time must be before end time
	if !trb.Spec.StartTime.Before(&trb.Spec.EndTime) {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("startTime"), trb.Spec.StartTime, "startTime must be before end time"))
	}

	// End time must be after now
	now := metav1.Now()
	if !trb.Spec.EndTime.After(now.Time) {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("endTime"), trb.Spec.EndTime, "endTime must be after now"))
	}

	// For TimedClusterRoleBinding, jobs must always have a namespace (if specified)
	if trb.Spec.PostActivate != nil && trb.Spec.PostActivate.JobTemplate != nil && trb.Spec.PostActivate.JobTemplate.GetNamespace() == "" {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("postActivate").Child("jobTemplate").Child("namespace"), trb.Spec.PostActivate.JobTemplate.Namespace, "namespace must be present in jobTemplate for TimedClusterRoleBinding"))
	}
	if trb.Spec.PostExpire != nil && trb.Spec.PostExpire.JobTemplate != nil && trb.Spec.PostExpire.JobTemplate.GetNamespace() == "" {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("postExpire").Child("jobTemplate").Child("namespace"), trb.Spec.PostExpire.JobTemplate.Namespace, "namespace must be present in jobTemplate for TimedClusterRoleBinding"))
	}

	if len(allErrs) > 0 {
		return errors.NewInvalid(schema.GroupKind{Group: rbacv1alpha1.GroupVersion.Group, Kind: "TimedRoleBinding"}, trb.Name, allErrs)
	}
	return nil
}
