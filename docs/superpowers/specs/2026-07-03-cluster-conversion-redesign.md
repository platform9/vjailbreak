# Feature Specification: Cluster Conversion Redesign

**Feature Directory**: `docs/superpowers/specs/`
**Created**: 2026-07-03
**Status**: Draft
**Author**: omkar@platform9.com

---

## Overview

Redesign the vJailbreak cluster conversion feature (ESXi host → PCD host conversion) to improve adoption, operator confidence, and resilience. The current architecture uses a rigid three-tier cascade (RollingMigrationPlan → ClusterMigration → ESXIMigration) where any single host failure terminates the entire batch. The redesign introduces a new `ClusterConversionBatch` CRD as a passive grouper, makes each `ESXIMigration` fully autonomous, calculates host eligibility dynamically and continuously, and isolates host failures from one another.

---

## Problem Statement

### Current Architecture Failures

The existing cluster conversion flow (`RollingMigrationPlan → ClusterMigration → ESXIMigration`) has three structural problems that are suppressing adoption:

**1. Cascade failure kills the batch.** In `clustermigration_controller.go`, when any `ESXIMigration` enters `Failed` phase, the `ClusterMigration` immediately transitions to `Failed`. `RollingMigrationPlanReconciler.ExecuteRollingMigrationPlan` then reads this and marks the `RollingMigrationPlan` as `Failed`, halting all remaining host conversions. One transient BMC failure on one host destroys hours of batch progress.

**2. Static validation blocks start.** Eligibility for all hosts is checked once at plan creation time (in `ValidateRollingMigrationPlan`). If any host is unregistered in MAAS or lacks a PCD cluster mapping, the entire plan stays in `ValidationFailed` and no host starts conversion — even hosts that are fully ready.

**3. No retry mechanism.** `ESXIMigration` treats `Failed` as a terminal state (`case ESXIMigrationPhaseFailed: return ctrl.Result{}, nil`). Engineers must delete and recreate CRs to recover.

These problems together mean that the first time an operator tries cluster conversion on a real, mixed-health datacenter, the entire workflow fails and requires manual intervention to resume.

### Adoption Blockers

- Engineers cannot confidently start a batch without 100% host pre-validation — blocking production use.
- No per-host retry forces a full re-start for transient failures (network blips, MAAS timeouts).
- UI shows `RollingMigrationPlan` status only; per-host eligibility is invisible until failure.
- Sequential host conversion (one host starts only after previous succeeds) is unnecessarily slow.

---

## Goals

1. Host failures are **isolated**: a failed host does not block or abort sibling hosts.
2. Per-host eligibility is **dynamic and continuous**: re-evaluated as cluster capacity changes during migration (as VMs evacuate, more hosts may become eligible).
3. Transient failures trigger **automatic retry with exponential backoff** up to a configurable limit.
4. Operators can **manually retry or skip** any host from the UI without editing Kubernetes CRs.
5. `ClusterConversionBatch` is a **passive grouper**: it stores selected hosts and aggregated status but never orchestrates or aborts `ESXIMigration` children.
6. `ESXIMigration` is **fully autonomous**: it does not require a parent `RollingMigrationPlan` to function.
7. Auto-start mode: eligible hosts start conversion automatically when ready (configurable).
8. In-flight `RollingMigrationPlan` resources **complete under the old controller** without disruption.

---

## Non-Goals

- PCD-side capacity gating (OpenStack quota checks before starting conversion).
- VM migration orchestration within cluster conversion (which VMs to migrate, in what order, via what plan — that is the existing VM migration track, left unchanged).
- Automatic host ordering/sequencing (no forced "convert host A before host B" unless operator explicitly sequences batches).
- Multi-region coordination or cross-cluster batches.
- Modifying the VM migration track (`Migration`, `MigrationPlan`, `MigrationTemplate` CRDs).

---

## User Scenarios & Testing

### User Story 1 — Create a batch and watch hosts auto-convert (Priority: P1)

An operator selects a vCenter cluster in the vJailbreak UI, picks five ESXi hosts for conversion, sets `AutoStart = Auto`, and submits. The system continuously checks eligibility per host. As each host becomes eligible (DRS ready, MAAS match found, BMConfig valid, enough cluster capacity), its `ESXIMigration` is automatically created and conversion begins. If one host fails mid-conversion due to a transient MAAS timeout, it retries automatically. The remaining hosts continue unaffected.

**Why this priority**: This is the primary use case. It requires all redesign goals to work together: dynamic eligibility, auto-start, isolation, and retry.

**Independent Test**: Create a `ClusterConversionBatch` with three hosts where one host has a transient failure. Verify the other two succeed and the failing host retries and eventually also succeeds.

**Acceptance Scenarios**:

1. **Given** a batch with 3 hosts and `AutoStart = Auto`, **When** all 3 hosts are eligible, **Then** all 3 `ESXIMigration` resources are created in parallel and conversion proceeds simultaneously.
2. **Given** a batch with 3 hosts where host B is initially not in MAAS, **When** host B is registered in MAAS while A and C are converting, **Then** host B becomes eligible and its conversion starts without operator action.
3. **Given** host A's ESXIMigration fails with a transient error, **When** the retry backoff elapses, **Then** the controller automatically deletes and recreates the `ESXIMigration` for host A, and hosts B and C are unaffected.
4. **Given** host A exhausts all retries, **When** the operator clicks "Skip" in the UI, **Then** host A's status becomes `Skipped` and the batch completes once B and C succeed.

---

### User Story 2 — Manual-trigger mode for cautious operators (Priority: P2)

An operator creates a batch with `AutoStart = Manual`. The UI shows each host with a readiness badge. When a host reaches `Ready` (all eligibility checks pass), a "Start" button activates for that host. The operator reviews the host details and clicks "Start". Conversion begins only for that host; other hosts remain in `Ready` state until individually triggered.

**Why this priority**: Enterprises converting production clusters need human approval per host before physical hardware is touched. This is the second-most common deployment pattern.

