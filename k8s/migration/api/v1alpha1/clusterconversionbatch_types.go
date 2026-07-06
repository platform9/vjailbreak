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

// AutoStartMode controls when eligible hosts begin conversion.
type AutoStartMode string

const (
	// AutoStartModeAuto starts conversion automatically once a host is eligible.
	AutoStartModeAuto AutoStartMode = "Auto"
	// AutoStartModeManual requires an explicit per-host trigger annotation.
	AutoStartModeManual AutoStartMode = "Manual"
)

// ClusterConversionBatchPhase is the aggregate phase of a ClusterConversionBatch.
type ClusterConversionBatchPhase string

const (
	// ClusterConversionBatchPhasePending means no hosts have started converting yet.
	ClusterConversionBatchPhasePending ClusterConversionBatchPhase = "Pending"
	// ClusterConversionBatchPhaseRunning means at least one host is Converting.
	ClusterConversionBatchPhaseRunning ClusterConversionBatchPhase = "Running"
	// ClusterConversionBatchPhaseSucceeded means all hosts succeeded.
	ClusterConversionBatchPhaseSucceeded ClusterConversionBatchPhase = "Succeeded"
	// ClusterConversionBatchPhasePartialFail means some hosts succeeded and some did not.
	ClusterConversionBatchPhasePartialFail ClusterConversionBatchPhase = "PartialFail"
	// ClusterConversionBatchPhaseFailed means no hosts succeeded and all are terminal.
	ClusterConversionBatchPhaseFailed ClusterConversionBatchPhase = "Failed"
)

// HostConversionPhase is the per-host lifecycle phase within a ClusterConversionBatch.
type HostConversionPhase string

const (
	// HostConversionPhaseCheckingEligibility means the controller is evaluating eligibility.
	HostConversionPhaseCheckingEligibility HostConversionPhase = "CheckingEligibility"
	// HostConversionPhaseNotReady means the host did not pass eligibility checks.
	HostConversionPhaseNotReady HostConversionPhase = "NotReady"
	// HostConversionPhaseReady means the host passed all eligibility checks.
	HostConversionPhaseReady HostConversionPhase = "Ready"
	// HostConversionPhaseConverting means an ESXIMigration is running for this host.
	HostConversionPhaseConverting HostConversionPhase = "Converting"
	// HostConversionPhaseSucceeded means the ESXIMigration completed successfully.
	HostConversionPhaseSucceeded HostConversionPhase = "Succeeded"
	// HostConversionPhaseFailed means the ESXIMigration failed; retries may still occur.
	HostConversionPhaseFailed HostConversionPhase = "Failed"
	// HostConversionPhaseNeedsAttention means retries are exhausted; operator must act.
	HostConversionPhaseNeedsAttention HostConversionPhase = "NeedsAttention"
	// HostConversionPhaseSkipped means the operator explicitly skipped this host.
	HostConversionPhaseSkipped HostConversionPhase = "Skipped"
)

// EligibilityStatus is the result of a per-host eligibility check.
type EligibilityStatus string

const (
	// EligibilityStatusReady means all eligibility criteria passed.
	EligibilityStatusReady EligibilityStatus = "Ready"
	// EligibilityStatusNotReady means at least one criterion failed.
	EligibilityStatusNotReady EligibilityStatus = "NotReady"
	// EligibilityStatusUnknown means eligibility could not be determined (transient error).
	EligibilityStatusUnknown EligibilityStatus = "Unknown"
)

// HostEntry identifies a single ESXi host included in the batch.
type HostEntry struct {
	// ESXiName is the display name of the ESXi host in vCenter.
	// +kubebuilder:validation:MinLength=1
	ESXiName string `json:"esxiName"`
}

// HostConversionStatus tracks per-host state within a ClusterConversionBatch.
type HostConversionStatus struct {
	// ESXiName matches the corresponding HostEntry.esxiName.
	ESXiName string `json:"esxiName"`

	// Phase is the current lifecycle phase for this host.
	Phase HostConversionPhase `json:"phase"`

	// EligibilityStatus is the result of the most recent eligibility check.
	// +optional
	EligibilityStatus EligibilityStatus `json:"eligibilityStatus,omitempty"`

	// EligibilityReason explains why EligibilityStatus is NotReady or Unknown.
	// +optional
	EligibilityReason string `json:"eligibilityReason,omitempty"`

	// RetryCount is how many ESXIMigrations have been created for this host (excluding the first).
	// +optional
	RetryCount int `json:"retryCount,omitempty"`

	// NextRetryAt is when the controller will create the next ESXIMigration after a failure.
	// +optional
	NextRetryAt *metav1.Time `json:"nextRetryAt,omitempty"`

	// ESXIMigrationRef references the currently active ESXIMigration for this host.
	// +optional
	ESXIMigrationRef *corev1.LocalObjectReference `json:"esxiMigrationRef,omitempty"`

	// Message contains a human-readable status message.
	// +optional
	Message string `json:"message,omitempty"`

	// StartedAt is when conversion began (ESXIMigration created).
	// +optional
	StartedAt *metav1.Time `json:"startedAt,omitempty"`

	// CompletedAt is when the host reached Succeeded.
	// +optional
	CompletedAt *metav1.Time `json:"completedAt,omitempty"`

	// SkippedAt is when the operator skipped this host.
	// +optional
	SkippedAt *metav1.Time `json:"skippedAt,omitempty"`
}

