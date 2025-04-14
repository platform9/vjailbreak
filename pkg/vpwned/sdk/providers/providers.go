package providers

import (
	"context"
	"errors"

	"github.com/platform9/vjailbreak/pkg/vpwned/openapiv3/proto/service/api"
)

var providers map[string]BMCProvider = make(map[string]BMCProvider)

type BMCProvider interface {
	//BM provisoner functions
	//a function to connect to the underlying provider
	Connect(accessInfo BMAccessInfo) error
	//a function to disconnect from the underlying provider
	Disconnect() error
	//return the current status in the underlying BM provisioner
	GetProviderBMStatus() (string, error)
	// a function to check if the BM is in ready state
	// this state could be a combination of different states in the
	// underlying BM provisioner
	IsBMReady() bool
	// a function to check if the BM is in running state
	IsBMRunning() bool
	//Power Functions
	//start the BM
	StartBM() error
	//Stop the BM
	StopBM() error
	//Set to PXE boot on next reboot
	SetBM2PXEBoot() error
	//Reclaim functions
	//Reclaim the VM such that we erase and resuse the VM as a PCD Host
	ReclaimBM() error
	// Identify the provider
	WhoAmI() string
	// List resources
	ListResources(ctx context.Context) ([]api.MachineInfo, error)
	// Set resource power
	SetResourcePower(ctx context.Context, resourceID string, action api.PowerStatus) error
}

type BMAccessInfo struct {
	Username    string
	Password    string
	APIKey      string
	BaseURL     string
	UseInsecure bool
}

func RegisterProvider(name string, provider BMCProvider) {
	providers[name] = provider
}

func DeleteProvider(name string) {
	delete(providers, name)
}

func GetProviders() []string {
	var names []string
	for name := range providers {
		names = append(names, name)
	}
	return names
}

func GetProvider(name string) (BMCProvider, error) {
	provider, ok := providers[name]
	if !ok {
		return nil, errors.New("provider not found")
	}
	return provider, nil
}
