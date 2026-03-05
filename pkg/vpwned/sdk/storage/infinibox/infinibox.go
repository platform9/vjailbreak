package infinibox

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/platform9/vjailbreak/pkg/vpwned/sdk/storage"
	"k8s.io/klog/v2"
)

// InfiniBox NAA prefix (OUI for Infinidat)
const InfiniboxProviderID = "6742b0f0"

// Context keys for mapping
const (
	hostIDContextKey      = "hostID"
	esxLogicalHostNameKey = "esxLogicalHostName"
	esxRealHostNameKey    = "esxRealHostName"
)

func init() {
	storage.RegisterStorageProvider("infinibox", &InfiniboxStorageProvider{})
}

// InfiniBox API response structures
type InfiniboxSystem struct {
	Name    string `json:"name"`
	Serial  int    `json:"serial"`
	Version string `json:"version"`
}

type InfiniboxVolume struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Size      int64  `json:"size"`
	Serial    string `json:"serial"`
	PoolID    int    `json:"pool_id"`
	CreatedAt int64  `json:"created_at"`
	ParentID  int    `json:"parent_id"`
}

type InfiniboxHost struct {
	ID    int             `json:"id"`
	Name  string          `json:"name"`
	Ports []InfiniboxPort `json:"ports"`
}

type InfiniboxPort struct {
	Address string `json:"address"`
	Type    string `json:"type"`
}

type InfiniboxLunMapping struct {
	ID            int  `json:"id"`
	Lun           int  `json:"lun"`
	VolumeID      int  `json:"volume_id"`
	HostID        int  `json:"host_id"`
	HostClusterID int  `json:"host_cluster_id"`
	Clustered     bool `json:"clustered"`
}

type InfiniboxPool struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type InfiniboxAPIResponse struct {
	Result json.RawMessage `json:"result"`
	Error  *InfiniboxError `json:"error"`
}

type InfiniboxError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// InfiniboxStorageProvider implements StorageProvider for Infinidat InfiniBox
type InfiniboxStorageProvider struct {
	storage.BaseStorageProvider
	poolID int
}

// Connect establishes connection to InfiniBox array
func (i *InfiniboxStorageProvider) Connect(ctx context.Context, accessInfo storage.StorageAccessInfo) error {
	i.AccessInfo = accessInfo
	i.Config = storage.VendorConfig{
		NAAPrefix: InfiniboxProviderID,
		Name:      "InfiniBox",
	}
	i.BaseURL = fmt.Sprintf("https://%s/api/rest", accessInfo.Hostname)
	i.Username = accessInfo.Username
	i.Password = accessInfo.Password

	i.InitHTTPClient(accessInfo.SkipSSLVerification)
	i.SetConnected(true)

	// Validate connection by getting system info
	system, err := i.getSystemInfo(ctx)
	if err != nil {
		i.SetConnected(false)
		return fmt.Errorf("failed to connect to InfiniBox: %w", err)
	}

	klog.Infof("Connected to InfiniBox: %s, Version: %s", system.Name, system.Version)
	return nil
}

// Disconnect closes the connection
func (i *InfiniboxStorageProvider) Disconnect() error {
	i.SetConnected(false)
	return nil
}

// ValidateCredentials validates the credentials
func (i *InfiniboxStorageProvider) ValidateCredentials(ctx context.Context) error {
	if !i.GetConnected() {
		err := i.Connect(ctx, i.AccessInfo)
		if err != nil {
			return err
		}
	}

	_, err := i.getSystemInfo(ctx)
	if err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}
	return nil
}

