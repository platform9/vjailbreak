$ErrorActionPreference = "Stop"
$LogFile = "C:\NIC-Recovery\Restore-Network.log"

function Write-Log {
    param(
        [string]$Message,
        [ValidateSet("INFO", "WARNING", "ERROR")]
        [string]$Level = "INFO"
    )

    $timestamp = Get-Date -Format "yyyy-MM-dd HH:mm:ss"
    $logEntry = "$timestamp - $Level - $Message"

    try {
        $logEntry | Out-File -FilePath $LogFile -Append -Encoding utf8
    } catch {
        [Console]::Error.WriteLine("Failed to write to log file: $_")
    }

    switch ($Level) {
        "ERROR" { [Console]::Error.WriteLine($logEntry) }
        default { [Console]::Out.WriteLine($logEntry) }
    }
}

function Normalize-MAC {
    param([string]$mac)
    return ($mac -replace '[:-]', '').ToLower()
}

try {
    Write-Log "=== Starting Network Configuration Restore (MAC-based) ==="

    Write-Log "Waiting 120 seconds for NICs to initialize..."
    Start-Sleep -Seconds 120

    if (-not (Test-Path "C:\NIC-Recovery\netconfig.json")) {
        throw "Missing netconfig.json"
    }

    $configs = Get-Content "C:\NIC-Recovery\netconfig.json" | ConvertFrom-Json
    Write-Log "Found $(($configs | Measure-Object).Count) NIC(s) to configure"

    $allNics = Get-NetAdapter -IncludeHidden

    foreach ($cfg in $configs) {

        $alias = $cfg.InterfaceAlias
        $macTarget = Normalize-MAC $cfg.MACAddress
        $ipAddr = $cfg.IPAddress

        Write-Log "Configuring $alias (MAC: $($cfg.MACAddress), IP: $ipAddr)"

        try {
            # ---- Find NIC via MAC ----
            $nic = $allNics | Where-Object {
                (Normalize-MAC $_.MacAddress) -eq $macTarget
            } | Select-Object -First 1

            if (-not $nic) {
                throw "No adapter found with MAC: $($cfg.MACAddress)"
            }

            Write-Log " - Found adapter: $($nic.Name), Status: $($nic.Status)"

            # ---- Rename (clean assumption: no conflict) ----
            if ($nic.Name -ne $alias) {
                Write-Log " - Renaming '$($nic.Name)' → '$alias'"
                Rename-NetAdapter -Name $nic.Name -NewName $alias -Confirm:$false -ErrorAction Stop
                Start-Sleep -Seconds 2
                $nic = Get-NetAdapter -Name $alias -ErrorAction Stop
            }

            # ---- Remove existing IPs (safe) ----
            $oldIPs = Get-NetIPAddress -InterfaceAlias $nic.Name -AddressFamily IPv4 -ErrorAction SilentlyContinue
            if ($oldIPs) {
                Write-Log " - Removing existing IPv4 addresses"
                $oldIPs | Remove-NetIPAddress -Confirm:$false -ErrorAction SilentlyContinue
            }

            # ---- Remove existing default routes ----
            Get-NetRoute -InterfaceAlias $nic.Name -DestinationPrefix "0.0.0.0/0" -ErrorAction SilentlyContinue |
                Remove-NetRoute -Confirm:$false -ErrorAction SilentlyContinue

            # ---- Configure Static IP ----
            $params = @{
                InterfaceAlias = $nic.Name
                IPAddress      = $ipAddr
                PrefixLength   = $cfg.PrefixLength
            }

            if ($cfg.Gateway) {
                $params.DefaultGateway = $cfg.Gateway
                Write-Log " - Setting IP: $ipAddr/$($cfg.PrefixLength) with GW $($cfg.Gateway)"
            } else {
                Write-Log " - Setting IP: $ipAddr/$($cfg.PrefixLength)"
            }

            New-NetIPAddress @params -ErrorAction Stop

            # ---- Disable DHCP AFTER static config ----
            Write-Log " - Disabling DHCP"
            Set-NetIPInterface -InterfaceAlias $nic.Name -Dhcp Disabled -Confirm:$false -ErrorAction Stop

            # ---- Configure DNS ----
            if ($cfg.DNSServers -and $cfg.DNSServers.Count -gt 0) {
                Write-Log " - Setting DNS: $($cfg.DNSServers -join ', ')"
                Set-DnsClientServerAddress -InterfaceAlias $nic.Name -ServerAddresses $cfg.DNSServers -ErrorAction Stop
            }

            # ---- Verification ----
            $newIP = Get-NetIPAddress -InterfaceAlias $nic.Name -AddressFamily IPv4 -ErrorAction SilentlyContinue

            if ($newIP) {
                Write-Log " - SUCCESS: $alias → $($newIP.IPAddress)/$($newIP.PrefixLength)"
            } else {
                Write-Log " - WARNING: Config applied but verification failed" -Level "WARNING"
            }

        } catch {
            Write-Log "Failed for $alias $_" -Level "ERROR"
            Write-Log "Stack Trace: $($_.ScriptStackTrace)" -Level "ERROR"
            continue
        }
    }

    Write-Log "=== Network Configuration Restore Completed Successfully ==="
    exit 0

} catch {
    Write-Log "Fatal error: $_" -Level "ERROR"
    Write-Log "Stack Trace: $($_.ScriptStackTrace)" -Level "ERROR"
    exit 1
}
