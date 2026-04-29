---
title: VMware Residual Artifacts
description: Residual VMware artifacts
---

In v0.4.4, the following artifacts remain on the Windows VMs after selecting "Remove VMware Tools" option:


### 1. VMware Driver Files

| Driver File Path | 2012 | 2016 | 2019 | 2022 | 2025 | Win11 |
|---|---|---|---|---|---|---|
| C:\Windows\System32\drivers\vmci.sys | Not Found | Not Found | Not Found | Not Found | Not Found | Not Found |
| C:\Windows\System32\drivers\vm3dmp.sys | Not Found | Not Found | Not Found | Not Found | Not Found | Not Found |
| C:\Windows\System32\drivers\vmaudio.sys | Not Found | Not Found | Not Found | Not Found | Not Found | Not Found |
| C:\Windows\System32\drivers\vmhgfs.sys | Not Found | Not Found | Not Found | Not Found | Not Found | Not Found |
| C:\Windows\System32\drivers\vmmemctl.sys | Not Found | Not Found | Not Found | Not Found | Not Found | Not Found |
| C:\Windows\System32\drivers\vmmouse.sys | Not Found | Not Found | Not Found | Not Found | Not Found | **Present** |
| C:\Windows\System32\drivers\vmrawdsk.sys | Not Found | Not Found | Not Found | Not Found | Not Found | Not Found |
| C:\Windows\System32\drivers\vmtools.sys | Not Found | Not Found | Not Found | Not Found | Not Found | Not Found |
| C:\Windows\System32\drivers\vmusbmouse.sys | Not Found | Not Found | Not Found | Not Found | Not Found | Not Found |
| C:\Windows\System32\drivers\vmvss.sys | Not Found | Not Found | Not Found | Not Found | Not Found | Not Found |
| C:\Windows\System32\drivers\vsock.sys | Not Found | Not Found | Not Found | Not Found | Not Found | Not Found |
| C:\Windows\System32\drivers\vmx_svga.sys | Not Found | Not Found | Not Found | Not Found | Not Found | Not Found |
| C:\Windows\System32\drivers\vmxnet3.sys | Not Found | Not Found | Not Found | Not Found | Not Found | Not Found |
| C:\Windows\System32\drivers\vm3dmp-stats.sys | Not Found | Not Found | Not Found | Not Found | Not Found | Not Found |
| C:\Windows\System32\drivers\vm3dmp_loader.sys | Not Found | Not Found | Not Found | Not Found | Not Found | Not Found |
| C:\Windows\System32\drivers\vm3dmp-debug.sys | Not Found | Not Found | Not Found | Not Found | Not Found | Not Found |
| C:\Windows\System32\drivers\vm3dservice.sys | Not Found | Not Found | Not Found | Not Found | Not Found | Not Found |
| C:\Windows\System32\drivers\vmgid.sys | Not Found | Not Found | Not Found | Not Found | Not Found | Not Found |
| C:\Windows\System32\drivers\vmgencounter.sys | Not Found | Not Found | Not Found | Not Found | Not Found | Not Found |
| C:\Windows\System32\drivers\vms3cap.sys | Not Found | Not Found | Not Found | Not Found | Not Found | Not Found |
| C:\Windows\System32\drivers\vmstorfl.sys | Not Found | Not Found | Not Found | Not Found | Not Found | Not Found |

### 2. VMware Registry Keys

