### Overview
This document explains the VMware Tools removal script usage for completely removing VMware Tools from Windows machines after migration

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
C:\Program Files\guestfs\Firstboot\scripts-done\vmware-tools-removal.ps1
```

![img2](<scripts-done-folder.png>)

#### Step 2: Immediate Cleanup Operations
The PowerShell script performs the following:

##### A. File and Directory Removal
Removes VMware Tools installations from:
- `C:\Program Files\VMware`
- `C:\Program Files (x86)\VMware`
- `C:\Program Files\Common Files\VMware`
- `C:\Program Files (x86)\Common Files\VMware`
- `C:\ProgramData\VMware`
- `C:\Users\[All Users]\AppData\Local\VMware`
- `C:\Users\[All Users]\AppData\Roaming\VMware`

##### B. Driver Removal
Deletes VMware drivers from `C:\Windows\System32\drivers`
- Any additional files matching `*vmware*` or `*vm*.sys` patterns


#### Step 3: Registry Cleanup (Delayed)
Creates a startup script for registry cleanup that runs on next boot:
```
C:\ProgramData\Microsoft\Windows\Start Menu\Programs\Startup\vmware_tools_removal.bat
```

This startup script removes registry keys from:
- `HKEY_LOCAL_MACHINE\SOFTWARE\VMware, Inc.`
- `HKEY_LOCAL_MACHINE\SOFTWARE\VMware`
- `HKEY_LOCAL_MACHINE\SOFTWARE\WOW6432Node\VMware, Inc.`
- `HKEY_LOCAL_MACHINE\SOFTWARE\WOW6432Node\VMware`
- Multiple VMware service entries under `HKEY_LOCAL_MACHINE\SYSTEM\CurrentControlSet\Services\`

#### 

---

## 3. Troubleshooting and Verification

### Log File Location
All operations are logged to:
```
C:\VMware_Removal_Log.txt
```

![img3](<VMware_Removal_Log.png>)
![img3-1](<c-drive-path.png>)

### Log File Contents
The log includes:
- Timestamp for each operation
- Success/failure status for each removal attempt
- error messages for any failures
- List of all files, drivers processed


### Verification Steps

The script's success or failure can be determined by checking its location after migration:

1. **File System:**
   ```
   - C:\Program Files\VMware (should not exist)
   - C:\Program Files (x86)\VMware (should not exist)
   - C:\ProgramData\VMware (should not exist)
   ```

2. **Device Manager:**
   - Open Device Manager
   - Check for any devices with "VMware" in the name
   - All VMware devices should be removed

3. **Registry:**
   - Open Registry Editor (regedit)
   - Navigate to `HKEY_LOCAL_MACHINE\SOFTWARE\`
   - Verify no "VMware" or "VMware, Inc." keys exist

4. **Log File:**
   - Review `C:\VMware_Removal_Log.txt`
   - Confirm all operations completed successfully

#### Script Succeeded
If the script executed successfully, it will be moved to:
```
C:\Program Files\guestfs\Firstboot\scripts-done\
```

![img4](<scripts-done-folder.png>)

#### Script Failed
If the script failed during execution, it will remain in:
```
C:\Program Files\guestfs\Firstboot\scripts\
```
