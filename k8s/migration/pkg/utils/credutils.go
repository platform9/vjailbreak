package utils

import (
	"context"
	"encoding/base64"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// VMwareCredsFromSecret holds the actual credentials after decoding
type VMwareCredsFromSecret struct {
	Host     string
	Username string
	Password string
	Insecure bool
}

// OpenStackCredentials holds the actual credentials after decoding
type OpenStackCredentialsFromSecret struct {
	AuthURL    string
	DomainName string
	Username   string
	Password   string
	RegionName string
	TenantName string
	Insecure   bool
}

// getVMwareCredsFromSecret retrieves vCenter credentials from a secret
func GetVMwareCredsFromSecret(ctx context.Context, secretName string) (VMwareCredsFromSecret, error) {
	secret := &corev1.Secret{}

	// Get In cluster client
	c, err := GetInclusterClient()
	if err != nil {
		return VMwareCredsFromSecret{}, fmt.Errorf("failed to get in cluster client: %w", err)
	}

	if err := c.Get(ctx, client.ObjectKey{Namespace: "migration-system", Name: secretName}, secret); err != nil {
		return VMwareCredsFromSecret{}, fmt.Errorf("failed to get secret '%s': %w", secretName, err)
	}

	if secret.Data == nil {
		return VMwareCredsFromSecret{}, fmt.Errorf("no data in secret '%s'", secretName)
	}

	host, err := decodeSecretData(secret.Data["vcenterHost"])
	if err != nil {
		return VMwareCredsFromSecret{}, fmt.Errorf("failed to decode vcenterHost: %w", err)
	}
	username, err := decodeSecretData(secret.Data["vcenterUsername"])
	if err != nil {
		return VMwareCredsFromSecret{}, fmt.Errorf("failed to decode vcenterUsername: %w", err)
	}
	password, err := decodeSecretData(secret.Data["vcenterPassword"])
	if err != nil {
		return VMwareCredsFromSecret{}, fmt.Errorf("failed to decode vcenterPassword: %w", err)
	}
	insecure, err := decodeSecretData(secret.Data["VcenterInsecure"])
	if err != nil {
		return VMwareCredsFromSecret{}, fmt.Errorf("failed to decode VcenterInsecure: %w", err)
	}

	Insecure := false
	if insecure == "true" {
		Insecure = true
	} else {
		Insecure = false
	}

	if host == "" || username == "" || password == "" {
		return VMwareCredsFromSecret{}, fmt.Errorf("incomplete vCenter credentials in secret '%s'", secretName)
	}

	return VMwareCredsFromSecret{
		Host:     host,
		Username: username,
		Password: password,
		Insecure: Insecure,
	}, nil

}

// getOpenStackCreds retrieves and decodes the secret
func GetOpenstackCredsFromSecret(ctx context.Context, secretName string) (OpenStackCredentialsFromSecret, error) {
	secret := &corev1.Secret{}
	// Get In cluster client
	c, err := GetInclusterClient()
	if err != nil {
		return OpenStackCredentialsFromSecret{}, fmt.Errorf("failed to get in cluster client: %w", err)
	}
	if err := c.Get(ctx, client.ObjectKey{Namespace: "migration-system", Name: secretName}, secret); err != nil {
		return OpenStackCredentialsFromSecret{}, fmt.Errorf("failed to get secret: %w", err)
	}

	// Decode each field
	authURL, err := decodeSecretData(secret.Data["OS_AUTH_URL"])
	if err != nil {
		return OpenStackCredentialsFromSecret{}, fmt.Errorf("failed to decode OS_AUTH_URL: %w", err)
	}

	domainName, err := decodeSecretData(secret.Data["OS_DOMAIN_NAME"])
	if err != nil {
		return OpenStackCredentialsFromSecret{}, fmt.Errorf("failed to decode OS_DOMAIN_NAME: %w", err)
	}

	username, err := decodeSecretData(secret.Data["OS_USERNAME"])
	if err != nil {
		return OpenStackCredentialsFromSecret{}, fmt.Errorf("failed to decode OS_USERNAME: %w", err)
	}

	password, err := decodeSecretData(secret.Data["OS_PASSWORD"])
	if err != nil {
		return OpenStackCredentialsFromSecret{}, fmt.Errorf("failed to decode OS_PASSWORD: %w", err)
	}

	regionName, err := decodeSecretData(secret.Data["OS_REGION_NAME"])
	if err != nil {
		return OpenStackCredentialsFromSecret{}, fmt.Errorf("failed to decode OS_REGION_NAME: %w", err)
	}

	tenantName, err := decodeSecretData(secret.Data["OS_TENANT_NAME"])
	if err != nil {
		return OpenStackCredentialsFromSecret{}, fmt.Errorf("failed to decode OS_TENANT_NAME: %w", err)
	}

	insecureStr, err := decodeSecretData(secret.Data["OS_INSECURE"])
	if err != nil {
		return OpenStackCredentialsFromSecret{}, fmt.Errorf("failed to decode OS_INSECURE: %w", err)
	}

	insecure := insecureStr == "true"

	return OpenStackCredentialsFromSecret{
		AuthURL:    authURL,
		DomainName: domainName,
		Username:   username,
		Password:   password,
		RegionName: regionName,
		TenantName: tenantName,
		Insecure:   insecure,
	}, nil
}

// decodeSecretData decodes a base64-encoded secret
func decodeSecretData(data []byte) (string, error) {
	if data == nil {
		return "", fmt.Errorf("secret data is missing")
	}

	decoded, err := base64.StdEncoding.DecodeString(string(data))
	if err != nil {
		return "", fmt.Errorf("failed to decode secret: %w", err)
	}

	return string(decoded), nil
}
