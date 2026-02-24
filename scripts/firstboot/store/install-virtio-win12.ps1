$DriverPath = 'C:\Windows\Drivers\VirtIO'
$LogFile = 'C:\firstboot\virtio-install.log'

function Write-Log {
    param([string]$Message, [string]$Level = "INFO")
    $timestamp = Get-Date -Format "yyyy-MM-dd HH:mm:ss"
    $logEntry = "$timestamp - $Level - $Message"
    
    try {
        $ScriptRoot = Split-Path $LogFile -Parent
        if (-not (Test-Path $ScriptRoot)) {
            New-Item -ItemType Directory -Path $ScriptRoot -Force | Out-Null
        }
        $logEntry | Out-File -FilePath $LogFile -Append -Encoding utf8
    } catch {
        Write-Host "Failed to write to log file: $_"
    }
    
    if ($Level -eq "ERROR") {
        Write-Host $logEntry -ForegroundColor Red
    } else {
        Write-Host $logEntry
    }
}

Write-Log "======================================"
Write-Log "VirtIO Driver Installation"
Write-Log "Date: $(Get-Date)"
Write-Log "======================================"

if (Test-Path "$env:ProgramFiles\Guestfs\Firstboot\pnp_wait.exe") {
    Write-Log 'Running pnp_wait.exe...'
    try {
        & "$env:ProgramFiles\Guestfs\Firstboot\pnp_wait.exe"
        Write-Log 'pnp_wait.exe completed'
    } catch {
        Write-Log "Warning: pnp_wait.exe failed: $_" -Level "WARNING"
    }
}

Write-Log 'Waiting for system initialization...'
Start-Sleep -Seconds 30

if (-not (Test-Path $DriverPath)) {
    Write-Log 'ERROR: Driver path not found' -Level "ERROR"
    exit 1
}

Write-Log 'Installing VirtIO drivers...'

$drivers = @('balloon.inf', 'vioser.inf', 'viorng.inf', 'vioinput.inf', 'pvpanic.inf', 'netkvm.inf', 'vioscsi.inf', 'viostor.inf')

foreach ($driver in $drivers) {
    $inf = Join-Path $DriverPath $driver
    if (Test-Path $inf) {
        Write-Log "Installing $driver..."
        try {
            $output = & pnputil.exe -i -a $inf 2>&1
            foreach ($line in $output) {
                Write-Log "$line"
            }
        } catch {
            Write-Log "Error installing ${driver}: $_" -Level "ERROR"
            exit 1
        }
    }
}

Write-Log 'Rescanning hardware...'
try {
    $output = & pnputil.exe /scan-devices 2>&1
    foreach ($line in $output) {
        Write-Log "$line"
    }
} catch {
    Write-Log "Error rescanning: $_" -Level "ERROR"
    exit 1
}

Write-Log ""
Write-Log "======================================"
Write-Log "VirtIO Driver Installation Complete"
Write-Log "======================================"
exit 0
