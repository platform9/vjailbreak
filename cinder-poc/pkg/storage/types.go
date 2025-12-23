package storage

// StorageAccessInfo contains minimal OpenStack auth + region info
type StorageAccessInfo struct {
	Hostname   string // Keystone endpoint, e.g. https://openstack.example.com:5000/v3
	Username   string
	Password   string
	TenantName string
	DomainName string
	Region     string
	Insecure   bool
}

// Volume is a generic storage volume representation used by the provider
type Volume struct {
	ID           string
	Name         string
	SizeBytes    int64
	NAA          string
	SerialNumber string
}

// MappingContext is a generic key-value map returned by providers
type MappingContext map[string]interface{}
