package netapp

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/platform9/vjailbreak/pkg/vpwned/sdk/storage"
	"k8s.io/klog/v2"
)

// NetApp ONTAP NAA prefix (OUI for NetApp)
const NetAppProviderID = "60a98000"

func init() {
	storage.RegisterStorageProvider("netapp", &NetAppStorageProvider{})
}

// NetAppStorageProvider implements StorageProvider for NetApp ONTAP
type NetAppStorageProvider struct {
	storage.BaseStorageProvider
}

// ONTAP API response structures
type OntapClusterInfo struct {
	Name    string `json:"name"`
	UUID    string `json:"uuid"`
	Version struct {
		Full string `json:"full"`
	} `json:"version"`
}

type OntapLUN struct {
	UUID         string `json:"uuid"`
	Name         string `json:"name"`
	SerialNumber string `json:"serial_number"`
	Space        struct {
		Size int64 `json:"size"`
	} `json:"space"`
	Location struct {
		Volume struct {
			Name string `json:"name"`
			UUID string `json:"uuid"`
		} `json:"volume"`
	} `json:"location"`
	CreateTime string `json:"create_time"`
}

type OntapLUNResponse struct {
	Records    []OntapLUN `json:"records"`
	NumRecords int        `json:"num_records"`
}

type OntapIgroup struct {
	UUID       string `json:"uuid"`
	Name       string `json:"name"`
	Protocol   string `json:"protocol"`
	Initiators []struct {
		Name string `json:"name"`
	} `json:"initiators"`
}

type OntapIgroupResponse struct {
	Records    []OntapIgroup `json:"records"`
	NumRecords int           `json:"num_records"`
}

// Connect establishes connection to NetApp ONTAP array
func (n *NetAppStorageProvider) Connect(ctx context.Context, accessInfo storage.StorageAccessInfo) error {
	n.AccessInfo = accessInfo
	n.Config = storage.VendorConfig{
		NAAPrefix: NetAppProviderID,
		Name:      "NetApp",
	}
	n.BaseURL = fmt.Sprintf("https://%s/api", accessInfo.Hostname)
	n.Username = accessInfo.Username
	n.Password = accessInfo.Password

	n.InitHTTPClient(accessInfo.SkipSSLVerification)
	n.SetConnected(true)

	// Validate connection by getting cluster info
	cluster, err := n.getClusterInfo(ctx)
	if err == nil {
		klog.Infof("Connected to NetApp ONTAP Cluster: %s, Version: %s", cluster.Name, cluster.Version.Full)
	}

	return nil
}

// Disconnect closes the connection
func (n *NetAppStorageProvider) Disconnect() error {
	n.SetConnected(false)
	return nil
}

// ValidateCredentials validates the credentials
func (n *NetAppStorageProvider) ValidateCredentials(ctx context.Context) error {
	if !n.GetConnected() {
		err := n.Connect(ctx, n.AccessInfo)
		if err != nil {
			return err
		}
	}

	// Try to get cluster info as validation
	_, err := n.getClusterInfo(ctx)
	if err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}
	return nil
}

// CreateVolume creates a new LUN on the NetApp array
func (n *NetAppStorageProvider) CreateVolume(volumeName string, size int64) (storage.Volume, error) {
	// TODO: Implement LUN creation via ONTAP REST API
	// POST /api/storage/luns
	return storage.Volume{}, errors.New("CreateVolume not implemented for NetApp")
}

// DeleteVolume deletes a LUN from the NetApp array
func (n *NetAppStorageProvider) DeleteVolume(volumeName string) error {
	// TODO: Implement LUN deletion via ONTAP REST API
	// DELETE /api/storage/luns/{uuid}
	return errors.New("DeleteVolume not implemented for NetApp")
}

