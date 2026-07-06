// Copyright © 2026 The vjailbreak authors

// Package cinder provides a vendor-neutral fallback for the ESXi LUN-mapping
// step of Storage-Accelerated-Copy. Instead of driving the array's own host
// and initiator-group APIs (see the pure and netapp providers), CinderMapper
// delegates map/unmap to the array's Cinder driver via the
// os-initialize_connection and os-terminate_connection volume actions, using
// an os-brick style connector dict built from the ESXi host's HBAs.
//
// Volume creation, deletion, NAA construction and Cinder-volume-to-LUN
// resolution deliberately stay vendor-native (storage.StorageProvider):
// creating the LUN through the array's REST API pins it to the same physical
// array as the source datastore, and the NAA comes from the array's create
// response rather than from driver-specific connection_info.
package cinder

import (
	"context"
	"fmt"
	"strings"

	"github.com/platform9/vjailbreak/pkg/vpwned/sdk/storage"
	"github.com/platform9/vjailbreak/pkg/vpwned/sdk/storage/fcutil"
	"k8s.io/klog/v2"
)

const (
	// DefaultConnectorHost is the connector["host"] value used when the
	// caller does not supply an ESXi-specific identity. Callers should
	// prefer a per-ESXi value (e.g. "vjailbreak-<esxi-ip>"): Cinder drivers
	// assume one connector host value per physical host.
	DefaultConnectorHost = "vjailbreak-xcopy"

	// ConnectorKey is the MappingContext key under which the os-brick style
	// connector dict is stored between CreateOrUpdateInitiatorGroup and the
	// map/unmap calls. The same connector must be presented on both
	// os-initialize_connection and os-terminate_connection so the driver
	// can identify the export to tear down.
	ConnectorKey = "connector"
)

// CinderActionClient is the minimal Cinder surface CinderMapper needs.
// v2v-helper's *utils.OpenStackClients satisfies it structurally.
type CinderActionClient interface {
	InitializeVolumeConnection(ctx context.Context, volumeID string, connector map[string]any) (map[string]any, error)
	TerminateVolumeConnection(ctx context.Context, volumeID string, connector map[string]any) error
}

// CinderMapper exposes/unexposes target LUNs to an ESXi host through the
// array's Cinder driver. It is selected by v2v-helper when the storage
// provider does not implement storage.VendorMapper (MappingMode auto), or
// unconditionally when MappingMode=cinder forces the fallback.
type CinderMapper struct {
	Client CinderActionClient
	// Host is placed in connector["host"]. Empty means DefaultConnectorHost.
	Host string
	// IP is placed in connector["ip"] when non-empty (the ESXi host IP).
	IP string
}

// CreateOrUpdateInitiatorGroup builds the os-brick connector dict from the
// ESXi HBA identifiers and stashes it in the MappingContext. No array or
// Cinder call is made: host/initiator-group management is the Cinder
// driver's responsibility, so the initiatorGroupName is unused.
func (m *CinderMapper) CreateOrUpdateInitiatorGroup(_ context.Context, _ string, hbaIdentifiers []string) (storage.MappingContext, error) {
	connector, err := BuildConnectorFromHBAs(hbaIdentifiers, m.Host, m.IP)
	if err != nil {
		return nil, err
	}
	return storage.MappingContext{ConnectorKey: connector}, nil
}

// MapVolumeToGroup exposes the volume to the connector's host via
// os-initialize_connection. The returned connection_info is intentionally
// discarded: ESXi discovers the device by NAA on rescan, and the NAA comes
// from the vendor CreateVolume response.
func (m *CinderMapper) MapVolumeToGroup(ctx context.Context, _ string, targetVolume storage.Volume, mappingContext storage.MappingContext) (storage.Volume, error) {
	connector, err := connectorFromContext(mappingContext)
	if err != nil {
		return storage.Volume{}, err
	}
	volumeID := targetVolume.OpenstackVol.ID
	if volumeID == "" {
		return storage.Volume{}, fmt.Errorf("cinder mapper: volume %q has no Cinder volume ID; it must be managed into Cinder before mapping", targetVolume.Name)
	}
	// Log the full connector: os-initialize_connection creates no Cinder
	// attachment record, so if the process dies before the deferred unmap
	// this line is what support needs to hand-run os-terminate_connection.
	klog.Infof("Cinder connector: volume=%s connector=%v", volumeID, connector)
	if _, err := m.Client.InitializeVolumeConnection(ctx, volumeID, connector); err != nil {
		return storage.Volume{}, fmt.Errorf("cinder mapper: os-initialize_connection failed for volume %s: %w", volumeID, err)
	}
	return targetVolume, nil
}

