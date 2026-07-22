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

// MigrationBlueprintSpec defines a reusable, named snapshot of a migration
// configuration. It captures everything from the migration form except the VM
// selection, so the UI can present it as a "Migration Template" and pre-fill
// the migration wizard. Blueprints are consumed only by the UI: applying one
// still creates the regular MigrationTemplate/MigrationPlan/mapping resources,
// so the migration controller never reads a blueprint.
type MigrationBlueprintSpec struct {
	// DisplayName is the human-readable name of the blueprint shown in the UI
	// +kubebuilder:validation:MinLength=1
	DisplayName string `json:"displayName"`
	// Description is a free-text description of the blueprint
	Description string `json:"description,omitempty"`

	// VMwareRef is the name of the VMwareCreds to use as the source environment.
	// Optional so partially filled forms can be saved as templates.
	VMwareRef string `json:"vmwareRef,omitempty"`
	// PCDRef is the name of the OpenstackCreds to use as the destination PCD environment.
	// Optional so partially filled forms can be saved as templates.
	PCDRef string `json:"pcdRef,omitempty"`
	// VMwareClusterName is the name of the source vCenter cluster VMs are
	// migrated from.
	VMwareClusterName string `json:"vmwareClusterName,omitempty"`
	// NoVMwareClusterFilter records that the user explicitly chose the
	// "No Cluster" source filter: VMs on standalone ESXi hosts that are not
	// part of any vCenter cluster in the datacenter referenced by VMwareRef.
	// This is distinct from VMwareClusterName simply not being set yet, so it
	// is tracked as its own field rather than inferred from an empty string.
	NoVMwareClusterFilter bool `json:"noVMwareClusterFilter,omitempty"`
	// TargetPCDClusterName is the name of the PCD cluster to migrate VMs into
	TargetPCDClusterName string `json:"targetPCDClusterName,omitempty"`

	// NetworkMappings is an inline snapshot of source-to-target network pairs.
	// Copied by value rather than referencing a NetworkMapping resource, since
	// those are per-migration objects that may be mutated or deleted.
	NetworkMappings []Network `json:"networkMappings,omitempty"`
	// StorageMappings is an inline snapshot of source-to-target storage pairs.
	// Copied by value for the same reason as NetworkMappings.
	StorageMappings []Storage `json:"storageMappings,omitempty"`
	// ArrayCredsMappings is an inline snapshot of datastore-to-ArrayCreds pairs.
	// Used when StorageCopyMethod is "StorageAcceleratedCopy".
	ArrayCredsMappings []DatastoreArrayCredsMapping `json:"arrayCredsMappings,omitempty"`
	// ProxyVMRef references the ProxyVM to use for data copy.
	// Used when StorageCopyMethod is "HotAdd".
	ProxyVMRef *corev1.LocalObjectReference `json:"proxyVMRef,omitempty"`

	// MigrationStrategy captures the migration type, cutover windows, and
	// health-check settings to pre-fill into the migration form.
	// Optional so partially filled forms can be saved as templates.
	MigrationStrategy *MigrationPlanStrategy `json:"migrationStrategy,omitempty"`
	// AdvancedOptions captures the advanced migration options to pre-fill
	AdvancedOptions AdvancedOptions `json:"advancedOptions,omitempty"`
	// PostMigrationAction captures the post-migration actions to pre-fill
	PostMigrationAction *PostMigrationAction `json:"postMigrationAction,omitempty"`
	// FirstBootScript is the script to run on first boot of migrated VMs
	FirstBootScript string `json:"firstBootScript,omitempty"`
	// SecurityGroups is the list of OpenStack security group names to apply
	SecurityGroups []string `json:"securityGroups,omitempty"`
	// ServerGroup is the OpenStack server group to place migrated VMs into
	ServerGroup string `json:"serverGroup,omitempty"`
	// FallbackToDHCP falls back to DHCP when static IP assignment is not possible
	FallbackToDHCP bool `json:"fallbackToDHCP,omitempty"`
	// PreserveSourceTags copies each source VM's vSphere tags and custom
	// attributes to the migrated VM as instance metadata
	PreserveSourceTags bool `json:"preserveSourceTags,omitempty"`
	// CustomMetadata is a map of additional key-value pairs applied as instance
	// metadata to every migrated VM
	CustomMetadata map[string]string `json:"customMetadata,omitempty"`
	// UseGPUFlavor indicates if the migration should filter and use GPU-enabled flavors
	UseGPUFlavor bool `json:"useGPUFlavor,omitempty"`
	// StorageCopyMethod indicates the method to use for storage migration
	// +kubebuilder:validation:Enum=normal;StorageAcceleratedCopy;HotAdd
	// +kubebuilder:default:=normal
	StorageCopyMethod string `json:"storageCopyMethod,omitempty"`
	// OSFamily is the OS type of the virtual machines this blueprint targets
	// +kubebuilder:validation:Enum=windowsGuest;linuxGuest
	OSFamily string `json:"osFamily,omitempty"`
	// VirtioWinDriver is the virtio-win driver version to use for Windows guests
	VirtioWinDriver string `json:"virtioWinDriver,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:printcolumn:name="Display Name",type="string",JSONPath=".spec.displayName"
// +kubebuilder:printcolumn:name="Source",type="string",JSONPath=".spec.vmwareRef"
// +kubebuilder:printcolumn:name="Destination",type="string",JSONPath=".spec.pcdRef"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// MigrationBlueprint is the Schema for the migrationblueprints API. It stores a
// reusable, named migration configuration that the UI presents as a "Migration
// Template": users save a configuration once and apply it later to pre-populate
// the migration form. Blueprints are not read by the migration controller.
type MigrationBlueprint struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec MigrationBlueprintSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// MigrationBlueprintList contains a list of MigrationBlueprint
type MigrationBlueprintList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MigrationBlueprint `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MigrationBlueprint{}, &MigrationBlueprintList{})
}
