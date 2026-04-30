---
title: Known Limitations
description: Known limitations and unsupported configurations in vJailbreak
---

This page documents known limitations, unsupported configurations, and important caveats in vJailbreak. Review this page before planning a migration to avoid unexpected failures.

## Windows Dynamic Disk (LDM)

Windows VMs that use **dynamic disks** (Logical Disk Manager / LDM) cannot be migrated directly. `virt-v2v` fails during the inspection phase when the Windows boot disk uses LDM volumes.

**Workaround**: Before migrating, boot the VM in VMware and convert dynamic disks to basic disks using `diskpart`:

```cmd
diskpart
select disk N
convert basic
```

If the disk contains data, back it up first. See the full troubleshooting guide: [Windows Dynamic Disk (LDM) migration issue](../../guides/troubleshooting/windows-dynamic-disk-ldm-migration-issue/).

| Configuration | Result |
|---|---|
| Root: Basic, Data: LDM | Works — import LDM disks in Windows post-migration |
| Root: LDM, Data: Basic | **Fails** — must convert root disk to basic |
| Root: LDM, Data: LDM | **Fails** — must convert root disk to basic |

## Active Directory Domain Controllers

Migrating an **Active Directory Domain Controller (DC)** VM carries significant risk. Migrating a DC creates a snapshot-like copy of the AD database, which can trigger a **USN rollback** — the domain rejects the migrated DC because its Update Sequence Numbers are out of sync with the replication topology. This causes replication failures and may corrupt the AD environment.

Additionally, after migration the DC may fail to start AD DS with error **C00002E2** (`STATUS_DS_SAM_INIT_FAILURE`). This occurs when Windows detects that the DirectoryServices-DomainController role cannot be initialized, which can happen when the DC was cloned without following the proper virtualization procedures.

**Symptoms**:

- The VM boots but Active Directory Domain Services fails to start
- Event log shows stop error C00002E2
- Other DCs in the domain report replication errors

**Recommended approach**: Do not migrate production Active Directory Domain Controllers directly. Instead:

1. **Build a new DC** in the target OpenStack environment and promote it using standard AD procedures.
2. **Replicate AD** to the new DC from an existing domain controller.
3. **Demote and decommission** the source DC from the domain before or after migration.

**If the DC fails to start after migration**, boot into **Directory Services Restore Mode (DSRM)** and either:

- Re-add the DirectoryServices-DomainController role: `dism.exe /online /enable-feature /featurename:DirectoryServices-DomainController`
- Perform a forced demotion: `Uninstall-AddsDomainController -ForceRemoval` (Windows Server 2012+) or `dcpromo.exe /forceremoval`

See: [Domain controller does not start — C00002E2 error](https://learn.microsoft.com/en-us/troubleshoot/windows-server/active-directory/domain-controller-not-start-c00002e2-error)

:::caution
For **member VMs** (non-DC workstations and servers joined to a domain), migration is safe but the VM will need to be re-joined to the domain after migration using a local administrator account, as the domain trust relationship is not preserved.
:::

## Persist Network: Windows Server 2012 and Below

The **Persist source network interfaces** option does not work for Windows Server 2012 and earlier (including Windows Server 2008 R2 and Windows Server 2008).

Static network interface name persistence relies on driver injection capabilities not available in these older Windows versions.

**Workaround**: Manually reconfigure network interface names and static IP settings inside the VM after migration.

## Assign IP and Persist Network Cannot Be Used Together

The **Assign IP** and **Persist Network** (Persist source network interfaces) options are mutually exclusive. Enabling both simultaneously produces undefined behavior and the migration may not apply either setting correctly.

**Rule**: Use one or the other — not both.

- Use **Assign IP** when you need to set a specific IP address on the destination VM.
- Use **Persist Network** when you need to preserve the source VM's interface names and static routes.

## Multi-IP Assignment: Only First IP Preserved

When configuring multiple IP addresses in the **Assign IPs** field, only the **first IP** is preserved on the destination VM. Subsequent IPs in the list are silently ignored.

**Workaround**: Assign additional IPs manually inside the VM after migration, or use OpenStack port configuration to attach additional floating IPs post-migration.

## VMware Tools Removal: Residual Artifacts

The VMware Tools removal process performed by `virt-v2v` during migration may leave behind residual files and registry entries on the destination VM.

These artifacts are typically harmless but may appear in application logs or security scans.

For a full list of known residual artifacts and cleanup steps, see: [VMware Residual Artifacts](../../guides/troubleshooting/vmware_residual_artifacts/).

## Multi-Boot VMs Not Supported

vJailbreak does not support VMs with **multiple bootable operating systems** (multi-boot configurations). `virt-v2v` inspects only a single OS installation per VM and cannot convert multi-boot disk layouts.

**Workaround**: Migrate each OS as a separate VM, or convert the disk to a single-boot configuration before migration.

## Hotplug Flavor Requirements

OpenStack **hotplug** (live CPU/RAM resize without VM reboot) is supported post-migration, but only if the assigned flavor is explicitly configured for hotplug by the OpenStack administrator.

To use hotplug after migration:

1. Ask your OpenStack admin to create a flavor with hotplug-enabled extra specs, for example:

   ```bash
   openstack flavor set <flavor-name> \
     --property hw:cpu_policy=mixed \
     --property hw:cpu_max_vcpus=<max> \
     --property hw:mem_page_size=any
   ```

2. Assign this flavor in the vJailbreak migration form before starting the migration.
3. After migration, resize the VM in OpenStack using the hotplug-capable flavor.

:::note
Standard flavors without hotplug extra specs will not support live resize. The VM must be powered off for a cold resize in that case.
:::

## Low Disk Space in the Source VM

Before starting conversion, `virt-v2v` checks that each filesystem inside the **source VM** has sufficient free space. If any filesystem is too full, the conversion fails before it begins.

Minimum free space required inside the source VM ([source: virt-v2v docs](https://libguestfs.org/virt-v2v.1.html)):

| Filesystem | Minimum free space |
|---|---|
| Linux root (`/`) | 100 MB |
| Linux `/boot` | 50 MB (needed to rebuild initramfs) |
| Windows `C:` drive | 100 MB (virtio drivers and guest agents are copied in) |
| Any other mountable filesystem | 10 MB |

Each filesystem must also have at least **100 free inodes**.

**Workaround**: Before migrating, free up space inside the source VM on any full partitions. Check with `df -h` (Linux) or Disk Management (Windows).

## Application Reboot During Migration

Cold migration (**Power off VMs, then copy**) powers off the source VM before copying its disk. The destination VM boots fresh after migration completes. **Applications must tolerate a reboot** — any in-memory state, open transactions, or non-persistent connections will be lost.

Hot migration (**Copy live VMs, then power off**) minimizes downtime but still requires a brief power-off during the final cutover phase to synchronize the last changed blocks. Applications should be tested for graceful handling of this cutover reboot.

:::caution
Before migrating, verify that your application starts cleanly after a cold reboot. Applications that require manual intervention to restart (e.g., databases with crash-inconsistent state) should be cleanly shut down inside the VM before initiating cold migration.
:::
