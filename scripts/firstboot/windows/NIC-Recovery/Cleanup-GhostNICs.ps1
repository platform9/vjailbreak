# Cleanup-GhostNICs.ps1
Write-Host "=== Removing Ghost Network Adapters ===" -ForegroundColor Cyan

$ghosts = Get-PnpDevice -Class Net | Where-Object Status -eq "Unknown"
if (-not $ghosts) { 
    Write-Host "No ghost NICs found."
    return 
}

foreach ($dev in $ghosts) {
    $fName = $dev.FriendlyName
    $iId = $dev.InstanceId
    Write-Host "Removing $fName"
    $path = "HKLM:\SYSTEM\CurrentControlSet\Enum\$iId"
    if (Test-Path $path) { 
        Get-Item $path | 
        Select-Object -ExpandProperty Property | 
        ForEach-Object { 
            Remove-ItemProperty -Path $path -Name $_ -Force -ErrorAction SilentlyContinue 
        }
    }
}