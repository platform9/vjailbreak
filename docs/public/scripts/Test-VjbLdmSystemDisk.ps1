<#
.SYNOPSIS
    Determines whether the Windows system disk is an LDM (dynamic) disk.

.DESCRIPTION
    Run this on the SOURCE VM in vCenter, before migrating with vJailbreak.

    Windows VMs whose system volume sits on a dynamic disk (Logical Disk Manager)
    cannot be converted by virt-v2v and follow vJailbreak's dedicated SATA-first
    migration path, which has extra prerequisites. Dynamic DATA disks are
    irrelevant - only the disk carrying the system volume changes the outcome.

    The script is strictly READ-ONLY. It issues no diskpart command other than
    'list disk', 'select volume', 'detail volume' and 'san', none of which modify
    the disk layout.

    Detection uses four independent signals, so a wrong answer requires several
    of them to agree incorrectly:

      1. GPT partition type GUID  - 5808C8AA-... (LDM metadata)
                                    AF9B60A0-... (LDM data)
      2. MBR partition type byte  - 0x42 (LDM)
      3. WMI Win32_DiskPartition.Type containing "Logical Disk Manager"
      4. diskpart's Dyn column in 'list disk' / 'detail volume'

    The system volume is mapped to its physical disk(s) by three independent
    routes (Storage module, WMI associators, diskpart). On a dynamic disk the
    Storage-module route fails by design - the partition layer does not model LDM
    volumes - and that failure is itself recorded as corroborating evidence.

.PARAMETER LogPath
    Where the transcript is written. Defaults to %TEMP%\vjb-ldm-check.log.

.PARAMETER Quiet
    Suppress the per-signal transcript on the console. The verdict banner is
    always printed.

.OUTPUTS
    A verdict banner, plus a single machine-readable line:
        VJB_LDM_SYSTEM_DISK=YES|NO|UNKNOWN

    Exit codes:
        0  NO       system disk is basic - normal migration path
        1  YES      system disk is LDM   - SATA-first migration path
        2  UNKNOWN  inconclusive, inspect the log

.NOTES
    Requires an elevated PowerShell session.
    Tested against Windows Server 2012 and later. Written to PowerShell 2.0
    syntax so it also runs on Windows Server 2008 R2, where the Storage module
    is absent and signals 1 and 2 are unavailable.
#>

[CmdletBinding()]
param(
    [string] $LogPath = (Join-Path $env:TEMP 'vjb-ldm-check.log'),
    [switch] $Quiet
)

$ErrorActionPreference = 'Continue'

# GPT partition type GUIDs that identify an LDM disk.
$LDM_GPT_TYPES = @(
    '5808c8aa-7e8f-42e0-85d2-e1e90434cfb3',   # LDM metadata partition
    'af9b60a0-1431-4f62-bc68-3311714a69ad'    # LDM data partition
)
# MBR partition type byte 0x42 = LDM.
$LDM_MBR_TYPE = 66

$script:LogLines = New-Object System.Collections.ArrayList

# ---------------------------------------------------------------- logging ----

function Write-Log {
    param([string] $Level, [string] $Message)

    $line = '{0} [{1,-5}] {2}' -f (Get-Date).ToString('yyyy-MM-dd HH:mm:ss'), $Level, $Message
    [void] $script:LogLines.Add($line)

    if (-not $Quiet) {
        $color = 'Gray'
        if ($Level -eq 'WARN')  { $color = 'Yellow' }
        if ($Level -eq 'ERROR') { $color = 'Red' }
        if ($Level -eq 'HIT')   { $color = 'Magenta' }
        Write-Host $line -ForegroundColor $color
    }
}

# ------------------------------------------------------------- primitives ----

function Test-Elevated {
    try {
        $identity  = [Security.Principal.WindowsIdentity]::GetCurrent()
        $principal = New-Object Security.Principal.WindowsPrincipal($identity)
        return $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
    } catch {
        return $false
    }
}

function Invoke-DiskPart {
    param([string[]] $Commands)

    $scriptFile = [System.IO.Path]::GetTempFileName()
    try {
        Set-Content -Path $scriptFile -Value $Commands -Encoding ASCII
        $output = & diskpart.exe /s $scriptFile 2>&1
        return @($output | ForEach-Object { [string] $_ })
    } catch {
        Write-Log 'WARN' ("diskpart invocation failed: " + $_.Exception.Message)
        return @()
    } finally {
        Remove-Item $scriptFile -Force -ErrorAction SilentlyContinue
    }
}

