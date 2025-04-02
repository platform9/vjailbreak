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

// MigrationTemplateSource defines the source details for the migrationtemplate
type MigrationTemplateSource struct {
	VMwareRef  string `json:"vmwareRef"`
	DataCenter string `json:"datacenter"`
}

// MigrationTemplateDestination defines the destination details for the migrationtemplate
type MigrationTemplateDestination struct {
	OpenstackRef string `json:"openstackRef"`
}

// MigrationTemplateSpec defines the desired state of MigrationTemplate
type MigrationTemplateSpec struct {
	// +kubebuilder:validation:Enum=windows;linux
	OSType          string                       `json:"osType,omitempty"`
	VirtioWinDriver string                       `json:"virtioWinDriver,omitempty"`
	NetworkMapping  string                       `json:"networkMapping"`
	StorageMapping  string                       `json:"storageMapping"`
	Source          MigrationTemplateSource      `json:"source"`
	Destination     MigrationTemplateDestination `json:"destination"`
}

// VMInfo contains information about a virtual machine
type VMInfo struct {
	Name       string   `json:"name"`
	Datastores []string `json:"datastores,omitempty"`
	Disks      []string `json:"disks,omitempty"`
	Networks   []string `json:"networks,omitempty"`
	IPAddress  string   `json:"ipAddress,omitempty"`
	VMState    string   `json:"vmState,omitempty"`
	OSType     string   `json:"osType,omitempty"`
}

// OpenstackInfo contains OpenStack-specific information
type OpenstackInfo struct {
	VolumeTypes []string `json:"volumeTypes,omitempty"`
	Networks    []string `json:"networks,omitempty"`
}

// MigrationTemplateStatus defines the observed state of MigrationTemplate
type MigrationTemplateStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	VMWare    []VMInfo      `json:"vmware,omitempty"`
	Openstack OpenstackInfo `json:"openstack,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// MigrationTemplate is the Schema for the migrationtemplates API
type MigrationTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MigrationTemplateSpec   `json:"spec,omitempty"`
	Status MigrationTemplateStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// MigrationTemplateList contains a list of MigrationTemplate
type MigrationTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MigrationTemplate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MigrationTemplate{}, &MigrationTemplateList{})
}
