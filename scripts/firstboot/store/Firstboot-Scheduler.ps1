# Firstboot-Scheduler.ps1
$ScriptRoot = "C:\firstboot"
$LogFile = Join-Path $ScriptRoot "Firstboot-Scheduler.log"
$TaskName = "FirstbootSchedulerPostReboot"
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
function Schedule-MyTask {
    param(
        [string]$TaskName,
        [string]$ScriptPath,
        [string]$Description
    )
   
    $taskName    = $TaskName
    $scriptPath  = $ScriptPath
    $description = $Description

    if (Get-ScheduledTask -TaskName $taskName -ErrorAction SilentlyContinue) {
        Unregister-ScheduledTask -TaskName $taskName -Confirm:$false
        Start-Sleep -Seconds 2   # give it time to release locks
    }

    # Action: run your .ps1 file (bypass execution policy just in case)
    $action = New-ScheduledTaskAction `
        -Execute "powershell.exe" `
        -Argument "-NoProfile -ExecutionPolicy Bypass -File `"$scriptPath`""

    # Trigger: at startup (next boot), only once
    $trigger = New-ScheduledTaskTrigger -AtStartup

    # ─── Changed: use SYSTEM instead of Administrators group ───
    $principal = New-ScheduledTaskPrincipal `
        -UserId    "NT AUTHORITY\SYSTEM" `
        -LogonType ServiceAccount `
        -RunLevel  Highest

    # Settings: added StartWhenAvailable + explicit power settings
    $settings = New-ScheduledTaskSettingsSet `
        -AllowStartIfOnBatteries    `
        -DontStopIfGoingOnBatteries `
        -ExecutionTimeLimit          (New-TimeSpan -Minutes 30)

    # Create the task
    Register-ScheduledTask -TaskName $taskName `
        -Action      $action `
        -Trigger     $trigger `
        -Principal   $principal `
        -Settings    $settings `
        -Description $description `
        -Force

    Write-Log "Task '$taskName' created → will run your script once after next reboot."
}
function Ensure-64BitPowerShell {
    if (-not [Environment]::Is64BitOperatingSystem) {
        Write-Verbose "This is a 32-bit operating system → no 64-bit PowerShell available. Continuing as-is."
        return
    }

    if ([Environment]::Is64BitProcess) {
        Write-Verbose "Already running in 64-bit PowerShell process."
        return
    }

    Write-Log "Detected 32-bit PowerShell on 64-bit OS → Relaunching in 64-bit PowerShell..." -Level "WARNING"

    $ps64Path = "$env:windir\sysnative\WindowsPowerShell\v1.0\powershell.exe"

    if (-not (Test-Path $ps64Path)) {
        Write-Warning "64-bit PowerShell not found at $ps64Path. Continuing in 32-bit (may cause issues)."
        return
    }

    $argList = @(
        '-NoProfile',
        '-ExecutionPolicy', 'Bypass'
    )

    if ($args.Count -gt 0) {
        $argList += $args
    }

    $argList += '-File'
    $argList += $PSCommandPath   # or $MyInvocation.MyCommand.Definition

    try {
        Start-Process -FilePath $ps64Path `
                      -ArgumentList $argList `
                      -Wait `
                      -NoNewWindow

        Write-Log "64-bit execution completed. Exiting 32-bit instance."
        exit 0
    }
    catch {
        Write-Log "Failed to relaunch in 64-bit PowerShell: $_" -Level "ERROR"
    }
}

function Remove-MyTask{
    param(
        [string]$TaskName
    )
    
    $taskName = $TaskName
    
    try {
        Unregister-ScheduledTask -TaskName $taskName -Confirm:$false
        Write-Log "Task '$taskName' removed successfully."
    } catch {
        Write-Log "Failed to remove task '$taskName': $_" -Level "ERROR"
    }
}

try {
    Write-Log "=== Starting Firstboot Scheduler ==="
    Write-Log "Script Root: $ScriptRoot"
    Write-Log "Log File: $LogFile"
    Write-Log "PowerShell Version: $($PSVersionTable.PSVersion)"
    Write-Log "OS: $([Environment]::OSVersion.VersionString)"
    Write-Log "64-bit Process: $([Environment]::Is64BitProcess)"
    Write-Log "64-bit OS: $([Environment]::Is64BitOperatingSystem)"
    
    # Ensure 64-bit PowerShell
    Ensure-64BitPowerShell
    
    # Read and parse scripts.json as PowerShell object
    $scriptsJsonPath = Join-Path $ScriptRoot "scripts.json"
    
    if (Test-Path $scriptsJsonPath) {
        Write-Log "Found scripts.json at: $scriptsJsonPath"
        
        try {
            $scriptsContent = Get-Content -Path $scriptsJsonPath -Raw -ErrorAction Stop
            $scriptsArray = $scriptsContent | ConvertFrom-Json -ErrorAction Stop
            
            Write-Log "Successfully parsed scripts.json, found $(($scriptsArray | Measure-Object).Count) script(s)"
            
            # Loop over the array and log each script content
            foreach ($script in $scriptsArray) {
                Write-Log "Script content: $script"
            }
            
        } catch {
            Write-Log "Failed to parse scripts.json: $_" -Level "ERROR"
        }
    } else {
        Write-Log "scripts.json not found at: $scriptsJsonPath" -Level "WARNING"
    }
    
    
    Write-Log "=== Firstboot Scheduler completed successfully ==="
    exit 0
    
} catch {
    Write-Log "FATAL ERROR: $_" -Level "ERROR"
    Write-Log "Stack Trace: $($_.ScriptStackTrace)" -Level "ERROR"
    exit 1
}