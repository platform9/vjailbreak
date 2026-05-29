# Implementation Plan: Hot-Add Proxy Migration

**Branch**: `1944-hot-add-proxy` | **Date**: 2026-05-19 | **Spec**: [spec.md](spec.md)  
**Status**: Fully implemented (all T001–T036 complete)  
**Input**: Feature specification from `specs/003-hot-add-proxy/spec.md`

## Summary

Introduce "Hot-Add" as a third data copy method in vJailbreak's VM migration workflow. During the data-copy phase, the system creates a vCenter snapshot of the source VM, attaches frozen disk images to a registered Proxy VM, serves each disk via `qemu-nbd` on the Proxy VM, and streams data to destination disks on the vJailbreak appliance via `nbdcopy`. Only the data-copy phase changes; all other migration phases are unchanged.

The implementation follows the SAM (Storage Accelerated Copy) feature as the reference pattern throughout: new `ProxyVM` CRD analogous to `ArrayCreds`/`ESXiSSHCreds`, new `hotadd_copy.go` analogous to `vaai_copy.go`, and a new UI management section parallel to existing credential pages.

---

## Technical Context

**Language/Version**: Go 1.21 (controller + v2v-helper), TypeScript/React 18 (UI)  
**Primary Dependencies**: controller-runtime, govmomi, golang.org/x/crypto/ssh, MUI, Vite  
**Storage**: Kubernetes etcd (CRD state), no new database  
**Testing**: `cd k8s/migration && make test` (controller), `make test-v2v-helper` (v2v-helper, Linux only), `yarn test` (UI)  
**Target Platform**: k3s on Linux (vJailbreak appliance), React SPA in browser  
**Project Type**: Kubernetes controller + migration worker pod + React UI  
**Performance Goals**: Hot-Add data transfer throughput bounded by NBD/network; no new latency targets beyond existing migration SLOs  
**Constraints**: vSphere 60-disk attachment limit per VM; existing code paths must not regress; minimal changes to existing files  
**Scale/Scope**: Single registered Proxy VM (initial version); up to 60 concurrently attached disks

---

## Constitution Check

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Kubernetes-Native Architecture | PASS | ProxyVM state in CRD; disk count tracked in CR status |
| II. External Documentation First | PASS | govmomi, qemu-nbd, nbdcopy docs consulted in research.md |
| III. Generated Code Protection | PASS | `make generate` required after CRD changes; zz_generated.deepcopy.go never hand-edited |
| IV. Test-First Development | PASS | Unit tests required for all new Go code; mocked govmomi + SSH interfaces |
| V. Module Independence | PASS | Constants in `pkg/common/`; controller in `k8s/migration/`; worker in `v2v-helper/`; no cross-module coupling added |
| VI. AI-Assisted Development | PASS | Skills invoked |
| VII. Code Reuse and Simplicity | PASS | Extending existing enum field; reusing SSH client pattern; reusing SAM code structure |

**Result**: All gates pass. No violations.

---

## Project Structure

### Documentation (this feature)

```text
specs/003-hot-add-proxy/
├── plan.md              # This file (/speckit-plan output)
├── research.md          # Phase 0 — technical decisions and rationale
├── data-model.md        # Phase 1 — entity definitions and state transitions
├── quickstart.md        # Phase 1 — developer guide
├── contracts/
│   └── crds.md          # Phase 1 — CRD YAML contracts and ConfigMap keys
└── tasks.md             # Phase 2 output (/speckit-tasks — NOT created by /speckit-plan)
```

### Source Code Changes

```text
# COMMON CONSTANTS (pkg/common/ module)
pkg/common/constants/
└── constants.go                            [MODIFY]

# CONTROLLER (k8s/migration/ module)
k8s/migration/api/v1alpha1/
├── proxyvm_types.go                        [NEW]
└── migrationtemplate_types.go              [MODIFY — extend enum + ProxyVMRef]

k8s/migration/internal/controller/
├── proxyvm_controller.go                   [NEW]
└── migrationplan_controller.go             [MODIFY — HotAdd validation + ConfigMap]

k8s/migration/cmd/
└── main.go                                 [MODIFY — register ProxyVM controller]

k8s/migration/api/v1alpha1/
└── groupversion_info.go                    [MODIFY — register ProxyVM in SchemeBuilder]

# V2V-HELPER (v2v-helper/ module)
v2v-helper/pkg/utils/
└── vcenterutils.go                         [MODIFY — new MigrationParams fields]

v2v-helper/migrate/
├── hotadd_copy.go                          [NEW]
└── migrate.go                              [MODIFY — HotAdd branch in MigrateVM()]

# UI
ui/src/api/proxy-vm/
└── index.ts                                [NEW]

ui/src/features/proxyVM/
├── ProxyVMPage.tsx                         [NEW]
└── AddProxyVMDialog.tsx                    [NEW]

ui/src/features/migration/
├── NetworkAndStorageMappingStep.tsx        [MODIFY]
└── MigrationForm.tsx                       [MODIFY]

ui/src/components/layout/
└── Sidebar.tsx                             [MODIFY — add Proxy VMs entry]
```

