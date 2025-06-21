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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"

	rbacv1alpha1 "github.com/hofman-tan/k8s-timed-rolebinding/api/v1alpha1"
)

func cleanup(ctx context.Context, typeNamespacedName types.NamespacedName) {
	// Cleanup TimedRoleBinding if exists
	timedRoleBinding := &rbacv1alpha1.TimedRoleBinding{}
	err := k8sClient.Get(ctx, typeNamespacedName, timedRoleBinding)
	if err == nil {
		By("Cleanup the specific resource instance TimedRoleBinding")
		Expect(k8sClient.Delete(ctx, timedRoleBinding)).To(Succeed())
	}

	// Cleanup RoleBinding if exists
	roleBinding := &rbacv1.RoleBinding{}
	err = k8sClient.Get(ctx, typeNamespacedName, roleBinding)
	if err == nil {
		Expect(k8sClient.Delete(ctx, roleBinding)).To(Succeed())
	}
}

var _ = Describe("TimedRoleBinding Controller", func() {
	const (
		resourceName = "test-resource"
		namespace    = "default"
	)

	Context("When reconciling a Pending TimedRoleBinding", func() {
		ctx := context.Background()

		It("should set status.phase to Pending when start time is in the future", func() {
			// Create TimedRoleBinding with future start time
			startTime := metav1.NewTime(time.Now().UTC().Add(1 * time.Hour))
			endTime := metav1.NewTime(time.Now().UTC().Add(2 * time.Hour))
			hash := rand.String(5)
			resourceName := resourceName + hash
			typeNamespacedName := types.NamespacedName{
				Name:      resourceName,
				Namespace: namespace,
			}

			timedRoleBinding := &rbacv1alpha1.TimedRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: namespace,
				},
				Spec: rbacv1alpha1.TimedRoleBindingSpec{
					Subjects: []rbacv1.Subject{
						{
							Kind:     "User",
							Name:     "test-user" + hash,
							APIGroup: "rbac.authorization.k8s.io",
						},
					},
					RoleRef: rbacv1.RoleRef{
						APIGroup: "rbac.authorization.k8s.io",
						Kind:     "Role",
						Name:     "test-role" + hash,
					},
					StartTime: startTime,
					EndTime:   endTime,
				},
			}

			Expect(k8sClient.Create(ctx, timedRoleBinding)).To(Succeed())

			// Verify status
			Eventually(func(g Gomega) {
				createdTimedRoleBinding := &rbacv1alpha1.TimedRoleBinding{}
				g.Expect(k8sClient.Get(ctx, typeNamespacedName, createdTimedRoleBinding)).To(Succeed())
				g.Expect(createdTimedRoleBinding.Status.Phase).To(Equal(rbacv1alpha1.TimedRoleBindingPhasePending))
			}, timeout, interval).Should(Succeed())

			// Verify no RoleBinding is created
			roleBinding := &rbacv1.RoleBinding{}
			err := k8sClient.Get(ctx, typeNamespacedName, roleBinding)
			Expect(errors.IsNotFound(err)).To(BeTrue())

			cleanup(ctx, typeNamespacedName)
		})
	})

	Context("When reconciling an Active TimedRoleBinding", func() {
		ctx := context.Background()

		It("should set status.phase to Active and create RoleBinding when within time window", func() {
			// Create TimedRoleBinding with current time window
			startTime := metav1.NewTime(time.Now().UTC().Add(-1 * time.Hour))
			endTime := metav1.NewTime(time.Now().UTC().Add(1 * time.Hour))
			hash := rand.String(5)
			resourceName := resourceName + hash
			typeNamespacedName := types.NamespacedName{
				Name:      resourceName,
				Namespace: namespace,
			}

			timedRoleBinding := &rbacv1alpha1.TimedRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: namespace,
				},
				Spec: rbacv1alpha1.TimedRoleBindingSpec{
					Subjects: []rbacv1.Subject{
						{
							Kind:     "User",
							Name:     "test-user" + hash,
							APIGroup: "rbac.authorization.k8s.io",
						},
					},
					RoleRef: rbacv1.RoleRef{
						APIGroup: "rbac.authorization.k8s.io",
						Kind:     "Role",
						Name:     "test-role" + hash,
					},
					StartTime: startTime,
					EndTime:   endTime,
					PostActivate: &rbacv1alpha1.PostActivate{
						JobTemplate: &rbacv1alpha1.JobTemplateSpec{
							Spec: batchv1.JobSpec{
								Template: corev1.PodTemplateSpec{
									Spec: corev1.PodSpec{
										Containers: []corev1.Container{
											{
												Name:  "test",
												Image: "busybox",
												Command: []string{
													"echo",
													"activated",
												},
											},
										},
										RestartPolicy: corev1.RestartPolicyNever,
									},
								},
							},
						},
					},
				},
			}

			Expect(k8sClient.Create(ctx, timedRoleBinding)).To(Succeed())

			// Verify status
			Eventually(func(g Gomega) {
				createdTimedRoleBinding := &rbacv1alpha1.TimedRoleBinding{}
				g.Expect(k8sClient.Get(ctx, typeNamespacedName, createdTimedRoleBinding)).To(Succeed())
				g.Expect(createdTimedRoleBinding.Status.Phase).To(Equal(rbacv1alpha1.TimedRoleBindingPhaseActive))
			}, timeout, interval).Should(Succeed())

			// Verify RoleBinding is created
			roleBinding := &rbacv1.RoleBinding{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, roleBinding)).To(Succeed())
			Expect(roleBinding.Subjects).To(Equal(timedRoleBinding.Spec.Subjects))
			Expect(roleBinding.RoleRef).To(Equal(timedRoleBinding.Spec.RoleRef))

			// Verify post-activate job was created
			job := &batchv1.Job{}
			jobName := resourceName + "-post-activate"
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: jobName, Namespace: namespace}, job)).To(Succeed())

			cleanup(ctx, typeNamespacedName)
		})
	})

	Context("When reconciling an Expired TimedRoleBinding", func() {
		ctx := context.Background()

		It("should set status.phase to Expired, delete RoleBinding, and create post-expire job", func() {
			// Create TimedRoleBinding with expired time window
			startTime := metav1.NewTime(time.Now().UTC().Add(-2 * time.Hour))
			endTime := metav1.NewTime(time.Now().UTC().Add(-1 * time.Hour))
			keepExpiredFor := &metav1.Duration{Duration: 24 * time.Hour}
			hash := rand.String(5)
			resourceName := resourceName + hash
			typeNamespacedName := types.NamespacedName{
				Name:      resourceName,
				Namespace: namespace,
			}

			timedRoleBinding := &rbacv1alpha1.TimedRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: namespace,
				},
				Spec: rbacv1alpha1.TimedRoleBindingSpec{
					Subjects: []rbacv1.Subject{
						{
							Kind:     "User",
							Name:     "test-user" + hash,
							APIGroup: "rbac.authorization.k8s.io",
						},
					},
					RoleRef: rbacv1.RoleRef{
						APIGroup: "rbac.authorization.k8s.io",
						Kind:     "Role",
						Name:     "test-role" + hash,
					},
					StartTime:      startTime,
					EndTime:        endTime,
					KeepExpiredFor: keepExpiredFor,
					PostExpire: &rbacv1alpha1.PostExpire{
						JobTemplate: &rbacv1alpha1.JobTemplateSpec{
							Spec: batchv1.JobSpec{
								Template: corev1.PodTemplateSpec{
									Spec: corev1.PodSpec{
										Containers: []corev1.Container{
											{
												Name:  "test",
												Image: "busybox",
												Command: []string{
													"echo",
													"expired",
												},
											},
										},
										RestartPolicy: corev1.RestartPolicyNever,
									},
								},
							},
						},
					},
				},
			}

			// First create a RoleBinding to be deleted
			roleBinding := &rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: namespace,
				},
				Subjects: timedRoleBinding.Spec.Subjects,
				RoleRef:  timedRoleBinding.Spec.RoleRef,
			}
			Expect(k8sClient.Create(ctx, roleBinding)).To(Succeed())

			// Then create the TimedRoleBinding
			Expect(k8sClient.Create(ctx, timedRoleBinding)).To(Succeed())

			// Verify status
			Eventually(func(g Gomega) {
				createdTimedRoleBinding := &rbacv1alpha1.TimedRoleBinding{}
				g.Expect(k8sClient.Get(ctx, typeNamespacedName, createdTimedRoleBinding)).To(Succeed())
				g.Expect(createdTimedRoleBinding.Status.Phase).To(Equal(rbacv1alpha1.TimedRoleBindingPhaseExpired))
			}, timeout, interval).Should(Succeed())

			// Verify RoleBinding was deleted
			err := k8sClient.Get(ctx, typeNamespacedName, roleBinding)
			Expect(errors.IsNotFound(err)).To(BeTrue())

			// Verify post-expire job was created
			job := &batchv1.Job{}
			jobName := resourceName + "-post-expire"
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: jobName, Namespace: namespace}, job)).To(Succeed())

			// Resource should not be deleted yet due to keepExpiredFor
			Expect(k8sClient.Get(ctx, typeNamespacedName, timedRoleBinding)).To(Succeed())

			cleanup(ctx, typeNamespacedName)
		})
	})
})
