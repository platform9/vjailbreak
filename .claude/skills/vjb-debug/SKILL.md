---
name: vjb-debug
description: |
  vJailbreak (VJB) VMware-to-PCD VM migration debugging. Use for: migration stuck or failed in
  any phase (discovery, mapping, validate, data copy, convert, cutover, post-migration), missing
  or wrong-version VDDK, credential/revalidation failures for VMwareCreds/OpenstackCreds, network
  or storage mapping errors, subnet mismatch or port-already-in-use during migration, nbdcopy/NFC
  copy failures, Storage-Accelerated Copy (XCOPY) failures on Pure Storage or NetApp, Hot-Add
  Proxy failures, virt-v2v conversion errors (resolv.conf immutable, dynamic disk/LDM, Hivex
  errors), Windows disks offline after migration, VMware Tools residual artifacts, admin cutover
  not progressing, Windows Failover Cluster (WSFC) issues post-migration (NetFT adapter missing,
  cluster IP unreachable), agent scaling, or Cluster Conversion (ESXi-to-PCD-hypervisor)
  problems. Keywords: vjailbreak, vjb, v2v-helper, migration-controller-manager, virt-v2v, VDDK,
  NBD, XCOPY, hot-add proxy, cutover, MigrationPlan, VMwareCreds, OpenstackCreds.
---

# vJailbreak Debugging

## Overview

vJailbreak migrates VMs from VMware vSphere to Platform9 PCD. It spans three distinct domains, and a failure can originate in any of them:

- **VMware side** — vCenter/ESXi: credentials, permissions, network port-groups, datastores, guest-OS state inside the source VM.
- **vJailbreak VM side** — a single VM running k3s in the target PCD environment: the `migration-controller-manager`, `vjailbreak-ui`, and one `v2v-helper` pod per in-flight migration.
- **PCD/OpenStack side** — the destination: Nova (boot), Neutron (networking/ports), Cinder (volumes).

**The single most important correlation ID is the migration/VM name.** It threads through the `Migration` CRD's name, the `<migration-name>-v2v-helper` pod, and the debug log file `/var/log/pf9/<migration-name>.log`. Get it first — from the UI's migration list or `kubectl get migration -n migration-system`.

## Architecture / Flow

```
Discovery → Mapping → Validate → Data Copy → Convert → [Cutover] → Post-Migration → Completed
   (VMware)   (both)   (VMware)   (3 methods:      (virt-v2v,       (admin or       (rename/
                                   NFC/XCOPY/       guest-OS         scheduled)      move/script/
                                   hot-add-proxy)   quirks)                          persist-net)
```

Data Copy has three interchangeable transports (Normal NFC, Storage-Accelerated XCOPY, Hot-Add Proxy) layered under the same phase — see [copy-methods.md](copy-methods.md). Cutover only applies when Admin-Initiated cutover was selected.

Full detail: [migration-lifecycle.md](migration-lifecycle.md).

## Tool Availability

| Tool | Available | What it gives you |
|---|---|---|
| SSH to vJailbreak VM | ✅ (user `ubuntu`, default password `password`) | Full access to logs, CRDs, ConfigMaps via `kubectl` |
| `kubectl` on vJailbreak VM | ✅ | Pod logs, CRD status, ConfigMap values — see [support-bundle.md](support-bundle.md) |
| `kubectl` locally (if kubeconfig configured) | ✅ if configured | Same as above, from dev machine |
| OpenStack CLI (`openstack`) | ✅ | Nova/Neutron/Cinder resources — use directly, no handoff needed |
| SSH/exec into ESXi hosts | ❌ | Would allow live ESXi-side verification. Without it: rely on vJailbreak logs and ask customer/VMware admin to check ESXi-side state. |
| Live exec into `v2v-helper`/controller pods | ⚠️ Possible but avoid | Diagnose from logs and CRD status — never `kubectl exec` to poke at a running migration; it can corrupt state. |

## Step-by-Step Debugging Workflow

### Step 1: Get the Migration Name and Phase

```bash
kubectl get migration -n migration-system
```
Note `.status.phase` — this determines which reference file to open next.

### Step 2: Classify and Route

