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

// VMwareCredsSpec defines the desired state of VMwareCreds
type VMwareCredsSpec struct {
	// DataCenter is the datacenter for the virtual machine
	DataCenter string `json:"datacenter"`
	// SecretRef is the reference to the Kubernetes secret holding VMware credentials
	SecretRef corev1.ObjectReference `json:"secretRef,omitempty"`
}

// VMwareCredsStatus defines the observed state of VMwareCreds
type VMwareCredsStatus struct {
	// VMwareValidationStatus is the status of the VMware validation
	VMwareValidationStatus string `json:"vmwareValidationStatus,omitempty"`
	// VMwareValidationMessage is the message associated with the VMware validation
	VMwareValidationMessage string `json:"vmwareValidationMessage,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:JSONPath=`.status.vmwareValidationStatus`,name=Status,type=string

// VMwareCreds is the Schema for the vmwarecreds API
type VMwareCreds struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VMwareCredsSpec   `json:"spec,omitempty"`
	Status VMwareCredsStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// VMwareCredsList contains a list of VMwareCreds
type VMwareCredsList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VMwareCreds `json:"items"`
}

func init() {
	SchemeBuilder.Register(&VMwareCreds{}, &VMwareCredsList{})
}
