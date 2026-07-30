# Implementation Plan: v2v-helper Module Restructure — Phase 1

**Branch**: `2180-v2v-restructure` | **Date**: 2026-07-30 | **Spec**: [spec.md](spec.md)

## Summary

Mechanical restructure of the `v2v-helper` Go module for navigability and cohesion. Three changes: (1) split `migrate/migrate.go` (2,723 LOC) into 5 focused files within the same package, (2) move `pkg/utils/openstackopsutils.go` to `openstack/clients.go` with package declaration update, (3) change `Migrate.Reporter` field type from concrete `*reporter.Reporter` to interface `reporter.ReporterOps`. No logic changes inside any function body.

## Technical Context

**Language/Version**: Go 1.24+
**Primary Dependencies**: Same as existing v2v-helper module (govmomi, gophercloud, libnbd CGO)
**Storage**: N/A — file moves only, no data model changes
**Testing**: `make test-v2v-helper` (CGO_ENABLED=1 GOOS=linux GOARCH=amd64); `go build ./...` on macOS for import validation
**Target Platform**: Linux (CGO required for libnbd); macOS for import-only validation
**Project Type**: Internal Go module refactor
**Performance Goals**: N/A — no runtime behavior changes
**Constraints**: CGO_ENABLED=1 GOOS=linux GOARCH=amd64 unchanged; no function body edits; all 10 existing test files must pass

## Constitution Check

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Kubernetes-Native Architecture | ✅ Pass | No state management changes |
| II. External Documentation First | ✅ Pass | Dependencies already consulted during investigation |
| III. Generated Code Protection | ✅ Pass | No generated files touched (no CRD changes) |
| IV. Test-First Development | ✅ Pass | Refactor only — existing tests cover all moved code; no new logic requires new tests |
| V. Module Independence | ✅ Pass | All changes stay within `v2v-helper/` module |
| VI. AI-Assisted Development | ✅ Pass | Skills invoked before any coding |
| VII. Code Reuse and Simplicity | ✅ Pass | This IS the simplicity improvement |

**Gate result**: PASS — proceed to Phase 0.

## Project Structure

### Documentation (this feature)

```text
specs/2180-v2v-restructure/
├── plan.md              ← this file
├── research.md          ← Phase 0 output
└── tasks.md             ← Phase 2 output (/speckit-tasks)
```

### Source Code Changes

```text
v2v-helper/
├── migrate/
│   ├── migrate.go           MODIFIED  (trimmed to ~550 LOC: struct + MigrateVM + cutover)
│   ├── nbd_copy.go          NEW       (NBD copy path: ~450 LOC)
│   ├── network.go           NEW       (port reservation + OS network config: ~760 LOC)
│   ├── conversion.go        NEW       (ConvertVolumes + boot detection + virt-v2v: ~780 LOC)
│   ├── vm_ops.go            NEW       (volume lifecycle + storage setup + instance creation: ~650 LOC)
│   ├── hotadd_copy.go       UNCHANGED
│   └── vaai_copy.go         UNCHANGED
├── openstack/
│   └── clients.go           NEW       (moved from pkg/utils/openstackopsutils.go)
└── pkg/utils/
    └── openstackopsutils.go DELETED
```

**Callers requiring import update** (pkg/utils → openstack):
- `migrate/` files (whichever split file holds the OpenStack calls)
- `migrate/vaai_copy.go`
- `main.go`

## Phase 0: Research

*No NEEDS CLARIFICATION items — codebase was fully investigated prior to planning.*

See [research.md](research.md) for findings.

## Phase 1: Design

### data-model.md
Not applicable — no data model changes. All entities (`Migrate` struct, `OpenStackClients`, etc.) move verbatim.

### contracts/
Not applicable — no public API/interface changes. `reporter.ReporterOps` interface already exists and is exported; only a struct field type changes.

### Implementation Phases

#### Step 1: Split migrate/migrate.go (zero import changes)

Within `package migrate` — file splits are transparent to the compiler and all tests.

Order of extraction (safest: smallest/clearest concern first):