// UnmapVolumeFromGroup removes the export via os-terminate_connection using
// the same connector that established it.
func (m *CinderMapper) UnmapVolumeFromGroup(ctx context.Context, _ string, targetVolume storage.Volume, mappingContext storage.MappingContext) error {
	connector, err := connectorFromContext(mappingContext)
	if err != nil {
		return err
	}
	volumeID := targetVolume.OpenstackVol.ID
	if volumeID == "" {
		return fmt.Errorf("cinder mapper: volume %q has no Cinder volume ID; cannot terminate connection", targetVolume.Name)
	}
	if err := m.Client.TerminateVolumeConnection(ctx, volumeID, connector); err != nil {
		return fmt.Errorf("cinder mapper: os-terminate_connection failed for volume %s: %w", volumeID, err)
	}
	return nil
}

func connectorFromContext(mappingContext storage.MappingContext) (map[string]any, error) {
	connector, ok := mappingContext[ConnectorKey].(map[string]any)
	if !ok || len(connector) == 0 {
		return nil, fmt.Errorf("cinder mapper: mapping context has no %q entry; CreateOrUpdateInitiatorGroup must run first", ConnectorKey)
	}
	return connector, nil
}

// BuildConnectorFromHBAs converts the ESXi HBA identifiers returned by
// esxi-ssh GetAllHostAdapters (lowercase "iqn...." or "fc.WWNN:WWPN") into an
// os-brick style connector dict. Conventions deliberately mirror os-brick so
// the connector stays inside the value space Cinder drivers are tested with:
// WWNs are lowercase colon-stripped hex, multipath is always true (some FC
// drivers truncate the WWPN list to a single path without it), and
// platform/os_type match a Linux os-brick host. Malformed FC identifiers are
// skipped with a warning; it is an error if no usable initiator remains.
func BuildConnectorFromHBAs(hbaIdentifiers []string, host, ip string) (map[string]any, error) {
	var iqns []string
	var wwpns, wwnns []string

	for _, hba := range hbaIdentifiers {
		hba = strings.TrimSpace(hba)
		switch {
		case hba == "":
			continue
		case strings.HasPrefix(strings.ToLower(hba), "iqn."):
			iqns = append(iqns, strings.ToLower(hba))
		case strings.HasPrefix(strings.ToLower(hba), "fc."):
			wwnn, wwpn, err := fcutil.ParseFCUID(hba)
			if err != nil {
				klog.Warningf("cinder mapper: skipping malformed FC adapter UID %q: %v", hba, err)
				continue
			}
			// os-brick convention: lowercase hex, no separators.
			wwnns = append(wwnns, strings.ToLower(fcutil.StripWWNFormatting(wwnn)))
			wwpns = append(wwpns, strings.ToLower(fcutil.StripWWNFormatting(wwpn)))
		default:
			klog.Warningf("cinder mapper: skipping unrecognised HBA identifier %q (expected iqn.* or fc.*)", hba)
		}
	}

	if len(iqns) == 0 && len(wwpns) == 0 {
		return nil, fmt.Errorf("cinder mapper: no usable iSCSI or FC initiators in %v", hbaIdentifiers)
	}

	if host == "" {
		host = DefaultConnectorHost
	}

	connector := map[string]any{
		"host":      host,
		"platform":  "x86_64",
		"os_type":   "linux",
		"multipath": true,
	}
	if ip != "" {
		connector["ip"] = ip
	}
	if len(iqns) > 0 {
		connector["initiator"] = iqns[0]
	}
	if len(wwpns) > 0 {
		connector["wwpns"] = wwpns
		connector["wwnns"] = wwnns
	}
	return connector, nil
}
