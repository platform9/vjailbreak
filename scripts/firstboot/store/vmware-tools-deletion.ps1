<#
.SYNOPSIS
    VMware Tools removal script.
    Designed for firstboot / scheduler-controlled environments.
#>

param(
    [string]$LogPath = "C:\VMware_Removal_Log.txt"
)

$ErrorActionPreference = "SilentlyContinue"

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
    "vmxnet3","vnetWFP"
    )

    foreach($svc in $services){

        sc.exe query $svc | Out-Null

        if($LASTEXITCODE -eq 0){
            try {
                sc.exe delete $svc | Out-Null
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

    $drivers=@(
    "vmci.sys","vm3dmp.sys","vm3dmp_loader.sys",
    "vm3dmp-debug.sys","vm3dmp-stats.sys",
    "vmaudio.sys","vmhgfs.sys",
    "vmmemctl.sys","vmmouse.sys",
    "vmrawdsk.sys","vmtools.sys",
    "vmusbmouse.sys","vmvss.sys",
    "vsock.sys","vmx_svga.sys","vmxnet3.sys"
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

    $drivers=pnputil /enum-drivers

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

                pnputil /delete-driver $published /uninstall /force | Out-Null
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

function Remove-VMwareFolders {

    Write-Log "Removing VMware folders"

    $paths=@(
    "C:\Program Files\VMware",
    "C:\Program Files (x86)\VMware",
    "C:\Program Files\Common Files\VMware",
    "C:\Program Files (x86)\Common Files\VMware",
    "C:\ProgramData\VMware",
    "C:\ProgramData\Microsoft\Windows\Start Menu\Programs\VMware"
    )

    foreach($p in $paths){

        if(Test-Path $p){

            try{

                takeown /F $p /R /D Y | Out-Null
                icacls $p /grant Administrators:F /T | Out-Null
                Remove-Item $p -Recurse -Force

            }catch{

                Schedule-DeleteOnReboot $p
            }
        }
    }
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


    # Force remove VMware folder if present
    $vmwarePath = "C:\Program Files\VMware"

    if(Test-Path $vmwarePath){

        Write-Host "VMware folder detected, attempting cleanup..."

        $devices = Get-WmiObject Win32_PnPEntity | Where-Object { $_.Name -match "VMware" }

        foreach ($dev in $devices) {
            $instanceId = $dev.DeviceID
            Write-Host "Final remove device:" $dev.Name
            pnputil /remove-device "$instanceId" /force 2>$null
        }

        try{
            takeown /F "$vmwarePath" /R /D Y | Out-Null
            icacls "$vmwarePath" /grant Administrators:F /T | Out-Null
            Remove-Item "$vmwarePath" -Recurse -Force -ErrorAction Stop
            Write-Host "VMware folder removed immediately"
        }
        catch{

            Write-Host "Folder locked, scheduling delete on reboot..."

            $regPath = "HKLM:\SYSTEM\CurrentControlSet\Control\Session Manager"

            reg delete "HKLM\SYSTEM\CurrentControlSet\Control\Session Manager" /v PendingFileRenameOperations /f 2>$null

            $dll = "\??\C:\Program Files\VMware\VMware Tools\vmStatsProvider\win64\vmStatsProvider.dll"

            $folder = "\??\C:\Program Files\VMware"

            Set-ItemProperty `
                -Path $regPath `
                -Name PendingFileRenameOperations `
                -Value @($dll,"",$folder,"") `
                -Type MultiString `
                -Force

            Write-Host "VMware folder scheduled for deletion at reboot"
        }
    }
    else{
        Write-Host "VMware folder not present"
    }


    # Force remove vmmemctl driver file
    $drv="C:\Windows\System32\drivers\vmmemctl.sys"

    if(Test-Path $drv){

        Write-Log "Removing vmmemctl driver"

        takeown /F $drv | Out-Null
        icacls $drv /grant Administrators:F | Out-Null
        Remove-Item $drv -Force -ErrorAction SilentlyContinue
    }


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


    # Force delete VMMemCtl registry key if still present
    $key = "HKLM:\SYSTEM\CurrentControlSet\Services\VMMemCtl"

    if(Test-Path $key){

        Write-Log "Force removing VMMemCtl service key"

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

        }catch{

            Write-Log "Failed to remove VMMemCtl registry key"
        }
    }
}

Write-Log "=== VMware Cleanup Start ==="

Stop-VMwareProcesses
Remove-VMwareServices
Remove-VMwareDrivers
Remove-DriverStore
Remove-VMwareDevices
Remove-VMwareFolders
Remove-VMwareRegistry
Remove-ControlPanelEntry
Remove-VMwareResiduals



Write-Log "Cleanup completed"

Restart-Computer -Force

exit 0
