# ============================================================
# Recover-HiddenNICMapping.ps1
# Recovers hidden NIC network configuration
# Outputs netconfig.json in script directory
# ============================================================

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$OutFile   = Join-Path $ScriptDir "netconfig.json"

# =========================
# Helper: SubnetMask -> PrefixLength
# =========================
function Convert-SubnetToPrefix {
    param ([string]$Mask)

    ($Mask -split '\.') |
        ForEach-Object { [Convert]::ToString([int]$_,2) } |
        ForEach-Object { $_.ToCharArray() } |
        Where-Object { $_ -eq '1' } |
        Measure-Object |
        Select-Object -ExpandProperty Count
}

# =========================
# Helper: Network Address
# =========================
function Get-Network {
    param (
        [string]$IP,
        [int]$Prefix
    )

    $ipBytes   = ([System.Net.IPAddress]::Parse($IP)).GetAddressBytes()
    $maskBytes = @(0,0,0,0)

    for ($i=0; $i -lt 4; $i++) {
        $bits = [Math]::Min(8, $Prefix - ($i*8))
        if ($bits -gt 0) {
            $maskBytes[$i] = [byte](255 -shl (8-$bits))
        }
    }

    for ($i=0; $i -lt 4; $i++) {
        $ipBytes[$i] = $ipBytes[$i] -band $maskBytes[$i]
    }

    ([System.Net.IPAddress]$ipBytes).ToString()
}

# =========================
# Active NICs (MAC ONLY)
# =========================
$activeNics = Get-NetIPConfiguration |
    Where-Object { $_.IPv4Address } |
    ForEach-Object {
        foreach ($ip in $_.IPv4Address) {
            [PSCustomObject]@{
                InterfaceAlias = $_.InterfaceAlias
                MACAddress     = $_.NetAdapter.MacAddress
                Network        = Get-Network $ip.IPAddress $ip.PrefixLength
            }
        }
    }

# Active aliases (used to EXCLUDE active NIC names)
$activeAliases = Get-NetAdapter |
    Select-Object -ExpandProperty InterfaceAlias

# =========================
# Hidden NIC IP + DNS (Registry)
# =========================
$hiddenIPs = Get-ChildItem "HKLM:\SYSTEM\CurrentControlSet\Services\Tcpip\Parameters\Interfaces" |
ForEach-Object {

    $p = Get-ItemProperty $_.PsPath -ErrorAction SilentlyContinue

    $ip   = ($p.IPAddress | Where-Object { $_ -and $_ -ne '0.0.0.0' } | Select-Object -First 1)
    $mask = ($p.SubnetMask | Select-Object -First 1)

    if (-not $ip -or -not $mask) { return }

    $prefix = Convert-SubnetToPrefix $mask

    # DNS must come ONLY from hidden NIC registry
    $dns =
        @(
            ($p.NameServer -split ','),
            ($p.DhcpNameServer -split ',')
        ) |
        Where-Object { $_ -and $_.Trim() }

    [PSCustomObject]@{
        GUID         = $_.PSChildName.ToUpper()
        IPAddress    = $ip
        PrefixLength = $prefix
        Network      = Get-Network $ip $prefix
        Gateway      = ($p.DefaultGateway | Select-Object -First 1)
        DNSServers   = $dns
    }
}

# =========================
# Hidden Interface Names
# =========================
$hiddenNames = Get-ChildItem "HKLM:\SYSTEM\CurrentControlSet\Control\Network\{4D36E972-E325-11CE-BFC1-08002BE10318}" |
ForEach-Object {

    $conn = Join-Path $_.PsPath "Connection"
    if (-not (Test-Path $conn)) { return }

    $p = Get-ItemProperty $conn -ErrorAction SilentlyContinue

    if ($p.Name -and $p.Name -notin $activeAliases) {
        [PSCustomObject]@{
            GUID = $_.PSChildName.ToUpper()
            Name = $p.Name
        }
    }
}

# =========================
# FINAL Mapping
# =========================
$result = foreach ($hidden in $hiddenIPs) {

    $name = $hiddenNames | Where-Object { $_.GUID -eq $hidden.GUID }
    if (-not $name) { continue }

    $match = $activeNics | Where-Object {
        $_.Network -eq $hidden.Network
    } | Select-Object -First 1

    if (-not $match) { continue }

    [PSCustomObject]@{
        InterfaceAlias = $name.Name
        MACAddress     = $match.MACAddress
        IPAddress      = $hidden.IPAddress
        PrefixLength   = $hidden.PrefixLength
        Gateway        = if ($hidden.Gateway) { $hidden.Gateway } else { $null }
        DNSServers     = @($hidden.DNSServers)
    }
}

# =========================
# Write JSON
# =========================
if (-not $result) {
    Write-Warning "No hidden NICs matched active adapters."
    $result = @()
}

$result |
    ConvertTo-Json -Depth 4 |
    Set-Content -Encoding UTF8 $OutFile

Write-Host "Recovered NIC count:" $result.Count
Write-Host "JSON file created at:" $OutFile