// CreateVolume creates a new volume on the InfiniBox array
func (i *InfiniboxStorageProvider) CreateVolume(volumeName string, size int64) (storage.Volume, error) {
	ctx := context.Background()

	if i.poolID == 0 {
		poolID, err := i.getDefaultPoolID(ctx)
		if err != nil {
			return storage.Volume{}, fmt.Errorf("failed to get default pool: %w", err)
		}
		i.poolID = poolID
	}

	klog.Infof("Creating InfiniBox volume %s with size %d bytes in pool %d", volumeName, size, i.poolID)

	reqBody := map[string]interface{}{
		"pool_id":  i.poolID,
		"size":     size,
		"name":     volumeName,
		"provtype": "THIN",
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return storage.Volume{}, fmt.Errorf("failed to marshal request: %w", err)
	}

	var volume InfiniboxVolume
	if err := i.doInfiniboxRequest(ctx, "POST", "/volumes", bytes.NewReader(jsonBody), &volume); err != nil {
		return storage.Volume{}, fmt.Errorf("failed to create volume %s: %w", volumeName, err)
	}

	klog.Infof("Created InfiniBox volume: %s, ID: %d, Serial: %s", volume.Name, volume.ID, volume.Serial)

	return storage.Volume{
		Name:         volume.Name,
		Size:         volume.Size,
		Id:           strconv.Itoa(volume.ID),
		SerialNumber: volume.Serial,
		NAA:          i.BuildNAA(volume.Serial),
	}, nil
}

// DeleteVolume deletes a volume from the InfiniBox array
func (i *InfiniboxStorageProvider) DeleteVolume(volumeName string) error {
	ctx := context.Background()

	volume, err := i.getVolumeByName(ctx, volumeName)
	if err != nil {
		return fmt.Errorf("failed to find volume %s: %w", volumeName, err)
	}

	klog.Infof("Deleting InfiniBox volume: %s (ID: %d)", volume.Name, volume.ID)

	if err := i.doInfiniboxRequest(ctx, "DELETE", fmt.Sprintf("/volumes/%d?approved=true", volume.ID), nil, nil); err != nil {
		return fmt.Errorf("failed to delete volume %s: %w", volumeName, err)
	}

	klog.Infof("Deleted InfiniBox volume: %s", volumeName)
	return nil
}

// GetVolumeInfo retrieves information about a volume from the InfiniBox array
func (i *InfiniboxStorageProvider) GetVolumeInfo(volumeName string) (storage.VolumeInfo, error) {
	ctx := context.Background()

	volume, err := i.getVolumeByName(ctx, volumeName)
	if err != nil {
		return storage.VolumeInfo{}, fmt.Errorf("failed to get volume %s: %w", volumeName, err)
	}

	return storage.VolumeInfo{
		Name:    volume.Name,
		Size:    volume.Size,
		Created: fmt.Sprintf("%d", volume.CreatedAt),
		NAA:     i.BuildNAA(volume.Serial),
	}, nil
}

// ListAllVolumes retrieves all volumes from the InfiniBox array
func (i *InfiniboxStorageProvider) ListAllVolumes() ([]storage.VolumeInfo, error) {
	ctx := context.Background()

	volumes, err := i.listVolumes(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list volumes: %w", err)
	}

	var volumeInfos []storage.VolumeInfo
	for _, v := range volumes {
		if v.ParentID == 0 {
			volumeInfos = append(volumeInfos, storage.VolumeInfo{
				Name:    v.Name,
				Size:    v.Size,
				Created: fmt.Sprintf("%d", v.CreatedAt),
				NAA:     i.BuildNAA(v.Serial),
			})
		}
	}

	return volumeInfos, nil
}

// GetAllVolumeNAAs retrieves NAA identifiers for all volumes on the array
func (i *InfiniboxStorageProvider) GetAllVolumeNAAs() ([]string, error) {
	return i.BaseStorageProvider.GetAllVolumeNAAs(i.ListAllVolumes)
}

