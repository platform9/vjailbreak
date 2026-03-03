# Collect-StaticRoutes.ps1
# PowerShell script to collect and restore static network routes
# Workflow: 1) Collect existing static routes and save to JSON file
#          2) Read from JSON file and restore routes to the OS

param(
    [Parameter(Mandatory=$false)]
    [string]$OutFile = "C:\NIC-Recovery\staticroutes.json",
    
    [Parameter(Mandatory=$false)]
    [string]$LogFile = "C:\NIC-Recovery\staticroutes.log",
    
    [Parameter(Mandatory=$false)]
    [ValidateSet("Collect", "Restore", "Both")]
    [string]$Mode = "Both"
)

#region Logging and Utility Functions
function Write-Log {
    param(
        [Parameter(Mandatory=$true)]
        [string]$Message,
        
        [Parameter(Mandatory=$false)]
        [ValidateSet("INFO", "WARNING", "ERROR", "SUCCESS")]
        [string]$Level = "INFO"
    )
    
    $timestamp = Get-Date -Format "yyyy-MM-dd HH:mm:ss"
    $logEntry = "[$timestamp] [$Level] $Message"
    
    try {
        $logDir = Split-Path -Path $LogFile -Parent
        if (-not (Test-Path $logDir)) { 
            New-Item -ItemType Directory -Path $logDir -Force | Out-Null 
        }
        $logEntry | Out-File -FilePath $LogFile -Append -Encoding utf8
    } catch {
        Write-Host "Failed to write log: $_" -ForegroundColor Red
    }
    
    # Console output with colors based on level
    switch ($Level) {
        "ERROR"   { Write-Host $logEntry -ForegroundColor Red }
        "WARNING" { Write-Host $logEntry -ForegroundColor Yellow }
        "SUCCESS" { Write-Host $logEntry -ForegroundColor Green }
        default   { Write-Host $logEntry -ForegroundColor White }
    }
}


function Initialize-Directories {
    try {
        $outDir = Split-Path -Path $OutFile -Parent
        if (-not (Test-Path $outDir)) {
            New-Item -ItemType Directory -Path $outDir -Force | Out-Null
            Write-Log "Created output directory: $outDir"
        }
        return $true
    } catch {
        Write-Log "Failed to create directories: $($_.Exception.Message)" -Level "ERROR"
        return $false
    }
}
#endregion

#region Route Collection Functions
function Get-StaticRoutesFromSystem {
    Write-Log "=== Starting Static Routes Collection ==="
    
    try {
        Write-Log "Retrieving all network adapters..."
        $adapters = Get-NetAdapter -ErrorAction SilentlyContinue | Where-Object { $_.Status -eq "Up" }
        
        if (-not $adapters) {
            Write-Log "No active network adapters found" -Level "WARNING"
            return @()
        }
        
        Write-Log "Found $($adapters.Count) active network adapters"
        $routesData = @()
        
        foreach ($adapter in $adapters) {
            Write-Log "Processing adapter: $($adapter.Name) (Index: $($adapter.InterfaceIndex))"
            
            try {
                # Get all IPv4 routes for this adapter
                $adapterRoutes = Get-NetRoute -InterfaceIndex $adapter.InterfaceIndex -ErrorAction SilentlyContinue | 
                               Where-Object { 
                                   $_.AddressFamily -eq "IPv4" -and 
                                   $_.RouteMetric -ne 256 -and  # Skip connected routes
                                   $_.NextHop -ne "0.0.0.0"      # Skip default gateway routes
                               }
                
                if ($adapterRoutes) {
                    Write-Log "Found $($adapterRoutes.Count) static routes for adapter $($adapter.Name)"
                    
                    foreach ($route in $adapterRoutes) {
                        $routeObj = [PSCustomObject]@{
                            InterfaceAlias = $adapter.Name
                            InterfaceDescription = $adapter.InterfaceDescription
                            InterfaceIndex = $adapter.InterfaceIndex
                            DestinationPrefix = $route.DestinationPrefix
                            NextHop = $route.NextHop
                            RouteMetric = $route.RouteMetric
                            Protocol = $route.RouteProtocol
                            Publish = $route.Publish
                            Age = $route.Age
                            Preference = $route.Preference
                            NextHopInterfaceIndex = $route.NextHopInterfaceIndex
                        }
                        
                        $routesData += $routeObj
                        Write-Log "Collected route: $($route.DestinationPrefix) -> $($route.NextHop) (Metric: $($route.RouteMetric))"
                    }
                } else {
                    Write-Log "No static routes found for adapter $($adapter.Name)"
                }
            } catch {
                Write-Log "Error retrieving routes for adapter $($adapter.Name): $($_.Exception.Message)" -Level "ERROR"
            }
        }
        
        Write-Log "Total static routes collected: $($routesData.Count)"
        return $routesData
        
    } catch {
        Write-Log "Critical error during route collection: $($_.Exception.Message)" -Level "ERROR"
        return @()
    }
}

