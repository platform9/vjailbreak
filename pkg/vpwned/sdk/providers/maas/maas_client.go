package maas

import (
	"context"
	"errors"
	"fmt"
	"strings"

	ipmi "github.com/bougou/go-ipmi"
	gomaasclient "github.com/canonical/gomaasclient/client"
	"github.com/canonical/gomaasclient/entity"
	api "github.com/platform9/vjailbreak/pkg/vpwned/api/proto/v1/service"
	"github.com/sirupsen/logrus"
)

// MaasClient represents a client for interacting with MaaS API
type MaasClient struct {
	BaseURL string
	ApiKey  string
	Client  *gomaasclient.Client
}

// NewMaasClient creates a new MaaS API client
func NewMaasClient(accessInfo MaasAccessInfo) (*MaasClient, error) {
	if !strings.Contains(accessInfo.BaseURL, "MAAS") {
		return nil, errors.New("invalid base URL")
	}
	client := &MaasClient{
		BaseURL: strings.TrimRight(accessInfo.BaseURL, "/"),
		ApiKey:  accessInfo.APIKey,
	}
	c, err := gomaasclient.GetClient(client.BaseURL, client.ApiKey, "2.0")
	if err != nil {
		logrus.Errorf("Failed to create MaaS client: %v", err)
		return nil, err
	}
	client.Client = c
	return client, nil
}

// ListMachines retrieves a list of machines from MaaS
func (m *MaasClient) ListMachines(ctx context.Context) ([]api.MachineInfo, error) {
	if m.Client == nil {
		return nil, errors.New("client not initialized")
	}
	machines, err := m.Client.Machines.Get(&entity.MachinesParams{})
	if err != nil {
		logrus.Errorf("Failed to list machines: %v", err)
		return nil, err
	}
	var result []api.MachineInfo
	result = make([]api.MachineInfo, len(machines))
	for i, v := range machines {
		result[i] = api.MachineInfo{
			Id:              v.SystemID,
			Fqdn:            v.FQDN,
			Os:              v.OSystem,
			PowerState:      v.PowerState,
			Hostname:        v.Hostname,
			Architecture:    v.Architecture,
			Memory:          fmt.Sprintf("%d", v.Memory),
			CpuCount:        fmt.Sprintf("%d", v.CPUCount),
			CpuSpeed:        fmt.Sprintf("%d", v.CPUSpeed),
			BootDiskSize:    fmt.Sprintf("%d", v.BootDisk.Size),
			Status:          v.StatusName,
			StatusMessage:   v.StatusMessage,
			StatusAction:    v.StatusAction,
			Description:     v.Description,
			Domain:          v.Domain.Name,
			Zone:            v.Zone.Name,
			Pool:            v.Pool.Name,
			TagNames:        strings.Join(v.TagNames, ","),
			Netboot:         v.Netboot,
			EphemeralDeploy: v.EphemeralDeploy,
			PowerType:       v.PowerType,
			BiosBootMethod:  v.BiosBootMethod,
		}
	}
	return result, nil
}

// SetMachinePower changes the power state of a machine
func (m *MaasClient) SetMachinePower(ctx context.Context, systemID string, action api.PowerStatus) error {
	if m.Client == nil {
		return errors.New("client not initialized")
	}
	_, err := m.Client.Machine.Get(systemID)
	if err != nil {
		logrus.Errorf("Failed to get machine: %v", err)
		return err
	}

	// Determine the power action
	var errs error
	errs = nil
	switch action {
	case api.PowerStatus_POWERED_ON:
		_, errs = m.Client.Machine.PowerOn(systemID, &entity.MachinePowerOnParams{})
		logrus.Infof("Machine %s powered on", systemID)
	case api.PowerStatus_POWERED_OFF:
		_, errs = m.Client.Machine.PowerOff(systemID, &entity.MachinePowerOffParams{})
		logrus.Infof("Machine %s powered off", systemID)
	default:
		return fmt.Errorf("unsupported power action: %v", action)
	}

	if errs != nil {
		logrus.Errorf("Failed to change power state: %v", errs)
		return errs
	}

	return nil
}

// Reclaim does the following:
// Take a cloud_init script that will be passed for deploying
// by default we will not erase disk, but an option is provided for the same.
// 1. Releases the Machine
// 2. Deploys the machine again with the parameters

func (m *MaasClient) Reclaim(ctx context.Context, req api.ReclaimBMRequest) error {
	if m.Client == nil {
		return errors.New("reclaim: client not initialized")
	}
	systemID := req.ResourceId
	eraseDisk := req.EraseDisk
	cloudInitScript := req.UserData
	bootSource := req.BootSource
	var err error
	machine, err := m.Client.Machine.Get(systemID)
	if err != nil {
		logrus.Errorf("Failed to get machine: %v", err)
		return err
	}
	con_interface := ipmi.InterfaceLanplus
	switch req.IpmiInterface.(type) {
	case *api.ReclaimBMRequest_Lan:
		con_interface = ipmi.InterfaceLan
	case *api.ReclaimBMRequest_Lanplus:
		con_interface = ipmi.InterfaceLanplus
	case *api.ReclaimBMRequest_OpenIpmi:
		con_interface = ipmi.InterfaceOpen
	case *api.ReclaimBMRequest_Tool:
		con_interface = ipmi.InterfaceTool
	}
	//Set machine to PXE Boot
	logrus.Infof("Setting %s to PXE boot over %s", machine.Hostname, con_interface)
	err = m.SetMachine2PXEBoot(ctx, systemID, req.PowerCycle, con_interface)
	if err != nil {
		logrus.Errorf("%s Failed to set machine to PXE boot: %v", ctx, err)
		return err
	}
	// Release the machine
	logrus.Infof("%s Releasing machine %s", ctx, systemID)
	_, err = m.Client.Machine.Release(systemID, &entity.MachineReleaseParams{
		Comment: "vJailbreak: Releasing machine for re-deployment",
		Erase:   eraseDisk,
	})
	if err != nil {
		logrus.Errorf("%s Failed to release machine: %v", ctx, err)
		return err
	}

	// Deploy the machine again
	logrus.Infof("%s Deploying machine %s", ctx, systemID)
	_, err = m.Client.Machine.Deploy(systemID, &entity.MachineDeployParams{
		UserData:     cloudInitScript,
		DistroSeries: bootSource.Release,
	})
	if err != nil {
		logrus.Errorf("Failed to deploy machine: %v", err)
		return err
	}

	return nil
}

