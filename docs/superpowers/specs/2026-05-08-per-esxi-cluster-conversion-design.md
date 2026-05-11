# Design: Per-ESXi Cluster Conversion Flow

**Date**: 2026-05-08
**Status**: Approved
**Author**: Omkar Deshpande

---

## Problem Statement

The current cluster conversion flow is a single end-to-end orchestration that:
- Converts an entire cluster in one shot — fragile, any failure stops everything
- Tightly couples to Ubuntu MAAS for bare-metal provisioning
- Puts too much orchestration logic inside vJailbreak (`ClusterMigration` controller)
- Offers no per-ESXi granularity or partial recovery

These problems are causing adoption challenges. Users want incremental, per-host control.

---

## Goals

- Allow users to convert one ESXi host at a time, independently
- Show per-ESXi VM count and conversion eligibility in UI
- Let users select and migrate VMs from a specific ESXi host
- Let users put an ESXi in maintenance mode at any point
- Show "Convert to PCD Host" only when ESXi is empty AND in maintenance mode
- Decouple `ESXIMigration` from `ClusterMigration` — standalone operation

---

## Non-Goals

- Replacing the existing VM migration form/wizard
- Removing `ClusterMigration` CRD (kept for backward compat, becomes optional orchestrator)
- Automated rolling conversion (user drives each step manually)

---

## UI Design

### Navigation

Existing `ClusterConversionsPage` → cluster list unchanged. Clicking a cluster opens the cluster
detail view with an ESXi host accordion (no new sidebar entry, no new page).

### Cluster Detail View — ESXi Accordion

Each ESXi host is a collapsible row. Header always shows:

| Element | Description |
|---|---|
| Host name | Monospace, e.g. `esxi-01.prod.local` |
| VM count bar | Progress bar + `N VMs` label; color-coded by count |
| State chip | Busy / Empty / Maintenance / PCD Host |
| Put in Maintenance | Always visible; calls maintenance API |
| Migrate VMs | Visible when VM count > 0 |
| Convert to PCD Host | Visible only when VM count = 0 AND state = Maintenance |

Summary bar at top of accordion: `N Busy · N Empty · N Maintenance · N Converted`

### Expanded Row — VM Table

Expanding a Busy host shows a VM table:

| Column | Source |
|---|---|
| Checkbox (multi-select) | UI state |
| VM Name | `ListVMs` API |
| OS | `ListVMs` API |
| Resources (vCPU · RAM) | `ListVMs` API |
| Status | Pending / Migrating / Migrated |

Actions below table:
- **Select All** toggle
- **Migrate Selected (N) →** — opens existing migration form pre-populated with selected VMs

### ESXi Host State Machine

```
[Any host]
  └─ "Put in Maintenance" always available

[Busy] ──(VM count → 0)──► [Empty]
  └─ "Migrate VMs" visible      └─ "Migrate VMs" hidden

[Any] ──(user clicks Put in Maintenance)──► [Maintenance]

[Maintenance + VM count = 0] ──► shows "Convert to PCD Host"
  └─ Creates ESXIMigration CR → row shows conversion progress
```

---

## Backend Design

### 1. `VMwareCluster` CRD — Add Per-Host VM Counts

**File**: `k8s/migration/api/v1alpha1/vmwarecluster_types.go`

Add to `VMwareClusterStatus`:

```go
type HostStatus struct {
    // Name is the ESXi host name
    Name string `json:"name"`
    // VMCount is the number of VMs currently running on this host
    VMCount int `json:"vmCount"`
    // InMaintenanceMode indicates whether the host is in vCenter maintenance mode
    InMaintenanceMode bool `json:"inMaintenanceMode"`
}

type VMwareClusterStatus struct {
    Phase   VMwareClusterPhase `json:"phase,omitempty"`
    // Hosts tracks per-host VM counts and maintenance state
    Hosts   []HostStatus       `json:"hosts,omitempty"`
}
```

`VMwareCluster` controller polls vCenter periodically to update `Hosts[*].VMCount` and
`InMaintenanceMode`. UI reads a single k8s object instead of N API calls on load.

Run `make generate` in `k8s/migration/` after this change.

### 2. `ESXIMigration` CRD — Make `RollingMigrationPlanRef` Optional

**File**: `k8s/migration/api/v1alpha1/esximigration_types.go`

```go
// Before (required — MAAS coupling)
RollingMigrationPlanRef corev1.LocalObjectReference `json:"rollingMigrationPlanRef"`

// After (optional — standalone operation)
RollingMigrationPlanRef *corev1.LocalObjectReference `json:"rollingMigrationPlanRef,omitempty"`
```

`ESXIMigration` controller: skip `RollingMigrationPlan` lookup when ref is nil. All other
phases (`Cordoned → InMaintenanceMode → ConvertingToPCDHost → AssigningRole → Succeeded`)
remain unchanged.

Run `make generate` in `k8s/migration/` after this change.

### 3. `ClusterMigration` Controller — No Change Required

`ClusterMigration` continues creating `ESXIMigration` children as before. New standalone
path simply creates `ESXIMigration` directly without a `ClusterMigration` parent. No
owner reference required — controller already reconciles by label/name, not owner.

### 4. Maintenance Mode API

Existing `CordonHost` endpoint (`/vpw/v1/cordon_host`) handles DRS cordon. Extend or
add a thin wrapper to also call govmomi `EnterMaintenanceMode` on the host object.
Return combined result. UI calls this single endpoint from "Put in Maintenance" button.

---

## UI Component Plan

### New / Modified Components

| Component | Change |
|---|---|
| `ClusterConversionsPage` | Add cluster-detail view with ESXi accordion |
| `ESXiHostRow` (new) | Collapsible row: header + expanded VM table |
| `ESXiVMTable` (new) | Checkbox table; "Migrate Selected" triggers existing migration form |
| `useVMwareClusterQuery` (existing) | Already fetches `VMwareCluster`; reads new `status.hosts` field |
| `useESXIMigrationsQuery` (existing) | Polls `ESXIMigration` status for converting hosts |

### Data Flow

```
VMwareCluster CR (status.hosts[]) ──► ESXiHostRow (VM count, maintenance state)
ListVMs API ──────────────────────► ESXiVMTable (VM details on expand)
ESXIMigration CR ─────────────────► ESXiHostRow (conversion progress)
```

---

## Migration / Compatibility

- Existing `ClusterMigration` + `ESXIMigration` workflows continue to work unchanged
- `RollingMigrationPlanRef` becoming a pointer is backward compatible (existing objects
  with the field set continue to work; new standalone objects omit it)
- `VMwareClusterStatus.Hosts` is additive — no existing fields removed

---

## Open Items

- Confirm govmomi API for `EnterMaintenanceMode` — consult govmomi docs before implementing
- Decide polling interval for `VMwareCluster` controller host VM count refresh (suggest 30s)
- Confirm whether "Convert to PCD Host" should block if an `ESXIMigration` already exists
  for this host (to prevent duplicate CRs)