// CreateOrUpdateInitiatorGroup finds hosts matching the provided HBA identifiers
func (i *InfiniboxStorageProvider) CreateOrUpdateInitiatorGroup(initiatorGroupName string, hbaIdentifiers []string) (storage.MappingContext, error) {
	ctx := context.Background()

	hosts, err := i.listHosts(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get all hosts: %w", err)
	}

	for _, host := range hosts {
		for _, port := range host.Ports {
			for _, adapterId := range hbaIdentifiers {
				if strings.HasPrefix(adapterId, "fc.") {
					wwpn, err := extractWWPN(adapterId)
					if err != nil {
						klog.Warningf("Failed to extract WWPN from adapter ID %s: %v", adapterId, err)
						continue
					}
					if compareWWNs(wwpn, port.Address) {
						klog.Infof("Found host %s with adapter ID %s (port address: %s)", host.Name, adapterId, port.Address)
						return createMappingContext(&host, initiatorGroupName), nil
					}
				} else {
					if strings.EqualFold(port.Address, adapterId) {
						klog.Infof("Found host %s with adapter ID %s (port address: %s)", host.Name, adapterId, port.Address)
						return createMappingContext(&host, initiatorGroupName), nil
					}
				}
			}
		}
	}

	return nil, fmt.Errorf("no host found with adapter IDs %v", hbaIdentifiers)
}

// MapVolumeToGroup maps a volume to a host
func (i *InfiniboxStorageProvider) MapVolumeToGroup(initiatorGroupName string, targetVolume storage.Volume, mappingCtx storage.MappingContext) (storage.Volume, error) {
	ctx := context.Background()

	if mappingCtx == nil {
		return targetVolume, fmt.Errorf("mapping context is required")
	}

	hostName := ""
	if esxHost, ok := mappingCtx[esxRealHostNameKey].(string); ok {
		hostName = esxHost
	}

	if hostName == "" {
		return targetVolume, fmt.Errorf("no host name found in mapping context")
	}

	klog.Infof("Mapping volume %s to host %s", targetVolume.Name, hostName)

	host, err := i.getHostByName(ctx, hostName)
	if err != nil {
		return targetVolume, fmt.Errorf("failed to find host %s: %w", hostName, err)
	}

	volumeID, err := strconv.Atoi(targetVolume.Id)
	if err != nil {
		volume, err := i.getVolumeByName(ctx, targetVolume.Name)
		if err != nil {
			return targetVolume, fmt.Errorf("failed to get volume %s: %w", targetVolume.Name, err)
		}
		volumeID = volume.ID
	}

	existingMappings, err := i.getLunsByVolume(ctx, volumeID)
	if err == nil {
		for _, mapping := range existingMappings {
			if mapping.HostID == host.ID {
				klog.Infof("Volume %s already mapped to host %s", targetVolume.Name, hostName)
				return targetVolume, nil
			}
		}
	}

	reqBody := map[string]interface{}{
		"volume_id": volumeID,
	}
	jsonBody, _ := json.Marshal(reqBody)

	if err := i.doInfiniboxRequest(ctx, "POST", fmt.Sprintf("/hosts/%d/luns", host.ID), bytes.NewReader(jsonBody), nil); err != nil {
		return targetVolume, fmt.Errorf("failed to map volume %s to host %s: %w", targetVolume.Name, hostName, err)
	}

	klog.Infof("Successfully mapped volume %s to host %s", targetVolume.Name, hostName)
	return targetVolume, nil
}

// UnmapVolumeFromGroup unmaps a volume from a host
func (i *InfiniboxStorageProvider) UnmapVolumeFromGroup(initiatorGroupName string, targetVolume storage.Volume, mappingCtx storage.MappingContext) error {
	ctx := context.Background()

	if mappingCtx == nil {
		return nil
	}

	hostName := ""
	if esxHost, ok := mappingCtx[esxRealHostNameKey].(string); ok {
		hostName = esxHost
	}

	if hostName == "" {
		klog.Warningf("No host name found in mapping context for unmapping")
		return nil
	}

	klog.Infof("Unmapping volume %s from host %s", targetVolume.Name, hostName)

	host, err := i.getHostByName(ctx, hostName)
	if err != nil {
		klog.Warningf("Failed to find host %s for unmapping: %v", hostName, err)
		return nil
	}

	volumeID, err := strconv.Atoi(targetVolume.Id)
	if err != nil {
		volume, err := i.getVolumeByName(ctx, targetVolume.Name)
		if err != nil {
			klog.Warningf("Failed to get volume %s for unmapping: %v", targetVolume.Name, err)
			return nil
		}
		volumeID = volume.ID
	}

	mappings, err := i.getLunsByVolume(ctx, volumeID)
	if err != nil {
		return fmt.Errorf("failed to get mappings for volume %s: %w", targetVolume.Name, err)
	}

	for _, mapping := range mappings {
		if mapping.HostID == host.ID {
			if err := i.doInfiniboxRequest(ctx, "DELETE", fmt.Sprintf("/hosts/%d/luns/volume_id/%d", host.ID, volumeID), nil, nil); err != nil {
				return fmt.Errorf("failed to unmap volume %s from host %s: %w", targetVolume.Name, hostName, err)
			}
			break
		}
	}

	klog.Infof("Successfully unmapped volume %s from host %s", targetVolume.Name, hostName)
	return nil
}

