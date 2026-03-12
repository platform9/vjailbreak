<#
.SYNOPSIS
    VMware Tools removal script.
    Designed for firstboot / scheduler-controlled environments.
#>

param(
    [string]$LogPath = "C:\VMware_Removal_Log.txt"
)

$ErrorActionPreference = 'Continue'
$WarningPreference = 'Continue'

function Write-Log {
    param(
        [string]$Message,
        [string]$Level = "INFO"
    )

    $timestamp = Get-Date -Format "yyyy-MM-dd HH:mm:ss"
    "$timestamp [$Level] $Message" | Out-File -FilePath $LogPath -Append -Encoding UTF8
    Write-Host "$timestamp [$Level] $Message"
}

if (-not ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    Write-Log "ERROR: Script must be run as Administrator!" "ERROR"
    exit 1
}

Write-Log "=== Starting VMware Tools COMPLETE Removal ==="

function Schedule-DeleteOnReboot {
    param([string]$Path)
    $regPath = "HKLM:\SYSTEM\CurrentControlSet\Control\Session Manager"
    $current = (Get-ItemProperty -Path $regPath -Name PendingFileRenameOperations -ErrorAction SilentlyContinue).PendingFileRenameOperations
    if (-not $current) { $current = @() }
    $newEntry = @($Path, "")
    Set-ItemProperty -Path $regPath -Name PendingFileRenameOperations -Value ($current + $newEntry)
    Write-Log "Scheduled for deletion on reboot: $Path" "WARNING"
}

function Stop-VMwareProcesses {
    Write-Log "Killing VMware processes..."
    $procs = @('vmtoolsd','vm3dservice','VGAuthService','vmwaretray','vmwareuser','vmware-svga','vmware-vdisk','vmxnet3','vmmouse')
    Get-Process -Name $procs -ErrorAction SilentlyContinue | Stop-Process -Force
}

function Remove-VMwareServices {
    Write-Log "Removing ALL VMware-related services..."
    $temp = "$env:TEMP\services.txt"
    sc.exe query type= service state= all > $temp

    $services = @()
    foreach ($line in (Get-Content $temp)) {
        if ($line -match '^SERVICE_NAME:\s*(.+)$') {
            $svc = $matches[1].Trim()
            $qc = sc.exe qc $svc 2>$null
            if ($qc -match 'VMware') { $services += $svc }
        }
    }
    Remove-Item $temp -Force -ErrorAction SilentlyContinue

    foreach ($svc in $services) {
        Write-Log "Processing service: $svc"
        Stop-Service $svc -Force -ErrorAction SilentlyContinue
        sc.exe config $svc start= disabled | Out-Null
        sc.exe delete $svc | Out-Null
        Write-Log "Service $svc removed"
    }
}

function Remove-VMwareDrivers {
    Write-Log "Removing VMware .sys drivers..."
    $drivers = @(
        "vmci.sys","vm3dmp.sys","vmaudio.sys","vmhgfs.sys","vmmemctl.sys",
        "vmmouse.sys","vmrawdsk.sys","vmtools.sys","vmusbmouse.sys","vmvss.sys",
        "vsock.sys","vmx_svga.sys","vmxnet3.sys","vmxnet3.sys","vmmemctl.sys"
    )
    $path = "C:\Windows\System32\drivers"
    foreach ($d in $drivers) {
        $full = Join-Path $path $d
        if (Test-Path $full) {
            try {
                takeown /F $full | Out-Null
                icacls $full /grant Administrators:F | Out-Null
                Remove-Item $full -Force
                Write-Log "Deleted $d"
            } catch {
                Schedule-DeleteOnReboot $full
                Write-Log "Scheduled $d for reboot deletion"
            }
        }
    }
}

function Remove-VMwareDevices {
    Write-Log "Removing VMware PnP devices (VMCI, Pointing Device, etc.)..."
    $vmDevices = Get-PnpDevice | Where-Object { $_.FriendlyName -like "*VMware*" -or $_.InstanceId -like "*VMware*" }
    foreach ($dev in $vmDevices) {
        try {
            Disable-PnpDevice -InstanceId $dev.InstanceId -Confirm:$false -ErrorAction SilentlyContinue
            Remove-PnpDevice -InstanceId $dev.InstanceId -Confirm:$false -Remove -ErrorAction SilentlyContinue
            Write-Log "Removed device: $($dev.FriendlyName)"
        } catch {
            Write-Log "Could not remove device $($dev.FriendlyName) – will be cleaned on reboot" "WARNING"
        }
    }
}

function Remove-VMwareFolders {
    Write-Log "Removing VMware folders + Start Menu..."
    $folders = @(
        'C:\Program Files\VMware',
        'C:\Program Files (x86)\VMware',
        'C:\Program Files\Common Files\VMware',
        'C:\Program Files (x86)\Common Files\VMware',
        'C:\ProgramData\VMware',
        'C:\ProgramData\Microsoft\Windows\Start Menu\Programs\VMware'
    )

    foreach ($p in $folders) {
        if (Test-Path $p) {
            try {
                takeown /F $p /R /D Y | Out-Null
                icacls $p /grant Administrators:F /T | Out-Null
                Remove-Item $p -Recurse -Force
                Write-Log "Removed folder: $p"
            } catch {
                Schedule-DeleteOnReboot $p
            }
        }
    }

    # User profiles
    Get-ChildItem "C:\Users" -Directory | ForEach-Object {
        @("$($_.FullName)\AppData\Local\VMware", "$($_.FullName)\AppData\Roaming\VMware") | ForEach-Object {
            if (Test-Path $_) {
                try { Remove-Item $_ -Recurse -Force } catch { Schedule-DeleteOnReboot $_ }
            }
        }
    }
}