**Independent Test**: Create a `ClusterConversionBatch` with `AutoStart = Manual`, verify hosts reach `Ready` but no `ESXIMigration` is created until the "Start" action is invoked per host.

**Acceptance Scenarios**:

1. **Given** a batch with `AutoStart = Manual` and two eligible hosts, **When** both reach `Ready`, **Then** the UI shows "Start" buttons for both and no `ESXIMigration` exists yet.
2. **Given** the operator clicks "Start" for host A only, **Then** only host A's `ESXIMigration` is created; host B remains `Ready`.
3. **Given** `AutoStart` is changed from `Manual` to `Auto` mid-flight via the UI, **Then** all currently-`Ready` hosts begin conversion automatically.

---

### User Story 3 — Per-host eligibility visibility before batch creation (Priority: P2)

Before creating a `ClusterConversionBatch`, an operator selects a cluster in the UI and sees a pre-flight eligibility table showing each host's readiness status (Ready / NotReady with reason). The operator can select only the `Ready` hosts, or select all and let the system wait for `NotReady` hosts to become eligible.

**Why this priority**: Operators need confidence before starting. This visibility is what makes the "select all, let system handle it" workflow safe.

**Independent Test**: Point the UI at a cluster where host A passes all checks and host B lacks a MAAS entry. Verify the pre-flight table shows host A as `Ready` and host B as `NotReady: host not found in MAAS`.

**Acceptance Scenarios**:

1. **Given** a cluster with 3 hosts, **When** operator opens the "Create Batch" dialog, **Then** the UI displays eligibility status for each host before the batch is created.
2. **Given** a host with `DRS = enabled but not fully automated`, **When** shown in the pre-flight table, **Then** the reason is `NotReady: DRS is not in fully-automated mode`.
3. **Given** the operator selects only eligible hosts and submits, **Then** the batch is created with only those hosts.

---

### User Story 4 — Retry and skip stuck hosts (Priority: P3)

A host enters `NeedsAttention` after exhausting retries. The operator clicks "Retry" in the UI, which resets the retry counter and triggers a new conversion attempt. If the host is genuinely broken, the operator clicks "Skip" and the batch finalizes with that host marked `Skipped`.

**Why this priority**: Operators need escape hatches that do not require `kubectl` access to production clusters.

**Independent Test**: Simulate a host that always fails. Verify after N retries it enters `NeedsAttention`. Verify "Retry" triggers another attempt. Verify "Skip" marks it `Skipped` and the batch phase advances.

**Acceptance Scenarios**:

1. **Given** a host fails 3 consecutive times (default `maxRetries`), **When** the last retry fails, **Then** the host phase transitions to `NeedsAttention` and an alert appears in the UI.
2. **Given** a host in `NeedsAttention`, **When** operator clicks "Retry", **Then** `retryCount` resets to 0 and a new `ESXIMigration` is created.
3. **Given** a host in `NeedsAttention`, **When** operator clicks "Skip", **Then** the host phase becomes `Skipped` and the batch recalculates overall phase.
4. **Given** all hosts are `Succeeded` or `Skipped`, **Then** the batch phase transitions to `Succeeded` (if any succeeded) or `PartialFail` (if none succeeded).

---

### Edge Cases

- Batch with 1 host: no siblings to affect; failure → batch fails after retries.
- Host already in maintenance mode when batch starts: eligibility check must handle this gracefully.
- VMware cluster loses DRS automation mid-batch: hosts not yet started should become `NotReady`.
- `BMConfig` becomes invalid mid-batch: already-converting hosts continue; newly-starting hosts fail eligibility.
- Concurrent batches referencing the same cluster: each batch manages its own hosts independently; eligibility calculation is per-host and considers cluster-wide remaining capacity.
- Operator deletes a batch while hosts are converting: `ESXIMigration` resources are NOT deleted (autonomous). The batch is removed; ESXIMigration objects remain and complete or fail independently.

---

## Architecture

### Two Independent Tracks

The redesign preserves the existing VM migration track completely unchanged. Host conversion becomes a separate, independent track:

```
VM Migration Track (unchanged):
  Migration → MigrationPlan → MigrationTemplate → VMMigration

Host Conversion Track (redesigned):
  ClusterConversionBatch (passive grouper)
      ↓ creates (per eligible host, one at a time or in parallel)
  ESXIMigration (fully autonomous)
      ↓ provisions via
  BMConfig (MAAS)
```

### ClusterConversionBatch Role

`ClusterConversionBatch` is a **passive grouper**. Its controller:

- Periodically re-evaluates eligibility for each host.
- Creates `ESXIMigration` for hosts that are `Ready` (in `Auto` mode) or explicitly triggered (in `Manual` mode).
- Reads `ESXIMigration.Status.Phase` to update per-host status in `ClusterConversionBatch.Status.Hosts`.
- Handles retry: deletes and recreates `ESXIMigration` after backoff.
- Aggregates overall batch phase from per-host outcomes.
- **Never aborts a sibling ESXIMigration** due to another host's failure.
- **Never modifies** an in-progress `ESXIMigration`'s spec.

### ESXIMigration Changes

`ESXIMigration` becomes fully self-contained:

- `RollingMigrationPlanRef` becomes optional (kept for backward compatibility with in-flight plans).
- `BMConfigRef` is added directly to `ESXIMigrationSpec` for new-style migrations.
- The controller no longer requires a parent `RollingMigrationPlan` to function.
- Failure in `ESXIMigration` does NOT propagate upward; the controller simply writes `Phase = Failed` and stops.
- **No logic removed except**: when `RollingMigrationPlanRef` is empty, skip the `rollingMigrationPlan` fetch.

### Deprecation Path

| CRD | Status | Behavior |
|-----|--------|----------|
| `RollingMigrationPlan` | Deprecated | Controller continues to handle existing resources; no new resources created via UI |
| `ClusterMigration` | Deprecated | Replaced by `ClusterConversionBatch`; existing resources complete normally |
| `ClusterConversionBatch` | New | Handles all new cluster conversion workflows |
| `ESXIMigration` | Enhanced | Made autonomous; used by both old (via ClusterMigration) and new (via ClusterConversionBatch) flows |

