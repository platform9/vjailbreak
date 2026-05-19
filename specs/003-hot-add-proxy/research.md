# Research: Hot-Add Proxy Migration

**Feature**: Hot-Add Proxy Migration  
**Branch**: `1944-hot-add-proxy`  
**Date**: 2026-05-19

---

## Decision 1: ProxyVM as a First-Class CRD

**Decision**: Introduce a `ProxyVM` CRD in `k8s/migration/api/v1alpha1/proxyvm_types.go`.

**Rationale**: The `ESXiSSHCreds` CRD is the closest analogue — it represents an external resource that must be validated for SSH connectivity and tracks per-resource validation status. ProxyVM follows the same pattern: a single registered resource, validated once, reused across migrations. Using a CRD keeps all state Kubernetes-native (constitution principle I).

**Spec fields**: `VMName string` (name in vCenter), `VMwareCredsRef LocalObjectReference` (which vCenter to look the VM up in).  
**Status fields**: `ValidationStatus string`, `ValidationMessage string`, `IPAddress string` (discovered), `LastValidationTime *metav1.Time`, `AttachedDiskCount int` (tracked atomically for the 60-disk limit).

**Alternatives considered**:
- Store ProxyVM config in a ConfigMap: rejected — no validation lifecycle, no status tracking.
- Store in MigrationTemplate: rejected — ProxyVM is a long-lived global resource, not per-migration config.

---

## Decision 2: Extend StorageCopyMethod Enum

**Decision**: Extend the existing `StorageCopyMethod` string field on `MigrationTemplateSpec` by adding `"HotAdd"` as a third enum value alongside `"normal"` and `"StorageAcceleratedCopy"`.

**Rationale**: Keeps copy-method selection in a single field, consistent with the SAM pattern. All existing switch/if branches on `StorageCopyMethod` remain unchanged; only a new `else if` arm is added. Minimal blast radius to existing code (constitution principle: existing code paths must not be broken).

**New MigrationTemplate field**: Add `ProxyVMRef LocalObjectReference` (optional, only required when `StorageCopyMethod == "HotAdd"`).

**Alternatives considered**:
- New separate `DataCopyMethod` field: rejected — creates two overlapping fields; SAM already established `StorageCopyMethod` as the canonical selector.

---

## Decision 3: New v2v-helper File `hotadd_copy.go`

**Decision**: Add `v2v-helper/migrate/hotadd_copy.go` alongside the existing `vaai_copy.go`.

**Rationale**: SAM copy is isolated in `vaai_copy.go` with a single entry point `StorageAcceleratedCopyCopyDisks()`. Hot-Add copy follows the same isolation pattern: all vCenter snapshot/attach/detach operations plus SSH-based block device identification and qemu-nbd lifecycle live in one file. This makes the new code auditable without touching SAM code paths.

**Entry point**: `HotAddCopyDisks(ctx, vcClient, sshClient, proxyVMIP, sourceVMName, snapshotName string, disks []SourceDisk, destDevices []string) error`

**Alternatives considered**:
- Inline in `migrate.go`: rejected — `migrate.go` is already large; isolation is the established pattern.

---

## Decision 4: SSH Access Strategy for ProxyVM

**Decision**: The v2v-helper pod uses the vJailbreak appliance's own SSH private key (at `/root/.ssh/id_rsa` inside the pod, same path used for ESXi operations) to SSH into the ProxyVM. No Kubernetes Secret is needed for ProxyVM SSH credentials.

**Rationale**: The spec explicitly states the operator pre-authorizes the vJailbreak public key on the ProxyVM. The private key is already present in the vJailbreak appliance's filesystem and mounted into pods. The `esxi-ssh/client.go` package already implements this pattern for ESXi host SSH. Reuse it.

**Alternatives considered**:
- Store ProxyVM SSH key in a Kubernetes Secret (like `ESXiSSHCreds`): rejected — the key is the appliance's own key, not a user-supplied credential; adding a SecretRef would be redundant.

---

## Decision 5: Port Allocation for qemu-nbd

**Decision**: Dynamic port allocation from range **10809–11808** (1,000 ports). The Go code SSHs to the ProxyVM and runs a single `cat` command; all iteration and selection logic runs in Go.

**SSH command** (single read, no bash logic):
```
cat /proc/net/tcp /proc/net/tcp6
```

**Go parsing**: Read output line by line using `bufio.Scanner`. Each data line has local-address in field 1 as `XXXXXXXX:PPPP` (hex). Extract the port hex from after the colon, parse with `strconv.ParseInt(..., 16, 32)`, build a `map[int]struct{}` of occupied ports. Then iterate `10809..11808` in Go to find the first port not in the map.

**Rationale**: `/proc/net/tcp` is guaranteed on every Linux kernel (procfs). No external tool dependency. All logic in Go — no bash loops, no `printf`, no `grep`. The SSH round-trip is a single `cat` command.

**Alternatives considered**:
- `ss -tlnp`: rejected — `ss` (iproute2) may not be installed on minimal Linux Proxy VMs.
- `netstat -tlnp`: rejected — `netstat` (net-tools) is deprecated and absent from many modern minimal images.
- Bash loop with `grep`: rejected — shell logic should stay in Go per project preference.
- Fixed port-per-slot: rejected — requires coordination across pods; dynamic scan in Go is simpler.

---

## Decision 6: Block Device Identification

**Decision**: Match vCenter disk UUID to guest block device via the `naa.*` wwid in `/sys/block/*/device/wwid`. All parsing in Go; minimal single-purpose SSH commands.

