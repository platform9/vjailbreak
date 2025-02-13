/*
Copyright 2024.

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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MigrationSpec defines the desired state of Migration
type MigrationSpec struct {
	MigrationPlan   string `json:"migrationPlan"`
	PodRef          string `json:"podRef"`
	VMName          string `json:"vmName"`
	InitiateCutover bool   `json:"initiateCutover"`
}

// MigrationStatus defines the observed state of Migration
type MigrationStatus struct {
	Phase      string                `json:"phase"`
	Conditions []corev1.PodCondition `json:"conditions"`
	AgentName  string                `json:"AgentName,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Migration is the Schema for the migrations API
type Migration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MigrationSpec   `json:"spec,omitempty"`
	Status MigrationStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// MigrationList contains a list of Migration
type MigrationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Migration `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Migration{}, &MigrationList{})
}
