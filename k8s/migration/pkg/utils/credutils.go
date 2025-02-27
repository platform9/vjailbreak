package utils

import (
	"context"
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

// OpenStackCredentialsFromSecret holds the actual credentials after decoding
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

	host, ok := secret.Data["VCENTER_HOST"]
	if !ok {
		return VMwareCredsFromSecret{}, fmt.Errorf("missing VCENTER_HOST in secret '%s'", secretName)
	}
	username, ok := secret.Data["VCENTER_USERNAME"]
	if !ok {
		return VMwareCredsFromSecret{}, fmt.Errorf("missing VCENTER_USERNAME in secret '%s'", secretName)
	}
	password, ok := secret.Data["VCENTER_PASSWORD"]
	if !ok {
		return VMwareCredsFromSecret{}, fmt.Errorf("missing VCENTER_PASSWORD in secret '%s'", secretName)
	}
	insecureStr, ok := secret.Data["VCENTER_INSECURE"]
	if !ok {
		return VMwareCredsFromSecret{}, fmt.Errorf("missing VCENTER_INSECURE in secret '%s'", secretName)
	}

	insecure := string(insecureStr) == "true"

	return VMwareCredsFromSecret{
		Host:     string(host),
		Username: string(username),
		Password: string(password),
		Insecure: insecure,
	}, nil
}

// getOpenStackCredsFromSecret retrieves and decodes the secret
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

	if secret.Data == nil {
		return OpenStackCredentialsFromSecret{}, fmt.Errorf("no data in secret '%s'", secretName)
	}

	fields := map[string]*string{
		"OS_AUTH_URL":    new(string),
		"OS_DOMAIN_NAME": new(string),
		"OS_USERNAME":    new(string),
		"OS_PASSWORD":    new(string),
		"OS_REGION_NAME": new(string),
		"OS_TENANT_NAME": new(string),
		"OS_INSECURE":    new(string),
	}

	for key, ptr := range fields {
		value, ok := secret.Data[key]
		if !ok {
			return OpenStackCredentialsFromSecret{}, fmt.Errorf("missing %s in secret '%s'", key, secretName)
		}
		*ptr = string(value)
	}

	insecure := *fields["OS_INSECURE"] == "true"

	return OpenStackCredentialsFromSecret{
		AuthURL:    *fields["OS_AUTH_URL"],
		DomainName: *fields["OS_DOMAIN_NAME"],
		Username:   *fields["OS_USERNAME"],
		Password:   *fields["OS_PASSWORD"],
		RegionName: *fields["OS_REGION_NAME"],
		TenantName: *fields["OS_TENANT_NAME"],
		Insecure:   insecure,
	}, nil
}