function Remove-VMwareRegistry {
    Write-Log "Applying ultra-granular registry cleanup..."
    $regList = @(
        "HKLM:\SOFTWARE\Clients\StartmenuInternet\VMWAREHOSTOPEN.EXE",
        "HKLM:\SOFTWARE\Classes\Applications\VMwareHostOpen.exe",
        "HKLM:\SOFTWARE\Classes\VMwareHostOpen.AssocFile",
        "HKLM:\SOFTWARE\Classes\VMwareHostOpen.AssocURL",
        "HKLM:\SOFTWARE\VMware, Inc.\VMwareHostOpen",
        "HKLM:\SYSTEM\CurrentControlSet\Services\vmci",
        "HKLM:\SYSTEM\CurrentControlSet\Services\vmmouse",
        "HKLM:\SYSTEM\CurrentControlSet\Services\vmusbmouse",
        "HKLM:\SYSTEM\CurrentControlSet\Services\vmhgfs",
        "HKLM:\SYSTEM\CurrentControlSet\Services\vmrawdsk",
        "HKLM:\SYSTEM\CurrentControlSet\Services\Eventlog\Application\vmtools",
        "HKLM:\SYSTEM\CurrentControlSet\Services\Eventlog\Application\VMware Tools",
        "HKLM:\SYSTEM\CurrentControlSet\Services\Eventlog\Application\VGAuth",
        "HKLM:\SOFTWARE\VMware, Inc.\VMware Drivers",
        "HKLM:\SOFTWARE\VMware, Inc.\VMware Tools",
        "HKLM:\SOFTWARE\VMware, Inc.\VMware VGAuth",
        "HKLM:\SOFTWARE\VMware, Inc.",
        "HKLM:\SOFTWARE\VMware, Inc.\CbLauncher",
        "HKLM:\SYSTEM\CurrentControlSet\Services\W32Time\TimeProviders\vmwTimeProvider",
        "HKLM:\SOFTWARE\Classes\CLSID\{C73DA087-EDDB-4a7c-B216-8EF8A3B92C7B}"
    )

    foreach ($key in $regList) {
        if (Test-Path $key) {
            Remove-Item $key -Recurse -Force -ErrorAction SilentlyContinue
            Write-Log "Deleted registry key: $key"
        }
    }
}

function Remove-VMwareDriverStore {
    Write-Log "Cleaning DriverStore (oem*.inf VMware packages)..."
    $output = pnputil /enum-drivers
    $packages = @()
    $current = @{}

    foreach ($line in $output) {
        if ($line -match "Published Name\s*:\s*(oem\d+\.inf)") { $current.PublishedName = $matches[1] }
        elseif ($line -match "Provider Name\s*:\s*(.+)") { $current.Provider = $matches[1].Trim() }
        elseif ($line -match "Class Name\s*:\s*(.+)") {
            if ($current.Provider -match "VMware") {
                $packages += $current.PublishedName
            }
            $current = @{}
        }
    }

    foreach ($inf in $packages) {
        pnputil /delete-driver $inf /uninstall /force | Out-Null
        Write-Log "Removed DriverStore package: $inf"
    }
}

function Remove-VMwareMSI {
    Write-Log "Attempting MSI uninstall of VMware Tools ..."
    try {
        $uninstallKeys = Get-ChildItem "HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall", "HKLM:\SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall" -ErrorAction SilentlyContinue
        foreach ($key in $uninstallKeys) {
            $name = (Get-ItemProperty -Path $key.PSPath -Name DisplayName -ErrorAction SilentlyContinue).DisplayName
            if ($name -like "*VMware Tools*") {
                $uninstall = (Get-ItemProperty -Path $key.PSPath -Name UninstallString -ErrorAction SilentlyContinue).UninstallString
                if ($uninstall) {
                    $uninstall = $uninstall -replace "/I", "/X"
                    $proc = Start-Process msiexec.exe -ArgumentList "$uninstall /quiet /norestart" -PassThru
                    $proc.WaitForExit(30000) | Out-Null
                    Write-Log "MSI uninstall attempted for $name (timed out if needed)"
                }
            }
        }
    } catch {
        Write-Log "MSI uninstall had an issue - continuing anyway" "WARNING"
    }
}

Stop-VMwareProcesses
Remove-VMwareMSI
Remove-VMwareServices
Remove-VMwareDevices
Remove-VMwareDrivers
Remove-VMwareFolders
Remove-VMwareRegistry
Remove-VMwareDriverStore

Write-Log "VMware Tools removal completed"
Write-Log "Rebooting once to finalize deletion..."
Restart-Computer -Force

exit 0