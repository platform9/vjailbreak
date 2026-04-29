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
| HKLM:\SYSTEM\CurrentControlSet\Services\vnetWFP | **Present** | Not Found | Not Found | Not Found | Not Found | Not Found |

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

Devices with **Error** status indicate a residual device entry whose driver was removed with VMware Tools.

| Device Name | 2012 | 2016 | 2019 | 2022 | 2025 | Win11 |
|---|---|---|---|---|---|---|
| VMware VMCI Host Device | Error | Error | Error | Not Found | Not Found | Not Found |
| VMware Pointing Device | Error | Error | Error | Not Found | Not Found | Error |
| **Total devices found** | **2** | **2** | **2** | **0** | **0** | **1** |

### 6. Impact of Remaining Artifacts

| Artifact | Versions | Impact |
|---|---|---|---|
| `vmmouse.sys` + `HKLM:\SYSTEM\CurrentControlSet\Services\vmmouse` | Win11 | Residual VMware mouse driver. After migration of VMware hypervisor, there's no VMware hardware to drive, so it's inert. The Pointing Device shows Error in Device Manager but Windows falls back to standard HID drivers — mouse input works normally. |
| `HKLM:\SOFTWARE\VMware, Inc.` | 2012 | Metadata-only registry key left by the VMware installer. No services load from it, no runtime effect. May appear in software inventory/audit tools as VMware still "installed" but it's not. |
| `HKLM:\SYSTEM\CurrentControlSet\Services\vnetWFP` | 2012 | VMware virtual network Windows Filtering Platform driver. The service entry remains but since the driver binary is gone, Windows will fail to start it silently. No network degradation observed. |
| VMware Pointing Device / VMCI Host Device (Error) | 2012, 2016, 2019, Win11 | Phantom device entries in Device Manager with no loaded driver (Code 28). Cosmetic only — no runtime effect, no performance impact, no BSOD risk. Windows ignores driver-less device entries during normal operation. |

**Summary:**
- None of these remnants are harmful to VM operation or stability post-migration.
- The `vnetWFP` service key on Windows 2012 is the most noteworthy from a compliance audit standpoint, but has no observed runtime impact.
- The Error devices in Device Manager are cosmetic — they do not affect functionality.
