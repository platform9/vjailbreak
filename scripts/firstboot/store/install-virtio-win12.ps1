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
# Pre-trust Red Hat, Inc. certificate → prevents "Windows Security" prompt
# ────────────────────────────────────────────────
Write-Log "Attempting to pre-trust Red Hat publisher certificate for silent installation..."

$certPath = $null
$possibleCertFiles = @(
    "$DriverPath\balloon.cat",
    "$DriverPath\balloon.inf",
    "$DriverPath\vioscsi.cat",
    "$DriverPath\vioscsi.inf",
    "$DriverPath\*.cat",
    "$DriverPath\*.inf"
)

foreach ($pattern in $possibleCertFiles) {
    $candidates = Get-ChildItem -Path $pattern -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($candidates) {
        $certPath = $candidates.FullName
        break
    }
}

if ($certPath) {
    try {
        # Load the signed file and extract the certificate
        $cert = New-Object System.Security.Cryptography.X509Certificates.X509Certificate2
        $cert.Import($certPath)

        # Add to Local Machine \ TrustedPublisher store
        $store = New-Object System.Security.Cryptography.X509Certificates.X509Store(
            [System.Security.Cryptography.X509Certificates.StoreName]::TrustedPublisher,
            [System.Security.Cryptography.X509Certificates.StoreLocation]::LocalMachine
        )
        
        $store.Open([System.Security.Cryptography.X509Certificates.OpenFlags]::ReadWrite)
        # Avoid duplicate if already present
        if (-not $store.Certificates.Contains($cert)) {
            $store.Add($cert)
            Write-Log "Successfully added Red Hat, Inc. certificate to TrustedPublisher store."
        } else {
            Write-Log "Red Hat certificate already present in TrustedPublisher store."
        }
        $store.Close()
    }
    catch {
        Write-Log "Warning: Failed to import certificate: $_" -Level "WARNING"
        Write-Log "Installation may still show a one-time prompt." -Level "WARNING"
    }
}
else {
    Write-Log "Warning: Could not locate any .cat or .inf file to extract certificate from." -Level "WARNING"
    Write-Log "Prompt may appear during first driver install." -Level "WARNING"
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
# Install all VirtIO drivers
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
            $output = & pnputil.exe /add-driver $inf /install 2>&1
            foreach ($line in $output) {
                if ($line -match '\S') { Write-Log "  $line" }
            }
        }
        catch {
            Write-Log "Error while installing ${driver}: $_" -Level "ERROR"
            # Continue with next driver instead of hard exit - adjust if desired
        }
    }
    else {
        Write-Log "Skipped (not found): $driver"
    }
}

# ────────────────────────────────────────────────
# Force hardware rescan
# ────────────────────────────────────────────────
Write-Log "Rescanning hardware for new devices..."
try {
    $output = & pnputil.exe /scan-devices 2>&1
    foreach ($line in $output) {
        if ($line -match '\S') { Write-Log "  $line" }
    }
}
catch {
    Write-Log "Warning: Hardware rescan failed: $_" -Level "WARNING"
}

# ────────────────────────────────────────────────
# Finish
# ────────────────────────────────────────────────
Write-Log ""
Write-Log "======================================"
Write-Log "VirtIO Driver Installation Complete"
Write-Log "======================================"

exit 0