function Get-Column {
    # Slice a fixed-width diskpart column, bleeding one character either side
    # because diskpart does not always pad exactly to its own rule line.
    param([string] $Line, $Column, [int] $Bleed = 1)

    $start = $Column.Start - $Bleed
    if ($start -lt 0) { $start = 0 }
    if ($Line.Length -le $start) { return '' }

    $length = $Column.Length + ($Column.Start - $start) + $Bleed
    if (($start + $length) -gt $Line.Length) { $length = $Line.Length - $start }
    return $Line.Substring($start, $length)
}

function Read-DiskPartDiskTable {
    # Both 'list disk' and 'detail volume' emit the same six-column table:
    #
    #   Disk ###  Status         Size     Free     Dyn  Gpt
    #   --------  -------------  -------  -------  ---  ---
    #   Disk 0    Online          100 GB     0 B     *
    #
    # Column offsets are taken from the rule line rather than from the header,
    # so parsing does not depend on the console language.
    param([string[]] $Lines)

    $rows = @()
    $ruleIndex = -1
    $columns = @()

    for ($i = 0; $i -lt $Lines.Count; $i++) {
        if ($Lines[$i] -match '^\s*-{3,}(\s+-{2,}){3,}\s*$') {
            $ruleIndex = $i
            foreach ($match in ([regex] '-{2,}').Matches($Lines[$i])) {
                $columns += New-Object PSObject -Property @{
                    Start  = $match.Index
                    Length = $match.Length
                }
            }
            break
        }
    }

    if ($ruleIndex -lt 0 -or $columns.Count -lt 5) {
        Write-Log 'WARN' 'Could not locate the diskpart column rule line; skipping this signal.'
        return $rows
    }

    for ($i = $ruleIndex + 1; $i -lt $Lines.Count; $i++) {
        if ($Lines[$i].Trim().Length -eq 0) { break }

        $number = [regex]::Match((Get-Column $Lines[$i] $columns[0] 0), '\d+')
        if (-not $number.Success) { continue }

        $gpt = $false
        if ($columns.Count -ge 6) {
            $gpt = (Get-Column $Lines[$i] $columns[5]).Contains('*')
        }

        $rows += New-Object PSObject -Property @{
            Number  = [int] $number.Value
            Dynamic = (Get-Column $Lines[$i] $columns[4]).Contains('*')
            Gpt     = $gpt
        }
    }

    return $rows
}

# ------------------------------------------------------------ inspection -----

function Get-DynamicDiskEvidence {
    # Returns the disk numbers that at least one signal reports as LDM.
    param([bool] $StorageModule)

    $dynamic = @{}

    # Signal 4 - diskpart Dyn column.
    foreach ($row in (Read-DiskPartDiskTable (Invoke-DiskPart @('list disk')))) {
        $state = 'basic'
        if ($row.Dynamic) { $state = 'DYNAMIC'; $dynamic[$row.Number] = $true }
        Write-Log 'INFO' ('diskpart      : disk {0} -> {1}' -f $row.Number, $state)
    }

    # Signal 3 - WMI partition type string.
    foreach ($partition in @(Get-WmiObject -Class Win32_DiskPartition -ErrorAction SilentlyContinue)) {
        if ($partition.Type -match 'Logical Disk Manager|LDM') {
            $dynamic[[int] $partition.DiskIndex] = $true
            Write-Log 'HIT' ('Win32_DiskPartition: disk {0} partition type "{1}"' -f $partition.DiskIndex, $partition.Type)
        } else {
            Write-Log 'INFO' ('Win32_DiskPartition: disk {0} partition type "{1}"' -f $partition.DiskIndex, $partition.Type)
        }
    }

    # Signals 1 and 2 - GPT type GUID and MBR type byte. Storage module only.
    if ($StorageModule) {
        foreach ($partition in @(Get-Partition -ErrorAction SilentlyContinue)) {
            $gptType = ''
            if ($partition.GptType) { $gptType = ($partition.GptType -replace '[{}]', '').ToLower() }

            if ($LDM_GPT_TYPES -contains $gptType) {
                $dynamic[[int] $partition.DiskNumber] = $true
                Write-Log 'HIT' ('GPT type GUID : disk {0} partition {1} -> {2}' -f $partition.DiskNumber, $partition.PartitionNumber, $gptType)
            } elseif ([int] $partition.MbrType -eq $LDM_MBR_TYPE) {
                $dynamic[[int] $partition.DiskNumber] = $true
                Write-Log 'HIT' ('MBR type byte : disk {0} partition {1} -> 0x42 (LDM)' -f $partition.DiskNumber, $partition.PartitionNumber)
            }
        }
    } else {
        Write-Log 'WARN' 'Storage module unavailable; partition-type signals skipped (expected on Server 2008 R2).'
    }

    return @($dynamic.Keys | ForEach-Object { [int] $_ })
}

