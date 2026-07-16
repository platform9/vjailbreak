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

## Active Directory-Joined VMs

### Domain Controllers

Migrating Active Directory Domain Controller VMs is **strongly not recommended**. The core risk is specific to how vJailbreak works: `virt-v2v` performs a disk-level conversion and creates a new VM on a different hypervisor. **VM-GenerationID** — the hypervisor metadata that Windows Server 2012+ uses to detect unsafe restores — is not stored on disk and is not preserved through this process. The migrated DC starts with a new (or absent) VMGenID, which Windows AD treats as an unsafe restore/clone.

What happens depends on the Windows version:

- **Windows Server 2012 and later**: The lost VMGenID triggers Windows' built-in safeguards. The DC automatically resets its invocation ID and forces a non-authoritative resync against replication partners. The domain may recover if other DCs are reachable, but this is unreliable in production and is not a supported migration path.
- **Windows Server 2008 R2 and earlier** (no VMGenID support): A genuine **USN rollback** can occur. The domain silently stops accepting replication from the migrated DC, and the AD environment can diverge without obvious errors. This is difficult to detect and hard to recover from.

In both cases the **source DC must be permanently removed from the domain** before or immediately after the migrated copy is brought online. Running both simultaneously on the same domain will corrupt AD.

**Recommended approach** (from [Microsoft guidance](https://learn.microsoft.com/en-us/troubleshoot/windows-server/active-directory/detect-and-recover-from-usn-rollback)):

1. **Provision a new DC** in the target OpenStack environment using standard AD promotion.
2. **Let AD replication populate it** from an existing domain controller.
3. **Decommission the source DC** via `dcpromo` or Server Manager once replication is verified complete.

**If you must migrate a DC** (lab/test environments, single-DC setups with no alternative), take these precautions:

- Cleanly shut down the source DC before migration — do not snapshot a running DC.
- Migrate only one DC at a time.
- After the migrated DC boots, verify replication health immediately:
  ```cmd
  repadmin /replsummary
  dcdiag /test:replications
  ```
- Confirm time synchronization (Kerberos requires clocks within 5 minutes of each other).
- Verify DNS is resolving correctly for all domain members.
- Decommission the source DC immediately — never run the original and migrated DC on the same domain simultaneously.

### Member Servers and Workstations

Migrating domain-joined **member VMs** (non-DC servers and workstations) is generally safe. The machine account password is stored in the VM's own LSA secrets and is copied with the disk, so domain membership typically survives the migration intact.

A few edge cases can cause domain authentication to fail post-migration:

- **Kerberos clock skew**: If the migrated VM's clock is more than 5 minutes off from the domain controller, Kerberos authentication will fail. Sync the VM's clock immediately after boot.
- **DNS resolution failures**: The VM must be able to resolve the domain controller's name and locate AD SRV records. Verify DNS settings after migration.
- **Pre-existing stale computer account**: If the source VM had been offline for an extended period (typically 90+ days) before migration, the domain controller may have already invalidated its computer account. This is a pre-existing condition unrelated to the migration itself.

If users see `The trust relationship between this workstation and the primary domain failed` after migration, run the following to reset the account:

```powershell
# Option 1 — reset computer account password without rejoining
netdom resetpwd /server:<domain-controller> /userd:<domain\admin> /passwordd:*

# Option 2 — rejoin the domain
Remove-Computer -WorkgroupName WORKGROUP -Force
Add-Computer -DomainName <domain> -Credential <domain\admin> -Restart
```

## Persist Network: Windows Server 2012 and Below

The **Persist source network interfaces** option does not work for Windows Server 2012 and earlier (including Windows Server 2008 R2 and Windows Server 2008).

Network interface name persistence depends on PowerShell capabilities, the Windows registry structure for network adapters, and a compatible version of `pnputil`. These prerequisites are not met on Windows Server 2012 and earlier.

**Workaround**: Manually reconfigure network interface names and static IP settings inside the VM after migration.

## Assign IP and Persist Network Cannot Be Used Together

The **Assign IP** and **Persist Network** (Persist source network interfaces) options are mutually exclusive. Enabling both simultaneously produces undefined behavior and the migration may not apply either setting correctly.

**Rule**: Use one or the other — not both.

- Use **Assign IP** when you need to set a specific IP address on the destination VM.
- Use **Persist Network** when you need to preserve the source VM's interface names and static routes.

## Multi-IP Assignment Not Supported

Only one IP address per network interface is supported in the **Assign IPs** field. The UI enforces this — the field accepts a single IP per interface. If multiple IPs are specified via CLI, the migration will fail.

**Workaround**: Assign additional IPs manually inside the VM after migration, or use OpenStack port configuration to attach additional floating IPs post-migration.

## VMware Tools Removal: Residual Artifacts

The VMware Tools removal process performed by `virt-v2v` during migration may leave behind residual files and registry entries on the destination VM.

These artifacts are typically harmless but may appear in application logs or security scans.

For a full list of known residual artifacts and cleanup steps, see: [VMware Residual Artifacts](../../guides/troubleshooting/vmware_residual_artifacts/).

## Multi-Boot VMs Not Supported

vJailbreak does not support VMs with **multiple bootable operating systems** (multi-boot configurations). `virt-v2v` inspects only a single OS installation per VM and cannot convert multi-boot disk layouts.

**Workaround**: Migrate each OS as a separate VM, or convert the disk to a single-boot configuration before migration.

## SUSE Linux (SLES / SLED) with Legacy GRUB 0.97

Older SUSE-family VMs — **SLES**, **SLED**, and other **SUSE** distributions — that still boot with **legacy GRUB (0.97)** require special handling. These are typically BIOS VMs on a multi-disk layout, where the first boot stage sits in one disk's MBR while its second stage and `/boot` live on a separate disk. After migration to KVM, the virtual disks are re-numbered and no longer match the original VMware ordering, so GRUB cannot find its second stage and the VM fails to boot with `GRUB Error 21`.

**Why we upgrade GRUB**: GRUB 0.97 is too old and fragile — it hard-codes disk numbers and block offsets that break the moment the hypervisor re-orders disks. `virt-v2v` also can't reconfigure GRUB 0.97 for KVM; it only manages GRUB2. But on these older SUSE releases, GRUB2 ships only as an EFI build (no legacy-BIOS version), so upgrading GRUB forces a switch to UEFI — which is exactly why a self-contained **EFI System Partition (ESP)** is required.

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

## PCI Slot Exhaustion When Attaching Disks with virtio-blk

During conversion, vJailbreak attaches the target volumes to the vJailbreak VM (or its agent VMs). If the vJailbreak image is uploaded without a disk bus setting, OpenStack uses the default **virtio-blk** bus, where every attached volume consumes its own PCI slot. Migrating VMs with many disks, or running many parallel migrations on one agent, Maximum 26 devices can be attached after which PCI slots will exhaust and volume attach fails with:

```text
libvirt.libvirtError: internal error: No more available PCI slots
```

**Workaround**: Set the disk bus to **virtio-scsi** on the vJailbreak image before creating the vJailbreak VM. All attached volumes then share a single SCSI controller (one PCI slot, up to 256 devices):

```bash
openstack image set \
  --property hw_disk_bus=scsi \
  --property hw_scsi_model=virtio-scsi \
  <vjailbreak-image-name-or-ID>
```

:::note
The disk bus is fixed when the VM is created. If the vJailbreak VM is already deployed, recreate it from the updated image. Agent VMs created during scale up use the same image, so set these properties before scaling up.
:::

See the full troubleshooting entry: [Disk attach fails during migration: No more available PCI slots](../../guides/troubleshooting/troubleshooting/#disk-attach-fails-during-migration-no-more-available-pci-slots).

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

## Hot Migration Requires Virtual Hardware Version 7 or Newer

vJailbreak **Hot migration** (**Copy live VMs, then power off**) relies on VMware **Changed Block Tracking (CBT)** to copy only changed disk blocks during the live sync phase. CBT is available only on VMs running **virtual hardware version 7 or newer** (VMware KB 1020128).

VMs on older hardware versions (for example, version 4) do not expose the CBT property at all, so Hot migration cannot track changed blocks for them.

**Symptom**: A Hot migration of a legacy-hardware VM fails at the CBT step. The reported error looks similar to:

```
CBT is not enabled on disk <id>
```

**What to do** — choose one:

1. **Use cold migration** (**Power off VMs, then copy**) for these VMs. Cold migration copies each disk once in full while the VM is powered off and does not use CBT, so it works on any hardware version. *(Recommended — requires no changes to the source VM.)*
2. **Upgrade the VM's virtual hardware version** to 7 or newer in vCenter, then use Hot migration if you need minimal downtime. Upgrading the hardware version requires a VM power-off and cannot be reversed — review VMware's documentation before proceeding.

| VM virtual hardware version | Hot migration | Cold migration |
|---|---|---|
| 7 or newer | Supported | Supported |
| Below 7 (e.g., version 4) | Not supported — use cold migration | Supported |

:::note
To check a VM's hardware version in vCenter, select the VM and look at **VM Hardware → Compatibility** (shown as "ESXi X.X and later (VM version N)").
:::

## Application Reboot During Migration

Cold migration (**Power off VMs, then copy**) powers off the source VM before copying its disk. The destination VM boots fresh after migration completes. **Applications must tolerate a reboot** — any in-memory state, open transactions, or non-persistent connections will be lost.

Hot migration (**Copy live VMs, then power off**) minimizes downtime but still requires a brief power-off during the final cutover phase to synchronize the last changed blocks. Applications should be tested for graceful handling of this cutover reboot.

:::caution
Before migrating, verify that your application starts cleanly after a cold reboot. Applications that require manual intervention to restart (e.g., databases with crash-inconsistent state) should be cleanly shut down inside the VM before initiating cold migration.
:::
