package utils

import "errors"

var providers map[string]BMCProvider = make(map[string]BMCProvider)

type BMCProvider interface {
	//BM provisoner functions
	//a function to connect to the underlying provider
	Connect() error
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
