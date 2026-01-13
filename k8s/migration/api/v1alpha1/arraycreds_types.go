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

// ArrayCredsInfo holds the actual storage array credentials after decoding from secret
type ArrayCredsInfo struct {
	// Hostname is the storage array IP address or hostname
	Hostname string
	// Username is the storage array username
	Username string
	// Password is the storage array password
	Password string
	// SkipSSLVerification is whether to skip SSL certificate verification
	SkipSSLVerification bool
}

// ArrayCredsSpec defines the desired state of ArrayCreds
type ArrayCredsSpec struct {
	// VendorType is the storage array vendor type (e.g., pure, ontap, hpalletra)
	VendorType string `json:"vendorType"`

	// SecretRef is the reference to the Kubernetes secret holding storage array credentials
	SecretRef corev1.ObjectReference `json:"secretRef,omitempty"`
}

// ArrayCredsStatus defines the observed state of ArrayCreds
type ArrayCredsStatus struct {
	// ArrayValidationStatus is the status of the storage array validation
	ArrayValidationStatus string `json:"arrayValidationStatus,omitempty"`
	// ArrayValidationMessage is the message associated with the storage array validation
	ArrayValidationMessage string `json:"arrayValidationMessage,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:JSONPath=`.spec.vendorType`,name=Vendor,type=string
// +kubebuilder:printcolumn:JSONPath=`.status.arrayValidationStatus`,name=Status,type=string

// ArrayCreds is the Schema for the storage array credentials API that defines authentication
// and connection details for storage arrays. It provides a secure way to store and validate
// storage array credentials for use in migration operations, supporting multiple vendors including
// Pure Storage, NetApp ONTAP, HPE Alletra, and others.
type ArrayCreds struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ArrayCredsSpec   `json:"spec,omitempty"`
	Status ArrayCredsStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ArrayCredsList contains a list of ArrayCreds
type ArrayCredsList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ArrayCreds `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ArrayCreds{}, &ArrayCredsList{})
}
