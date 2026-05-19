---
description: "Full task list for Hot-Add Proxy Migration feature (UI + Backend)"
---

# Tasks: Hot-Add Proxy Migration

**Feature**: `1944-hot-add-proxy`  
**Branch**: `1944-hot-add-proxy`  
**Reference patterns**: ESXi SSH Keys · SAM copy (`vaai_copy.go`) · StorageAcceleratedCopy validation block in `migrationplan_controller.go`

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this belongs to
- Exact file paths in every description

---

## Phase 1: Setup — API Client ✅

- [X] T001 Create `ui/src/api/proxy-vm/model.ts` with `ProxyVM`, `ProxyVMList`, `ProxyVMComponentCheck` TypeScript interfaces
- [X] T002 Create `ui/src/api/proxy-vm/proxyVm.ts` with `getProxyVMs`, `getProxyVM`, `createProxyVM`, `deleteProxyVM`
- [X] T003 Create `ui/src/api/proxy-vm/index.ts` barrel re-exports

---

## Phase 2: Foundational — Routing & Navigation ✅

- [X] T004 Add `<Route path="proxy-vms" element={<ProxyVMPage />} />` to `/dashboard` nested routes in `ui/src/App.tsx`
- [X] T005 Add "Proxy VMs" nav entry to `ui/src/config/navigation.tsx` after `esxi-ssh-keys`

---

## Phase 3: User Story 1 — ProxyVM Management Page ✅

- [X] T006 [P] [US1] Create `ui/src/features/proxyVM/components/AddProxyVMDialog.tsx` with SSH key prerequisite Alert, VM Name TextField, VMware Credentials Select (validated creds only), and `createProxyVM` call on submit
- [X] T007 [US1] Create `ui/src/features/proxyVM/pages/ProxyVMPage.tsx` with DataGrid (Name, VM Name, Status Chip, IP, Attached Disks, Age, Actions), "Add Proxy VM" toolbar button, `refetchInterval: 5000` polling while Pending/Verifying
- [X] T008 [US1] Add `ComponentsVerifiedTooltip` sub-component in `ProxyVMPage.tsx` showing ✓/✗ per component on VerificationFailed rows

---

## Phase 4: User Story 2 — Hot-Add in Migration Form ✅

- [X] T009 [P] [US2] Extend `storageCopyMethod` union to include `'HotAdd'`, add `proxyVMRef?: string` to `FormValues`, add HotAdd branch in template build and form validation in `ui/src/features/migration/MigrationForm.tsx`
- [X] T010 [US2] Append `{ value: 'HotAdd', label: 'Hot-Add via Proxy VM' }` to copy method options; add ProxyVM Select below StorageMapping table when HotAdd is selected in `ui/src/features/migration/NetworkAndStorageMappingStep.tsx`

---

## Phase 5: User Story 4 — ProxyVM Lifecycle ✅

- [X] T011 [US4] Add delete confirmation Dialog and success/error Snackbar; add "Last Validated" column with relative time in `ui/src/features/proxyVM/pages/ProxyVMPage.tsx`

---

## Phase 6: UI Polish ✅

- [X] T012 [P] Create `ui/src/features/proxyVM/index.ts` barrel export for `ProxyVMPage`
- [X] T013 [P] Verify no TypeScript errors: run `cd ui && yarn tsc --noEmit` and fix any issues in modified files
- [X] T014 Smoke-test normal and StorageAcceleratedCopy paths still render correctly after `NetworkAndStorageMappingStep.tsx` changes

---

## Phase 7: Backend Data Structures

**Purpose**: Thread HotAdd parameters from ConfigMap through MigrationParams into the Migrate struct so `hotadd_copy.go` can access Proxy VM coordinates.

- [X] T015 In `v2v-helper/migrate/migrate.go` (line ~81, after `ArrayCredsMapping string`), add two fields to the `Migrate` struct:
  ```go
  ProxyVMIP   string
  ProxyVMName string
  ```

