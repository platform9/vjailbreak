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
	// VMName is the display name of the Proxy VM in vCenter.
	VMName string `json:"vmName"`

	// VMwareCredsRef references the VMwareCreds used to locate and connect to the Proxy VM.
	VMwareCredsRef corev1.LocalObjectReference `json:"vmwareCredsRef"`
}

// ProxyVMComponentCheck records whether a required component was found on the Proxy VM.
type ProxyVMComponentCheck struct {
	// Name is the component name (e.g. "lsblk", "qemu-nbd", "nbdkit", "sshd").
	Name string `json:"name"`
	// Present indicates whether the component was found in PATH.
	Present bool `json:"present"`
	// Message provides detail for missing components.
	// +optional
	Message string `json:"message,omitempty"`
}

// ProxyVMStatus defines the observed state of ProxyVM
type ProxyVMStatus struct {
	// ValidationStatus is one of: Pending, Verifying, Ready, VerificationFailed.
	// +optional
	ValidationStatus string `json:"validationStatus,omitempty"`

	// ValidationMessage contains a human-readable summary of the last validation result.
	// +optional
	ValidationMessage string `json:"validationMessage,omitempty"`

	// IPAddress is the IP address discovered from vCenter guest info.
	// +optional
	IPAddress string `json:"ipAddress,omitempty"`

	// AttachedDiskCount is the number of source-snapshot disks currently attached
	// to this Proxy VM across all active Hot-Add migrations. Max 60.
	// +optional
	AttachedDiskCount int `json:"attachedDiskCount,omitempty"`

	// ComponentsVerified lists each required component and whether it was found.
	// +optional
	ComponentsVerified []ProxyVMComponentCheck `json:"componentsVerified,omitempty"`

	// LastValidationTime records when the last verification completed.
	// +optional
	LastValidationTime *metav1.Time `json:"lastValidationTime,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:JSONPath=`.spec.vmName`,name=VM-Name,type=string
// +kubebuilder:printcolumn:JSONPath=`.status.validationStatus`,name=Status,type=string
// +kubebuilder:printcolumn:JSONPath=`.status.ipAddress`,name=IP,type=string
// +kubebuilder:printcolumn:JSONPath=`.status.attachedDiskCount`,name=Attached-Disks,type=integer

// ProxyVM is the Schema for a registered Proxy VM used in Hot-Add data copy migrations.
// The controller validates SSH connectivity and required components on the referenced VM.
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