---

## CRD Schemas

### ClusterConversionBatch (New)

```go
package v1alpha1

import (
    corev1  "k8s.io/api/core/v1"
    metav1  "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AutoStartMode controls when eligible hosts begin conversion.
// +kubebuilder:validation:Enum=Auto;Manual
type AutoStartMode string

const (
    // AutoStartModeAuto starts conversion automatically when a host becomes eligible.
    AutoStartModeAuto AutoStartMode = "Auto"
    // AutoStartModeManual requires operator to explicitly trigger each host.
    AutoStartModeManual AutoStartMode = "Manual"
)

// ClusterConversionBatchPhase is the overall batch lifecycle phase.
// +kubebuilder:validation:Enum=Pending;Running;Succeeded;PartialFail;Failed
type ClusterConversionBatchPhase string

const (
    // ClusterConversionBatchPhasePending indicates no host has started converting.
    ClusterConversionBatchPhasePending ClusterConversionBatchPhase = "Pending"
    // ClusterConversionBatchPhaseRunning indicates at least one host is actively converting.
    ClusterConversionBatchPhaseRunning ClusterConversionBatchPhase = "Running"
    // ClusterConversionBatchPhaseSucceeded indicates all selected hosts converted successfully.
    ClusterConversionBatchPhaseSucceeded ClusterConversionBatchPhase = "Succeeded"
    // ClusterConversionBatchPhasePartialFail indicates the batch finished with some hosts
    // failed or skipped and at least one succeeded.
    ClusterConversionBatchPhasePartialFail ClusterConversionBatchPhase = "PartialFail"
    // ClusterConversionBatchPhaseFailed indicates all hosts failed or were skipped, none succeeded.
    ClusterConversionBatchPhaseFailed ClusterConversionBatchPhase = "Failed"
)

// HostConversionPhase is the per-host lifecycle phase within a batch.
// +kubebuilder:validation:Enum=CheckingEligibility;NotReady;Ready;Converting;Succeeded;Failed;NeedsAttention;Skipped
type HostConversionPhase string

const (
    // HostConversionPhaseCheckingEligibility is the initial phase: eligibility not yet computed.
    HostConversionPhaseCheckingEligibility HostConversionPhase = "CheckingEligibility"
    // HostConversionPhaseNotReady indicates the host does not yet meet eligibility criteria.
    HostConversionPhaseNotReady HostConversionPhase = "NotReady"
    // HostConversionPhaseReady indicates the host passes all eligibility checks and awaits conversion start.
    // In Manual mode, conversion does not start automatically from this phase.
    HostConversionPhaseReady HostConversionPhase = "Ready"
    // HostConversionPhaseConverting indicates an ESXIMigration exists and is in-progress.
    HostConversionPhaseConverting HostConversionPhase = "Converting"
    // HostConversionPhaseSucceeded indicates the ESXIMigration completed successfully.
    HostConversionPhaseSucceeded HostConversionPhase = "Succeeded"
    // HostConversionPhaseFailed indicates the ESXIMigration failed and retries remain.
    HostConversionPhaseFailed HostConversionPhase = "Failed"
    // HostConversionPhaseNeedsAttention indicates the host exhausted all automatic retries.
    HostConversionPhaseNeedsAttention HostConversionPhase = "NeedsAttention"
    // HostConversionPhaseSkipped indicates the host was explicitly skipped by an operator.
    HostConversionPhaseSkipped HostConversionPhase = "Skipped"
)

// EligibilityStatus is the outcome of the per-host eligibility check.
// +kubebuilder:validation:Enum=Ready;NotReady;Unknown
type EligibilityStatus string

const (
    EligibilityStatusReady    EligibilityStatus = "Ready"
    EligibilityStatusNotReady EligibilityStatus = "NotReady"
    EligibilityStatusUnknown  EligibilityStatus = "Unknown"
)

// HostEntry identifies a single ESXi host selected for conversion.
type HostEntry struct {
    // ESXiName is the display name of the ESXi host as it appears in vCenter.
    ESXiName string `json:"esxiName"`
}

// HostConversionStatus is the per-host status stored in ClusterConversionBatch.Status.Hosts.
type HostConversionStatus struct {
    // ESXiName identifies the host.
    ESXiName string `json:"esxiName"`

    // Phase is the current conversion lifecycle phase for this host.
    Phase HostConversionPhase `json:"phase"`

    // EligibilityStatus is the most recent eligibility check result.
    EligibilityStatus EligibilityStatus `json:"eligibilityStatus,omitempty"`

    // EligibilityReason is a human-readable explanation when EligibilityStatus = NotReady.
    EligibilityReason string `json:"eligibilityReason,omitempty"`

    // RetryCount is the number of automatic conversion retries attempted so far.
    RetryCount int `json:"retryCount,omitempty"`

    // NextRetryAt is the earliest time the controller will attempt another retry.
    NextRetryAt *metav1.Time `json:"nextRetryAt,omitempty"`

    // ESXIMigrationRef references the child ESXIMigration object, if one exists.
    ESXIMigrationRef *corev1.LocalObjectReference `json:"esxiMigrationRef,omitempty"`

    // Message is the most recent human-readable status message for this host.
    Message string `json:"message,omitempty"`

    // StartedAt is the time conversion was first initiated for this host.
    StartedAt *metav1.Time `json:"startedAt,omitempty"`

    // CompletedAt is the time this host reached a terminal phase (Succeeded, NeedsAttention, Skipped).
    CompletedAt *metav1.Time `json:"completedAt,omitempty"`

    // SkippedAt is the time an operator explicitly skipped this host.
    SkippedAt *metav1.Time `json:"skippedAt,omitempty"`
}

// ClusterConversionBatchSpec defines the desired state of a cluster conversion batch.
type ClusterConversionBatchSpec struct {
    // VMwareClusterName is the display name of the vCenter cluster containing the hosts.
    VMwareClusterName string `json:"vmwareClusterName"`

    // VMwareCredsRef references the VMwareCreds object for vCenter access.
    VMwareCredsRef corev1.LocalObjectReference `json:"vmwareCredsRef"`

    // OpenstackCredsRef references the OpenstackCreds object for PCD access.
    OpenstackCredsRef corev1.LocalObjectReference `json:"openstackCredsRef"`

    // BMConfigRef references the BMConfig object for MAAS bare-metal provisioning.
    BMConfigRef corev1.LocalObjectReference `json:"bmConfigRef"`

    // CloudInitConfigRef optionally references a secret containing cloud-init user data
    // to inject into newly provisioned PCD hosts.
    CloudInitConfigRef *corev1.SecretReference `json:"cloudInitConfigRef,omitempty"`

    // Hosts lists the ESXi hosts selected for conversion in this batch.
    // Order is not significant; hosts may convert in parallel.
    // +kubebuilder:validation:MinItems=1
    Hosts []HostEntry `json:"hosts"`

    // AutoStart controls when eligible hosts begin conversion.
    // Auto: conversion starts automatically when a host becomes Ready.
    // Manual: operator must explicitly trigger each host via the API or UI.
    // +kubebuilder:default="Auto"
    AutoStart AutoStartMode `json:"autoStart,omitempty"`

    // MaxRetries is the maximum number of automatic retry attempts per host
    // before transitioning to NeedsAttention.
    // +kubebuilder:default=3
    // +kubebuilder:validation:Minimum=0
    MaxRetries int `json:"maxRetries,omitempty"`

    // RetryBackoffSeconds is the base retry interval in seconds.
    // The actual backoff doubles with each attempt (exponential backoff).
    // +kubebuilder:default=60
    // +kubebuilder:validation:Minimum=30
    RetryBackoffSeconds int `json:"retryBackoffSeconds,omitempty"`
}

// ClusterConversionBatchStatus defines the observed state of a ClusterConversionBatch.
type ClusterConversionBatchStatus struct {
    // Phase is the overall batch lifecycle phase.
    Phase ClusterConversionBatchPhase `json:"phase,omitempty"`

    // Hosts contains per-host status entries, one per entry in Spec.Hosts.
    Hosts []HostConversionStatus `json:"hosts,omitempty"`

    // TotalHosts is the total number of hosts in the batch.
    TotalHosts int `json:"totalHosts,omitempty"`

    // SucceededHosts is the count of hosts that completed conversion successfully.
    SucceededHosts int `json:"succeededHosts,omitempty"`

    // NeedsAttentionHosts is the count of hosts that exhausted all automatic retries and require operator action.
    NeedsAttentionHosts int `json:"needsAttentionHosts,omitempty"`

    // SkippedHosts is the count of hosts explicitly skipped by an operator.
    SkippedHosts int `json:"skippedHosts,omitempty"`

    // RunningHosts is the count of hosts currently converting (ESXIMigration in progress).
    RunningHosts int `json:"runningHosts,omitempty"`

    // PendingHosts is the count of hosts in CheckingEligibility, NotReady, or Ready phases.
    PendingHosts int `json:"pendingHosts,omitempty"`

    // StartedAt is the time the first host conversion began.
    StartedAt *metav1.Time `json:"startedAt,omitempty"`

    // CompletedAt is the time the batch reached a terminal phase.
    CompletedAt *metav1.Time `json:"completedAt,omitempty"`

    // Message is a human-readable summary of overall batch status.
    Message string `json:"message,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Total",type="integer",JSONPath=".status.totalHosts"
