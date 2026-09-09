# Known Issues — Confirmed Case Log

Living, append-only log of **confirmed** root causes from real debugging sessions. This is
distinct from [tool-internals.md](tool-internals.md) (static reference on how tools behave) and
[SKILL.md](SKILL.md)'s Quick Error Pattern Reference (fast lookup table) — this file is the
evidence trail behind those entries, and where NEW confirmed cases get recorded.

## Append rules (SKILL.md Step 6)

Only append a row when:
1. The root cause is **confirmed by evidence** (log lines, doc citation, reproduced behavior) —
   not a hypothesis, not inherited from a JIRA ticket's own theory. See SKILL.md's
   independent-evidence rule.
2. The user has **explicitly confirmed** they want it recorded. Never write silently.
3. Cite the evidence (log excerpt, file:line, doc URL) so a future reader can verify, not just
   trust the row.

Format:

```
### <short symptom title>
- **Symptom pattern**: <what the log/UI shows>
- **Confirmed root cause**: <the actual cause>
- **Evidence**: <log line excerpt, file:line, or doc citation that proves it>
- **JIRA/ticket ref**: <ref or "none">
- **Date**: <YYYY-MM-DD>
```

## Fast-path lookup (SKILL.md Step 2.5)

Before deep investigation, grep this file for the symptom string. A match here means the case is
already solved — confirm it still applies (same phase, same tool version) before reusing the
conclusion wholesale.

---

## Seeded from prior Quick Error Pattern Reference (pre-dates this file; evidence lives in the
## session/PR history that originally added each row, not re-verified here)

### No VMs found during discovery
- **Symptom pattern**: Discovery phase returns zero VMs
- **Confirmed root cause**: VMware credential/permission issue
- **Evidence**: See `VMwareCreds` revalidation status
- **JIRA/ticket ref**: none
- **Date**: unknown (legacy entry)

### Neutron port-create fails / "port already in use"
- **Symptom pattern**: Mapping phase fails with port conflict
- **Confirmed root cause**: Subnet mismatch, or stale port from a prior attempt
- **Evidence**: [networking.md](networking.md); retry-vs-refill tree in migration-lifecycle.md
- **JIRA/ticket ref**: none
- **Date**: unknown (legacy entry)

### guestfish multi-boot operating systems are not supported
- **Symptom pattern**: `guestfish: multi-boot operating systems are not supported` during Convert
- **Confirmed root cause**: guestfish `-i` inspects all disks together; finds 2+ OS roots (common:
  OpenSUSE Btrfs+Snapper subvolumes, or 2-disk VM where both disks have OS-like content)
- **Evidence**: Test each disk individually: `guestfish --ro -a /dev/vdb : run : inspect-os`; see
  [tool-internals.md §guestfish-i](tool-internals.md#guestfish-i)
- **JIRA/ticket ref**: none
- **Date**: unknown (legacy entry)

### virt-v2v-in-place resolv.conf rename fails
- **Symptom pattern**: Convert phase fails renaming `/etc/resolv.conf`
- **Confirmed root cause**: Immutable attribute (`chattr +i`) set on source file
- **Evidence**: `chattr -i /etc/resolv.conf` on source, retry succeeds
- **JIRA/ticket ref**: none
- **Date**: unknown (legacy entry)

### No more available PCI slots
- **Symptom pattern**: Convert/disk-attach fails, PCI slot exhaustion
- **Confirmed root cause**: Image using `virtio-blk` instead of `virtio-scsi` (1 slot per disk vs.
  shared bus)
- **Evidence**: Rebuild image with `hw_disk_bus=scsi`
- **JIRA/ticket ref**: none
- **Date**: unknown (legacy entry)

### Hivex/registry read errors during inspection (Windows)
- **Symptom pattern**: Convert phase, Hivex errors reading Windows registry
- **Confirmed root cause**: Dynamic disk (LDM) on boot disk
- **Evidence**: Convert to basic disk pre-migration resolves it
- **JIRA/ticket ref**: none
- **Date**: unknown (legacy entry)

### Windows secondary disks show Offline post-migration
- **Symptom pattern**: Post-migration, secondary Windows disks show Offline in Disk Management
- **Confirmed root cause**: Windows "Offline Shared" SAN policy carried over
- **Evidence**: `diskpart` → `SAN POLICY=OnlineAll`, or firstboot script
- **JIRA/ticket ref**: none
- **Date**: unknown (legacy entry)

### nbdcopy DNS errors during data copy
- **Symptom pattern**: Migration fails during nbdcopy, DNS errors in debug log
- **Confirmed root cause**: ESXi hostname not resolvable from vJailbreak VM
- **Evidence**: Add ESXi host entries to `/etc/hosts` on vJailbreak VM resolves it
- **JIRA/ticket ref**: none
- **Date**: unknown (legacy entry)

### XCOPY SSH connection to ESXi fails
- **Symptom pattern**: Storage-Accelerated Copy fails at SSH connect step
- **Confirmed root cause**: SSH disabled on ESXi, wrong key type (need RSA-4096), or network path
  blocked
- **Evidence**: See [copy-methods.md](copy-methods.md) troubleshooting table
- **JIRA/ticket ref**: none
- **Date**: unknown (legacy entry)

### XCOPY mapping fails on Pure Storage
- **Symptom pattern**: XCOPY mapping step fails, Pure array
- **Confirmed root cause**: No existing host object for ESXi's WWPN/IQN — FC zoning/onboarding
  incomplete. Not a vJailbreak bug: Pure host objects belong to at most one host group, and
  vJailbreak deliberately never creates a new group (would silently unmap production volumes).
- **Evidence**: Storage-admin task; see Pure/NetApp asymmetry in copy-methods.md
- **JIRA/ticket ref**: none
- **Date**: unknown (legacy entry)

### Cluster IP / floating IP unreachable post-migration (WSFC)
- **Symptom pattern**: WSFC cluster IP unreachable after migration; `ClusSvc` won't start, Event ID
  1289
- **Confirmed root cause**: NetFT adapter missing after virt-v2v-in-place NIC driver swap, and/or
  Neutron anti-spoofing dropping the GARP re-announcement
- **Evidence**: Independently reproduced on both nodes of the source RCA — check ALL cluster
  nodes, not just the first. See WSFC case study in [networking.md](networking.md) and
  [guest-os-issues.md](guest-os-issues.md)
- **JIRA/ticket ref**: none
- **Date**: unknown (legacy entry)
