---
title: VMware Residual Artifacts
description: Residual VMware artifacts
---

In v0.4.1, the following artifacts remain on the windows vms after selecting "Remove VMware Tools" option:


### 1. VMware Driver Files

| Driver File Path | 2012 | 2016 | 2019 | 2022 | 2025 |
|---|---|---|---|---|---|
| C:\Windows\System32\drivers\vmci.sys | Not Found | Not Found | Not Found | Not Found | Not Found |
| C:\Windows\System32\drivers\vm3dmp.sys | Not Found | Not Found | Not Found | Not Found | Not Found |
| C:\Windows\System32\drivers\vmaudio.sys | Not Found | Not Found | Not Found | Not Found | Not Found |
| C:\Windows\System32\drivers\vmhgfs.sys | Not Found | Not Found | Not Found | Not Found | Not Found |
| C:\Windows\System32\drivers\vmmemctl.sys | Not Found | Not Found | **Present** | Not Found | **Present** |
| C:\Windows\System32\drivers\vmmouse.sys | Not Found | Not Found | **Present** | Not Found | Not Found |
| C:\Windows\System32\drivers\vmrawdsk.sys | Not Found | Not Found | Not Found | Not Found | Not Found |
| C:\Windows\System32\drivers\vmtools.sys | Not Found | Not Found | Not Found | Not Found | Not Found |
| C:\Windows\System32\drivers\vmusbmouse.sys | Not Found | Not Found | Not Found | Not Found | Not Found |
| C:\Windows\System32\drivers\vmvss.sys | Not Found | Not Found | Not Found | Not Found | Not Found |
| C:\Windows\System32\drivers\vsock.sys | Not Found | Not Found | Not Found | Not Found | Not Found |
| C:\Windows\System32\drivers\vmx_svga.sys | Not Found | Not Found | Not Found | Not Found | Not Found |
| C:\Windows\System32\drivers\vmxnet3.sys | Not Found | Not Found | Not Found | Not Found | Not Found |
| C:\Windows\System32\drivers\vm3dmp-stats.sys | **Present** | Not Found | **Present** | **Present** | **Present** |
| C:\Windows\System32\drivers\vm3dmp_loader.sys | **Present** | Not Found | **Present** | **Present** | **Present** |
| C:\Windows\System32\drivers\vm3dmp-debug.sys | **Present** | Not Found | **Present** | **Present** | **Present** |

### 2. VMware Registry Keys

| Registry Key Path | 2012 | 2016 | 2019 | 2022 | 2025 |
|---|---|---|---|---|---|
| HKLM:\SYSTEM\CurrentControlSet\Services\vm3dmp_loader | **Present** | **Present** | **Present** | **Present** | **Present** |
| HKLM:\SYSTEM\CurrentControlSet\Services\vm3dmp-debug | **Present** | **Present** | **Present** | **Present** | **Present** |
| HKLM:\SYSTEM\CurrentControlSet\Services\vm3dmp-stats | **Present** | **Present** | **Present** | **Present** | **Present** |
| HKLM:\SYSTEM\CurrentControlSet\Services\vmrawdsk | **Present** | Not Found | Not Found | Not Found | Not Found |
| HKLM:\SYSTEM\CurrentControlSet\Services\vnetWFP | **Present** | Not Found | Not Found | Not Found | Not Found |
| All other VMware registry keys (VMware, Inc., vmci, vm3dmp, etc.) | Not Found | Not Found | Not Found | Not Found | Not Found |

### 3. VMware Folders

| Folder Path | 2012 | 2016 | 2019 | 2022 | 2025 |
|---|---|---|---|---|---|
| C:\Program Files\VMware | **Present** | **Present** | **Present** | Not Found | **Present** |
| C:\ProgramData\Microsoft\Windows\Start Menu\Programs\VMware | **Present** | **Present** | **Present** | Not Found | **Present** |
| All other VMware folders (Program Files (x86), Common Files, ProgramData\VMware) | Not Found | Not Found | Not Found | Not Found | Not Found |

### 4. Startup Entries

| Startup Entry | 2012 | 2016 | 2019 | 2022 | 2025 |
|---|---|---|---|---|---|
| vmtoolsd (HKLM:\Software\Microsoft\Windows\CurrentVersion\Run) | **Present** | Not Found | **Present** | Not Found | **Present** |

### 5. VMware Devices (Device Manager)

| Device Name | 2012 | 2016 | 2019 | 2022 | 2025 |
|---|---|---|---|---|---|
| VMware VMCI Host Device | Error | Error | Not Found | Not Found | Not Found |
| VMware Pointing Device | Error | Error | Error | Not Found | Error |
