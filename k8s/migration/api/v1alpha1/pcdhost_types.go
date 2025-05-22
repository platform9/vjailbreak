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

type PCDHostInterface struct {
	IPAddresses []string `json:"ipAddresses,omitempty"`
	MACAddress  string   `json:"macAddress,omitempty"`
	Name        string   `json:"name,omitempty"`
}

// PCDHostSpec defines the desired state of PCDHost
type PCDHostSpec struct {
	// HostName is the name of the host
	HostName string `json:"hostName,omitempty"`

	// HostID is the ID of the host
	HostID string `json:"hostID,omitempty"`
	// HostState is the state of the host
	HostState string `json:"hostState,omitempty"`
	// RolesAssigned is the list of roles assigned to the host
	RolesAssigned []string           `json:"rolesAssigned,omitempty"`
	OSFamily      string             `json:"osFamily,omitempty"`
	Arch          string             `json:"arch,omitempty"`
	OSInfo        string             `json:"osInfo,omitempty"`
	Interfaces    []PCDHostInterface `json:"interfaces,omitempty"`
}

// PCDHostStatus defines the observed state of PCDHost
type PCDHostStatus struct {
	Responding bool   `json:"responding,omitempty"`
	RoleStatus string `json:"roleStatus,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// PCDHost is the Schema for the pcdhosts API
type PCDHost struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PCDHostSpec   `json:"spec,omitempty"`
	Status PCDHostStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// PCDHostList contains a list of PCDHost
type PCDHostList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PCDHost `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PCDHost{}, &PCDHostList{})
}
