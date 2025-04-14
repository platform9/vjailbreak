package maas

import (
	"context"
	"errors"
	"fmt"
	"strings"

	gomaasclient "github.com/canonical/gomaasclient/client"
	"github.com/canonical/gomaasclient/entity"
	"github.com/platform9/vjailbreak/pkg/vpwned/openapiv3/proto/service/api"
	"github.com/platform9/vjailbreak/pkg/vpwned/sdk/providers"
	"github.com/platform9/vjailbreak/pkg/vpwned/sdk/providers/base"
	"github.com/sirupsen/logrus"
)

const (
	MaasProviderName = "maas"
)

// MaasAccessInfo contains credentials and connection details for MaaS
type MaasAccessInfo struct {
	APIKey      string
	BaseURL     string
	UseInsecure bool
}

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
	case api.PowerStatus_POWERING_OFF:
		_, errs = m.Client.Machine.PowerOff(systemID, &entity.MachinePowerOffParams{})
		logrus.Infof("Machine %s powering off", systemID)
	case api.PowerStatus_POWERING_ON:
		_, errs = m.Client.Machine.PowerOn(systemID, &entity.MachinePowerOnParams{})
	default:
		return fmt.Errorf("unsupported power action: %v", action)
	}

	if errs != nil {
		logrus.Errorf("Failed to change power state: %v", errs)
		return errs
	}

	return nil
}

// MaasProvider implements the Provider interface for MaaS
type MaasProvider struct {
	base.BaseProvider
	client *MaasClient
}

// Connect establishes a connection to the MaaS server
func (p *MaasProvider) Connect(auth providers.BMAccessInfo) error {
	accessInfo := MaasAccessInfo{
		BaseURL:     auth.BaseURL,
		APIKey:      auth.APIKey,
		UseInsecure: auth.UseInsecure,
	}
	// Create MaaS client
	client, err := NewMaasClient(accessInfo)
	if err != nil {
		return err
	}
	p.client = client
	return nil
}

// ListResources retrieves a list of machines
func (p *MaasProvider) ListResources(ctx context.Context) ([]api.MachineInfo, error) {
	return p.client.ListMachines(ctx)
}

// SetResourcePower changes the power state of a machine
func (p *MaasProvider) SetResourcePower(ctx context.Context, resourceID string, action api.PowerStatus) error {
	return p.client.SetMachinePower(ctx, resourceID, action)
}

func (p *MaasProvider) WhoAmI() string {
	return MaasProviderName
}

func init() {
	providers.RegisterProvider(MaasProviderName, &MaasProvider{BaseProvider: base.BaseProvider{}, client: nil})
}