// GetMappedGroups returns the hosts the volume is mapped to
func (i *InfiniboxStorageProvider) GetMappedGroups(targetVolume storage.Volume, mappingCtx storage.MappingContext) ([]string, error) {
	ctx := context.Background()

	volumeID, err := strconv.Atoi(targetVolume.Id)
	if err != nil {
		volume, err := i.getVolumeByName(ctx, targetVolume.Name)
		if err != nil {
			return nil, fmt.Errorf("failed to get volume %s: %w", targetVolume.Name, err)
		}
		volumeID = volume.ID
	}

	klog.Infof("Checking mappings for volume ID %d", volumeID)
	lunInfos, err := i.getLunsByVolume(ctx, volumeID)
	if err != nil {
		return nil, fmt.Errorf("failed to get LUN mappings for volume %s: %w", targetVolume.Name, err)
	}

	if len(lunInfos) == 0 {
		return []string{}, nil
	}

	allHosts, err := i.listHosts(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get all hosts: %w", err)
	}

	hostByID := make(map[int]*InfiniboxHost)
	for idx := range allHosts {
		hostByID[allHosts[idx].ID] = &allHosts[idx]
	}

	mappedHosts := make([]string, 0, len(lunInfos))
	for _, lunInfo := range lunInfos {
		if host, exists := hostByID[lunInfo.HostID]; exists {
			mappedHosts = append(mappedHosts, host.Name)
		}
	}

	return mappedHosts, nil
}

// ResolveCinderVolumeToLUN resolves a Cinder volume name to a storage Volume/LUN
func (i *InfiniboxStorageProvider) ResolveCinderVolumeToLUN(volumeID string) (storage.Volume, error) {
	ctx := context.Background()

	volumeName := fmt.Sprintf("volume-%s", volumeID)

	volume, err := i.getVolumeByName(ctx, volumeName)
	if err != nil {
		volume, err = i.getVolumeByName(ctx, volumeID)
		if err != nil {
			return storage.Volume{}, fmt.Errorf("failed to find volume with ID %s: %w", volumeID, err)
		}
	}

	klog.Infof("Resolved Cinder volume %s to InfiniBox volume: %s (ID: %d)", volumeID, volume.Name, volume.ID)

	return storage.Volume{
		Name:         volume.Name,
		Size:         volume.Size,
		Id:           strconv.Itoa(volume.ID),
		SerialNumber: volume.Serial,
		NAA:          i.BuildNAA(volume.Serial),
	}, nil
}

// WhoAmI returns the provider name
func (i *InfiniboxStorageProvider) WhoAmI() string {
	return "infinibox"
}

// REST API helper methods

func (i *InfiniboxStorageProvider) doInfiniboxRequest(ctx context.Context, method, endpoint string, body *bytes.Reader, result interface{}) error {
	var resp InfiniboxAPIResponse
	if err := i.DoRequestJSON(ctx, method, endpoint, body, &resp); err != nil {
		return err
	}
	if resp.Error != nil && resp.Error.Code != "" {
		return fmt.Errorf("InfiniBox API error: %s - %s", resp.Error.Code, resp.Error.Message)
	}
	if result != nil && len(resp.Result) > 0 {
		if err := json.Unmarshal(resp.Result, result); err != nil {
			return fmt.Errorf("failed to unmarshal result: %w", err)
		}
	}
	return nil
}

