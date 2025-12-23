package cinder

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/blockstorage/v3/volumes"
	"github.com/platform9/vjailbreak/cinder-poc/pkg/storage"
	"k8s.io/klog/v2"
)

// CinderStorageProvider implements a thin generic provider backed by OpenStack Cinder
type CinderStorageProvider struct {
	client      *gophercloud.ServiceClient
	accessInfo  storage.StorageAccessInfo
	isConnected bool
}

func New(provider storage.StorageAccessInfo) *CinderStorageProvider {
	return &CinderStorageProvider{
		accessInfo: provider,
	}
}

func (c *CinderStorageProvider) Connect(ctx context.Context) error {
	// Create auth options
	opts := gophercloud.AuthOptions{
		IdentityEndpoint: c.accessInfo.Hostname,
		Username:         c.accessInfo.Username,
		Password:         c.accessInfo.Password,
		TenantName:       c.accessInfo.TenantName,
		DomainName:       c.accessInfo.DomainName,
		AllowReauth:      true,
		Scope: &gophercloud.AuthScope{
			ProjectName: c.accessInfo.TenantName,
			DomainName:  c.accessInfo.DomainName, // project domain
		},
	}

	// Create provider client with custom TLS config if needed
	var providerClient *gophercloud.ProviderClient
	var err error

	if c.accessInfo.Insecure {
		// Create custom HTTP client with TLS verification disabled
		transport := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		httpClient := &http.Client{Transport: transport}

		// Create provider client with custom HTTP client
		providerClient, err = openstack.NewClient(c.accessInfo.Hostname)
		if err != nil {
			return fmt.Errorf("failed to create provider client: %w", err)
		}
		providerClient.HTTPClient = *httpClient

		// Authenticate
		err = openstack.Authenticate(providerClient, opts)
		if err != nil {
			return fmt.Errorf("authentication failed: %w", err)
		}
		klog.Infof("TLS verification disabled (insecure mode)")
	} else {
		// Use standard authenticated client
		providerClient, err = openstack.AuthenticatedClient(opts)
		if err != nil {
			return fmt.Errorf("failed to create OpenStack provider client: %w", err)
		}
	}

	endpointOpts := gophercloud.EndpointOpts{
		Region: c.accessInfo.Region,
	}
	cinderClient, err := openstack.NewBlockStorageV3(providerClient, endpointOpts)
	if err != nil {
		return fmt.Errorf("failed to create cinder client: %w", err)
	}

	c.client = cinderClient
	c.isConnected = true
	klog.Infof("Connected to Cinder at %s (region=%s)", c.accessInfo.Hostname, c.accessInfo.Region)
	return nil
}

func (c *CinderStorageProvider) Disconnect(ctx context.Context) error {
	c.isConnected = false
	c.client = nil
	return nil
}

func (c *CinderStorageProvider) ValidateCredentials(ctx context.Context) error {
	if !c.isConnected {
		return fmt.Errorf("not connected")
	}
	// simple smoke test: list a page of volumes
	_, err := volumes.List(c.client, volumes.ListOpts{Limit: 1}).AllPages()
	if err != nil {
		return fmt.Errorf("validation list failed: %w", err)
	}
	return nil
}

// CreateVolume accepts volumeName, sizeBytes and optional volumeType
func (c *CinderStorageProvider) CreateVolume(ctx context.Context, volumeName string, sizeBytes int64, volumeType string) (*storage.Volume, error) {
	if !c.isConnected {
		return nil, fmt.Errorf("not connected")
	}
	// convert bytes -> GB (round up)
	gb := int((sizeBytes + (1024*1024*1024 - 1)) / (1024 * 1024 * 1024))
	createOpts := volumes.CreateOpts{
		Name: volumeName,
		Size: gb,
	}
	if volumeType != "" {
		createOpts.VolumeType = volumeType
	}

	vol, err := volumes.Create(c.client, createOpts).Extract()
	if err != nil {
		return nil, fmt.Errorf("create volume failed: %w", err)
	}

	// wait for available
	if err := c.waitForVolumeStatus(vol.ID, "available", 3*time.Minute); err != nil {
		return nil, fmt.Errorf("volume %s did not become available: %w", vol.ID, err)
	}

	return &storage.Volume{
		ID:        vol.ID,
		Name:      vol.Name,
		SizeBytes: int64(vol.Size) * 1024 * 1024 * 1024,
	}, nil
}

func (c *CinderStorageProvider) DeleteVolume(ctx context.Context, volumeName string) error {
	vol, err := c.findVolumeByName(volumeName)
	if err != nil {
		return err
	}
	if err := volumes.Delete(c.client, vol.ID, nil).ExtractErr(); err != nil {
		return fmt.Errorf("delete volume: %w", err)
	}
	// optional: wait for deletion - omitted for brevity
	return nil
}

