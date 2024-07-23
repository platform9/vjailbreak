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

// OpenstackCredsSpec defines the desired state of OpenstackCreds
type OpenstackCredsSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// OS_AUTH_URL is the OpenStack authentication URL
	OS_AUTH_URL string `json:"OS_AUTH_URL,omitempty"`

	// OS_DOMAIN_NAME is the OpenStack domain name
	OS_DOMAIN_NAME string `json:"OS_DOMAIN_NAME,omitempty"`

	// OS_USERNAME is the OpenStack username
	OS_USERNAME string `json:"OS_USERNAME,omitempty"`

	// OS_PASSWORD is the OpenStack password
	OS_PASSWORD string `json:"OS_PASSWORD,omitempty"`

	// OS_REGION_NAME is the OpenStack region name
	OS_REGION_NAME string `json:"OS_REGION_NAME,omitempty"`

	// OS_TENANT_NAME is the OpenStack tenant name
	OS_TENANT_NAME string `json:"OS_TENANT_NAME,omitempty"`
}

// OpenstackCredsStatus defines the observed state of OpenstackCreds
type OpenstackCredsStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	OpenStackValidationStatus  string `json:"openstackValidationStatus,omitempty"`
	OpenStackValidationMessage string `json:"openstackValidationMessage,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// OpenstackCreds is the Schema for the openstackcreds API
type OpenstackCreds struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpenstackCredsSpec   `json:"spec,omitempty"`
	Status OpenstackCredsStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// OpenstackCredsList contains a list of OpenstackCreds
type OpenstackCredsList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpenstackCreds `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OpenstackCreds{}, &OpenstackCredsList{})
}
