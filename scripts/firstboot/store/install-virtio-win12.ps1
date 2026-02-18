$DriverPath = 'C:\Windows\Drivers\VirtIO'
$LogFile = 'C:\ProgramData\virtio-install.log'

$log = @()
$log += '======================================'
$log += 'VirtIO Driver Installation'
$log += "Date: $(Get-Date)"
$log += '======================================'

if (Test-Path "$env:ProgramFiles\Guestfs\Firstboot\pnp_wait.exe") {
    $log += 'Running pnp_wait.exe...'
    try {
        & "$env:ProgramFiles\Guestfs\Firstboot\pnp_wait.exe"
        $log += 'pnp_wait.exe completed'
    } catch {
        $log += "Warning: pnp_wait.exe failed: $_"
    }
}

$log += 'Waiting for system initialization...'
Start-Sleep -Seconds 30

if (-not (Test-Path $DriverPath)) {
    $log += 'ERROR: Driver path not found'
    $log | Add-Content $LogFile
    exit 1
}

$log += 'Installing VirtIO drivers...'

$drivers = @('balloon.inf', 'vioser.inf', 'viorng.inf', 'vioinput.inf', 'pvpanic.inf', 'netkvm.inf', 'vioscsi.inf', 'viostor.inf')

foreach ($driver in $drivers) {
    $inf = Join-Path $DriverPath $driver
    if (Test-Path $inf) {
        $log += "Installing $driver..."
        try {
            $output = & pnputil.exe -i -a $inf 2>&1
            $log += $output
        } catch {
            $log += "Error installing ${driver}: $_"
        }
    }
}

$log += 'Rescanning hardware...'
try {
    $output = & pnputil.exe /scan-devices 2>&1
    $log += $output
} catch {
    $log += "Error rescanning: $_"
}

$log += ''
$log += '======================================'
$log += 'VirtIO Driver Installation Complete'
$log += '======================================'

$log | Add-Content $LogFile
