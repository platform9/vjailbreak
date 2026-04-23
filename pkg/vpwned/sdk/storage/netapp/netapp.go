package netapp

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/platform9/vjailbreak/pkg/vpwned/sdk/storage"
	"github.com/platform9/vjailbreak/pkg/vpwned/sdk/storage/fcutil"
	"k8s.io/klog/v2"
)

// NetApp ONTAP NAA prefix (OUI for NetApp)
const NetAppProviderID = "600a0980"

// VendorName is the canonical vendor-type string registered with the storage
// SDK and persisted on ArrayCreds.spec.vendorType. Use this wherever code
// outside the NetApp SDK needs to compare against the NetApp vendor type.
const VendorName = "netapp"

// ProviderOptions keys read by the NetApp provider from
// storage.StorageAccessInfo.ProviderOptions. Unknown keys are ignored.
const (
	OptionSVM     = "svm"
	OptionFlexVol = "flexvol"
)

func init() {
	storage.RegisterStorageProvider(VendorName, &NetAppStorageProvider{})
}

// NetAppStorageProvider implements StorageProvider for NetApp ONTAP.
// SVM and FlexVol scope the array operations to a specific storage virtual
// machine and FlexVol. They may be set explicitly via ProviderOptions; if
// empty at operation time, the provider falls back to best-effort discovery
// from existing LUNs or a single-option auto-pick.
type NetAppStorageProvider struct {
	storage.BaseStorageProvider
	SVM     string
	FlexVol string
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
	SVM struct {
		Name string `json:"name"`
		UUID string `json:"uuid"`
	} `json:"svm"`
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
	UUID     string `json:"uuid"`
	Name     string `json:"name"`
	Protocol string `json:"protocol"`
	SVM      struct {
		Name string `json:"name"`
		UUID string `json:"uuid"`
	} `json:"svm"`
	Initiators []struct {
		Name string `json:"name"`
	} `json:"initiators"`
}

type OntapIgroupResponse struct {
	Records    []OntapIgroup `json:"records"`
	NumRecords int           `json:"num_records"`
}

type OntapSVM struct {
	UUID string `json:"uuid"`
	Name string `json:"name"`
}

type OntapSVMResponse struct {
	Records    []OntapSVM `json:"records"`
	NumRecords int        `json:"num_records"`
}

type OntapFlexVol struct {
	UUID  string `json:"uuid"`
	Name  string `json:"name"`
	Style string `json:"style"`
	SVM   struct {
		Name string `json:"name"`
		UUID string `json:"uuid"`
	} `json:"svm"`
}

