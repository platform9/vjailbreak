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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RollingMigrationPlanSpec defines the desired state of RollingMigrationPlan
type RollingMigrationPlanSpec struct {
}

// RollingMigrationPlanStatus defines the observed state of RollingMigrationPlan
type RollingMigrationPlanStatus struct {
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// RollingMigrationPlan is the Schema for the rollingmigrationplans API
type RollingMigrationPlan struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RollingMigrationPlanSpec   `json:"spec,omitempty"`
	Status RollingMigrationPlanStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// RollingMigrationPlanList contains a list of RollingMigrationPlan
type RollingMigrationPlanList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RollingMigrationPlan `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RollingMigrationPlan{}, &RollingMigrationPlanList{})
}
