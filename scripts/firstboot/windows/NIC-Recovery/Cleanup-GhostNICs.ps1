# Cleanup-GhostNICs.ps1
param(
    [switch]$WhatIf,
    [string]$LogFile = "C:\NIC-Recovery\Cleanup-GhostNICs.log"
)

function Write-Log {
    param([string]$Message)
    $timestamp = Get-Date -Format "yyyy-MM-dd HH:mm:ss"
    try {
        # Ensure the log directory exists
        $logDir = Split-Path -Path $LogFile -Parent
        if (-not (Test-Path $logDir)) {
            New-Item -ItemType Directory -Path $logDir -Force | Out-Null
        }
        "$timestamp - $Message" | Out-File -FilePath $LogFile -Append -Encoding utf8
    } catch {
        Write-Host "Failed to write to log file: $_"
    }
    Write-Host "$timestamp - $Message"
}

# Check for admin rights
if (-NOT ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole] "Administrator")) {
    $errorMsg = "This script requires Administrator privileges. Please run as Administrator."
    Write-Log $errorMsg
    throw $errorMsg
}

Write-Log "=== Starting Ghost Network Adapters Cleanup ==="

try {
    $ghosts = Get-PnpDevice -Class Net -ErrorAction Stop | Where-Object { $_.Status -eq "Unknown" -and $_.InstanceId }
    
    if (-not $ghosts) { 
        Write-Log "No ghost NICs found."
        return 
    }

    foreach ($dev in $ghosts) {
        $fName = if ($dev.FriendlyName) { $dev.FriendlyName } else { "Unknown" }
        $iId = $dev.InstanceId
        
        if (-not $iId) {
            Write-Log "Skipping device with empty InstanceID"
            continue
        }

        Write-Log "Processing ghost NIC: $fName (InstanceID: $iId)"
        
        # Clean up device registry entries
        $path = "HKLM:\SYSTEM\CurrentControlSet\Enum\$iId"
        if (Test-Path $path) {
            try {
                if ($WhatIf) {
                    Write-Log "[WhatIf] Would remove registry key: $path"
                } else {
                    # First try to remove properties
                    $props = Get-ItemProperty -Path $path -ErrorAction Stop | 
                            Select-Object -ExpandProperty Property -ErrorAction SilentlyContinue
                    
                    if ($props) {
                        foreach ($prop in $props) {
                            try {
                                Remove-ItemProperty -Path $path -Name $prop -Force -ErrorAction Stop
                                Write-Log "Removed property: $prop"
                            } catch {
                                Write-Log "Warning: Failed to remove property $prop - $_"
                            }
                        }
                    }
                    
                    # Then remove the key
                    Remove-Item -Path $path -Recurse -Force -ErrorAction Stop
                    Write-Log "Removed registry key: $path"
                }
            } catch {
                Write-Log "Warning: Error processing $path - $_"
            }
        } else {
            Write-Log "Registry path not found: $path"
        }
        
        # Additional cleanup for network configurations
        $netConfigPath = "HKLM:\SYSTEM\CurrentControlSet\Control\Network\{4D36E972-E325-11CE-BFC1-08002BE10318}\$iId"
        if (Test-Path $netConfigPath) {
            try {
                if ($WhatIf) {
                    Write-Log "[WhatIf] Would remove network configuration: $netConfigPath"
                } else {
                    Remove-Item -Path $netConfigPath -Recurse -Force -ErrorAction Stop
                    Write-Log "Removed network configuration: $netConfigPath"
                }
            } catch {
                Write-Log "Warning: Failed to remove network configuration $netConfigPath - $_"
            }
        } else {
            Write-Log "Network configuration path not found: $netConfigPath"
        }
    }
    
    Write-Log "Ghost NIC cleanup completed successfully."
} catch {
    $errorMsg = "Error during ghost NIC cleanup: $_"
    Write-Log $errorMsg
    throw $errorMsg
}