type OntapFlexVolResponse struct {
	Records    []OntapFlexVol `json:"records"`
	NumRecords int            `json:"num_records"`
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

	if accessInfo.ProviderOptions != nil {
		if svm := strings.TrimSpace(accessInfo.ProviderOptions[OptionSVM]); svm != "" {
			n.SVM = svm
		}
		if flexvol := strings.TrimSpace(accessInfo.ProviderOptions[OptionFlexVol]); flexvol != "" {
			n.FlexVol = flexvol
		}
	}

	// Validate connection by getting cluster info
	cluster, err := n.getClusterInfo(ctx)
	if err != nil {
		n.SetConnected(false)
		return fmt.Errorf("failed to connect to NetApp ONTAP cluster: %w", err)
	}

	klog.Infof("Connected to NetApp ONTAP Cluster: %s, Version: %s (SVM: %q, FlexVol: %q)",
		cluster.Name, cluster.Version.Full, n.SVM, n.FlexVol)
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
	ctx := context.Background()

	// Get the volume path and SVM from existing LUNs
	volumePath, svmName, err := n.getDefaultVolumePathAndSVM(ctx)
	if err != nil {
		return storage.Volume{}, fmt.Errorf("failed to determine volume path: %w", err)
	}

	// Build full LUN path: /vol/<volume_name>/<lun_name>
	lunPath := fmt.Sprintf("%s/%s", volumePath, volumeName)
	klog.Infof("Creating NetApp LUN at path: %s with size: %d on SVM: %s", lunPath, size, svmName)

	// Create LUN via ONTAP REST API
	reqBody := map[string]interface{}{
		"name": lunPath,
		"svm": map[string]interface{}{
			"name": svmName,
		},
		"space": map[string]interface{}{
			"size": size,
		},
		"os_type": "vmware",
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return storage.Volume{}, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Use return_records=true to get the created LUN in the response
	type CreateResponse struct {
		Records []OntapLUN `json:"records"`
	}
	var response CreateResponse
	err = n.DoRequestJSON(ctx, "POST", "/storage/luns?return_records=true", bytes.NewReader(jsonBody), &response)
	if err != nil {
		return storage.Volume{}, fmt.Errorf("failed to create LUN %s: %w", volumeName, err)
	}

	if len(response.Records) == 0 {
		return storage.Volume{}, fmt.Errorf("LUN creation succeeded but no records returned for %s", volumeName)
	}

	lun := response.Records[0]
	klog.Infof("Created NetApp LUN: %s, Serial: %s", lun.Name, lun.SerialNumber)

	return storage.Volume{
		Name:         lun.Name,
		Size:         lun.Space.Size,
		Id:           lun.UUID,
		SerialNumber: lun.SerialNumber,
		NAA:          n.BuildNAA(lun.SerialNumber),
	}, nil
}

// DeleteVolume deletes a LUN from the NetApp array
func (n *NetAppStorageProvider) DeleteVolume(volumeName string) error {
	ctx := context.Background()

	// Find the LUN by name
	luns, err := n.listLUNs(ctx, fmt.Sprintf("name=*%s*", volumeName))
	if err != nil {
		return fmt.Errorf("failed to find LUN %s: %w", volumeName, err)
	}

	if len(luns) == 0 {
		return fmt.Errorf("LUN %s not found", volumeName)
	}

	lun := luns[0]
	klog.Infof("Deleting NetApp LUN: %s (UUID: %s)", lun.Name, lun.UUID)

	// Delete LUN via ONTAP REST API
	err = n.DoRequestJSON(ctx, "DELETE", fmt.Sprintf("/storage/luns/%s", lun.UUID), nil, nil)
	if err != nil {
		return fmt.Errorf("failed to delete LUN %s: %w", volumeName, err)
	}

	klog.Infof("Deleted NetApp LUN: %s", volumeName)
	return nil
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

// detectSANProtocol inspects hbaIdentifiers and returns the ONTAP igroup protocol
// string: "fcp" when all adapters are Fibre Channel, "iscsi" when all are iSCSI,
// or "mixed" when both types are present.
func detectSANProtocol(hbaIdentifiers []string) string {
	hasFC, hasIQN := false, false
	for _, id := range hbaIdentifiers {
		lower := strings.ToLower(id)
		switch {
		case strings.HasPrefix(lower, "fc."):
			hasFC = true
		case strings.HasPrefix(lower, "iqn."), strings.HasPrefix(lower, "eui."), strings.HasPrefix(lower, "nqn."):
			hasIQN = true
		}
	}
	switch {
	case hasFC && !hasIQN:
		return "fcp"
	case hasIQN && !hasFC:
		return "iscsi"
	default:
		return "mixed"
	}
}

// isWWPNLike reports whether s looks like an FC world-wide port name: pure hex
// after removing the usual separators. Used to pick between case-insensitive
// string equality (IQN/NQN/EUI) and fcutil's normalising comparison (WWPN).
func isWWPNLike(s string) bool {
	stripped := fcutil.StripWWNFormatting(s)
	if stripped == "" {
		return false
	}
	for _, r := range stripped {
		switch {
		case r >= '0' && r <= '9':
		case r >= 'A' && r <= 'F':
		default:
			return false
		}
	}
	return true
}

// initiatorMatches reports whether candidate matches any entry in initiators.
// WWPNs are compared with separator/case-insensitive normalisation via fcutil;
// IQNs fall back to plain case-insensitive equality.
func initiatorMatches(candidate string, initiators []string) bool {
	if isWWPNLike(candidate) {
		for _, init := range initiators {
			if isWWPNLike(init) && fcutil.EqualWWNs(candidate, init) {
				return true
			}
		}
		return false
	}
	return storage.ContainsIgnoreCase(initiators, candidate)
}

// normaliseToONTAPInitiators converts HBA identifiers to the format ONTAP expects.
// FC adapter UIDs (fc.WWNN:WWPN) are converted to colon-separated WWPN strings;
// IQN / EUI / NQN values are kept as-is.
func normaliseToONTAPInitiators(hbaIdentifiers []string) ([]string, error) {
	out := make([]string, 0, len(hbaIdentifiers))
	for _, id := range hbaIdentifiers {
		if strings.HasPrefix(strings.ToLower(id), "fc.") {
			wwpn, err := fcutil.FormattedWWPNFromFCUID(id)
			if err != nil {
				return nil, fmt.Errorf("failed to parse FC adapter UID %q: %w", id, err)
			}
			out = append(out, wwpn)
		} else {
			out = append(out, id)
		}
	}
	return out, nil
}

// ensureIgroupExists returns the UUID of the named igroup on svmName, creating it
// (with the given protocol and os_type "vmware") if it does not already exist.
func (n *NetAppStorageProvider) ensureIgroupExists(ctx context.Context, name, protocol, svmName string) (string, error) {
	igroups, err := n.listIgroups(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to list igroups: %w", err)
	}
	for _, ig := range igroups {
		if ig.Name == name {
			klog.Infof("Igroup %q already exists (UUID: %s)", name, ig.UUID)
			return ig.UUID, nil
		}
	}

	klog.Infof("Creating igroup %q (protocol: %s, SVM: %s)", name, protocol, svmName)
	reqBody := map[string]interface{}{
		"name":     name,
		"protocol": protocol,
		"os_type":  "vmware",
		"svm": map[string]interface{}{
			"name": svmName,
		},
	}
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal igroup create request: %w", err)
	}

	var response OntapIgroupResponse
	err = n.DoRequestJSON(ctx, "POST", "/protocols/san/igroups?return_records=true", bytes.NewReader(jsonBody), &response)
	if err != nil {
		// Concurrent migrations can race on the same igroup name. If ONTAP
		// reports the igroup already exists, re-resolve it by name so both
		// reconciliations converge on the same UUID.
		if isONTAPConflict(err) {
			klog.Infof("Igroup %q already exists (conflict on create), re-resolving UUID", name)
			ig, lookupErr := n.getIgroupByName(ctx, name)
			if lookupErr == nil {
				return ig.UUID, nil
			}
			return "", fmt.Errorf("igroup %q exists but lookup failed: %w", name, lookupErr)
		}
		return "", fmt.Errorf("failed to create igroup %q: %w", name, err)
	}
	if len(response.Records) == 0 {
		return "", fmt.Errorf("igroup creation returned no records for %q", name)
	}
	klog.Infof("Created igroup %q with UUID %s", name, response.Records[0].UUID)
	return response.Records[0].UUID, nil
}

// isONTAPConflict reports whether err represents an HTTP 409 / duplicate-entry
// response from ONTAP. Both the message substrings ("already exists",
// "duplicate") and the "status 409" marker produced by DoRequest are accepted
// so that the caller can treat the desired state as already satisfied.
func isONTAPConflict(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "status 409") ||
		strings.Contains(msg, "already exists") ||
		strings.Contains(msg, "duplicate") ||
		strings.Contains(msg, "already mapped") ||
		strings.Contains(msg, "already in group")
}

