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

// PCDClusterSpec defines the desired state of PCDCluster
type PCDClusterSpec struct {
	// ClusterName is the name of the PCD cluster
	ClusterName string `json:"clusterName,omitempty"`
	// Hosts is the list of hosts in the PCD cluster
	Hosts []string `json:"hosts,omitempty"`
}

// PCDClusterStatus defines the observed state of PCDCluster
type PCDClusterStatus struct {
	// ClusterID is the ID of the PCD cluster
	ClusterID string `json:"clusterID,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// PCDCluster is the Schema for the pcdclusters API
type PCDCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PCDClusterSpec   `json:"spec,omitempty"`
	Status PCDClusterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// PCDClusterList contains a list of PCDCluster
type PCDClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PCDCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PCDCluster{}, &PCDClusterList{})
}