```
.status.phase / symptom                         → go to
──────────────────────────────────────────────────────────────────────
Discovery (no VMs listed / hangs)                → migration-lifecycle.md §1, check VMwareCreds
Mapping (subnet mismatch, port conflict)          → networking.md, migration-lifecycle.md §2
Validate (missing OS for powered-off VM)          → migration-lifecycle.md §3
Data Copy (NFC/nbdcopy failure)                   → copy-methods.md (Normal NFC section)
Data Copy (XCOPY/SSH/array failure)               → copy-methods.md (Storage-Accelerated section)
Data Copy (Hot-Add Proxy failure)                 → copy-methods.md (Hot-Add Proxy section)
Convert (resolv.conf, dynamic disk, PCI slots)    → guest-os-issues.md
Cutover (stuck at waitForAdminCutover, or fails)  → migration-lifecycle.md §6
Post-Migration (network unreachable, WSFC issue)  → networking.md WSFC case study, guest-os-issues.md
Cluster Conversion specific                       → cluster-conversion.md
Nova/Neutron/Cinder failures                      → run openstack CLI directly (see support-bundle.md)
```

### Step 3: vJailbreak-VM-Side Investigation (Always First)

Pull the per-migration debug log and the `v2v-helper` pod logs before anything else — see [support-bundle.md](support-bundle.md). Check the `vjailbreak-settings` ConfigMap for the `CLEANUP_*` and `PERIODIC_SYNC_*` values relevant to the phase in question.

### Step 4: Decide Retry vs. Refill-and-Restart vs. Fix Code

Use the retry-vs-cleanup decision tree in [migration-lifecycle.md](migration-lifecycle.md) — it is keyed by phase and by whether the failure is a config problem or a runtime/environment blip. If the failure points to a code bug, identify the owning source file:

| Phase | Owning code |
|---|---|
| Discovery / VMwareCreds | `k8s/migration/internal/controller/vmwarecreds_controller.go` |
| Mapping / NetworkMapping / StorageMapping | `k8s/migration/internal/controller/` |
| Data Copy — NFC | `v2v-helper/pkg/nbdcopy/` or `v2v-helper/pkg/copy/` |
| Data Copy — XCOPY | `v2v-helper/pkg/storage/` (Pure: `pure.go`, NetApp: `netapp.go`) |
| Data Copy — Hot-Add Proxy | `v2v-helper/pkg/hotadd/` |
| Convert | `v2v-helper/pkg/virtv2v/` |
| Cutover | `v2v-helper/cmd/` + `k8s/migration/internal/controller/migration_controller.go` |
| Post-Migration | `k8s/migration/internal/controller/migration_controller.go` |

### Step 5: PCD-Side Investigation (Fallback)

If vJailbreak's own logs don't explain the failure and it clearly involves Nova/Neutron/Cinder, use the OpenStack CLI directly:

```bash
# Neutron port issues
openstack port list --device-id <migration-vm-uuid> --insecure
openstack port show <port-id> --insecure

# Cinder volume stuck
openstack volume show <volume-id> --insecure
openstack volume list --status error --insecure

# Nova scheduling failure
openstack server show <server-id> --insecure
openstack server event list <server-id> --insecure
```

## Quick Error Pattern Reference