// +kubebuilder:printcolumn:name="Succeeded",type="integer",JSONPath=".status.succeededHosts"
// +kubebuilder:printcolumn:name="NeedsAttention",type="integer",JSONPath=".status.needsAttentionHosts"
// +kubebuilder:printcolumn:name="Running",type="integer",JSONPath=".status.runningHosts"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// ClusterConversionBatch groups ESXi hosts for coordinated conversion to PCD hosts.
// It is a passive grouper: it tracks eligibility and status per host and creates
// ESXIMigration resources, but never orchestrates or aborts them once started.
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
```

---

### ESXIMigration Changes (Minimal)

The following fields are added to `ESXIMigrationSpec`. All existing fields remain unchanged.

```go
// ESXIMigrationSpec additions for autonomous operation.
// Add these fields to the existing ESXIMigrationSpec struct.

type ESXIMigrationSpec struct {
    // ... existing fields unchanged ...

    // ESXiName is the name of the ESXi host to be migrated.
    ESXiName string `json:"esxiName"`

    // OpenstackCredsRef is the reference to the OpenStack credentials.
    OpenstackCredsRef corev1.LocalObjectReference `json:"openstackCredsRef"`

    // VMwareCredsRef is the reference to the VMware credentials.
    VMwareCredsRef corev1.LocalObjectReference `json:"vmwareCredsRef"`

    // RollingMigrationPlanRef is the reference to the parent RollingMigrationPlan.
    // DEPRECATED: Required only for ESXIMigration resources created by the old flow.
    // New ClusterConversionBatch-managed resources set BMConfigRef directly instead.
    // +optional
    RollingMigrationPlanRef corev1.LocalObjectReference `json:"rollingMigrationPlanRef,omitempty"`

    // BMConfigRef references the BMConfig for bare-metal provisioning.
    // Required when RollingMigrationPlanRef is not set (new-style autonomous flow).
    // When both are set, BMConfigRef takes precedence.
    // +optional
    BMConfigRef *corev1.LocalObjectReference `json:"bmConfigRef,omitempty"`

    // ClusterConversionBatchRef references the owning ClusterConversionBatch.
    // Set when created by the ClusterConversionBatch controller; absent for old-flow resources.
    // +optional
    ClusterConversionBatchRef *corev1.LocalObjectReference `json:"clusterConversionBatchRef,omitempty"`
}
```

**New ESXIMigration phase added:**

```go
const (
    // ... existing phases unchanged ...

    // ESXIMigrationPhaseNeedsAttention indicates a transient failure that requires operator review.
    // Unlike Failed (which is terminal in the old flow), this phase allows retry.
    // Used by the ClusterConversionBatch controller to signal retry is needed.
    // The ESXIMigration controller does not self-transition to this phase.
    ESXIMigrationPhaseNeedsAttention ESXIMigrationPhase = "NeedsAttention"
)
```

---

### Manual Trigger API Annotation

Operators trigger per-host conversion in `Manual` mode by patching an annotation on the `ClusterConversionBatch`:

```yaml
metadata:
  annotations:
    vjailbreak.k8s.pf9.io/trigger-host: "esxi01.example.com"
