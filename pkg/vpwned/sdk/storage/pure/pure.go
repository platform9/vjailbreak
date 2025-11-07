package pure

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/devans10/pugo/flasharray"
	"github.com/platform9/vjailbreak/pkg/vpwned/sdk/storage"
	"k8s.io/klog/v2"
)

const FlashProviderID = "624a9370"

func init() {
	storage.RegisterStorageProvider("pure", &PureStorageProvider{})
}

// PureStorageProvider implements StorageProvider for Pure Storage FlashArray
type PureStorageProvider struct {
	client      *flasharray.Client
	accessInfo  storage.StorageAccessInfo
	isConnected bool
}

// Connect establishes connection to Pure Storage array
func (p *PureStorageProvider) Connect(ctx context.Context, accessInfo storage.StorageAccessInfo) error {
	p.accessInfo = accessInfo
	// Create Pure Storage client
	client, err := flasharray.NewClient(
		accessInfo.Hostname,            // target
		accessInfo.Username,            // username
		accessInfo.Password,            // password
		"",                             // apiToken
		"",                             // restVersion
		true,                           // verifyHTTPS
		accessInfo.SkipSSLVerification, // sslCert
		"",                             // userAgent
		map[string]string{},            // requestKwargs
	)
	if err != nil {
		return fmt.Errorf("failed to create Pure Storage client: %w", err)
	}

	p.client = client
	p.isConnected = true

	// Log array info
	array, err := p.client.Array.Get(nil)
	if err == nil {
		klog.Infof("Connected to Pure Array: %s, ID: %s", array.ArrayName, array.ID)
	}

	return nil
}

// Disconnect closes the connection
func (p *PureStorageProvider) Disconnect() error {
	p.isConnected = false
	return nil
}

// ValidateCredentials validates the credentials
func (p *PureStorageProvider) ValidateCredentials(ctx context.Context) error {
	if !p.isConnected {
		err := p.Connect(ctx, p.accessInfo)
		if err != nil {
			return err
		}
	}

	// Try to get array info as validation
	_, err := p.client.Array.Get(nil)
	if err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}
	return nil
}

// CreateOrUpdateInitiatorGroup creates or updates an initiator group with the ESX adapters
// mapping esxi's hba adapters initiator group to the volume host in pure.
func (p *PureStorageProvider) CreateOrUpdateInitiatorGroup(initiatorGroupName string, hbaIdentifiers []string) (storage.MappingContext, error) {
	hosts, err := p.client.Hosts.ListHosts(nil)
	if err != nil {
		return nil, err
	}

	matchedHosts := []string{}

	for _, h := range hosts {
		klog.Infof("Checking host %s, iqns: %v, wwns: %v", h.Name, h.Iqn, h.Wwn)

		// Check IQNs
		for _, iqn := range h.Iqn {
			if slices.Contains(hbaIdentifiers, iqn) {
				klog.Infof("Adding host %s to group (matched IQN: %s)", h.Name, iqn)
				matchedHosts = append(matchedHosts, h.Name)
				break
			}
		}

		// Check WWNs (Fibre Channel)
		for _, wwn := range h.Wwn {
			for _, hostAdapter := range hbaIdentifiers {
				if !strings.HasPrefix(hostAdapter, "fc.") {
					continue
				}
				adapterWWPN, err := fcUIDToWWPN(hostAdapter)
				if err != nil {
					klog.Warningf("Failed to extract WWPN from adapter %s: %s", hostAdapter, err)
					continue
				}

				// Format WWNs consistently for comparison
				formattedHostWwn := strings.ReplaceAll(strings.ToUpper(wwn), ":", "")
				formattedAdapterWwpn := strings.ReplaceAll(adapterWWPN, ":", "")

				klog.Infof("Comparing ESX adapter WWPN %s with Pure host WWN %s", formattedAdapterWwpn, formattedHostWwn)
				if formattedAdapterWwpn == formattedHostWwn {
					klog.Infof("Match found. Adding host %s to mapping context.", h.Name)
					matchedHosts = append(matchedHosts, h.Name)
					break
				}
			}
		}
	}

	if len(matchedHosts) == 0 {
		return nil, fmt.Errorf("no hosts found matching any of the provided IQNs/FC adapters: %v", hbaIdentifiers)
	}

	return storage.MappingContext{"hosts": matchedHosts}, nil
}

