<#
.SYNOPSIS
    VMware Tools removal script for use with Firstboot-Scheduler.ps1.
    Designed to run across multiple reboots via the scheduler.
    Exit codes:
      0    = VMware Tools fully removed (done)
      3010 = Reboot required to continue removal
      1    = Fatal error / max attempts exceeded
#>

$WorkDir = "$env:ProgramData\VMwareRemoval"
$MarkerPath = "$WorkDir\vmware_tools_removed.marker"
$LogPath = "$WorkDir\VMware_Removal_Log.txt"
$MaxAttempts = 15

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

# Marker check - already done from a previous migration
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
    Write-Log "Max attempts ($MaxAttempts) reached. Giving up." 'ERROR'
    New-Item -ItemType File -Path "$WorkDir\vmware_tools_removed.failed" -Force | Out-Null
    exit 1
}

Write-Log "=== VMware Tools Removal - Run #$attempt of $MaxAttempts ==="

# ====================== SERVICES ======================
$services = @(
    'VMTools','vm3dservice','VGAuthService',
    'VMwareCAF','VMwareCAFCommAmqpListener','VMwareCAFManagementAgentHost',
    'vmci','vm3dmp','vmaudio','vmhgfs','VMMemCtl','vmmouse',
    'VMRawDisk','vmusbmouse','vmvss','vmvsok','vsock','vmxnet3'
)

foreach ($svc in $services) {
    $s = Get-Service -Name $svc -ErrorAction SilentlyContinue
    if ($s) {
        Stop-Service -Name $svc -Force -ErrorAction SilentlyContinue
        sc.exe delete $svc 2>&1 | Out-Null
        $Global:didWork = $true
        Write-Log "Deleted service: $svc"
    }
}

# ====================== PROCESSES ======================
$processes = @('vmtoolsd','vm3dservice','VGAuthService','vmwaretray','vmwareuser')

foreach ($proc in $processes) {
    Get-Process -Name $proc -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
}

# ====================== MSI UNINSTALL ======================
function Try-UninstallVMwareTools {
    $uninstallPaths = @(
        'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\*',
        'HKLM:\SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall\*'
    )
    foreach ($path in $uninstallPaths) {
        $entries = Get-ItemProperty -Path $path -ErrorAction SilentlyContinue |
            Where-Object { $_.DisplayName -like '*VMware Tools*' }
        foreach ($entry in $entries) {
            $uninstallString = $entry.UninstallString
            if ($uninstallString -match 'MsiExec') {
                $productCode = $entry.PSChildName
                Write-Log "Attempting MSI uninstall for product: $productCode"
                $proc = Start-Process -FilePath 'msiexec.exe' -ArgumentList "/x $productCode /qn /norestart REBOOT=ReallySuppress" -Wait -PassThru -ErrorAction SilentlyContinue
                if ($proc) {
                    Write-Log "MSI uninstall exit code: $($proc.ExitCode)"
                    $Global:didWork = $true
                    return $proc.ExitCode
                }
            }
        }
    }
    return $null
}

$uninstallExit = Try-UninstallVMwareTools
Write-Log "MSI uninstall result: $uninstallExit"

# ====================== REGISTRY CLEANUP ======================
$regKeys = @(
    'HKLM:\SOFTWARE\VMware, Inc.',
    'HKLM:\SOFTWARE\VMware',
    'HKLM:\SOFTWARE\WOW6432Node\VMware, Inc.',
    'HKLM:\SOFTWARE\WOW6432Node\VMware'
)

$svcRegKeys = @(
    'HKLM:\SYSTEM\CurrentControlSet\Services\vmci',
    'HKLM:\SYSTEM\CurrentControlSet\Services\vm3dmp',
    'HKLM:\SYSTEM\CurrentControlSet\Services\vm3dservice',
    'HKLM:\SYSTEM\CurrentControlSet\Services\vmaudio',
    'HKLM:\SYSTEM\CurrentControlSet\Services\vmhgfs',
    'HKLM:\SYSTEM\CurrentControlSet\Services\VMMemCtl',
    'HKLM:\SYSTEM\CurrentControlSet\Services\vmmouse',
    'HKLM:\SYSTEM\CurrentControlSet\Services\VMRawDisk',
    'HKLM:\SYSTEM\CurrentControlSet\Services\vmusbmouse',
    'HKLM:\SYSTEM\CurrentControlSet\Services\vmvss',
    'HKLM:\SYSTEM\CurrentControlSet\Services\vsock',
    'HKLM:\SYSTEM\CurrentControlSet\Services\vmxnet3',
    'HKLM:\SYSTEM\CurrentControlSet\Services\VMTools',
    'HKLM:\SYSTEM\CurrentControlSet\Services\VGAuthService',
    'HKLM:\SYSTEM\CurrentControlSet\Services\VMwareCAF'
)

