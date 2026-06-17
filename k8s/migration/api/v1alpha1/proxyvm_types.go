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

// ProxyVMSpec defines the desired state of ProxyVM
type ProxyVMSpec struct {
	VMName         string                      `json:"vmName"`
	VMwareCredsRef corev1.LocalObjectReference `json:"vmwareCredsRef"`
	// +optional
	SSHKeyPairRef *corev1.LocalObjectReference `json:"sshKeyPairRef,omitempty"`
}

// ProxyVMComponentCheck records whether a required component was found on the Proxy VM.
type ProxyVMComponentCheck struct {
	Name    string `json:"name"`
	Present bool   `json:"present"`
	// +optional
	Message string `json:"message,omitempty"`
}

// ProxyVMStatus defines the observed state of ProxyVM
type ProxyVMStatus struct {
	// +optional
	ValidationStatus string `json:"validationStatus,omitempty"`
	// +optional
	ValidationMessage string `json:"validationMessage,omitempty"`
	// +optional
	IPAddress string `json:"ipAddress,omitempty"`
	// +optional
	AttachedDiskCount int `json:"attachedDiskCount,omitempty"`
	// +optional
	ComponentsVerified []ProxyVMComponentCheck `json:"componentsVerified,omitempty"`
	// +optional
	LastValidationTime *metav1.Time `json:"lastValidationTime,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:JSONPath=`.spec.vmName`,name=VM-Name,type=string
// +kubebuilder:printcolumn:JSONPath=`.status.validationStatus`,name=Status,type=string
// +kubebuilder:printcolumn:JSONPath=`.status.ipAddress`,name=IP,type=string
// +kubebuilder:printcolumn:JSONPath=`.status.attachedDiskCount`,name=Attached-Disks,type=integer

// ProxyVM is the Schema for the proxyvms API.
type ProxyVM struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ProxyVMSpec   `json:"spec,omitempty"`
	Status ProxyVMStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ProxyVMList contains a list of ProxyVM
type ProxyVMList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ProxyVM `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ProxyVM{}, &ProxyVMList{})
}
