# VMware Tools Manual Removal Script
# This script performs manual cleanup of VMware Tools from Windows machines
# Run as Administrator for full functionality

param(
    [string]$LogPath = "C:\VMware_Removal_Log.txt"
)

# Function to log messages
function Write-Log {
    param([string]$Message, [string]$Level = "INFO")
    $timestamp = Get-Date -Format "yyyy-MM-dd HH:mm:ss"
    $logEntry = "[$timestamp] [$Level] $Message"
    Write-Host $logEntry
    Add-Content -Path $LogPath -Value $logEntry
}

Write-Log "=== VMware Tools Manual Removal Started ==="

# Check if running as Administrator
if (-NOT ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole] "Administrator")) {
    Write-Log "This script must be run as Administrator!" "ERROR"
    exit 1
}

# Get current username for user-specific paths
$currentUser = $env:USERNAME
$allUsers = Get-ChildItem "C:\Users" -Directory | Where-Object { $_.Name -notmatch "^(Public|Default|All Users)$" }

Write-Log "Starting VMware Tools removal process..."

# Step 1: Delete VMware installation files
Write-Log "Step 1: Removing VMware installation files..."

$vmwarePaths = @(
    "C:\Program Files\VMware",
    "C:\Program Files (x86)\VMware",
    "C:\Program Files\Common Files\VMware",
    "C:\Program Files (x86)\Common Files\VMware",
    "C:\ProgramData\VMware"
)

# Add user-specific paths for all users
foreach ($user in $allUsers) {
    $userPath = "C:\Users\$($user.Name)\AppData\Local\VMware"
    $vmwarePaths += $userPath
    $userPath = "C:\Users\$($user.Name)\AppData\Roaming\VMware"
    $vmwarePaths += $userPath
}

foreach ($path in $vmwarePaths) {
    if (Test-Path $path) {
        try {
            Write-Log "Removing directory: $path"
            Remove-Item -Path $path -Recurse -Force -ErrorAction Stop
            Write-Log "Successfully removed: $path"
        }
        catch {
            Write-Log "Failed to remove $path : $($_.Exception.Message)" "WARNING"
        }
    }
    else {
        Write-Log "Path not found (skipping): $path"
    }
}

# Step 2: Remove VMware services from registry
Write-Log "Step 2: Removing VMware services from registry..."

$vmwareServices = @(
    "vmci", "vm3dmp", "vmaudio", "vmhgfs", "VMMemCtl", "vmmouse", 
    "VMRawDisk", "VMTools", "vmusbmouse", "vmvss", "VMwareCAF",
    "VMwareCAFCommAmqpListener", "VMwareCAFManagementAgentHost"
)

$servicesPath = "HKLM:\SYSTEM\CurrentControlSet\Services"

foreach ($service in $vmwareServices) {
    try {
        $servicePath = Join-Path $servicesPath $service
        if (Test-Path $servicePath) {
            Write-Log "Removing service registry entry: $service"
            Remove-Item -Path $servicePath -Recurse -Force
            Write-Log "Successfully removed service: $service"
        }
        else {
            Write-Log "Service registry entry not found: $service"
        }
    }
    catch {
        Write-Log "Failed to remove service $service : $($_.Exception.Message)" "WARNING"
    }
}

# Remove additional VMware service patterns
try {
    $allServices = Get-ChildItem $servicesPath | Where-Object { $_.Name -like "*vmware*" -or $_.Name -like "*VMware*" }
    foreach ($service in $allServices) {
        Write-Log "Removing additional VMware service: $($service.PSChildName)"
        Remove-Item -Path $service.PSPath -Recurse -Force
    }
}
catch {
    Write-Log "Error removing additional services: $($_.Exception.Message)" "WARNING"
}

# Step 3: Remove VMware.Inc from SOFTWARE registry
Write-Log "Step 3: Removing VMware.Inc from SOFTWARE registry..."

$vmwareSoftwareKey = "HKLM:\SOFTWARE\VMware, Inc."
if (Test-Path $vmwareSoftwareKey) {
    try {
        Write-Log "Removing VMware.Inc registry key"
        Remove-Item -Path $vmwareSoftwareKey -Recurse -Force
        Write-Log "Successfully removed VMware.Inc registry key"
    }
    catch {
        Write-Log "Failed to remove VMware.Inc registry key: $($_.Exception.Message)" "WARNING"
    }
}
else {
    Write-Log "VMware.Inc registry key not found"
}

# Also check for other VMware registry entries
$otherVMwareKeys = @(
    "HKLM:\SOFTWARE\VMware",
    "HKLM:\SOFTWARE\WOW6432Node\VMware, Inc.",
    "HKLM:\SOFTWARE\WOW6432Node\VMware"
)

foreach ($key in $otherVMwareKeys) {
    if (Test-Path $key) {
        try {
            Write-Log "Removing additional VMware registry key: $key"
            Remove-Item -Path $key -Recurse -Force
        }
        catch {
            Write-Log "Failed to remove registry key $key : $($_.Exception.Message)" "WARNING"
        }
    }
}

# Step 4: Delete VMware drivers from System32\Drivers
Write-Log "Step 4: Removing VMware drivers from System32\Drivers..."

