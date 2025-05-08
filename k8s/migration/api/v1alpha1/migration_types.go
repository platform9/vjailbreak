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

// MigrationPhase represents the current phase of the VM migration process
// +kubebuilder:validation:Enum=Pending;Validating;AwaitingDataCopyStart;CopyingBlocks;CopyingChangedBlocks;ConvertingDisk;AwaitingCutOverStartTime;AwaitingAdminCutOver;Succeeded;Failed;Unknown
type VMMigrationPhase string

// MigrationConditionType represents the type of condition for a migration
type MigrationConditionType string

const (
	// VMMigrationPhasePending indicates the migration is waiting to start
	VMMigrationPhasePending VMMigrationPhase = "Pending"
	// VMMigrationPhaseValidating indicates the migration prerequisites are being validated
	VMMigrationPhaseValidating VMMigrationPhase = "Validating"
	// VMMigrationPhaseAwaitingDataCopyStart indicates the migration is waiting to begin data copy
	VMMigrationPhaseAwaitingDataCopyStart VMMigrationPhase = "AwaitingDataCopyStart"
	// VMMigrationPhaseCopying indicates initial block copying is in progress
	VMMigrationPhaseCopying VMMigrationPhase = "CopyingBlocks"
	// VMMigrationPhaseCopyingChangedBlocks indicates copying of changed blocks is in progress
	VMMigrationPhaseCopyingChangedBlocks VMMigrationPhase = "CopyingChangedBlocks"
	// VMMigrationPhaseConvertingDisk indicates disk format conversion is in progress
	VMMigrationPhaseConvertingDisk VMMigrationPhase = "ConvertingDisk"
	// VMMigrationPhaseAwaitingCutOverStartTime indicates waiting for scheduled cutover time
	VMMigrationPhaseAwaitingCutOverStartTime VMMigrationPhase = "AwaitingCutOverStartTime"
	// VMMigrationPhaseAwaitingAdminCutOver indicates waiting for admin to initiate cutover
	VMMigrationPhaseAwaitingAdminCutOver VMMigrationPhase = "AwaitingAdminCutOver"
	// VMMigrationPhaseSucceeded indicates the migration completed successfully
	VMMigrationPhaseSucceeded VMMigrationPhase = "Succeeded"
	// VMMigrationPhaseFailed indicates the migration has failed
	VMMigrationPhaseFailed VMMigrationPhase = "Failed"
	// VMMigrationPhaseUnknown indicates the migration state is unknown
	VMMigrationPhaseUnknown VMMigrationPhase = "Unknown"
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

// MigrationStatus defines the observed state of Migration
type MigrationStatus struct {
	// Phase is the current phase of the migration
	Phase VMMigrationPhase `json:"phase"`

	// Conditions is the list of conditions of the migration object pod
	Conditions []corev1.PodCondition `json:"conditions,omitempty"`

	// AgentName is the name of the agent where migration is running
	AgentName string `json:"agentName,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Agent Name",type="string",JSONPath=".status.agentName"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

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
