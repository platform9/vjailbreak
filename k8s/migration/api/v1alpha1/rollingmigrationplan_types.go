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

type RollingMigrationPlanPhase string

type VMSequenceInfo struct {
	// VMName is the name of the virtual machine to be migrated
	VMName string `json:"vmName"`
	// ESXiName is the name of the ESXi host where the virtual machine is located
	ESXiName string `json:"esxiName"`
}

type ClusterMigrationInfo struct {
	// ClusterName is the name of the vCenter cluster to be migrated
	ClusterName string `json:"clusterName"`
	// VMSequence is the sequence of virtual machines to be migrated
	VMSequence []VMSequenceInfo `json:"vmSequence"`
}

// RollingMigrationPlanSpec defines the desired state of RollingMigrationPlan
type RollingMigrationPlanSpec struct {
	// ClusterSequence is the sequence of vCenter clusters to be migrated
	ClusterSequence []ClusterMigrationInfo `json:"clusterSequence"`

	// VMwareCredsRef is the reference to the VMware credentials
	VMwareCredsRef corev1.LocalObjectReference `json:"vmwareCredsRef"`

	// OpenstackCredsRef is the reference to the OpenStack credentials
	OpenstackCredsRef corev1.LocalObjectReference `json:"openstackCredsRef"`
}

// RollingMigrationPlanStatus defines the observed state of RollingMigrationPlan
type RollingMigrationPlanStatus struct {
	// Phase is the current phase of the migration
	Phase RollingMigrationPlanPhase `json:"phase,omitempty"`
	// CurrentESXi is the name of the current ESXi host being migrated
	CurrentESXi string `json:"currentESXi,omitempty"`
	// CurrentCluster is the name of the current vCenter cluster being migrated
	CurrentCluster string `json:"currentCluster,omitempty"`
	// CurrentVM is the name of the current virtual machine being migrated
	CurrentVM string `json:"currentVM,omitempty"`
	// Message is the message associated with the current state of the migration
	Message string `json:"message,omitempty"`
	// MigratedVMs is the list of virtual machines that have been migrated
	MigratedVMs []string `json:"migratedVMs,omitempty"`
	// FailedVMs is the list of virtual machines that have failed to migrate
	FailedVMs []string `json:"failedVMs,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// RollingMigrationPlan is the Schema for the rollingmigrationplans API
type RollingMigrationPlan struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RollingMigrationPlanSpec   `json:"spec,omitempty"`
	Status RollingMigrationPlanStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// RollingMigrationPlanList contains a list of RollingMigrationPlan
type RollingMigrationPlanList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RollingMigrationPlan `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RollingMigrationPlan{}, &RollingMigrationPlanList{})
}