foreach ($k in ($regKeys + $svcRegKeys)) {
    if (Test-Path $k) {
        Remove-Item $k -Recurse -Force -ErrorAction SilentlyContinue
        $Global:didWork = $true
        Write-Log "Removed registry key: $k"
    }
}

# ====================== FOLDER CLEANUP ======================
$paths = @(
    'C:\Program Files\VMware',
    'C:\Program Files (x86)\VMware',
    'C:\Program Files\Common Files\VMware',
    'C:\Program Files (x86)\Common Files\VMware',
    'C:\ProgramData\VMware'
)

foreach ($p in $paths) {
    if (Test-Path $p) {
        takeown /F $p /R /D Y 2>&1 | Out-Null
        icacls $p /grant Administrators:F /T /Q 2>&1 | Out-Null
        Remove-Item $p -Recurse -Force -ErrorAction SilentlyContinue
        $Global:didWork = $true
        Write-Log "Removed folder: $p"
    }
}

# ====================== DRIVERSTORE CLEANUP ======================
$pnputil = if (Test-Path "$env:WINDIR\Sysnative\pnputil.exe") {
    "$env:WINDIR\Sysnative\pnputil.exe"
} else {
    "$env:WINDIR\System32\pnputil.exe"
}

$driverOutput = & $pnputil /enum-drivers 2>&1 | Out-String
$driverOutput | Select-String -Pattern '(?i)vmware|vmxnet|vmmouse|vmhgfs|vmci|vm3d|pvscsi|vsock|efifw' -Context 0,10 |
  ForEach-Object {
      if ($_ -match '(?i)Published Name\s*:\s*(oem\d+\.inf)') {
          & $pnputil /delete-driver $matches[1] /uninstall /force 2>&1 | Out-Null
          $Global:didWork = $true
          Write-Log "DriverStore deleted: $($matches[1])"
      }
  }

# ====================== PNPLOCKDOWN CLEANUP ======================
$lockPath = 'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Setup\PnpLockdownFiles'
if (Test-Path $lockPath) {
    $props = Get-Item $lockPath -ErrorAction SilentlyContinue
    if ($props) {
        foreach ($name in $props.Property) {
            if ($name -match '(?i)vm|pvscsi|efifw') {
                Remove-ItemProperty -Path $lockPath -Name $name -Force -ErrorAction SilentlyContinue
                Write-Log "Cleared PnpLockdown: $name"
            }
        }
    }
}

# ====================== REMAINING CHECK ======================
function Test-Remaining {
    # Folder check
    if (Test-Path 'C:\Program Files\Common Files\VMware') { return $true }
    if (Test-Path 'C:\Program Files\VMware') { return $true }
    if (Test-Path 'C:\ProgramData\VMware') { return $true }

    # Service check
    if (Get-Service -Name 'vm3dservice','VMTools','VGAuthService' -ErrorAction SilentlyContinue) { return $true }

    # Process check
    if (Get-Process -Name 'vmtoolsd','vm3dservice' -ErrorAction SilentlyContinue) { return $true }

    # MSI check
    $apps = Get-ItemProperty -Path 'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\*','HKLM:\SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall\*' -ErrorAction SilentlyContinue
    if ($apps | Where-Object { $_.DisplayName -like '*VMware Tools*' }) { return $true }

    # DriverStore still has VMware?
    $out = & $pnputil /enum-drivers 2>&1 | Out-String
    if ($out -match '(?i)VMware') { return $true }

    return $false
}

if (-not (Test-Remaining)) {
    Write-Log '=== NO REMNANTS DETECTED === VMware Tools fully removed'
    New-Item -ItemType File -Path $MarkerPath -Force | Out-Null
    Write-Log 'Cleanup finished successfully.'
    exit 0
}

# Remnants still present - rebooting the machine
Write-Log 'Remnants still present - rebooting for next pass' 'WARNING'
Restart-Computer -Force
Start-Sleep -Seconds 30
exit 0