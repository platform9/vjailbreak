# Recover-HiddenNICMapping.ps1
param(
    [string]$OutFile = "C:\NIC-Recovery\netconfig.json",
    [string]$LogFile = "C:\NIC-Recovery\Recover-HiddenNICMapping.log"
)

function Write-Log {
    param([string]$Message)
    $timestamp = Get-Date -Format "yyyy-MM-dd HH:mm:ss"
    try {
        $logDir = Split-Path -Path $LogFile -Parent
        if (-not (Test-Path $logDir)) { New-Item -ItemType Directory -Path $logDir -Force | Out-Null }
        "$timestamp - $Message" | Out-File -FilePath $LogFile -Append -Encoding utf8
    } catch {
        Write-Host "Failed to write log: $_"
    }
    Write-Host "$timestamp - $Message"
}

Write-Log "=== Starting Recover-HiddenNICMapping ==="
Write-Log "PowerShell version: $($PSVersionTable.PSVersion)"
Write-Log "OS: $([System.Environment]::OSVersion.VersionString)"

function Convert-SubnetToPrefix {
    param ([string]$Mask)
    ($Mask -split '\.') | ForEach-Object { [Convert]::ToString([int]$_,2) } | 
    ForEach-Object { $_.ToCharArray() } | 
    Where-Object { $_ -eq '1' } | 
    Measure-Object | 
    Select-Object -ExpandProperty Count
}

function Get-Network {
    param ([string]$IP, [int]$Prefix)
    $ipBytes = ([System.Net.IPAddress]::Parse($IP)).GetAddressBytes()
    $maskBytes = @(0,0,0,0)
    if ($prefix -lt 0 -or $prefix -gt 32) { 
        Write-Warning "Invalid prefix length $Prefix for $IP"
    }else{

    for ($i=0; $i -lt 4; $i++) {
        $bits = [Math]::Min(8, $Prefix - ($i*8))
        if ($bits -gt 0) { 
            $maskBytes[$i] = [byte](0xFF -shr (8 - $bits))
        }
    }
    for ($i=0; $i -lt 4; $i++) { 
        $ipBytes[$i] = $ipBytes[$i] -band $maskBytes[$i] 
    }
    }
    ([System.Net.IPAddress]$ipBytes).ToString()
}

Write-Log "Retrieving active NIC configurations..."
$activeNics = try { 
    Get-NetIPConfiguration -ErrorAction Stop | 
    Where-Object { $_.IPv4Address } | 
    ForEach-Object { 
        foreach ($ip in $_.IPv4Address) { 
            [PSCustomObject]@{ 
                InterfaceAlias = $_.InterfaceAlias
                MACAddress = $_.NetAdapter.MacAddress
                Network = Get-Network $ip.IPAddress $ip.PrefixLength
            } 
        } 
    } 
} catch { 
    Write-Log "Warning: Could not retrieve active NIC configurations: $($_.Exception.Message)"
    $null 
}
Write-Log "Found $($activeNics.Count) active NICs with IP configuration"

Write-Log "Retrieving active adapter aliases..."
$activeAliases = try { 
    Get-NetAdapter -ErrorAction SilentlyContinue | 
    Select-Object -ExpandProperty InterfaceAlias 
} catch { 
    Write-Log "Warning: Could not retrieve active adapter aliases: $($_.Exception.Message)"
    @() 
}
Write-Log "Found $($activeAliases.Count) active adapter aliases"

Write-Log "Reading hidden IP configurations from registry..."
$hiddenIPs = Get-ChildItem "HKLM:\SYSTEM\CurrentControlSet\Services\Tcpip\Parameters\Interfaces" -ErrorAction SilentlyContinue | 
    ForEach-Object { 
        $p = Get-ItemProperty $_.PsPath -ErrorAction SilentlyContinue
        $ip = ($p.IPAddress | Where-Object { $_ -and $_ -ne '0.0.0.0' } | Select-Object -First 1)
        $mask = ($p.SubnetMask | Select-Object -First 1)
        if (-not $ip -or -not $mask) { return }
        
        $prefix = Convert-SubnetToPrefix $mask
        $dns = @(($p.NameServer -split ','), ($p.DhcpNameServer -split ',')) | 
               Where-Object { $_ -and $_.Trim() }
        
        Write-Log "Found hidden IP config: GUID=$($_.PSChildName.ToUpper()), IP=$ip, Prefix=$prefix"
        
        [PSCustomObject]@{
            GUID = $_.PSChildName.ToUpper()
            IPAddress = $ip
            PrefixLength = $prefix
            Network = Get-Network $ip $prefix
            Gateway = ($p.DefaultGateway | Select-Object -First 1)
            DNSServers = $dns
        }
    }
Write-Log "Found $($hiddenIPs.Count) hidden IP configurations"

Write-Log "Reading hidden adapter names from registry..."
$hiddenNames = Get-ChildItem "HKLM:\SYSTEM\CurrentControlSet\Control\Network\{4D36E972-E325-11CE-BFC1-08002BE10318}" -ErrorAction SilentlyContinue | 
    ForEach-Object { 
        $conn = Join-Path $_.PsPath "Connection"
        if (-not (Test-Path $conn)) { return }
        $p = Get-ItemProperty $conn -ErrorAction SilentlyContinue
        if ($p.Name -and $p.Name -notin $activeAliases) { 
            Write-Log "Found hidden adapter name: GUID=$($_.PSChildName.ToUpper()), Name=$($p.Name)"
            [PSCustomObject]@{
                GUID = $_.PSChildName.ToUpper()
                Name = $p.Name
            }
        }
    }
Write-Log "Found $($hiddenNames.Count) hidden adapter names"

Write-Log "Matching hidden IPs with hidden names and active NICs..."
$result = foreach ($hidden in $hiddenIPs) {
    $name = $hiddenNames | Where-Object { $_.GUID -eq $hidden.GUID }
    if (-not $name) { 
        Write-Log "No matching name found for GUID $($hidden.GUID)"
        continue 
    }
    
    $match = $activeNics | 
             Where-Object { $_.Network -eq $hidden.Network } | 
             Select-Object -First 1
             
    if (-not $match) { 
        Write-Log "No matching active NIC found for network $($hidden.Network) (GUID: $($hidden.GUID))"
        continue 
    }
    
    Write-Log "Matched: $($name.Name) -> $($match.MACAddress) (Network: $($hidden.Network))"
    
    [PSCustomObject]@{
        InterfaceAlias = $name.Name
        MACAddress = $match.MACAddress
        IPAddress = $hidden.IPAddress
        PrefixLength = $hidden.PrefixLength
        Gateway = if ($hidden.Gateway) { $hidden.Gateway } else { $null }
        DNSServers = @($hidden.DNSServers)
    }
}

if (-not $result) { 
    Write-Log "No matching configurations found - output will be empty array"
    $result = @() 
} else {
    Write-Log "Found $($result.Count) matched configurations"
}

try {
    $result | ConvertTo-Json -Depth 4 | Set-Content -Encoding UTF8 $OutFile
    Write-Log "Successfully wrote configuration to $OutFile"
} catch {
    Write-Log "ERROR: Failed to write output file: $($_.Exception.Message)"
    exit 1
}

Write-Log "=== Recover-HiddenNICMapping finished successfully ==="
exit 0