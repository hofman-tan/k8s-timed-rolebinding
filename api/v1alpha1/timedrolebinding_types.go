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
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type TimedRoleBindingPhase string

const (
	TimedRoleBindingPhasePending TimedRoleBindingPhase = "Pending"
	TimedRoleBindingPhaseActive  TimedRoleBindingPhase = "Active"
	TimedRoleBindingPhaseExpired TimedRoleBindingPhase = "Expired"
	TimedRoleBindingPhaseFailed  TimedRoleBindingPhase = "Failed"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

func (trb *TimedRoleBinding) GetSpec() *TimedRoleBindingSpec {
	return &trb.Spec
}

func (trb *TimedRoleBinding) GetStatus() *TimedRoleBindingStatus {
	return &trb.Status
}

func (trb *TimedRoleBinding) BuildObjectForRoleBinding() client.Object {
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      trb.GetName(),
			Namespace: trb.GetNamespace(),
		},
		Subjects: trb.Spec.Subjects,
		RoleRef:  trb.Spec.RoleRef,
	}
}

func (trb *TimedRoleBinding) BuildJobObject(name string, jobTemplateSpec *batchv1.JobTemplateSpec) batchv1.Job {
	job := batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: trb.GetNamespace(), // always in the same namespace as the CR
		},
		Spec: jobTemplateSpec.Spec,
	}

	// Inject object name as an environment variable into the containers
	for i := range job.Spec.Template.Spec.Containers {
		job.Spec.Template.Spec.Containers[i].Env = append(job.Spec.Template.Spec.Containers[i].Env, corev1.EnvVar{
			Name:  "TIMED_ROLE_BINDING_NAME",
			Value: trb.GetName(),
		})
	}

	return job
}

type PostActivate struct {
	JobTemplate *batchv1.JobTemplateSpec `json:"jobTemplate,omitempty"`
}

type PostExpire struct {
	JobTemplate *batchv1.JobTemplateSpec `json:"jobTemplate,omitempty"`
}

// TimedRoleBindingSpec defines the desired state of TimedRoleBinding.
type TimedRoleBindingSpec struct {
	Subjects       []rbacv1.Subject `json:"subjects"`
	RoleRef        rbacv1.RoleRef   `json:"roleRef"`
	StartTime      metav1.Time      `json:"startTime"`
	EndTime        metav1.Time      `json:"endTime"`
	KeepExpiredFor *metav1.Duration `json:"keepExpiredFor,omitempty"` // how long to keep the CRD after it expires
	PostActivate   *PostActivate    `json:"postActivate,omitempty"`
	PostExpire     *PostExpire      `json:"postExpire,omitempty"`
}

// TimedRoleBindingStatus defines the observed state of TimedRoleBinding.
type TimedRoleBindingStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Phase              TimedRoleBindingPhase `json:"phase,omitempty"`
	Message            string                `json:"message,omitempty"`
	LastTransitionTime metav1.Time           `json:"lastTransitionTime,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// TimedRoleBinding is the Schema for the timedrolebindings API.
type TimedRoleBinding struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TimedRoleBindingSpec   `json:"spec,omitempty"`
	Status TimedRoleBindingStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Namespaced

// TimedRoleBindingList contains a list of TimedRoleBinding.
type TimedRoleBindingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TimedRoleBinding `json:"items"`
}

func init() {
	SchemeBuilder.Register(&TimedRoleBinding{}, &TimedRoleBindingList{})
}
