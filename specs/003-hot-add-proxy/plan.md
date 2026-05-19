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
| SSH key strategy | Reuse vJailbreak appliance key at `/home/ubuntu/.ssh/id_rsa` (constant `hotAddSSHKeyPath`); no new Secret |
| Port allocation | Dynamic scan of range 10809–11808 via `ss -tlnp` on ProxyVM |
| Block device identification | wwid/naa UUID matching via `/sys/block/sd*/device/wwid` |
| Concurrency (60-disk limit) | Track `AttachedDiskCount` in ProxyVM status; checked in migrationplan_controller |
| Snapshot naming | `vjailbreak-hotadd-<migration-name>`; delete existing before creating |
| Retry strategy | 3 retries for block device identification; 3 retries for data transfer failure |

---

## Phase 1: Design (Complete)

### Data Model

See [data-model.md](data-model.md).

**New CRDs**: `ProxyVM`  
**Modified CRDs**: `MigrationTemplate` (extended `StorageCopyMethod` enum + `ProxyVMRef` field)  
**New v2v-helper types**: `hotAddDiskTransfer` (in-memory struct in `hotadd_copy.go`), `ProxyVMIP`/`ProxyVMName` fields in `MigrationParams` and `Migrate` structs  
**New constants** (already present in `pkg/common/constants/constants.go`): `HotAddCopyMethod = "HotAdd"`, `ProxyVMStatusReady`, `ProxyVMStatusVerifying`, `ProxyVMStatusVerificationFailed`, `ProxyVMStatusPending`, `HotAddPortRangeMin = 10809`, `HotAddPortRangeMax = 11808`  
**migrate.go changes**: disk-count guard extended with `&& StorageCopyMethod != HotAddCopyMethod`; HotAdd `else if` branch: `CreateVolumes → AttachVolume per disk → HotAddCopyDisks`

### Contracts

See [contracts/crds.md](contracts/crds.md).

**New CRD**: `ProxyVM` with full YAML examples (Ready + VerificationFailed states)  
**Modified CRD**: `MigrationTemplate` with `storageCopyMethod: HotAdd` + `proxyVMRef` example  
**ConfigMap keys**: `PROXY_VM_IP`, `PROXY_VM_NAME` (alongside existing `STORAGE_COPY_METHOD`)  
**REST API**: Standard Kubernetes CRUD on `/apis/vjailbreak.k8s.pf9.io/v1alpha1/namespaces/migration-system/proxyvms`  
**Retry API**: `PATCH proxyvms/<name>` with `Content-Type: application/merge-patch+json` body `{"metadata":{"annotations":{"vjailbreak.k8s.pf9.io/retry-at":"<ISO-timestamp>"}}}` — triggers controller reconcile and re-runs full verification

### Implementation Details

#### ProxyVM Controller (`proxyvm_controller.go`)

Reconcile loop triggered by ProxyVM Create/Update:

1. Set status to `Verifying`
2. Fetch VMwareCreds → connect to vCenter via govmomi
3. Find VM by `spec.vmName` → extract guest IP from `guest.ipAddress`; set `status.ipAddress`
4. SSH to `root@<ip>` using `/root/.ssh/id_rsa` (same as existing ESXi SSH pattern)
5. Check each required component by running `which <name>` via SSH (one command per component — POSIX, present everywhere); check exit code in Go (0 = found); populate `status.componentsVerified`
6. Check `disk.EnableUUID` via govmomi `ExtraConfig`; if not set, set it and trigger VM reboot (wait for guest ready), then re-verify
7. If all checks pass → set status to `Ready`; else set `VerificationFailed` with per-component messages

#### migrationplan_controller.go changes (HotAdd block)

Insert after the `StorageAcceleratedCopy` validation block (~line 670):

```
if migrationtemplate.Spec.StorageCopyMethod == constants.HotAddCopyMethod {
    // 1. Fetch ProxyVM CR
    // 2. Check ProxyVM.Status.ValidationStatus == Ready → else block migration
    // 3. Check ProxyVM.Status.AttachedDiskCount + len(sourceDisks) <= 60 → else capacity error
    // 4. Populate configMapData["PROXY_VM_IP"] and configMapData["PROXY_VM_NAME"]
}
```

Decrement `AttachedDiskCount` in cleanup path (on migration success or failure).

#### hotadd_copy.go — `HotAddCopyDisks()`

All remote ProxyVM interactions use Go's `crypto/ssh` client. SSH commands are minimal single-purpose reads; all logic (iteration, matching, parsing) runs in Go (see Decision 10 in research.md).

```
func HotAddCopyDisks(ctx, params) error {
    defer cleanup(ctx, params, pids)    // always runs; sends "kill <pid>" per qemu-nbd

    snapshotRef = takeSnapshot(sourceVMName, snapshotName)    // govmomi
    frozenVMDKs = getFrozenVMDKs(snapshotRef)                 // govmomi device info

    for each disk in frozenVMDKs:
        attachDiskToProxyVM(disk, proxyVM)                     // govmomi

    transfers = identifyBlockDevices(proxyVM, frozenVMDKs)
    // SSH "find /sys/block -maxdepth 4 -name wwid" → Go parses paths
    // SSH "cat <path1> <path2> ..." → Go parses wwids
    // govmomi: disk UUIDs already in memory; Go normalizes and matches
    // Retry ×3 with 5s wait if any UUID unmatched

    for each transfer in transfers:
        port = findFreePort(proxyVM)
        // SSH "cat /proc/net/tcp /proc/net/tcp6" → Go parses hex ports → finds free slot

        pid = serveViaNBD(proxyVM, transfer.blockDevice, port)
        // SSH "qemu-nbd --format=raw --port=<port> --bind=0.0.0.0 --fork --persistent <dev>"
        // Go captures PID from stdout

        nbdcopy(proxyIP, port, transfer.destDevice)            // local exec, retry ×3
        pids[port] = pid
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
