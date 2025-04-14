package utils

import "fmt"

type MAASProvider struct {
}

func (m *MAASProvider) Connect() error {
	fmt.Println("Connecting to MAAS")
	return nil
}

func (m *MAASProvider) Disconnect() error {
	fmt.Println("Disconnecting from MAAS")
	return nil
}

func (m *MAASProvider) GetProviderBMStatus() (string, error) {
	fmt.Println("Getting provider BM status")
	return "", nil
}

func (m *MAASProvider) IsBMReady() bool {
	fmt.Println("Checking if BM is ready")
	return false
}

func (m *MAASProvider) IsBMRunning() bool {
	fmt.Println("Checking if BM is running")
	return false
}

func (m *MAASProvider) StartBM() error {
	fmt.Println("Starting BM")
	return nil
}

func (m *MAASProvider) StopBM() error {
	fmt.Println("Stopping BM")
	return nil
}

func (m *MAASProvider) SetBM2PXEBoot() error {
	fmt.Println("Setting BM to PXE boot")
	return nil
}

func (m *MAASProvider) ReclaimBM() error {
	fmt.Println("Reclaiming BM")
	return nil
}