function Export-RoutesToFile {
    param(
        [Parameter(Mandatory=$true)]
        [array]$RoutesData
    )
    
    try {
        if ($RoutesData.Count -eq 0) {
            Write-Log "No routes to export" -Level "WARNING"
            return $false
        }
        
        # Group routes by interface for better organization
        $groupedRoutes = $RoutesData | Group-Object -Property InterfaceAlias
        
        $outputData = @()
        foreach ($group in $groupedRoutes) {
            $interfaceData = [PSCustomObject]@{
                InterfaceAlias = $group.Name
                InterfaceDescription = ($group.Group | Select-Object -First 1).InterfaceDescription
                InterfaceIndex = ($group.Group | Select-Object -First 1).InterfaceIndex
                Routes = $group.Group | Select-Object DestinationPrefix, NextHop, RouteMetric, Protocol, Publish, Age, Preference
            }
            $outputData += $interfaceData
        }
        
        # Add metadata
        $metadata = [PSCustomObject]@{
            CollectionTime = Get-Date -Format "yyyy-MM-dd HH:mm:ss"
            ComputerName = $env:COMPUTERNAME
            OSVersion = [System.Environment]::OSVersion.VersionString
            PowerShellVersion = $PSVersionTable.PSVersion.ToString()
            TotalInterfaces = $outputData.Count
            TotalRoutes = $RoutesData.Count
        }
        
        $finalOutput = [PSCustomObject]@{
            Metadata = $metadata
            Interfaces = $outputData
        }
        
        # Export to JSON with proper formatting
        $finalOutput | ConvertTo-Json -Depth 4 | Set-Content -Encoding UTF8 $OutFile
        
        Write-Log "Successfully exported $($RoutesData.Count) routes from $($outputData.Count) interfaces to $OutFile" -Level "SUCCESS"
        return $true
        
    } catch {
        Write-Log "Failed to export routes to file: $($_.Exception.Message)" -Level "ERROR"
        return $false
    }
}
#endregion

#region Route Restoration Functions
function Import-RoutesFromFile {
    try {
        if (-not (Test-Path $OutFile)) {
            Write-Log "Route file not found: $OutFile" -Level "ERROR"
            return $null
        }
        
        Write-Log "Importing routes from: $OutFile"
        $routeData = Get-Content -Path $OutFile -Encoding UTF8 | ConvertFrom-Json
        
        Write-Log "Successfully loaded route data"
        Write-Log "Collection time: $($routeData.Metadata.CollectionTime)"
        Write-Log "Source computer: $($routeData.Metadata.ComputerName)"
        Write-Log "Total interfaces: $($routeData.Metadata.TotalInterfaces)"
        Write-Log "Total routes: $($routeData.Metadata.TotalRoutes)"
        
        return $routeData
        
    } catch {
        Write-Log "Failed to import routes from file: $($_.Exception.Message)" -Level "ERROR"
        return $null
    }
}

