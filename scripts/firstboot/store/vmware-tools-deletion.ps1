<#
.SYNOPSIS
    VMware Tools removal script.
    Designed for firstboot / scheduler-controlled environments.
#>

param(
    [string]$LogPath = "C:\VMware_Removal_Log.txt"
)

$ErrorActionPreference = "SilentlyContinue"

$ProgressPreference = 'SilentlyContinue'
$VerbosePreference  = 'SilentlyContinue'
$InformationPreference = 'SilentlyContinue'

function Write-Log {
    param([string]$Message,[string]$Level="INFO")
    $timestamp = Get-Date -Format "yyyy-MM-dd HH:mm:ss"
    "$timestamp [$Level] $Message" | Out-File -Append -FilePath $LogPath
}

function Schedule-DeleteOnReboot {
    param([string]$Path)

    $regPath="HKLM:\SYSTEM\CurrentControlSet\Control\Session Manager"
    $current=(Get-ItemProperty $regPath -Name PendingFileRenameOperations -ErrorAction SilentlyContinue).PendingFileRenameOperations

    if(!$current){$current=@()}

    if($current -contains $Path){ return }

    $new=@($Path,"")
    Set-ItemProperty $regPath -Name PendingFileRenameOperations -Value ($current + $new)
}

function Stop-VMwareProcesses {

    Write-Log "Stopping VMware processes"

    $procs=@(
    "vmtoolsd","vm3dservice","VGAuthService",
    "vmwaretray","vmwareuser","vmware-svga"
    )

    foreach($p in $procs){
        Get-Process -Name $p -ErrorAction SilentlyContinue | Stop-Process -Force
    }
}

function Remove-VMwareServices {

    Write-Log "Removing VMware services"

    $services=@(
    "VMTools","vm3dservice","VGAuthService",
    "VMwareCAF","VMwareCAFCommAmqpListener","VMwareCAFManagementAgentHost",
    "vmci","vm3dmp","vm3dmp_loader","vm3dmp-debug","vm3dmp-stats",
    "vmaudio","vmhgfs","VMMemCtl",
    "vmmouse","VMRawDisk","vmrawdsk",
    "vmusbmouse","vmvss","vsock",
    "vmxnet3","vnetWFP","WmiApSrv"
    )

    foreach($svc in $services){

        sc.exe query $svc *> $null

        if($LASTEXITCODE -eq 0){
            try {
                sc.exe delete $svc *> $null
                Write-Log "Deleted service $svc"
            }
            catch {
                Write-Log "Failed to delete service $svc"
            }
        }
    }
}

function Remove-VMwareDrivers {

    Write-Log "Removing VMware drivers"

    sc.exe stop vmmouse *> $null
    sc.exe delete vmmouse *> $null
    sc stop VMMemCtl *> $null
    sc delete VMMemCtl *> $null

    $drivers=@(
    "vmci.sys","vm3dmp.sys","vm3dmp_loader.sys",
    "vm3dmp-debug.sys","vm3dmp-stats.sys",
    "vmaudio.sys","vmhgfs.sys",
    "vmmemctl.sys","vmmouse.sys",
    "vmrawdsk.sys","vmtools.sys",
    "vmusbmouse.sys","vmvss.sys",
    "vsock.sys","vmx_svga.sys",
    "vmxnet3.sys","vmgencounter.sys",
    "vmgid.sys","vms3cap.sys",
    "vmstorfl.sys","vmscsi.sys"
    )

    $driverPath="C:\Windows\System32\drivers"

    foreach($d in $drivers){

        $full=Join-Path $driverPath $d

        if(Test-Path $full){

            try{

                takeown /F $full | Out-Null
                icacls $full /grant Administrators:F | Out-Null
                Remove-Item $full -Force

                Write-Log "Removed driver $d"

            }catch{

                Schedule-DeleteOnReboot $full
                Write-Log "Scheduled driver delete $d"
            }
        }
    }
}

function Remove-DriverStore {

    Write-Log "Cleaning DriverStore VMware entries"

    $drivers = pnputil /enum-drivers *> $null

    $published=""
    $provider=""

    foreach($line in $drivers){

        if($line -match "Published Name\s*:\s*(oem\d+\.inf)"){
            $published=$matches[1]
        }

        if($line -match "Provider Name\s*:\s*(.+)"){
            $provider=$matches[1].Trim()
        }

        if($published -and $provider){

            if($provider -match "VMware"){

                Write-Log "Removing driverstore $published"

                pnputil /delete-driver $published /force *> $null
            }

            $published=""
            $provider=""
        }
    }
}

