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
	"k8s.io/apimachinery/pkg/runtime"
)

// MigrationBucketMapping is a single source -> target mapping (network or storage).
type MigrationBucketMapping struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

// MigrationBucketConfig is the migration configuration carried by a bucket. It mirrors the
// inputs of a MigrationPlan so that, at trigger time, a bucket can be compiled into the
// existing migration objects without translation.
type MigrationBucketConfig struct {
	// SourceCluster is the VMware source cluster (derived from the bucket's VMs).
	// +optional
	SourceCluster string `json:"sourceCluster,omitempty"`

	// PCDCluster is the destination PCD cluster.
	// +optional
	PCDCluster string `json:"pcdCluster,omitempty"`

	// NetworkMappings maps source networks to destination networks.
	// +optional
	NetworkMappings []MigrationBucketMapping `json:"networkMappings,omitempty"`

	// StorageMappings maps source datastores to destination volume types.
	// +optional
	StorageMappings []MigrationBucketMapping `json:"storageMappings,omitempty"`

	// SecurityGroups is the list of OpenStack security groups to apply.
	// +optional
	SecurityGroups []string `json:"securityGroups,omitempty"`

	// ServerGroup is the OpenStack server group to place VMs in.
	// +optional
	ServerGroup string `json:"serverGroup,omitempty"`

	// DataCopyMethod is the migration data-copy method (e.g. cold/hot).
	// +optional
	DataCopyMethod string `json:"dataCopyMethod,omitempty"`

	// FormValues holds the full Migration Form inputs so the bucket editor can round-trip
	// the exact configuration. Opaque to the controller.
	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	FormValues *runtime.RawExtension `json:"formValues,omitempty"`

	// SelectedOptions records which optional migration-options checkboxes were enabled.
	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	SelectedOptions *runtime.RawExtension `json:"selectedOptions,omitempty"`
}

// MigrationBucketSpec defines the desired state of a MigrationBucket: a named, persistent
// group of VMs for one VMware credential, carrying a migration configuration and an optional
// schedule. At trigger time, buckets compile into the existing MigrationPlan/RollingMigrationPlan.
type MigrationBucketSpec struct {
	// VMwareCredsRef references the source VMware credential this bucket belongs to.
	VMwareCredsRef corev1.LocalObjectReference `json:"vmwareCredsRef"`

	// VMs is the list of member VM names. Must be non-empty.
	VMs []string `json:"vms"`

	// IsDefault marks the auto-created default bucket (which cannot be deleted).
	// +optional
	IsDefault bool `json:"isDefault,omitempty"`

	// Schedule is an optional future time at which the bucket's migration should start.
	// +optional
	Schedule *metav1.Time `json:"schedule,omitempty"`

	// Config is the embedded migration configuration.
	// +optional
	Config MigrationBucketConfig `json:"config,omitempty"`
}

// MigrationBucketPhase represents the lifecycle status of a bucket.
type MigrationBucketPhase string

const (
	// MigrationBucketPhaseNotMigrated indicates the bucket has not been migrated.
	MigrationBucketPhaseNotMigrated MigrationBucketPhase = "NotMigrated"
	// MigrationBucketPhaseScheduled indicates the bucket is scheduled for migration.
	MigrationBucketPhaseScheduled MigrationBucketPhase = "Scheduled"
	// MigrationBucketPhaseInProgress indicates the bucket's migration is running.
	MigrationBucketPhaseInProgress MigrationBucketPhase = "InProgress"
	// MigrationBucketPhaseMigrated indicates all of the bucket's VMs have migrated.
	MigrationBucketPhaseMigrated MigrationBucketPhase = "Migrated"
)

// MigrationBucketStatus defines the observed state of a MigrationBucket.
type MigrationBucketStatus struct {
	// Phase is the derived lifecycle status of the bucket.
	// +optional
	Phase MigrationBucketPhase `json:"phase,omitempty"`

	// Message provides human-readable detail (e.g. an invariant violation).
	// +optional
	Message string `json:"message,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:JSONPath=`.spec.isDefault`,name=Default,type=boolean
// +kubebuilder:printcolumn:JSONPath=`.status.phase`,name=Phase,type=string

// MigrationBucket is the Schema for the migrationbuckets API. It represents a persistent,
// named group of VMs (for one VMware credential) plus a migration configuration, used by the
// Inventory / Migration Planner to organize and trigger migrations.
type MigrationBucket struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MigrationBucketSpec   `json:"spec,omitempty"`
	Status MigrationBucketStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// MigrationBucketList contains a list of MigrationBucket
type MigrationBucketList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MigrationBucket `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MigrationBucket{}, &MigrationBucketList{})
}
