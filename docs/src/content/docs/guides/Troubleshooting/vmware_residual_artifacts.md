---
title: VMware Residual Artifacts
description: Residual VMware artifacts
---

In v0.4.3, the following artifacts remain on the Windows VMs after selecting "Remove VMware Tools" option:


### 1. VMware Driver Files

| Driver File Path | 2012 | 2016 | 2019 | 2022 | 2025 |
|---|---|---|---|---|---|
| C:\Windows\System32\drivers\vmci.sys | Not Found | Not Found | Not Found | Not Found | Not Found |
| C:\Windows\System32\drivers\vm3dmp.sys | Not Found | Not Found | Not Found | Not Found | Not Found |
| C:\Windows\System32\drivers\vmaudio.sys | Not Found | Not Found | Not Found | Not Found | Not Found |
| C:\Windows\System32\drivers\vmhgfs.sys | Not Found | Not Found | Not Found | Not Found | Not Found |
| C:\Windows\System32\drivers\vmmemctl.sys | Not Found | Not Found | **Present** (v7.5.7.0) | Not Found | **Present** (v7.5.7.0) |
| C:\Windows\System32\drivers\vmmouse.sys | Not Found | Not Found | **Present** (v12.5.12.0) | Not Found | Not Found |
| C:\Windows\System32\drivers\vmrawdsk.sys | Not Found | Not Found | Not Found | Not Found | Not Found |
| C:\Windows\System32\drivers\vmtools.sys | Not Found | Not Found | Not Found | Not Found | Not Found |
| C:\Windows\System32\drivers\vmusbmouse.sys | Not Found | Not Found | Not Found | Not Found | Not Found |
| C:\Windows\System32\drivers\vmvss.sys | Not Found | Not Found | Not Found | Not Found | Not Found |
| C:\Windows\System32\drivers\vsock.sys | Not Found | Not Found | Not Found | Not Found | Not Found |
| C:\Windows\System32\drivers\vmx_svga.sys | Not Found | Not Found | Not Found | Not Found | Not Found |
| C:\Windows\System32\drivers\vmxnet3.sys | Not Found | Not Found | Not Found | Not Found | Not Found |
| C:\Windows\System32\drivers\vm3dmp-stats.sys | Not Found | Not Found | Not Found | Not Found | Not Found |
| C:\Windows\System32\drivers\vm3dmp_loader.sys | Not Found | Not Found | Not Found | Not Found | Not Found |
| C:\Windows\System32\drivers\vm3dmp-debug.sys | Not Found | Not Found | Not Found | Not Found | Not Found |

### 2. VMware Registry Keys

| Registry Key Path | 2012 | 2016 | 2019 | 2022 | 2025 |
|---|---|---|---|---|---|
| All VMware registry keys (SOFTWARE\VMware, Inc., Services\vm3dmp\*, Services\vmrawdsk, Services\vnetWFP, etc.) | Not Found | Not Found | Not Found | Not Found | Not Found |

### 3. VMware Folders

| Folder Path | 2012 | 2016 | 2019 | 2022 | 2025 |
|---|---|---|---|---|---|
| C:\Program Files\VMware | **Present** (0.14 MB) | Not Found | **Present** (0.14 MB) | Not Found | **Present** (0.14 MB) |
| C:\ProgramData\Microsoft\Windows\Start Menu\Programs\VMware | Not Found | Not Found | Not Found | Not Found | Not Found |
| All other VMware folders (Program Files (x86), Common Files, ProgramData\VMware) | Not Found | Not Found | Not Found | Not Found | Not Found |

### 4. Startup Entries

| Startup Entry | 2012 | 2016 | 2019 | 2022 | 2025 |
|---|---|---|---|---|---|
| vmtoolsd (HKLM:\Software\Microsoft\Windows\CurrentVersion\Run) | Not Found | Not Found | Not Found | Not Found | Not Found |

### 5. VMware Devices (Device Manager)

Devices with **Error** status indicate a missing driver for a residual device entry. Devices with **Unknown** status are VMware hypervisor hardware detected by Windows and are expected when running on a VMware hypervisor regardless of VMware Tools installation.

| Device Name | 2012 | 2016 | 2019 | 2022 | 2025 |
|---|---|---|---|---|---|
| VMware VMCI Host Device | Error | Not Found | Not Found | Not Found | Not Found |
| VMware VMCI Bus Device | Not Found | Unknown | Unknown | Unknown | Unknown |
| VMware Pointing Device | Error | Unknown | Error + Unknown | Unknown | Error + Unknown |
| VMware USB Pointing Device | Not Found | Unknown | Unknown | Unknown | Unknown |
| NECVMWar VMware SATA CD00 | Not Found | Unknown | Unknown | Unknown | Unknown |
| VMware SVGA 3D | Not Found | Unknown | Unknown | Unknown | Unknown |
| VMware Virtual disk SCSI Disk Device | Not Found | Unknown | Unknown | Unknown | Unknown |
| VMware, Inc. VMware20,1 | Not Found | Not Found | Not Found | Not Found | Unknown |
| **Total devices found** | **2** | **6** | **7** | **6** | **8** |

> **Note:** The remaining artifacts (`vmmemctl.sys`, `vmmouse.sys`, residual `C:\Program Files\VMware` folder, and VMware devices with Error status) will be further addressed and cleaned up in upcoming releases.
