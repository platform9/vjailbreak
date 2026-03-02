# Enhanced-Cleanup-v3.ps1
# Improved for Windows Server 2016 compatibility
# Goal: Remove ghost NICs + free old names so Rename-NetAdapter works later

param(
    [switch]$WhatIf,
    [string]$LogFile = "C:\NIC-Recovery\Enhanced-Cleanup-v3.log",
    [string]$DevconPath = "C:\NIC-Recovery\devcon.exe"   # <-- put devcon.exe here if you have it
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

# Must be admin
if (-not ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    throw "Script requires Administrator rights."
}

Write-Log "=== Starting Enhanced Network Cleanup v3 ==="
Write-Log "PowerShell version: $($PSVersionTable.PSVersion)"
Write-Log "OS: $([System.Environment]::OSVersion.VersionString)"

try {
    # 0. Check if devcon is available (optional but very effective)
    $useDevcon = (Test-Path $DevconPath -PathType Leaf)
    if ($useDevcon) {
        Write-Log "devcon.exe found at $DevconPath → will use for removal"
    } else {
        Write-Log "devcon.exe not found → removal will be limited"
    }

    # 1. Rename conflicting visible adapters
    Write-Log "Renaming any existing conflicting interface names (vjb*, Ethernet, etc.)..."
    $interfaces = Get-NetAdapter -ErrorAction SilentlyContinue |
                  Where-Object { $_.Name -match '(?i)^vjb|Ethernet|Local Area Connection|Wi-Fi' }

    foreach ($nic in $interfaces) {
        $newName = "vjb_" + [guid]::NewGuid().ToString().Substring(0,8)
        Write-Log "Renaming '$($nic.Name)' → '$newName' to avoid name conflicts"
        if (-not $WhatIf) {
            try {
                Rename-NetAdapter -Name $nic.Name -NewName $newName -ErrorAction Stop
                Write-Log "  → Success"
            } catch {
                Write-Log "  → Failed: $($_.Exception.Message)"
            }
        } else {
            Write-Log "  [WhatIf] Would rename '$($nic.Name)' to '$newName'"
        }
    }

    # 2. Detect and handle ghost / non-present network devices
    Write-Log "Removing ghost/non-present network devices..."
    $ghosts = Get-PnpDevice -Class Net -ErrorAction SilentlyContinue |
              Where-Object { $_.Status -notin @("OK", "Degraded") }

    if (-not $ghosts) {
        Write-Log "No ghost NICs found via Get-PnpDevice."
    } else {
        # Check if pnputil supports /remove-device
        $pnputilSupportsRemove = $false
        try {
            $help = & pnputil /remove-device /? 2>&1
            if ($help -match "remove-device") { $pnputilSupportsRemove = $true }
        } catch { }

        Write-Log "pnputil /remove-device supported? $pnputilSupportsRemove"

        foreach ($dev in $ghosts) {
            $fname = if ($dev.FriendlyName) { $dev.FriendlyName } else { "Unknown Device" }
            $iid   = $dev.InstanceId
            Write-Log "Found ghost: $fname ($iid)  [Status: $($dev.Status)]"

            if (-not $WhatIf) {
                try {
                    # Step A: Disable (almost always works)
                    Disable-PnpDevice -InstanceId $iid -Confirm:$false -ErrorAction SilentlyContinue
                    Write-Log "  → Disabled via Disable-PnpDevice"

                    # Step B: Try real removal (preferred order)
                    $removed = $false

                    # Try devcon first (if present)
                    if ($useDevcon) {
                        & $DevconPath remove "@$iid" | Out-Null
                        if ($LASTEXITCODE -eq 0) {
                            Write-Log "  → Removed via devcon.exe"
                            $removed = $true
                        } else {
                            Write-Log "  → devcon remove failed (exit $($LASTEXITCODE))"
                        }
                    }

                    # Then try pnputil if supported and not already removed
                    if (-not $removed -and $pnputilSupportsRemove) {
                        $output = & pnputil /remove-device "$iid" 2>&1
                        if ($LASTEXITCODE -eq 0) {
                            Write-Log "  → Removed via pnputil /remove-device"
                            $removed = $true
                        } else {
                            Write-Log "  → pnputil /remove-device failed: $output"
                        }
                    }

                    if (-not $removed) {
                        Write-Log "  → Could not fully remove device (fallback: disabled only)"
                    }
                }
                catch {
                    Write-Log "  → Exception during removal attempt: $($_.Exception.Message)"
                }
            }
            else {
                Write-Log "  [WhatIf] Would attempt to remove/disable $iid"
            }
        }
    }

    # 3. Clean stale network profiles
    Write-Log "Cleaning stale network profile names..."
    $oldProfiles = Get-ChildItem "HKLM:\SOFTWARE\Microsoft\Windows NT\CurrentVersion\NetworkList\Profiles" -ErrorAction SilentlyContinue |
                   Where-Object {
                       $p = Get-ItemProperty $_.PSPath -ErrorAction SilentlyContinue
                       ($p.ProfileName -match '(?i)vjb|Ethernet|Local Area|Wi-Fi') -or
                       ($p.Description -match '(?i)vjb|Ethernet|Local Area|Wi-Fi') -or
                       ([string]::IsNullOrWhiteSpace($p.ProfileName))
                   }

    foreach ($prof in $oldProfiles) {
        $name = (Get-ItemProperty $prof.PSPath -ErrorAction SilentlyContinue).ProfileName
        $guid = $prof.PSChildName
        Write-Log "Found stale profile: '$name' ($guid)"

        if (-not $WhatIf) {
            try {
                Remove-Item $prof.PSPath -Recurse -Force -ErrorAction Stop
                Write-Log "  → Deleted profile key"
            } catch {
                Write-Log "  → Failed to delete profile: $($_.Exception.Message)"
            }
        } else {
            Write-Log "  [WhatIf] Would delete profile $guid"
        }
    }

    # 4. Trigger hardware rescan
    Write-Log "Triggering hardware rescan..."
    if (-not $WhatIf) {
        & "$env:SystemRoot\System32\pnputil.exe" /scan-devices | Out-Null
        Start-Sleep -Seconds 4   # give PnP manager a moment
    }

    Write-Log "=== Cleanup finished ==="
    if ($WhatIf) { Write-Log "NOTE: WhatIf mode - no actual changes were made" }

    exit 0
}
catch {
    Write-Log "CRITICAL ERROR: $($_.Exception.Message)"
    Write-Log $_.ScriptStackTrace
    throw
}