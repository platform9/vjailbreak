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

vJailbreak provides automated scripts to detect and fix this issue during first boot after migration.

### Available Scripts

Two BAT scripts are available in the `scripts/firstboot/windows/` directory. These scripts generate PowerShell scripts that run on first boot:

1. **`check-disks.bat`** - Generates a diagnostic PowerShell script that only checks disk status
2. **`disk-online-fix.bat`** - Generates an automated fix PowerShell script that brings offline disks online

**Usage**: Copy the contents of either BAT file and paste it into the **Post Migration Script** field in the migration form. The script will execute automatically on first boot after migration.

### Script 1: Check Disks (Diagnostic Only)

The `check-disks.bat` script generates `check-disks.ps1` which performs a read-only analysis:

- Scans all physical disks
- Reports operational status (Online/Offline)
- Lists partitions and drive letter assignments
- Identifies disks without drive letters
- Checks for read-only disks and health issues
- Generates a detailed report at `C:\DiskStatus_Report.txt`


### Script 2: Disk Online Fix (Automated Repair)

The `disk-online-fix.bat` script generates and executes `check-disks-fix.ps1` which automatically fixes offline disk issues:

- Performs all diagnostic checks from Script 1
- **Automatically brings ALL offline disks online**
- Logs all actions to `C:\DiskStatus_Report.txt`

**Note**: The generated PowerShell script (`check-disks-fix.ps1`) is a separate file from the diagnostic-only `check-disks.ps1`. It includes all diagnostic functionality plus automated repair capabilities.


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

- Review the diagnostic output from `check-disks.bat` before running the automated fix
- Document which disks should be online in your source environment
- Test the migration process with a non-critical VM first
- Review the log file at `C:\DiskStatus_Report.txt` after running the fix script
- Manually verify that all expected drives are accessible after the fix

## Prevention

To prevent this issue in future migrations, you can:

1. **Pre-configure SAN Policy**: Before migration, set the SAN policy on the source VM to `OnlineAll`
2. **Post-migration Automation**: Copy the contents of `disk-online-fix.bat` into the Post Migration Script field in the migration form
3. **Document Disk States**: Maintain documentation of which disks should be online/offline for each VM

