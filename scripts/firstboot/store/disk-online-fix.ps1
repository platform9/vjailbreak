[string]$LogPath = "C:\DiskStatus_Report.txt"

function Write-Log {
    param([string]$Message, [string]$Level = "INFO")
    $timestamp = Get-Date -Format "yyyy-MM-dd HH:mm:ss"
    $logEntry = "[$timestamp] [$Level] $Message"
    Write-Host $logEntry
    Add-Content -Path $LogPath -Value $logEntry
}

Write-Log "=== Disk Status Check Started (Post-VMware to KVM Migration Fix Mode) ==="

if (-NOT ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole] "Administrator")) {
    Write-Log "This script must be run as Administrator!" "ERROR"
    exit 1
}

Write-Log "Running in AUTO-FIX MODE - Blanket attempting to bring all offline disks online" "WARNING"
Write-Log "Context: Post-VMware to OpenStack/KVM conversion. Pre-migration disk states unknown." "INFO"

try {
    # Get all physical disks
    Write-Log "Scanning for all physical disks..."
    $physicalDisks = Get-Disk
    Write-Log "Found $($physicalDisks.Count) physical disk(s)"

    # Get all partitions
    Write-Log "Scanning for partition drive letter assignments..."
    $allPartitions = Get-Partition
    $unassignedPartitions = @($allPartitions | Where-Object { $_.DriveLetter -eq 0 -and $_.Type -notin @("Recovery", "Reserved", "System", "Dynamic") -and $_.Size -gt 100MB })

    if ($unassignedPartitions.Count -gt 0) {
        Write-Log "ISSUE: Found $($unassignedPartitions.Count) partition(s) without drive letters" "WARNING"
        foreach ($partition in $unassignedPartitions) {
            Write-Log " - Disk $($partition.DiskNumber), Partition $($partition.PartitionNumber) (Size: $($partition.Size/1GB -as [int]) GB) - NO DRIVE LETTER" "WARNING"
        }
    } else {
        Write-Log "OK: All eligible partitions have drive letters assigned" "INFO"
    }

    # Detailed status report
    Write-Log "=== Detailed Disk Configuration Status ==="

    foreach ($disk in $physicalDisks) {
        $status = $disk.OperationalStatus
        $readonly = if ($disk.IsReadOnly) { "Read-Only" } else { "Read-Write" }
        $health = $disk.HealthStatus
        Write-Log "Disk $($disk.Number): $status, $readonly, Health: $health, Size: $($disk.Size/1GB -as [int]) GB"

        $diskPartitions = @(Get-Partition -DiskNumber $disk.Number -ErrorAction SilentlyContinue)

        if ($diskPartitions.Count -gt 0) {
            Write-Log " Partition Partition Number: Drive letter, type, size in GB"
        } else {
            Write-Log " No Partitions Found for this disk" "INFO"
        }

        foreach ($partition in $diskPartitions) {
            $letter = if ($partition.DriveLetter) { $partition.DriveLetter + ":" } else { "No Letter" }
            $type = $partition.Type
            $size = $partition.Size/1GB -as [int]
            Write-Log " Partition $($partition.PartitionNumber): $letter, $type, $size GB"
        }
    }

    # Summary of issues
    Write-Log "=== Status Summary ==="
    $offlineDisks = @($physicalDisks | Where-Object { $_.OperationalStatus -eq "Offline" })
    $readOnlyDisks = @($physicalDisks | Where-Object { $_.IsReadOnly -eq $true })
    $unhealthyDisks = @($physicalDisks | Where-Object { $_.HealthStatus -ne "Healthy" })

    $totalIssues = $offlineDisks.Count + $readOnlyDisks.Count + $unhealthyDisks.Count + $unassignedPartitions.Count

    if ($totalIssues -eq 0) {
        Write-Log "ALL CHECKS PASSED: All disks are online, healthy, and all eligible partitions have drive letters assigned" "SUCCESS"
    } else {
        Write-Log "ISSUES DETECTED: $totalIssues issue(s) found that may require attention" "WARNING"
    }

    if ($offlineDisks.Count -gt 0) {
        Write-Log "ISSUES FOUND: $($offlineDisks.Count) disk(s) are offline" "WARNING"
        foreach ($disk in $offlineDisks) {
            Write-Log " - Disk $($disk.Number): $($disk.FriendlyName)" "WARNING"
        }

        Write-Log "Attempting to bring all offline disks online..." "INFO"
        $fixedCount = 0
        $failedCount = 0

        foreach ($disk in $offlineDisks) {
            try {
                Write-Log "Attempting to bring Disk $($disk.Number) online..." "INFO"
                Set-Disk -Number $disk.Number -IsOffline $false -ErrorAction Stop
                Write-Log "SUCCESS: Disk $($disk.Number) is now online" "SUCCESS"
                $fixedCount++
            }
            catch {
                Write-Log "FAILED to bring Disk $($disk.Number) online: $($_.Exception.Message)" "ERROR"
                $failedCount++
            }
        }

        Write-Log "Fix Summary: $fixedCount disk(s) brought online, $failedCount failed" "INFO"
    } else {
        Write-Log "OK: No offline disks detected" "INFO"
    }

    if ($readOnlyDisks.Count -gt 0) {
        Write-Log "ISSUES FOUND: $($readOnlyDisks.Count) disk(s) are read-only" "WARNING"
        foreach ($disk in $readOnlyDisks) {
            Write-Log " - Disk $($disk.Number): $($disk.FriendlyName)" "WARNING"
        }
    }

    if ($unhealthyDisks.Count -gt 0) {
        Write-Log "ISSUES FOUND: $($unhealthyDisks.Count) disk(s) have health issues" "WARNING"
        foreach ($disk in $unhealthyDisks) {
            Write-Log " - Disk $($disk.Number): $($disk.FriendlyName) - Status: $($disk.HealthStatus)" "WARNING"
        }
    }

    if ($unassignedPartitions.Count -gt 0) {
        Write-Log "ISSUES FOUND: $($unassignedPartitions.Count) partition(s) without drive letters" "WARNING"
        Write-Log "Note: Drive letter assignment is not automated in this script to avoid conflicts. Use Disk Management GUI or additional cmdlets if needed." "INFO"
    }
}
catch {
    Write-Log "Critical error during disk status check: $($_.Exception.Message)" "ERROR"
    exit 1
}

Write-Log "=== Disk Status Check Completed ==="
Write-Log "Status report saved to: $LogPath"

Write-Host "- Detailed report saved to: $LogPath" -ForegroundColor Cyan