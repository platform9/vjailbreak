# Developer Quickstart: Hot-Add Proxy Migration

**Branch**: `1944-hot-add-proxy`

---

## Overview

Hot-Add is a new data copy method. Only the data-copy phase changes; all other migration phases are unchanged.

Code changes span three Go modules (independent — run commands from each module directory) and the React UI.

---

## Module Locations

| Module | Directory | Test command |
|--------|-----------|-------------|
| Controller | `k8s/migration/` | `make test` |
| v2v-helper | `v2v-helper/` | `make test-v2v-helper` (Linux only) |
| Common constants | `pkg/common/` | `go test ./...` |
| UI | `ui/` | `yarn test` |

---

## Implementation Order

Follow this order to avoid compilation breaks between modules:

### Step 1 — Common constants (`pkg/common/`)

Add to `pkg/common/constants/constants.go`:
- `HotAddCopyMethod = "HotAdd"`
- `ProxyVMStatusPending/Verifying/Ready/VerificationFailed`
- `ProxyVMMaxAttachedDisks = 60`
- `HotAddPortRangeMin/Max = 10809/11808`
- Event message constants (see data-model.md)
- Migration phase constants for Hot-Add phases

Then run `go mod tidy` in `pkg/common/`.

### Step 2 — ProxyVM CRD (`k8s/migration/`)

1. Create `k8s/migration/api/v1alpha1/proxyvm_types.go` (see data-model.md for struct definitions)
2. Add `ProxyVMRef *corev1.LocalObjectReference` and update `StorageCopyMethod` enum in `migrationtemplate_types.go`
3. Run `make generate` inside `k8s/migration/` to regenerate deepcopy and CRD YAML
4. Register the new type in `k8s/migration/api/v1alpha1/groupversion_info.go` (add to `SchemeBuilder.Register(...)`)
5. Add the new controller in `k8s/migration/internal/controller/proxyvm_controller.go`
6. Register the controller in `k8s/migration/cmd/main.go`
7. Run `make test` to verify

### Step 3 — Controller Hot-Add logic (`k8s/migration/`)

In `migrationplan_controller.go`:
- Add `HotAddCopyMethod` validation block (after the `StorageCopyMethod == StorageCopyMethod` block at line ~670)
- Validates ProxyVM is Ready, checks 60-disk capacity, increments AttachedDiskCount
- Populates ConfigMap with `PROXY_VM_IP`, `PROXY_VM_NAME` (alongside existing `STORAGE_COPY_METHOD`)

### Step 4 — v2v-helper Hot-Add copy (`v2v-helper/`)

1. Add `ProxyVMIP`, `ProxyVMName` fields to `MigrationParams` in `v2v-helper/pkg/utils/vcenterutils.go`
2. Extend `GetMigrationParams()` to read `PROXY_VM_IP`, `PROXY_VM_NAME` from ConfigMap
3. Add corresponding fields to the `Migrate` struct in `v2v-helper/migrate/migrate.go`
4. Create `v2v-helper/migrate/hotadd_copy.go`:
   - `HotAddCopyDisks()` entry point
   - `takeSnapshot()`, `attachSnapshotDisks()`, `detachDisks()`, `deleteSnapshot()` — all govmomi, no SSH
   - `identifyBlockDevices()` — SSH `find /sys/block -maxdepth 4 -name wwid` + SSH `cat <paths...>`; all wwid→device matching in Go; retry ×3
   - `findFreePort()` — SSH `cat /proc/net/tcp /proc/net/tcp6`; parse hex ports in Go (`bufio.Scanner` + `strconv.ParseInt`); iterate 10809-11808 in Go
   - `serveViaNBD()` — SSH `qemu-nbd --format=raw --port=<port> --bind=0.0.0.0 --fork --persistent <dev>`; Go captures PID from stdout
   - `copyDisk()` — local `nbdcopy` exec via `os/exec`, retry ×3
   - `cleanup()` — defer: SSH `kill <pid>` per running qemu-nbd; govmomi detach disks; govmomi delete snapshot
   - **Rule**: no shell loops/pipes/conditionals in SSH command strings; all logic in Go