| Error / Symptom | Phase | Likely cause | First action |
|---|---|---|---|
| No VMs found during discovery | Discovery | VMware credential/permission issue | Check `VMwareCreds` revalidation status |
| Neutron port-create fails / "port already in use" | Mapping | Subnet mismatch, or stale port from a prior attempt | See [networking.md](networking.md); retry-vs-refill tree in migration-lifecycle.md |
| Missing OS type for a powered-off VM | Validate | VMware Tools unavailable (VM off) | Manually fill OS/IP in the migration form |
| `virt-v2v` rename `/etc/resolv.conf` fails | Convert | Immutable attribute set on source | `chattr -i /etc/resolv.conf` on source, retry |
| `No more available PCI slots` | Convert / disk attach | Image using `virtio-blk` instead of `virtio-scsi` | Rebuild image with `hw_disk_bus=scsi` |
| Hivex/registry read errors during inspection (Windows) | Convert | Dynamic disk (LDM) on boot disk | Convert to basic disk pre-migration |
| Windows secondary disks show Offline | Post-migration | Windows "Offline Shared" SAN policy | `diskpart` → `SAN POLICY=OnlineAll`, or firstboot script |
| VMware Tools registry/driver remnants | Post-migration | Incomplete uninstaller | No action — cosmetic, harmless |
| Migration fails during nbdcopy, DNS errors in debug log | Data Copy (Normal) | ESXi hostname not resolvable from vJailbreak VM | Add ESXi host entries to `/etc/hosts` on vJailbreak VM |
| XCOPY: SSH connection to ESXi fails | Data Copy (XCOPY) | SSH disabled, wrong key type (need RSA-4096), or network | See [copy-methods.md](copy-methods.md) troubleshooting table |
| XCOPY: mapping fails on Pure | Data Copy (XCOPY) | No existing host object for ESXi's WWPN/IQN — FC zoning/onboarding incomplete | Storage-admin task, not a vJailbreak bug — see Pure/NetApp asymmetry in copy-methods.md |
| XCOPY: mapping fails on NetApp cross-SVM | Data Copy (XCOPY) | Target volume's SVM differs from source ESXi's mapped SVM, igroup creation failed | Check array-side igroup-creation permissions on target SVM |
| Cluster IP / floating IP unreachable post-migration | Post-migration / Networking | Neutron anti-spoofing dropping the GARP | See WSFC case study in [networking.md](networking.md) |
| WSFC `ClusSvc` won't start, Event ID 1289 | Post-migration | NetFT adapter missing after virt-v2v NIC driver swap | See [guest-os-issues.md](guest-os-issues.md) |
| Cutover stuck at `waitForAdminCutover` with no progress after triggering | Cutover | Normal 10–30s propagation lag | Wait; don't re-trigger within the window |
| Cluster Conversion VM discovery inconsistent with duplicate names | Cluster Conversion | Known VM-ID-append regression | See [cluster-conversion.md](cluster-conversion.md) maturity caveat |

## Key Operational Rules

- **VMware Tools residual artifacts are cosmetic and harmless** — do not spend time trying to fully remove them; this is documented, expected behavior.
- **Leftover ports/volumes after a failed migration may be intentional**, not a cleanup bug — check `CLEANUP_PORTS_AFTER_MIGRATION_FAILURE` / `CLEANUP_VOLUMES_AFTER_CONVERT_FAILURE` in `vjailbreak-settings` before assuming otherwise.
- **A Neutron allowed-address-pair change does not retroactively announce an IP** — the GARP (or equivalent) must actually be re-sent (e.g. via cluster failover) after the permission change, or it will still look broken.
- **Mapping-phase failures require refilling the migration form** — there is no "edit migration" capability; port-conflict failures are the one exception (retry the same migration after freeing the port).
- **XCOPY on Pure never creates a new host group** — it only reuses an existing one, because a Pure host object can belong to at most one host group and creating a new one would silently unmap production volumes. If no matching host object exists, that's a storage-admin gap, not a bug.
- **Storage-Accelerated Copy (XCOPY) is cold-migration-only** and requires the exact same physical array on both sides — don't recommend it for a hot-migration or cross-array requirement.
- **Check ALL nodes of a migrated cluster (WSFC or similar), not just the first** — the NetFT-missing failure was independently reproduced on both nodes in the source RCA; a single healthy-looking node is not sufficient evidence.

## References

### Internal Skill Docs
- [architecture.md](architecture.md) — pods, CRDs, credentials, settings, scaling, compatibility, known limitations
- [migration-lifecycle.md](migration-lifecycle.md) — phase-by-phase flow, retry/cleanup decision tree, cutover, post-migration options
- [copy-methods.md](copy-methods.md) — Normal NFC, Storage-Accelerated XCOPY (Pure/NetApp), Hot-Add Proxy
- [networking.md](networking.md) — mapping, IP/MAC/interface persistence, WSFC/Neutron case study
- [guest-os-issues.md](guest-os-issues.md) — Windows/Linux conversion quirks
- [cluster-conversion.md](cluster-conversion.md) — ESXi-to-PCD-hypervisor conversion
- [support-bundle.md](support-bundle.md) — log/CRD map, support-bundle ZIP layout

### Public Documentation
- vJailbreak docs: https://platform9.github.io/vjailbreak/
- virt-v2v: https://libguestfs.org/virt-v2v.1.html
- virt-v2v support matrix: https://libguestfs.org/virt-v2v-support.1.html
- libguestfs: https://libguestfs.org/
- nbdkit: https://libguestfs.org/nbdkit.1.html
- Architecture deep-dive: https://deepwiki.com/platform9/vjailbreak
