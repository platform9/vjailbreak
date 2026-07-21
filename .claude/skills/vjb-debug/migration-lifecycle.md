# vJailbreak Migration Lifecycle

## Phase Flow

```
Discovery → Mapping → Validate → Data Copy → Convert → [Cutover] → Post-Migration → Completed
```

`Migration.status.phase` tracks this. Get the migration name first (see [architecture.md](architecture.md)) — every log file, pod name, and CRD is keyed by it.

## Phase-by-Phase

### 1. Discovery
User selects a vCenter cluster (or "no cluster" to list every VM in the vCenter, unscoped) and a destination PCD cluster. vJailbreak lists VMs from that scope, marking any it has already migrated **from this vJailbreak instance's own record** — a VM migrated by a *different* vJailbreak instance or a different tool will NOT show as already-migrated here; this is per-instance state, not global.

**Failure signature**: no VMs listed, or listing hangs → almost always a VMware credential/permission problem (wrong vCenter, insufficient RBAC to enumerate VMs) rather than a vJailbreak bug. Check `VMwareCreds` revalidation status first (see [architecture.md](architecture.md)).

### 2. Mapping
Two independent mappings, both required:
- **Network mapping**: source vSphere network/port-group → destination PCD network. See [networking.md](networking.md) for the subnet-mismatch failure mode.
- **Storage mapping**: source VMware datastore → destination PCD Cinder backend/volume-type. Picking a backend with insufficient free space is a common silent failure — it looks like a bug but is actually just "out of space"; check the destination backend's free capacity before assuming code is broken.

### 3. Validate
For powered-ON VMs, vJailbreak reads OS/network details automatically via VMware Tools. For powered-OFF VMs, VMware Tools can't be queried — the user must manually supply OS type and (optionally) IP address in the migration form. If a powered-off VM's migration fails here with missing-OS-type errors, this is expected — the fix is filling in the form field, not a vJailbreak defect.

### 4. Data Copy
See [copy-methods.md](copy-methods.md) for the three copy mechanisms (Normal NFC, Storage-Accelerated XCOPY, Hot-Add Proxy) and their distinct failure modes. All three write progress to the `v2v-helper` pod logs; XCOPY additionally has the `pframe/` raw block-copy debug logs (see [support-bundle.md](support-bundle.md)).

### 5. Convert
`v2v-helper` runs `virt-v2v` in-place on the copied disk: finds the boot device, swaps drivers (VMware vmxnet3/pvscsi → virtio), fixes the bootloader, and detects the OS. Most guest-OS-specific failures (dynamic disks, immutable resolv.conf, PCI slot exhaustion) surface here — see [guest-os-issues.md](guest-os-issues.md).

### 6. Cutover (only relevant for Admin-Initiated cutover)
Migration parks in `waitForAdminCutover`. Two ways to trigger it:
- **UI**: open the migration, click "Admin Cutover", confirm in the dialog.
- **kubectl**: find the pod name (`<migration-name>-v2v-helper`), then patch its label:
  ```bash
  kubectl -n migration-system get pods | grep <migration-name>
  kubectl -n migration-system label pod <migration-name>-v2v-helper startCutover=yes --overwrite
  ```
  This is a write action — per this skill's read-only stance, report this command for the user/operator to run rather than executing it yourself.
- Expect a **10–30 second lag** between triggering cutover and progress resuming — this is the flag propagating into the `v2v-helper` pod's reconciliation loop, not a hang. Don't re-trigger within that window.
- After cutover starts: final delta-block copy (only the blocks changed since the last sync) → convert → attach → power on.

### 7. Post-Migration
Optional, configured per-migration: run a custom post-migration script (a comment at the top of the script distinguishes Windows vs Linux execution), rename the source VMware VM (e.g. suffix `_migrated_to_pcd`) for tracking, move the source VM to a vCenter folder, disconnect the source VM's network adapter, persist network interface names (see [networking.md](networking.md)), remove VMware Tools (see [guest-os-issues.md](guest-os-issues.md)).

## Data Copy Methods — Timing/Downtime Summary

| Method | Source powered off? | CBT used? | Downtime |
|---|---|---|---|
| Hot (live) | No, until conversion | Yes | Lowest — VM stays up during bulk copy |
| Cold | Yes, before copy starts | No | Full migration duration |
| Mock | No, on either side | Yes | None — for testing only; **use a different subnet/network than the source** to avoid IP conflict since both VMs are live simultaneously |
| Periodic Sync | No, until cutover | Yes | Full initial copy happens once, then only small deltas at cutover — for very large (multi-TB, multi-day) migrations |

## Cutover Scheduling Variants

- **Immediate**: cutover fires automatically right after data copy completes.
- **Time Window**: cutover only allowed within a configured window (e.g. a maintenance window) — commonly combined with Admin-Initiated.
- **Admin-Initiated**: waits indefinitely for a human/API trigger (see above).

## Retry vs. Cleanup Decision Tree