1. Extract NBD copy path → `nbd_copy.go`
   - Methods: `LiveReplicateDisks`, `EnableCBTWrapper`, `SyncCBT`, `WaitForAdminCutover`
   - Build check: `go build ./...`

2. Extract network logic → `network.go`
   - Methods: `ReservePortsForVM`, `reuseExistingPorts`, `createPortsForNetworks`, `createPort`, `resolveNICOverride`, `logAndCollectDetectedIPs`, `applyPreserveIPOverride`, `applyPreserveMACOverride`, `syncIPperMacFromPort`, `configureWindowsNetwork`, `configureLinuxNetwork`, `configureUbuntuNetwork`, `configureRHELNetwork`, `addUdevRulesForUbuntu`, `DetectAndHandleNetwork`
   - Build check: `go build ./...`

3. Extract conversion pipeline → `conversion.go`
   - Methods: `ConvertVolumes`, `attachAllVolumes`, `detectBootVolume`, `handleLinuxOSDetection`, `handleWindowsBootDetection`, `validateLinuxOS`, `getBootCommand`, `performDiskConversion`
   - Standalone functions: `parseVersionID`, `isNetplanSupported`, `blockDriverFromMetadata`
   - Build check: `go build ./...`

4. Extract VM ops → `vm_ops.go`
   - Methods: `CreateVolumes`, `AttachVolume`, `DetachVolume`, `DetachAllVolumes`, `DetachAllVolumesWithCleanup`, `DeleteAllVolumes`, `applyImageMetadataForXCOPYVolumes`, `verifyVMCreatedDespiteTimeout`, `InitializeStorageProvider`, `LoadESXiSSHKey`, `buildProviderOptions`, `reportStagedVolumeIDs`, `CreateTargetInstance`, `HealthCheck`, `pingVM`, `checkHTTPGet`, `tryConnection`
   - Standalone functions: `extractFileName`, `logDiskCopyPlan`, `validateDiskMapping`, `logMessage`, `LogMessage`
   - Build check: `go build ./...`

5. Verify migrate.go contains only: `Migrate` struct, `MigrateVM`, `cleanup`, `WaitforCutover`, `CheckIfAdminCutoverSelected`, `CheckCutoverOptions`, `gracefulTerminate`
   - Line count check: `wc -l migrate/migrate.go` → must be ≤600

#### Step 2: Fix openstack interface/implementation split

1. Pre-check: verify `openstack/` does not import `pkg/utils/` (no circular import risk)
   ```bash
   grep -r "pkg/utils" v2v-helper/openstack/
   ```

2. Create `openstack/clients.go`:
   - Copy content of `pkg/utils/openstackopsutils.go` verbatim
   - Change first line: `package utils` → `package openstack`

3. Delete `pkg/utils/openstackopsutils.go`

4. Update all callers — change imports from `pkg/utils` to `openstack` package at all sites where `OpenStackClients` is referenced:
   - `main.go`
   - `migrate/vaai_copy.go`
   - Any split file from Step 1 that references `OpenStackClients`

5. Build check: `go build ./...`

#### Step 3: Wire ReporterOps interface

1. Pre-check: verify `*reporter.Reporter` satisfies `reporter.ReporterOps`
   ```bash
   grep -A 20 "type ReporterOps interface" v2v-helper/reporter/reporter.go
   grep -A 20 "type Reporter struct" v2v-helper/reporter/reporter.go
   ```

2. In `migrate/migrate.go`, change `Migrate` struct field:
   ```go
   // From:
   Reporter *reporter.Reporter
   // To:
   Reporter reporter.ReporterOps
   ```

3. Build check: `go build ./...`
4. Vet check: `go vet ./...`

#### Step 4: Final validation

```bash
cd v2v-helper
go build ./...
go vet ./...
wc -l migrate/migrate.go   # must be ≤600
ls pkg/utils/              # openstackopsutils.go must be absent
ls openstack/              # clients.go must be present
```

Full test run (Linux/CGO):
```bash
make test-v2v-helper
```