// addInitiatorToIgroup adds a single initiator (IQN or colon-separated WWPN) to
// the igroup identified by igroupUUID. Duplicate-initiator errors are silently
// ignored since the desired state is already satisfied.
func (n *NetAppStorageProvider) addInitiatorToIgroup(ctx context.Context, igroupUUID, initiatorName string) error {
	reqBody := map[string]interface{}{"name": initiatorName}
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}
	endpoint := fmt.Sprintf("/protocols/san/igroups/%s/initiators", igroupUUID)
	err = n.DoRequestJSON(ctx, "POST", endpoint, bytes.NewReader(jsonBody), nil)
	if err != nil {
		if isONTAPConflict(err) {
			klog.Infof("Initiator %q already in igroup %s — skipping", initiatorName, igroupUUID)
			return nil
		}
		return fmt.Errorf("failed to add initiator %q to igroup %s: %w", initiatorName, igroupUUID, err)
	}
	klog.Infof("Added initiator %q to igroup %s", initiatorName, igroupUUID)
	return nil
}

// CreateOrUpdateInitiatorGroup creates or updates an igroup with the ESX adapters.
// It supports both iSCSI (IQN) and Fibre Channel (fc.WWNN:WWPN) adapter identifiers.
//
// The function first searches all existing igroups for one that already contains a
// matching initiator. If none is found it creates a protocol-specific vjailbreak
// igroup (name: "<initiatorGroupName>-fcp" or "<initiatorGroupName>-iscsi") and
// populates it with the normalised initiator list.
func (n *NetAppStorageProvider) CreateOrUpdateInitiatorGroup(initiatorGroupName string, hbaIdentifiers []string) (storage.MappingContext, error) {
	ctx := context.Background()

	// Normalise FC UIDs → colon-separated WWPN; IQNs are kept unchanged.
	ontapInitiators, err := normaliseToONTAPInitiators(hbaIdentifiers)
	if err != nil {
		return nil, fmt.Errorf("failed to normalise HBA identifiers: %w", err)
	}

	protocol := detectSANProtocol(hbaIdentifiers)
	klog.Infof("Detected SAN protocol: %s, normalised initiators: %v", protocol, ontapInitiators)

	// Search existing igroups for a matching initiator.
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
		klog.Infof("Checking igroup %s (protocol: %s), initiators: %v", ig.Name, ig.Protocol, initiatorNames)

		// Skip igroups whose protocol does not match what the host is using;
		// ONTAP will not accept a LUN map on a protocol-mismatched igroup.
		if protocol != "mixed" && ig.Protocol != "" && !strings.EqualFold(ig.Protocol, protocol) && !strings.EqualFold(ig.Protocol, "mixed") {
			continue
		}

		matched := false
		for _, init := range ig.Initiators {
			if initiatorMatches(init.Name, ontapInitiators) {
				klog.Infof("Matched igroup %s via initiator %s", ig.Name, init.Name)
				matchedIgroups = append(matchedIgroups, ig.Name)
				matched = true
				break
			}
		}
		if !matched {
			continue
		}

		// Flag multipath gaps: if any of the host's initiators is missing from
		// the matched igroup the LUN will only be visible on the registered
		// paths and the operator needs to add the rest.
		for _, want := range ontapInitiators {
			present := false
			for _, init := range ig.Initiators {
				if initiatorMatches(init.Name, []string{want}) {
					present = true
					break
				}
			}
			if !present {
				klog.Warningf("Matched igroup %s is missing host initiator %q — multipath coverage may be impaired; add the initiator on the NetApp side", ig.Name, want)
			}
		}
	}

	if len(matchedIgroups) > 0 {
		return storage.MappingContext{"igroups": matchedIgroups}, nil
	}

	// No existing igroup matched — create a vjailbreak-managed one.
	if protocol == "mixed" {
		return nil, fmt.Errorf(
			"host has both FC and iSCSI adapters; cannot create a single ONTAP igroup — "+
				"ensure the ESXi host uses a single transport type: %v", hbaIdentifiers)
	}

	_, svmName, err := n.getDefaultVolumePathAndSVM(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to discover SVM for igroup creation: %w", err)
	}

	igroupName := fmt.Sprintf("%s-%s", initiatorGroupName, protocol)
	igroupUUID, err := n.ensureIgroupExists(ctx, igroupName, protocol, svmName)
	if err != nil {
		return nil, err
	}

	for _, initiator := range ontapInitiators {
		klog.Infof("Adding initiator %q to igroup %s", initiator, igroupName)
		if err := n.addInitiatorToIgroup(ctx, igroupUUID, initiator); err != nil {
			return nil, err
		}
	}

	return storage.MappingContext{"igroups": []string{igroupName}}, nil
}

