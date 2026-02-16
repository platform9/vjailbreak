---
title: Windows Dynamic Disk (LDM) migration issue
description: Migration failures when converting Windows VMs with dynamic disks (LDM) using virt-v2v.
---

## Problem Statement

We are encountering failures when attempting to migrate Windows VMs with dynamic disks (LDM - Logical Disk Manager) from VMware to KVM through VJailbreak's migration pipeline. VJailbreak uses virt-v2v internally for VM conversion, and the migration fails during the virt-v2v inspection phase with Hivex-related errors when attempting to read Windows registry hives.

## VJailbreak Context

**VJailbreak** The migration workflow involves:

1. **Migration Trigger**: User creates a Migration specifying source VMs
2. **Migration Controller**: VJailbreak controller reconciles the Migration
3. **Virt-v2v Execution**: Controller spawns virt-v2v pods to perform the actual conversion
4. **Conversion Phase**: Virt-v2v converts VMware disk formats and installs KVM drivers
5. **Import Phase**: Converted VMs are imported into the target KVM environment

**The LDM issue manifests during step 3-4**, where virt-v2v fails to inspect and convert Windows VMs with dynamic disks.

## Technical Background

### What is LDM?

- **LDM (Logical Disk Manager)** is Windows' proprietary volume management system for "dynamic disks"
- Similar in concept to Linux LVM, but incompatible with standard partition tables
- Stores volume metadata in a 1MB journaled database at the end of each disk
- Commonly used for spanning volumes across multiple disks, mirroring, or RAID configurations

### How Virt-v2v Works

1. **Inspection Phase**: Uses libguestfs APIs to detect OS, read registry hives via Hivex
2. **Mounting**: Mounts filesystems based on inspection data
3. **Conversion**: Installs VirtIO drivers, removes old hypervisor drivers, modifies registry
4. **Registry Operations**: Requires both read and write access to SYSTEM and SOFTWARE hives

## Root Cause Analysis

### Why LDM Causes Failures

**Inspection Dependency Chain:**

```
VJailbreak MigrationTrigger Controller
  └─> Virt-v2v Pod
      └─> libguestfs inspection
          └─> Hivex library (registry reading)
              └─> Direct file access to registry hives
                  └─> FAILS if hives are on LDM volumes
```

### Linux LDM Support Limitations

1. LDM volumes may not be properly assembled by libguestfs
2. Registry hives may be fragmented across LDM volumes
3. Hivex expects complete, contiguous registry files
4. Even if volumes assemble, registry data may appear corrupted

## Impact on VJailbreak Migration Workflow

## Recommended Solutions

### Solution 1: Pre-Migration Disk Conversion (RECOMMENDED)

**Steps:**

1. **Before creating Migration**, boot Windows VM in VMware vSphere
2. Disable Fast Startup:

   ```powershell
   powercfg /h off
   ```

3. Convert dynamic disks to basic:

   ```cmd
   diskpart
   list disk
   select disk 1
   convert basic
   select disk 2
   convert basic
   ```

4. Clean shutdown: `shutdown /s /f /t 0`
5. Do the migration in vJailbreak

**Note:** `convert basic` requires empty disk. If data exists, must backup → clean → convert → restore.

**In VJailbreak:** Retry the migration

## Impact on Non-Root Disks

**Important:** Virt-v2v only inspects the **root/boot disk** where Windows is installed.

| Configuration | Result | Action |
|---------------|--------|--------|
| Root: Basic, Data: LDM | Works | Import LDM disks in Windows post-migration |
| Root: LDM, Data: Basic | Fails | Must convert root disk to basic |
| Root: LDM, Data: LDM | Fails | Must convert root disk (data optional) |
| All: Basic | Works | No action needed |

## Conclusion

**Primary Recommendation for VJailbreak Users:** Before creating MigrationPlans for Windows VMs, verify disk configuration in VMware vSphere. If dynamic disks are detected, boot the VM, disable Fast Startup, convert dynamic disks to basic using diskpart, perform a clean shutdown, then proceed with VJailbreak migration.
