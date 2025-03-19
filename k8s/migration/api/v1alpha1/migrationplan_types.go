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

type MigrationPlanStrategy struct {
	// +kubebuilder:validation:Enum=hot;cold
	Type string `json:"type"`
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:Format:=date-time
	DataCopyStart metav1.Time `json:"dataCopyStart,omitempty"`
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:Format:=date-time
	VMCutoverStart metav1.Time `json:"vmCutoverStart,omitempty"`
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:Format:=date-time
	VMCutoverEnd metav1.Time `json:"vmCutoverEnd,omitempty"`
	// +kubebuilder:default:=false
	AdminInitiatedCutOver bool `json:"adminInitiatedCutOver,omitempty"`
	// +kubebuilder:default:=false
	PerformHealthChecks bool `json:"performHealthChecks,omitempty"`
	// +kubebuilder:default:="443"
	HealthCheckPort string `json:"healthCheckPort,omitempty"`
}

type AdvancedOptions struct {
	GranularVolumeTypes []string `json:"granularVolumeTypes,omitempty"`
	GranularNetworks    []string `json:"granularNetworks,omitempty"`
	GranularPorts       []string `json:"granularPorts,omitempty"`
}

// MigrationPlanSpec defines the desired state of MigrationPlan
type MigrationPlanSpec struct {
	MigrationTemplate string                `json:"migrationTemplate"`
	MigrationStrategy MigrationPlanStrategy `json:"migrationStrategy"`
	Retry             bool                  `json:"retry,omitempty"`
	VirtualMachines   [][]string            `json:"virtualMachines"`
	AdvancedOptions   AdvancedOptions       `json:"advancedOptions,omitempty"`
	// +kubebuilder:default:="echo \"Add your startup script here!\""
	FirstBootScript string `json:"firstBootScript,omitempty"`
}

// MigrationPlanStatus defines the observed state of MigrationPlan
type MigrationPlanStatus struct {
	MigrationStatus  string `json:"migrationStatus"`
	MigrationMessage string `json:"migrationMessage"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:JSONPath=`.status.migrationStatus`,name=Status,type=string

// MigrationPlan is the Schema for the migrationplans API
type MigrationPlan struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MigrationPlanSpec   `json:"spec,omitempty"`
	Status MigrationPlanStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// MigrationPlanList contains a list of MigrationPlan
type MigrationPlanList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MigrationPlan `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MigrationPlan{}, &MigrationPlanList{})
}
