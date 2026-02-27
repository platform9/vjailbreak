[string]$LogPath = "C:\DiskStatus_Report.txt"

$ErrorActionPreference = "Stop"

function Write-Log {
    param([string]$Message, [string]$Level = "INFO")
    $timestamp = Get-Date -Format "yyyy-MM-dd HH:mm:ss"
    $logEntry = "[$timestamp] [$Level] $Message"
    Write-Host $logEntry
    Add-Content -Path $LogPath -Value $logEntry
}

Write-Log "=== Disk Status Check Started (Universal Mode) ==="

if (-NOT ([Security.Principal.WindowsPrincipal] `
    [Security.Principal.WindowsIdentity]::GetCurrent()
).IsInRole([Security.Principal.WindowsBuiltInRole] "Administrator")) {

    Write-Log "This script must be run as Administrator!" "ERROR"
    exit 1
}

Write-Log "Running in AUTO-FIX MODE - Bringing offline disks online where possible" "WARNING"

try {

    Write-Log "Scanning for all physical disks..."
    $physicalDisks = @(Get-Disk -ErrorAction Stop)
    Write-Log "Found $($physicalDisks.Count) physical disk(s)"

    Write-Log "Scanning for partition drive letter assignments..."
    $allPartitions = @(Get-Partition -ErrorAction Stop)

    $unassignedPartitions = @(
        $allPartitions | Where-Object {
            -not $_.DriveLetter -and
            $_.Size -gt 1GB -and
            $_.Type -eq "Basic"
        }
    )

    if ($unassignedPartitions.Count -gt 0) {
        Write-Log "ISSUE: Found $($unassignedPartitions.Count) data partition(s) without drive letters" "WARNING"
        foreach ($partition in $unassignedPartitions) {
            Write-Log " - Disk $($partition.DiskNumber), Partition $($partition.PartitionNumber) (Size: $([int]($partition.Size/1GB)) GB) - NO DRIVE LETTER" "WARNING"
        }
    }
    else {
        Write-Log "OK: All eligible partitions have drive letters assigned"
    }

    Write-Log "=== Detailed Disk Configuration Status ==="

    foreach ($disk in $physicalDisks) {

        $status = $disk.OperationalStatus
        $readonly = if ($disk.IsReadOnly) { "Read-Only" } else { "Read-Write" }
        $health = $disk.HealthStatus

        Write-Log "Disk $($disk.Number): $status, $readonly, Health: $health, Size: $([int]($disk.Size/1GB)) GB"

        $diskPartitions = @(Get-Partition -DiskNumber $disk.Number -ErrorAction SilentlyContinue)

        if ($diskPartitions.Count -eq 0) {
            Write-Log " No Partitions Found for this disk"
        }

        foreach ($partition in $diskPartitions) {
            $letter = if ($partition.DriveLetter) { "$($partition.DriveLetter):" } else { "No Letter" }
            $type   = $partition.Type
            $size   = [int]($partition.Size/1GB)
            Write-Log " Partition $($partition.PartitionNumber): $letter, $type, $size GB"
        }
    }

    Write-Log "=== Status Summary ==="

    $offlineDisks   = @($physicalDisks | Where-Object { $_.OperationalStatus -eq "Offline" })
    $readOnlyDisks  = @($physicalDisks | Where-Object { $_.IsReadOnly })
    $unhealthyDisks = @($physicalDisks | Where-Object { $_.HealthStatus -ne "Healthy" })

    if ($offlineDisks.Count -gt 0) {

        Write-Log "Found $($offlineDisks.Count) offline disk(s)" "WARNING"

        foreach ($disk in $offlineDisks) {
            try {
                Write-Log "Bringing Disk $($disk.Number) online..."
                Set-Disk -Number $disk.Number -IsOffline $false -ErrorAction Stop
                Write-Log "SUCCESS: Disk $($disk.Number) is now online" "SUCCESS"
            }
            catch {
                Write-Log "FAILED to bring Disk $($disk.Number) online: $($_.Exception.Message)" "ERROR"
            }
        }
    }
    else {
        Write-Log "OK: No offline disks detected"
    }

    if ($readOnlyDisks.Count -gt 0) {
        Write-Log "WARNING: $($readOnlyDisks.Count) disk(s) are read-only"
    }

    if ($unhealthyDisks.Count -gt 0) {
        Write-Log "WARNING: $($unhealthyDisks.Count) disk(s) report health issues"
    }

}
catch {
    Write-Log "Critical error during disk status check: $($_.Exception.Message)" "ERROR"
    exit 1
}

Write-Log "=== Disk Status Check Completed ==="
Write-Log "Status report saved to: $LogPath"

Write-Host "- Detailed report saved to: $LogPath"

exit 0