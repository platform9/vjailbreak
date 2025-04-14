package utils

import (
	"context"

	"github.com/pkg/errors"
	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func InitBMProvisioner(ctx context.Context, bmType vjailbreakv1alpha1.BMCProviderName) (BMCProvider, error) {
	switch bmType {
	case vjailbreakv1alpha1.MAASProvider:
		return &MAASProvider{}, nil
	default:
		return nil, errors.New("invalid BMC provider")
	}
}

func ConvertESXiToPCDHost(ctx context.Context, client client.Client, esxiName string, bmProvider BMCProvider) error {
	err := bmProvider.SetBM2PXEBoot()
	if err != nil {
		return errors.Wrap(err, "failed to set PXE boot")
	}

	err = bmProvider.ReclaimBM()
	if err != nil {
		return errors.Wrap(err, "failed to reclaim BMC")
	}
	return nil
}

func GetMachineID(ctx context.Context, client client.Client, esxiName string, bmProvider BMCProvider) (string, error) {
	return "", nil
}

func GetMachineName(ctx context.Context, client client.Client, esxiName string, bmProvider BMCProvider) (string, error) {
	return "", nil
}

func GetMachineType(ctx context.Context, client client.Client, esxiName string, bmProvider BMCProvider) (string, error) {
	return "", nil
}