// MapVolumeToGroup maps a LUN to igroups
func (n *NetAppStorageProvider) MapVolumeToGroup(initiatorGroupName string, targetVolume storage.Volume, mappingCtx storage.MappingContext) (storage.Volume, error) {
	ctx := context.Background()

	igroupsVal, ok := mappingCtx["igroups"]
	if !ok {
		return storage.Volume{}, errors.New("igroups not found in mapping context")
	}

	igroups, ok := igroupsVal.([]string)
	if !ok || len(igroups) == 0 {
		return storage.Volume{}, errors.New("invalid or empty igroups list in mapping context")
	}

	// Get LUN details to find UUID
	lun, err := n.getLUNByName(ctx, targetVolume.Name)
	if err != nil {
		return storage.Volume{}, fmt.Errorf("failed to get LUN %s: %w", targetVolume.Name, err)
	}

	for _, igroupName := range igroups {
		klog.Infof("Mapping LUN %s to igroup %s", targetVolume.Name, igroupName)

		// Get igroup UUID - ONTAP API requires UUID for lun-maps
		igroup, err := n.getIgroupByName(ctx, igroupName)
		if err != nil {
			return storage.Volume{}, fmt.Errorf("failed to get igroup %s: %w", igroupName, err)
		}

		reqBody := map[string]interface{}{
			"svm": map[string]interface{}{
				"name": lun.SVM.Name,
			},
			"lun": map[string]interface{}{
				"uuid": lun.UUID,
			},
			"igroup": map[string]interface{}{
				"uuid": igroup.UUID,
			},
		}

		jsonBody, err := json.Marshal(reqBody)
		if err != nil {
			return storage.Volume{}, fmt.Errorf("failed to marshal request: %w", err)
		}

		err = n.DoRequestJSON(ctx, "POST", "/protocols/san/lun-maps", bytes.NewReader(jsonBody), nil)
		if err != nil {
			if isONTAPConflict(err) {
				klog.Infof("LUN %s already mapped to igroup %s", targetVolume.Name, igroupName)
				continue
			}
			return storage.Volume{}, fmt.Errorf("failed to map LUN %s to igroup %s: %w", targetVolume.Name, igroupName, err)
		}
		klog.Infof("Successfully mapped LUN %s to igroup %s", targetVolume.Name, igroupName)
	}

	return targetVolume, nil
}