// GetVolumeInfo retrieves information about a LUN from the NetApp array
func (n *NetAppStorageProvider) GetVolumeInfo(volumeName string) (storage.VolumeInfo, error) {
	ctx := context.Background()
	luns, err := n.listLUNs(ctx, fmt.Sprintf("name=%s", volumeName))
	if err != nil {
		return storage.VolumeInfo{}, fmt.Errorf("failed to get LUN %s: %w", volumeName, err)
	}

	if len(luns) == 0 {
		return storage.VolumeInfo{}, fmt.Errorf("LUN %s not found", volumeName)
	}

	lun := luns[0]
	return storage.VolumeInfo{
		Name:    lun.Name,
		Size:    lun.Space.Size,
		Created: lun.CreateTime,
		NAA:     n.BuildNAA(lun.SerialNumber),
	}, nil
}

// ListAllVolumes retrieves all LUNs from the NetApp array
func (n *NetAppStorageProvider) ListAllVolumes() ([]storage.VolumeInfo, error) {
	ctx := context.Background()
	luns, err := n.listLUNs(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("failed to list LUNs: %w", err)
	}

	var volumeInfos []storage.VolumeInfo
	for _, lun := range luns {
		volumeInfos = append(volumeInfos, storage.VolumeInfo{
			Name:    lun.Name,
			Size:    lun.Space.Size,
			Created: lun.CreateTime,
			NAA:     n.BuildNAA(lun.SerialNumber),
		})
	}

	return volumeInfos, nil
}

// GetAllVolumeNAAs retrieves NAA identifiers for all LUNs on the array
func (n *NetAppStorageProvider) GetAllVolumeNAAs() ([]string, error) {
	return n.BaseStorageProvider.GetAllVolumeNAAs(n.ListAllVolumes)
}

// CreateOrUpdateInitiatorGroup creates or updates an igroup with the ESX adapters
func (n *NetAppStorageProvider) CreateOrUpdateInitiatorGroup(initiatorGroupName string, hbaIdentifiers []string) (storage.MappingContext, error) {
	ctx := context.Background()

	// List existing igroups and find matches
	igroups, err := n.listIgroups(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list igroups: %w", err)
	}

	matchedIgroups := []string{}

	for _, ig := range igroups {
		initiatorNames := make([]string, len(ig.Initiators))
		for i, init := range ig.Initiators {
			initiatorNames[i] = init.Name
		}
		klog.Infof("Checking igroup %s, initiators: %v", ig.Name, initiatorNames)

		for _, init := range ig.Initiators {
			if storage.ContainsIgnoreCase(hbaIdentifiers, init.Name) {
				klog.Infof("Adding igroup %s (matched initiator: %s)", ig.Name, init.Name)
				matchedIgroups = append(matchedIgroups, ig.Name)
				break
			}
		}
	}

	if len(matchedIgroups) == 0 {
		return nil, fmt.Errorf("no igroups found matching any of the provided IQNs/WWNs: %v", hbaIdentifiers)
	}

	return storage.MappingContext{"igroups": matchedIgroups}, nil
}

// MapVolumeToGroup maps a LUN to igroups
func (n *NetAppStorageProvider) MapVolumeToGroup(initiatorGroupName string, targetVolume storage.Volume, context storage.MappingContext) (storage.Volume, error) {
	igroupsVal, ok := context["igroups"]
	if !ok {
		return storage.Volume{}, errors.New("igroups not found in mapping context")
	}

	igroups, ok := igroupsVal.([]string)
	if !ok || len(igroups) == 0 {
		return storage.Volume{}, errors.New("invalid or empty igroups list in mapping context")
	}

	// TODO: Implement LUN mapping via ONTAP REST API
	// POST /api/protocols/san/lun-maps
	for _, igroup := range igroups {
		klog.Infof("Mapping LUN %s to igroup %s", targetVolume.Name, igroup)
		// API call to map LUN to igroup
	}

	return targetVolume, nil
}

// UnmapVolumeFromGroup unmaps a LUN from igroups
func (n *NetAppStorageProvider) UnmapVolumeFromGroup(initiatorGroupName string, targetVolume storage.Volume, context storage.MappingContext) error {
	igroupsVal, ok := context["igroups"]
	if !ok {
		return nil // No igroups to unmap
	}

	igroups, ok := igroupsVal.([]string)
	if !ok || len(igroups) == 0 {
		return nil
	}

	// TODO: Implement LUN unmapping via ONTAP REST API
	// DELETE /api/protocols/san/lun-maps/{lun.uuid}/{igroup.uuid}
	for _, igroup := range igroups {
		klog.Infof("Unmapping LUN %s from igroup %s", targetVolume.Name, igroup)
	}

	return nil
}

