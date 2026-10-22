---
title: Windows Offline Disks After Migration
description: Troubleshooting guide for Windows VMs with offline disks after migration from VMware to PCD
---

# Windows Offline Disks After Migration

## Problem Description

After migrating a Windows VM from VMware vCenter to PCD using vJailbreak, additional disks (beyond the primary C: drive) may not be visible inside the Windows operating system, even though they are successfully attached to the VM in PCD.

### Symptoms

- The VM migrates successfully and shows as active in PCD
- PCD shows all volumes/disks are attached (e.g., 3 disks attached)
- Inside the Windows VM, only the primary C: drive is visible
- Additional drives (e.g., E:, F:, G:) that existed before migration are missing
- The disks exist but are in an "Offline" state in Windows Disk Management

### Root Cause

This issue occurs due to Windows SAN Policy settings. When Windows detects disks on a SAN (Storage Area Network), it applies a policy that determines whether new disks are automatically brought online or kept offline.

After migration from VMware to PCD, the storage subsystem changes, and Windows may apply the **"Offline Shared"** SAN policy to the migrated disks. This policy keeps disks offline by default to prevent data corruption in shared storage scenarios.

The default SAN policies in Windows are:
- **Offline Shared**: Keeps all shared disks offline (common after migration)
- **Offline All**: Keeps all new disks offline
- **Online All**: Automatically brings all new disks online

## Manual Workaround

Before migrating the VM, you can manually fix this issue by changing the SAN policy and bringing disks online.

### Step 1: Check Current SAN Policy

Open Command Prompt as Administrator and run:

```cmd
C:\> diskpart
DISKPART> SAN
```

You will likely see:
```
SAN Policy : Offline Shared
```

### Step 2: Change SAN Policy to Online All

```cmd
DISKPART> SAN POLICY=OnlineAll
```

### Step 3: Verify the Change

```cmd
DISKPART> SAN
```

You should now see:
```
SAN Policy : Online All
```

### Step 4: Bring Offline Disks Online

While still in diskpart:

```cmd
DISKPART> list disk
```

Identify offline disks (marked with an asterisk *), then for each offline disk:

```cmd
DISKPART> select disk <number>
DISKPART> online disk
```

### Step 5: Exit and Verify

```cmd
DISKPART> exit
```

Check File Explorer - your drives (E:, F:, G:, etc.) should now be visible.

## Automated Solution

vJailbreak fixes this issue automatically. During conversion, it adds a PowerShell script, `disk-online-fix.ps1`, to every converted Windows VM. The script runs on the VM's first boot after migration. You do not need to copy or paste anything into the migration form.

The script:

- Sets the SAN policy to `OnlineAll` (as of v0.6.0)
- Scans all physical disks and reports their operational status (Online/Offline)
- Lists partitions and drive letter assignments, and identifies disks without drive letters
- **Brings all offline disks online**
- Clears the read-only attribute on disks other than the boot and system disks (as of v0.6.0; earlier versions only reported read-only disks)
- Writes a detailed report of everything it found and did to `C:\DiskStatus_Report.txt` on the migrated VM, and logs an error for any disk that is still offline or read-only

On a Windows failover cluster node (where the Cluster service is installed), the script changes nothing: it leaves the SAN policy and all disks as they are, so that shared cluster disks stay under the cluster's control.

:::note
Older versions of this guide described two BAT scripts (`check-disks.bat` and `disk-online-fix.bat`) to paste into the post-migration script field. That is no longer needed: `disk-online-fix.ps1` runs automatically on every converted Windows VM.
:::


## ⚠️ Important Warnings

### Blanket Online Policy

The automated fix script uses a **blanket approach** to bring ALL offline disks online without discrimination. This is necessary because:

- Pre-migration disk states (online/offline) are unknown
- The script cannot determine if a disk was intentionally kept offline before migration
- This is designed for automated firstboot scenarios after VM conversion

### Risks and Considerations

1. **Intentionally Offline Disks**: If the source VM had disks that were intentionally kept offline (for backup, security, or operational reasons), the script will bring them online. This may not align with your original configuration.

2. **Shared Storage**: In environments with shared storage, bringing disks online indiscriminately could potentially cause issues if the same disk is accessed by multiple systems.

3. **Testing Required**: Always test this script in a non-production environment first to ensure it aligns with your migration policy.

4. **No Drive Letter Assignment**: The script does NOT automatically assign drive letters to partitions. If a partition lacks a drive letter, you must assign it manually using Disk Management or PowerShell cmdlets.

### Recommendations

- Review the report at `C:\DiskStatus_Report.txt` on each migrated Windows VM
- Document which disks should be online in your source environment
- Test the migration process with a non-critical VM first
- Manually verify that all expected drives are accessible after the fix

## Prevention

To prevent this issue in future migrations, you can:

1. **Pre-configure SAN Policy**: As of v0.6.0, the script sets the SAN policy to `OnlineAll` itself. Set it on the source VM before migration only for failover cluster nodes, VMs whose system volume is on a dynamic disk (see below), or earlier vJailbreak versions
2. **Post-migration Automation**: No action is needed. vJailbreak runs `disk-online-fix.ps1` automatically on the first boot of every converted Windows VM
3. **Document Disk States**: Maintain documentation of which disks should be online/offline for each VM

:::caution[LDM system volumes]
The automated solution above relies on firstboot scripts, which `virt-v2v` installs
during conversion. Windows VMs whose system volume is on a dynamic disk (LDM) skip
conversion, so `disk-online-fix.ps1` does not run for them — the SAN policy
**must** be set on the source VM beforehand. Leaving a disk offline on these guests
breaks the LDM volume set. See
[Windows Dynamic Disk (LDM) Migration](../../how-to/windows-ldm-migration/).
:::

