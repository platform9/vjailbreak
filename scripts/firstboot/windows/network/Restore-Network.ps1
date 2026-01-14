$ErrorActionPreference = "Stop"

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$configs   = Get-Content (Join-Path $ScriptDir "netconfig.json") | ConvertFrom-Json

foreach ($cfg in $configs) {

    Write-Host "Configuring $($cfg.InterfaceAlias)" -ForegroundColor Cyan

    $nic = Get-NetAdapter | Where-Object {
        ($_.MacAddress -replace '[-:]','') -eq ($cfg.MACAddress -replace '[-:]','')
    }

    if (-not $nic) {
        Write-Warning "NIC not found for MAC $($cfg.MACAddress)"
        continue
    }

    Set-NetIPInterface -InterfaceIndex $nic.ifIndex -Dhcp Disabled -Confirm:$false

    Get-NetIPAddress -InterfaceIndex $nic.ifIndex -AddressFamily IPv4 `
        -ErrorAction SilentlyContinue |
        Remove-NetIPAddress -Confirm:$false -ErrorAction SilentlyContinue

    Get-NetRoute -InterfaceIndex $nic.ifIndex -DestinationPrefix "0.0.0.0/0" `
        -ErrorAction SilentlyContinue |
        Remove-NetRoute -Confirm:$false -ErrorAction SilentlyContinue

    if ($nic.Name -ne $cfg.InterfaceAlias) {
        Rename-NetAdapter -Name $nic.Name -NewName $cfg.InterfaceAlias -Confirm:$false
        $nic = Get-NetAdapter -Name $cfg.InterfaceAlias
    }

    $params = @{
        InterfaceIndex = $nic.ifIndex
        IPAddress      = $cfg.IPAddress
        PrefixLength   = $cfg.PrefixLength
        AddressFamily  = "IPv4"
    }

    if ($cfg.Gateway) {
        $params.DefaultGateway = $cfg.Gateway
    }

    New-NetIPAddress @params

    if ($cfg.DNSServers -and $cfg.DNSServers.Count -gt 0) {
        Set-DnsClientServerAddress `
            -InterfaceIndex $nic.ifIndex `
            -ServerAddresses $cfg.DNSServers
    }
}

Write-Host "Network restore completed successfully" -ForegroundColor Green

$TaskName = "NIC-Network-Restore"

Write-Host "Removing scheduled task..."
Unregister-ScheduledTask -TaskName $TaskName -Confirm:$false -ErrorAction SilentlyContinue

Write-Host "Cleanup completed. System is stable." -ForegroundColor Green