function Remove-VMwareDevices {

    Write-Log "Removing VMware hidden devices"

    $devices = pnputil /enum-devices

    foreach($line in $devices){

        if($line -match "Instance ID:\s*(.+)"){
            $id=$matches[1]
        }

        if($line -match "Driver Name:\s*(oem\d+\.inf)"){

            if($line -match "vm"){

                Write-Log "Removing device $id"

                pnputil /remove-device "$id" | Out-Null
            }
        }
    }
}

function Unregister-VMwareDLLs {
    Write-Log "Unregistering VMware COM/VSS/WMI providers"
    $dlls = @(
        "C:\Program Files\Common Files\VMware\Drivers\vss\VCBSnapshotProvider.dll",
        "C:\Program Files\VMware\VMware Tools\vmStatsProvider\win64\vmStatsProvider.dll"
    )
    foreach ($dll in $dlls) {
        if (Test-Path $dll) {
            try {
                regsvr32 /s /u $dll
                Write-Log "Unregistered $dll"
            } catch { Write-Log "Failed to unregister $dll" }
        }
    }
    Stop-Service -Name "WmiApSrv" -Force -ErrorAction SilentlyContinue
}

function Remove-VMwareFolderAggressive {
    param([string]$TargetPath)

    if (!(Test-Path $TargetPath)) {
        Write-Log "$TargetPath not present"
        return
    }

    Write-Log "Aggressively removing folder: $TargetPath"

    Stop-Process -Name "vmtoolsd","vmwareuser","vmwaretray" -Force -ErrorAction SilentlyContinue
    Stop-Service -Name "WmiApSrv" -Force -ErrorAction SilentlyContinue

    takeown /F $TargetPath /R /D Y | Out-Null
    icacls $TargetPath /grant Administrators:F /T | Out-Null

    $empty = Join-Path $env:TEMP "empty_$(Get-Random)"
    New-Item -ItemType Directory -Path $empty -Force | Out-Null
    robocopy "$empty" "$TargetPath" /MIR /R:5 /W:5 /NP /NFL /NDL /NJH /NJS | Out-Null
    Remove-Item $empty -Recurse -Force -ErrorAction SilentlyContinue

    Get-ChildItem $TargetPath -Recurse -Force -File | ForEach-Object {
        try {
            Remove-Item $_.FullName -Force -ErrorAction Stop
            Write-Log "Deleted: $($_.FullName)"
        } catch {
            Schedule-DeleteOnReboot $_.FullName
            Write-Log "Scheduled on reboot: $($_.FullName)"
        }
    }

    try {
        Remove-Item $TargetPath -Recurse -Force -ErrorAction Stop
        Write-Log "Removed folder: $TargetPath"
    } catch {
        Schedule-DeleteOnReboot $TargetPath
        Write-Log "Scheduled folder on reboot: $TargetPath"
    }
}

function Remove-VMwareFolders {
    Write-Log "Removing VMware folders aggressively"
    Remove-VMwareFolderAggressive "C:\Program Files\VMware\VMware Tools"
    Remove-VMwareFolderAggressive "C:\Program Files\Common Files\VMware"
    Remove-VMwareFolderAggressive "C:\Program Files (x86)\VMware"
    Remove-VMwareFolderAggressive "C:\ProgramData\VMware"
    Remove-VMwareFolderAggressive "C:\ProgramData\Microsoft\Windows\Start Menu\Programs\VMware"
}


function Remove-VMwareRegistry {

    Write-Log "Cleaning VMware registry"

    $keys=@(
    "HKLM:\SOFTWARE\VMware, Inc.",
    "HKLM:\SOFTWARE\VMware",
    "HKLM:\SOFTWARE\WOW6432Node\VMware, Inc.",
    "HKLM:\SOFTWARE\WOW6432Node\VMware"
    )

    foreach($k in $keys){
        Remove-Item $k -Recurse -Force
    }

    $run="HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Run"

    Remove-ItemProperty -Path $run -Name "VMware User Process" -ErrorAction SilentlyContinue
    Remove-ItemProperty -Path $run -Name "vmtoolsd" -ErrorAction SilentlyContinue
}