- [X] T016 In `v2v-helper/pkg/utils/vcenterutils.go` (line ~15, `MigrationParams` struct, after `ArrayCredsMapping string`), add:
  ```go
  ProxyVMIP   string
  ProxyVMName string
  ```

- [X] T017 In `GetMigrationParams()` in `v2v-helper/pkg/utils/vcenterutils.go` (after the `ArrayCredsMapping` read), read the two new keys:
  ```go
  params.ProxyVMIP   = configMap.Data["PROXY_VM_IP"]
  params.ProxyVMName = configMap.Data["PROXY_VM_NAME"]
  ```

- [X] T018 In `v2v-helper/main.go` (lines ~168–178, Migrate struct literal), wire in the new fields:
  ```go
  ProxyVMIP:   migrationparams.ProxyVMIP,
  ProxyVMName: migrationparams.ProxyVMName,
  ```

---

## Phase 8: MigrationPlan Controller Changes

**Purpose**: Validate HotAdd prerequisites at plan-time, populate the ConfigMap for the v2v-helper pod, and track per-ProxyVM disk counts.

- [X] T019 Add RBAC marker above the `MigrationPlanReconciler` struct in `k8s/migration/internal/controller/migrationplan_controller.go`:
  ```go
  // +kubebuilder:rbac:groups=vjailbreak.k8s.pf9.io,resources=proxyvms,verbs=get;list;watch;update;patch
  ```
  Then run `make generate` inside `k8s/migration/` to regenerate RBAC manifests.

- [X] T020 In `k8s/migration/internal/controller/migrationplan_controller.go`, after the `StorageAcceleratedCopy` validation block (~line 670), add a HotAdd validation block that:
  1. Fetches the `ProxyVM` named by `migrationtemplate.Spec.ProxyVMRef.Name`
  2. Returns an error/condition if `proxyVM.Status.ValidationStatus != "Ready"`
  3. Returns an error/condition if `proxyVM.Status.AttachedDiskCount + len(sourceDisks) > 60`

- [X] T021 In `setOSFamilyAndStorageFields()` in `k8s/migration/internal/controller/migrationplan_controller.go` (~line 1604), add a HotAdd `else if` branch after the StorageAcceleratedCopy branch that sets:
  ```go
  configMapData["STORAGE_COPY_METHOD"] = string(constants.HotAddCopyMethod)
  configMapData["PROXY_VM_IP"]   = proxyVM.Status.IPAddress
  configMapData["PROXY_VM_NAME"] = proxyVM.Spec.VMName
  ```
  Fetch the ProxyVM resource using `migrationtemplate.Spec.ProxyVMRef.Name`.

- [X] T022 Add `incrementProxyVMDiskCount` and `decrementProxyVMDiskCount` helpers in `k8s/migration/internal/controller/migrationplan_controller.go` that patch `status.attachedDiskCount` on the ProxyVM resource; call increment when a HotAdd migration transitions to running, decrement on completion or failure.

---

## Phase 9: hotadd_copy.go — New File

**Purpose**: All Hot-Add data copy logic. Modelled on `v2v-helper/migrate/vaai_copy.go`.  
**File to create**: `v2v-helper/migrate/hotadd_copy.go`

- [X] T023 Create `v2v-helper/migrate/hotadd_copy.go` with package declaration, imports, and the `hotAddDiskTransfer` struct:
  ```go
  type hotAddDiskTransfer struct {
      BlockDevice     string  // /dev/sdX on the Proxy VM
      DestDevice      string  // /dev/sdX on the vJailbreak appliance
      SnapshotVMDKPath string // frozen parent VMDK path
      DiskKey         int32  // vCenter device key for detach
      WWID            string // normalised UUID (no dashes, lowercase)
      NBDPort         int
      NBDPid          int
  }
  ```

- [X] T024 Implement `takeVMSnapshot(ctx context.Context, vcClient *govmomi.Client, sourceVMName, snapshotName string) error` in `hotadd_copy.go`:
  - Use govmomi to call `snapshot.Create` with `memory=false`, `quiesce=false`
  - If a snapshot with the same name already exists, remove it first with `snapshot.Remove`