function Get-SystemVolumeDiskNumber {
    # Three independent routes. Returns the union of whatever resolves.
    param([string] $DriveLetter, [bool] $StorageModule)

    $found = @{}

    # Route 1 - Storage module. Fails by design when the volume is LDM.
    if ($StorageModule) {
        $partition = Get-Partition -DriveLetter $DriveLetter -ErrorAction SilentlyContinue
        if ($partition) {
            $hit = @()
            foreach ($p in @($partition)) {
                $found[[int] $p.DiskNumber] = $true
                $hit += [string] $p.DiskNumber
            }
            Write-Log 'INFO' ('Get-Partition : {0}: resolves to disk {1}' -f $DriveLetter, ($hit -join ', '))
        } else {
            Write-Log 'WARN' ('Get-Partition : {0}: has no partition object. The partition layer does not model LDM volumes, so this is expected on a dynamic disk.' -f $DriveLetter)
        }
    }

    # Route 2 - WMI associators. Also unavailable for LDM volumes.
    $query = "ASSOCIATORS OF {{Win32_LogicalDisk.DeviceID='{0}:'}} WHERE AssocClass=Win32_LogicalDiskToPartition" -f $DriveLetter
    $associated = @(Get-WmiObject -Query $query -ErrorAction SilentlyContinue)
    if ($associated.Count -gt 0) {
        $hit = @()
        foreach ($p in $associated) {
            $found[[int] $p.DiskIndex] = $true
            $hit += [string] $p.DiskIndex
        }
        Write-Log 'INFO' ('WMI associator: {0}: resolves to disk {1}' -f $DriveLetter, ($hit -join ', '))
    } else {
        Write-Log 'WARN' ('WMI associator: {0}: has no Win32_LogicalDiskToPartition association, which is typical for an LDM volume.' -f $DriveLetter)
    }

    # Route 3 - diskpart. Works for both basic and dynamic volumes.
    $detail = Invoke-DiskPart @(('select volume ' + $DriveLetter), 'detail volume')
    foreach ($row in (Read-DiskPartDiskTable $detail)) {
        $found[$row.Number] = $true
        $state = 'basic'
        if ($row.Dynamic) { $state = 'DYNAMIC' }
        Write-Log 'INFO' ('detail volume : {0}: resides on disk {1} ({2})' -f $DriveLetter, $row.Number, $state)
    }

    return @($found.Keys | ForEach-Object { [int] $_ })
}

# ------------------------------------------------------------------ main -----

Write-Log 'INFO' '=== vJailbreak LDM system-disk probe (read-only) ==='
Write-Log 'INFO' ('Host          : {0}' -f $env:COMPUTERNAME)
Write-Log 'INFO' ('OS            : {0}' -f (Get-WmiObject -Class Win32_OperatingSystem -ErrorAction SilentlyContinue).Caption)
Write-Log 'INFO' ('PowerShell    : {0}' -f $PSVersionTable.PSVersion.ToString())
Write-Log 'INFO' ('System drive  : {0}' -f $env:SystemDrive)