```

The controller reads this annotation on each reconcile, processes the trigger, and removes the annotation. This keeps the operator action lightweight (annotation patch) without requiring spec changes.

Similarly, retry and skip are expressed via annotations:

```yaml
vjailbreak.k8s.pf9.io/retry-host: "esxi01.example.com"
vjailbreak.k8s.pf9.io/skip-host:  "esxi01.example.com"
```

The UI handles annotation patching transparently; operators do not need `kubectl` access.

---

## Controller Logic Changes

### New: ClusterConversionBatchReconciler

```
ClusterConversionBatchReconciler.Reconcile:
  1. Fetch ClusterConversionBatch
  2. Initialize Status.Hosts if empty (one entry per Spec.Hosts item)
  3. Process annotation-based operator actions (trigger, retry, skip) — see below
  4. For each host entry in Status.Hosts (in no particular order):
     a. If phase is terminal (Succeeded, Skipped): skip
     b. If phase is NeedsAttention: skip (awaits retry/skip annotation)
     c. If ESXIMigration exists for this host:
        - Read ESXIMigration.Status.Phase
        - Mirror phase into HostConversionStatus.Phase (Converting / Succeeded / Failed)
        - If ESXIMigration.Status.Phase == Failed:
            → Increment RetryCount
            → If RetryCount <= MaxRetries:
                Compute NextRetryAt = now + backoff(RetryCount)
                Set HostConversionStatus.Phase = Failed
              Else:
                Set HostConversionStatus.Phase = NeedsAttention
     d. If no ESXIMigration exists:
        - Run eligibility check for this host (see Eligibility Algorithm below)
        - Update EligibilityStatus and EligibilityReason
        - If EligibilityStatus == Ready:
            If AutoStart == Auto:
              Create ESXIMigration
              Set HostConversionStatus.Phase = Converting
            Else (Manual):
              Set HostConversionStatus.Phase = Ready
        - If EligibilityStatus == NotReady:
            Set HostConversionStatus.Phase = NotReady
     e. If phase == Failed AND NextRetryAt <= now:
        - Delete failed ESXIMigration (if it still exists)
        - Re-run eligibility check
        - If still eligible: Create new ESXIMigration
  5. Aggregate counts into Status counters
  6. Derive batch Phase from host phases (see Batch Phase Table below)
  7. Update ClusterConversionBatch.Status
  8. Return requeue after 30 seconds (always re-evaluate eligibility)
```

**Batch Phase Derivation Table:**

| Condition | Batch Phase |
|-----------|-------------|
| All hosts in terminal phases, all Succeeded | `Succeeded` |
| All hosts in terminal phases, some Succeeded and some NeedsAttention/Skipped | `PartialFail` |
| All hosts in terminal phases, none Succeeded | `Failed` |
| Any host Converting or in retry backoff | `Running` |
| All hosts in CheckingEligibility/NotReady/Ready | `Pending` |

**Annotation Processing (step 3):**

```
For annotation vjailbreak.k8s.pf9.io/trigger-host: "<esxiName>":
  Find host entry with matching ESXiName
  If phase == Ready: create ESXIMigration, set phase = Converting
  Remove annotation from ClusterConversionBatch

For annotation vjailbreak.k8s.pf9.io/retry-host: "<esxiName>":
  Find host entry with matching ESXiName
  If phase == NeedsAttention:
    Reset RetryCount = 0
    Delete existing ESXIMigration if present
    Set phase = CheckingEligibility
  Remove annotation from ClusterConversionBatch

For annotation vjailbreak.k8s.pf9.io/skip-host: "<esxiName>":
  Find host entry with matching ESXiName
  Set phase = Skipped
  Set SkippedAt = now
  Do NOT delete the ESXIMigration if one is in progress (it runs to completion independently)
  Remove annotation from ClusterConversionBatch