function Restore-RoutesToSystem {
    param(
        [Parameter(Mandatory=$true)]
        [object]$RouteData
    )
    
    Write-Log "=== Starting Static Routes Restoration ==="
    
    if (-not $RouteData -or -not $RouteData.Interfaces) {
        Write-Log "Invalid route data provided" -Level "ERROR"
        return $false
    }
    
    $successCount = 0
    $failedCount = 0
    $skippedCount = 0
    
    foreach ($interfaceData in $RouteData.Interfaces) {
        $interfaceName = $interfaceData.InterfaceAlias
        Write-Log "Processing interface: $interfaceName"
        
        # Find the network adapter
        $adapter = Get-NetAdapter | Where-Object { 
            $_.Name -eq $interfaceName -or 
            $_.InterfaceDescription -match $interfaceName 
        } | Select-Object -First 1
        
        if (-not $adapter) {
            Write-Log "Adapter not found for interface: $interfaceName - skipping" -Level "WARNING"
            $skippedCount += $interfaceData.Routes.Count
            continue
        }
        
        Write-Log "Found adapter: $($adapter.Name) (Index: $($adapter.InterfaceIndex))"
        
        # Clean existing conflicting routes
        try {
            $existingRoutes = Get-NetRoute -InterfaceIndex $adapter.InterfaceIndex -ErrorAction SilentlyContinue | 
                            Where-Object { 
                                $_.AddressFamily -eq "IPv4" -and 
                                $_.RouteMetric -ne 256 -and
                                $_.NextHop -ne "0.0.0.0"
                            }
            
            foreach ($existingRoute in $existingRoutes) {
                $conflictRoute = $interfaceData.Routes | Where-Object { 
                    $_.DestinationPrefix -eq $existingRoute.DestinationPrefix -and
                    $_.NextHop -eq $existingRoute.NextHop
                }
                
                if (-not $conflictRoute) {
                    Remove-NetRoute -DestinationPrefix $existingRoute.DestinationPrefix `
                                  -NextHop $existingRoute.NextHop `
                                  -InterfaceIndex $adapter.InterfaceIndex `
                                  -Confirm:$false -ErrorAction SilentlyContinue
                    Write-Log "Removed conflicting route: $($existingRoute.DestinationPrefix) -> $($existingRoute.NextHop)"
                }
            }
        } catch {
            Write-Log "Warning: Could not clean existing routes for $($adapter.Name): $($_.Exception.Message)" -Level "WARNING"
        }
        
        # Add new routes
        foreach ($route in $interfaceData.Routes) {
            try {
                # Check if route already exists
                $existingRoute = Get-NetRoute -DestinationPrefix $route.DestinationPrefix `
                                            -NextHop $route.NextHop `
                                            -InterfaceIndex $adapter.InterfaceIndex `
                                            -ErrorAction SilentlyContinue
                
                if ($existingRoute) {
                    Write-Log "Route already exists: $($route.DestinationPrefix) -> $($route.NextHop) - skipping"
                    $skippedCount++
                    continue
                }
                
                New-NetRoute -DestinationPrefix $route.DestinationPrefix `
                            -NextHop $route.NextHop `
                            -InterfaceIndex $adapter.InterfaceIndex `
                            -RouteMetric $route.RouteMetric `
                            -ErrorAction Stop
                
                Write-Log "Successfully restored route: $($route.DestinationPrefix) -> $($route.NextHop) (Metric: $($route.RouteMetric))" -Level "SUCCESS"
                $successCount++
                
            } catch {
                Write-Log "Failed to restore route $($route.DestinationPrefix) -> $($route.NextHop): $($_.Exception.Message)" -Level "ERROR"
                $failedCount++
            }
        }
    }
    
    Write-Log "Route restoration completed - Success: $successCount, Failed: $failedCount, Skipped: $skippedCount"
    
    if ($failedCount -eq 0) {
        Write-Log "=== Static Routes Restoration completed successfully ===" -Level "SUCCESS"
        return $true
    } else {
        Write-Log "=== Static Routes Restoration completed with $failedCount failures ===" -Level "WARNING"
        return $false
    }
}
#endregion

#region Main Execution
function Main {
    Write-Log "=== Static Routes Management Script Started ==="
    Write-Log "PowerShell version: $($PSVersionTable.PSVersion)"
    Write-Log "OS: $([System.Environment]::OSVersion.VersionString)"
    Write-Log "Computer: $env:COMPUTERNAME"
    Write-Log "Mode: $Mode"
    Write-Log "Output file: $OutFile"
    Write-Log "Log file: $LogFile"
    
    # Initialize directories
    if (-not (Initialize-Directories)) {
        exit 1
    }
    
    $overallSuccess = $true
    
    # Step 1: Collection Phase
    if ($Mode -eq "Collect" -or $Mode -eq "Both") {
        Write-Log "Starting route collection phase..."
        
        $routesData = Get-StaticRoutesFromSystem
        
        if ($routesData.Count -gt 0) {
            $exportSuccess = Export-RoutesToFile -RoutesData $routesData
            if (-not $exportSuccess) {
                $overallSuccess = $false
            }
        } else {
            Write-Log "No static routes found to collect" -Level "WARNING"
        }
    }
    
    # Step 2: Restoration Phase
    if ($Mode -eq "Restore" -or $Mode -eq "Both") {
        Write-Log "Starting route restoration phase..."
        
        $routeData = Import-RoutesFromFile
        if ($routeData) {
            $restoreSuccess = Restore-RoutesToSystem -RouteData $routeData
            if (-not $restoreSuccess) {
                $overallSuccess = $false
            }
        } else {
            Write-Log "No route data available for restoration" -Level "ERROR"
            $overallSuccess = $false
        }
    }
    
    Write-Log "=== Static Routes Management Script Completed ==="
    
    if ($overallSuccess) {
        Write-Log "Script completed successfully" -Level "SUCCESS"
        exit 0
    } else {
        Write-Log "Script completed with errors" -Level "ERROR"
        exit 1
    }
}

# Execute main function
try {
    Main
} catch {
    Write-Log "Unhandled exception: $($_.Exception.Message)" -Level "ERROR"
    Write-Log "Stack trace: $($_.ScriptStackTrace)" -Level "ERROR"
    exit 1
}
#endregion