package maas

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	gomaasclient "github.com/canonical/gomaasclient/client"
	"github.com/canonical/gomaasclient/entity"
	"github.com/platform9/vjailbreak/pkg/vpwned/openapiv3/proto/service/api"
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

func (m *MaasClient) Reclaim(ctx context.Context, systemID string, cloudInitScript string, eraseDisk bool) error {
	if m.Client == nil {
		return errors.New("reclaim: client not initialized")
	}

	// Release the machine
	logrus.Infof("%s Releasing machine %s", ctx, systemID)
	_, err := m.Client.Machine.Release(systemID, &entity.MachineReleaseParams{
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
		DistroSeries: "",
		HWEKernel:    "",
	})
	if err != nil {
		logrus.Errorf("Failed to deploy machine: %v", err)
		return err
	}

	return nil
}

// TODO:
// Check if we can get PowerState Options and if they are IPMI
// we should try using goipmi to set the bootdev to PXE
// this would trigger MaaS to boot an ephemeral image on next reboot for this host
// then we can move to Deploy state for the host.
func (m *MaasClient) SetMachine2PXEBoot(ctx context.Context, systemID string) error {
	if m.Client == nil {
		return errors.New("client not initialized")
	}
	machine, err := m.Client.Machine.Get(systemID)
	if err != nil {
		logrus.Errorf("Failed to get machine: %v", err)
		return err
	}
	b, err := json.MarshalIndent(machine, "    ", "  ")
	if err != nil {
		logrus.Errorf("Failed to marshal machine: %v", err)
		return err
	}
	fmt.Println(string(b))
	return nil
}
