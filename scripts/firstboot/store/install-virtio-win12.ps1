# =====================================================================
# VirtIO Driver Installation Script - Silent / No Prompt Version
# =====================================================================

$DriverPath = 'C:\Windows\Drivers\VirtIO'
$LogFile    = 'C:\firstboot\virtio-install.log'

function Write-Log {
    param(
        [string]$Message,
        [string]$Level = "INFO"
    )
    
    $timestamp = Get-Date -Format "yyyy-MM-dd HH:mm:ss"
    $logEntry  = "$timestamp - $Level - $Message"
    
    try {
        $ScriptRoot = Split-Path $LogFile -Parent
        if (-not (Test-Path $ScriptRoot)) {
            New-Item -ItemType Directory -Path $ScriptRoot -Force | Out-Null
        }
        $logEntry | Out-File -FilePath $LogFile -Append -Encoding utf8
    }
    catch {
        Write-Host "Failed to write to log file: $_"
    }
    
    if ($Level -eq "ERROR") {
        Write-Host $logEntry -ForegroundColor Red
    }
    else {
        Write-Host $logEntry
    }
}

# ────────────────────────────────────────────────
# Start logging
# ────────────────────────────────────────────────
Write-Log "======================================"
Write-Log "VirtIO Driver Installation Started"
Write-Log "Date: $(Get-Date)"
Write-Log "Log file: $LogFile"
Write-Log "Driver folder: $DriverPath"
Write-Log "======================================"

# ────────────────────────────────────────────────
# Pre-trust Red Hat publisher using Get-AuthenticodeSignature method
# This is more reliable for .cat signed drivers
# ────────────────────────────────────────────────
Write-Log "Attempting to pre-trust Red Hat publisher (to suppress Windows Security prompt)..."

$certImported = $false
$possibleCatFiles = @(
    "$DriverPath\balloon.cat",
    "$DriverPath\*.cat"
)

foreach ($pattern in $possibleCatFiles) {
    $catFiles = Get-ChildItem -Path $pattern -ErrorAction SilentlyContinue
    foreach ($cat in $catFiles) {
        try {
            $signature = Get-AuthenticodeSignature -FilePath $cat.FullName
            if ($signature.Status -eq "Valid" -and $signature.SignerCertificate) {
                $cert = $signature.SignerCertificate
                
                $store = New-Object System.Security.Cryptography.X509Certificates.X509Store(
                    "TrustedPublisher", "LocalMachine"
                )
                $store.Open("ReadWrite")
                
                # Check if already present to avoid duplicates
                $existing = $store.Certificates | Where-Object { $_.Thumbprint -eq $cert.Thumbprint }
                if (-not $existing) {
                    $store.Add($cert)
                    Write-Log "Successfully imported Red Hat certificate (thumbprint: $($cert.Thumbprint)) to TrustedPublisher store from $($cat.Name)."
                    $certImported = $true
                } else {
                    Write-Log "Red Hat certificate (thumbprint: $($cert.Thumbprint)) already in TrustedPublisher store."
                    $certImported = $true
                }
                
                $store.Close()
                break  # Stop after first valid cert found/imported
            }
        }
        catch {
            Write-Log "Failed to process $($cat.Name): $_" -Level "WARNING"
        }
    }
    if ($certImported) { break }
}

if (-not $certImported) {
    Write-Log "WARNING: No valid signed .cat file found or import failed. Prompt may appear during install." -Level "WARNING"
    Write-Log "If prompt happens once interactively (with 'Always trust' checked), future runs will be silent." -Level "WARNING"
}

# ────────────────────────────────────────────────
# Optional: wait for PnP manager / early boot settling
# ────────────────────────────────────────────────
if (Test-Path "$env:ProgramFiles\Guestfs\Firstboot\pnp_wait.exe") {
    Write-Log "Running pnp_wait.exe..."
    try {
        & "$env:ProgramFiles\Guestfs\Firstboot\pnp_wait.exe"
        Write-Log "pnp_wait.exe completed"
    }
    catch {
        Write-Log "Warning: pnp_wait.exe failed: $_" -Level "WARNING"
    }
}
else {
    Write-Log "Waiting for system initialization (30 seconds)..."
    Start-Sleep -Seconds 30
}

# ────────────────────────────────────────────────
# Validate driver folder
# ────────────────────────────────────────────────
if (-not (Test-Path $DriverPath)) {
    Write-Log "ERROR: Driver directory not found: $DriverPath" -Level "ERROR"
    exit 1
}

# ────────────────────────────────────────────────
# Install all VirtIO drivers (unchanged)
# ────────────────────────────────────────────────
Write-Log "Installing VirtIO drivers..."

$drivers = @(
    'balloon.inf',
    'vioser.inf',
    'viorng.inf',
    'vioinput.inf',
    'pvpanic.inf',
    'netkvm.inf',
    'vioscsi.inf',
    'viostor.inf'
)

foreach ($driver in $drivers) {
    $inf = Join-Path $DriverPath $driver
    
    if (Test-Path $inf) {
        Write-Log "Installing $driver ..."
        try {
            $output = & pnputil.exe -i -a $inf 2>&1
            foreach ($line in $output) {
                Write-Log "$line"
            }
        }
        catch {
            Write-Log "Error installing ${driver}: $_" -Level "ERROR"
            # Continue (as in original)
        }
    }
    else {
        Write-Log "Skipped (not found): $driver"
    }
}

# ────────────────────────────────────────────────
# Force hardware rescan (unchanged)
# ────────────────────────────────────────────────
Write-Log "Rescanning hardware..."
try {
    $output = & pnputil.exe /scan-devices 2>&1
    foreach ($line in $output) {
        Write-Log "$line"
    }
}
catch {
    Write-Log "Error rescanning: $_" -Level "ERROR"
}

Write-Log ""
Write-Log "======================================"
Write-Log "VirtIO Driver Installation Complete"
Write-Log "======================================"

exit 0