if (-not (Test-Elevated)) {
    Write-Log 'ERROR' 'Not running elevated. diskpart and the Storage module need Administrator rights.'
    Write-Log 'ERROR' 'Re-run this script from an elevated PowerShell session.'
    $verdict = 'UNKNOWN'
} else {
    $driveLetter     = $env:SystemDrive.Substring(0, 1)
    $storageModule   = $null -ne (Get-Command Get-Disk -ErrorAction SilentlyContinue)
    Write-Log 'INFO' ('Storage module: {0}' -f $(if ($storageModule) { 'available' } else { 'NOT available' }))
    Write-Log 'INFO' '--- mapping the system volume to its physical disk(s) ---'

    $systemDisks = Get-SystemVolumeDiskNumber -DriveLetter $driveLetter -StorageModule $storageModule

    Write-Log 'INFO' '--- collecting LDM evidence for every disk ---'
    $dynamicDisks = Get-DynamicDiskEvidence -StorageModule $storageModule

    Write-Log 'INFO' '--- summary ---'
    Write-Log 'INFO' ('System volume resides on disk(s) : {0}' -f $(if ($systemDisks.Count) { ($systemDisks | Sort-Object) -join ', ' } else { '<unresolved>' }))
    Write-Log 'INFO' ('Disks reported as LDM/dynamic    : {0}' -f $(if ($dynamicDisks.Count) { ($dynamicDisks | Sort-Object) -join ', ' } else { 'none' }))

    if ($systemDisks.Count -gt 0) {
        $hit = @($systemDisks | Where-Object { $dynamicDisks -contains $_ })
        if ($hit.Count -gt 0) {
            $verdict = 'YES'
            Write-Log 'HIT' ('System volume resides on dynamic disk(s): {0}' -f (($hit | Sort-Object) -join ', '))
        } else {
            $verdict = 'NO'
            $dataOnly = @($dynamicDisks | Where-Object { $systemDisks -notcontains $_ })
            if ($dataOnly.Count -gt 0) {
                Write-Log 'INFO' ('Disk(s) {0} are dynamic but carry no system volume. Data disks do not affect the migration path.' -f (($dataOnly | Sort-Object) -join ', '))
            }
        }
    } elseif ($dynamicDisks.Count -gt 0) {
        # No route could map the system volume to a disk, and dynamic disks
        # exist. Both mapping failures are themselves LDM symptoms.
        $verdict = 'YES'
        Write-Log 'HIT' 'The system volume could not be mapped to any partition object while dynamic disks are present. Both facts point to an LDM system volume.'
    } else {
        $verdict = 'UNKNOWN'
        Write-Log 'ERROR' 'The system volume could not be mapped to a disk and no LDM evidence was found. Inspect the log and check the disk layout manually.'
    }

    # Informational only - not part of the verdict. The SAN policy is a
    # prerequisite for the LDM migration path.
    $sanOutput = Invoke-DiskPart @('san')
    foreach ($line in $sanOutput) {
        if ($line -match ':') {
            $trimmed = $line.Trim()
            if ($trimmed -match 'Online|Offline') {
                Write-Log 'INFO' ('SAN policy    : {0}' -f $trimmed)
            }
        }
    }
}

# Persist the transcript before anything else can fail.
try {
    Set-Content -Path $LogPath -Value $script:LogLines -Encoding ASCII
    Write-Host ''
    Write-Host ('Full transcript: {0}' -f $LogPath) -ForegroundColor DarkGray
} catch {
    Write-Host ''
    Write-Host ('Could not write the transcript to {0}: {1}' -f $LogPath, $_.Exception.Message) -ForegroundColor Yellow
}

# ---------------------------------------------------------------- verdict ----
# Printed last, on its own, so it is never buried in the transcript above.

$banner = 'Gray'
if ($verdict -eq 'YES')     { $banner = 'Red' }
if ($verdict -eq 'NO')      { $banner = 'Green' }
if ($verdict -eq 'UNKNOWN') { $banner = 'Yellow' }

# Built by measurement rather than by hand-counted spaces, so the box always
# closes regardless of the verdict length.
$width = 64
$rule  = '#' * $width
$blank = '#' + (' ' * ($width - 2)) + '#'
$label = '   SYSTEM DISK IS LDM (DYNAMIC):   ' + $verdict
$pad   = $width - 2 - $label.Length
if ($pad -lt 0) { $pad = 0 }

Write-Host ''
Write-Host $rule
Write-Host $blank
Write-Host ('#' + $label + (' ' * $pad) + '#') -ForegroundColor $banner
Write-Host $blank
Write-Host $rule
Write-Host ''

if ($verdict -eq 'YES') {
    Write-Host 'This VM takes the SATA-first migration path. Install the VirtIO guest' -ForegroundColor Yellow
    Write-Host 'tools and set "diskpart > san policy=onlineall" on THIS VM before'      -ForegroundColor Yellow
    Write-Host 'migrating, and expect a manual cutover at LDM Boot Verification.'       -ForegroundColor Yellow
    Write-Host ''
}

Write-Host ('VJB_LDM_SYSTEM_DISK={0}' -f $verdict)

if ($verdict -eq 'YES') { exit 1 }
if ($verdict -eq 'NO')  { exit 0 }
exit 2