// ClusterConversionBatchSpec defines the desired state of ClusterConversionBatch.
type ClusterConversionBatchSpec struct {
	// VMwareClusterName is the vCenter cluster these hosts belong to.
	// +kubebuilder:validation:MinLength=1
	VMwareClusterName string `json:"vmwareClusterName"`

	// VMwareCredsRef references the VMwareCreds for this batch.
	VMwareCredsRef corev1.LocalObjectReference `json:"vmwareCredsRef"`

	// OpenstackCredsRef references the OpenStack credentials (PCD type) for this batch.
	OpenstackCredsRef corev1.LocalObjectReference `json:"openstackCredsRef"`

	// BMConfigRef references the BMConfig for bare-metal provisioning.
	BMConfigRef corev1.LocalObjectReference `json:"bmConfigRef"`

	// CloudInitConfigRef is an optional reference to a Secret with cloud-init configuration.
	// +optional
	CloudInitConfigRef *corev1.SecretReference `json:"cloudInitConfigRef,omitempty"`

	// Hosts lists the ESXi hosts to convert.
	// +kubebuilder:validation:MinItems=1
	Hosts []HostEntry `json:"hosts"`

	// AutoStart controls whether eligible hosts start converting automatically.
	// +kubebuilder:default=Auto
	AutoStart AutoStartMode `json:"autoStart,omitempty"`

	// MaxRetries is the maximum number of retry attempts per host before NeedsAttention.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default=3
	// +optional
	MaxRetries int `json:"maxRetries,omitempty"`

	// RetryBackoffSeconds is the base backoff duration in seconds for the exponential retry formula.
	// +kubebuilder:validation:Minimum=30
	// +kubebuilder:default=60
	// +optional
	RetryBackoffSeconds int `json:"retryBackoffSeconds,omitempty"`
}

// ClusterConversionBatchStatus defines the observed state of ClusterConversionBatch.
type ClusterConversionBatchStatus struct {
	// Phase is the aggregate phase of the batch.
	// +optional
	Phase ClusterConversionBatchPhase `json:"phase,omitempty"`

	// Hosts contains per-host conversion status, one entry per spec.hosts entry.
	// +optional
	Hosts []HostConversionStatus `json:"hosts,omitempty"`

	// TotalHosts is the total number of hosts in the batch.
	// +optional
	TotalHosts int `json:"totalHosts,omitempty"`

	// SucceededHosts is the number of hosts in Succeeded phase.
	// +optional
	SucceededHosts int `json:"succeededHosts,omitempty"`

	// NeedsAttentionHosts is the number of hosts in NeedsAttention phase.
	// +optional
	NeedsAttentionHosts int `json:"needsAttentionHosts,omitempty"`

	// SkippedHosts is the number of hosts in Skipped phase.
	// +optional
	SkippedHosts int `json:"skippedHosts,omitempty"`

	// RunningHosts is the number of hosts in Converting phase.
	// +optional
	RunningHosts int `json:"runningHosts,omitempty"`

	// PendingHosts is the number of hosts not yet started (CheckingEligibility/NotReady/Ready).
	// +optional
	PendingHosts int `json:"pendingHosts,omitempty"`

	// StartedAt is when the first host began converting.
	// +optional
	StartedAt *metav1.Time `json:"startedAt,omitempty"`

	// CompletedAt is when the batch reached a terminal phase.
	// +optional
	CompletedAt *metav1.Time `json:"completedAt,omitempty"`

	// Message is a human-readable summary.
	// +optional
	Message string `json:"message,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Cluster",type=string,JSONPath=`.spec.vmwareClusterName`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Total",type=integer,JSONPath=`.status.totalHosts`
// +kubebuilder:printcolumn:name="Succeeded",type=integer,JSONPath=`.status.succeededHosts`
// +kubebuilder:printcolumn:name="Running",type=integer,JSONPath=`.status.runningHosts`
// +kubebuilder:printcolumn:name="NeedsAttention",type=integer,JSONPath=`.status.needsAttentionHosts`
// +kubebuilder:printcolumn:name="AutoStart",type=string,JSONPath=`.spec.autoStart`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// ClusterConversionBatch is the Schema for converting a set of ESXi hosts to PCD hosts.
// It acts as a passive grouper: it tracks per-host conversion state and aggregates status,
// but never cascades failures or deletions to sibling ESXIMigration resources.
type ClusterConversionBatch struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterConversionBatchSpec   `json:"spec,omitempty"`
	Status ClusterConversionBatchStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ClusterConversionBatchList contains a list of ClusterConversionBatch.
type ClusterConversionBatchList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterConversionBatch `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClusterConversionBatch{}, &ClusterConversionBatchList{})
}