$driversPath = "C:\Windows\System32\Drivers"
$vmwareDrivers = @(
    "vmci.sys", "vm3dmp.sys", "vmaudio.sys", "vmhgfs.sys", "vmmemctl.sys", 
    "vmmouse.sys", "vmrawdsk.sys", "vmtools.sys", "vmusbmouse.sys", 
    "vmvss.sys", "vsock.sys", "vmx_svga.sys"
)

foreach ($driver in $vmwareDrivers) {
    $driverPath = Join-Path $driversPath $driver
    if (Test-Path $driverPath) {
        try {
            Write-Log "Removing driver: $driver"
            Remove-Item -Path $driverPath -Force
            Write-Log "Successfully removed driver: $driver"
        }
        catch {
            Write-Log "Failed to remove driver $driver : $($_.Exception.Message)" "WARNING"
        }
    }
    else {
        Write-Log "Driver not found: $driver"
    }
}

# Remove any additional VMware drivers
try {
    $additionalDrivers = Get-ChildItem $driversPath | Where-Object { $_.Name -like "*vmware*" -or $_.Name -like "*vm*.sys" }
    foreach ($driver in $additionalDrivers) {
        Write-Log "Removing additional VMware driver: $($driver.Name)"
        Remove-Item -Path $driver.FullName -Force
    }
}
catch {
    Write-Log "Error removing additional drivers: $($_.Exception.Message)" "WARNING"
}

# Step 5: Remove VMware SVGA driver from Device Manager
Write-Log "Step 5: Removing VMware SVGA driver from Device Manager..."

try {
    # Use PnPUtil to remove VMware drivers
    Write-Log "Attempting to remove VMware drivers using PnPUtil..."
    
    # Get all VMware-related drivers
    $pnpDrivers = & pnputil /enum-drivers | Select-String -Pattern "vmware" -Context 2
    
    if ($pnpDrivers) {
        # Extract OEM names and remove drivers
        foreach ($line in $pnpDrivers) {
            if ($line -match "oem(\d+)\.inf") {
                $oemName = $matches[0]
                Write-Log "Removing driver package: $oemName"
                & pnputil /delete-driver $oemName /uninstall /force
            }
        }
    }
    
    # Additional method: Remove devices via PowerShell
    Write-Log "Removing VMware devices from Device Manager..."
    
    $vmwareDevices = Get-PnpDevice | Where-Object { 
        $_.FriendlyName -like "*VMware*" -or 
        $_.HardwareID -like "*VMware*" -or
        $_.InstanceId -like "*VMware*"
    }
    
    foreach ($device in $vmwareDevices) {
        try {
            Write-Log "Removing device: $($device.FriendlyName)"
            $device | Disable-PnpDevice -Confirm:$false
            Remove-PnpDevice -InstanceId $device.InstanceId -Confirm:$false
        }
        catch {
            Write-Log "Could not remove device $($device.FriendlyName): $($_.Exception.Message)" "WARNING"
        }
    }
}
catch {
    Write-Log "Error during device removal: $($_.Exception.Message)" "WARNING"
}

# Step 6: Clean up additional locations
Write-Log "Step 6: Additional cleanup..."

# Remove VMware from startup programs
try {
    $startupLocations = @(
        "HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Run",
        "HKLM:\SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Run",
        "HKCU:\SOFTWARE\Microsoft\Windows\CurrentVersion\Run" #This is because User level startup programs reside only in this location
    )
    
    foreach ($location in $startupLocations) {
        if (Test-Path $location) {
            $entries = Get-ItemProperty -Path $location
            foreach ($property in $entries.PSObject.Properties) {
                if ($property.Value -like "*VMware*" -or $property.Value -like "*vmware*") {
                    Write-Log "Removing startup entry: $($property.Name)"
                    Remove-ItemProperty -Path $location -Name $property.Name -Force
                }
            }
        }
    }
}
catch {
    Write-Log "Error cleaning startup entries: $($_.Exception.Message)" "WARNING"
}

# Remove VMware environment variables
try {
    [System.Environment]::GetEnvironmentVariables([System.EnvironmentVariableTarget]::Machine).Keys | 
    Where-Object { $_ -like "*VMware*" } | 
    ForEach-Object {
        Write-Log "Removing environment variable: $_"
        [System.Environment]::SetEnvironmentVariable($_, $null, [System.EnvironmentVariableTarget]::Machine)
    }
}
catch {
    Write-Log "Error removing environment variables: $($_.Exception.Message)" "WARNING"
}

Write-Log "=== VMware Tools removal process completed ==="
Write-Log "Log file saved to: $LogPath"

# Step 7: Prompt for restart
Write-Log "Step 7: Restart prompt..."
Write-Host "`nVMware Tools removal completed!" -ForegroundColor Green
Write-Host "A system restart is required to complete the removal process." -ForegroundColor Yellow

$restart = Read-Host "`nWould you like to restart now? (Y/N)"
if ($restart -eq "Y" -or $restart -eq "y") {
    Write-Log "User chose to restart immediately"
    Write-Host "Restarting in 10 seconds..." -ForegroundColor Red
    Start-Sleep -Seconds 10
    Restart-Computer -Force
}
else {
    Write-Log "User chose to restart later"
    Write-Host "Please restart your computer manually to complete the VMware Tools removal." -ForegroundColor Yellow
}