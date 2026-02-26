<#
.SYNOPSIS
    VMware Tools removal script for use with Firstboot-Scheduler.ps1.
    Designed to run across multiple reboots via the scheduler.
    Exit codes:
      0    = VMware Tools fully removed (done)
      3010 = Reboot required to continue removal
      1    = Fatal error / max attempts exceeded
#>

param(
    [string]$WorkDir = "$env:ProgramData\VMwareRemoval",
    [string]$MarkerPath = "$WorkDir\vmware_tools_removed.marker",
    [string]$LogPath = "$WorkDir\VMware_Removal_Log.txt",
    [int]$MaxAttempts = 15
)

$ErrorActionPreference = 'SilentlyContinue'
$Global:didWork = $false

function Write-Log {
    param([string]$Message, [string]$Level = 'INFO')
    $ts = Get-Date -Format 'yyyy-MM-dd HH:mm:ss'
    $line = "[$ts] [$Level] $Message"
    Add-Content -Path $LogPath -Value $line -ErrorAction SilentlyContinue
    Write-Host $line
}

# Create working dir
if (-not (Test-Path $WorkDir)) { New-Item -ItemType Directory -Path $WorkDir -Force | Out-Null }

if (Test-Path $MarkerPath) {
    Write-Log 'Marker found. VMware Tools already removed.'
    exit 0
}

# Attempt counter
$attemptFile = Join-Path $WorkDir 'attempts.txt'
$attempt = 1
if (Test-Path $attemptFile) { $attempt = [int](Get-Content $attemptFile) + 1 }
Set-Content -Path $attemptFile -Value $attempt

if ($attempt -gt $MaxAttempts) {
    Write-Log "Max attempts ($MaxAttempts) reached. Suggest running in Safe Mode." 'ERROR'
    New-Item -ItemType File -Path "$WorkDir\vmware_tools_removed.failed" -Force | Out-Null
    exit 1
}

Write-Log "=== VMware Tools Removal - Run #$attempt of $MaxAttempts ==="

# Get OS version for version-specific logic
$osVersion = [System.Environment]::OSVersion.Version.Major

# ====================== MSI UNINSTALL ======================
function Try-UninstallVMwareTools {
    $uninstallPaths = @(
        'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\*',
        'HKLM:\SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall\*'
    )
    $productCode = $null
    foreach ($path in $uninstallPaths) {
        $entries = Get-ItemProperty -Path $path -ErrorAction SilentlyContinue |
            Where-Object { $_.DisplayName -like '*VMware Tools*' }
        if ($entries) {
            $productCode = $entries[0].PSChildName
            break
        }
    }
    if ($productCode) {
        Write-Log "Attempting MSI uninstall for product: $productCode"
        $args = "/x $productCode /qn /norestart REBOOT=ReallySuppress"
        $proc = Start-Process -FilePath 'msiexec.exe' -ArgumentList $args -Wait -PassThru -ErrorAction SilentlyContinue
        if ($proc) {
            Write-Log "MSI uninstall exit code: $($proc.ExitCode)"
            $Global:didWork = $true
            return $proc.ExitCode
        }
    }
    return $null
}

