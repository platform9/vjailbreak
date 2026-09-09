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

## Ground Rules (apply throughout every step below)

- **Independent-evidence rule.** A JIRA/GitHub issue title, a customer-authored RCA doc, or a
  reporter's stated theory is **one input, not ground truth**. It may be wrong, biased toward a
  convenient explanation ("must be a regression"), or simply guessing. Refer to it, cite it, but
  verify independently before adopting its conclusion. State every finding as one of: **evidence
  observed** (a specific log line, doc citation, or reproduced behavior) vs. **hypothesis
  proposed** (unconfirmed, needs more evidence) vs. **hypothesis confirmed/refuted** (evidence
  now settles it). Never present the second as the third.
- **Verify-before-assert rule.** Do not state how an underlying tool (guestfish/libguestfs/
  virt-v2v-in-place/nbdkit/vCenter/OpenStack) behaves from memory. Check it — against the actual
  log evidence in front of you, or against the tool's own docs/source (see the specialist agents
  in Step 3.5). If you can't verify a claim, say it's unverified rather than stating it with
  confidence.
- **Diagnosis-first gate.** Your job by default is to explain what happened, not to fix it. Reach
  and report a root cause, then **stop and wait for explicit go-ahead** before editing any source
  file. Do not jump from "root cause found" straight into `Edit` calls. See Step 4.
- **Parallel-fan-out rule.** When investigation splits into N genuinely independent pieces — N
  debug bundles to summarize, N migrations to RCA, N competing hypotheses to check, N tool
  domains a failure might touch — dispatch N agents together rather than one at a time. Each
  agent's prompt should itself carry the independent-evidence rule (explicit "do not assume X is
  causal," "do not accept the reporter's theory uncritically," "report facts only, no fix
  proposals," a word-count cap). Only synthesize/diff the results afterward, centrally — no
  single agent has the full picture.

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

### Step 2.5: Check Known Issues First

Grep [known-issues.md](known-issues.md) for the symptom pattern (error string, phase, guest OS)
before starting a fresh deep investigation. A match means this case has been confirmed before —
but still confirm it still applies (same phase, same tool version, same migration path) rather
than reusing the conclusion wholesale. If nothing matches, proceed to Step 3 as normal; this is a
fast-path, not a gate.

### Step 3: vJailbreak-VM-Side Investigation (Always First)

Pull the per-migration debug log and the `v2v-helper` pod logs before anything else — see [support-bundle.md](support-bundle.md). Check the `vjailbreak-settings` ConfigMap for the `CLEANUP_*` and `PERIODIC_SYNC_*` values relevant to the phase in question.

**Offline debug bundle instead of a live cluster?** Same steps, reading files from the extracted
bundle instead of `kubectl`. If you're given a second bundle to compare against (e.g. "here's a
successful run of the same VM"), see [bundle-comparison.md](bundle-comparison.md) before diffing
them — confirming VM identity and diffing phase-by-phase, in parallel per the fan-out rule above,
not by hand.

### Step 3.5: Second-Order Analysis (MANDATORY for Convert and Data Copy failures)

**Identifying the error string is NOT the end of debugging.** After classifying the phase and finding the error, you MUST answer:

> "WHY does the underlying tool (guestfish / libguestfs / virt-v2v-in-place / nbdkit / vCenter / OpenStack) behave this way for THIS guest OS, disk layout, and environment?"

Route by tool error. Check [tool-internals.md](tool-internals.md) first (fast, static); escalate
to the cited specialist agent when the claim is contested, unprecedented, or tool-internals.md
doesn't cover it — the specialist fetches live docs instead of relying on a cached summary:

| Error contains | Ask | Static reference | Specialist agent (if escalating) |
|---|---|---|---|
| `guestfish: multi-boot` | How many OS roots does each disk have individually? Is this Btrfs/Snapper? | [tool-internals.md §guestfish-i](tool-internals.md#guestfish-i) | `vjb-guestfs-behavior` |
| `inspect-os` failure | Is guest using Btrfs? Multiple disks? Snapshots? | [tool-internals.md §inspect-os](tool-internals.md#inspect-os) | `vjb-guestfs-behavior` |
| `resolv.conf` immutable | Did source have `chattr +i` set? | [guest-os-issues.md](guest-os-issues.md) | `vjb-virtv2v-inplace-behavior` |
| `No more available PCI slots` | Is image using `virtio-blk` vs `virtio-scsi`? | [tool-internals.md §virt-v2v](tool-internals.md#virt-v2v) | `vjb-virtv2v-inplace-behavior` |
| `initramfs` / dracut / mkinitrd | Does guest use dracut or legacy mkinitrd? | [tool-internals.md §virt-v2v](tool-internals.md#virt-v2v) | `vjb-virtv2v-inplace-behavior` |
| `nbdkit` / VDDK / transports | Is ESXi reachable by hostname? Thumbprint match? | [tool-internals.md §nbdkit](tool-internals.md#nbdkit) | `vjb-guestfs-behavior` (nbdkit) or `vjb-vcenter-behavior` (VDDK/ESXi-side) |
| NFC / vpxa / snapshot / moref errors | Is this documented vCenter/ESXi behavior, or unexpected? | — | `vjb-vcenter-behavior` |
| Nova/Neutron/Cinder errors reaching this deep | Capacity/quota/admin-config on target cloud, or vjailbreak bug? | [support-bundle.md](support-bundle.md) PCD-side table | `vjb-openstack-behavior` |

**When the failure could plausibly originate in more than one domain** (e.g. "is this a vCenter
snapshot quirk or a guestfs multi-boot false-positive?"), dispatch the relevant specialists **in
parallel, one message**, per the fan-out rule — don't check them one at a time.

**Do not stop at "code bug in X.go".** Explain what guest-OS or environment characteristic triggered the bug.

### Step 4: Report Root Cause — Stop Here

Per the diagnosis-first ground rule: present the root cause (evidence observed vs. hypothesis
confirmed — see ground rules) and **stop**. Do not proceed to editing source files in the same
turn. Wait for the user to explicitly ask for a fix, a retry, or further investigation before
continuing to Step 4.5.

### Step 4.5: Decide Retry vs. Refill-and-Restart vs. Fix Code (after user confirms)

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

### Step 6: Record Confirmed Findings (only with user confirmation)

If the root cause reached **confirmed** status (per the independent-evidence ground rule — actual
evidence, not an inherited hypothesis), ask the user whether to record it in
[known-issues.md](known-issues.md). **Never append without an explicit yes** — this file is
checked into git and read by future debugging sessions, so a wrong or premature entry compounds.
If confirmed, append a row using the format in known-issues.md's own header.

## Quick Error Pattern Reference

| Error / Symptom | Phase | Likely cause | First action |
|---|---|---|---|
| No VMs found during discovery | Discovery | VMware credential/permission issue | Check `VMwareCreds` revalidation status |
| Neutron port-create fails / "port already in use" | Mapping | Subnet mismatch, or stale port from a prior attempt | See [networking.md](networking.md); retry-vs-refill tree in migration-lifecycle.md |
| Missing OS type for a powered-off VM | Validate | VMware Tools unavailable (VM off) | Manually fill OS/IP in the migration form |
| `guestfish: multi-boot operating systems are not supported` | Convert | guestfish `-i` inspects all disks together; finds 2+ OS roots (common: OpenSUSE Btrfs+Snapper subvolumes, or 2-disk VM where both disks have OS-like content) | Test each disk individually: `guestfish --ro -a /dev/vdb : run : inspect-os`; see [tool-internals.md §guestfish-i](tool-internals.md#guestfish-i) |
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
- [tool-internals.md](tool-internals.md) — guestfish `-i` multi-boot detection, libguestfs inspect-os, Btrfs/Snapper, virt-v2v pipeline, nbdkit/VDDK failures
- [architecture.md](architecture.md) — pods, CRDs, credentials, settings, scaling, compatibility, known limitations
- [migration-lifecycle.md](migration-lifecycle.md) — phase-by-phase flow, retry/cleanup decision tree, cutover, post-migration options
- [copy-methods.md](copy-methods.md) — Normal NFC, Storage-Accelerated XCOPY (Pure/NetApp), Hot-Add Proxy
- [networking.md](networking.md) — mapping, IP/MAC/interface persistence, WSFC/Neutron case study
- [guest-os-issues.md](guest-os-issues.md) — Windows/Linux conversion quirks
- [cluster-conversion.md](cluster-conversion.md) — ESXi-to-PCD-hypervisor conversion
- [support-bundle.md](support-bundle.md) — log/CRD map, support-bundle ZIP layout
- [bundle-comparison.md](bundle-comparison.md) — diffing two offline debug bundles (identity check, phase-by-phase timeline)
- [known-issues.md](known-issues.md) — living, confirmed-case log; check first (Step 2.5), append with user confirmation (Step 6)

### Specialist Agents
- `vjb-debug-triage` — controller + v2v-helper log triage; live (kubectl) or bundle (offline tarball) mode
- `vjb-module-investigator` — cross-module code locator (all 4 Go modules)
- `vjb-vcenter-behavior` — vCenter/ESXi/govmomi behavior, cites vSphere API docs + govmomi
- `vjb-guestfs-behavior` — libguestfs/guestfish/nbdkit internals, cites libguestfs.org/nbdkit.1.html
- `vjb-virtv2v-inplace-behavior` — `virt-v2v-in-place` specifically, cites virt-v2v-in-place(1)/virt-v2v-support(1)
- `vjb-openstack-behavior` — Nova/Neutron/Cinder behavior, cites docs.openstack.org

Dispatch the relevant specialists in parallel (one message) when a hypothesis spans domains — see
the parallel-fan-out ground rule.

### Public Documentation
- vJailbreak docs: https://platform9.github.io/vjailbreak/
- virt-v2v-in-place: https://libguestfs.org/virt-v2v-in-place.1.html
- virt-v2v support matrix: https://libguestfs.org/virt-v2v-support.1.html
- libguestfs: https://libguestfs.org/
- nbdkit: https://libguestfs.org/nbdkit.1.html
- vSphere API reference: https://developer.vmware.com/apis/vsphere-automation/latest/
- govmomi: https://github.com/vmware/govmomi
- OpenStack docs: https://docs.openstack.org/
- Architecture deep-dive: https://deepwiki.com/platform9/vjailbreak