// UnmapVolumeFromGroup unmaps a LUN from igroups
func (n *NetAppStorageProvider) UnmapVolumeFromGroup(initiatorGroupName string, targetVolume storage.Volume, mappingCtx storage.MappingContext) error {
	ctx := context.Background()

	igroupsVal, ok := mappingCtx["igroups"]
	if !ok {
		return nil // No igroups to unmap
	}

	igroups, ok := igroupsVal.([]string)
	if !ok || len(igroups) == 0 {
		return nil
	}

	// Get LUN details to find UUID
	lun, err := n.getLUNByName(ctx, targetVolume.Name)
	if err != nil {
		klog.Warningf("Failed to get LUN %s for unmapping: %v", targetVolume.Name, err)
		return nil // LUN might already be deleted
	}

	for _, igroupName := range igroups {
		klog.Infof("Unmapping LUN %s from igroup %s", targetVolume.Name, igroupName)

		// Get igroup UUID
		igroup, err := n.getIgroupByName(ctx, igroupName)
		if err != nil {
			klog.Warningf("Failed to get igroup %s: %v", igroupName, err)
			continue
		}

		endpoint := fmt.Sprintf("/protocols/san/lun-maps/%s/%s", lun.UUID, igroup.UUID)
		err = n.DoRequestJSON(ctx, "DELETE", endpoint, nil, nil)
		if err != nil {
			klog.Warningf("Failed to unmap LUN %s from igroup %s: %v", targetVolume.Name, igroupName, err)
			continue
		}
		klog.Infof("Successfully unmapped LUN %s from igroup %s", targetVolume.Name, igroupName)
	}

	return nil
}

