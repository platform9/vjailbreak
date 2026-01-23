$ErrorActionPreference = "Stop"

Start-Sleep -Seconds 15

$configs = Get-Content "C:\NIC-Recovery\netconfig.json" | ConvertFrom-Json

foreach ($cfg in $configs) {
    $alias = $cfg.InterfaceAlias
    $mac   = $cfg.MACAddress
    $ipAddr = $cfg.IPAddress
    Write-Host "Configuring $alias (IP: $ipAddr)" -ForegroundColor Cyan

    # Normalize MAC for comparison (remove : and -)
    $macClean = $mac -replace '[:-]',''

    $nic = Get-NetIPAddress -IPAddress $ipAddr | Get-NetAdapter

    if (-not $nic) {
        Write-Warning "No adapter found with IP: $ipAddr"
        continue
    }

    # In case multiple nics match (very rare), take first
    $nic = $nic | Select-Object -First 1

    try {
        Write-Host " - Current name: $($nic.Name)" -NoNewline

        # Disable DHCP
        Set-NetIPInterface -InterfaceAlias $nic.Name -Dhcp Disabled -Confirm:$false -ErrorAction Stop

        # Remove existing IPv4 addresses
        Get-NetIPAddress -InterfaceAlias $nic.Name -AddressFamily IPv4 -ErrorAction SilentlyContinue |
           Remove-NetIPAddress -Confirm:$false -ErrorAction SilentlyContinue

        if ($nic.Name -ne $alias) {
            Write-Host " -> Renaming to: $alias" -NoNewline
            Rename-NetAdapter -Name $nic.Name -NewName $alias -ErrorAction Stop -Confirm:$false

            # Refresh object after rename
            $nic = Get-NetAdapter -Name $alias -ErrorAction Stop
            Write-Host " [OK]" -ForegroundColor Green
        }
        else {
            Write-Host " [No rename needed]" -ForegroundColor Gray
        }

        # Prepare static IP parameters
       $params = @{
           InterfaceAlias = $nic.Name
           IPAddress      = $ipAddr
           PrefixLength   = $cfg.PrefixLength
       }

        if ($cfg.Gateway) {
           $params.DefaultGateway = $cfg.Gateway
       }

        # In Windows PowerShell → use New-NetIPAddress (no -DefaultGateway in some versions → separate route if needed)
        New-NetIPAddress @params -ErrorAction Stop

        # DNS
        if ($cfg.DNSServers -and $cfg.DNSServers.Count -gt 0) {
           Set-DnsClientServerAddress -InterfaceAlias $nic.Name -ServerAddresses $cfg.DNSServers -ErrorAction Stop
       }

        Write-Host " - Successfully configured $alias" -ForegroundColor Green
    }catch {
        Write-Error " - Failed to configure $alias : $_"
    }
}
