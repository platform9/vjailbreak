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

// ESXiSSHCredsInfo holds the actual ESXi SSH credentials after decoding from secret
type ESXiSSHCredsInfo struct {
	// Username is the SSH username (typically "root")
	Username string
	// PrivateKey is the SSH private key in PEM format
	PrivateKey []byte
}

// ESXiSSHCredsSpec defines the desired state of ESXiSSHCreds
type ESXiSSHCredsSpec struct {
	// SecretRef is the reference to the Kubernetes secret holding the SSH private key
	// The secret should contain a key named "privateKey" with the SSH private key in PEM format
	SecretRef corev1.ObjectReference `json:"secretRef"`

	// Username is the SSH username to use for connecting to ESXi hosts (default: "root")
	// +kubebuilder:default:="root"
	// +optional
	Username string `json:"username,omitempty"`

	// Hosts is an optional explicit list of ESXi host IPs or hostnames to validate
	// If not specified, the controller will automatically discover all ESXi hosts from all VMwareCreds in the system
	// +optional
	Hosts []string `json:"hosts,omitempty"`
}

// ESXiSSHCredsStatus defines the observed state of ESXiSSHCreds
type ESXiSSHCredsStatus struct {
	// ValidationStatus is the overall status of the ESXi SSH validation
	// Possible values: Pending, Validating, Succeeded, PartiallySucceeded, Failed
	ValidationStatus string `json:"validationStatus,omitempty"`

	// ValidationMessage is the message associated with the overall validation
	ValidationMessage string `json:"validationMessage,omitempty"`

	// TotalHosts is the total number of ESXi hosts to validate
	TotalHosts int `json:"totalHosts,omitempty"`

	// SuccessfulHosts is the number of ESXi hosts that passed validation
	SuccessfulHosts int `json:"successfulHosts,omitempty"`

	// FailedHosts is the number of ESXi hosts that failed validation
	FailedHosts int `json:"failedHosts,omitempty"`

	// LastValidationTime is the timestamp of the last validation run
	LastValidationTime metav1.Time `json:"lastValidationTime,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:JSONPath=`.status.validationStatus`,name=Status,type=string
// +kubebuilder:printcolumn:JSONPath=`.status.successfulHosts`,name=Successful,type=integer
// +kubebuilder:printcolumn:JSONPath=`.status.failedHosts`,name=Failed,type=integer
// +kubebuilder:printcolumn:JSONPath=`.status.totalHosts`,name=Total,type=integer

// ESXiSSHCreds is the Schema for the esxisshcreds API that defines SSH credentials
// for connecting to ESXi hosts. It validates SSH connectivity to ESXi hosts either
// discovered from vCenter via VMwareCreds or explicitly specified in the hosts list.
// The controller validates connections in parallel with throttling to avoid overwhelming
// the network, and reports per-host validation results in the status.
type ESXiSSHCreds struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ESXiSSHCredsSpec   `json:"spec,omitempty"`
	Status ESXiSSHCredsStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ESXiSSHCredsList contains a list of ESXiSSHCreds
type ESXiSSHCredsList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ESXiSSHCreds `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ESXiSSHCreds{}, &ESXiSSHCredsList{})
}
