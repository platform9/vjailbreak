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

const (
	MigrationPhasePending                  MigrationPhase = "Pending"
	MigrationPhaseValidating               MigrationPhase = "Validating"
	MigrationPhaseAwaitingDataCopyStart    MigrationPhase = "AwaitingDataCopyStart"
	MigrationPhaseCopying                  MigrationPhase = "CopyingBlocks"
	MigrationPhaseCopyingChangedBlocks     MigrationPhase = "CopyingChangedBlocks"
	MigrationPhaseConvertingDisk           MigrationPhase = "ConvertingDisk"
	MigrationPhaseAwaitingCutOverStartTime MigrationPhase = "AwaitingCutOverStartTime"
	MigrationPhaseAwaitingAdminCutOver     MigrationPhase = "AwaitingAdminCutOver"
	MigrationPhaseSucceeded                MigrationPhase = "Succeeded"
	MigrationPhaseFailed                   MigrationPhase = "Failed"
	MigrationPhaseUnknown                  MigrationPhase = "Unknown"
)

// MigrationSpec defines the desired state of Migration
type MigrationSpec struct {
	// MigrationPlan is the name of the migration plan
	MigrationPlan string `json:"migrationPlan"`

	// PodRef is the name of the pod
	PodRef string `json:"podRef"`

	// VMName is the name of the VM getting migrated from VMWare to Openstack
	VMName string `json:"vmName"`

	// InitiateCutover is the flag to initiate cutover
	InitiateCutover bool `json:"initiateCutover"`
}

type MigrationPhase string
type MigrationConditionType string

// MigrationStatus defines the observed state of Migration
type MigrationStatus struct {
	// Phase is the current phase of the migration
	Phase MigrationPhase `json:"phase"`

	// Conditions is the list of conditions of the migration object pod
	Conditions []corev1.PodCondition `json:"conditions,omitempty"`

	// AgentName is the name of the agent where migration is running
	AgentName string `json:"agentName,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:JSONPath=`.status.phase`,name=Phase,type=string
// +kubebuilder:printcolumn:JSONPath=`.status.agentName`,name=AgentName,type=string

// Migration is the Schema for the migrations API
type Migration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of Migration
	Spec MigrationSpec `json:"spec,omitempty"`

	// Status defines the observed state of Migration
	Status MigrationStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// MigrationList contains a list of Migration
type MigrationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	// Items is the list of Migration objects
	Items []Migration `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Migration{}, &MigrationList{})
}
