$ScriptRoot = "C:\NIC-Recovery"
$TaskName   = "NIC-Network-Restore"

Write-Host "=== NIC Recovery Automation Started ===" -ForegroundColor Cyan

# -------------------------
# Phase 1
# -------------------------
Write-Host "Running NIC discovery..."
& "$ScriptRoot\Recover-HiddenNICMapping.ps1"

Write-Host "Cleaning ghost NICs..."
& "$ScriptRoot\Cleanup-GhostNICs.ps1"

# -------------------------
# Create Post-Reboot Task
# -------------------------
Write-Host "Registering post-reboot restore task..."

$action = New-ScheduledTaskAction `
    -Execute "powershell.exe" `
    -Argument "-ExecutionPolicy Bypass -File `"$ScriptRoot\Restore-Network.ps1`""

$trigger = New-ScheduledTaskTrigger -AtStartup

$principal = New-ScheduledTaskPrincipal `
    -UserId "SYSTEM" `
    -RunLevel Highest

Register-ScheduledTask `
    -TaskName $TaskName `
    -Action $action `
    -Trigger $trigger `
    -Principal $principal `
    -Force

# -------------------------
# Reboot
# -------------------------
Write-Host "Rebooting system to apply changes..." -ForegroundColor Yellow
Restart-Computer -Force
