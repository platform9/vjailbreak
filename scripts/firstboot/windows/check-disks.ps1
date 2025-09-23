# Disk Management Status Check Script
# Reports on disk status and drive letter assignments (READ-ONLY)
# Run as Administrator

param(
    [string]$LogPath = "C:\DiskStatus_Report.txt"
)

# Function to log messages
function Write-Log {
    param([string]$Message, [string]$Level = "INFO")
    $timestamp = Get-Date -Format "yyyy-MM-dd HH:mm:ss"
    $logEntry = "[$timestamp] [$Level] $Message"
    Write-Host $logEntry
    Add-Content -Path $LogPath -Value $logEntry
}

Write-Log "=== Disk Status Check Started ==="

# Check if running as Administrator
if (-NOT ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole] "Administrator")) {
    Write-Log "This script must be run as Administrator!" "ERROR"
    exit 1
}

Write-Log "Running in READ-ONLY mode - no changes will be made" "INFO"

try {
    # Get all physical disks
    Write-Log "Scanning for all physical disks..."
    $physicalDisks = Get-Disk
    Write-Log "Found $($physicalDisks.Count) physical disk(s)"

    # Get all partitions
    Write-Log "Scanning for partition drive letter assignments..."
    $allPartitions = Get-Partition
    $unassignedPartitions = $allPartitions | Where-Object { $_.DriveLetter -eq $null -and $_.Type -eq "Basic" -and $_.Size -gt 100MB } # Type has been added to avoid Dynamic Disk partitions getting scanned by this check

    if ($unassignedPartitions.Count -gt 0) {
        Write-Log "ISSUE: Found $($unassignedPartitions.Count) partition(s) without drive letters" "WARNING"
        foreach ($partition in $unassignedPartitions) {
            Write-Log "  - Disk $($partition.DiskNumber), Partition $($partition.PartitionNumber) (Size: $($partition.Size/1GB -as [int]) GB) - NO DRIVE LETTER" "WARNING"
        }
    }
    else {
        Write-Log "OK: All eligible partitions have drive letters assigned"
    }

    # Detailed status report
    Write-Log "=== Detailed Disk Configuration Status ==="

    foreach ($disk in $physicalDisks) {
        $status = $disk.OperationalStatus
        $readonly = if ($disk.IsReadOnly) { "Read-Only" } else { "Read-Write" }
        $health = $disk.HealthStatus
        Write-Log "Disk $($disk.Number): $status, $readonly, Health: $health, Size: $($disk.Size/1GB -as [int]) GB"

        # List partitions for this disk
        $diskPartitions = @(Get-Partition -DiskNumber $disk.Number -ErrorAction SilentlyContinue)

        if ($diskPartitions.Count -gt 0) {
              Write-Log "  └─ Partition Partition Number: Drive letter, type, size in GB"
        }        
        else {
        Write-Log "  └─ No Partitions Found for this disk" "INFO"
        }

        foreach ($partition in $diskPartitions) {
            $letter = if ($partition.DriveLetter) { $partition.DriveLetter + ":" } else { "No Letter" }
            $type = $partition.Type
            $size = $partition.Size/1GB -as [int]
            Write-Log "  └─ Partition $($partition.PartitionNumber): $letter, $type, $size GB"
        }
    }

    # Summary of issues
    Write-Log "=== Status Summary ==="
    $offlineDisks = @($physicalDisks | Where-Object { $_.OperationalStatus -eq "Offline" })
    $readOnlyDisks = @($physicalDisks | Where-Object { $_.IsReadOnly -eq $true })
    $unhealthyDisks = @($physicalDisks | Where-Object { $_.HealthStatus -ne "Healthy" })
    
    $totalIssues = $offlineDisks.Count + $readOnlyDisks.Count + $unhealthyDisks.Count + $unassignedPartitions.Count
    
    if ($totalIssues -eq 0) {
        Write-Log "✓ ALL CHECKS PASSED: All disks are online, healthy, and all eligible partitions have drive letters assigned" "SUCCESS"
    }
    else {
        Write-Log "ISSUES DETECTED: $totalIssues issue(s) found that may require attention" "WARNING"
    }

    if ($offlineDisks.Count -gt 0) {
        Write-Log "ISSUES FOUND: $($offlineDisks.Count) disk(s) are offline" "WARNING"

        foreach ($disk in $offlineDisks) {
            Write-Log "  - Disk $($disk.Number): $($disk.FriendlyName)" "WARNING"
        }
    }

    if ($readOnlyDisks.Count -gt 0) {
        Write-Log "ISSUES FOUND: $($readOnlyDisks.Count) disk(s) are read-only" "WARNING"

        foreach ($disk in $readOnlyDisks) {
            Write-Log "  - Disk $($disk.Number): $($disk.FriendlyName)" "WARNING"
        }
    }

    if ($unhealthyDisks.Count -gt 0) {
        Write-Log "ISSUES FOUND: $($unhealthyDisks.Count) disk(s) have health issues" "WARNING"

        foreach ($disk in $unhealthyDisks) {
            Write-Log "  - Disk $($disk.Number): $($disk.FriendlyName) - Status: $($disk.HealthStatus)" "WARNING"
        }
    }

    if ($unassignedPartitions.Count -gt 0) {
        Write-Log "ISSUES FOUND: $($unassignedPartitions.Count) partition(s) without drive letters" "WARNING"
    }
   
}
catch {
    Write-Log "Critical error during disk status check: $($_.Exception.Message)" "ERROR"
    exit 1
}

Write-Log "=== Disk Status Check Completed ==="
Write-Log "Status report saved to: $LogPath"
# Summary output

Write-Host "- Detailed report saved to: $LogPath" -ForegroundColor Cyan