This is the actual triage logic engineers use (source: live Q&A with the vJailbreak team) — use it before guessing whether to retry in place or start over.

| Failed phase | Is it a config problem or a runtime/environment blip? | What to do |
|---|---|---|
| **Discovery** | Config (bad VMware creds/permissions) | Fix creds/permissions in `VMwareCreds`. No artifacts exist yet — just retry discovery. |
| **Mapping** | Config (wrong network/subnet or wrong datastore/backend chosen) | There is **no "edit migration" capability** in the UI/CRD model. Refill the migration form and create a fresh `Migration`/`MigrationPlan` — do not try to patch the existing one. |
| **Mapping — port conflict** ("port already in use") | NOT a config error — it's a side effect (a stale port from a prior attempt, or genuinely in use by another VM) | Detach/free the specific conflicting port on OpenStack if this exact port must belong to this VM, then **retry the SAME migration** — no form refill needed. |
| **Validate** | Config (something conflicts with the overall migration context, e.g. missing OS for a powered-off VM) | Refill the form and restart. |
| **Validate** | Runtime/environment (transient network error) | Just retry the same migration after fixing the environment issue. No refill needed. |
| **Data Copy / Convert** | Usually environment (vCenter connectivity, ESXi DNS resolution) or an unsupported guest condition (unknown filesystem like XFS, unsupported disk layout) | Pull `v2v-helper` pod logs + `pframe/` debug logs + source disk layout. If vCenter/ESXi connectivity — fix and retry. If unsupported filesystem/layout — collect the debug logs and source layout and treat as a product limitation to escalate, not something to retry your way out of. |
| **Data Copy / Convert — attachment/detachment timeout, backend out of space, Cinder incompatibility** | PCD-side | Check PCD-side logs (Cinder) **before** re-reading vJailbreak logs — the root cause is usually on that side. Hand off to the `cinder` skill (`pcd-common/skills/cinder/`; see [support-bundle.md](support-bundle.md)). |
| **Cutover** | Usually the same causes as Data Copy/Convert (data-copy or network-connectivity issue) | Retry after confirming environment is healthy; if it keeps failing, use the Data Copy/Convert debug path above. |
| **Post-Migration** | Depends on which optional feature failed | Source VM is assumed powered off at this point (unless Mock migration) — check the specific feature's documented log location (e.g. custom script exit code, rename/move confirmation in controller logs). |

## Cleanup Behavior on Failure

Two `vjailbreak-settings` flags control whether artifacts from a *failed* migration are cleaned up automatically — see [architecture.md](architecture.md) for the full settings table:
- `CLEANUP_PORTS_AFTER_MIGRATION_FAILURE`
- `CLEANUP_VOLUMES_AFTER_CONVERT_FAILURE`

**If these are disabled** (check the actual ConfigMap value, don't assume), leftover ports/volumes from a failed attempt are deliberately left behind — a retry is expected to detect and reuse them rather than fail on "already exists." This is why the port-conflict row above says "just retry" rather than "clean up first."

**If a volume is stuck and won't clean up even with the flag enabled**: look for an **attachment/detachment TIMEOUT** log line in the `v2v-helper` logs. A volume can get stuck in Cinder's `detaching`/`reserved`/`in-use` state after a timeout, and vJailbreak cannot force-clean a volume in that state — it requires manual intervention on the OpenStack/Cinder side (hand off to the `cinder` skill (`pcd-common/skills/cinder/`)).

## Post-Migration Options Reference

| Option | Effect |
|---|---|
| Post-migration script | Custom script executed at first boot; comment header distinguishes Windows/Linux. |
| Rename VMware VM | Appends a suffix (e.g. `_migrated_to_pcd`) to the SOURCE VM's name in vCenter, for tracking. |
| Move to folder | Relocates the source VM to a specified vCenter folder post-migration. |
| Disconnect source network | Removes the source VM's network adapter after migration (prevents accidental dual-homing). |
| Persist network interfaces | See [networking.md](networking.md). |
| Remove VMware Tools | See [guest-os-issues.md](guest-os-issues.md). |
| GPU-flavor filter bypass | By default vJailbreak filters out GPU-tagged flavors when auto-picking a destination flavor; check this box to allow GPU flavors. |
| Dynamic hot-plug-enabled flavor | Uses a pre-existing PCD hot-plug flavor automatically instead of a fixed flavor. |
| Fallback to DHCP | If the mapped network doesn't contain the source IP's subnet (cross-network migration) or the desired port/IP is in conflict, grabs any free IP from the mapped network instead of failing. |

## References
- [architecture.md](architecture.md), [copy-methods.md](copy-methods.md), [networking.md](networking.md), [guest-os-issues.md](guest-os-issues.md), [support-bundle.md](support-bundle.md)
- Docs: `/vjailbreak/concepts/migration-options/`, `/vjailbreak/guides/how-to/perform_admin_cutover/`, `/vjailbreak/guides/how-to/vjailbreak_settings/`
