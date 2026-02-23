<#
.SYNOPSIS
    Script to completely remove VMware Tools from Windows VMs after migration.
    Handles multi-stage removal with reboots if necessary.
    Designed to run as a post-migration script.
#>

param(
    [string]$WorkDir = "$env:ProgramData\VMwareRemoval",
    [string]$MarkerPath = "$WorkDir\vmware_tools_removed.marker",
    [string]$LogPath = "C:\VMware_Removal_Log.txt",
    [string]$BootLogPath = "$WorkDir\vmware-removal-bootstrap.log",
    [string]$TaskName = "VMwareToolsRemoval",
    [int]$MaxAttempts = 10
)

$ErrorActionPreference = 'SilentlyContinue'

# Function to write logs
function Write-Log {
    param(
        [string]$Message,
        [string]$Level = 'INFO',
        [string]$LogFile = $LogPath
    )
    $ts = Get-Date -Format 'yyyy-MM-dd HH:mm:ss'
    $line = "[$ts] [$Level] $Message"
    try { Add-Content -Path $LogFile -Value $line } catch { }
    try { Add-Content -Path (Join-Path $WorkDir 'VMware_Removal_Log.txt') -Value $line } catch { }
}

# Create working directory if it doesn't exist
if (-not (Test-Path $WorkDir)) { New-Item -ItemType Directory -Path $WorkDir -Force | Out-Null }

# Check if marker exists (removal complete)
if (Test-Path $MarkerPath) {
    Write-Log 'Marker found. Deleting task and exiting.' -LogFile $BootLogPath
    schtasks /Delete /TN $TaskName /F > $null 2>&1
    exit 0
}

# Bootstrap logic: If not running from WorkDir, stage the script and schedule task
$scriptPath = $PSCommandPath
$stagedScript = Join-Path $WorkDir 'vmware-tools-removal.ps1'

if ($scriptPath -ne $stagedScript) {
    Write-Log 'First run detected. Staging script...' -LogFile $BootLogPath
    Copy-Item -Path $scriptPath -Destination $stagedScript -Force

    Write-Log "Creating Scheduled Task $TaskName..." -LogFile $BootLogPath
    $taskExists = schtasks /Query /TN $TaskName > $null 2>&1
    if (-not $taskExists) {
        $taskAction = New-ScheduledTaskAction -Execute 'powershell.exe' -Argument "-NoProfile -ExecutionPolicy Bypass -File `"$stagedScript`" -WorkDir `"$WorkDir`" -MarkerPath `"$MarkerPath`" -LogPath `"$LogPath`" -TaskName `"$TaskName`""
        $taskTrigger = New-ScheduledTaskTrigger -AtStartup
        $taskSettings = New-ScheduledTaskSettingsSet -ExecutionTimeLimit (New-TimeSpan -Hours 1) -Hidden
        $taskPrincipal = New-ScheduledTaskPrincipal -GroupId 'BUILTIN\Administrators' -RunLevel Highest
        Register-ScheduledTask -TaskName $TaskName -Action $taskAction -Trigger $taskTrigger -Settings $taskSettings -Principal $taskPrincipal -Force | Out-Null
    }

    Write-Log 'Script scheduled. Triggering reboot to release driver locks.' -LogFile $BootLogPath
    Restart-Computer -Force -Delay 5 -Reason 'VMware Removal Initial Phase'
    exit 0
}

# Post-reboot execution starts here
Write-Log '--- Post-Reboot Execution Start ---' -LogFile $BootLogPath

# Attempt counter to prevent reboot loops
$attemptFile = Join-Path $WorkDir 'attempts.txt'
$attempt = 0
if (Test-Path $attemptFile) {
    $attempt = [int](Get-Content -Path $attemptFile -ErrorAction SilentlyContinue)
}
$attempt++
Set-Content -Path $attemptFile -Value $attempt

if ($attempt -gt $MaxAttempts) {
    Write-Log "Max attempts reached ($attempt/$MaxAttempts). Stopping to avoid reboot loop." 'ERROR'
    New-Item -ItemType File -Path (Join-Path $WorkDir 'vmware_tools_removed.failed') -Force | Out-Null
    schtasks /Delete /TN $TaskName /F | Out-Null
    exit 1
}

$didWork = $false

# Function to stop and delete service if exists
function Stop-IfExistsService {
    param([string]$Name)
    $svc = Get-Service -Name $Name -ErrorAction SilentlyContinue
    if ($svc) {
        if ($svc.Status -ne 'Stopped') { Stop-Service -Name $Name -Force -ErrorAction SilentlyContinue }
        sc.exe delete $Name | Out-Null
        $script:didWork = $true
    }
}

