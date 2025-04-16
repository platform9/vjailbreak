package keystone

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/platform9/vjailbreak/k8s/migration/pkg/sdk/du"
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

func CreateFromEnv() (Client, error) {
	// TODO support overriding it
	duInfo, err := du.ParseInfoFromEnv()
	if err != nil {
		return nil, err
	}
	return CreateFromDuInfo(duInfo), nil
}

func CreateFromDuInfo(duInfo du.Info) Client {
	keystoneEndpoint := fmt.Sprintf("%s/keystone", duInfo.URL)
	return NewClient(keystoneEndpoint)
}
