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

// TimedClusterRoleBindingReconciler reconciles a TimedClusterRoleBinding object
type TimedClusterRoleBindingReconciler struct {
	Reconciler Reconciler
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
	return r.Reconciler.reconcileObject(ctx, req, &rbacv1alpha1.TimedClusterRoleBinding{})
}

// SetupWithManager sets up the controller with the Manager.
func (r *TimedClusterRoleBindingReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&rbacv1alpha1.TimedClusterRoleBinding{}).
		Owns(&rbacv1.ClusterRoleBinding{}).
		Named("timedclusterrolebinding").
		Complete(r)
}