# Function to uninstall VMware Tools
function Try-UninstallVMwareTools {
    $exitCode = $null
    $apps = Get-ItemProperty -Path 'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\*', 'HKLM:\SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall\*' -ErrorAction SilentlyContinue
    $vmw = $apps | Where-Object { $_.DisplayName -like '*VMware Tools*' } | Select-Object -First 1
    if ($vmw) {
        $u = $vmw.QuietUninstallString
        if (-not $u) { $u = $vmw.UninstallString }
        if ($u) {
            if ($u -match '\{[0-9A-Fa-f-]{36}\}') {
                $guid = $matches[0]
                Write-Log "Uninstalling VMware Tools via msiexec product code: $guid"
                $p = Start-Process -FilePath 'msiexec.exe' -ArgumentList "/x $guid /qn /norestart" -Wait -PassThru
                $exitCode = $p.ExitCode
            } else {
                Write-Log "Uninstalling VMware Tools via uninstall string: $u"
                $p = Start-Process -FilePath 'cmd.exe' -ArgumentList "/c $u" -Wait -PassThru
                $exitCode = $p.ExitCode
            }
            Write-Log "Uninstall exit code: $exitCode"
            $script:didWork = $true
        }
    }
    return $exitCode
}

# Function for MSI custom action hack (if standard uninstall fails)
function Try-MsiCustomActionHack {
    $installer = New-Object -ComObject WindowsInstaller.Installer
    $productsRoot = 'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Installer\UserData\S-1-5-18\Products'
    $productKeys = Get-ChildItem -Path $productsRoot -ErrorAction SilentlyContinue
    foreach ($pk in $productKeys) {
        $ip = Join-Path $pk.PSPath 'InstallProperties'
        $props = Get-ItemProperty -Path $ip -ErrorAction SilentlyContinue
        if ($props -and $props.DisplayName -eq 'VMware Tools') {
            $localPackage = $props.LocalPackage
            if ($localPackage -and (Test-Path $localPackage)) {
                Write-Log "Attempting MSI custom action hack for LocalPackage: $localPackage" 'WARNING'
                $db = $installer.OpenDatabase($localPackage, 2)
                $q = "DELETE FROM CustomAction WHERE Action='VM_LogStart' OR Action='VM_CheckRequirements'"
                $view = $db.OpenView($q)
                $view.Execute()
                $view.Close()
                $db.Commit()
                $p = Start-Process -FilePath 'msiexec.exe' -ArgumentList "/x `"$localPackage`" /qn /norestart" -Wait -PassThru
                Write-Log "MSI hack uninstall exit code: $($p.ExitCode)" 'WARNING'
                $script:didWork = $true
                break
            }
        }
    }
}

# Function to remove path forcefully
function Remove-PathForce {
    param([string]$Path)
    if (Test-Path $Path) {
        takeown.exe /f $Path /r /d y | Out-Null
        icacls.exe $Path /grant Administrators:F /t /q | Out-Null
        Remove-Item -Path $Path -Recurse -Force -ErrorAction SilentlyContinue
        $script:didWork = $true
    }
}

# Function to remove registry key
function Remove-RegKey {
    param([string]$Key)
    if (Test-Path $Key) {
        Remove-Item -Path $Key -Recurse -Force -ErrorAction SilentlyContinue
        $script:didWork = $true
    }
}

# Function to get pnputil path
function Get-PnpUtilPath {
    $sysDir = if (Test-Path "$env:WINDIR\Sysnative") { "$env:WINDIR\Sysnative" } else { "$env:WINDIR\System32" }
    return Join-Path $sysDir 'pnputil.exe'
}

# Function to remove VMware drivers from DriverStore
function Remove-DriverStoreVMware {
    $pnputil = Get-PnpUtilPath
    if (-not (Test-Path $pnputil)) { return }

    $out = & $pnputil /enum-drivers 2>&1 | Out-String
    $blocks = $out -split "(\r?\n){2,}"

    foreach ($b in $blocks) {
        $oem = $null
        $provider = $null
        $original = $null

        if ($b -match '(?im)^Published Name\s*:\s*(oem\d+\.inf)') { $oem = $matches[1] }
        if ($b -match '(?im)^Provider Name\s*:\s*(.+)$') { $provider = $matches[1].Trim() }
        if ($b -match '(?im)^Original Name\s*:\s*(.+)$') { $original = $matches[1].Trim() }

        $isVmw = $false
        if ($provider -match 'VMware') { $isVmw = $true }
        if ($original -match '(?i)vmware|vmxnet|vmmouse|vmhgfs|vmci|vm3d|vgauth|vmvss|vmvsock|vsock') { $isVmw = $true }
        if ($b -match '(?i)VMware') { $isVmw = $true }

        if ($oem -and $isVmw) {
            Write-Log "Removing driver package: $oem (Provider='$provider', Original='$original')"
            & $pnputil /delete-driver $oem /uninstall /force | Out-Null
            $script:didWork = $true
        }
    }
}

Write-Log '=== VMware Tools removal run started ==='

# Stop and delete services
$services = @(
    'VMTools','VGAuthService','VMwareCAF','VMwareCAFCommAmqpListener','VMwareCAFManagementAgentHost',
    'vmci','vm3dmp','vmaudio','vmhgfs','VMMemCtl','vmmouse','VMRawDisk','vmusbmouse','vmvss','vmvsock','vsock','vmxnet3'
)
foreach ($svc in $services) { Stop-IfExistsService $svc }

# Stop processes
$processes = @('vmtoolsd','vmwaretray','vmwareuser','VGAuthService')
foreach ($proc in $processes) { Stop-Process -Name $proc -Force -ErrorAction SilentlyContinue; $didWork = $true }

# Attempt uninstall
$uninstallExit = Try-UninstallVMwareTools
if ($uninstallExit -ne $null -and $uninstallExit -ne 0 -and $uninstallExit -ne 3010) {
    Try-MsiCustomActionHack
}

# Remove registry keys
$regKeys = @(
    'HKLM:\SOFTWARE\VMware, Inc.','HKLM:\SOFTWARE\VMware',
    'HKLM:\SOFTWARE\WOW6432Node\VMware, Inc.','HKLM:\SOFTWARE\WOW6432Node\VMware'
)
foreach ($rk in $regKeys) { Remove-RegKey $rk }

$svcRegKeys = @(
    'HKLM:\SYSTEM\CurrentControlSet\Services\vmci',
    'HKLM:\SYSTEM\CurrentControlSet\Services\vm3dmp',
    'HKLM:\SYSTEM\CurrentControlSet\Services\vmaudio',
    'HKLM:\SYSTEM\CurrentControlSet\Services\vmhgfs',
    'HKLM:\SYSTEM\CurrentControlSet\Services\VMMemCtl',
    'HKLM:\SYSTEM\CurrentControlSet\Services\vmmouse',
    'HKLM:\SYSTEM\CurrentControlSet\Services\VMRawDisk',
    'HKLM:\SYSTEM\CurrentControlSet\Services\vmrawdsk',
    'HKLM:\SYSTEM\CurrentControlSet\Services\VMTools',
    'HKLM:\SYSTEM\CurrentControlSet\Services\vmusbmouse',
    'HKLM:\SYSTEM\CurrentControlSet\Services\vmvss',
    'HKLM:\SYSTEM\CurrentControlSet\Services\vmvsock',
    'HKLM:\SYSTEM\CurrentControlSet\Services\vsock',
    'HKLM:\SYSTEM\CurrentControlSet\Services\vmxnet3',
    'HKLM:\SYSTEM\CurrentControlSet\Services\VMwareCAF',
    'HKLM:\SYSTEM\CurrentControlSet\Services\VMwareCAFCommAmqpListener',
    'HKLM:\SYSTEM\CurrentControlSet\Services\VMwareCAFManagementAgentHost'
)
foreach ($sk in $svcRegKeys) { Remove-RegKey $sk }

# Remove files and folders
$paths = @(
    'C:\Program Files\VMware',
    'C:\Program Files (x86)\VMware',
    'C:\Program Files\Common Files\VMware',
    'C:\Program Files (x86)\Common Files\VMware',
    'C:\ProgramData\VMware',
    'C:\Windows\System32\drivers\vmci.sys',
    'C:\Windows\System32\drivers\vm3dmp.sys',
    'C:\Windows\System32\drivers\vmaudio.sys',
    'C:\Windows\System32\drivers\vmhgfs.sys',
    'C:\Windows\System32\drivers\vmmemctl.sys',
    'C:\Windows\System32\drivers\vmmouse.sys',
    'C:\Windows\System32\drivers\vmrawdsk.sys',
    'C:\Windows\System32\drivers\vmusbmouse.sys',
    'C:\Windows\System32\drivers\vmvss.sys',
    'C:\Windows\System32\drivers\vmvsock.sys',
    'C:\Windows\System32\drivers\vsock.sys',
    'C:\Windows\System32\drivers\vmxnet3.sys'
)
foreach ($p in $paths) { Remove-PathForce $p }

# Remove drivers from DriverStore
Remove-DriverStoreVMware

# Test if any remnants remain
function Test-Remaining {
    $testPaths = @(
        'C:\Program Files\VMware',
        'C:\Program Files (x86)\VMware',
        'C:\ProgramData\VMware'
    )
    foreach ($p in $testPaths) { if (Test-Path $p) { return $true } }

    $apps = Get-ItemProperty -Path 'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\*', 'HKLM:\SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall\*' -ErrorAction SilentlyContinue
    $vmw = $apps | Where-Object { $_.DisplayName -like '*VMware Tools*' }
    if ($vmw) { return $true }

    return $false
}

if (-not (Test-Remaining)) {
    Write-Log 'No remaining VMware Tools artifacts detected. Marking complete.'
    New-Item -ItemType File -Path $MarkerPath -Force | Out-Null
    schtasks /Delete /TN $TaskName /F | Out-Null
    # Exit with 3010 to indicate potential final reboot if needed, but since clean, can be 0
    exit 0
}

Write-Log 'VMware Tools artifacts still detected; rebooting to continue cleanup.' 'WARNING'
Restart-Computer -Force -Delay 5
exit 0
