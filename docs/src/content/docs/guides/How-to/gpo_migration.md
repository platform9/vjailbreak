---
title: Group Policy Object (GPO) VM Migration
description: Guide for disabling Group Policy settings that may interfere with VM migration operations
---

## Overview

Some Group Policy Object (GPO) settings can interfere with driver injection, which is critical for a successful VM migration. **GPO is a Windows-specific feature** that manages system settings for Windows VMs. Temporarily disabling these policies ensures that driver injection can proceed reliably.

### What is Affected by GPO

**Driver installation cannot be done properly, leading to:**
- Blue Screen of Death (BSOD)
- Windows VM getting stuck in boot loop
- Migration failures due to hardware driver conflicts

### What to Do

**Option 1: Disable via GUI - Step by Step**

1. Open Group Policy Editor (`gpedit.msc`)

2. Navigate to Driver Policies

   Go to:
   ```
   Computer Configuration
    → Administrative Templates
      → System
        → Device Installation
   ```

3. Disable ALL restrictive policies here

   Check and set these to Not Configured (or Disabled where applicable):

   - 🚫 **Critical ones (must fix)**
     - `Prevent installation of devices not described by other policy settings` → Set to Not Configured
     - `Prevent installation of devices that match any of these device IDs` → Not Configured
     - `Prevent installation of devices using drivers that match these device setup classes` → Not Configured

**Option 2: Sure Shot Way - Remove GPO via PowerShell**
```powershell
Remove-Item -Recurse -Force "C:\Windows\System32\GroupPolicy" -ErrorAction SilentlyContinue
Remove-Item -Recurse -Force "C:\Windows\System32\GroupPolicyUsers" -ErrorAction SilentlyContinue
Remove-Item -Recurse -Force "HKLM:\Software\Policies" -ErrorAction SilentlyContinue
Remove-Item -Recurse -Force "HKCU:\Software\Policies" -ErrorAction SilentlyContinue

gpupdate /force
```

:::warning
The PowerShell method completely removes all local Group Policy settings. Use this only when GUI methods fail or when you need to ensure complete GPO removal.
:::