| Registry Key Path | 2012 | 2016 | 2019 | 2022 | 2025 | Win11 |
|---|---|---|---|---|---|---|
| HKLM:\SOFTWARE\VMware, Inc. | **Present** | Not Found | Not Found | Not Found | Not Found | Not Found |
| HKLM:\SOFTWARE\VMware | Not Found | Not Found | Not Found | Not Found | Not Found | Not Found |
| HKLM:\SOFTWARE\WOW6432Node\VMware, Inc. | Not Found | Not Found | Not Found | Not Found | Not Found | Not Found |
| HKLM:\SOFTWARE\WOW6432Node\VMware | Not Found | Not Found | Not Found | Not Found | Not Found | Not Found |
| HKLM:\SYSTEM\CurrentControlSet\Services\vmci | Not Found | Not Found | Not Found | Not Found | Not Found | Not Found |
| HKLM:\SYSTEM\CurrentControlSet\Services\vm3dmp | Not Found | Not Found | Not Found | Not Found | Not Found | Not Found |
| HKLM:\SYSTEM\CurrentControlSet\Services\vm3dmp_loader | Not Found | Not Found | Not Found | Not Found | Not Found | Not Found |
| HKLM:\SYSTEM\CurrentControlSet\Services\vm3dmp-debug | Not Found | Not Found | Not Found | Not Found | Not Found | Not Found |
| HKLM:\SYSTEM\CurrentControlSet\Services\vm3dmp-stats | Not Found | Not Found | Not Found | Not Found | Not Found | Not Found |
| HKLM:\SYSTEM\CurrentControlSet\Services\vm3dservice | Not Found | Not Found | Not Found | Not Found | Not Found | Not Found |
| HKLM:\SYSTEM\CurrentControlSet\Services\vmaudio | Not Found | Not Found | Not Found | Not Found | Not Found | Not Found |
| HKLM:\SYSTEM\CurrentControlSet\Services\vmhgfs | Not Found | Not Found | Not Found | Not Found | Not Found | Not Found |
| HKLM:\SYSTEM\CurrentControlSet\Services\VMMemCtl | Not Found | Not Found | Not Found | Not Found | Not Found | Not Found |
| HKLM:\SYSTEM\CurrentControlSet\Services\vmmouse | Not Found | Not Found | Not Found | Not Found | Not Found | **Present** |
| HKLM:\SYSTEM\CurrentControlSet\Services\vmrawdsk | Not Found | Not Found | Not Found | Not Found | Not Found | Not Found |
| HKLM:\SYSTEM\CurrentControlSet\Services\VMRawDisk | Not Found | Not Found | Not Found | Not Found | Not Found | Not Found |
| HKLM:\SYSTEM\CurrentControlSet\Services\VMTools | Not Found | Not Found | Not Found | Not Found | Not Found | Not Found |
| HKLM:\SYSTEM\CurrentControlSet\Services\vmusbmouse | Not Found | Not Found | Not Found | Not Found | Not Found | Not Found |
| HKLM:\SYSTEM\CurrentControlSet\Services\vmvss | Not Found | Not Found | Not Found | Not Found | Not Found | Not Found |
| HKLM:\SYSTEM\CurrentControlSet\Services\vmvsock | Not Found | Not Found | Not Found | Not Found | Not Found | Not Found |
| HKLM:\SYSTEM\CurrentControlSet\Services\VMwareCAF | Not Found | Not Found | Not Found | Not Found | Not Found | Not Found |
| HKLM:\SYSTEM\CurrentControlSet\Services\VMwareCAFCommAmqpListener | Not Found | Not Found | Not Found | Not Found | Not Found | Not Found |
| HKLM:\SYSTEM\CurrentControlSet\Services\VMwareCAFManagementAgentHost | Not Found | Not Found | Not Found | Not Found | Not Found | Not Found |
| HKLM:\SYSTEM\CurrentControlSet\Services\vnetWFP | Not Found | Not Found | Not Found | Not Found | Not Found | Not Found |

### 3. VMware Folders

| Folder Path | 2012 | 2016 | 2019 | 2022 | 2025 | Win11 |
|---|---|---|---|---|---|---|
| C:\Program Files\VMware | Not Found | Not Found | Not Found | Not Found | Not Found | Not Found |
| C:\Program Files (x86)\VMware | Not Found | Not Found | Not Found | Not Found | Not Found | Not Found |
| C:\Program Files\Common Files\VMware | Not Found | Not Found | Not Found | Not Found | Not Found | Not Found |
| C:\Program Files (x86)\Common Files\VMware | Not Found | Not Found | Not Found | Not Found | Not Found | Not Found |
| C:\ProgramData\VMware | Not Found | Not Found | Not Found | Not Found | Not Found | Not Found |
| C:\Users\Administrator\AppData\Local\VMware | Not Found | Not Found | Not Found | Not Found | Not Found | Not Found |
| C:\Users\Administrator\AppData\Roaming\VMware | Not Found | Not Found | Not Found | Not Found | Not Found | Not Found |
| C:\ProgramData\Microsoft\Windows\Start Menu\Programs\VMware | Not Found | Not Found | Not Found | Not Found | Not Found | Not Found |

### 4. Startup Entries

| Startup Entry | 2012 | 2016 | 2019 | 2022 | 2025 | Win11 |
|---|---|---|---|---|---|---|
| vmtoolsd (HKLM:\Software\Microsoft\Windows\CurrentVersion\Run) | Not Found | Not Found | Not Found | Not Found | Not Found | Not Found |

### 5. VMware Devices (Device Manager)

Devices with **Error** status indicate a missing driver for a residual device entry. Devices with **Unknown** status are VMware hypervisor hardware detected by Windows and are expected when running on a VMware hypervisor regardless of VMware Tools installation.

| Device Name | 2012 | 2016 | 2019 | 2022 | 2025 | Win11 |
|---|---|---|---|---|---|---|
| VMware VMCI Host Device | Error | Not Found | Not Found | Not Found | Not Found | Not Found |
| VMware VMCI Bus Device | Not Found | Unknown | Unknown | Unknown | Unknown | Unknown |
| VMware Pointing Device | Error | Unknown | Error + Unknown | Unknown | Error + Unknown | Error + Unknown |
| VMware USB Pointing Device | Not Found | Unknown | Unknown | Unknown | Unknown | Unknown |
| NECVMWar VMware SATA CD00 | Not Found | Unknown | Unknown | Unknown | Unknown | Unknown |
| VMware SVGA 3D | Not Found | Unknown | Unknown | Unknown | Unknown | Unknown |
| VMware Virtual disk SCSI Disk Device | Not Found | Unknown | Unknown | Unknown | Unknown | Unknown (×2) |
| VMware, Inc. VMware20,1 | Not Found | Not Found | Not Found | Not Found | Unknown | Unknown |
| **Total devices found** | **2** | **6** | **7** | **6** | **8** | **9** |

> **Note:** The remaining artifacts (`vmmouse.sys` driver and `HKLM:\SYSTEM\CurrentControlSet\Services\vmmouse` registry key on Win11, `HKLM:\SOFTWARE\VMware, Inc.` registry key on Windows 2012, and VMware devices with Error status) will be further addressed and cleaned up in upcoming releases.