func (c *CinderStorageProvider) GetVolumeInfo(ctx context.Context, volumeName string) (storage.Volume, error) {
	vol, err := c.findVolumeByName(volumeName)
	if err != nil {
		return storage.Volume{}, err
	}
	v := storage.Volume{
		ID:        vol.ID,
		Name:      vol.Name,
		SizeBytes: int64(vol.Size) * 1024 * 1024 * 1024,
		NAA:       c.getVolumeNAA(vol),
	}
	return v, nil
}

func (c *CinderStorageProvider) ListAllVolumes(ctx context.Context) ([]storage.Volume, error) {
	pages, err := volumes.List(c.client, volumes.ListOpts{}).AllPages()
	if err != nil {
		return nil, fmt.Errorf("list volumes: %w", err)
	}
	all, err := volumes.ExtractVolumes(pages)
	if err != nil {
		return nil, fmt.Errorf("extract volumes: %w", err)
	}
	out := make([]storage.Volume, 0, len(all))
	for _, vv := range all {
		out = append(out, storage.Volume{
			ID:        vv.ID,
			Name:      vv.Name,
			SizeBytes: int64(vv.Size) * 1024 * 1024 * 1024,
			NAA:       c.getVolumeNAA(&vv),
		})
	}
	return out, nil
}

func (c *CinderStorageProvider) GetAllVolumeNAAs(ctx context.Context) ([]string, error) {
	vols, err := c.ListAllVolumes(ctx)
	if err != nil {
		return nil, err
	}
	var naas []string
	for _, v := range vols {
		if v.NAA != "" {
			naas = append(naas, v.NAA)
		}
	}
	return naas, nil
}

// CreateOrUpdateInitiatorGroup: store iqns in context; optionally pre-create via temp volume
func (c *CinderStorageProvider) CreateOrUpdateInitiatorGroup(ctx context.Context, initiatorGroupName string, hbaIdentifiers []string, precreate bool) (storage.MappingContext, error) {
	klog.Infof("CreateOrUpdateInitiatorGroup %s iqns=%v precreate=%v", initiatorGroupName, hbaIdentifiers, precreate)
	if len(hbaIdentifiers) == 0 {
		return nil, fmt.Errorf("no initiators provided")
	}
	// store in context
	ctxMap := storage.MappingContext{
		"initiator_group_name": initiatorGroupName,
		"iqns":                 hbaIdentifiers,
		"created_hosts":        []string{},
	}
	if precreate {
		// create minimal temp volume and call initialize_connection for each IQN to force array host creation
		for idx, iqn := range hbaIdentifiers {
			tempName := fmt.Sprintf("vj-init-%s-%d", initiatorGroupName, idx)
			klog.Infof("Creating temp vol %s to precreate host for IQN %s", tempName, iqn)
			temp, err := c.CreateVolume(ctx, tempName, 1*1024*1024*1024, "") // 1GB
			if err != nil {
				klog.Warningf("temp create failed: %v", err)
				continue
			}
			connector := map[string]interface{}{
				"initiator": iqn,
				"host":      initiatorGroupName,
				"platform":  "VMware_ESXi",
				"os_type":   "vmware",
			}
			if _, err := c.initializeConnection(temp.ID, connector); err != nil {
				klog.Warningf("init connection failed for %s: %v", iqn, err)
			} else {
				created := ctxMap["created_hosts"].([]string)
				created = append(created, initiatorGroupName)
				ctxMap["created_hosts"] = created
			}
			// try to terminate and delete
			_ = c.terminateConnection(temp.ID, connector)
			_ = c.DeleteVolume(ctx, temp.Name)
		}
	}
	return ctxMap, nil
}

// MapVolumeToGroup uses initialize_connection to map the named volume to each IQN (creates host & mapping)
func (c *CinderStorageProvider) MapVolumeToGroup(ctx context.Context, initiatorGroupName string, targetVolume storage.Volume, contextMap storage.MappingContext) (storage.Volume, error) {
	iqnsVal, ok := contextMap["iqns"]
	if !ok {
		return storage.Volume{}, fmt.Errorf("iqns missing in mapping context")
	}
	iqns, ok := iqnsVal.([]string)
	if !ok || len(iqns) == 0 {
		return storage.Volume{}, fmt.Errorf("invalid iqns")
	}
	// find volume
	volPage, err := volumes.List(c.client, volumes.ListOpts{Name: targetVolume.Name}).AllPages()
	if err != nil {
		return storage.Volume{}, fmt.Errorf("find volume: %w", err)
	}
	vols, _ := volumes.ExtractVolumes(volPage)
	if len(vols) == 0 {
		return storage.Volume{}, fmt.Errorf("volume %s not found", targetVolume.Name)
	}
	vol := vols[0]

	for _, iqn := range iqns {
		connector := map[string]interface{}{
			"initiator": iqn,
			"host":      initiatorGroupName,
			"platform":  "VMware_ESXi",
			"os_type":   "vmware",
		}
		if _, err := c.initializeConnection(vol.ID, connector); err != nil {
			return storage.Volume{}, fmt.Errorf("initialize connection failed for %s: %w", iqn, err)
		}
		klog.Infof("Initialized connection for volume %s to iqn %s", vol.Name, iqn)
	}
	// update returned volume info
	targetVolume.NAA = c.getVolumeNAA(&vol)
	targetVolume.ID = vol.ID
	return targetVolume, nil
}

