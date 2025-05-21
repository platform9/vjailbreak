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

type ESXIMigrationPhase string

const (
	ESXIMigrationPhaseWaiting                ESXIMigrationPhase = "Waiting"
	ESXIMigrationPhaseRunning                ESXIMigrationPhase = "Running"
	ESXIMigrationPhaseCordoned               ESXIMigrationPhase = "Cordoned"
	ESXIMigrationPhaseInMaintenanceMode      ESXIMigrationPhase = "InMaintenanceMode"
	ESXIMigrationPhaseWaitingForVMsToBeMoved ESXIMigrationPhase = "WaitingForVMsToBeMoved"
	ESXIMigrationPhaseConvertingToPCDHost    ESXIMigrationPhase = "ConvertingToPCDHost"
	ESXIMigrationPhaseAssigningRole          ESXIMigrationPhase = "AssigningRole"
	ESXIMigrationPhaseFailed                 ESXIMigrationPhase = "Failed"
	ESXIMigrationPhaseSucceeded              ESXIMigrationPhase = "Succeeded"
	ESXIMigrationPhasePaused                 ESXIMigrationPhase = "Paused"
)

// ESXIMigrationSpec defines the desired state of ESXIMigration
type ESXIMigrationSpec struct {
	// ESXiName is the name of the ESXi host to be migrated
	ESXiName string `json:"esxiName"`
	// OpenstackCredsRef is the reference to the OpenStack credentials
	OpenstackCredsRef corev1.LocalObjectReference `json:"openstackCredsRef"`
	// VMwareCredsRef is the reference to the VMware credentials
	VMwareCredsRef corev1.LocalObjectReference `json:"vmwareCredsRef"`
	// RollingMigrationPlanRef is the reference to the RollingMigrationPlan
	RollingMigrationPlanRef corev1.LocalObjectReference `json:"rollingMigrationPlanRef"`
}

// ESXIMigrationStatus defines the observed state of ESXIMigration
type ESXIMigrationStatus struct {
	// VMs is the list of VMs present on the ESXi host
	VMs []string `json:"vms,omitempty"`
	// Phase is the current phase of the migration
	Phase ESXIMigrationPhase `json:"phase,omitempty"`
	// Message is the message associated with the current state of the migration
	Message string `json:"message,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// ESXIMigration is the Schema for the esximigrations API
type ESXIMigration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ESXIMigrationSpec   `json:"spec,omitempty"`
	Status ESXIMigrationStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ESXIMigrationList contains a list of ESXIMigration
type ESXIMigrationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ESXIMigration `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ESXIMigration{}, &ESXIMigrationList{})
}
