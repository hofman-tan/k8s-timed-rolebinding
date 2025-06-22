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

	rbacv1alpha1 "github.com/hofman-tan/k8s-timed-rolebinding/api/v1alpha1"
	rbacv1 "k8s.io/api/rbac/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

// TimedRoleBindingReconciler reconciles a TimedRoleBinding object
type TimedRoleBindingReconciler struct {
	Reconciler Reconciler
}

// +kubebuilder:rbac:groups=rbac.hhh.github.io,resources=timedrolebindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.hhh.github.io,resources=timedrolebindings/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=rbac.hhh.github.io,resources=timedrolebindings/finalizers,verbs=update
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles,verbs=bind
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete

func (r *TimedRoleBindingReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	return r.Reconciler.reconcileObject(ctx, req, &rbacv1alpha1.TimedRoleBinding{})
}

// SetupWithManager sets up the controller with the Manager.
func (r *TimedRoleBindingReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&rbacv1alpha1.TimedRoleBinding{}).
		Owns(&rbacv1.RoleBinding{}).
		Named("timedrolebinding").
		Complete(r)
}
