package utils

import (
	"context"
	"fmt"

	"github.com/platform9/vjailbreak/k8s/migration/pkg/constants"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// VMwareCredentials holds the actual credentials after decoding
type VMwareCredentials struct {
	Host     string
	Username string
	Password string
	Insecure bool
}

// OpenStackCredentials holds the actual credentials after decoding
type OpenStackCredentials struct {
	AuthURL    string
	Username   string
	Password   string
	RegionName string
	TenantName string
	Insecure   bool
	DomainName string
}

// GetVMwareCredentials retrieves vCenter credentials from a secret
func GetVMwareCredentials(ctx context.Context, secretName string) (VMwareCredentials, error) {
	secret := &corev1.Secret{}

	// Get In cluster client
	c, err := GetInclusterClient()
	if err != nil {
		return VMwareCredentials{}, fmt.Errorf("failed to get in cluster client: %w", err)
	}

	if err := c.Get(ctx, client.ObjectKey{Namespace: constants.NamespaceMigrationSystem, Name: secretName}, secret); err != nil {
		return VMwareCredentials{}, fmt.Errorf("failed to get secret '%s': %w", secretName, err)
	}

	if secret.Data == nil {
		return VMwareCredentials{}, fmt.Errorf("no data in secret '%s'", secretName)
	}

	host := string(secret.Data["VCENTER_HOST"])
	username := string(secret.Data["VCENTER_USERNAME"])
	password := string(secret.Data["VCENTER_PASSWORD"])
	insecureStr := string(secret.Data["VCENTER_INSECURE"])

	if host == "" {
		return VMwareCredentials{}, fmt.Errorf("VCENTER_HOST is missing in secret '%s'", secretName)
	}
	if username == "" {
		return VMwareCredentials{}, fmt.Errorf("VCENTER_USERNAME is missing in secret '%s'", secretName)
	}
	if password == "" {
		return VMwareCredentials{}, fmt.Errorf("VCENTER_PASSWORD is missing in secret '%s'", secretName)
	}

	insecure := insecureStr == "true"

	return VMwareCredentials{
		Host:     host,
		Username: username,
		Password: password,
		Insecure: insecure,
	}, nil
}

// getOpenStackCreds retrieves and checks the secret
func GetOpenstackCredentials(ctx context.Context, secretName string) (OpenStackCredentials, error) {
	secret := &corev1.Secret{}
	// Get In cluster client
	c, err := GetInclusterClient()
	if err != nil {
		return OpenStackCredentials{}, fmt.Errorf("failed to get in cluster client: %w", err)
	}
	if err := c.Get(ctx, client.ObjectKey{Namespace: constants.NamespaceMigrationSystem, Name: secretName}, secret); err != nil {
		return OpenStackCredentials{}, fmt.Errorf("failed to get secret: %w", err)
	}

	// Extract and validate each field
	fields := map[string]string{
		"AuthURL":    string(secret.Data["OS_AUTH_URL"]),
		"DomainName": string(secret.Data["OS_DOMAIN_NAME"]),
		"Username":   string(secret.Data["OS_USERNAME"]),
		"Password":   string(secret.Data["OS_PASSWORD"]),
		"TenantName": string(secret.Data["OS_TENANT_NAME"]),
	}

	for key, value := range fields {
		if value == "" {
			return OpenStackCredentials{}, fmt.Errorf("%s is missing in secret '%s'", key, secretName)
		}
	}

	insecureStr := string(secret.Data["OS_INSECURE"])
	insecure := insecureStr == "true"

	return OpenStackCredentials{
		AuthURL:    fields["AuthURL"],
		DomainName: fields["DomainName"],
		Username:   fields["Username"],
		Password:   fields["Password"],
		RegionName: fields["RegionName"],
		TenantName: fields["TenantName"],
		Insecure:   insecure,
	}, nil
}
