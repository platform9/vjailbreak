# v2v-helper Module Restructure — Phase 1 Design

**Date:** 2026-07-30
**Scope:** v2v-helper Go module only
**Type:** Mechanical restructure — no logic changes inside any function body
**Phase:** 1 of N

---

## Problem Statement

The `v2v-helper/` module has grown organically, producing three critical structural smells:

1. **God object** — `migrate/migrate.go` is 2,723 LOC with 52 methods mixing orchestration, copy strategies, network config, volume lifecycle, and conversion pipeline.
2. **Business logic in utils** — `pkg/utils/openstackopsutils.go` is 1,392 LOC of OpenStack business logic (34 methods on `OpenStackClients`) hiding in a "utils" package.
3. **Interface/impl split across packages** — `openstack/openstackops.go` defines the 24-method interface; the implementation lives in `pkg/utils/`. Violates cohesion, requires import workarounds.

---

## Goals & Constraints

**Goal:** Restructure for navigability and cohesion. No behavior change.

**Hard constraints:**
- Zero logic changes inside any function body
- No code changes within function bodies — functions move verbatim
- No new abstractions or interfaces (except wiring existing `ReporterOps`)
- All existing tests must pass after restructuring
- CGO build constraints unchanged (`CGO_ENABLED=1 GOOS=linux GOARCH=amd64`)

**Success criteria:**
- `migrate.go` shrinks from 2,723 → ~550 LOC (orchestration + struct only)
- `pkg/utils/openstackopsutils.go` removed — implementation lives in `openstack/clients.go`
- Each package's purpose is unambiguous from its name and contents
- `go build ./...` and `go vet ./...` pass

---

## Target Package Structure

### 1. `migrate/` — Split by Concern

All new files remain in the same Go package (`package migrate`). Within-package file splits are zero-impact in Go — no import changes required anywhere.

| File | Contents | Approx LOC |
|------|---------|------------|
| `migrate.go` | `Migrate` struct + `MigrateVM()` orchestration + `cleanup()` + cutover helpers (`WaitforCutover`, `CheckIfAdminCutoverSelected`, `CheckCutoverOptions`, `gracefulTerminate`) | ~550 |
| `nbd_copy.go` | `LiveReplicateDisks` + `EnableCBTWrapper` + `SyncCBT` + `WaitForAdminCutover` | ~450 |
| `network.go` | Port reservation (`ReservePortsForVM`, `reuseExistingPorts`, `createPortsForNetworks`, `createPort`, `resolveNICOverride`, `logAndCollectDetectedIPs`, `applyPreserveIPOverride`, `applyPreserveMACOverride`, `syncIPperMacFromPort`) + OS network config (`configureWindowsNetwork`, `configureLinuxNetwork`, `configureUbuntuNetwork`, `configureRHELNetwork`, `addUdevRulesForUbuntu`, `DetectAndHandleNetwork`) | ~760 |
| `conversion.go` | `ConvertVolumes` + `attachAllVolumes` + boot detection (`detectBootVolume`, `handleLinuxOSDetection`, `handleWindowsBootDetection`, `validateLinuxOS`, `getBootCommand`) + virt-v2v dispatch (`performDiskConversion`) + OS helpers (`parseVersionID`, `isNetplanSupported`, `blockDriverFromMetadata`) | ~780 |
| `vm_ops.go` | Volume lifecycle (`CreateVolumes`, `AttachVolume`, `DetachVolume`, `DetachAllVolumes`, `DetachAllVolumesWithCleanup`, `DeleteAllVolumes`, `applyImageMetadataForXCOPYVolumes`, `verifyVMCreatedDespiteTimeout`) + storage provider setup (`InitializeStorageProvider`, `LoadESXiSSHKey`, `buildProviderOptions`, `reportStagedVolumeIDs`) + instance creation + health checks (`CreateTargetInstance`, `HealthCheck`, `pingVM`, `checkHTTPGet`, `tryConnection`) + utilities (`extractFileName`, `logDiskCopyPlan`, `validateDiskMapping`, `logMessage`, `LogMessage`) | ~650 |
| `hotadd_copy.go` | Unchanged | 616 |
| `vaai_copy.go` | Unchanged | 510 |

**Total after split:** 7 files, no file exceeds ~800 LOC, each with a single clear domain.

### 2. `openstack/` — Fix Interface/Implementation Split

Move `pkg/utils/openstackopsutils.go` into the `openstack/` package where the interface already lives.

| Change | Detail |
|--------|--------|
| Move file | `pkg/utils/openstackopsutils.go` → `openstack/clients.go` |
| Package declaration | `package utils` → `package openstack` (1 line change) |
| All method bodies | Verbatim — zero changes |
| Update callers | All files importing `pkg/utils` for `OpenStackClients` → import `openstack` |

**Callers to update (~3-4 import sites):**
- `migrate/vm_ops.go` (post-split)
- `migrate/vaai_copy.go`
- `main.go`

### 3. `reporter/` — Wire Existing Interface to Callers

`ReporterOps` interface already exists and is exported. Issue: `migrate.go` holds `*reporter.Reporter` (concrete type) instead of the interface.

| Change | From | To |
|--------|------|---|
| `Migrate.Reporter` field type | `*reporter.Reporter` | `reporter.ReporterOps` |

Struct field type change only — not inside a function body. `*reporter.Reporter` already satisfies the interface, so no callers break.

---

## Unchanged Packages

`nbd/`, `vcenter/`, `vm/`, `virtv2v/`, `esxi-ssh/`, `pkg/k8sutils/`, `pkg/xml/`, `pkg/version/`, `pkg/utils/vmutils/`

`pkg/utils/` retains: `nbdutils.go`, `virtv2v_errors.go`, `nbd_errors.go`, `vcenterutils.go`, `utils.go`, `targetmetadata.go` — all legitimate utilities.

---

## Build & Validation

Run after each sub-step:

```bash
cd v2v-helper && go build ./...
cd v2v-helper && go vet ./...

# Full test (requires Linux CGO environment):
make test-v2v-helper
```

---

## Risk Assessment

| Risk | Likelihood | Mitigation |
|------|-----------|------------|
| Missed import update for `openstackopsutils.go` move | Medium | `go build ./...` catches immediately |
| Circular import after moving `openstackopsutils.go` | Low | `openstack/` already imported by `migrate/` — impl move doesn't change import graph |
| Method accidentally omitted from split files | Low | `go build ./...` catches missing symbols; verify with `grep -c "^func" migrate.go` before and after |

---

## Out of Scope (Phase 2+)

- Decomposing `Migrate` struct (107 fields) into nested config structs
- Adding new tests for `esxi-ssh/`, `hotadd_copy.go`, `vaai_copy.go`
- Extracting `CopyStrategy` interface
- Splitting `vm/vmops.go` (1,098 LOC) or `virtv2v/virtv2vops.go` by concern
- Moving single-caller helper functions (`getDiskKeys`, `readGuestWWIDs`, `sanitizeVolumeName`) — YAGNI until reused
