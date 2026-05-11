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

// HostStatus contains the observed state of a single ESXi host within a VMware cluster,
// populated by the VMwareCluster controller by polling vCenter.
type HostStatus struct {
	// Name is the hostname of the ESXi host
	Name string `json:"name"`
	// VMCount is the number of VMs currently running on this host
	VMCount int `json:"vmCount"`
	// InMaintenance indicates whether the host is currently in maintenance mode
	InMaintenance bool `json:"inMaintenance"`
	// MaintenanceState is the vSphere maintenance state string (e.g. "inMaintenance", "enteringMaintenance")
	// +optional
	MaintenanceState string `json:"maintenanceState,omitempty"`
}

// VMwareClusterPhase represents the lifecycle phase of a VMware cluster during the migration process,
// tracking its progression from initial discovery through running state to completion or failure.
// This status tracking enables monitoring of the overall migration progress at the cluster level.
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
	// Name is the name of the VMware cluster
	Name string `json:"name,omitempty"`
	// Hosts is the list of hosts in the VMware cluster
	Hosts []string `json:"hosts,omitempty"`
	// VMwareCredsRef is a reference to the VMware credentials used to authenticate to vCenter.
	// Required for the VMwareCluster controller to poll per-host VM counts.
	VMwareCredsRef corev1.LocalObjectReference `json:"vmwareCredsRef,omitempty"`
	// BMConfigRef is an optional cluster-level reference to a BMConfig, used as the default
	// bare-metal provider config when converting hosts to PCD hosts via this cluster.
	// +optional
	BMConfigRef *corev1.LocalObjectReference `json:"bmConfigRef,omitempty"`
	// PCDClusterRef is an optional cluster-level reference to a PCDCluster, used as the default
	// destination when converting hosts to PCD hosts via this cluster.
	// +optional
	PCDClusterRef *corev1.LocalObjectReference `json:"pcdClusterRef,omitempty"`
}

// VMwareClusterStatus defines the observed state of VMwareCluster
type VMwareClusterStatus struct {
	// Phase is the current phase of the VMwareCluster
	Phase VMwareClusterPhase `json:"phase,omitempty"`
	// HostStatuses contains the per-host observed state within this cluster,
	// refreshed by the VMwareCluster controller on each reconciliation cycle.
	// +optional
	HostStatuses []HostStatus `json:"hostStatuses,omitempty"`
	// Conditions contains standard Kubernetes status conditions for the VMwareCluster.
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// LastPollError records the most recent error encountered while polling vCenter.
	// Set to empty string on successful poll. Allows distinguishing stale status from failures
	// without overwriting the last-known HostStatuses (see EC-001 in spec).
	// +optional
	LastPollError string `json:"lastPollError,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// VMwareCluster is the Schema for the vmwareclusters API that represents a VMware vSphere cluster
// in the source environment. It tracks cluster configuration, member hosts, and migration status
// as part of the VMware to Platform9 Distributed Cloud migration process. VMwareCluster resources
// serve as source components that are migrated to corresponding PCDCluster resources in the target environment.
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
