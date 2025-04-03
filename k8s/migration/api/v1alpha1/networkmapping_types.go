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

// NetworkMappingSpec defines the desired state of NetworkMapping
type NetworkMappingSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Networks is the list of networks
	Networks []Network `json:"networks"`
}

// Network defines the mapping between source and destination networks
type Network struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

// NetworkMappingStatus defines the observed state of NetworkMapping
type NetworkMappingStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	NetworkmappingValidationStatus  string `json:"networkMappingValidationStatus,omitempty"`
	NetworkmappingValidationMessage string `json:"networkMappingValidationMessage,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:JSONPath=`.status.NetworkmappingValidationStatus`,name=Status,type=string

// NetworkMapping is the Schema for the networkmappings API
type NetworkMapping struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NetworkMappingSpec   `json:"spec,omitempty"`
	Status NetworkMappingStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// NetworkMappingList contains a list of NetworkMapping
type NetworkMappingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NetworkMapping `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NetworkMapping{}, &NetworkMappingList{})
}