// GetMappedGroups returns the igroups the LUN is mapped to
func (n *NetAppStorageProvider) GetMappedGroups(targetVolume storage.Volume, context storage.MappingContext) ([]string, error) {
	// TODO: Implement via ONTAP REST API
	// GET /api/protocols/san/lun-maps?lun.name={name}
	return nil, nil
}

// ResolveCinderVolumeToLUN resolves a Cinder volume name to a storage LUN
func (n *NetAppStorageProvider) ResolveCinderVolumeToLUN(volumeID string) (storage.Volume, error) {
	ctx := context.Background()

	// NetApp Cinder driver naming convention - search for LUN containing the volume ID
	// Common patterns: "volume-<uuid>", "/vol/<volume>/lun-<uuid>"
	luns, err := n.listLUNs(ctx, fmt.Sprintf("name=*%s*", volumeID))
	if err != nil {
		return storage.Volume{}, fmt.Errorf("failed to search for LUN with volume ID %s: %w", volumeID, err)
	}

	if len(luns) == 0 {
		return storage.Volume{}, fmt.Errorf("no LUN found matching Cinder volume ID %s", volumeID)
	}

	lun := luns[0]
	klog.Infof("Resolved Cinder volume %s to LUN: %+v", volumeID, lun)

	return storage.Volume{
		Name:         lun.Name,
		Size:         lun.Space.Size,
		SerialNumber: lun.SerialNumber,
		NAA:          n.BuildNAA(lun.SerialNumber),
	}, nil
}

// GetVolumeFromNAA retrieves a NetApp LUN by its NAA identifier
func (n *NetAppStorageProvider) GetVolumeFromNAA(naaID string) (storage.Volume, error) {
	serial, err := n.ExtractSerialFromNAA(naaID)
	if err != nil {
		return storage.Volume{}, err
	}

	ctx := context.Background()
	luns, err := n.listLUNs(ctx, "")
	if err != nil {
		return storage.Volume{}, fmt.Errorf("failed to list LUNs: %w", err)
	}

	for _, lun := range luns {
		if strings.ToUpper(lun.SerialNumber) == serial {
			klog.Infof("Found NetApp LUN %s with serial %s matching NAA %s", lun.Name, lun.SerialNumber, naaID)
			return storage.Volume{
				Name:         lun.Name,
				Size:         lun.Space.Size,
				SerialNumber: lun.SerialNumber,
				NAA:          naaID,
			}, nil
		}
	}

	return storage.Volume{}, fmt.Errorf("no NetApp LUN found with NAA %s (serial: %s)", naaID, serial)
}

// WhoAmI returns the provider name
func (n *NetAppStorageProvider) WhoAmI() string {
	return "netapp"
}

// Helper methods

func (n *NetAppStorageProvider) getClusterInfo(ctx context.Context) (*OntapClusterInfo, error) {
	var cluster OntapClusterInfo
	err := n.DoRequestJSON(ctx, "GET", "/cluster", nil, &cluster)
	if err != nil {
		return nil, err
	}
	return &cluster, nil
}

func (n *NetAppStorageProvider) listLUNs(ctx context.Context, filter string) ([]OntapLUN, error) {
	endpoint := "/storage/luns?fields=serial_number,space,location,create_time"
	if filter != "" {
		endpoint = fmt.Sprintf("%s&%s", endpoint, filter)
	}

	var response OntapLUNResponse
	err := n.DoRequestJSON(ctx, "GET", endpoint, nil, &response)
	if err != nil {
		return nil, err
	}

	return response.Records, nil
}

func (n *NetAppStorageProvider) listIgroups(ctx context.Context) ([]OntapIgroup, error) {
	var response OntapIgroupResponse
	err := n.DoRequestJSON(ctx, "GET", "/protocols/san/igroups?fields=initiators", nil, &response)
	if err != nil {
		return nil, err
	}

	return response.Records, nil
}
