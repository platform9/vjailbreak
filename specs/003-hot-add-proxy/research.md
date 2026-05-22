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

**Decision**: During ProxyVM onboarding, the controller generates an SSH keypair and stores the private key in a Kubernetes Secret named `"{proxyVMK8sName}-hot-add-ssh-key"` (constant suffix `HotAddSSHSecretSuffix`). The corresponding public key is stored in the same secret. The v2v-helper reads the private key via `k8sutils.GetHotAddPrivateKey(ctx, k8sClient, proxyVMK8sName)` at migration time.

**Rationale**: Per-ProxyVM secrets allow key rotation and scoping without touching the appliance's own identity key. The secret is created during ProxyVM verification (controller side) and consumed by the v2v-helper at migration time. The operator copies the public key to the ProxyVM's `authorized_keys` during initial setup.

**Alternatives considered**:
- Use the appliance's own key at `/root/.ssh/id_rsa`: initially considered but rejected — using a dedicated per-ProxyVM keypair is more secure and doesn't tie ProxyVM access to the appliance's primary identity key.

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

**Decision**: Disk count tracking is split across two layers:
1. **Pre-flight check** (`migrationplan_controller`): checks `ProxyVM.Status.AttachedDiskCount + pendingDisks <= 60` before scheduling.
2. **Runtime tracking** (v2v-helper `hotadd_copy.go`): `adjustProxyDiskCount(ctx, delta)` increments count after disks are attached, decrements in `cleanupHotAdd`. Uses optimistic locking with 3 retries on k8s conflict.

**Rationale**: The controller-side check is best-effort (race window exists between check and attach). The v2v-helper is the authoritative incrementer since it knows exactly when disks are attached. Decrement on cleanup ensures the count returns to 0 even on migration failure.

---

## Decision 9: Snapshot Naming

**Decision**: Snapshot name = fixed constant `"vjailbreak-hotadd-snap"`. If a snapshot with this name already exists on the source VM, delete it first (per spec clarification Q2).

**Rationale**: HotAdd is cold-only (VM is powered off), so only one Hot-Add migration can run against a given source VM at a time — no cross-migration collision risk. A fixed name simplifies cleanup: the controller can always delete by the same constant name without needing migration-name context.

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
- Check each component individually: `which qemu-nbd`, `which sshd` — `which` is POSIX and present everywhere; parse exit code in Go (0 = found, non-0 = missing). One SSH command per component.
- `lsblk` and `nbdkit` are not required: block device identification uses `/sys/block/*/device/wwid` directly; NBD serving uses `qemu-nbd` only.

---

## Deferred Edge Cases (From Clarification Session)

| Edge Case | Disposition |
|-----------|-------------|
| Port exhaustion (>60 disks, all ports taken) | Bounded by 60-disk limit; port range covers all possible concurrent disks |
| Source VM with no disks | Existing pre-migration guard in controller validates VM has disks before scheduling |

---

## Decision 11: Cold-Only Constraint for HotAdd

**Decision**: HotAdd migration type is restricted to `cold` only. Controller blocks `hot` and `mock` migration types. UI enforces this via `useEffect`.

**Rationale**: HotAdd requires the source VM to be powered off before snapshotting to guarantee disk consistency. Hot migration (live with CBT) is incompatible with the freeze-snapshot-attach workflow. Enforcing this at both the controller and UI layers prevents invalid migrations from starting.

**Power-off sequence**: `VMPowerOff()` → `DoRetryWithExponentialBackoff(checkPowerState, 3 retries, 5-min cap)` → `takeVMSnapshot(ctx, name)` (quiesce=true). Snapshot does not include memory (`memory=false`).

---

## Decision 12: Migration Phase Tracking

**Decision**: `SetupMigrationPhase()` in `migration_controller.go` maps HotAdd event message constants to `VMMigrationPhase` typed constants. HotAdd phases share numeric slots in `VMMigrationStatesEnum` with equivalent SAC phases since the two paths are mutually exclusive.

**Rationale**: Without this fix, HotAdd migrations showed "Validating" for the entire data copy duration. The controller defaults to `VMMigrationPhaseValidating` when a pod is running but no recognized event message has been emitted. Adding the 6 HotAdd event message → phase mappings makes the UI accurately reflect: `SnapshottingSourceVM → AttachingDisksToProxy → IdentifyingBlockDevices → HotAddTransferInProgress → HotAddCleanup`.

**Implementation**: New typed `VMMigrationPhase` constants added to `migration_types.go`; `+kubebuilder:validation:Enum` tag updated; phases added to `VMMigrationStatesEnum`. Requires `make generate` to regenerate CRD YAML with new enum values.
