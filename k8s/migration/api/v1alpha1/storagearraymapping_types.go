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

type StorageArrayMappingSpec struct {
	Datastores []DatastoreMapping `json:"datastores"`
	Arrays     []StorageArray     `json:"arrays"`
}

type DatastoreMapping struct {
	Name      string `json:"name"`
	ArrayName string `json:"arrayName"`
	// ArrayType is the vendor type of the storage array (example: "pure", "netapp")
	ArrayType string `json:"arrayType"`
}

type StorageArray struct {
	Name               string       `json:"name"`
	Type               string       `json:"type"`
	ManagementEndpoint string       `json:"managementEndpoint"`
	CredentialsSecret  string       `json:"credentialsSecret"`
	ISCSI              *ISCSIConfig `json:"iscsi,omitempty"`
}

type ISCSIConfig struct {
	Targets []string `json:"targets,omitempty"`
	Port    int      `json:"port,omitempty"`
}

type StorageArrayMappingStatus struct {
	ValidationStatus  string               `json:"validationStatus,omitempty"`
	ValidationMessage string               `json:"validationMessage,omitempty"`
	Arrays            []StorageArrayStatus `json:"arrays,omitempty"`
}

type StorageArrayStatus struct {
	Name        string      `json:"name"`
	Reachable   bool        `json:"reachable"`
	Version     string      `json:"version,omitempty"`
	LastChecked metav1.Time `json:"lastChecked,omitempty"`
	Error       string      `json:"error,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.validationStatus"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// It defines mappings between VMware datastores and backend storage arrays,
type StorageArrayMapping struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   StorageArrayMappingSpec   `json:"spec,omitempty"`
	Status StorageArrayMappingStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

type StorageArrayMappingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []StorageArrayMapping `json:"items"`
}

func init() {
	SchemeBuilder.Register(&StorageArrayMapping{}, &StorageArrayMappingList{})
}
