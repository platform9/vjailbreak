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
        $proc = Start-Process -FilePath 'msiexec.exe' -ArgumentList $args -Wait -PassThru
        Write-Log "MSI uninstall exit code: $($proc.ExitCode)"
        $Global:didWork = $true
        return $proc.ExitCode
    }
    return $null
}

# MSI Hack if uninstall fails
function Try-MsiHack {
    $installer = New-Object -ComObject WindowsInstaller.Installer
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
                    $view = $db.OpenView("SELECT Action FROM CustomAction")
                    $view.Execute()
                    $record = $view.Fetch()
                    $actionsToDelete = @()
                    while ($record -ne $null) {
                        $actionName = $record.StringData(1)
                        if ($actionName -match '^VM_') {
                            $actionsToDelete += $actionName
                        }
                        $record = $view.Fetch()
                    }
                    $view.Close()

                    foreach ($action in $actionsToDelete) {
                        $delView = $db.OpenView("DELETE FROM CustomAction WHERE Action = '$action'")
                        $delView.Execute()
                        $delView.Close()
                    }
                    $db.Commit()
                    $Global:didWork = $true
                    
                    # Retry uninstall after hack
                    Write-Log "Retrying msiexec after removing custom actions"
                    $proc = Start-Process 'msiexec.exe' -ArgumentList "/x `"$localPackage`" /qn /norestart REBOOT=ReallySuppress" -Wait -PassThru
                    Write-Log "MSI hack uninstall exit code: $($proc.ExitCode)"
                } catch {
                    Write-Log "MSI hack failed: $_" 'ERROR'
                }
                break
            }
        }
    }
}

# Run MSI Uninstall FIRST before destroying services/processes it might depend on
$uninstallExit = Try-UninstallVMwareTools
if ($uninstallExit -and $uninstallExit -ne 0 -and $uninstallExit -ne 3010) {
    Write-Log "Standard uninstall failed ($uninstallExit). Trying MSI hack."
    Try-MsiHack
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
        Write-Log "Deleted service: $svc"
    } catch {
        Write-Log "Failed to delete service ${svc}: $_" 'WARNING'
    }
}

# ====================== PROCESSES ======================
$processes = @('vmtoolsd','vm3dservice','VGAuthService','vmwaretray','vmwareuser')

foreach ($proc in $processes) {
    try {
        Get-Process -Name $proc | Stop-Process -Force
        $Global:didWork = $true
    } catch {}
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
    'HKLM:\SYSTEM\CurrentControlSet\Services\VMwareCAF','HKLM:\SYSTEM\CurrentControlSet\Services\vmStatsProvider'  # Added
)

foreach ($k in ($regKeys + $svcRegKeys)) {
    try {
        if (Test-Path $k) {
            Remove-Item $k -Recurse -Force
            $Global:didWork = $true
            Write-Log "Removed registry key: $k"
        }
    } catch {
        Write-Log "Failed to remove reg key $($k): $_" 'WARNING'
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
        
            Remove-Item $p -Recurse -Force -ErrorAction SilentlyContinue
            if (Test-Path $p) {
                cmd.exe /c "rd /s /q `"$p`""
            }
        
            if (-not (Test-Path $p)) {
                $Global:didWork = $true
                Write-Log "Removed folder: $p"
            } else {
                Write-Log "Failed to fully remove folder: $p" 'WARNING'
            }
        }
    } catch {
        Write-Log "Failed to remove folder ${p}: $_" 'WARNING'
    }
}

# ====================== DRIVERSTORE CLEANUP ======================
$pnputil = if (Test-Path "$env:WINDIR\Sysnative\pnputil.exe") { "$env:WINDIR\Sysnative\pnputil.exe" } else { "$env:WINDIR\System32\pnputil.exe" }

try {
    $driverOutput = & $pnputil -e 2>&1 | Out-String
    if ($driverOutput -match "Invalid command" -or $driverOutput -match "not recognized") {
        $driverOutput = & $pnputil /enum-drivers 2>&1 | Out-String
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
        & $pnputil /delete-driver $d /uninstall /force 2>&1 | Out-Null
        & $pnputil -f -d $d 2>&1 | Out-Null
        $Global:didWork = $true
        Write-Log "DriverStore deleted: $d"
    }
} catch {
    Write-Log "DriverStore cleanup failed: $_" 'ERROR'
}

# ====================== PNPLOCKDOWN CLEANUP ======================
$lockPath = 'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Setup\PnpLockdownFiles'
if (Test-Path $lockPath) {
    try {
        $props = Get-Item $lockPath
        foreach ($name in $props.Property) {
            if ($name -match '(?i)vm|pvscsi|efifw|vm3d') {
                Remove-ItemProperty -Path $lockPath -Name $name -Force
                Write-Log "Cleared PnpLockdown: $name"
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

    # Folders
    $testPaths = @('C:\Program Files\VMware', 'C:\Program Files\Common Files\VMware', 'C:\ProgramData\VMware', 'C:\ProgramData\VMware\VMware VGAuth')
    foreach ($p in $testPaths) { 
        if (Test-Path $p) { 
            Write-Log "Remnant found: Folder $p" 'WARNING'
            $hasRemnants = $true
        } 
    }

    # Services
    $coreServices = @('VMTools', 'vm3dservice', 'VGAuthService')
    foreach ($svc in $coreServices) {
        if (Get-Service $svc -ErrorAction SilentlyContinue) { 
            Write-Log "Remnant found: Service $svc" 'WARNING'
            $hasRemnants = $true 
        }
    }

    # Processes
    $coreProcs = @('vmtoolsd', 'vm3dservice', 'VGAuthService', 'vmwaretray', 'vmwareuser')
    foreach ($proc in $coreProcs) {
        if (Get-Process $proc -ErrorAction SilentlyContinue) { 
            Write-Log "Remnant found: Process $proc" 'WARNING'
            $hasRemnants = $true 
        }
    }

    # Uninstall entry
    $apps = Get-ItemProperty -Path 'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\*','HKLM:\SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall\*' -ErrorAction SilentlyContinue
    $vmwApps = $apps | Where-Object { $_.DisplayName -like '*VMware Tools*' }
    if ($vmwApps) { 
        Write-Log "Remnant found: Uninstall registry key" 'WARNING'
        $hasRemnants = $true 
    }

    # Drivers
    try {
        $outStr = & $pnputil -e 2>&1 | Out-String
        if ($outStr -match "Invalid command" -or $outStr -match "not recognized") {
            $outStr = & $pnputil /enum-drivers 2>&1 | Out-String
        }
        $outLines = $outStr -split "`r`n"
        $oem = $null
        foreach ($line in $outLines) {
            if ($line -match '(?i)Published Name\s*:\s*(oem\d+\.inf)') { $oem = $matches[1] }
            elseif ($line -match '^\s*$') { $oem = $null }
            elseif ($line -match '(?i)vmware|vmxnet|vmmouse|vmhgfs|vmci|vm3d|pvscsi|vsock|efifw' -and $oem) { 
                Write-Log "Remnant found: Driver $oem" 'WARNING'
                $hasRemnants = $true
                break
            }
        }
    } catch {}

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
Start-Sleep -Seconds 30 
exit 3010