// MapVolumeToGroup maps a volume to hosts (not groups in Pure's case)
func (p *PureStorageProvider) MapVolumeToGroup(initiatorGroupName string, targetVolume storage.Volume, context storage.MappingContext) (storage.Volume, error) {
	hostsVal, ok := context["hosts"]
	if !ok {
		return storage.Volume{}, errors.New("hosts not found in mapping context")
	}

	hosts, ok := hostsVal.([]string)
	if !ok || len(hosts) == 0 {
		return storage.Volume{}, errors.New("invalid or empty hosts list in mapping context")
	}

	for _, host := range hosts {
		klog.Infof("Connecting host %s to volume %s", host, targetVolume.Name)
		_, err := p.client.Hosts.ConnectHost(host, targetVolume.Name, nil)
		if err != nil {
			if strings.Contains(err.Error(), "Connection already exists.") {
				klog.Infof("Connection already exists for host %s and volume %s", host, targetVolume.Name)
				continue
			}
			return storage.Volume{}, fmt.Errorf("connect host %q to volume %q: %w", host, targetVolume.Name, err)
		}
	}

	return targetVolume, nil
}

// UnmapVolumeFromGroup unmaps a volume from hosts
func (p *PureStorageProvider) UnmapVolumeFromGroup(initiatorGroupName string, targetVolume storage.Volume, context storage.MappingContext) error {
	hostsVal, ok := context["hosts"]
	if !ok {
		return nil // No hosts to unmap
	}

	hosts, ok := hostsVal.([]string)
	if !ok || len(hosts) == 0 {
		return nil
	}

	for _, host := range hosts {
		klog.Infof("Disconnecting host %s from volume %s", host, targetVolume.Name)
		_, err := p.client.Hosts.DisconnectHost(host, targetVolume.Name)
		if err != nil {
			return fmt.Errorf("disconnect host %q from volume %q: %w", host, targetVolume.Name, err)
		}
	}

	return nil
}

// GetMappedGroups returns the hosts the volume is mapped to
// We don't use the host group feature in Pure, so this returns nil
func (p *PureStorageProvider) GetMappedGroups(targetVolume storage.Volume, context storage.MappingContext) ([]string, error) {
	// Pure doesn't use host groups for this use case
	// A host can't belong to two separate groups
	return nil, nil
}

// ResolveCinderVolumeToVolume resolves a Cinder volume name to a storage Volume/LUN
func (p *PureStorageProvider) ResolveCinderVolumeToLUN(volumeID string) (storage.Volume, error) {
	// Get volume by name
	// Pure driver adds prefix volume and suffix -cinder to the volume name
	volumeName := fmt.Sprintf("volume-%s-cinder", volumeID)
	v, err := p.client.Volumes.GetVolume(volumeName, nil)
	if err != nil {
		return storage.Volume{}, fmt.Errorf("failed to get volume %s: %w", volumeName, err)
	}

	klog.Infof("Resolved cinder volume %s to volume: %+v", volumeName, v)

	lun := storage.Volume{
		Name:         v.Name,
		SerialNumber: v.Serial,
		NAA:          fmt.Sprintf("naa.%s%s", FlashProviderID, strings.ToLower(v.Serial)),
	}

	return lun, nil
}

// WhoAmI returns the provider name
func (p *PureStorageProvider) WhoAmI() string {
	return "pure"
}

// fcUIDToWWPN extracts the WWPN (port name) from an ESXi fcUid string.
// The expected input is of the form: 'fc.WWNN:WWPN' where the WWNN and WWPN
// are not separated with colons every byte (2 hex chars) like 00:00:00:00:00:00:00:00
func fcUIDToWWPN(fcUid string) (string, error) {
	if !strings.HasPrefix(fcUid, "fc.") {
		return "", fmt.Errorf("fcUid %q doesn't start with 'fc.'", fcUid)
	}
	parts := strings.Split(fcUid[3:], ":")
	if len(parts) != 2 || len(parts[1]) == 0 {
		return "", fmt.Errorf("fcUid %q is not in the expected fc.WWNN:WWPN format", fcUid)
	}

	wwpn := strings.ToUpper(parts[1])
	if len(wwpn)%2 != 0 {
		return "", fmt.Errorf("WWPN %q length isn't even", wwpn)
	}

	var formattedParts []string
	for i := 0; i < len(wwpn); i += 2 {
		formattedParts = append(formattedParts, wwpn[i:i+2])
	}
	return strings.Join(formattedParts, ":"), nil
}