```

### Modified: ESXIMigrationReconciler

**Change 1: Optional RollingMigrationPlanRef fetch**

```go
// Replace the mandatory RollingMigrationPlan fetch with conditional logic:
if esxiMigration.Spec.RollingMigrationPlanRef.Name != "" {
    rollingMigrationPlan := &vjailbreakv1alpha1.RollingMigrationPlan{}
    key := client.ObjectKey{Namespace: ..., Name: esxiMigration.Spec.RollingMigrationPlanRef.Name}
    if err := r.Get(ctx, key, rollingMigrationPlan); err != nil {
        if apierrors.IsNotFound(err) && !esxiMigration.DeletionTimestamp.IsZero() {
            return r.reconcileDelete(ctx, scope)
        }
        return ctrl.Result{}, errors.Wrap(err, "failed to get RollingMigrationPlan")
    }
    scope.RollingMigrationPlan = rollingMigrationPlan
}
```

**Change 2: Autonomous BMConfig resolution**

```go
// In reconcileNormal, resolve BMConfig autonomously:
func resolveBMConfig(ctx context.Context, client client.Client, esxiMigration *ESXIMigration, rollingMigrationPlan *RollingMigrationPlan) (*BMConfig, error) {
    var bmConfigName string
    if esxiMigration.Spec.BMConfigRef != nil && esxiMigration.Spec.BMConfigRef.Name != "" {
        bmConfigName = esxiMigration.Spec.BMConfigRef.Name
    } else if rollingMigrationPlan != nil {
        bmConfigName = rollingMigrationPlan.Spec.BMConfigRef.Name
    } else {
        return nil, errors.New("no BMConfig reference available: set spec.bmConfigRef on ESXIMigration")
    }
    bmConfig := &BMConfig{}
    if err := client.Get(ctx, types.NamespacedName{Name: bmConfigName, Namespace: ...}, bmConfig); err != nil {
        return nil, errors.Wrap(err, "failed to get BMConfig")
    }
    return bmConfig, nil
}
```

**Change 3: No logic removed from ESXIMigration controller**

The ESXIMigration controller does NOT need to change its failure behavior. It still sets `Phase = Failed` on error. The isolation guarantee comes from the `ClusterConversionBatch` controller NOT cascading that failure to siblings.

### Unchanged: RollingMigrationPlanReconciler and ClusterMigrationReconciler

These controllers continue to operate as-is for in-flight resources. No changes required.

---

## Eligibility Algorithm

The eligibility check is run per-host on every reconcile of `ClusterConversionBatch`. It is **not cached** — each check goes to vCenter to get current state.

```
EligibilityCheck(ctx, host, batch) → (EligibilityStatus, reason):

  1. CLUSTER_HOST_COUNT: Get all hosts in the VMware cluster.
     If count <= 1: return NotReady, "cluster has only one host; cannot vMotion VMs"

  2. DRS_ENABLED: Check cluster DRS configuration.
     If DRS not enabled: return NotReady, "DRS is not enabled on the cluster"

  3. DRS_FULLY_AUTOMATED: Check DRS automation level.
     If DRS automation level != FullyAutomated: return NotReady, "DRS is not in fully-automated mode"

  4. VMOTION_CAPACITY: Estimate if remaining hosts (excluding this host) can absorb this host's VMs.
     - Count VMs on this host (excluding cold-migrated VMs, i.e., VMs selected in an active MigrationPlan)
     - Sum CPU and memory of those VMs
     - Sum available CPU and memory headroom on remaining hosts in the cluster
     - If remaining capacity < VM load: return NotReady, "remaining hosts lack capacity to absorb <N> VMs (need <X> CPU, <Y> memory)"

  5. ANTI_AFFINITY_RULES: Check for VM anti-affinity or HA rules that would block vMotion.
     - Query vCenter for DRS rules referencing VMs on this host
     - Exclude VMs already targeted by an active cold migration (MigrationPlan)
     - If any must-not-run-on-same-host rule cannot be satisfied: return NotReady, "VM anti-affinity rule <name> would block vMotion for VMs: <list>"

  6. MAAS_MATCH: Verify this host is registered in MAAS.
     - Fetch BMConfig, connect to MAAS provider
     - Match by hardware UUID (primary) or MAC address (fallback), matching existing EnsureESXiInMass logic
     - If no match: return NotReady, "host not found in MAAS (tried UUID and MAC matching)"
     - If match found but status != Deployed or Allocated: return NotReady, "host found in MAAS but status is <status>; expected Deployed or Allocated"

  7. BMCONFIG_VALID: Verify BMConfig.Status.ValidationStatus == "Succeeded"
     - If not: return NotReady, "BMConfig <name> validation has not succeeded (status: <status>)"

  8. PCD_CLUSTER_CONFIGURED: Verify at least one PCDCluster object exists for the target OpenStack credentials.
     - If none: return NotReady, "no PCD cluster configured for OpenStack credentials <name>"

  9. All checks passed: return Ready, ""