- [X] T025 Implement `getFrozenVMDKs(ctx context.Context, vcClient *govmomi.Client, sourceVMName string) ([]hotAddDiskTransfer, error)` in `hotadd_copy.go`:
  - Enumerate `VirtualDisk` devices from the VM config
  - For each disk, if `backing.Parent != nil`, use `backing.Parent.FileName`; otherwise use `backing.FileName`
  - Return one `hotAddDiskTransfer` per disk with `SnapshotVMDKPath` and `DiskKey` populated

- [X] T026 Implement `attachDiskToProxy(ctx context.Context, vcClient *govmomi.Client, proxyVMName, datastoreName, diskPath string) error` in `hotadd_copy.go`:
  - Use govmomi `vm.AddDevice` to attach the frozen VMDK to the Proxy VM
  - Set mode to `VirtualDiskMode_independent_nonpersistent`
  - Return the updated device key

- [X] T027 Implement `identifyBlockDevices(sshClient *ssh.Client, transfers []hotAddDiskTransfer, vcClient *govmomi.Client, proxyVMName string) error` in `hotadd_copy.go`:
  - Via SSH: `for d in /sys/block/sd*; do w=$(cat $d/device/wwid 2>/dev/null); case "$w" in naa.*) echo "$(basename $d)|${w#naa.}";; esac; done`
  - Via govmomi: `device.info -json 'disk-*'` on the Proxy VM, extract `backing.uuid`, strip dashes and lowercase
  - Match each transfer's WWID to a block device; set `transfer.BlockDevice = "/dev/" + matched`
  - Retry up to 3 times with 5-second sleep between attempts (disk may take a moment to appear in guest)

- [X] T028 Implement `findFreePort(sshClient *ssh.Client, rangeMin, rangeMax int) (int, error)` in `hotadd_copy.go`:
  - Via SSH: `cat /proc/net/tcp /proc/net/tcp6`
  - Parse each line's local_address field (column 2): hex `XXXXXXXX:PPPP` — extract port from last 4 hex digits
  - Build a set of used ports; return first port in `[rangeMin, rangeMax]` not in the set

- [X] T029 Implement `serveViaNBD(sshClient *ssh.Client, blockDevice string, port int) (pid int, err error)` in `hotadd_copy.go`:
  - Via SSH run: `qemu-nbd --format=raw --port=<port> --bind=0.0.0.0 --fork --persistent <blockDevice>`
  - The `--fork` flag makes qemu-nbd print the child PID to stdout then exit; capture and parse the PID
  - Return the PID so cleanup can kill it later

- [X] T030 Implement `runNBDCopy(ctx context.Context, proxyIP string, port int, destDevice string) error` in `hotadd_copy.go`:
  - Execute locally: `nbdcopy nbd://<proxyIP>:<port> <destDevice>`
  - Retry up to 3 times with 10-second backoff on non-zero exit; log stderr on each attempt
  - Return nil on success, wrapped error after 3 failures

- [X] T031 Implement `cleanupHotAdd(sshClient *ssh.Client, transfers []hotAddDiskTransfer, vcClient *govmomi.Client, proxyVMName, sourceVMName, snapshotName string)` in `hotadd_copy.go`:
  - For each transfer with `NBDPid > 0`: SSH `kill <pid>` (ignore errors — process may already be gone)
  - For each transfer with `DiskKey != 0`: govmomi `vm.RemoveDevice` on the Proxy VM using the device key
  - Call govmomi `snapshot.Remove` on the source VM for `snapshotName`
  - Log but do not fail on individual cleanup errors

- [X] T032 Implement `HotAddCopyDisks(ctx context.Context, migobj *Migrate, vminfo vm.VMInfo) error` in `hotadd_copy.go`:
  - Orchestrates T024–T031 in order: takeVMSnapshot → getFrozenVMDKs → attachDiskToProxy (per disk) → identifyBlockDevices → per disk: findFreePort + serveViaNBD + runNBDCopy
  - `defer cleanupHotAdd(...)` immediately after snapshot is created
  - Log progress at each step with disk name and sizes