func (i *InfiniboxStorageProvider) getSystemInfo(ctx context.Context) (*InfiniboxSystem, error) {
	var system InfiniboxSystem
	if err := i.doInfiniboxRequest(ctx, "GET", "/system", nil, &system); err != nil {
		return nil, err
	}
	return &system, nil
}

func (i *InfiniboxStorageProvider) getVolumeByName(ctx context.Context, name string) (*InfiniboxVolume, error) {
	var volumes []InfiniboxVolume
	endpoint := fmt.Sprintf("/volumes?name=%s", name)
	if err := i.doInfiniboxRequest(ctx, "GET", endpoint, nil, &volumes); err != nil {
		return nil, err
	}
	if len(volumes) == 0 {
		return nil, fmt.Errorf("volume %s not found", name)
	}
	return &volumes[0], nil
}

func (i *InfiniboxStorageProvider) listVolumes(ctx context.Context) ([]InfiniboxVolume, error) {
	var volumes []InfiniboxVolume
	if err := i.doInfiniboxRequest(ctx, "GET", "/volumes", nil, &volumes); err != nil {
		return nil, err
	}
	return volumes, nil
}

func (i *InfiniboxStorageProvider) listHosts(ctx context.Context) ([]InfiniboxHost, error) {
	var hosts []InfiniboxHost
	if err := i.doInfiniboxRequest(ctx, "GET", "/hosts?fields=id,name,ports", nil, &hosts); err != nil {
		return nil, err
	}
	return hosts, nil
}

func (i *InfiniboxStorageProvider) getHostByName(ctx context.Context, name string) (*InfiniboxHost, error) {
	var hosts []InfiniboxHost
	endpoint := fmt.Sprintf("/hosts?name=%s&fields=id,name,ports", name)
	if err := i.doInfiniboxRequest(ctx, "GET", endpoint, nil, &hosts); err != nil {
		return nil, err
	}
	if len(hosts) == 0 {
		return nil, fmt.Errorf("host %s not found", name)
	}
	return &hosts[0], nil
}

func (i *InfiniboxStorageProvider) getLunsByVolume(ctx context.Context, volumeID int) ([]InfiniboxLunMapping, error) {
	var mappings []InfiniboxLunMapping
	endpoint := fmt.Sprintf("/volumes/%d/luns", volumeID)
	if err := i.doInfiniboxRequest(ctx, "GET", endpoint, nil, &mappings); err != nil {
		return nil, err
	}
	return mappings, nil
}

func (i *InfiniboxStorageProvider) getDefaultPoolID(ctx context.Context) (int, error) {
	var pools []InfiniboxPool
	if err := i.doInfiniboxRequest(ctx, "GET", "/pools", nil, &pools); err != nil {
		return 0, err
	}
	if len(pools) == 0 {
		return 0, fmt.Errorf("no pools found")
	}
	klog.Infof("Using InfiniBox pool: %s (ID: %d)", pools[0].Name, pools[0].ID)
	return pools[0].ID, nil
}

func createMappingContext(host *InfiniboxHost, initiatorGroup string) storage.MappingContext {
	return storage.MappingContext{
		hostIDContextKey:      host.ID,
		esxLogicalHostNameKey: initiatorGroup,
		esxRealHostNameKey:    host.Name,
	}
}

func extractWWPN(adapterId string) (string, error) {
	if !strings.HasPrefix(adapterId, "fc.") {
		return "", fmt.Errorf("invalid FC adapter ID format: %s", adapterId)
	}
	return strings.TrimPrefix(adapterId, "fc."), nil
}

func compareWWNs(wwn1, wwn2 string) bool {
	normalize := func(wwn string) string {
		wwn = strings.ToLower(wwn)
		wwn = strings.ReplaceAll(wwn, ":", "")
		wwn = strings.ReplaceAll(wwn, "-", "")
		return wwn
	}
	return normalize(wwn1) == normalize(wwn2)
}
