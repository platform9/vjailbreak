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

// VMwareClusterPhase represents the phase of a VMwareCluster
type VMwareClusterPhase string

const (
	// VMwareClusterPending is the initial phase of a VMwareCluster
	VMwareClusterPending VMwareClusterPhase = "Pending"
	// VMwareClusterRunning is the phase of a VMwareCluster when it is running
	VMwareClusterRunning VMwareClusterPhase = "Running"
	// VMwareClusterFailed is the phase of a VMwareCluster when it fails
	VMwareClusterFailed VMwareClusterPhase = "Failed"
	// VMwareClusterCompleted is the final phase of a VMwareCluster
	VMwareClusterCompleted VMwareClusterPhase = "Completed"
)

// VMwareClusterSpec defines the desired state of VMwareCluster
type VMwareClusterSpec struct {
	Name  string   `json:"name,omitempty"`
	Hosts []string `json:"hosts,omitempty"`
}

// VMwareClusterStatus defines the observed state of VMwareCluster
type VMwareClusterStatus struct {
	Phase VMwareClusterPhase `json:"phase,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// VMwareCluster is the Schema for the vmwareclusters API
type VMwareCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VMwareClusterSpec   `json:"spec,omitempty"`
	Status VMwareClusterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// VMwareClusterList contains a list of VMwareCluster
type VMwareClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VMwareCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&VMwareCluster{}, &VMwareClusterList{})
}