5. In `MigrateVM()` in `migrate.go`, add `else if migobj.StorageCopyMethod == constants.HotAddCopyMethod` branch calling `HotAddCopyDisks()`

### Step 5 — UI (`ui/`)

1. Create `ui/src/api/proxy-vm/` with CRUD API client (follow `ui/src/api/esxi-ssh-creds/` pattern)
2. Create `ui/src/features/proxyVM/` management page:
   - List view with status, IP, attached-disk count
   - Add form (VMName + VMwareCreds dropdown)
   - Delete action
   - Pre-requisite SSH key setup instructions
3. Add "Proxy VMs" entry to sidebar (follow pattern of other sidebar entries)
4. In `NetworkAndStorageMappingStep.tsx`:
   - Add `{ value: 'HotAdd', label: 'Hot-Add via Proxy VM' }` to copy method dropdown
   - Add conditional rendering: when HotAdd selected, show ProxyVM selector instead of StorageMapping/ArrayCredsMapping
5. In `MigrationForm.tsx`:
   - Extend `FormValues.storageCopyMethod` type to include `'HotAdd'`
   - Add validation: when HotAdd, ProxyVM must be selected and Ready
   - Pass `proxyVMRef` to MigrationTemplate CR on form submission

---

## Key File Paths

### New Files

| File | Purpose |
|------|---------|
| `k8s/migration/api/v1alpha1/proxyvm_types.go` | ProxyVM CRD type definitions |
| `k8s/migration/internal/controller/proxyvm_controller.go` | ProxyVM verification controller |
| `v2v-helper/migrate/hotadd_copy.go` | Hot-Add data copy implementation |
| `ui/src/api/proxy-vm/index.ts` | API client for ProxyVM |
| `ui/src/features/proxyVM/ProxyVMPage.tsx` | Management page |
| `ui/src/features/proxyVM/AddProxyVMDialog.tsx` | Add dialog |

### Modified Files

| File | Change |
|------|--------|
| `pkg/common/constants/constants.go` | New Hot-Add constants |
| `k8s/migration/api/v1alpha1/migrationtemplate_types.go` | Extend enum + add ProxyVMRef |
| `k8s/migration/api/v1alpha1/groupversion_info.go` | Register ProxyVM |
| `k8s/migration/cmd/main.go` | Register ProxyVM controller |
| `k8s/migration/internal/controller/migrationplan_controller.go` | HotAdd validation + ConfigMap |
| `v2v-helper/pkg/utils/vcenterutils.go` | New MigrationParams fields |
| `v2v-helper/migrate/migrate.go` | HotAdd branch in MigrateVM |
| `ui/src/features/migration/NetworkAndStorageMappingStep.tsx` | HotAdd option + ProxyVM selector |
| `ui/src/features/migration/MigrationForm.tsx` | HotAdd form logic |
| `ui/src/components/layout/Sidebar.tsx` | Add Proxy VMs entry |

---

## After CRD Changes

Always run after changing files in `k8s/migration/api/v1alpha1/`:

```bash
cd k8s/migration && make generate
```

This regenerates `zz_generated.deepcopy.go` and updates `deploy/` CRD YAML. Never hand-edit these.

---

## Testing Checklist

- [ ] ProxyVM controller unit tests (mock vCenter + SSH clients)
- [ ] migrationplan_controller unit tests for HotAdd validation path (no ProxyVM, ProxyVM not Ready, capacity exceeded)
- [ ] `hotadd_copy.go` unit tests (mock govmomi, mock SSH client): snapshot creation, disk attachment, UUID matching, NBD serving, cleanup
- [ ] UI component tests for ProxyVM management page
- [ ] Manual E2E: register ProxyVM → verify Ready → run Hot-Add migration → confirm data integrity + no orphaned resources
