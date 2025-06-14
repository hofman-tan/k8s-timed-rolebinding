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

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	rbacv1alpha1 "github.com/hofman-tan/k8s-timed-rolebinding/api/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// nolint:unused
// log is for logging in this package.
var trbLog = logf.Log.WithName("timedrolebinding-resource")

// SetupTimedRoleBindingWebhookWithManager registers the webhook for TimedRoleBinding in the manager.
func SetupTimedRoleBindingWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&rbacv1alpha1.TimedRoleBinding{}).
		WithValidator(&TimedRoleBindingCustomValidator{}).
		WithDefaulter(&TimedRoleBindingCustomDefaulter{
			DefaultKeepExpiredFor: metav1.Duration{
				Duration: 24 * time.Hour, // 1 day
			},
		}).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-rbac-hhh-github-io-v1alpha1-timedrolebinding,mutating=true,failurePolicy=fail,sideEffects=None,groups=rbac.hhh.github.io,resources=timedrolebindings,verbs=create;update,versions=v1alpha1,name=mtimedrolebinding-v1alpha1.kb.io,admissionReviewVersions=v1

// TimedRoleBindingCustomDefaulter struct is responsible for setting default values on the custom resource of the
// Kind TimedRoleBinding when those are created or updated.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as it is used only for temporary operations and does not need to be deeply copied.
type TimedRoleBindingCustomDefaulter struct {
	DefaultKeepExpiredFor metav1.Duration
}

var _ webhook.CustomDefaulter = &TimedRoleBindingCustomDefaulter{}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the Kind TimedRoleBinding.
func (d *TimedRoleBindingCustomDefaulter) Default(ctx context.Context, obj runtime.Object) error {
	trb, ok := obj.(*rbacv1alpha1.TimedRoleBinding)

	if !ok {
		return fmt.Errorf("expected an TimedRoleBinding object but got %T", obj)
	}
	trbLog.Info("Defaulting for TimedRoleBinding", "name", trb.GetName())

	// Apply defaults
	if trb.Spec.KeepExpiredFor == nil {
		trbLog.Info("Defaulting keepExpiredFor", "name", trb.GetName())
		trb.Spec.KeepExpiredFor = &d.DefaultKeepExpiredFor
	}

	return nil
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
// NOTE: The 'path' attribute must follow a specific pattern and should not be modified directly here.
// Modifying the path for an invalid path can cause API server errors; failing to locate the webhook.
// +kubebuilder:webhook:path=/validate-rbac-hhh-github-io-v1alpha1-timedrolebinding,mutating=false,failurePolicy=fail,sideEffects=None,groups=rbac.hhh.github.io,resources=timedrolebindings,verbs=create;update,versions=v1alpha1,name=vtimedrolebinding-v1alpha1.kb.io,admissionReviewVersions=v1

// TimedRoleBindingCustomValidator struct is responsible for validating the TimedRoleBinding resource
// when it is created, updated, or deleted.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
type TimedRoleBindingCustomValidator struct {
	// TODO(user): Add more fields as needed for validation
}

var _ webhook.CustomValidator = &TimedRoleBindingCustomValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type TimedRoleBinding.
func (v *TimedRoleBindingCustomValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	trb, ok := obj.(*rbacv1alpha1.TimedRoleBinding)
	if !ok {
		return nil, fmt.Errorf("expected a TimedRoleBinding object but got %T", obj)
	}
	trbLog.Info("Validation for TimedRoleBinding upon creation", "name", trb.GetName())

	// TODO(user): fill in your validation logic upon object creation.

	return nil, validateTimedRoleBinding(trb)
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type TimedRoleBinding.
func (v *TimedRoleBindingCustomValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	trb, ok := newObj.(*rbacv1alpha1.TimedRoleBinding)
	if !ok {
		return nil, fmt.Errorf("expected a TimedRoleBinding object for the newObj but got %T", newObj)
	}
	trbLog.Info("Validation for TimedRoleBinding upon update", "name", trb.GetName())

	// TODO(user): fill in your validation logic upon object update.

	return nil, validateTimedRoleBinding(trb)
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type TimedRoleBinding.
func (v *TimedRoleBindingCustomValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	trb, ok := obj.(*rbacv1alpha1.TimedRoleBinding)
	if !ok {
		return nil, fmt.Errorf("expected a TimedRoleBinding object but got %T", obj)
	}
	trbLog.Info("Validation for TimedRoleBinding upon deletion", "name", trb.GetName())

	// TODO(user): fill in your validation logic upon object deletion.

	return nil, nil
}

func validateTimedRoleBinding(trb *rbacv1alpha1.TimedRoleBinding) error {
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

	if len(allErrs) > 0 {
		return errors.NewInvalid(schema.GroupKind{Group: rbacv1alpha1.GroupVersion.Group, Kind: "TimedRoleBinding"}, trb.Name, allErrs)
	}

	return nil
}
