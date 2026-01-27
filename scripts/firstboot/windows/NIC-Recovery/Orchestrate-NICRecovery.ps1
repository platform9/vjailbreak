# Orchestrate-NICRecovery.ps1
$ScriptRoot = "C:\NIC-Recovery"
$LogFile = Join-Path $ScriptRoot "NIC-Recovery.log"

function Write-Log {
    param([string]$Message, [string]$Level = "INFO")
    $timestamp = Get-Date -Format "yyyy-MM-dd HH:mm:ss"
    $logEntry = "$timestamp - $Level - $Message"
    
    try {
        if (-not (Test-Path $ScriptRoot)) {
            New-Item -ItemType Directory -Path $ScriptRoot -Force | Out-Null
        }
        $logEntry | Out-File -FilePath $LogFile -Append -Encoding utf8
    } catch {
        Write-Host "Failed to write to log file: $_"
    }
    
    if ($Level -eq "ERROR") {
        Write-Host $logEntry -ForegroundColor Red
    } else {
        Write-Host $logEntry
    }
}

try {
    Write-Log "=== Starting NIC Recovery Orchestration ==="
    
    # Start Network Setup Service
    try {
        Write-Log "Starting Network Setup Service..."
        Start-Service NetSetupSvc -ErrorAction Stop
        Write-Log "Network Setup Service started successfully"
    } catch {
        Write-Log "Warning: Failed to start Network Setup Service - $_" -Level "WARNING"
    }
    
    Start-Sleep -Seconds 5

    # Run Recover-HiddenNICMapping.ps1
    try {
        Write-Log "Running Recover-HiddenNICMapping.ps1..."
        & "$ScriptRoot\Recover-HiddenNICMapping.ps1" *>> $LogFile
        if ($LASTEXITCODE -ne 0) { throw "Script failed with exit code $LASTEXITCODE" }
        Write-Log "Recover-HiddenNICMapping.ps1 completed successfully"
    } catch {
        Write-Log "Error in Recover-HiddenNICMapping.ps1 - $_" -Level "ERROR"
        throw $_
    }

    # Run Cleanup-GhostNICs.ps1
    try {
        Write-Log "Running Cleanup-GhostNICs.ps1..."
        & "$ScriptRoot\Cleanup-GhostNICs.ps1" *>> $LogFile
        if ($LASTEXITCODE -ne 0) { throw "Script failed with exit code $LASTEXITCODE" }
        Write-Log "Cleanup-GhostNICs.ps1 completed successfully"
    } catch {
        Write-Log "Error in Cleanup-GhostNICs.ps1 - $_" -Level "ERROR"
        throw $_
    }

    # Check for network configuration and restore if needed
    if (Test-Path "$ScriptRoot\netconfig.json") {
        try {
            Write-Log "Network configuration found, running Restore-Network.ps1..."
            & "$ScriptRoot\Restore-Network.ps1" *>> $LogFile
            if ($LASTEXITCODE -ne 0) { throw "Script failed with exit code $LASTEXITCODE" }
            Write-Log "Network configuration restored successfully"
        } catch {
            Write-Log "Error restoring network configuration - $_" -Level "ERROR"
            throw $_
        }
    } else {
        Write-Log "No network configuration found (netconfig.json not present)"
    }

    Write-Log "=== NIC Recovery Orchestration completed successfully ==="
    exit 0
} catch {
    Write-Log "FATAL ERROR: $_" -Level "ERROR"
    Write-Log "Stack Trace: $($_.ScriptStackTrace)" -Level "ERROR"
    exit 1
}