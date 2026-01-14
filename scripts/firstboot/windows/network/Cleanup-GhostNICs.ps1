Write-Host "=== Removing Ghost Network Adapters ===" -ForegroundColor Cyan

$ghosts = Get-PnpDevice -Class Net | Where-Object Status -eq "Unknown"

if (-not $ghosts) {
    Write-Host "No ghost NICs found."
    return
}

foreach ($dev in $ghosts) {
    Write-Host "Removing $($dev.FriendlyName)"
    $path = "HKLM:\SYSTEM\CurrentControlSet\Enum\$($dev.InstanceId)"

    if (Test-Path $path) {
        Get-Item $path |
        Select-Object -ExpandProperty Property |
        ForEach-Object {
            Remove-ItemProperty -Path $path -Name $_ -Force -ErrorAction SilentlyContinue
        }
    }
}

Write-Host " Ghost NIC cleanup completed reboot required" -ForegroundColor Yellow
