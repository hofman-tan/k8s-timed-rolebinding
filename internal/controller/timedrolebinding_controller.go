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

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	rbacv1alpha1 "github.com/hofman-tan/k8s-timed-rolebinding/api/v1alpha1"
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
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete

func (r *TimedRoleBindingReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	return reconciles(ctx, req, r)
}

func (r *TimedRoleBindingReconciler) GetObject(ctx context.Context, req ctrl.Request) (*rbacv1alpha1.TimedRoleBinding, error) {
	trb := &rbacv1alpha1.TimedRoleBinding{}
	if err := r.Get(ctx, req.NamespacedName, trb); err != nil {
		return nil, err
	}
	return trb, nil
}

// CreateRoleBinding creates the associated RoleBinding
func (r *TimedRoleBindingReconciler) CreateRoleBinding(ctx context.Context, trb *rbacv1alpha1.TimedRoleBinding) error {
	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      trb.GetName(),
			Namespace: trb.GetNamespace(),
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

// DeleteRoleBinding deletes the associated RoleBinding
func (r *TimedRoleBindingReconciler) DeleteRoleBinding(ctx context.Context, trb *rbacv1alpha1.TimedRoleBinding) error {
	if err := r.Delete(ctx, &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      trb.GetName(),
			Namespace: trb.GetNamespace(),
		},
	}); client.IgnoreNotFound(err) != nil {
		return err
	}
	return nil
}

// CreateHookJob creates a hook job
func (r *TimedRoleBindingReconciler) CreateHookJob(ctx context.Context, trb *rbacv1alpha1.TimedRoleBinding, name string, templateSpec rbacv1alpha1.JobTemplateSpec) error {
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: trb.GetNamespace(), // always in the same namespace as the CR
		},
		Spec: templateSpec.Spec,
	}

	// Inject the TimedRoleBinding name as env variable into the job's containers.
	for i := range job.Spec.Template.Spec.Containers {
		job.Spec.Template.Spec.Containers[i].Env = append(job.Spec.Template.Spec.Containers[i].Env, corev1.EnvVar{
			Name:  "TIMED_ROLE_BINDING_NAME",
			Value: trb.GetName(),
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

func (r *TimedRoleBindingReconciler) GetObjectName(trb *rbacv1alpha1.TimedRoleBinding) string {
	return trb.GetName()
}

func (r *TimedRoleBindingReconciler) GetObjectNamespace(trb *rbacv1alpha1.TimedRoleBinding) string {
	return trb.GetNamespace()
}

func (r *TimedRoleBindingReconciler) GetObjectSpec(trb *rbacv1alpha1.TimedRoleBinding) rbacv1alpha1.TimedRoleBindingSpec {
	return trb.Spec
}

func (r *TimedRoleBindingReconciler) GetObjectStatus(trb *rbacv1alpha1.TimedRoleBinding) *rbacv1alpha1.TimedRoleBindingStatus {
	return &trb.Status
}

func (r *TimedRoleBindingReconciler) SetObjectStatus(trb *rbacv1alpha1.TimedRoleBinding, phase rbacv1alpha1.TimedRoleBindingPhase, msg string) {
	trb.Status.Phase = phase
	trb.Status.Message = msg
}

func (r *TimedRoleBindingReconciler) UpdateObjectStatus(ctx context.Context, trb *rbacv1alpha1.TimedRoleBinding) error {
	return r.Status().Update(ctx, trb)
}

// SetupWithManager sets up the controller with the Manager.
func (r *TimedRoleBindingReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&rbacv1alpha1.TimedRoleBinding{}).
		Owns(&rbacv1.RoleBinding{}).
		Named("timedrolebinding").
		Complete(r)
}
