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

// TimedClusterRoleBindingReconciler reconciles a TimedClusterRoleBinding object
type TimedClusterRoleBindingReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=rbac.hhh.github.io,resources=timedclusterrolebindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.hhh.github.io,resources=timedclusterrolebindings/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=rbac.hhh.github.io,resources=timedclusterrolebindings/finalizers,verbs=update
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterrolebindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles,verbs=bind
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the TimedClusterRoleBinding object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.20.4/pkg/reconcile
func (r *TimedClusterRoleBindingReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	return reconciles(ctx, req, r)
}

func (r *TimedClusterRoleBindingReconciler) GetObject(ctx context.Context, req ctrl.Request) (*rbacv1alpha1.TimedClusterRoleBinding, error) {
	trb := &rbacv1alpha1.TimedClusterRoleBinding{}
	if err := r.Get(ctx, req.NamespacedName, trb); err != nil {
		return nil, err
	}
	return trb, nil
}

// CreateRoleBinding creates the associated ClusterRoleBinding
func (r *TimedClusterRoleBindingReconciler) CreateRoleBinding(ctx context.Context, trb *rbacv1alpha1.TimedClusterRoleBinding) error {
	roleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: trb.GetName(),
		},
		Subjects: trb.Spec.Subjects,
		RoleRef:  trb.Spec.RoleRef,
	}

	// Sets the owner reference to the TimedClusterRoleBinding.
	// This ensures that the ClusterRoleBinding will be deleted when the TimedClusterRoleBinding is deleted.
	controllerutil.SetControllerReference(trb, roleBinding, r.Scheme)

	if err := r.Create(ctx, roleBinding); client.IgnoreAlreadyExists(err) != nil {
		return err
	}
	return nil
}

// DeleteRoleBinding deletes the associated ClusterRoleBinding
func (r *TimedClusterRoleBindingReconciler) DeleteRoleBinding(ctx context.Context, trb *rbacv1alpha1.TimedClusterRoleBinding) error {
	if err := r.Delete(ctx, &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: trb.GetName(),
		},
	}); client.IgnoreNotFound(err) != nil {
		return err
	}
	return nil
}

// CreateHookJob creates a hook job
func (r *TimedClusterRoleBindingReconciler) CreateHookJob(ctx context.Context, trb *rbacv1alpha1.TimedClusterRoleBinding, name string, templateSpec rbacv1alpha1.JobTemplateSpec) error {
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: templateSpec.Namespace, // job must have a namespace
		},
		Spec: templateSpec.Spec,
	}

	// Inject the TimedClusterRoleBinding name as env variable into the job's containers.
	for i := range job.Spec.Template.Spec.Containers {
		job.Spec.Template.Spec.Containers[i].Env = append(job.Spec.Template.Spec.Containers[i].Env, corev1.EnvVar{
			Name:  "TIMED_CLUSTER_ROLE_BINDING_NAME",
			Value: trb.GetName(),
		})
	}

	// Sets the owner reference to the TimedClusterRoleBinding.
	// This ensures that the Job will be deleted when the TimedClusterRoleBinding is deleted.
	controllerutil.SetControllerReference(trb, job, r.Scheme)

	if err := r.Create(ctx, job); client.IgnoreAlreadyExists(err) != nil {
		return err
	}
	return nil
}

func (r *TimedClusterRoleBindingReconciler) GetObjectName(trb *rbacv1alpha1.TimedClusterRoleBinding) string {
	return trb.GetName()
}

func (r *TimedClusterRoleBindingReconciler) GetObjectNamespace(trb *rbacv1alpha1.TimedClusterRoleBinding) string {
	return trb.GetNamespace()
}

func (r *TimedClusterRoleBindingReconciler) GetObjectSpec(trb *rbacv1alpha1.TimedClusterRoleBinding) rbacv1alpha1.TimedRoleBindingSpec {
	return trb.Spec
}

func (r *TimedClusterRoleBindingReconciler) GetObjectStatus(trb *rbacv1alpha1.TimedClusterRoleBinding) *rbacv1alpha1.TimedRoleBindingStatus {
	return &trb.Status
}

func (r *TimedClusterRoleBindingReconciler) SetObjectStatus(trb *rbacv1alpha1.TimedClusterRoleBinding, phase rbacv1alpha1.TimedRoleBindingPhase, msg string) {
	trb.Status.Phase = phase
	trb.Status.Message = msg
}

func (r *TimedClusterRoleBindingReconciler) UpdateObjectStatus(ctx context.Context, trb *rbacv1alpha1.TimedClusterRoleBinding) error {
	return r.Status().Update(ctx, trb)
}

// SetupWithManager sets up the controller with the Manager.
func (r *TimedClusterRoleBindingReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&rbacv1alpha1.TimedClusterRoleBinding{}).
		Owns(&rbacv1.ClusterRoleBinding{}).
		Named("timedclusterrolebinding").
		Complete(r)
}
