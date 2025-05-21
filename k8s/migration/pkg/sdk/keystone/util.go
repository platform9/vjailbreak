package keystone

import (
	"errors"
	"fmt"
	"os"
	"strings"

	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	pcd "github.com/platform9/vjailbreak/k8s/migration/pkg/sdk/pcd"
)

func ParseCredentialsFromEnv() (Credentials, error) {
	creds := Credentials{
		Username: strings.TrimSpace(os.Getenv("KEYSTONE_USER")),
		Password: strings.TrimSpace(os.Getenv("KEYSTONE_PASSWORD")),
		Tenant:   strings.TrimSpace(os.Getenv("KEYSTONE_TENANT")),
		Region:   strings.TrimSpace(os.Getenv("KEYSTONE_REGION")),
	}
	if len(creds.Username) == 0 {
		return creds, errors.New("environment variable KEYSTONE_USER is required")
	}
	if len(creds.Password) == 0 {
		return creds, errors.New("environment variable KEYSTONE_PASSWORD is required")
	}
	if len(creds.Tenant) == 0 {
		return creds, errors.New("environment variable KEYSTONE_TENANT is required")
	}
	if len(creds.Region) == 0 {
		return creds, errors.New("environment variable KEYSTONE_REGION is required")
	}
	return creds, nil
}

func ParseCredentialsFromOpenstackCreds(openstackCreds vjailbreakv1alpha1.OpenStackCredsInfo) (Credentials, error) {
	return Credentials{
		Username: openstackCreds.Username,
		Password: openstackCreds.Password,
		Tenant:   openstackCreds.TenantName,
		Region:   openstackCreds.RegionName,
	}, nil
}

func CreateFromEnv() (Client, error) {
	// TODO support overriding it
	pcdInfo, err := pcd.ParseInfoFromEnv()
	if err != nil {
		return nil, err
	}
	return CreateFromDuInfo(pcdInfo), nil
}

func CreateFromOpenstackCreds(openstackCreds vjailbreakv1alpha1.OpenStackCredsInfo) (Client, error) {
	// TODO support overriding it
	pcdInfo, err := pcd.ParseInfoFromOpenstackCreds(openstackCreds)
	if err != nil {
		return nil, err
	}
	return CreateFromDuInfo(pcdInfo), nil
}

func CreateFromDuInfo(pcdInfo pcd.Info) Client {
	keystoneEndpoint := fmt.Sprintf("%s/keystone", pcdInfo.URL)
	return NewClient(keystoneEndpoint, pcdInfo.Insecure)
}