function Remove-ControlPanelEntry {

    Write-Log "Removing VMware uninstall entries"

    $paths=@(
    "HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall",
    "HKLM:\SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall"
    )

    foreach($p in $paths){

        Get-ChildItem $p | ForEach-Object{

            $disp=(Get-ItemProperty $_.PSPath).DisplayName

            if($disp -match "VMware"){
                Remove-Item $_.PSPath -Recurse -Force
                Write-Log "Removed uninstall entry $disp"
            }
        }
    }
}

function Remove-VMwarePnPDevicesAggressive {
    Write-Log "Removing remaining VMware PnP devices from Device Manager (including hidden)"

    $vmDevices = Get-PnpDevice | Where-Object {
        $_.FriendlyName  -like "*VMware*" -or
        $_.InstanceId    -like "*VMWARE*" -or
        $_.Class         -like "*VMware*" -or
        $_.Manufacturer  -like "*VMware*"
    }

    Stop-Process -Name "vmtoolsd","vmwareuser" -Force -ErrorAction SilentlyContinue

    foreach ($dev in $vmDevices) {

        try {
            Write-Log "Removing device: $($dev.FriendlyName) [$($dev.InstanceId)]"

            Disable-PnpDevice -InstanceId $dev.InstanceId -Confirm:$false -ErrorAction SilentlyContinue

            pnputil /remove-device "$($dev.InstanceId)" /force *> $null

            $wmiDev = Get-WmiObject -Class Win32_PnPEntity | Where-Object { $_.DeviceID -eq $dev.InstanceId }
            if ($wmiDev) { $wmiDev.Delete() | Out-Null }

            Write-Log "Removed: $($dev.FriendlyName)"
        }
        catch {
            Write-Log "Failed to remove $($dev.FriendlyName) - may require reboot"
        }
    }

    Write-Log "VMware PnP device cleanup completed"
}

function Remove-VMwareResiduals {

    Write-Log "Running additional VMware residual cleanup"

    # Ghost MSI uninstall
    try{
        $ghost = Get-WmiObject -Class Win32_Product | Where-Object {
            $_.IdentifyingNumber -eq "{AF174E64-22CF-4386-A9EC-73F285739998}"
        }

        if($ghost){
            Write-Log "Removing ghost VMware MSI registration"
            $ghost.Uninstall() | Out-Null
        }

    }catch{
        Write-Log "Ghost MSI uninstall failed"
    }

    # Remove installer registry leftovers
    Remove-Item -Path "HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Installer\UserData\S-1-5-18\Products\46E471FAFC2268349ACE372F58379989" -Recurse -Force -ErrorAction SilentlyContinue

    Remove-Item -Path "HKLM:\SOFTWARE\Classes\Installer\Products\46E471FAFC2268349ACE372F58379989" -Recurse -Force -ErrorAction SilentlyContinue


    # Remove VMware devices via WMI fallback
    try{

        $devices = Get-WmiObject Win32_PnPEntity | Where-Object {$_.Name -match "VMware"}

        foreach ($dev in $devices){

            Write-Log "Removing device $($dev.Name)"

            try{
                $dev.Delete() | Out-Null
            }catch{
                Write-Log "Device removal failed for $($dev.Name)"
            }
        }

    }catch{
        Write-Log "Device enumeration failed"
    }


    # Remove VMMemCtl registry key
    $key = "HKLM:\SYSTEM\CurrentControlSet\Services\VMMemCtl"

    if(Test-Path $key){

        Write-Log "Removing VMMemCtl registry key"

        try{

            $acl = Get-Acl $key
            $owner = New-Object System.Security.Principal.NTAccount("Administrators")
            $acl.SetOwner($owner)
            Set-Acl $key $acl

            $rule = New-Object System.Security.AccessControl.RegistryAccessRule(
                "Administrators","FullControl","ContainerInherit,ObjectInherit","None","Allow")

            $acl = Get-Acl $key
            $acl.SetAccessRule($rule)
            Set-Acl $key $acl

            Remove-Item $key -Recurse -Force

            Write-Log "VMMemCtl registry removed"

        }catch{

            Write-Log "Failed to remove VMMemCtl registry key"
        }
    }
}

Write-Log "=== VMware Cleanup Start ==="

Stop-VMwareProcesses
Unregister-VMwareDLLs
Remove-VMwareServices
Remove-VMwareDrivers
Remove-DriverStore
Remove-VMwareDevices
Remove-VMwarePnPDevicesAggressive
Remove-VMwareFolders
Remove-VMwareRegistry
Remove-ControlPanelEntry
Remove-VMwareResiduals



Write-Log "Cleanup completed"

Restart-Computer -Force

exit 0