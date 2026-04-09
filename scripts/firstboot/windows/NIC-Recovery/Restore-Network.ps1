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

function Rename-NICRobust {
    param(
        [string]$CurrentName,
        [string]$TargetName,
        [int]$MaxRetries = 5
    )

    for ($i = 1; $i -le $MaxRetries; $i++) {
        try {
            Write-Log "Attempt [$i/$MaxRetries]: Renaming '$CurrentName' → '$TargetName'"

            Rename-NetAdapter -Name $CurrentName -NewName $TargetName -Confirm:$false -ErrorAction Stop
            Start-Sleep -Seconds 2

            $renamed = Get-NetAdapter -Name $TargetName -ErrorAction SilentlyContinue
            if ($renamed) {
                Write-Log "Rename successful: '$TargetName'"
                return $renamed
            } else {
                throw "Rename command succeeded but adapter not found"
            }
        }
        catch {
            $err = $_.Exception.Message
            Write-Log "Rename attempt failed: $err" -Level "WARNING"

            if ($err -match "exists|in use|duplicate") {

                Write-Log "Conflict detected for '$TargetName'"

                $conflict = Get-NetAdapter -Name $TargetName -ErrorAction SilentlyContinue

                if ($conflict) {
                    $tempName = "conflict_" + [guid]::NewGuid().ToString().Substring(0,6)
                    Write-Log "Renaming conflicting adapter '$TargetName' → '$tempName'"

                    try {
                        Rename-NetAdapter -Name $TargetName -NewName $tempName -Confirm:$false -ErrorAction Stop
                        Start-Sleep -Seconds 2
                    } catch {
                        Write-Log "Failed to rename conflicting adapter: $($_.Exception.Message)" -Level "WARNING"
                    }
                }
                else {
                    Write-Log "Ghost NIC likely holding name '$TargetName'" -Level "WARNING"

                    # Fallback: break binding
                    $tempSelf = "temp_" + [guid]::NewGuid().ToString().Substring(0,6)
                    Write-Log "Renaming current adapter '$CurrentName' → '$tempSelf'"

                    try {
                        Rename-NetAdapter -Name $CurrentName -NewName $tempSelf -Confirm:$false -ErrorAction Stop
                        Start-Sleep -Seconds 2
                        $CurrentName = $tempSelf
                    } catch {
                        Write-Log "Fallback rename failed: $($_.Exception.Message)" -Level "WARNING"
                    }
                }
            }

            Start-Sleep -Seconds (2 * $i)

            # Refresh adapter reference
            $refreshed = Get-NetAdapter -IncludeHidden -ErrorAction SilentlyContinue |
                         Where-Object { $_.Name -eq $CurrentName } |
                         Select-Object -First 1

            if ($refreshed) {
                $CurrentName = $refreshed.Name
                Write-Log "Adapter refreshed: '$CurrentName'"
            } else {
                Write-Log "Adapter not found during refresh" -Level "WARNING"
            }
        }
    }

    throw "Failed to rename '$CurrentName' → '$TargetName' after retries"
}

try {
    Write-Log "=== Starting Network Configuration Restore ==="
    Write-Log "Waiting 200 seconds for network interfaces to initialize..."
    Start-Sleep -Seconds 200

    Write-Log "Stabilizing NIC state..."
    Start-Sleep -Seconds 10

    if (-not (Test-Path "C:\NIC-Recovery\netconfig.json")) {
        throw "Missing netconfig.json"
    }

    $configs = Get-Content "C:\NIC-Recovery\netconfig.json" | ConvertFrom-Json
    Write-Log "Found $(($configs | Measure-Object).Count) NIC(s) to configure"

    foreach ($cfg in $configs) {

        $alias = $cfg.InterfaceAlias
        $ipAddr = $cfg.IPAddress

        Write-Log "Configuring $alias (IP: $ipAddr)"

        try {
            # IP-based detection (kept as requested)
            $nic = Get-NetIPAddress -IPAddress $ipAddr -ErrorAction SilentlyContinue |
                   Get-NetAdapter -ErrorAction SilentlyContinue |
                   Select-Object -First 1

            if (-not $nic) {
                Write-Log "No NIC found with IP $ipAddr, retrying after delay..." -Level "WARNING"
                Start-Sleep -Seconds 5

                # Retry once (important!)
                $nic = Get-NetIPAddress -IPAddress $ipAddr -ErrorAction SilentlyContinue |
                       Get-NetAdapter -ErrorAction SilentlyContinue |
                       Select-Object -First 1
            }

            if (-not $nic) {
                throw "No network adapter found with IP: $ipAddr"
            }

            Write-Log " - Found adapter: $($nic.Name), Status: $($nic.Status), Speed: $($nic.LinkSpeed)"

            # Rename if needed
            if ($nic.Name -ne $alias) {
                Write-Log " - Renaming '$($nic.Name)' → '$alias'"
                $nic = Rename-NICRobust -CurrentName $nic.Name -TargetName $alias
            }

            Write-Log " - Final adapter name: $($nic.Name)"

        } catch {
            Write-Log "Failed to configure $alias (IP: $ipAddr): $_" -Level "ERROR"
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
