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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

func (trb *TimedClusterRoleBinding) GetSpec() *TimedRoleBindingSpec {
	return &trb.Spec
}

func (trb *TimedClusterRoleBinding) GetStatus() *TimedRoleBindingStatus {
	return &trb.Status
}

func (trb *TimedClusterRoleBinding) BuildObjectForRoleBinding() client.Object {

	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: trb.GetName(),
		},
		Subjects: trb.Spec.Subjects,
		RoleRef:  trb.Spec.RoleRef,
	}
}

func (trb *TimedClusterRoleBinding) BuildJobObject(name string, jobTemplateSpec *batchv1.JobTemplateSpec) batchv1.Job {
	job := batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: jobTemplateSpec.GetNamespace(), // job must have a namespace
		},
		Spec: jobTemplateSpec.Spec,
	}

	// Inject object name as an environment variable into the containers
	for i := range job.Spec.Template.Spec.Containers {
		job.Spec.Template.Spec.Containers[i].Env = append(job.Spec.Template.Spec.Containers[i].Env, corev1.EnvVar{
			Name:  "TIMED_CLUSTER_ROLE_BINDING_NAME",
			Value: trb.GetName(),
		})
	}

	return job
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// TimedClusterRoleBinding is the Schema for the timedclusterrolebindings API.
type TimedClusterRoleBinding struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TimedRoleBindingSpec   `json:"spec,omitempty"`
	Status TimedRoleBindingStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster

// TimedClusterRoleBindingList contains a list of TimedClusterRoleBinding.
type TimedClusterRoleBindingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TimedClusterRoleBinding `json:"items"`
}

func init() {
	SchemeBuilder.Register(&TimedClusterRoleBinding{}, &TimedClusterRoleBindingList{})
}