**Structure Decision**: Multi-module project with three independent Go modules and a React UI. Each module modified minimally; new code isolated in dedicated files. Follows existing module layout exactly.

---

## Phase 0: Research (Complete)

See [research.md](research.md). All unknowns resolved:

| Unknown | Resolution |
|---------|------------|
| ProxyVM CRD structure | `ESXiSSHCreds` pattern (VMName + VMwareCredsRef + validation status) |
| StorageCopyMethod extension | Extend existing enum to `normal|StorageAcceleratedCopy|HotAdd` |
| SSH key strategy | Per-ProxyVM keypair stored in k8s Secret `"{proxyVMK8sName}-hot-add-ssh-key"`; v2v-helper reads via `GetHotAddPrivateKey()` |
| Port allocation | `findFreePorts(sshClient, min, max, count)` allocates all N ports at once via `cat /proc/net/tcp /proc/net/tcp6` on ProxyVM |
| Block device identification | wwid/naa UUID matching via `/sys/block/sd*/device/wwid` |
| Concurrency (60-disk limit) | Pre-flight check in controller; runtime increment/decrement in v2v-helper `adjustProxyDiskCount()` |
| Snapshot naming | Fixed constant `"vjailbreak-hotadd-snap"`; delete existing before creating |
| Retry strategy | 3 retries for block device identification; 3 retries for data transfer failure |

---

## Phase 1: Design (Complete)

### Data Model

See [data-model.md](data-model.md).

**New CRDs**: `ProxyVM`  
**Modified CRDs**: `MigrationTemplate` (extended `StorageCopyMethod` enum + `ProxyVMRef` field)  
**New v2v-helper types**: `hotAddDiskTransfer` (in-memory struct in `hotadd_copy.go`), `ProxyVMIP`/`ProxyVMName`/`ProxyVMK8sName` fields in `MigrationParams` and `Migrate` structs  
**New constants** (already present in `pkg/common/constants/constants.go`): `HotAddCopyMethod = "HotAdd"`, `ProxyVMStatusReady`, `ProxyVMStatusVerifying`, `ProxyVMStatusVerificationFailed`, `ProxyVMStatusPending`, `HotAddPortRangeMin = 10809`, `HotAddPortRangeMax = 11808`  
**migrate.go changes**: disk-count guard extended with `&& StorageCopyMethod != HotAddCopyMethod`; HotAdd `else if` branch: `CreateVolumes → AttachVolume per disk → HotAddCopyDisks`

### Contracts

See [contracts/crds.md](contracts/crds.md).

**New CRD**: `ProxyVM` with full YAML examples (Ready + VerificationFailed states)  
**Modified CRD**: `MigrationTemplate` with `storageCopyMethod: HotAdd` + `proxyVMRef` example  
**ConfigMap keys**: `PROXY_VM_IP`, `PROXY_VM_NAME`, `PROXY_VM_K8S_NAME` (alongside existing `STORAGE_COPY_METHOD`)  
**REST API**: Standard Kubernetes CRUD on `/apis/vjailbreak.k8s.pf9.io/v1alpha1/namespaces/migration-system/proxyvms`  
**Retry API**: `PATCH proxyvms/<name>` with `Content-Type: application/merge-patch+json` body `{"metadata":{"annotations":{"vjailbreak.k8s.pf9.io/retry-at":"<ISO-timestamp>"}}}` — triggers controller reconcile and re-runs full verification

### Implementation Details

#### ProxyVM Controller (`proxyvm_controller.go`)

Reconcile loop triggered by ProxyVM Create/Update:

1. Set status to `Verifying`
2. Fetch VMwareCreds → connect to vCenter via govmomi
3. Find VM by `spec.vmName` → extract guest IP from `guest.ipAddress`; set `status.ipAddress`
4. SSH to `root@<ip>` using the private key from k8s Secret `"{proxyVMK8sName}-hot-add-ssh-key"` (generated during ProxyVM onboarding)
5. Check each required component by running `which <name>` via SSH (one command per component — POSIX, present everywhere); check exit code in Go (0 = found); populate `status.componentsVerified`
6. Check `disk.EnableUUID` via govmomi `ExtraConfig`; if not set, set it and trigger VM reboot (wait for guest ready), then re-verify
7. If all checks pass → set status to `Ready`; else set `VerificationFailed` with per-component messages

