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

// MigrationSource defines the source details for the migration
type MigrationSource struct {
	VMwareRef       string   `json:"vmwareref"`
	DataCenter      string   `json:"datacenter"`
	VirtualMachines []string `json:"virtualmachines"`
	// +kubebuilder:validation:Enum=windows;linux
	OSType          string `json:"ostype"`
	VirtioWinDriver string `json:"virtiowindriver,omitempty"`
}

// MigrationDestination defines the destination details for the migration
type MigrationDestination struct {
	OpenstackRef string `json:"openstackref"`
}

// MigrationSpec defines the desired state of Migration
type MigrationSpec struct {
	NetworkMapping string               `json:"networkmapping"`
	StorageMapping string               `json:"storagemapping"`
	Source         MigrationSource      `json:"source"`
	Destination    MigrationDestination `json:"destination"`
}

type VMMigrationStatus struct {
	VMName string `json:"vmname"`
	Status string `json:"status"`
}

// MigrationStatus defines the observed state of Migration
type MigrationStatus struct {
	Active            []corev1.ObjectReference `json:"active,omitempty"`
	VMMigrationStatus []VMMigrationStatus      `json:"vmmigrationstatus,omitempty"`
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
