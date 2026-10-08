# vJailbreak Guest OS Conversion Issues

Quick reference — symptom to section:

| Symptom | Section |
|---|---|
| `virt-v2v` fails renaming `/sysroot/etc/resolv.conf` | [resolv.conf Immutable](#resolvconf-immutable-linux) |
| `No more available PCI slots` during disk attach | [PCI Slot Exhaustion](#pci-slot-exhaustion) |
| Hivex/registry errors during virt-v2v inspection (Windows) | [Windows Dynamic Disk / LDM](#windows-dynamic-disk-ldm) |
| Secondary Windows disks show "Offline" post-migration | [Windows Disks Offline](#windows-disks-offline-post-migration) |
| Leftover VMware driver/registry entries after "Remove VMware Tools" | [VMware Tools Residual Artifacts](#vmware-tools-residual-artifacts) |
| WSFC `ClusSvc` won't start, Event ID 1289, NetFT adapter missing | [NetFT Adapter Missing (WSFC)](#netft-adapter-missing-wsfc) |

## resolv.conf Immutable (Linux)

**Symptom**: migration fails during `virt-v2v` conversion with an error about being unable to rename `/etc/resolv.conf` ("Operation not permitted").

**Cause**: the file has the immutable attribute set. Verify with `lsattr /etc/resolv.conf` — the `i` flag confirms it.

**Fix**: on the **source** VM, before migrating: `chattr -i /etc/resolv.conf`, confirm with `lsattr` again, then retry the migration. This is a write action on the source VM's guest OS — report it for the customer/operator to run rather than executing it yourself. This is a documented upstream `virt-v2v` limitation, not a vJailbreak bug. If a fleet of VMs consistently hits this, their configuration-management tooling is marking `resolv.conf` immutable — flag that as the systemic fix.

## PCI Slot Exhaustion

**Symptom**: mid-migration disk attach fails with `libvirt.libvirtError: internal error: No more available PCI slots`.

**Cause**: the vJailbreak appliance image (or agent image) defaults to the `virtio-blk` bus, where every attached volume consumes a separate PCI slot — a VM with several disks, or several migrations running in parallel against the same target, exhausts the limited slot count (documented cap of roughly 26 devices under `virtio-blk`).

**Fix**: switch the image to `virtio-scsi` **before** VM creation:
```bash
openstack image set \
  --property hw_disk_bus=scsi \
  --property hw_scsi_model=virtio-scsi \
  <vjailbreak-image-name-or-ID>
```
This is a write action against OpenStack (the destination image) — report it for the customer/operator to run rather than executing it yourself. Disk bus is chosen at VM-creation time — **existing** VJailbreak/agent VMs must be recreated from the updated image; this must also be applied before scaling out additional agents (see [architecture.md](architecture.md)).

## Windows Dynamic Disk (LDM)

**Symptom**: migration fails during `virt-v2v`'s inspection phase on a Windows VM, with Hivex-related errors reading registry hives.

**Cause**: Windows Dynamic Disks (LDM) store volume metadata in a proprietary 1 MB journaled database at the end of the disk. `libguestfs`/Hivex have limited LDM support — registry hives needed for OS inspection can be fragmented across LDM volumes and fail to assemble. `virt-v2v` only inspects the root/boot disk, so **any** dynamic disk on the boot disk blocks the entire migration at the inspection step — this is a known limitation, not a data-corruption bug.

**Fix** (convert to basic disks before migrating):
1. Boot the Windows VM in VMware.
2. Disable Fast Startup: `powercfg /h off`.
3. Use `diskpart` to convert the dynamic disk(s) to basic.
4. Perform a clean shutdown.
5. Proceed with the vJailbreak migration.

This is a write action on the source VM's guest OS — report it for the customer/operator to run rather than executing it yourself. Requires enough free/empty space to perform the conversion — back up data first if the disk is nearly full.

## Windows Disks Offline Post-Migration

**Symptom**: after migration, secondary/data disks (not `C:`) show as **"Offline"** in Disk Management, despite having attached successfully; only the primary boot drive is accessible.

**Cause**: Windows applies a SAN policy based on the detected storage subsystem. Moving from VMware to PCD's storage triggers Windows' default **"Offline Shared"** SAN policy, which proactively keeps non-boot disks offline to avoid corruption on genuinely shared storage — a defensive default, not a migration error.

**Fix (manual)**:
```
diskpart
SAN POLICY=OnlineAll
online disk
```
(repeat `online disk` for each offline disk, after selecting it)

This is a write action on the destination guest OS — report it for the customer/operator to run rather than executing it yourself.

**Fix (automated)**: two scripts under `scripts/firstboot/windows/` in the vJailbreak repo —
- `check-disks.bat` — diagnostic only, makes no changes.
- `disk-online-fix.bat` — brings **all** offline disks online automatically.

Either can be pasted into the migration's **Post Migration Script** field to run at first boot. This too is a write action on the destination guest OS — report it for the customer/operator to add via the Post Migration Script field rather than adding it yourself.

**Warning**: `disk-online-fix.bat` is a blanket fix. If the source VM intentionally kept a disk offline (e.g. a backup or warm-standby disk), the automated fix will bring it online too — don't apply it uncritically on VMs where that distinction matters.

Note: recent vJailbreak releases have the "Remove VMware Tools" option also address a related cosmetic offline/error-device quirk automatically — check the release notes for the version in use before assuming manual intervention is still required.

## VMware Tools Residual Artifacts

**Symptom**: after checking "Remove VMware Tools" during migration, some driver files, registry keys, folders, or Device Manager error entries remain on Windows VMs. Older Windows (2012) leaves the most remnants; newer versions (2025) leave minimal traces.

**Cause**: the VMware Tools uninstaller doesn't remove every component it ever wrote.

**Fix: none required.** This is documented as harmless — orphaned drivers don't load, Device Manager error entries are cosmetic only, stale registry keys have no runtime effect, and Windows automatically falls back to a standard driver (e.g. generic HID for mouse/keyboard input) so functionality is unaffected. Tell the customer this is expected and safe to ignore; do not spend time trying to fully scrub it.

## NetFT Adapter Missing (WSFC)

**Symptom**: on a migrated Windows Server Failover Cluster node, `ClusSvc` fails to start. System Event Log shows Event ID 1289 (Critical, FailoverClustering, Cluster Virtual Adapter); the cluster log shows `[CS] Service CreateNodeThread Failed... 'Network interface for NetFT adapter not found.'`, with `GetFaultTolerantAdapter` retrying (20× @ 500ms) before giving up.

**Cause**: the "Microsoft Failover Cluster Virtual Adapter" (NetFT), which the Failover-Clustering feature creates, never got instantiated on the node at all post-migration. Confirm with `Get-NetAdapter -IncludeHidden` — the adapter is **absent entirely**, not merely `Disconnected` (a briefly-`Disconnected`-at-boot adapter is normal and different from this). This is strongly suspected to be caused by `virt-v2v`'s NIC driver swap (VMware vmxnet3 → virtio) disrupting the NetFT PnP binding — it was reproduced independently on both nodes of a 2-node cluster in the source RCA, which rules out node-specific hardware flukiness.

**Fix**: force-recreate the adapter by reinstalling the Failover-Clustering Windows feature:
```powershell
Uninstall-WindowsFeature -Name Failover-Clustering -Restart
# after reboot:
Install-WindowsFeature -Name Failover-Clustering -IncludeManagementTools -Restart
```
Apply independently on each affected node. After reboot, `Get-NetAdapter -IncludeHidden` should show the adapter present (in `Disconnected` state — normal prior to the cluster service starting). This is a write action on the guest OS — report it for the customer/operator to run.

**Gotcha**: this reinstall cycle **also resets `ClusSvc`'s startup type back to `Disabled`** every time. The following must be reapplied after **every** reinstall, or the service won't start despite the adapter now existing:
```powershell
Set-Service -Name ClusSvc -StartupType Automatic
net start clussvc
```

**Treat as a repeatable pattern, not a one-off**: check NetFT presence on **every** node of a migrated WSFC (or any Microsoft Failover Cluster) — don't stop checking after the first node comes up clean; the RCA source found it missing on both nodes independently, so a single "it worked on node 1" check is not sufficient evidence the cluster is healthy. See the pre/post-migration checklist in [networking.md](networking.md)'s WSFC case study, which this pairs with (that section covers the Cluster-IP-unreachable half of the same class of failure; this section covers the cluster-service/adapter half).

## References
- [migration-lifecycle.md](migration-lifecycle.md), [networking.md](networking.md), [support-bundle.md](support-bundle.md)
- Docs: `/vjailbreak/guides/troubleshooting/troubleshooting/`, `/vjailbreak/guides/troubleshooting/windows-dynamic-disk-ldm-migration-issue/`, `/vjailbreak/guides/troubleshooting/windows-offline-disks/`, `/vjailbreak/guides/troubleshooting/vmware_residual_artifacts/`
- WSFC RCA (internal): full root-cause chain for the NetFT + Neutron GARP failure pair