func (c *CinderStorageProvider) UnmapVolumeFromGroup(ctx context.Context, initiatorGroupName string, targetVolume storage.Volume, contextMap storage.MappingContext) error {
	iqnsVal, ok := contextMap["iqns"]
	if !ok {
		return nil
	}
	iqns, ok := iqnsVal.([]string)
	if !ok || len(iqns) == 0 {
		return nil
	}
	vol, err := c.findVolumeByName(targetVolume.Name)
	if err != nil {
		return err
	}
	for _, iqn := range iqns {
		connector := map[string]interface{}{
			"initiator": iqn,
			"host":      initiatorGroupName,
		}
		if err := c.terminateConnection(vol.ID, connector); err != nil {
			klog.Warningf("terminate failed for %s: %v", iqn, err)
		}
	}
	return nil
}

func (c *CinderStorageProvider) GetMappedHosts(ctx context.Context, targetVolume storage.Volume) ([]string, error) {
	vol, err := c.findVolumeByName(targetVolume.Name)
	if err != nil {
		return nil, err
	}
	var hosts []string
	for _, att := range vol.Attachments {
		if att.HostName != "" {
			hosts = append(hosts, att.HostName)
		}
	}
	return hosts, nil
}

func (c *CinderStorageProvider) ResolveCinderVolumeToLUN(ctx context.Context, volumeID string) (storage.Volume, error) {
	vol, err := volumes.Get(c.client, volumeID).Extract()
	if err != nil {
		return storage.Volume{}, fmt.Errorf("get volume: %w", err)
	}
	return storage.Volume{
		ID:           vol.ID,
		Name:         vol.Name,
		SizeBytes:    int64(vol.Size) * 1024 * 1024 * 1024,
		NAA:          c.getVolumeNAA(vol),
		SerialNumber: c.getVolumeSerial(vol),
	}, nil
}

func (c *CinderStorageProvider) WhoAmI() string {
	return "cinder"
}

// ---------- helpers ----------

func (c *CinderStorageProvider) findVolumeByName(name string) (*volumes.Volume, error) {
	page, err := volumes.List(c.client, volumes.ListOpts{Name: name}).AllPages()
	if err != nil {
		return nil, err
	}
	all, err := volumes.ExtractVolumes(page)
	if err != nil {
		return nil, err
	}
	if len(all) == 0 {
		return nil, fmt.Errorf("volume %s not found", name)
	}
	return &all[0], nil
}

func (c *CinderStorageProvider) waitForVolumeStatus(volumeID, want string, timeout time.Duration) error {
	start := time.Now()
	for {
		vol, err := volumes.Get(c.client, volumeID).Extract()
		if err != nil {
			return err
		}
		if vol.Status == want {
			return nil
		}
		if time.Since(start) > timeout {
			return fmt.Errorf("timeout waiting for %s", want)
		}
		time.Sleep(2 * time.Second)
	}
}

func (c *CinderStorageProvider) getVolumeNAA(vol *volumes.Volume) string {
	// check metadata 'naa'
	if naa, ok := vol.Metadata["naa"]; ok {
		return naa
	}

	return ""
}

func (c *CinderStorageProvider) getVolumeSerial(vol *volumes.Volume) string {
	return vol.ID
}

func (c *CinderStorageProvider) initializeConnection(volumeID string, connector map[string]interface{}) (map[string]interface{}, error) {
	url := c.client.ServiceURL("volumes", volumeID, "action")
	req := map[string]interface{}{
		"os-initialize_connection": map[string]interface{}{
			"connector": connector,
		},
	}
	var resp map[string]interface{}
	_, err := c.client.Post(url, req, &resp, &gophercloud.RequestOpts{OkCodes: []int{200}})
	if err != nil {
		return nil, err
	}
	// response has key "connection_info" or "initialize_connection"
	// return raw response for caller to parse
	return resp, nil
}

func (c *CinderStorageProvider) terminateConnection(volumeID string, connector map[string]interface{}) error {
	url := c.client.ServiceURL("volumes", volumeID, "action")
	req := map[string]interface{}{
		"os-terminate_connection": map[string]interface{}{
			"connector": connector,
		},
	}
	_, err := c.client.Post(url, req, nil, &gophercloud.RequestOpts{OkCodes: []int{202, 200}})
	return err
}