# MSI Hack if uninstall fails
function Try-MsiHack {
    if ($osVersion -lt 10) {
        Write-Log "Skipping MSI hack on older OS (e.g., 2012) due to COM limitations - falling back to manual cleanup" 'WARNING'
        # Manual fallback for older OS: Delete uninstall reg key
        $uninstallPaths = @(
            'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\*',
            'HKLM:\SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall\*'
        )
        foreach ($path in $uninstallPaths) {
            $entries = Get-ItemProperty -Path $path -ErrorAction SilentlyContinue |
                Where-Object { $_.DisplayName -like '*VMware Tools*' }
            if ($entries) {
                $keyPath = $entries.PSPath
                try {
                    Remove-Item -Path $keyPath -Force
                    Write-Log "Manually deleted uninstall registry key: $keyPath" 'INFO'
                    $Global:didWork = $true
                } catch {
                    Write-Log "Failed to manually delete uninstall reg key: $_" 'WARNING'
                }
            }
        }
        return
    }
    $installer = New-Object -ComObject WindowsInstaller.Installer -ErrorAction SilentlyContinue
    if (-not $installer) {
        Write-Log "Failed to create WindowsInstaller COM object - skipping hack" 'ERROR'
        return
    }
    $productsRoot = 'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Installer\UserData\S-1-5-18\Products'
    $productKeys = Get-ChildItem -Path $productsRoot -ErrorAction SilentlyContinue
    foreach ($pk in $productKeys) {
        $ip = Join-Path $pk.PSPath 'InstallProperties'
        $props = Get-ItemProperty -Path $ip -ErrorAction SilentlyContinue
        if ($props.DisplayName -match 'VMware Tools') {
            $localPackage = $props.LocalPackage
            if (Test-Path $localPackage) {
                Write-Log "Applying MSI hack for: $localPackage" 'WARNING'
                try {
                    $db = $installer.OpenDatabase($localPackage, 2)
                    $q = "DELETE FROM CustomAction WHERE Action = 'VM_LogStart' OR Action = 'VM_CheckRequirements' OR Action LIKE 'VM_%'"
                    $view = $db.OpenView($q)
                    $view.Execute()
                    $view.Close()
                    $db.Commit()
                    $Global:didWork = $true
                   
                    # Retry uninstall after hack (up to 2 times if locked)
                    Write-Log "Retrying msiexec after hack"
                    $retryCount = 0
                    do {
                        $proc = Start-Process 'msiexec.exe' -ArgumentList "/x `"$localPackage`" /qn /norestart REBOOT=ReallySuppress" -Wait -PassThru -ErrorAction SilentlyContinue
                        if ($proc) {
                            Write-Log "MSI hack uninstall exit code: $($proc.ExitCode) (retry $retryCount)"
                            if ($proc.ExitCode -eq 0) { break }
                        }
                        $retryCount++
                        Start-Sleep -Seconds 5
                    } while ($retryCount -lt 2)
                } catch {
                    Write-Log "MSI hack failed: $_" 'ERROR'
                }
                break
            }
        }
    }
    # Force unregister if still registered
    $apps = Get-ItemProperty -Path 'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\*','HKLM:\SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall\*' -ErrorAction SilentlyContinue
    $vmwApps = $apps | Where-Object { $_.DisplayName -like '*VMware Tools*' }
    if ($vmwApps) {
        $productCode = $vmwApps[0].PSChildName
        Start-Process 'msiexec.exe' -ArgumentList "/x $productCode /qn /norestart REBOOT=ReallySuppress" -Wait -ErrorAction SilentlyContinue
    }
}

# Run MSI Uninstall FIRST
$uninstallExit = Try-UninstallVMwareTools
if ($uninstallExit -and $uninstallExit -ne 0 -and $uninstallExit -ne 3010) {
    Write-Log "Standard uninstall failed ($uninstallExit). Trying MSI hack."
    Try-MsiHack
}

# ====================== PROCESSES ======================
$processes = @('vmtoolsd','vm3dservice','VGAuthService','vmwaretray','vmwareuser','vmware-svga') # Added for locked DLLs
foreach ($proc in $processes) {
    try {
        Get-Process -Name $proc | Stop-Process -Force
        $Global:didWork = $true
    } catch {
        Write-Log "Failed to stop process $($proc): $_" 'WARNING'
    }
}
# ====================== SERVICES ======================
$services = @(
    'VMTools','vm3dservice','VGAuthService','VMwareCAF','VMwareCAFCommAmqpListener','VMwareCAFManagementAgentHost',
    'vmci','vm3dmp','vmaudio','vmhgfs','VMMemCtl','vmmouse','VMRawDisk','vmusbmouse','vmvss','vmvsock','vsock','vmxnet3',
    'vmStatsProvider'
)

foreach ($svc in $services) {
    try {
        $s = Get-Service -Name $svc
        Stop-Service -Name $svc -Force
        sc.exe delete $svc 2>&1 | Out-Null
        $Global:didWork = $true
        Write-Log "Deleted service: $($svc)"
    } catch {
        Write-Log "Failed to delete service $($svc): $_" 'WARNING'
    }
}

# ====================== DRIVER FILES ======================
$driversPath = "C:\Windows\System32\drivers"
$targetDrivers = @("vmci.sys", "vm3dmp.sys", "vmaudio.sys", "vmhgfs.sys", "vmmemctl.sys", "vmmouse.sys", "vmrawdsk.sys", "vmtools.sys", "vmusbmouse.sys", "vmvss.sys", "vsock.sys", "vmx_svga.sys", "vmxnet3.sys")
foreach ($d in $targetDrivers) {
    $driverFullPath = Join-Path $driversPath $d
    try {
        if (Test-Path $driverFullPath) {
            Stop-Service -Name $d.Replace('.sys','') -Force -ErrorAction SilentlyContinue
            Remove-Item $driverFullPath -Force -ErrorAction Stop
            $Global:didWork = $true
            Write-Log "Removed driver file: $($d)"
        }
    } catch {
        Write-Log "Failed to remove driver $($d): $_ - attempting rename for reboot delete" 'WARNING'
        $newName = $d + ".delete"
        $newPath = Join-Path $driversPath $newName
        if (Test-Path $newPath) { Remove-Item $newPath -Force -ErrorAction SilentlyContinue }
        Rename-Item -Path $driverFullPath -NewName $newName -Force -ErrorAction SilentlyContinue
        if (Test-Path $newPath) {
            $runOnceKey = "HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\RunOnce"
            Set-ItemProperty -Path $runOnceKey -Name "!DeleteVMwareDriver_$($d.Replace('.sys',''))" -Value "cmd.exe /c del /f /q `"$newPath`"" -Force
            $Global:didWork = $true
        }
    }
}
# ====================== REGISTRY CLEANUP ======================
$regKeys = @(
    'HKLM:\SOFTWARE\VMware, Inc.','HKLM:\SOFTWARE\VMware',
    'HKLM:\SOFTWARE\WOW6432Node\VMware, Inc.','HKLM:\SOFTWARE\WOW6432Node\VMware',
    'HKLM:\SOFTWARE\Classes\VMPerfProvider.VMStatsProvider','HKLM:\SOFTWARE\Classes\VMPerfProvider.VMStatsProvider.1',
    'HKLM:\SYSTEM\CurrentControlSet\Services\EventLog\Application\vmStatsProvider',
    'HKLM:\SYSTEM\CurrentControlSet\Services\EventLog\Application\vmtools','HKLM:\SYSTEM\CurrentControlSet\Services\EventLog\Application\VMware Tools'
)
$svcRegKeys = @(
    'HKLM:\SYSTEM\CurrentControlSet\Services\vmci','HKLM:\SYSTEM\CurrentControlSet\Services\vm3dmp','HKLM:\SYSTEM\CurrentControlSet\Services\vm3dservice',
    'HKLM:\SYSTEM\CurrentControlSet\Services\vmaudio','HKLM:\SYSTEM\CurrentControlSet\Services\vmhgfs','HKLM:\SYSTEM\CurrentControlSet\Services\VMMemCtl',
    'HKLM:\SYSTEM\CurrentControlSet\Services\vmmouse','HKLM:\SYSTEM\CurrentControlSet\Services\VMRawDisk','HKLM:\SYSTEM\CurrentControlSet\Services\vmusbmouse',
    'HKLM:\SYSTEM\CurrentControlSet\Services\vmvss','HKLM:\SYSTEM\CurrentControlSet\Services\vmvsock','HKLM:\SYSTEM\CurrentControlSet\Services\vsock',
    'HKLM:\SYSTEM\CurrentControlSet\Services\vmxnet3','HKLM:\SYSTEM\CurrentControlSet\Services\VMTools','HKLM:\SYSTEM\CurrentControlSet\Services\VGAuthService',
    'HKLM:\SYSTEM\CurrentControlSet\Services\VMwareCAF','HKLM:\SYSTEM\CurrentControlSet\Services\vmStatsProvider'
)

foreach ($k in ($regKeys + $svcRegKeys)) {
    try {
        if (Test-Path $k) {
            Remove-Item $k -Recurse -Force
            $Global:didWork = $true
            Write-Log "Removed registry key: $($k)"
        }
    } catch {
        Write-Log "Failed to remove reg key $($k): $_" 'WARNING'
    }
}

$uninstallGuid = '{A18706D7-E79F-44F4-A0C6-1DB887F47F64}'
$uninstallPaths = @(
    "HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\$uninstallGuid",
    "HKLM:\SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall\$uninstallGuid"
)
foreach ($up in $uninstallPaths) {
    if (Test-Path $up) {
        Remove-Item $up -Force
        $Global:didWork = $true
        Write-Log "Removed uninstall registry: $($up)"
    }
}
# ====================== FOLDER CLEANUP ======================
$paths = @(
    'C:\Program Files\VMware','C:\Program Files (x86)\VMware',
    'C:\Program Files\Common Files\VMware','C:\Program Files (x86)\Common Files\VMware',
    'C:\ProgramData\VMware','C:\ProgramData\VMware\VMware VGAuth'
)

foreach ($p in $paths) {
    try {
        if (Test-Path $p) {
            Get-Process | Where-Object { $_.Path -like "$p\*" } | Stop-Process -Force -ErrorAction SilentlyContinue
            takeown /F $p /R /D Y 2>&1 | Out-Null
            icacls $p /grant Administrators:F /T /Q 2>&1 | Out-Null
            # Retry removal up to 3 times
            $retryCount = 0
            do {
                Remove-Item $p -Recurse -Force -ErrorAction Continue
                Start-Sleep -Seconds 2
                if (Test-Path $p) {
                    cmd.exe /c "rd /s /q `"$p`""
                }
                $retryCount++
            } while ((Test-Path $p) -and $retryCount -lt 3)
            if ((Test-Path $p)) {
                # Rename if still locked, schedule delete on reboot
                $newPath = $p + ".delete"
                if (Test-Path $newPath) { Remove-Item $newPath -Recurse -Force -ErrorAction SilentlyContinue }
                Rename-Item -Path $p -NewName $newPath -Force -ErrorAction SilentlyContinue
                Write-Log "Renamed locked folder $p to $newPath for deletion on reboot" 'WARNING'
                # Schedule cmd to delete on reboot
                $runOnceKey = "HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\RunOnce"
                Set-ItemProperty -Path $runOnceKey -Name "!DeleteVMwareFolder" -Value "cmd.exe /c rd /s /q `"$newPath`"" -Force
                $Global:didWork = $true
            } else {
                $Global:didWork = $true
                Write-Log "Removed folder: $($p)"
            }
        }
    } catch {
        Write-Log "Failed to remove folder $($p): $_" 'DEBUG' # Changed to DEBUG to suppress in main output
    }
}

# ====================== DRIVERSTORE CLEANUP ======================
$pnputil = if (Test-Path "$env:WINDIR\Sysnative\pnputil.exe") { "$env:WINDIR\Sysnative\pnputil.exe" } else { "$env:WINDIR\System32\pnputil.exe" }

try {
    # For older OS like 2012, use -e
    $driverOutput = if ($osVersion -lt 10) {
        & $pnputil -e 2>&1 | Out-String
    } else {
        & $pnputil /enum-drivers 2>&1 | Out-String
    }
    $driverLines = $driverOutput -split "`r`n"
    $oem = $null
    $driversToDelete = @()
    foreach ($line in $driverLines) {
        if ($line -match '(?i)Published Name\s*:\s*(oem\d+\.inf)') {
            $oem = $matches[1]
        }
        elseif ($line -match '^\s*$') {
            $oem = $null
        }
        elseif ($line -match '(?i)vmware|vmxnet|vmmouse|vmhgfs|vmci|vm3d|pvscsi|vsock|efifw' -and $oem) {
            if ($driversToDelete -notcontains $oem) {
                $driversToDelete += $oem
            }
        }
    }
    foreach ($d in $driversToDelete) {
        if ($osVersion -lt 10) {
            & $pnputil -d $d 2>&1 | Out-Null
        } else {
            & $pnputil /delete-driver $d /uninstall /force 2>&1 | Out-Null
        }
        & $pnputil -f -d $d 2>&1 | Out-Null
        $Global:didWork = $true
        Write-Log "DriverStore deleted: $($d)"
    }
} catch {
    Write-Log "DriverStore cleanup failed: $_" 'ERROR'
}

# ====================== PNPLOCKDOWN CLEANUP ======================
$lockPath = 'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Setup\PnpLockdownFiles'
if (Test-Path $lockPath) {
    try {
        $props = Get-Item $lockPath
        $lockProps = $props.Property
        foreach ($name in $lockProps) {
            if ($name -match '(?i)vm|pvscsi|efifw|vm3d|vmStatsProvider') {
                Remove-ItemProperty -Path $lockPath -Name $name -Force -ErrorAction SilentlyContinue
                Write-Log "Cleared PnpLockdown: $($name)"
                $Global:didWork = $true
            }
        }
    } catch {
        Write-Log "PnpLockdown cleanup failed: $_" 'WARNING'
    }
}

# ====================== REMAINING CHECK ======================
function Test-Remaining {
    $hasRemnants = $false
    $checkProcesses = @('vmtoolsd','vm3dservice','VGAuthService','vmwaretray','vmwareuser')
    foreach ($p in $checkProcesses) {
        $found = Get-Process -Name $p -ErrorAction SilentlyContinue
        if ($found) {
            Write-Log "Remnant found: Process $($p)" 'WARNING'
            $hasRemnants = $true
        }
    }
    $checkServices = @('VMTools','vm3dservice','VGAuthService','VMwareCAF','VMwareCAFCommAmqpListener','VMwareCAFManagementAgentHost','vmci','vm3dmp','vmaudio','vmhgfs','VMMemCtl','vmmouse','VMRawDisk','vmusbmouse','vmvss','vmvsock','vsock','vmxnet3','vmStatsProvider')
    foreach ($s in $checkServices) {
        $found = Get-Service -Name $s -ErrorAction SilentlyContinue
        if ($found) {
            Write-Log "Remnant found: Service $($s)" 'WARNING'
            $hasRemnants = $true
        }
    }
    $driversPath = "C:\Windows\System32\drivers"
    $checkDrivers = @("vmci.sys", "vm3dmp.sys", "vmaudio.sys", "vmhgfs.sys", "vmmemctl.sys", "vmmouse.sys", "vmrawdsk.sys", "vmtools.sys", "vmusbmouse.sys", "vmvss.sys", "vsock.sys", "vmx_svga.sys", "vmxnet3.sys")
    foreach ($d in $checkDrivers) {
        $found = Test-Path (Join-Path $driversPath $d)
        if ($found) {
            Write-Log "Remnant found: Driver File $($d)" 'WARNING'
            $hasRemnants = $true
        }
    }
    $pnputil = if (Test-Path "$env:WINDIR\Sysnative\pnputil.exe") { "$env:WINDIR\Sysnative\pnputil.exe" } else { "$env:WINDIR\System32\pnputil.exe" }
    $pnpOutput = if ($osVersion -lt 10) {
        & $pnputil -e 2>&1 | Out-String
    } else {
        & $pnputil /enum-drivers 2>&1 | Out-String
    }
    $pnpPresent = $pnpOutput -match "VMware|vmxnet|vmmouse|pvscsi"
    if ($pnpPresent) {
        Write-Log "Remnant found: DriverStore VMware INF Packages" 'WARNING'
        $hasRemnants = $true
    }
    $checkFolders = @('C:\Program Files\VMware','C:\Program Files (x86)\VMware','C:\ProgramData\VMware','C:\Program Files\Common Files\VMware')
    foreach ($f in $checkFolders) {
        if (Test-Path $f -and -not (Test-Path "$f.delete")) {
            Write-Log "Remnant found: Folder $($f)" 'WARNING'
            $hasRemnants = $true
            Get-ChildItem $f -Recurse -File | ForEach-Object { Write-Log "Locked file in $($f): $($_.FullName)" 'DEBUG' }
        }
    }
    $checkRegKeys = @(
        'HKLM:\SOFTWARE\VMware, Inc.',
        'HKLM:\SOFTWARE\VMware',
        'HKLM:\SOFTWARE\WOW6432Node\VMware, Inc.',
        'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\{A18706D7-E79F-44F4-A0C6-1DB887F47F64}',
        'HKLM:\SYSTEM\CurrentControlSet\Services\vmci'
    )
    foreach ($k in $checkRegKeys) {
        if (Test-Path $k) {
            Write-Log "Remnant found: Registry $($k)" 'WARNING'
            $hasRemnants = $true
        }
    }
    return $hasRemnants
}
if (-not (Test-Remaining)) {
    Write-Log '=== NO REMNANTS DETECTED === VMware Tools fully removed'
    New-Item -ItemType File -Path $MarkerPath -Force | Out-Null
    Write-Log 'Cleanup finished successfully.'
    exit 0
}
Write-Log 'Remnants still present - rebooting for next pass' 'WARNING'
Restart-Computer -Force
exit 3010