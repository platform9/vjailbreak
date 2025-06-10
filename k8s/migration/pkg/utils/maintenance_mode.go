package utils

import (
	"context"
	"fmt"

	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// CanEnterMaintenanceMode checks if an ESXi host can successfully enter maintenance mode
// with all VMs automatically migrating off to other hosts. It checks host, VM, and cluster
// settings that could block automatic VM migration.
func CanEnterMaintenanceMode(ctx context.Context, k8sClient client.Client, vmwcreds *vjailbreakv1alpha1.VMwareCreds, hostName string) (bool, []string, error) {
	// List of VMs that cannot migrate
	var blockedVMs []string

	// Connect to vCenter
	c, err := ValidateVMwareCreds(ctx, k8sClient, vmwcreds)
	if err != nil {
		return false, blockedVMs, fmt.Errorf("failed to validate vCenter connection: %w", err)
	}

	// Create a finder to locate objects
	finder := find.NewFinder(c, true)

	// Use default datacenter
	dc, err := finder.DefaultDatacenter(ctx)
	if err != nil {
		return false, blockedVMs, fmt.Errorf("error getting datacenter: %w", err)
	}
	finder.SetDatacenter(dc)

	// Get host system
	host, err := finder.HostSystem(ctx, hostName)
	if err != nil {
		return false, blockedVMs, fmt.Errorf("host not found: %w", err)
	}

	// Get host properties
	var hostProps mo.HostSystem
	err = host.Properties(ctx, host.Reference(), []string{"vm", "parent"}, &hostProps)
	if err != nil {
		return false, blockedVMs, fmt.Errorf("error getting host properties: %w", err)
	}

	// Check if host already in maintenance mode
	var hostSummary mo.HostSystem
	err = host.Properties(ctx, host.Reference(), []string{"runtime"}, &hostSummary)
	if err != nil {
		return false, blockedVMs, fmt.Errorf("error getting host runtime: %w", err)
	}

	if hostSummary.Runtime.InMaintenanceMode {
		klog.Infof("Host %s already in maintenance mode", hostName)
		return true, blockedVMs, nil
	}

	// Check cluster settings
	clusterMoRef := hostProps.Parent
	var cluster mo.ClusterComputeResource
	pc := property.DefaultCollector(c)
	err = pc.RetrieveOne(ctx, *clusterMoRef, []string{"configuration", "name"}, &cluster)
	if err != nil {
		return false, blockedVMs, fmt.Errorf("failed to get cluster information: %w", err)
	}

	// Check if DRS is enabled and in fully automated mode
	if cluster.Configuration.DrsConfig.Enabled == nil {
		return false, blockedVMs, fmt.Errorf("cluster configuration not available")
	}

	drsEnabled := cluster.Configuration.DrsConfig.Enabled != nil && *cluster.Configuration.DrsConfig.Enabled
	drsAutomationLevel := cluster.Configuration.DrsConfig.DefaultVmBehavior

	// Check if DRS automation level supports automatic migration
	if !drsEnabled {
		return false, blockedVMs, fmt.Errorf("DRS not enabled on cluster %s, VM migration will not be automatic", cluster.Name)
	}

	fullyAutomated := drsAutomationLevel == types.DrsBehaviorFullyAutomated
	if !fullyAutomated {
		klog.Warningf("DRS not in fully automated mode on cluster %s, some VMs may not migrate automatically", cluster.Name)
	}

	// Check if there are any other hosts in the cluster to migrate to
	var clusterHosts mo.ClusterComputeResource
	err = pc.RetrieveOne(ctx, *clusterMoRef, []string{"host"}, &clusterHosts)
	if err != nil {
		return false, blockedVMs, fmt.Errorf("failed to get cluster hosts: %w", err)
	}

	if len(clusterHosts.Host) <= 1 {
		return false, blockedVMs, fmt.Errorf("cluster %s has only one host, nowhere to migrate VMs", cluster.Name)
	}

	// Check cluster resource capacity
	var clusterResourceUsageSummary mo.ClusterComputeResource
	err = pc.RetrieveOne(ctx, *clusterMoRef, []string{"summary"}, &clusterResourceUsageSummary)
	if err != nil {
		return false, blockedVMs, fmt.Errorf("failed to get cluster resource summary: %w", err)
	}

	// Check if remaining hosts have enough capacity
	if clusterResourceUsageSummary.Summary != nil && clusterResourceUsageSummary.Summary.GetComputeResourceSummary() != nil {
		summary := clusterResourceUsageSummary.Summary.GetComputeResourceSummary()

		// Calculate if removing this host would leave enough capacity
		// This is a basic estimation - in reality vSphere DRS has more sophisticated algorithms
		effectiveHosts := len(clusterHosts.Host) - 1 // Removing the host we're checking
		if effectiveHosts > 0 {
			currentCpuUsage := float64(summary.TotalCpu-summary.EffectiveCpu) / float64(summary.TotalCpu)
			currentMemUsage := float64(summary.TotalMemory-summary.EffectiveMemory) / float64(summary.TotalMemory)

			// A rough estimation assuming equal hosts
			projectedCpuUsage := currentCpuUsage * float64(len(clusterHosts.Host)) / float64(effectiveHosts)
			projectedMemUsage := currentMemUsage * float64(len(clusterHosts.Host)) / float64(effectiveHosts)

			if projectedCpuUsage > 0.90 || projectedMemUsage > 0.90 {
				klog.Warningf("Cluster might not have enough resources after removing host. Projected usage: CPU %.2f%%, Memory %.2f%%",
					projectedCpuUsage*100, projectedMemUsage*100)
			}
		}
	}

	// Check VMs on the host
	if len(hostProps.Vm) == 0 {
		klog.Infof("No VMs on host %s, maintenance mode can be entered", hostName)
		return true, blockedVMs, nil
	}

	// Get properties for all VMs on the host
	var vms []mo.VirtualMachine
	var vmRefs []types.ManagedObjectReference
	vmRefs = append(vmRefs, hostProps.Vm...)

	err = pc.Retrieve(ctx, vmRefs, []string{"name", "runtime", "config", "summary"}, &vms)
	if err != nil {
		return false, blockedVMs, fmt.Errorf("failed to retrieve VM properties: %w", err)
	}

	for _, vm := range vms {
		// Check VM power state
		if vm.Runtime.PowerState == types.VirtualMachinePowerStatePoweredOff {
			klog.V(4).Infof("VM %s is powered off, no migration needed", vm.Name)
			continue
		}

		// Check for vMotion capability
		if vm.Runtime.Host == nil {
			blockedVMs = append(blockedVMs, fmt.Sprintf("%s (no host information)", vm.Name))
			continue
		}

		// Check for VM specific issues that would block migration
		if vm.Config == nil {
			klog.Warningf("Could not get configuration for VM %s", vm.Name)
			continue
		}

		// Check for connected devices that would block migration
		hasBlockingDevices := false
		if vm.Config.Hardware.Device != nil {
			for _, device := range vm.Config.Hardware.Device {
				// Check for connected local media like ISOs
				if cdrom, ok := device.(*types.VirtualCdrom); ok {
					if cdrom.Connectable != nil && cdrom.Connectable.Connected && cdrom.Connectable.StartConnected {
						// Check if it's a client device or ISO
						// Remote passthrough devices are client devices that would block migration
						// File-backed ISOs are typically ok for migration
						if _, ok := cdrom.Backing.(*types.VirtualCdromRemotePassthroughBackingInfo); ok {
							hasBlockingDevices = true
							klog.Warningf("VM %s has connected client device", vm.Name)
							break
						}

						// Also check for host device passthrough which blocks migration
						if _, ok := cdrom.Backing.(*types.VirtualCdromAtapiBackingInfo); ok {
							hasBlockingDevices = true
							klog.Warningf("VM %s has host passthrough device", vm.Name)
							break
						}
					}
				}

				// Check for USB and other passthrough devices
				if _, ok := device.(*types.VirtualUSB); ok {
					hasBlockingDevices = true
					klog.Warningf("VM %s has USB device that may block migration", vm.Name)
					break
				}
			}
		}

		if hasBlockingDevices {
			blockedVMs = append(blockedVMs, fmt.Sprintf("%s (connected devices)", vm.Name))
			continue
		}

		// Check VM-Host affinity rules
		// This is a basic implementation - in a production environment you would
		// check detailed DRS rules including must/should constraints
		if vm.Config.ExtraConfig != nil {
			hasAffinityRules := false
			for _, opt := range vm.Config.ExtraConfig {
				if opt.GetOptionValue().Key == "hostAffinityRule" {
					hasAffinityRules = true
					break
				}
			}
			if hasAffinityRules {
				blockedVMs = append(blockedVMs, fmt.Sprintf("%s (affinity rules)", vm.Name))
				continue
			}
		}

		// Check additional migration constraints

		// Check VM runtime status that could prevent migration
		if vm.Runtime.PowerState == types.VirtualMachinePowerStateSuspended {
			klog.Warningf("VM %s is in suspended state, which prevents migration", vm.Name)
			blockedVMs = append(blockedVMs, fmt.Sprintf("%s (suspended state)", vm.Name))
		}

		// Check if there's a pending question on the VM (requires user input)
		if vm.Runtime.Question != nil {
			klog.Warningf("VM %s has pending questions requiring user input", vm.Name)
			blockedVMs = append(blockedVMs, fmt.Sprintf("%s (pending question)", vm.Name))
		}
	}

	if len(blockedVMs) > 0 {
		return false, blockedVMs, fmt.Errorf("some VMs on host %s are blocked for migration", hostName)
	}

	return true, blockedVMs, nil
}

func GetMaintenanceModeOptions(ctx context.Context, k8sClient client.Client, vmwcreds *vjailbreakv1alpha1.VMwareCreds, hostName string) (*types.HostMaintenanceSpec, error) {
	// Connect to vCenter
	c, err := ValidateVMwareCreds(ctx, k8sClient, vmwcreds)
	if err != nil {
		return nil, fmt.Errorf("failed to validate vCenter connection: %w", err)
	}

	// Create a finder to locate objects
	finder := find.NewFinder(c, true)

	// Use default datacenter
	dc, err := finder.DefaultDatacenter(ctx)
	if err != nil {
		return nil, fmt.Errorf("error getting datacenter: %w", err)
	}
	finder.SetDatacenter(dc)

	// Get host system
	host, err := finder.HostSystem(ctx, hostName)
	if err != nil {
		return nil, fmt.Errorf("host not found: %w", err)
	}

	// Create maintenance spec with defaults
	spec := &types.HostMaintenanceSpec{
		// Default to migrating VMs to other hosts
		VsanMode: &types.VsanHostDecommissionMode{
			ObjectAction: string(types.VsanHostDecommissionModeObjectActionEnsureObjectAccessibility),
		},
	}

	// Get host capabilities to determine VSAN options if needed
	var hostProps mo.HostSystem
	err = host.Properties(ctx, host.Reference(), []string{"capability", "config"}, &hostProps)
	if err != nil {
		return nil, fmt.Errorf("error getting host properties: %w", err)
	}

	// Set appropriate options based on host capabilities
	if hostProps.Config != nil && hostProps.Config.VsanHostConfig != nil && hostProps.Config.VsanHostConfig.Enabled != nil && *hostProps.Config.VsanHostConfig.Enabled {
		// Host has VSAN enabled, set appropriate mode
		spec.VsanMode = &types.VsanHostDecommissionMode{
			ObjectAction: string(types.VsanHostDecommissionModeObjectActionEnsureObjectAccessibility),
		}
	}

	return spec, nil
}