**Step 1 — Get UUIDs from vCenter (Go, no SSH)**:  
Use govmomi `property.DefaultCollector` to fetch `config.hardware.device` for the ProxyVM. Iterate `VirtualDisk` devices, extract `Backing.Uuid`, normalize to lowercase hex without dashes.

**Step 2 — Read wwids from ProxyVM (SSH, one command)**:
```
find /sys/block -maxdepth 4 -name wwid
```
Go parses the output: split by newline to get a list of absolute wwid file paths (e.g., `/sys/block/sdb/device/wwid`).

**Step 3 — Read each wwid file (SSH, batched)**:  
Concatenate all paths into a single `cat` command:
```
cat /sys/block/sdb/device/wwid /sys/block/sdc/device/wwid ...
```
Go reads the multi-line output; line `N` corresponds to path `N` from step 2. Normalize each wwid: strip `naa.` prefix, lowercase, remove dashes.

**Step 4 — Match (Go)**:  
Compare normalized UUID map against normalized wwid map; resolve `label → /dev/sdX`. Retry the full sequence up to 3 times with 5-second waits if any UUID has no match (FR-008).

**Alternatives considered**:
- Bash `for d in /sys/block/sd*; do cat $d/device/wwid; done`: rejected — bash logic should be in Go.
- `lsblk -o +WWN -J` (JSON output): rejected — `lsblk` may use a different wwid format and requires parsing a larger JSON blob; direct procfs read is simpler and tool-independent.

---

## Decision 7: Cleanup Strategy

**Decision**: Cleanup is a separate `cleanup()` function in `hotadd_copy.go` called via `defer`. Steps: (1) kill qemu-nbd processes on ProxyVM via SSH using `kill <pid>` (PID tracked in Go from the `qemu-nbd` start output), (2) detach disks from ProxyVM via govmomi `vm.disk.detach`, (3) delete snapshot via govmomi `snapshot.remove`. All vCenter operations use govmomi Go APIs; SSH is used only to send `kill <pid>` — no shell scripts.

**PID tracking**: When `qemu-nbd` is started with `--fork`, it prints the child PID to stdout. Go captures and stores this PID per disk. Cleanup sends `kill <pid>` via SSH — a minimal command with no bash logic.

**Rationale**: Best-effort cleanup per spec assumption. Using `defer` ensures cleanup runs even on panic. All bash-avoidable operations (vCenter disk detach, snapshot delete) go through govmomi. The only SSH in cleanup is `kill <pid>` — one integer, one command, no parsing needed.

---

## Decision 8: Concurrency and the 60-Disk Limit

**Decision**: The `migrationplan_controller` checks `ProxyVM.Status.AttachedDiskCount + pendingDisks <= 60` before scheduling a Hot-Add migration. The count is incremented in the ProxyVM status before creating migration pods and decremented after cleanup. Uses Kubernetes optimistic locking (status update retry on conflict).

**Rationale**: The vSphere hardware limit is the binding constraint. Tracking count in ProxyVM status keeps the check Kubernetes-native. The migrationplan_controller already serializes per-plan reconciliation, reducing race window.

---

## Decision 9: Snapshot Naming

**Decision**: Snapshot name = `"vjailbreak-hotadd-<migration-name>"`. If a snapshot with this name already exists on the source VM, delete it first (per spec clarification Q2).

**Rationale**: Deterministic snapshot name makes cleanup idempotent — a crashed migration can be retried and will clean up the stale snapshot. Including `migration-name` scopes it to the specific migration, avoiding cross-migration collisions.

---

## Decision 10: Go-First SSH Interaction Principle

**Decision**: All ProxyVM interactions use Go's `golang.org/x/crypto/ssh` client. SSH commands sent to the ProxyVM must be **minimal single-purpose reads** (e.g., `cat <file>`, `find <path> -name <file>`, `kill <pid>`). All iteration, matching, filtering, and branching logic lives in Go — not in shell scripts or pipelines on the remote host.

**Rules**:
1. No shell loops (`for`, `while`) in SSH command strings.
2. No shell pipelines (`|`) in SSH command strings.
3. No shell conditionals (`if`, `&&`, `||`) in SSH command strings.
4. Prefer reading raw kernel/procfs files (`/proc/net/tcp`, `/sys/block/*/device/wwid`) over invoking userspace tools — no tool dependency risk.
5. When a userspace command is unavoidable (e.g., `qemu-nbd`, `kill`), use it directly with explicit arguments — no shell wrapping.
6. Parse all remote output in Go (`bufio.Scanner`, `strings.Fields`, `strconv.ParseInt`).

**Rationale**: Minimal bash in SSH commands reduces dependency on shell features and external tools that may not be present on minimal Linux guests. Go parsing is testable with unit tests (mock SSH output as strings); shell scripts in SSH commands are not. The existing `esxi-ssh/client.go` already runs individual commands and captures output — this pattern scales to ProxyVM operations.

**Applies to**: port scan (Decision 5), block device identification (Decision 6), qemu-nbd start/stop (Decision 7), component verification in ProxyVM controller.

**ProxyVM controller component checks** (updated to match this principle):
- Check each component individually: SSH `command -v lsblk`, `command -v qemu-nbd`, `command -v nbdkit`, `command -v sshd || systemctl is-active sshd`
  → Actually: run `which lsblk` etc. — `which` is POSIX and present everywhere; parse exit code in Go (0 = found, non-0 = missing). One SSH command per component.

---

## Deferred Edge Cases (From Clarification Session)

| Edge Case | Disposition |
|-----------|-------------|
| Port exhaustion (>60 disks, all ports taken) | Bounded by 60-disk limit; port range covers all possible concurrent disks |
| Source VM with no disks | Existing pre-migration guard in controller validates VM has disks before scheduling |
