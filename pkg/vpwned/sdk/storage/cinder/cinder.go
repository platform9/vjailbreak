package cinder

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gophercloud/gophercloud/v2"
	"github.com/gophercloud/gophercloud/v2/openstack"
	"github.com/gophercloud/gophercloud/v2/openstack/blockstorage/v3/volumes"
	"github.com/platform9/vjailbreak/pkg/vpwned/sdk/storage"
	"k8s.io/klog/v2"
)

func init() {
	storage.RegisterStorageProvider("cinder", &CinderStorageProvider{})
}

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

func (c *CinderStorageProvider) Connect(ctx context.Context, accessInfo storage.StorageAccessInfo) error {
	// Store access info
	c.accessInfo = accessInfo

	// Create auth options
	opts := gophercloud.AuthOptions{
		IdentityEndpoint: accessInfo.Hostname,
		Username:         accessInfo.Username,
		Password:         accessInfo.Password,
		TenantName:       accessInfo.TenantName,
		DomainName:       accessInfo.DomainName,
		AllowReauth:      true,
		Scope: &gophercloud.AuthScope{
			ProjectName: accessInfo.TenantName,
			DomainName:  accessInfo.DomainName, // project domain
		},
	}

	// Create provider client with custom TLS config if needed
	var providerClient *gophercloud.ProviderClient
	var err error

	if accessInfo.Insecure || accessInfo.SkipSSLVerification {
		// Create custom HTTP client with TLS verification disabled
		transport := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		httpClient := &http.Client{Transport: transport}

		// Create provider client with custom HTTP client
		providerClient, err = openstack.NewClient(accessInfo.Hostname)
		if err != nil {
			return fmt.Errorf("failed to create provider client: %w", err)
		}
		providerClient.HTTPClient = *httpClient

		// Authenticate with context
		err = openstack.Authenticate(ctx, providerClient, opts)
		if err != nil {
			return fmt.Errorf("authentication failed: %w", err)
		}
		klog.Infof("TLS verification disabled (insecure mode)")
	} else {
		// Use standard authenticated client with context
		providerClient, err = openstack.AuthenticatedClient(ctx, opts)
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

func (c *CinderStorageProvider) Disconnect() error {
	c.isConnected = false
	c.client = nil
	return nil
}

func (c *CinderStorageProvider) ValidateCredentials(ctx context.Context) error {
	if !c.isConnected {
		return fmt.Errorf("not connected")
	}
	// simple smoke test: list a page of volumes
	_, err := volumes.List(c.client, volumes.ListOpts{Limit: 1}).AllPages(ctx)
	if err != nil {
		return fmt.Errorf("validation list failed: %w", err)
	}
	return nil
}

// CreateVolume creates a volume with the given name and size in bytes
func (c *CinderStorageProvider) CreateVolume(volumeName string, size int64) (storage.Volume, error) {
	ctx := context.Background()
	if !c.isConnected {
		return storage.Volume{}, fmt.Errorf("not connected")
	}
	// convert bytes -> GB (round up)
	gb := int((size + (1024*1024*1024 - 1)) / (1024 * 1024 * 1024))
	createOpts := volumes.CreateOpts{
		Name: volumeName,
		Size: gb,
	}

	vol, err := volumes.Create(ctx, c.client, createOpts, nil).Extract()
	if err != nil {
		return storage.Volume{}, fmt.Errorf("create volume failed: %w", err)
	}

	// wait for available
	if err := c.waitForVolumeStatus(ctx, vol.ID, "available", 3*time.Minute); err != nil {
		return storage.Volume{}, fmt.Errorf("volume %s did not become available: %w", vol.ID, err)
	}

	return storage.Volume{
		Id:   vol.ID,
		Name: vol.Name,
		Size: int64(vol.Size) * 1024 * 1024 * 1024,
	}, nil
}

func (c *CinderStorageProvider) DeleteVolume(volumeName string) error {
	ctx := context.Background()
	vol, err := c.findVolumeByName(ctx, volumeName)
	if err != nil {
		return err
	}
	if err := volumes.Delete(ctx, c.client, vol.ID, nil).ExtractErr(); err != nil {
		return fmt.Errorf("delete volume: %w", err)
	}
	// optional: wait for deletion - omitted for brevity
	return nil
}

func (c *CinderStorageProvider) GetVolumeInfo(volumeName string) (storage.VolumeInfo, error) {
	ctx := context.Background()
	vol, err := c.findVolumeByName(ctx, volumeName)
	if err != nil {
		return storage.VolumeInfo{}, err
	}
	v := storage.VolumeInfo{
		Name: vol.Name,
		Size: int64(vol.Size) * 1024 * 1024 * 1024,
		NAA:  c.getVolumeNAA(vol),
	}
	return v, nil
}

func (c *CinderStorageProvider) ListAllVolumes() ([]storage.VolumeInfo, error) {
	ctx := context.Background()
	pages, err := volumes.List(c.client, volumes.ListOpts{}).AllPages(ctx)
	if err != nil {
		return nil, fmt.Errorf("list volumes: %w", err)
	}
	all, err := volumes.ExtractVolumes(pages)
	if err != nil {
		return nil, fmt.Errorf("extract volumes: %w", err)
	}
	out := make([]storage.VolumeInfo, 0, len(all))
	for _, vv := range all {
		out = append(out, storage.VolumeInfo{
			Name: vv.Name,
			Size: int64(vv.Size) * 1024 * 1024 * 1024,
			NAA:  c.getVolumeNAA(&vv),
		})
	}
	return out, nil
}

func (c *CinderStorageProvider) GetAllVolumeNAAs() ([]string, error) {
	vols, err := c.ListAllVolumes()
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
func (c *CinderStorageProvider) CreateOrUpdateInitiatorGroup(initiatorGroupName string, hbaIdentifiers []string) (storage.MappingContext, error) {
	precreate := false // Don't precreate hosts by default
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
			temp, err := c.CreateVolume(tempName, 1*1024*1024*1024) // 1GB
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
			if _, err := c.initializeConnection(temp.Id, connector); err != nil {
				klog.Warningf("init connection failed for %s: %v", iqn, err)
			} else {
				created := ctxMap["created_hosts"].([]string)
				created = append(created, initiatorGroupName)
				ctxMap["created_hosts"] = created
			}
			// try to terminate and delete
			_ = c.terminateConnection(temp.Id, connector)
			_ = c.DeleteVolume(temp.Name)
		}
	}
	return ctxMap, nil
}

// MapVolumeToGroup uses initialize_connection to map the named volume to each IQN (creates host & mapping)
func (c *CinderStorageProvider) MapVolumeToGroup(initiatorGroupName string, targetVolume storage.Volume, contextMap storage.MappingContext) (storage.Volume, error) {
	ctx := context.Background()
	iqnsVal, ok := contextMap["iqns"]
	if !ok {
		return storage.Volume{}, fmt.Errorf("iqns missing in mapping context")
	}
	iqns, ok := iqnsVal.([]string)
	if !ok || len(iqns) == 0 {
		return storage.Volume{}, fmt.Errorf("invalid iqns")
	}
	// find volume
	volPage, err := volumes.List(c.client, volumes.ListOpts{Name: targetVolume.Name}).AllPages(ctx)
	if err != nil {
		return storage.Volume{}, fmt.Errorf("find volume: %w", err)
	}
	vols, _ := volumes.ExtractVolumes(volPage)
	if len(vols) == 0 {
		return storage.Volume{}, fmt.Errorf("volume %s not found", targetVolume.Name)
	}
	vol := vols[0]

	var connectionInfo map[string]interface{}
	for _, iqn := range iqns {
		connector := map[string]interface{}{
			"initiator": iqn,
			"host":      initiatorGroupName,
			"platform":  "VMware_ESXi",
			"os_type":   "vmware",
		}
		resp, err := c.initializeConnection(vol.ID, connector)
		if err != nil {
			return storage.Volume{}, fmt.Errorf("initialize connection failed for %s: %w", iqn, err)
		}
		// Store the first connection info response to extract NAA
		if connectionInfo == nil {
			connectionInfo = resp
			klog.Infof("Connection info response: %+v", resp)
		}
		klog.Infof("Initialized connection for volume %s to iqn %s", vol.Name, iqn)
	}
	// update returned volume info
	targetVolume.NAA = c.extractNAAFromConnectionInfo(connectionInfo)
	targetVolume.Id = vol.ID
	klog.Infof("Extracted NAA: %s for volume %s", targetVolume.NAA, vol.Name)
	return targetVolume, nil
}

func (c *CinderStorageProvider) UnmapVolumeFromGroup(initiatorGroupName string, targetVolume storage.Volume, contextMap storage.MappingContext) error {
	ctx := context.Background()
	iqnsVal, ok := contextMap["iqns"]
	if !ok {
		return nil
	}
	iqns, ok := iqnsVal.([]string)
	if !ok || len(iqns) == 0 {
		return nil
	}
	vol, err := c.findVolumeByName(ctx, targetVolume.Name)
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

func (c *CinderStorageProvider) GetMappedGroups(targetVolume storage.Volume, contextMap storage.MappingContext) ([]string, error) {
	ctx := context.Background()
	vol, err := c.findVolumeByName(ctx, targetVolume.Name)
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

func (c *CinderStorageProvider) ResolveCinderVolumeToLUN(volumeName string) (storage.Volume, error) {
	ctx := context.Background()
	vol, err := c.findVolumeByName(ctx, volumeName)
	if err != nil {
		return storage.Volume{}, fmt.Errorf("get volume: %w", err)
	}
	return storage.Volume{
		Id:           vol.ID,
		Name:         vol.Name,
		Size:         int64(vol.Size) * 1024 * 1024 * 1024,
		NAA:          c.getVolumeNAA(vol),
		SerialNumber: c.getVolumeSerial(vol),
	}, nil
}

func (c *CinderStorageProvider) WhoAmI() string {
	return "cinder"
}

// ---------- helpers ----------

func (c *CinderStorageProvider) findVolumeByName(ctx context.Context, name string) (*volumes.Volume, error) {
	page, err := volumes.List(c.client, volumes.ListOpts{Name: name}).AllPages(ctx)
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

func (c *CinderStorageProvider) waitForVolumeStatus(ctx context.Context, volumeID, want string, timeout time.Duration) error {
	start := time.Now()
	for {
		vol, err := volumes.Get(ctx, c.client, volumeID).Extract()
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

// extractNAAFromConnectionInfo extracts the NAA identifier from Cinder's initialize_connection response
func (c *CinderStorageProvider) extractNAAFromConnectionInfo(connInfo map[string]interface{}) string {
	if connInfo == nil {
		return ""
	}

	// The response structure is: { "connection_info": { "data": { ... } } }
	connectionInfo, ok := connInfo["connection_info"].(map[string]interface{})
	if !ok {
		klog.Warningf("No connection_info in response")
		return ""
	}

	data, ok := connectionInfo["data"].(map[string]interface{})
	if !ok {
		klog.Warningf("No data in connection_info")
		return ""
	}

	// For Pure Storage, NAA is typically in device_path or target_wwn or explicitly as "naa"
	// Example: device_path = "/dev/disk/by-id/scsi-3naa.624a937011f9af6d5a344f0500016e27"
	if devicePath, ok := data["device_path"].(string); ok {
		// Extract NAA from device path
		if idx := strings.Index(devicePath, "naa."); idx != -1 {
			naa := devicePath[idx:]
			// Pure Storage may return "naa.36..." but ESXi expects "naa.6..."
			// Strip the "3" after "naa." prefix if present
			if strings.HasPrefix(naa, "naa.36") {
				naa = "naa." + naa[5:] // Keep "naa." + rest without "3"
			}
			klog.Infof("Extracted NAA from device_path: %s", naa)
			return naa
		}
	}

	// Check for explicit "naa" field
	if naa, ok := data["naa"].(string); ok {
		// Normalize: strip "naa." if present, handle "36" prefix, then re-add "naa."
		naa = strings.TrimPrefix(naa, "naa.")
		if strings.HasPrefix(naa, "36") {
			naa = naa[1:] // Strip leading "3"
		}
		naa = "naa." + naa
		klog.Infof("Found explicit NAA field: %s", naa)
		return naa
	}

	// Check for wwn field (Pure Storage uses this)
	if wwn, ok := data["wwn"].(string); ok {
		wwn = strings.ToLower(wwn)
		// Pure Storage returns wwn with "36" prefix (e.g., "3624a937..."),
		// but ESXi expects "6" prefix (e.g., "624a937...").
		// Strip the leading "3" if present for ESXi compatibility.
		if strings.HasPrefix(wwn, "36") {
			wwn = wwn[1:] // Remove the leading "3"
		}
		naa := fmt.Sprintf("naa.%s", wwn)
		klog.Infof("Extracted NAA from wwn: %s", naa)
		return naa
	}

	// Check for target_wwn (for FC)
	if wwn, ok := data["target_wwn"].(string); ok {
		naa := fmt.Sprintf("naa.%s", strings.ToLower(wwn))
		klog.Infof("Extracted NAA from target_wwn: %s", naa)
		return naa
	}

	klog.Warningf("Could not extract NAA from connection info: %+v", data)
	return ""
}

func (c *CinderStorageProvider) getVolumeSerial(vol *volumes.Volume) string {
	return vol.ID
}

func (c *CinderStorageProvider) initializeConnection(volumeID string, connector map[string]interface{}) (map[string]interface{}, error) {
	ctx := context.Background()
	url := c.client.ServiceURL("volumes", volumeID, "action")
	req := map[string]interface{}{
		"os-initialize_connection": map[string]interface{}{
			"connector": connector,
		},
	}
	klog.Infof("Calling initialize_connection for volume %s with connector: %+v", volumeID, connector)
	var resp map[string]interface{}
	httpResp, err := c.client.Post(ctx, url, req, &resp, &gophercloud.RequestOpts{
		OkCodes: []int{200},
	})
	if err != nil {
		klog.Errorf("initialize_connection failed: %v", err)
		klog.Errorf("Error type: %T", err)
		if httpResp != nil {
			klog.Errorf("HTTP Status: %d", httpResp.StatusCode)
		}
		// Try to parse error details from gophercloud error
		switch e := err.(type) {
		case gophercloud.ErrUnexpectedResponseCode:
			klog.Errorf("ErrUnexpectedResponseCode - Expected: %v, Actual: %d, Body: %s",
				e.Expected, e.Actual, string(e.Body))
		case *gophercloud.ErrUnexpectedResponseCode:
			klog.Errorf("*ErrUnexpectedResponseCode - Expected: %v, Actual: %d, Body: %s",
				e.Expected, e.Actual, string(e.Body))
		default:
			klog.Errorf("Unknown error type, full error: %+v", err)
		}
		return nil, fmt.Errorf("%v", err)
	}
	klog.Infof("initialize_connection response: %+v", resp)
	// response has key "connection_info" or "initialize_connection"
	// return raw response for caller to parse
	return resp, nil
}

func (c *CinderStorageProvider) terminateConnection(volumeID string, connector map[string]interface{}) error {
	ctx := context.Background()
	url := c.client.ServiceURL("volumes", volumeID, "action")
	req := map[string]interface{}{
		"os-terminate_connection": map[string]interface{}{
			"connector": connector,
		},
	}
	_, err := c.client.Post(ctx, url, req, nil, &gophercloud.RequestOpts{OkCodes: []int{202, 200}})
	return err
}
