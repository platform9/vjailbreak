package base

import (
	"errors"

	"github.com/platform9/vjailbreak/pkg/vpwned/sdk/providers"
)

const (
	BaseProviderName = "base"
)

// BaseProvider is a base implementation of the BMCProvider interface
// that provides common functionality for all providers

type BaseProvider struct {
	providers.BMCProvider
}

func (p *BaseProvider) Connect(auth providers.BMAccessInfo) error {
	return errors.New("not implemented")
}

func (p *BaseProvider) Disconnect() error {
	return errors.New("not implemented")
}

func (p *BaseProvider) GetProviderBMStatus() (string, error) {
	return "", errors.New("not implemented")
}

func (p *BaseProvider) IsBMReady() bool {
	return false
}

func (p *BaseProvider) IsBMRunning() bool {
	return false
}

func (p *BaseProvider) StartBM() error {
	return errors.New("not implemented")
}

func (p *BaseProvider) StopBM() error {
	return errors.New("not implemented")
}

func (p *BaseProvider) SetBM2PXEBoot() error {
	return errors.New("not implemented")
}

func (p *BaseProvider) ReclaimBM() error {
	return errors.New("not implemented")
}

func (p *BaseProvider) WhoAmI() string {
	return BaseProviderName
}

func init() {
	providers.RegisterProvider(BaseProviderName, &BaseProvider{})
}