func (m *MaasClient) ListBootSource(ctx context.Context) ([]api.BootsourceSelections, error) {
	if m.Client == nil {
		return nil, errors.New("client not initialized")
	}
	// Get boot sources
	bootSources, err := m.Client.BootSources.Get()
	if err != nil {
		logrus.Errorf("cannot get boot sources, err: %v", err)
		return nil, err
	}
	bootSource_ids := make(map[int]int)
	// Get Boot source using the ID
	for _, v := range bootSources {
		bs, err := m.Client.BootSource.Get(v.ID)
		if err != nil {
			logrus.Errorf("cannot get boot source, err: %v", err)
			return nil, err
		}
		bootSource_ids[v.ID] = bs.ID
	}

	var bootSourcesList []api.BootsourceSelections
	// Get boot source selections for that ID
	for _, v := range bootSource_ids {
		bootSourceSelections, err := m.Client.BootSourceSelections.Get(v)
		if err != nil {
			logrus.Errorf("cannot get boot source selections, err: %v", err)
			return nil, err
		}

		for _, v := range bootSourceSelections {
			bs, err := m.Client.BootSourceSelection.Get(v.BootSourceID, v.ID)
			if err != nil {
				logrus.Errorf("cannot get boot source, err: %v", err)
				return nil, err
			}
			bootSourcesList = append(bootSourcesList, api.BootsourceSelections{
				OS:           bs.OS,
				Release:      bs.Release,
				ResourceURI:  bs.ResourceURI,
				Arches:       bs.Arches,
				Subarches:    bs.Subarches,
				Labels:       bs.Labels,
				ID:           int32(v.ID),
				BootSourceID: int32(v.BootSourceID),
			})
		}
	}
	return bootSourcesList, nil
}

// TODO:
// Check if we can get PowerState Options and if they are IPMI
// we should try using goipmi to set the bootdev to PXE
// this would trigger MaaS to boot an ephemeral image on next reboot for this host
// then we can move to Deploy state for the host.
func (m *MaasClient) SetMachine2PXEBoot(ctx context.Context, systemID string, power_cycle bool, ipmi_interface ipmi.Interface) error {
	if m.Client == nil {
		return errors.New("client not initialized")
	}
	machine, err := m.Client.Machine.Get(systemID)
	if err != nil {
		logrus.Errorf("Failed to get machine: %v", err)
		return err
	}
	if !strings.EqualFold(machine.PowerType, "ipmi") {
		logrus.Errorf("Machine %s does not support IPMI power type", systemID)
		return errors.New("machine does not support IPMI power type")
	}

	// Extract IPMI connection details from machine's power parameters
	powerParams, err := m.Client.Machine.GetPowerParameters(systemID)
	if err != nil {
		logrus.Errorf("Failed to get power parameters: %v", err)
		return err
	}
	host := powerParams["power_address"]
	username := powerParams["power_user"]
	password := powerParams["power_pass"]
	if host == nil || username == nil || password == nil {
		logrus.Errorf("Failed to get power parameters: %v", err)
		return errors.New("failed to get power parameters")
	}
	// Configure IPMI connection
	config, err := ipmi.NewClient(host.(string), 623, username.(string), password.(string))
	if err != nil {
		logrus.Errorf("Failed to create IPMI client: %v", err)
		return err
	}
	config.WithInterface(ipmi_interface)

	// Open IPMI connection
	err = config.Connect(ctx)
	if err != nil {
		logrus.Errorf("Failed to open IPMI connection: %v", err)
		return err
	}
	defer config.Close(ctx)

	// Set boot device to PXE
	bootDevice := ipmi.BootDeviceSelectorForcePXE
	boot_type := ipmi.BIOSBootTypeLegacy
	if machine.BiosBootMethod == "efi" {
		boot_type = ipmi.BIOSBootTypeEFI
	}

	logrus.Debugf("IPMI connection details: host=%s, username=%s, ipmi_interface=%s, boot_type=%s", host, username, ipmi_interface, boot_type)

	config.SetBootDevice(ctx, bootDevice, boot_type, false)
	if err != nil {
		logrus.Errorf("Failed to set boot device to PXE: %v", err)
		return err
	}

	// Power cycle the machine to apply PXE boot based on the flag passed in [optional]
	powerState, err := config.GetChassisStatus(ctx)
	if err != nil {
		logrus.Errorf("Failed to get chassis status: %v", err)
		return err
	}
	logrus.Infof("current power state for %s is %s", systemID, powerState.ChassisIdentifyState)

	//If machine is on, perform a power reset
	if power_cycle && powerState.PowerIsOn {
		_, err = config.ChassisControl(ctx, ipmi.ChassisControlPowerCycle)
		if err != nil {
			logrus.Errorf("Failed to power cycle machine: %v", err)
			return err
		}
	}
	logrus.Infof("Successfully set machine %s to PXE boot", systemID)
	return nil
}