// GetMappedGroups returns the igroups the LUN is mapped to
func (n *NetAppStorageProvider) GetMappedGroups(targetVolume storage.Volume, mappingCtx storage.MappingContext) ([]string, error) {
	ctx := context.Background()

	lun, err := n.getLUNByName(ctx, targetVolume.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to get LUN %s: %w", targetVolume.Name, err)
	}

	var response struct {
		Records []struct {
			Igroup struct {
				Name string `json:"name"`
			} `json:"igroup"`
		} `json:"records"`
	}

	endpoint := fmt.Sprintf("/protocols/san/lun-maps?lun.uuid=%s", lun.UUID)
	err = n.DoRequestJSON(ctx, "GET", endpoint, nil, &response)
	if err != nil {
		return nil, fmt.Errorf("failed to get LUN mappings: %w", err)
	}

	var groups []string
	for _, record := range response.Records {
		groups = append(groups, record.Igroup.Name)
	}

	return groups, nil
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
		if strings.ToUpper(lun.SerialNumber) == strings.ToUpper(serial) {
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

// BuildNAA constructs a NAA identifier from a NetApp serial number
// NetApp serial numbers are ASCII and need to be hex-encoded for NAA
func (n *NetAppStorageProvider) BuildNAA(serialNumber string) string {
	hexSerial := hex.EncodeToString([]byte(serialNumber))
	return fmt.Sprintf("naa.%s%s", n.Config.NAAPrefix, strings.ToLower(hexSerial))
}

// ExtractSerialFromNAA extracts the serial number from a NAA identifier
// Decodes the hex-encoded serial back to ASCII
func (n *NetAppStorageProvider) ExtractSerialFromNAA(naaID string) (string, error) {
	prefix := "naa." + n.Config.NAAPrefix
	if !strings.HasPrefix(strings.ToLower(naaID), strings.ToLower(prefix)) {
		return "", fmt.Errorf("NAA ID %s is not from NetApp (expected prefix: %s)", naaID, prefix)
	}

	hexSerial := strings.TrimPrefix(strings.ToLower(naaID), strings.ToLower(prefix))
	serialBytes, err := hex.DecodeString(hexSerial)
	if err != nil {
		return "", fmt.Errorf("failed to decode NAA serial: %w", err)
	}

	return string(serialBytes), nil
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
	endpoint := "/storage/luns?fields=serial_number,space,location,create_time,svm"
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
	err := n.DoRequestJSON(ctx, "GET", "/protocols/san/igroups?fields=uuid,name,protocol,svm,initiators", nil, &response)
	if err != nil {
		return nil, err
	}

	return response.Records, nil
}

// getLUNByName retrieves a LUN by its name
func (n *NetAppStorageProvider) getLUNByName(ctx context.Context, name string) (*OntapLUN, error) {
	var luns []OntapLUN
	var err error

	// If name is a full path (starts with /), use exact match
	// Otherwise, use wildcard search since NetApp requires path format
	if strings.HasPrefix(name, "/") {
		luns, err = n.listLUNs(ctx, fmt.Sprintf("name=%s", name))
	} else {
		// Use wildcard search for non-path names (e.g., "volume-xxx-cinder")
		luns, err = n.listLUNs(ctx, fmt.Sprintf("name=*%s*", name))
	}

	if err != nil {
		return nil, err
	}
	if len(luns) == 0 {
		return nil, fmt.Errorf("LUN %s not found", name)
	}
	return &luns[0], nil
}

// getIgroupByName retrieves an igroup by its name
func (n *NetAppStorageProvider) getIgroupByName(ctx context.Context, name string) (*OntapIgroup, error) {
	igroups, err := n.listIgroups(ctx)
	if err != nil {
		return nil, err
	}
	for _, ig := range igroups {
		if ig.Name == name {
			return &ig, nil
		}
	}
	return nil, fmt.Errorf("igroup %s not found", name)
}

// getDefaultVolumePathAndSVM returns the /vol/<flexvol> path and SVM name to
// target for LUN creation. Resolution order:
//  1. Explicit n.SVM + n.FlexVol (configured via ProviderOptions).
//  2. Probe existing LUNs and use the first LUN's SVM + FlexVol.
//  3. Auto-pick when exactly one SVM with exactly one FlexVol is available.
//
// Returns an error if none of the above succeed so the caller can surface a
// clear "please configure SVM/FlexVol" message.
func (n *NetAppStorageProvider) getDefaultVolumePathAndSVM(ctx context.Context) (string, string, error) {
	// 1. Explicit configuration wins.
	if n.SVM != "" && n.FlexVol != "" {
		return fmt.Sprintf("/vol/%s", n.FlexVol), n.SVM, nil
	}

	// 2. LUN-based probe (legacy behaviour).
	if path, svm, err := n.probeTargetFromExistingLUNs(ctx); err == nil {
		return path, svm, nil
	} else {
		klog.V(2).Infof("LUN probe for SVM/FlexVol unavailable: %v", err)
	}

	// 3. Auto-pick when the array has exactly one SVM with exactly one FlexVol.
	groups, err := n.DiscoverBackendTargets(ctx)
	if err != nil {
		return "", "", fmt.Errorf("failed to discover SVMs/FlexVols: %w", err)
	}
	if len(groups) == 1 && len(groups[0].Children) == 1 {
		svm := groups[0].Name
		flexvol := groups[0].Children[0].Name
		klog.Infof("Auto-picked single SVM %q with single FlexVol %q for NetApp target", svm, flexvol)
		return fmt.Sprintf("/vol/%s", flexvol), svm, nil
	}

	return "", "", fmt.Errorf(
		"NetApp SVM and FlexVol are not configured and cannot be auto-detected (found %d SVM(s)); "+
			"set ArrayCreds.spec.netAppConfig.svm and flexVol explicitly", len(groups))
}

// probeTargetFromExistingLUNs inspects existing LUNs on the array and returns
// the /vol/<flexvol> path and SVM name of the first LUN. Used as a legacy
// fallback when explicit configuration is absent.
func (n *NetAppStorageProvider) probeTargetFromExistingLUNs(ctx context.Context) (string, string, error) {
	luns, err := n.listLUNs(ctx, "")
	if err != nil {
		return "", "", fmt.Errorf("failed to list LUNs: %w", err)
	}

	if len(luns) == 0 {
		return "", "", fmt.Errorf("no existing LUNs found to determine volume path and SVM")
	}

	lun := luns[0]

	// LUN name format: /vol/<volume_name>/<lun_name>
	// Extract /vol/<volume_name>
	parts := strings.Split(lun.Name, "/")
	if len(parts) < 4 || parts[1] != "vol" {
		return "", "", fmt.Errorf("unexpected LUN path format: %s", lun.Name)
	}

	volumePath := fmt.Sprintf("/vol/%s", parts[2])
	svmName := lun.SVM.Name

	if svmName == "" {
		return "", "", fmt.Errorf("SVM name not found for LUN: %s", lun.Name)
	}

	klog.Infof("Discovered NetApp volume path: %s, SVM: %s from LUN: %s", volumePath, svmName, lun.Name)

	return volumePath, svmName, nil
}

// ListSVMs returns all data SVMs on the ONTAP cluster.
// Uses ONTAP REST: GET /svm/svms
func (n *NetAppStorageProvider) ListSVMs(ctx context.Context) ([]OntapSVM, error) {
	var response OntapSVMResponse
	err := n.DoRequestJSON(ctx, "GET", "/svm/svms?fields=name,uuid", nil, &response)
	if err != nil {
		return nil, fmt.Errorf("failed to list SVMs: %w", err)
	}
	return response.Records, nil
}

// ListFlexVolsForSVM returns FlexVols (style=flexvol) on the given SVM.
// FlexGroups and other styles are excluded since LUN creation requires a
// plain FlexVol.
// Uses ONTAP REST: GET /storage/volumes?svm.name=<svm>&style=flexvol
func (n *NetAppStorageProvider) ListFlexVolsForSVM(ctx context.Context, svmName string) ([]OntapFlexVol, error) {
	if svmName == "" {
		return nil, fmt.Errorf("svmName is required")
	}
	endpoint := fmt.Sprintf("/storage/volumes?svm.name=%s&style=flexvol&fields=name,uuid,style,svm", svmName)
	var response OntapFlexVolResponse
	err := n.DoRequestJSON(ctx, "GET", endpoint, nil, &response)
	if err != nil {
		return nil, fmt.Errorf("failed to list FlexVols for SVM %s: %w", svmName, err)
	}
	return response.Records, nil
}

// DiscoverBackendTargets returns a two-level SVM -> FlexVol tree suitable for
// user selection. Implements storage.BackendTargetDiscoverer.
func (n *NetAppStorageProvider) DiscoverBackendTargets(ctx context.Context) ([]storage.BackendTargetGroup, error) {
	svms, err := n.ListSVMs(ctx)
	if err != nil {
		return nil, err
	}
	groups := make([]storage.BackendTargetGroup, 0, len(svms))
	for _, svm := range svms {
		flexvols, err := n.ListFlexVolsForSVM(ctx, svm.Name)
		if err != nil {
			klog.Warningf("Failed to list FlexVols for SVM %s: %v", svm.Name, err)
			continue
		}
		children := make([]storage.BackendTarget, 0, len(flexvols))
		for _, fv := range flexvols {
			children = append(children, storage.BackendTarget{Name: fv.Name, UUID: fv.UUID})
		}
		groups = append(groups, storage.BackendTargetGroup{
			Name:     svm.Name,
			UUID:     svm.UUID,
			Children: children,
		})
	}
	return groups, nil
}
