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

// VMwareCredsSpec defines the desired state of VMwareCreds
type VMwareCredsSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	VCENTER_HOST     string `json:"VCENTER_HOST"`
	VCENTER_INSECURE bool   `json:"VCENTER_INSECURE"`
	VCENTER_PASSWORD string `json:"VCENTER_PASSWORD"`
	VCENTER_USERNAME string `json:"VCENTER_USERNAME"`
}

// VMwareCredsStatus defines the observed state of VMwareCreds
type VMwareCredsStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	VMwareValidationStatus  string `json:"VMwareValidationStatus,omitempty"`
	VMwareValidationMessage string `json:"VMwareValidationMessage,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

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
