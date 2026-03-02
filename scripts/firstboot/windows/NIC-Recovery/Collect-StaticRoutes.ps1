# Collect-StaticRoutes.ps1
param(
    [string]$OutFile = "C:\NIC-Recovery\staticroutes.json",
    [string]$LogFile = "C:\NIC-Recovery\staticroutes.log"
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

Write-Log "=== Starting Static Routes Collection ==="
Write-Log "PowerShell version: $($PSVersionTable.PSVersion)"
Write-Log "OS: $([System.Environment]::OSVersion.VersionString)"

try {
    Write-Log "Retrieving all network adapters..."
    $adapters = Get-NetAdapter -ErrorAction SilentlyContinue
    
    if (!$adapters) {
        Write-Log "Warning: No network adapters found"
        $routesData = @()
    } else {
        Write-Log "Found $($adapters.Count) network adapters"
        
        $routesData = @()
        
        foreach ($adapter in $adapters) {
            Write-Log "Processing adapter: $($adapter.Name) (Interface Index: $($adapter.InterfaceIndex))"
            
            try {
                # Get all routes for this adapter
                $adapterRoutes = Get-NetRoute -InterfaceIndex $adapter.InterfaceIndex -ErrorAction SilentlyContinue | 
                               Where-Object {$_.AddressFamily -eq "IPv4"}
                
                if ($adapterRoutes) {
                    Write-Log "Found $($adapterRoutes.Count) routes for adapter $($adapter.Name)"
                    
                    foreach ($route in $adapterRoutes) {
                        # Skip auto-generated/connected routes (metric 256)
                        if ($route.RouteMetric -eq 256) {
                            Write-Log "Skipping connected route: $($route.DestinationPrefix) on $($adapter.Name)"
                            continue
                        }
                        
                        # Create route object with all relevant information
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
                        Write-Log "Collected route: $($route.DestinationPrefix) -> $($route.NextHop) via $($adapter.Name)"
                    }
                } else {
                    Write-Log "No routes found for adapter $($adapter.Name)"
                }
            } catch {
                Write-Log "Warning: Could not retrieve routes for adapter $($adapter.Name): $($_.Exception.Message)"
            }
        }
    }

    Write-Log "Total routes collected: $($routesData.Count)"
    
    # Group routes by interface for better organization
    $groupedRoutes = $routesData | Group-Object -Property InterfaceAlias
    
    $outputData = @()
    foreach ($group in $groupedRoutes) {
        $interfaceData = [PSCustomObject]@{
            InterfaceAlias = $group.Name
            InterfaceDescription = ($group.Group | Select-Object -First 1).InterfaceDescription
            Routes = $group.Group | Select-Object DestinationPrefix, NextHop, RouteMetric, Protocol, Publish, Age, Preference
        }
        $outputData += $interfaceData
    }

    # Export to JSON
    try {
        $outputData | ConvertTo-Json -Depth 4 | Set-Content -Encoding UTF8 $OutFile
        Write-Log "Successfully wrote static routes to $OutFile"
    } catch {
        Write-Log "ERROR: Failed to write output file: $($_.Exception.Message)"
        exit 1
    }

    Write-Log "=== Static Routes Collection finished successfully ==="
    exit 0
}
catch {
    Write-Log "CRITICAL ERROR: $($_.Exception.Message)"
    Write-Log $_.ScriptStackTrace
    throw
}