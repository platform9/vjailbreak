# Data-Only Migration Design

**Issue:** [#492](https://github.com/platform9/vjailbreak/issues/492)  
**Date:** 2026-07-21  
**Status:** Draft

## Problem

vjailbreak always creates an OpenStack Nova VM at the end of every migration. There is no way to copy and convert VM disks without provisioning the target VM. Teams that pre-stage workloads, run DR pipelines, or require manual approval before instantiation have no supported path.

## Goal

Add a `dataOnly` mode to `MigrationPlanStrategy` that:
- Copies VM disk(s) from VMware to OpenStack Cinder volumes
- Converts disks (vmdk → raw/qcow2, virt-v2v, initramfs)
- Skips Neutron port reservation and Nova VM creation
- Reports a new terminal phase `DataCopied` with the staged Cinder volume IDs

## Out of Scope

- Glance image upload (volumes left as Cinder volumes only)
- New CRD or separate controller
- Automatic VM creation from staged volumes (user handles that)

---

## Design

### 1. CRD Changes

**`MigrationPlanStrategy`** (`k8s/migration/api/v1alpha1/migrationplan_types.go`):

```go
// DataOnly skips OpenStack VM creation; converted Cinder volumes are left staged.
// Compatible with all strategy types (hot, cold, mock).
// +kubebuilder:default:=false
DataOnly bool `json:"dataOnly,omitempty"`
```

`DataOnly` is orthogonal to `Type` (hot/cold/mock). `Type` controls how disks are copied; `DataOnly` controls whether a VM is created after conversion.

**`MigrationSpec`** (`k8s/migration/api/v1alpha1/migration_types.go`):

```go
// DataOnly indicates no OpenStack VM should be created after disk conversion.
// +optional
DataOnly bool `json:"dataOnly,omitempty"`
```

**`VMMigrationPhase`** enum — add terminal phase:

```go
// VMMigrationPhaseDataCopied indicates disks were copied and converted but no VM was created.
VMMigrationPhaseDataCopied VMMigrationPhase = "DataCopied"
```

Update `+kubebuilder:validation:Enum=...` tag to include `DataCopied`.

**`MigrationStatus`** — add staged volume IDs:

```go
// StagedVolumeIDs lists the Cinder volume IDs created during a data-only migration.
// +optional
StagedVolumeIDs []string `json:"stagedVolumeIDs,omitempty"`
```

Run `make generate` in `k8s/migration/` after all type edits. Run `go mod tidy` in `k8s/migration/` and `v2v-helper/` if any dependencies change.

---

### 2. Data Flow

```
MigrationPlanStrategy.DataOnly
  → MigrationSpec.DataOnly          (controller: copied when creating Migration CR)
  → MigrationParams.DataOnly        (v2v-helper/pkg/utils/params.go)
  → Migrate.DataOnly                (v2v-helper/migrate/migrate.go Migrate struct)
```

No new environment variables. `MigrationParams` already reads `Migration` CR fields in-cluster.

---

### 3. Controller Changes

**File:** `k8s/migration/internal/controller/migrationplan_controller.go` line ~1000

When creating a `Migration` CR from a `MigrationPlan`, add to the `MigrationSpec` literal:
```go
DataOnly: migrationplan.Spec.MigrationStrategy.DataOnly,
```

`DataCopied` is a terminal success phase — treat identically to `Succeeded` for:
- `MigrationPlan` progress tracking
- `RollingMigrationPlan` slot release

Cutover-related phases (`AwaitingCutOverStartTime`, `AwaitingAdminCutOver`) are never entered in data-only mode.

---

### 4. v2v-helper Changes

**`Migrate` struct** (`v2v-helper/migrate/migrate.go`):

```go
DataOnly bool
```

**`main.go`** — pass `migrationparams.DataOnly` when constructing `migrate.Migrate{}`.

**`MigrateVM`** behavior when `DataOnly=true`:

| Step | Normal | DataOnly |
|------|--------|----------|
| `ReservePortsForVM` | runs | **skipped** |
| Disk copy (NBD/HotAdd/XCOPY) | runs | runs |
| `ConvertVolumes` | runs | runs |
| `CreateTargetInstance` | runs | **skipped** |
| `DisconnectSourceNetworkIfRequested` | runs | **skipped** |
| Terminal phase | `Succeeded` | `DataCopied` |

At the end of `MigrateVM` when `DataOnly=true`:

```go
if migobj.DataOnly {
    migobj.logMessage("DataOnly mode: skipping VM creation, volumes staged")
    migobj.reportStagedVolumeIDs(ctx, vminfo)
    return nil
}
// existing CreateTargetInstance + DisconnectSourceNetwork code
```

**`reportStagedVolumeIDs`** (new helper in `migrate.go`):
- Iterates `vminfo.VMDisks`, collects `disk.VolumeID`
- Patches `Migration.Status.StagedVolumeIDs` via `K8sClient`
- Sends `DataCopied` phase via `EventReporter` channel

**Cleanup path unchanged** — on failure before conversion, volumes and snapshots are still deleted.

**Phase transitions for data-only:**

```
Pending → Validating → AwaitingDataCopyStart → CopyingBlocks
  → [CopyingChangedBlocks — hot only] → ConvertingDisk → DataCopied
```

---

### 5. UI Changes

**Migration form** (strategy section):
- Add checkbox: **"Data only (no VM creation)"** → `strategy.dataOnly`
- When checked: hide/disable flavor, availability zone, security groups, server group, network mapping fields
- When checked: show info callout — *"Converted Cinder volumes will be staged in OpenStack. No VM will be created."*

**Migration status display:**
- Phase `DataCopied` renders as green success with label `"Data Copied"`
- Show `stagedVolumeIDs` list for copy/reference

---

### 6. Testing

**TDD sequence (constitution IV — NON-NEGOTIABLE):** Write tests first → confirm they fail → implement → confirm they pass (Red-Green-Refactor).

Unit tests required for every Go file modified (not just new files — per project memory).

| Test file | Go file covered | Coverage |
|-----------|----------------|----------|
| `v2v-helper/migrate/migrate_test.go` | `migrate.go` | `DataOnly=true`: `CreateTargetInstance` not called, `DataCopied` phase returned |
| `v2v-helper/migrate/migrate_test.go` | `migrate.go` | `DataOnly=false`: existing behavior unchanged (regression) |
| `v2v-helper/pkg/utils/params_test.go` | `params.go` | `DataOnly` read correctly from `Migration` CR |
| `k8s/migration/internal/controller/migrationplan_controller_test.go` | `migrationplan_controller.go` | `DataOnly` propagated from `MigrationPlanStrategy` → `MigrationSpec` |

Tests use table-driven style and mock external dependencies (vCenter, OpenStack, Kubernetes API — no live system contact).

---

### 7. External Documentation (Constitution II)

Before implementing, consult:
- **controller-runtime** — https://pkg.go.dev/sigs.k8s.io/controller-runtime (reconciler patterns, status patch)
- **virt-v2v** — https://libguestfs.org/virt-v2v.1.html (conversion behavior unchanged but verify no side effects of skipping VM boot step)

---

### 8. Implementation Branch

All implementation commits must land on a feature branch (e.g., `feature/492-data-only-migration`), never directly on `main` (per project memory and CONTRIBUTING.md).

---

## Files Changed

| File | Change |
|------|--------|
| `k8s/migration/api/v1alpha1/migrationplan_types.go` | Add `DataOnly` to `MigrationPlanStrategy` |
| `k8s/migration/api/v1alpha1/migration_types.go` | Add `DataOnly` to `MigrationSpec`; add `DataCopied` phase; add `StagedVolumeIDs` to status |
| `k8s/migration/api/v1alpha1/zz_generated.deepcopy.go` | Regenerated |
| `k8s/migration/internal/controller/` | Propagate `DataOnly`; treat `DataCopied` as terminal success |
| `v2v-helper/migrate/migrate.go` | Add `DataOnly` field; skip ports+VM creation; add `reportStagedVolumeIDs` |
| `v2v-helper/pkg/utils/params.go` | Add `DataOnly` to `MigrationParams` |
| `v2v-helper/main.go` | Pass `DataOnly` to `Migrate` struct |
| `ui/src/` | Checkbox + conditional field visibility + `DataCopied` status display |
| `k8s/migration/config/crd/bases/` | Regenerated CRD YAML (via `make generate`) |
