# Orchestrate-NICRecovery.ps1
$ScriptRoot = "C:\NIC-Recovery"

Start-Service NetSetupSvc -ErrorAction SilentlyContinue
Start-Sleep -Seconds 5

& "$ScriptRoot\Recover-HiddenNICMapping.ps1"
& "$ScriptRoot\Cleanup-GhostNICs.ps1"

if (Test-Path "$ScriptRoot\netconfig.json") {
    # Directly run Restore-Network.ps1 without reboot
    & "$ScriptRoot\Restore-Network.ps1"
}