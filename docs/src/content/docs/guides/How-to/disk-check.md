## Overview
This document explains the Disk Check script usage for verifying disk integrity, drive letter assignments, and disk health status after migration. The script is designed to run automatically after migration completes, ensuring all disks are properly configured and accessible.

---
## 1. Script Deployment
### How to Add the Script
link to scripts: [windows-firstbootscripts](https://github.com/amar-chand/vjailbreak/tree/main/scripts/firstboot/windows)

The script is deployed through the migration form interface:
1. Navigate to the **Migration Options** section in your migration form
2. Check the **Post Migration Script** option
3. Paste the complete contents of `firstbootscripts` into the script field, if you have multiple scripts, append it in the end of the existing script.
4. Start the migration once al the options are set.

![img1](VJB-form.png)
> **Note:** The script contents should be added directly into the migration form
---
## 2. Script Execution Flow
### When Does It Run?
- The script executes **automatically after the migration completes**
### What Happens During Execution?
#### Step 1: PowerShell Script Generation
The batch script creates a PowerShell script at:
```
C:\Program Files\guestfs\Firstboot\scripts-done\check-disk.ps1
```
![img2](<scripts-done-folder.png>)

---

## 3. Detailed Operations
A Physical Disk Scanning

The script identifies:
- All physical disks connected to the system
- Operational status of each disk (Healthy, Warning, Unhealthy)
- Read/Write capabilities
- Total disk capacity in GB

B Partition Verification

Checks for:
- Drive letter assignments for each partition
- Unassigned partitions and their sizes
- Partition types
- Partition sizes in GB

C Issue Detection

Identifies potential issues such as:

- **Offline Disks**: Disks that are not accessible
- **Read-only Disks**: Disks in read-only mode
- **Unhealthy Disks**: Disks reporting errors or warnings
- **Unassigned Partitions**: Partitions without drive letters

---

## 4. Troubleshooting and Verification
### Log File Location
All operations are logged to:

```
C:\DiskStatus_Report.txt
```
![img3](<DiskStatus_Report.png>)
![img3-1](<c-drive-path.png>)
Log Contents Include:

- Timestamp of each operation
- Operational status of each disk
- List of physical disks with detailed info
- Summary of detected issues
- Overall status assessment ( ALL Check Passed / Issues Found )

The script's success or failure can be determined by checking its location after migration:

#### Script Succeeded
If the script executed successfully, it will be moved to:
```
C:\Program Files\guestfs\Firstboot\scripts-done\
```

![img4](scripts-done-folder.png)

#### Script Failed
If the script failed during execution, it will remain in:
```
C:\Program Files\guestfs\Firstboot\scripts\
```