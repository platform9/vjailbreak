package vcenter

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/platform9/vjailbreak/pkg/vpwned/openapiv3/proto/service/api"
	"github.com/platform9/vjailbreak/pkg/vpwned/sdk/targets"
	"github.com/sirupsen/logrus"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/view"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

type Vcenter struct {
	targets.Targets
}

type VMCenterAccessInfo struct {
	targets.AccessInfo
	Datacenter string
}

type VCenterOpts struct {
	key   string
	value string
}

func NewVCenterCreds(a targets.AccessInfo, opts ...VCenterOpts) VMCenterAccessInfo {
	creds := VMCenterAccessInfo{
		AccessInfo: a,
	}
	for _, opt := range opts {
		switch opt.key {
		case "datacenter":
			creds.Datacenter = opt.value
		}
	}
	return creds
}

func findPowerStatus(state types.VirtualMachinePowerState) api.PowerStatus {
	if strings.EqualFold(string(state), "poweredOn") {
		return api.PowerStatus_POWERED_ON
	} else if strings.EqualFold(string(state), "poweredOff") {
		return api.PowerStatus_POWERED_OFF
	} else if strings.EqualFold(string(state), "suspended") {
		return api.PowerStatus_POWERED_ON
	}
	return api.PowerStatus_POWER_STATE_UNKNOWN
}

func findBootDevice(devices []types.BaseVirtualMachineBootOptionsBootableDevice) string {
	for _, bootDevice := range devices {
		// Type switch to handle different device types
		switch device := bootDevice.(type) {
		case *types.VirtualMachineBootOptionsBootableCdromDevice:
			return "cdrom"
		case *types.VirtualMachineBootOptionsBootableDiskDevice:
			return "disk"
		case *types.VirtualMachineBootOptionsBootableEthernetDevice:
			return "network"
		default:
			logrus.Warnf("Unknown Boot Device Type: %T", device)
		}
	}
	return "unknown"
}

func (v *Vcenter) connect(a VMCenterAccessInfo) (*govmomi.Client, *find.Finder, error) {
	ctx := context.Background()
	logrus.Info("Connecting to vCenter...")
	// Parse vCenter URL
	u, err := url.Parse(fmt.Sprintf("https://%s/sdk", a.HostnameOrIP))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse vCenter URL: %v", err)
	}

	// Set credentials
	u.User = url.UserPassword(a.Username, a.Password)

	// Create vSphere client
	client, err := govmomi.NewClient(ctx, u, true)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect to vCenter: %v", err)
	}

	// Create finder
	finder := find.NewFinder(client.Client, true)

	// If VMCenterAccessInfo is provided and Datacenter is set, set the datacenter
	if a.Datacenter != "" {
		dc, err := finder.Datacenter(ctx, a.Datacenter)
		if err != nil {
			client.Logout(ctx)
			return nil, nil, fmt.Errorf("failed to find datacenter: %v", err)
		}
		finder.SetDatacenter(dc)
	}

	return client, finder, nil
}

func (v *Vcenter) ListVMs(a VMCenterAccessInfo) ([]targets.VMInfo, error) {
	ctx := context.Background()

	// Connect to vCenter
	client, _, err := v.connect(a)
	if err != nil {
		return nil, err
	}
	defer client.Logout(ctx)
	logrus.Info("Connected to vCenter")

	// Create container view of all virtual machines
	m := view.NewManager(client.Client)
	containerView, err := m.CreateContainerView(ctx, client.Client.ServiceContent.RootFolder, []string{"VirtualMachine"}, true)
	if err != nil {
		return nil, fmt.Errorf("failed to create container view: %v", err)
	}
	defer containerView.Destroy(ctx)

	// Retrieve all VMs
	var vms []mo.VirtualMachine
	err = containerView.Retrieve(ctx, []string{"VirtualMachine"}, []string{"summary", "runtime.powerState", "config.bootOptions"}, &vms)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve VMs: %v", err)
	}

	// Convert VMs to VMInfo
	vmInfos := make([]targets.VMInfo, len(vms))
	for i, vmRef := range vms {
		// Populate VM info
		vmInfos[i] = targets.VMInfo{
			Name:        vmRef.Summary.Config.Name,
			GuestOS:     vmRef.Summary.Config.GuestFullName,
			PowerStatus: findPowerStatus(vmRef.Runtime.PowerState),
			CPU:         int64(vmRef.Summary.Config.NumCpu),
			Memory:      int64(vmRef.Summary.Config.MemorySizeMB),
			IPv4Addr:    vmRef.Summary.Guest.IpAddress,
			IPv6Addr:    vmRef.Summary.Guest.IpAddress,
		}

		// Set boot device if available
		if vmRef.Config != nil && vmRef.Config.BootOptions != nil {
			vmInfos[i].BootDevice = findBootDevice(vmRef.Config.BootOptions.BootOrder)
		}
	}

	return vmInfos, nil
}

func (v *Vcenter) GetVM(a VMCenterAccessInfo, name string) (targets.VMInfo, error) {
	ctx := context.Background()

	// Connect to vCenter
	client, finder, err := v.connect(a)
	if err != nil {
		return targets.VMInfo{}, err
	}
	defer client.Logout(ctx)

	// Find specific VM
	vm, err := finder.VirtualMachine(ctx, name)
	if err != nil {
		return targets.VMInfo{}, fmt.Errorf("VM not found: %v", err)
	}

	// Populate VM info
	vmInfo := targets.VMInfo{
		Name: vm.Name(),
	}

	// Get power state
	powerState, err := vm.PowerState(ctx)
	if err == nil {
		vmInfo.PowerStatus = findPowerStatus(powerState)
	}

	return vmInfo, nil
}

func (v *Vcenter) ReclaimVM(a VMCenterAccessInfo, name string, args ...string) error {
	ctx := context.Background()

	// Connect to vCenter
	client, finder, err := v.connect(a)
	if err != nil {
		return err
	}
	defer client.Logout(ctx)

	// Find specific VM
	vm, err := finder.VirtualMachine(ctx, name)
	if err != nil {
		return fmt.Errorf("VM not found: %v", err)
	}

	// Power off VM if it's running
	powerState, err := vm.PowerState(ctx)
	if err != nil {
		return fmt.Errorf("failed to get VM power state: %v", err)
	}

	if powerState == types.VirtualMachinePowerStatePoweredOn {
		task, err := vm.PowerOff(ctx)
		if err != nil {
			return fmt.Errorf("failed to power off VM: %v", err)
		}

		// Wait for power off to complete
		if err := task.Wait(ctx); err != nil {
			return fmt.Errorf("error waiting for VM power off: %v", err)
		}
	}

	// Destroy/delete the VM
	task, err := vm.Destroy(ctx)
	if err != nil {
		return fmt.Errorf("failed to destroy VM: %v", err)
	}

	// Wait for destroy to complete
	if err := task.Wait(ctx); err != nil {
		return fmt.Errorf("error waiting for VM destruction: %v", err)
	}

	return nil
}
