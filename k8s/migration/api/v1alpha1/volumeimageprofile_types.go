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

// VolumeImageProfileSpec defines the desired state of VolumeImageProfile
type VolumeImageProfileSpec struct {
	// OSFamily scopes this profile to a specific VMware guest family.
	// +kubebuilder:validation:Enum=windowsGuest;linuxGuest;any
	OSFamily string `json:"osFamily"`

	// Properties is the map of Cinder volume image metadata key-value pairs
	Properties map[string]string `json:"properties"`

	// Description will indicate the usage context of this profile
	// +optional
	Description string `json:"description,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:printcolumn:name="OSFamily",type="string",JSONPath=".spec.osFamily"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// VolumeImageProfile defines a reusable set of Cinder volume_image_metadata properties
// that can be applied to VM volumes during migration.
type VolumeImageProfile struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec VolumeImageProfileSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// VolumeImageProfileList contains a list of VolumeImageProfile
type VolumeImageProfileList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VolumeImageProfile `json:"items"`
}

func init() {
	SchemeBuilder.Register(&VolumeImageProfile{}, &VolumeImageProfileList{})
}