#### migrationplan_controller.go changes (HotAdd block)

Insert after the `StorageAcceleratedCopy` validation block (~line 670):

```
if migrationtemplate.Spec.StorageCopyMethod == constants.HotAddCopyMethod {
    // 1. Fetch ProxyVM CR
    // 2. Reject if MigrationType == "hot" or "mock" (cold only)
    // 3. Check ProxyVM.Status.ValidationStatus == Ready → else block migration
    // 4. Check ProxyVM.Status.AttachedDiskCount + len(sourceDisks) <= 60 → else capacity error
    // 5. Populate configMapData["PROXY_VM_IP"], ["PROXY_VM_NAME"], ["PROXY_VM_K8S_NAME"]
}
```

`AttachedDiskCount` is incremented/decremented by the v2v-helper (`adjustProxyDiskCount`) at runtime, not by the controller.

#### hotadd_copy.go — `HotAddCopyDisks()`

All remote ProxyVM interactions use Go's `crypto/ssh` client. SSH commands are minimal single-purpose reads; all logic (iteration, matching, parsing) runs in Go (see Decision 10 in research.md).

```
func HotAddCopyDisks(ctx, vminfo) error {
    // 1. Power off source VM + verify with exponential backoff
    VMPowerOff(); DoRetryWithExponentialBackoff(checkPowerState, 3, 5min)

    // 2. Snapshot (quiesce=true, memory=false); fixed name "vjailbreak-hotadd-snap"
    takeVMSnapshot(ctx, "vjailbreak-hotadd-snap")              // govmomi

    // 3. Enumerate frozen VMDKs
    transfers = getFrozenVMDKs(ctx, vminfo)                    // govmomi device info

    // 4. SSH connect to Proxy VM (key from k8s secret "{proxyVMK8sName}-hot-add-ssh-key")

    // 5. Find Proxy VM object in vCenter
    defer cleanupHotAdd(ctx, sshClient, transfers, proxyVMObj) // always runs

    // 6. Attach all frozen disks to Proxy VM
    for each disk: attachDiskToProxy(ctx, proxyVMObj, vmdkPath) → DiskKey
    adjustProxyDiskCount(ctx, +len(transfers))                 // patch ProxyVM status

    // 7. Identify block devices (SSH wwid matching, retry ×3 with 5s wait)
    identifyBlockDevices(ctx, sshClient, transfers, proxyVMObj)

    // 8. Pre-allocate all ports at once (avoids concurrent goroutine race)
    ports = findFreePorts(sshClient, 10809, 11808, len(transfers))

    // 9. Copy all disks concurrently (one goroutine per disk)
    for each transfer in parallel:
        pid = serveViaNBD(sshClient, transfer.BlockDevice, port)
        // SSH: qemu-nbd --format=raw --port=<port> --bind=0.0.0.0 --fork --persistent <dev>
        runNBDCopy(ctx, proxyIP, port, transfer.DestDevice)    // local exec, retry ×3
    wait for all goroutines; collect errors
}
```

#### UI: NetworkAndStorageMappingStep.tsx changes

- Extend copy method options array with `{ value: 'HotAdd', label: 'Hot-Add via Proxy VM' }`
- Add conditional rendering: when `storageCopyMethod === 'HotAdd'`, show a ProxyVM selector (dropdown of Ready ProxyVMs) instead of StorageMapping/ArrayCredsMapping sections
- Fetch Ready ProxyVMs from Kubernetes API when HotAdd is selected

#### UI: ProxyVM management page

- List: name, VM name, status badge (with per-component tooltip on VerificationFailed), IP, attached-disk count, age, last-validated time, delete button, **retry-verification button** (visible only for VerificationFailed rows)
- **Retry mechanism**: `retryProxyVM()` in `ui/src/api/proxy-vm/proxyVm.ts` patches the ProxyVM resource with annotation `vjailbreak.k8s.pf9.io/retry-at: <ISO timestamp>` using `Content-Type: application/merge-patch+json`. This triggers a controller reconcile without deleting the resource.
- Add dialog: **VM Name Autocomplete** (MUI `freeSolo`) — on credential select, fetches VMwareMachines filtered by `labelSelector: vjailbreak.k8s.pf9.io/vmwarecreds=<cred>` and populates a searchable dropdown; free-text still accepted for unlisted VMs. VMwareCreds dropdown (validated only) with SSH key prerequisite Alert above it.
- Status polling: re-fetch ProxyVM list every 5s while any item is Pending/Verifying (same pattern as credentials page)
- Sidebar entry: "Proxy VMs" with ComputerIcon

#### UI: Source files changed