```

**Notes on dynamic recalculation:**

- Steps 4 and 5 (capacity and anti-affinity) change continuously as VMs are cold-migrated or vMotioned during the batch. A host that was `NotReady` due to capacity may become `Ready` after other VMs are migrated off the cluster.
- The algorithm deliberately excludes cold-migrated VMs (VMs in an active `MigrationPlan` with `MigratingVMs` or `Succeeded` status) from the capacity calculation, as those VMs will not need to be vMotioned.
- Step 6 (MAAS match) reuses the existing `EnsureESXiInMass` function from `rollingmigrationutils.go` without modification.

---

## Failure Handling

### Transient Failure → Auto Retry

When an `ESXIMigration` fails:

1. `ClusterConversionBatch` controller detects `ESXIMigration.Status.Phase == Failed`.
2. Increments `HostConversionStatus.RetryCount`.
3. Computes next retry time: `now + RetryBackoffSeconds * 2^(RetryCount-1)` (exponential, e.g., 60s, 120s, 240s for default 60s base).
4. Sets `HostConversionStatus.NextRetryAt`.
5. On next reconcile after `NextRetryAt`:
   - Deletes the failed `ESXIMigration`.
   - Re-runs eligibility check.
   - If eligible: creates new `ESXIMigration`.

### Persistent Failure → NeedsAttention

When `RetryCount > MaxRetries`:

1. Sets `HostConversionStatus.Phase = NeedsAttention`.
2. Batch continues for all other hosts.
3. UI shows alert badge for this host with last error message.
4. No further automatic action.

### Operator Retry

Operator clicks "Retry" in UI (or patches annotation `vjailbreak.k8s.pf9.io/retry-host`):

1. Resets `HostConversionStatus.RetryCount = 0`.
2. Deletes existing failed `ESXIMigration` if present.
3. Transitions host to `CheckingEligibility`.
4. On next reconcile, full eligibility re-check runs.

### Operator Skip

Operator clicks "Skip" in UI (or patches annotation `vjailbreak.k8s.pf9.io/skip-host`):

1. Sets `HostConversionStatus.Phase = Skipped` and `SkippedAt = now`.
2. If an `ESXIMigration` is actively converting, it is **not deleted** — it runs to completion independently. The batch controller simply stops tracking it.
3. Batch recalculates overall phase. If all other hosts are complete, batch may reach `Succeeded` or `PartialFail`.

### Sibling Isolation Guarantee

The `ClusterConversionBatch` controller reconcile loop processes each host entry independently. There is no shared state or early-exit that would halt processing of subsequent hosts if one host entry encounters an error. Errors are written to `HostConversionStatus.Message` and the loop continues to the next host.

### ESXIMigration Failure Semantics (Unchanged)

`ESXIMigration` controller behavior is NOT changed. It still:
- Sets `Phase = Failed` on unrecoverable errors.
- Does not self-retry.
- Does not propagate failures upward (it has no reference to `ClusterConversionBatch`).

The `ClusterConversionBatch` controller owns retry logic entirely.

---

## UI Requirements

### Cluster Conversions Page

The existing `ClusterConversionsPage` (currently showing `RollingMigrationPlans`) should:

1. Show `ClusterConversionBatch` resources as primary cards.
2. Show deprecated `RollingMigrationPlan` resources in a separate "Legacy" section (read-only, no create button).

### Create Batch Flow

New "Create Conversion Batch" dialog:

1. **Step 1: Select VMware cluster**
   - Dropdown of available vCenter clusters (from VMwareCreds).
   - After selection, show a pre-flight eligibility table (see Pre-flight Table below).

2. **Step 2: Configure batch**
   - Host selection checkboxes (pre-checked for Ready hosts; unchecked for NotReady, still selectable).
   - `AutoStart` toggle (Auto / Manual).
   - Advanced: `MaxRetries` (default 3), `RetryBackoffSeconds` (default 60).
   - BMConfig selector (if multiple configured).
   - OpenStack creds selector.
   - Optional cloud-init secret selector.

3. **Step 3: Confirm and create**
   - Summary of selected hosts and configuration.
   - "Create Batch" button.

### Pre-flight Eligibility Table

Displayed before batch creation and updated in real time while the dialog is open:

| Column | Description |
|--------|-------------|
| Host Name | ESXi hostname |
| Status | Ready (green) / NotReady (amber) / Unknown (gray) |
| Reason | Shown only when NotReady; human-readable eligibility failure |
| VMs | Count of VMs on this host |
| CPU/Memory | Current utilization |

### Batch Detail View

Clicking a batch opens a detail panel with:

- Batch-level status badge and progress bar (e.g., "3/5 hosts converted").
- Per-host table:

| Column | Description |
|--------|-------------|
| Host | ESXi hostname |
| Phase | CheckingEligibility / NotReady / Ready / Converting / Succeeded / Failed / NeedsAttention / Skipped |
| Eligibility | Ready or NotReady with reason (shown when not Converting or beyond) |
| Retries | `RetryCount / MaxRetries` (e.g., "2/3") |
| Duration | Time since conversion started |
| Actions | Context-sensitive per phase (see Action Buttons below) |

### Action Buttons (Per Host)

| Host Phase | Available Actions |
|------------|------------------|
| Ready (Manual mode) | "Start" |
| Converting | None |
| Failed (retries remaining) | None (system retries automatically) |
| NeedsAttention | "Retry", "Skip" |
| Skipped | None |
| Succeeded | None |

### AutoStart Mode Toggle

- Displayed in the batch detail header.
- Toggle switch: "Auto" / "Manual".
- Changing from Manual → Auto immediately triggers conversion for all currently-`Ready` hosts.
- Changing from Auto → Manual does not stop in-progress conversions.
- Change is applied by patching `ClusterConversionBatch.Spec.AutoStart`.

---

## Migration Path

### In-flight RollingMigrationPlans

- `RollingMigrationPlanReconciler` and `ClusterMigrationReconciler` remain in the codebase and continue to reconcile existing resources.
- No changes to these controllers.
- Existing `RollingMigrationPlan` resources complete under the old controller without disruption.

### New Batches

- All new cluster conversion workflows use `ClusterConversionBatch`.
- The UI "Create Cluster Conversion" button creates a `ClusterConversionBatch`.
- The UI no longer creates `RollingMigrationPlan` resources.

### ESXIMigration Backward Compatibility

- Existing `ESXIMigration` resources (created by `ClusterMigration`) have `RollingMigrationPlanRef` set and `BMConfigRef` absent. The modified controller handles this via the conditional fetch logic.
- New `ESXIMigration` resources (created by `ClusterConversionBatch`) have `BMConfigRef` set and `RollingMigrationPlanRef` absent.
- Both variants function correctly with the same controller.

### Deprecation Notice

- `RollingMigrationPlan` and `ClusterMigration` CRDs are marked deprecated via annotation:
  ```yaml
  metadata:
    annotations:
      vjailbreak.k8s.pf9.io/deprecated: "true"
      vjailbreak.k8s.pf9.io/deprecated-message: "Use ClusterConversionBatch instead"
  ```
- CRD YAML files remain; they are not deleted until all customers have migrated (minimum 2 release cycles).
- `kubectl get rollingmigrationplan` continues to work; no breaking change.

---

## Requirements

### Functional Requirements

- **FR-001**: System MUST create a `ClusterConversionBatch` resource for each new cluster conversion batch submitted via the UI.
- **FR-002**: System MUST calculate per-host eligibility independently, covering all eight eligibility criteria (DRS enabled, DRS fully automated, multi-host cluster, vMotion capacity, anti-affinity rules, MAAS match, BMConfig valid, PCD cluster configured).
- **FR-003**: System MUST recalculate host eligibility on every controller reconcile cycle (approximately every 30 seconds), not just at batch creation.
- **FR-004**: In `AutoStart = Auto` mode, system MUST automatically create an `ESXIMigration` for a host as soon as its eligibility transitions to `Ready`.
- **FR-005**: In `AutoStart = Manual` mode, system MUST NOT create an `ESXIMigration` for a host until the operator explicitly triggers it via the UI or API annotation.
- **FR-006**: Failure of one host's `ESXIMigration` MUST NOT affect the conversion of any sibling host within the same batch.
- **FR-007**: System MUST automatically retry a failed host conversion up to `MaxRetries` times with exponential backoff, starting from `RetryBackoffSeconds` as the base interval.
- **FR-008**: After exhausting retries, system MUST transition the host to `NeedsAttention` phase and expose an alert in the UI.
- **FR-009**: UI MUST provide a per-host "Retry" action for hosts in `NeedsAttention` phase that resets the retry counter and re-initiates conversion.
- **FR-010**: UI MUST provide a per-host "Skip" action for hosts in `NeedsAttention` phase that marks the host as `Skipped` without deleting an in-progress `ESXIMigration`.
- **FR-011**: `AutoStart` mode MUST be changeable at any time during batch execution without restarting the batch.
- **FR-012**: System MUST consider the batch complete (terminal phase) when all hosts are in `Succeeded`, `NeedsAttention`, or `Skipped` phases. `Failed` is NOT a terminal host phase — it transitions to `Converting` once the retry backoff elapses, or to `NeedsAttention` when retries are exhausted.
- **FR-013**: Existing `RollingMigrationPlan` resources MUST continue to run to completion without disruption after this redesign is deployed.
- **FR-014**: `ESXIMigration` resources MUST function without a parent `RollingMigrationPlan` when `BMConfigRef` is provided directly in the spec.
- **FR-015**: System MUST expose the eligibility failure reason in `HostConversionStatus.EligibilityReason` for each `NotReady` host.

### Key Entities

- **ClusterConversionBatch**: Groups ESXi hosts for coordinated conversion. Stores batch-level configuration and aggregated per-host status. Never orchestrates or aborts children.
- **HostConversionStatus**: Per-host status within a batch. Tracks eligibility, phase, retry count, timing, and references to the child `ESXIMigration`.
- **ESXIMigration**: Per-host conversion unit. Now fully autonomous; acquires BMConfig, OpenStack creds, and VMware creds from its own spec rather than requiring a parent plan.
- **BMConfig**: MAAS configuration. Validated independently. Referenced directly by autonomous `ESXIMigration` resources.
- **EligibilityCheck**: A computed, stateless function that evaluates eight criteria against live vCenter and MAAS state. Not stored as its own resource.

---

## Success Criteria

### Measurable Outcomes

- **SC-001**: A 5-host batch where one host has a transient MAAS timeout completes successfully (remaining 4 hosts `Succeeded`, failing host eventually `Succeeded` after retry) within the expected conversion time without operator intervention.
- **SC-002**: Eligibility status for each host reflects current vCenter cluster state within 60 seconds of a change (e.g., a VM evacuating from a host increases remaining capacity, allowing a previously `NotReady` host to become `Ready`).
- **SC-003**: Operators can create and monitor a cluster conversion batch without using `kubectl` — all required actions (create, retry, skip, mode change) are available in the UI.
- **SC-004**: A host that fails 3 consecutive times enters `NeedsAttention` and receives no further automatic conversion attempts or vCenter API calls until an operator acts (the controller still requeues every 30s but skips all processing for that host).
- **SC-005**: An in-flight `RollingMigrationPlan` created before the redesign continues to completion after upgrading to the new version, with no manual migration steps required.
- **SC-006**: Switching `AutoStart` from Manual to Auto in the UI triggers conversion for all currently-`Ready` hosts within one reconcile cycle (≤ 60 seconds).
- **SC-007**: The Create Batch dialog displays per-host health indicators (sourced from `VMwareHost` objects) before batch creation to aid host selection. Full eight-criteria eligibility results (DRS, MAAS, BMConfig, PCD cluster, capacity, anti-affinity) are visible in the batch detail view within 60 seconds of batch creation, with specific failure reasons for `NotReady` hosts.

---

## Assumptions

- VMware vCenter DRS settings are accessible via govmomi's cluster configuration APIs without additional authentication.
- MAAS hardware UUID and MAC address matching logic already implemented in `EnsureESXiInMass` is sufficient and does not need to change.
- vCenter capacity estimation (step 4 in eligibility) can be computed from `mo.HostSystem` properties (`summary.hardware.memorySize`, `summary.hardware.numCpuCores`) and `mo.VirtualMachine` properties (`summary.config.numCpu`, `summary.config.memorySizeMB`).
- The `ClusterConversionBatch` controller reconciles every 30 seconds (standard `ctrl.Result{RequeueAfter: 30*time.Second}`), which is fast enough for real-time eligibility updates.
- Each `ClusterConversionBatch` targets hosts in a single vCenter cluster. Cross-cluster batches are out of scope.
- All operator actions (trigger, retry, skip, mode change) that require CR modification go through the vJailbreak REST API or UI — operators do not need direct `kubectl` access to the k3s cluster.
- The `PCDCluster` CRD and its listing API exist and are queryable within the migration namespace (referenced in existing `EnsurePCDHasClusterConfigured` function).

---

## Open Questions

1. **Eligibility recalculation cost**: Checking vCenter capacity and MAAS for every host on every 30-second reconcile may generate excessive API load for large batches (20+ hosts). Should eligibility be cached with a TTL (e.g., 5 minutes) per host, with only a forced refresh when operator requests retry? What is the acceptable vCenter API call frequency?

2. **Skip-but-ESXIMigration-still-running**: When an operator skips a host that has an in-progress `ESXIMigration`, the spec says "ESXIMigration runs to completion independently." However, the ESXi host will be converted to a PCD host even though the batch considers it skipped. Is this the intended behavior? Should the UI warn operators before allowing skip on a `Converting` host?

3. **Concurrent batches, same cluster**: If two `ClusterConversionBatch` resources target different hosts in the same VMware cluster, eligibility capacity calculations on one batch must account for hosts being converted by the other batch. How should the capacity calculation identify hosts being converted by other batches? By label on `ESXIMigration`? By checking `ClusterConversionBatch` status across all batches?

4. **ClusterConversionBatch deletion behavior**: If a batch is deleted while hosts are converting, the spec says `ESXIMigration` resources are NOT deleted (they run to completion). This orphans `ESXIMigration` objects. Should the batch controller use owner references on `ESXIMigration` objects? If so, Kubernetes garbage collection would delete them on batch deletion, which conflicts with the "autonomous" goal.

5. **Eligibility step 4 precision**: The vMotion capacity check is an approximation (does not account for vSphere admission control, HA reserved capacity, per-VM overheads). At what level of approximation is this useful vs. misleading? Should the check be a configurable option (enabled/disabled via BMConfig or batch spec) to avoid false `NotReady` results in environments with complex HA configurations?
