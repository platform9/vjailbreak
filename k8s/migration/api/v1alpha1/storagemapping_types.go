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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// StorageMappingSpec defines the desired state of StorageMapping
type StorageMappingSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	Storages []Storage `json:"storages"`
}

type Storage struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

// StorageMappingStatus defines the observed state of StorageMapping
type StorageMappingStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	StoragemappingValidationStatus  string `json:"storageMappingValidationStatus,omitempty"`
	StoragemappingValidationMessage string `json:"storageMappingValidationMessage,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// StorageMapping is the Schema for the storagemappings API
type StorageMapping struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   StorageMappingSpec   `json:"spec,omitempty"`
	Status StorageMappingStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// StorageMappingList contains a list of StorageMapping
type StorageMappingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []StorageMapping `json:"items"`
}

func init() {
	SchemeBuilder.Register(&StorageMapping{}, &StorageMappingList{})
}
