package constants

import "time"

const (
	HotplugCPUKey       = "HOTPLUG_CPU"
	HotplugMemoryKey    = "HOTPLUG_MEMORY"
	HotplugCPUMaxKey    = "HOTPLUG_CPU_MAX"
	HotplugMemoryMaxKey = "HOTPLUG_MEMORY_MAX"
	RhelFirstBootScript = `#!/bin/bash
set -e
LOG_FILE="/var/log/network_fix.log"
echo "$(date '+%Y-%m-%d %H:%M:%S') - Starting network fix script" >> "$LOG_FILE"

if ! systemctl is-active NetworkManager >/dev/null 2>&1; then
    echo "$(date '+%Y-%m-%d %H:%M:%S') - NetworkManager not active, attempting to start" >> "$LOG_FILE"
    systemctl start NetworkManager
    if ! systemctl is-active NetworkManager >/dev/null 2>&1; then
        echo "$(date '+%Y-%m-%d %H:%M:%S') - Failed to start NetworkManager, exiting" >> "$LOG_FILE"
        exit 1
    fi
fi
echo "$(date '+%Y-%m-%d %H:%M:%S') - NetworkManager is active" >> "$LOG_FILE"

nmcli con reload || {
    echo "$(date '+%Y-%m-%d %H:%M:%S') - Warning: Reload failed, restarting NM" >> "$LOG_FILE"
    systemctl restart NetworkManager
    sleep 5
    nmcli con reload || echo "$(date '+%Y-%m-%d %H:%M:%S') - Error: Reload still failed" >> "$LOG_FILE"
}

OLD_CONNS=$(nmcli -t -f NAME,TYPE connection show | grep -v ':loopback' | cut -d: -f1)
if [ -z "$OLD_CONNS" ]; then
    echo "$(date '+%Y-%m-%d %H:%M:%S') - No existing connections found" >> "$LOG_FILE"
else
    echo "$(date '+%Y-%m-%d %H:%M:%S') - Found connections: $OLD_CONNS" >> "$LOG_FILE"
fi

for conn in $OLD_CONNS; do
    echo "$(date '+%Y-%m-%d %H:%M:%S') - Processing connection: $conn" >> "$LOG_FILE"
    nmcli con mod "$conn" ipv4.method auto ipv4.addresses "" ipv4.gateway "" 2>>"$LOG_FILE" || \
        echo "$(date '+%Y-%m-%d %H:%M:%S') - Error: Failed to modify IPv4 for $conn" >> "$LOG_FILE"
    nmcli con mod "$conn" ipv6.method auto ipv6.addresses "" ipv6.gateway "" 2>>"$LOG_FILE" || \
        echo "$(date '+%Y-%m-%d %H:%M:%S') - Error: Failed to modify IPv6 for $conn" >> "$LOG_FILE"
    nmcli con up "$conn" 2>>"$LOG_FILE" || \
        echo "$(date '+%Y-%m-%d %H:%M:%S') - Warning: Failed to activate $conn" >> "$LOG_FILE"
done

NEW_IFACES=$(ip link show | grep -o '^[0-9]\+: [a-zA-Z0-9]\+:' | cut -d ' ' -f2 | cut -d ':' -f1 | grep -v lo)
if [ -z "$NEW_IFACES" ]; then
    echo "$(date '+%Y-%m-%d %H:%M:%S') - No new interfaces detected" >> "$LOG_FILE"
else
    echo "$(date '+%Y-%m-%d %H:%M:%S') - Detected interfaces: $NEW_IFACES" >> "$LOG_FILE"
fi

for iface in $NEW_IFACES; do
    echo "$(date '+%Y-%m-%d %H:%M:%S') - Processing new interface: $iface" >> "$LOG_FILE"
    conn_name="$iface"
    if ! nmcli con show "$conn_name" >/dev/null 2>&1; then
        echo "$(date '+%Y-%m-%d %H:%M:%S') - Creating new connection for $iface" >> "$LOG_FILE"
        nmcli con add type ethernet con-name "$conn_name" ifname "$iface" ipv4.method auto ipv6.method auto 2>>"$LOG_FILE" || \
            echo "$(date '+%Y-%m-%d %H:%M:%S') - Error: Failed to create connection for $iface" >> "$LOG_FILE"
    fi
    nmcli con up "$conn_name" 2>>"$LOG_FILE" || \
        echo "$(date '+%Y-%m-%d %H:%M:%S') - Error: Failed to activate $iface" >> "$LOG_FILE"
done
echo "$(date '+%Y-%m-%d %H:%M:%S') - Network fix script completed" >> "$LOG_FILE"`

	MaxCPU = 9999999
	MaxRAM = 9999999

	// Number of intervals to wait for the volume to become available
	MaxIntervalCount = 60

	InspectOSCommand         = "inspect-os"
	LSBootCommand            = "ls /boot"
	XMLFileName              = "libxml.xml"
	MigrationSnapshotName    = "migration-snap"
	MaxHTTPRetryCount        = 5
	MaxVMActiveCheckCount    = 15
	VMActiveCheckInterval    = 20 * time.Second
	NamespaceMigrationSystem = "migration-system"
	TrueString               = "true"

	LogsDir = "/var/log/pf9"

	EventMessageConvertingDisk                    = "Converting disk"
	EventMessageWaitingForCutOverStart            = "Waiting for VM Cutover start time"
	EventMessageCopyingChangedBlocksWithIteration = "Copying changed blocks"
	EventMessageWaitingForDataCopyStart           = "Waiting for data copy start time"
	EventMessageDataCopyStart                     = "Data copy start time reached"
	EventMessageWaitingForAdminCutOver            = "Waiting for Admin Cutover conditions to be met"
	EventMessageMigrationSucessful                = "VM created successfully"
	EventMessageMigrationFailed                   = "Trying to perform cleanup"
	EventMessageCopyingDisk                       = "Copying disk"
	EventMessageFailed                            = "Failed to"
	EventDisconnect                               = "Disconnected network interfaces"

	// StorageAcceleratedCopy specific event messages
	EventMessageEsxiSSHConnect                       = "Connecting to ESXi"
	EventMessageEsxiSSHTest                          = "Testing ESXi connection"
	EventMessageEsxiConnected                        = "Connected to ESXi"
	EventMessageInitiatorGroup                       = "Creating/updating initiator group"
	EventMessageStorageAcceleratedCopyCreatingVolume = "Creating target volume"
	EventMessageStorageAcceleratedCopyCinderManage   = "Cinder managing the volume"
	EventMessageStorageAcceleratedCopyMappingVolume  = "Mapping target volume"
	EventMessageStorageAcceleratedCopyRescanStorage  = "Waiting for target volume"
	EventMessageStorageAcceleratedCopyTargetDevice   = "Target device is visible:"

	OSFamilyWindows = "windowsguest"
	OSFamilyLinux   = "linuxguest"

	PCDClusterNameNoCluster = "NO CLUSTER"

	// VCenterVMScanConcurrencyLimit is the limit for concurrency while scanning vCenter VMs
	VCenterVMScanConcurrencyLimit = 100

	// ConfigMap default values
	ChangedBlocksCopyIterationThreshold = 20
	PeriodicSyncInterval                = "1h"
	// VMActiveWaitIntervalSeconds is the interval to wait for vm to become active
	VMActiveWaitIntervalSeconds = 20

	// VMActiveWaitRetryLimit is the number of retries to wait for vm to become active
	VMActiveWaitRetryLimit = 15

	// VolumeAvailableWaitIntervalSeconds is the interval to wait for volume to become available
	VolumeAvailableWaitIntervalSeconds = 5

	// VolumeAvailableWaitRetryLimit is the number of retries to wait for volume to become available
	VolumeAvailableWaitRetryLimit = 15

	// DefaultMigrationMethod is the default migration method
	DefaultMigrationMethod = "cold"

	// VCenterScanConcurrencyLimit is the max number of vcenter scan pods
	VCenterScanConcurrencyLimit = 100

	// CleanupVolumesAfterConvertFailure is the default value for cleanup volumes after convert failure
	CleanupVolumesAfterConvertFailure = true

	// CleanupPortsAfterMigrationFailure is the default value for cleanup ports after migration failure
	CleanupPortsAfterMigrationFailure = false

	// PopulateVMwareMachineFlavors is the default value for populating VMwareMachine objects with OpenStack flavors
	PopulateVMwareMachineFlavors = true

	// ValidateRDMOwnerVMs is the default value for RDM owner VM validation
	ValidateRDMOwnerVMs = true

	// VjailbreakSettingsConfigMapName is the name of the vjailbreak settings configmap
	VjailbreakSettingsConfigMapName = "vjailbreak-settings"

	// VCenterLoginRetryLimit is the number of retries for vcenter login
	VCenterLoginRetryLimit = 1

	// OpenstackCredsRequeueAfterMinutes is the time to requeue after.
	OpenstackCredsRequeueAfterMinutes = 60

	// VMwareCredsRequeueAfterMinutes is the time to requeue after.
	VMwareCredsRequeueAfterMinutes = 60

	// PeriodicSyncMaxRetries is the max number of retries for CBT sync
	PeriodicSyncMaxRetries = 3

	// PeriodicSyncRetryCap is the max retry interval for CBT sync
	PeriodicSyncRetryCap = "3h"
	// ValidateRDMOwnerVMsKey is the key for enabling/disabling RDM owner VM validation
	ValidateRDMOwnerVMsKey = "VALIDATE_RDM_OWNER_VMS"

	// ESXiSSHSecretName is the name of the Kubernetes secret containing ESXi SSH private key
	ESXiSSHSecretName = "esxi-ssh-key"

	// AutoFstabUpdate is the default value for automatic fstab update
	AutoFstabUpdate = false
	// AutoFstabUpdateKey is the key for enabling/disabling automatic fstab update
	AutoFstabUpdateKey = "AUTO_FSTAB_UPDATE"

	// AutoPXEBootOnConversionDefault is the default value for automatic PXE boot during cluster conversion
	AutoPXEBootOnConversionDefault = false
	// AutoPXEBootOnConversionKey is the key for enabling/disabling automatic PXE boot during cluster conversion
	AutoPXEBootOnConversionKey = "AUTO_PXE_BOOT_ON_CONVERSION"

	// StorageCopyMethod is the default value for storage copy method
	StorageCopyMethod = "StorageAcceleratedCopy"

	// Windows Script
	WindowsFirtsBootNetworkPersistence = `@echo off
set "TargetDir=C:\NIC-Recovery"

:: 1. Initialize Directory
if not exist "%TargetDir%" mkdir "%TargetDir%"
cd /d "%TargetDir%"

echo Writing Discovery Script...
echo # Recover-HiddenNICMapping.ps1 > "Recover-HiddenNICMapping.ps1"
echo $OutFile = "C:\NIC-Recovery\netconfig.json" >> "Recover-HiddenNICMapping.ps1"
echo function Convert-SubnetToPrefix { param ([string]$Mask^) ($Mask -split '\.'^) ^| ForEach-Object { [Convert]::ToString([int]$_,2^) } ^| ForEach-Object { $_.ToCharArray(^) } ^| Where-Object { $_ -eq '1' } ^| Measure-Object ^| Select-Object -ExpandProperty Count } >> "Recover-HiddenNICMapping.ps1"
echo function Get-Network { param ([string]$IP, [int]$Prefix^) $ipBytes = ([System.Net.IPAddress]::Parse($IP^)^).GetAddressBytes(^); $maskBytes = @(0,0,0,0^); for ($i=0; $i -lt 4; $i++^) { $bits = [Math]::Min(8, $Prefix - ($i*8^)^); if ($bits -gt 0^) { $maskBytes[$i] = [byte](255 -shl (8-$bits^)^) } }; for ($i=0; $i -lt 4; $i++^) { $ipBytes[$i] = $ipBytes[$i] -band $maskBytes[$i] }; ([System.Net.IPAddress]$ipBytes^).ToString(^) } >> "Recover-HiddenNICMapping.ps1"
echo $activeNics = try { Get-NetIPConfiguration -ErrorAction Stop ^| Where-Object { $_.IPv4Address } ^| ForEach-Object { foreach ($ip in $_.IPv4Address^) { [PSCustomObject]@{ InterfaceAlias = $_.InterfaceAlias; MACAddress = $_.NetAdapter.MacAddress; Network = Get-Network $ip.IPAddress $ip.PrefixLength } } } } catch { $null } >> "Recover-HiddenNICMapping.ps1"
echo $activeAliases = try { Get-NetAdapter -ErrorAction SilentlyContinue ^| Select-Object -ExpandProperty InterfaceAlias } catch { @() } >> "Recover-HiddenNICMapping.ps1"
echo $hiddenIPs = Get-ChildItem "HKLM:\SYSTEM\CurrentControlSet\Services\Tcpip\Parameters\Interfaces" ^| ForEach-Object { $p = Get-ItemProperty $_.PsPath -ErrorAction SilentlyContinue; $ip = ($p.IPAddress ^| Where-Object { $_ -and $_ -ne '0.0.0.0' } ^| Select-Object -First 1^); $mask = ($p.SubnetMask ^| Select-Object -First 1^); if (-not $ip -or -not $mask^) { return }; $prefix = Convert-SubnetToPrefix $mask; $dns = @(($p.NameServer -split ','^), ($p.DhcpNameServer -split ','^)^) ^| Where-Object { $_ -and $_.Trim(^) }; [PSCustomObject]@{ GUID = $_.PSChildName.ToUpper(^); IPAddress = $ip; PrefixLength = $prefix; Network = Get-Network $ip $prefix; Gateway = ($p.DefaultGateway ^| Select-Object -First 1^); DNSServers = $dns } } >> "Recover-HiddenNICMapping.ps1"
echo $hiddenNames = Get-ChildItem "HKLM:\SYSTEM\CurrentControlSet\Control\Network\{4D36E972-E325-11CE-BFC1-08002BE10318}" ^| ForEach-Object { $conn = Join-Path $_.PsPath "Connection"; if (-not (Test-Path $conn^)^) { return }; $p = Get-ItemProperty $conn -ErrorAction SilentlyContinue; if ($p.Name -and $p.Name -notin $activeAliases^) { [PSCustomObject]@{ GUID = $_.PSChildName.ToUpper(^); Name = $p.Name } } } >> "Recover-HiddenNICMapping.ps1"
echo $result = foreach ($hidden in $hiddenIPs^) { $name = $hiddenNames ^| Where-Object { $_.GUID -eq $hidden.GUID }; if (-not $name^) { continue }; $match = $activeNics ^| Where-Object { $_.Network -eq $hidden.Network } ^| Select-Object -First 1; if (-not $match^) { continue }; [PSCustomObject]@{ InterfaceAlias = $name.Name; MACAddress = $mac = $match.MACAddress; IPAddress = $hidden.IPAddress; PrefixLength = $hidden.PrefixLength; Gateway = if ($hidden.Gateway^) { $hidden.Gateway } else { $null }; DNSServers = @($hidden.DNSServers^) } } >> "Recover-HiddenNICMapping.ps1"
echo if (-not $result^) { $result = @(^) }; $result ^| ConvertTo-Json -Depth 4 ^| Set-Content -Encoding UTF8 $OutFile >> "Recover-HiddenNICMapping.ps1"

echo Writing Cleanup Script...
echo Write-Host "=== Removing Ghost Network Adapters ===" -ForegroundColor Cyan > "Cleanup-GhostNICs.ps1"
echo $ghosts = Get-PnpDevice -Class Net ^| Where-Object Status -eq "Unknown" >> "Cleanup-GhostNICs.ps1"
echo if ^(-not $ghosts^) { Write-Host "No ghost NICs found."; return } >> "Cleanup-GhostNICs.ps1"
echo foreach ^($dev in $ghosts^) { $fName = $dev.FriendlyName; $iId = $dev.InstanceId; Write-Host "Removing $fName"; $path = "HKLM:\SYSTEM\CurrentControlSet\Enum\$iId"; if ^(Test-Path $path^) { Get-Item $path ^| Select-Object -ExpandProperty Property ^| ForEach-Object { Remove-ItemProperty -Path $path -Name $_ -Force -ErrorAction SilentlyContinue } } } >> "Cleanup-GhostNICs.ps1"

echo Writing Restore Script...
echo $ErrorActionPreference = "Stop" > "Restore-Network.ps1"
echo Start-Sleep -Seconds 15 >> "Restore-Network.ps1"
echo $configs = Get-Content "C:\NIC-Recovery\netconfig.json" ^| ConvertFrom-Json >> "Restore-Network.ps1"
echo foreach ^($cfg in $configs^) { $alias = $cfg.InterfaceAlias; $mac = $cfg.MACAddress; Write-Host "Configuring $alias" -ForegroundColor Cyan; $nic = Get-NetAdapter ^| Where-Object { ^($_.MacAddress -replace '[-:]',''^) -eq ^($mac -replace '[-:]',''^) }; if ^(-not $nic^) { continue }; Set-NetIPInterface -InterfaceIndex $nic.ifIndex -Dhcp Disabled -Confirm:$false; Get-NetIPAddress -InterfaceIndex $nic.ifIndex -AddressFamily IPv4 -ErrorAction SilentlyContinue ^| Remove-NetIPAddress -Confirm:$false -ErrorAction SilentlyContinue; if ^($nic.Name -ne $alias^) { Rename-NetAdapter -Name $nic.Name -NewName $alias -Confirm:$false; $nic = Get-NetAdapter -Name $alias }; $params = @{ InterfaceIndex = $nic.ifIndex; IPAddress = $cfg.IPAddress; PrefixLength = $cfg.PrefixLength; AddressFamily = "IPv4" }; if ^($cfg.Gateway^) { $params.DefaultGateway = $cfg.Gateway }; New-NetIPAddress @params; if ^($cfg.DNSServers -and $cfg.DNSServers.Count -gt 0^) { Set-DnsClientServerAddress -InterfaceIndex $nic.ifIndex -ServerAddresses $cfg.DNSServers } } >> "Restore-Network.ps1"

echo Writing Orchestrator...
echo $ScriptRoot = "C:\NIC-Recovery" > "Orchestrate-NICRecovery.ps1"
echo Start-Service NetSetupSvc -ErrorAction SilentlyContinue >> "Orchestrate-NICRecovery.ps1"
echo Start-Sleep -Seconds 5 >> "Orchestrate-NICRecovery.ps1"
echo ^& "$ScriptRoot\Recover-HiddenNICMapping.ps1" >> "Orchestrate-NICRecovery.ps1"
echo ^& "$ScriptRoot\Cleanup-GhostNICs.ps1" >> "Orchestrate-NICRecovery.ps1"
echo if ^(Test-Path "$ScriptRoot\netconfig.json"^) { >> "Orchestrate-NICRecovery.ps1"
echo     # Inject into RunOnce Registry Key instead of Task Scheduler >> "Orchestrate-NICRecovery.ps1"
echo     $cmd = 'powershell.exe -NoProfile -ExecutionPolicy Bypass -WindowStyle Hidden -File "C:\NIC-Recovery\Restore-Network.ps1"' >> "Orchestrate-NICRecovery.ps1"
echo     Set-ItemProperty -Path "HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\RunOnce" -Name "RestoreNICs" -Value $cmd >> "Orchestrate-NICRecovery.ps1"
echo } >> "Orchestrate-NICRecovery.ps1"

echo Writing Admin Launcher...
echo @echo off > "Run-Orchestrator-Admin.bat"
echo ^>nul 2^>^&1 "%%SYSTEMROOT%%\system32\cacls.exe" "%%SYSTEMROOT%%\system32\config\system" >> "Run-Orchestrator-Admin.bat"
echo if '%%errorlevel%%' NEQ '0' ^( >> "Run-Orchestrator-Admin.bat"
echo    echo Set UAC = CreateObject^^^("Shell.Application"^^^) ^> "%%temp%%\getadmin.vbs" >> "Run-Orchestrator-Admin.bat"
echo    echo UAC.ShellExecute "cmd.exe", "/c %%~s0", "", "runas", 1 ^>^> "%%temp%%\getadmin.vbs" >> "Run-Orchestrator-Admin.bat"
echo    "%%temp%%\getadmin.vbs" ^& exit /B >> "Run-Orchestrator-Admin.bat"
echo ^) >> "Run-Orchestrator-Admin.bat"
echo powershell -NoProfile -ExecutionPolicy Bypass -File "C:\NIC-Recovery\Orchestrate-NICRecovery.ps1" >> "Run-Orchestrator-Admin.bat"

echo All files created.
call "Run-Orchestrator-Admin.bat"
exit
`
)
