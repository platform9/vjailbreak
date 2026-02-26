<#
.SYNOPSIS
    VMware Tools removal script.
    Designed for firstboot / scheduler-controlled environments.
#>

param(
    [string]$LogPath = "C:\VMware_Removal_Log.txt"
)

$ErrorActionPreference = 'SilentlyContinue'

function Write-Log {
    param(
        [string]$Message,
        [string]$Level = "INFO"
    )

    $timestamp = Get-Date -Format "yyyy-MM-dd HH:mm:ss"
    "$timestamp [$Level] $Message" | Out-File -FilePath $LogPath -Append -Encoding UTF8
}

function Schedule-DeleteOnReboot {
    param([string]$Path)

    $regPath = "HKLM:\SYSTEM\CurrentControlSet\Control\Session Manager"
    $current = (Get-ItemProperty -Path $regPath -Name PendingFileRenameOperations -ErrorAction SilentlyContinue).PendingFileRenameOperations

    if (-not $current) { $current = @() }

    $newEntry = @($Path, "")
    $updated = $current + $newEntry

    Set-ItemProperty -Path $regPath -Name PendingFileRenameOperations -Value $updated
    Write-Log "Scheduled for deletion on reboot: $Path" "WARNING"
}

function Stop-VMwareProcesses {

    Write-Log "Stopping VMware processes..."

    $processes = @(
        'vmtoolsd','vm3dservice','VGAuthService',
        'vmwaretray','vmwareuser','vmware-svga'
    )

    foreach ($proc in $processes) {
        Get-Process -Name $proc -ErrorAction SilentlyContinue | Stop-Process -Force
    }
}

function Remove-VMwareServices {

    Write-Log "Stopping and removing VMware services..."

    $services = @(
        'VMTools','vm3dservice','VGAuthService',
        'VMwareCAF','VMwareCAFCommAmqpListener','VMwareCAFManagementAgentHost',
        'vmci','vm3dmp','vmaudio','vmhgfs','VMMemCtl',
        'vmmouse','VMRawDisk','vmusbmouse','vmvss',
        'vsock','vmxnet3'
    )

    foreach ($svc in $services) {
        Stop-Service $svc -Force -ErrorAction SilentlyContinue
        sc.exe delete $svc 2>&1 | Out-Null
    }
}

function Remove-VMwareDrivers {

    Write-Log "Removing VMware drivers..."

    $drivers = @(
        "vmci.sys","vm3dmp.sys","vmaudio.sys","vmhgfs.sys",
        "vmmemctl.sys","vmmouse.sys","vmrawdsk.sys",
        "vmtools.sys","vmusbmouse.sys","vmvss.sys",
        "vsock.sys","vmx_svga.sys","vmxnet3.sys"
    )

    $driverPath = "C:\Windows\System32\drivers"

    foreach ($d in $drivers) {

        $full = Join-Path $driverPath $d

        if (Test-Path $full) {
            try {
                takeown /F $full | Out-Null
                icacls $full /grant Administrators:F | Out-Null
                Remove-Item $full -Force
                Write-Log "Deleted driver: $d"
            }
            catch {
                Schedule-DeleteOnReboot -Path $full
            }
        }
    }
}


function Remove-VMwareFolders {

    Write-Log "Removing VMware folders..."

    $staticPaths = @(
        'C:\Program Files\VMware',
        'C:\Program Files (x86)\VMware',
        'C:\Program Files\Common Files\VMware',
        'C:\Program Files (x86)\Common Files\VMware',
        'C:\ProgramData\VMware'
    )

    foreach ($p in $staticPaths) {
        if (Test-Path $p) {
            try {
                takeown /F $p /R /D Y | Out-Null
                icacls $p /grant Administrators:F /T | Out-Null
                Remove-Item $p -Recurse -Force
                Write-Log "Removed folder: $p"
            }
            catch {
                Schedule-DeleteOnReboot -Path $p
            }
        }
    }

    Get-ChildItem "C:\Users" -Directory -ErrorAction SilentlyContinue | ForEach-Object {

        $localPath   = "$($_.FullName)\AppData\Local\VMware"
        $roamingPath = "$($_.FullName)\AppData\Roaming\VMware"

        foreach ($userPath in @($localPath, $roamingPath)) {

            if (Test-Path $userPath) {
                try {
                    takeown /F $userPath /R /D Y | Out-Null
                    icacls $userPath /grant Administrators:F /T | Out-Null
                    Remove-Item $userPath -Recurse -Force
                    Write-Log "Removed user folder: $userPath"
                }
                catch {
                    Schedule-DeleteOnReboot -Path $userPath
                }
            }
        }
    }
}

function Remove-VMwareRegistry {

    Write-Log "Cleaning VMware registry keys..."

    $keys = @(
        'HKLM:\SOFTWARE\VMwareInc.',
        'HKLM:\SOFTWARE\VMware',
        'HKLM:\SOFTWARE\WOW6432Node\VMwareInc',
        'HKLM:\SOFTWARE\WOW6432Node\VMware'
    )

    foreach ($k in $keys) {
        Remove-Item $k -Recurse -Force -ErrorAction SilentlyContinue
    }

    $serviceRoot = 'HKLM:\SYSTEM\CurrentControlSet\Services'
    $serviceKeys = @(
        'vmci','vm3dmp','vmaudio','vmhgfs','VMMemCtl',
        'vmmouse','VMRawDisk','VMTools','vmusbmouse',
        'vmvss','VMwareCAF','VMwareCAFCommAmqpListener',
        'VMwareCAFManagementAgentHost'
    )

    foreach ($s in $serviceKeys) {
        Remove-Item "$serviceRoot\$s" -Recurse -Force -ErrorAction SilentlyContinue
    }
}

function Remove-VMwareDriverStore {

    Write-Log "Cleaning DriverStore entries..."

    $osVersion = (Get-CimInstance Win32_OperatingSystem).Version

    if ($osVersion -like "6.2*") {
        $drivers = pnputil -e | Select-String -Pattern "vmware"
        foreach ($line in $drivers) {
            if ($line -match "Published Name\s*:\s*(oem\d+\.inf)") {
                pnputil -d $matches[1] | Out-Null
            }
        }
    }
    else {
        $drivers = pnputil /enum-drivers | Select-String -Pattern "vmware" -Context 0,3
        foreach ($block in $drivers) {
            if ($block.Context.PostContext -match "Published Name\s*:\s*(oem\d+\.inf)") {
                pnputil /delete-driver $matches[1] /uninstall /force | Out-Null
            }
        }
    }
}

Write-Log "=== Starting VMware Tools One-Pass Removal ==="

Stop-VMwareProcesses
Remove-VMwareServices
Remove-VMwareDrivers
Remove-VMwareFolders
Remove-VMwareRegistry
Remove-VMwareDriverStore

Write-Log "Cleanup attempt completed."
Write-Log "Rebooting once to finalize deletion..."

Restart-Computer -Force

exit 0