---

## Phase 10: migrate.go Integration

**Purpose**: Wire the HotAdd code path into the existing migration execution loop.

- [X] T033 In `v2v-helper/migrate/migrate.go` at line ~1862 (the disk count check that skips for SAM), extend the condition to also skip for HotAdd:
  ```go
  // Before:
  if migobj.StorageCopyMethod != constants.StorageCopyMethod {
  // After:
  if migobj.StorageCopyMethod != constants.StorageCopyMethod &&
     migobj.StorageCopyMethod != constants.HotAddCopyMethod {
  ```

- [X] T034 In `v2v-helper/migrate/migrate.go` at line ~1881 (the `if migobj.StorageCopyMethod == constants.StorageCopyMethod` branch for SAM), add an `else if` branch for HotAdd:
  ```go
  } else if migobj.StorageCopyMethod == constants.HotAddCopyMethod {
      if err := CreateVolumes(ctx, migobj, vminfo); err != nil {
          return err
      }
      if err := AttachVolumes(ctx, migobj, vminfo); err != nil {
          return err
      }
      if err := HotAddCopyDisks(ctx, migobj, vminfo); err != nil {
          return err
      }
  }
  ```
  (CreateVolumes and AttachVolumes create and attach the destination volumes on OpenStack, same as the normal path.)

---

## Phase 11: Constants & CRD

- [X] T035 Verify `constants.HotAddCopyMethod` is defined in `pkg/common/constants/constants.go` (add `HotAddCopyMethod StorageCopyMethodType = "HotAdd"` if missing)
- [X] T036 Verify `MigrationTemplate` CRD spec includes `proxyVMRef` field; if not present, add it to `k8s/migration/api/v1alpha1/migrationtemplate_types.go` and run `make generate` inside `k8s/migration/`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 7**: Independent — start immediately
- **Phase 8**: Depends on Phase 7 data structures being clear (can start in parallel)
- **Phase 9**: Depends on Phase 7 (uses `Migrate` struct fields) — must have T015 done
- **Phase 10**: Depends on Phase 9 (calls `HotAddCopyDisks`) — must have T032 done
- **Phase 11**: Independent — can run in parallel with Phase 7

### Critical Path

```
T015/T016/T017/T018  →  T032  →  T033/T034   (data flow: ConfigMap → Migrate → hotadd_copy → migrate.go)
T019/T020/T021/T022             (controller changes, independent of v2v-helper)
T023 → T024 → T025 → T026 → T027 → T028 → T029 → T030 → T031 → T032
T035/T036                       (constants/CRD, unblock everything)
```

### Parallel Opportunities

```
[Thread A] T035 → T036 → T015 → T016 → T017 → T018 → T033 → T034
[Thread B] T019 → T020 → T021 → T022
[Thread C] T023 → T024 → T025 → T026 → T027 → T028 → T029 → T030 → T031 → T032
```

Threads A and C converge at T033/T034.

---

## Implementation Notes

### SSH client in hotadd_copy.go
Use `golang.org/x/crypto/ssh` (already a transitive dep). Create the client with the same key-path pattern used by `proxyvm_controller.go` (`/home/ubuntu/.ssh/id_rsa`). Pass it into each function rather than re-opening.

### qemu-nbd PID capture
`qemu-nbd --fork` prints the daemon PID to stdout on the background process line. Parse with `strconv.Atoi(strings.TrimSpace(out))`.

### Port range for NBD
Use `10809–10909` (100 ports). Port 10809 is the IANA-registered NBD port; the range avoids conflicts with common services.

### govmomi disk detach
Use `object.VirtualMachine.RemoveDevice(ctx, false, device)` — `keepFiles=false` would delete the VMDK; since these are snapshot reference disks we must NOT delete the file, so pass `keepFiles=true`.

### Error propagation
`cleanupHotAdd` is called via `defer` and must not panic. Use `log.Error` for each sub-step failure and continue cleanup.