| File | Change |
|------|--------|
| `ui/src/api/proxy-vm/model.ts` | New — ProxyVM, ProxyVMList, ProxyVMComponentCheck interfaces |
| `ui/src/api/proxy-vm/proxyVm.ts` | New — getProxyVMs, getProxyVM, createProxyVM, deleteProxyVM, **retryProxyVM** |
| `ui/src/api/proxy-vm/index.ts` | New — barrel re-exports |
| `ui/src/features/proxyVM/components/AddProxyVMDialog.tsx` | New — VM Autocomplete + credentials select |
| `ui/src/features/proxyVM/pages/ProxyVMPage.tsx` | New — DataGrid + retry/delete actions + polling |
| `ui/src/features/proxyVM/index.ts` | New — barrel export |
| `ui/src/features/migration/MigrationForm.tsx` | Modified — HotAdd in storageCopyMethod union + proxyVMRef field |
| `ui/src/features/migration/NetworkAndStorageMappingStep.tsx` | Modified — HotAdd option + ProxyVM selector |
| `ui/src/App.tsx` | Modified — proxy-vms route |
| `ui/src/config/navigation.tsx` | Modified — Proxy VMs nav entry |

---

## Complexity Tracking

No constitution violations. No additional complexity entries needed.

---

## Implementation Notes (Post-Plan Decisions)

The following decisions were made during implementation that differ from or extend the original plan:

### Cold-Only Enforcement (New — FR-017)
HotAdd was determined to require cold (powered-off) migration during implementation. Two enforcement layers were added:
1. **Controller layer** (`migrationplan_controller.go`): Returns an error when `StorageCopyMethod == HotAdd` AND `MigrationType == "hot"` or `"mock"`.
2. **UI layer** (`MigrationOptionsAlt.tsx`): `useEffect` forces `dataCopyMethod` to `"cold"` and disables hot/mock menu items when HotAdd is selected.

### Power-Off Before Snapshot (New — FR-018, FR-019)
The original plan had `takeVMSnapshot()` as a self-contained function. During implementation:
- Power-off logic was placed in `HotAddCopyDisks()` step 1 (separate from `takeVMSnapshot` per explicit design decision)
- Uses `utils.DoRetryWithExponentialBackoff(ctx, checkPowerState, MaxPowerOffRetryLimit=3, PowerOffRetryCap=5*min)` to verify `VirtualMachinePowerStatePoweredOff`
- Snapshot uses `quiesce=true, memory=false` (original plan said `quiesce=false`)

### Migration Phase Tracking Fix (New — FR-020)
Root cause: `SetupMigrationPhase()` in `migration_controller.go` maps k8s event messages (string contains) to `VMMigrationPhase` values. HotAdd event message constants were not registered, so migrations stayed at `Validating` through the entire data copy.

Fix: Added `VMMigrationPhaseSnapshottingSourceVM/AttachingDisksToProxy/IdentifyingBlockDevices/HotAddTransferring/HotAddCleanup` as typed `VMMigrationPhase` constants in `migration_types.go`, added them to `VMMigrationStatesEnum` in `constants.go`, and wired 6 new case blocks in `SetupMigrationPhase()`.

Ordering in `VMMigrationStatesEnum`: HotAdd phases share numeric slots with SAC phases (5–7) since they are mutually exclusive code paths; `HotAddTransferInProgress` = 11 (same as `CopyingBlocks`); `HotAddCleanup` = 12 (before `ConvertingDisk`=15).

### T022 — Disk Count Tracking Moved to v2v-helper
Controller-side `incrementProxyVMDiskCount`/`decrementProxyVMDiskCount` helpers were removed. Instead, `adjustProxyDiskCount(ctx, delta)` in `hotadd_copy.go` handles both increment (after disks are attached, step 6) and decrement (inside `cleanupHotAdd` via `defer`). Uses optimistic locking with 3 retries on k8s conflict. The pre-flight capacity check in the controller still reads `AttachedDiskCount` to gate new migrations.

### ProxyVM Controller Refactor
Three helpers extracted for lint compliance (gocyclo limit 30):
- `isDiskEnableUUIDSet(extraConfig) bool` — pure function
- `setDiskEnableUUIDAndReboot(ctx, proxyVM, vmObj) (ctrl.Result, error)` — method, logs status update failure at V(1) before returning RequeueAfter
- `parseComponentCheckOutput(output) ([]ComponentCheck, []missing, bool)` — pure parser

### ProxyVM Delete UX Fix
React Query optimistic update pattern added to `doDelete` in `ProxyVMPage.tsx`:
- `onMutate`: immediately remove VM from cache
- `onError`: restore previous cache state
- Rows additionally filtered by `!vm.metadata.deletionTimestamp` (finalizer pattern: object stays in API while controller processes deletion)
- `deletionTimestamp?: string` added to `ProxyVM.metadata` interface in `model.ts`
