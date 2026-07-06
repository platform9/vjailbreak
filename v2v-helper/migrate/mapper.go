// Copyright © 2026 The vjailbreak authors

package migrate

import (
	"context"
	"fmt"
	"strings"

	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	"github.com/platform9/vjailbreak/pkg/vpwned/sdk/storage"
	"github.com/platform9/vjailbreak/pkg/vpwned/sdk/storage/cinder"
	"github.com/platform9/vjailbreak/v2v-helper/openstack"
)

// Mapper abstracts the three mapping operations the Storage-Accelerated-Copy
// flow needs to expose a target LUN to the ESXi host. It is deliberately
// context-aware (unlike storage.VendorMapper) so the Cinder fallback gets
// cancellation/timeouts; vendor-native providers are bridged by
// vendorMapperAdapter without any change to their code.
type Mapper interface {
	CreateOrUpdateInitiatorGroup(ctx context.Context, initiatorGroupName string, hbaIdentifiers []string) (storage.MappingContext, error)
	MapVolumeToGroup(ctx context.Context, initiatorGroupName string, targetVolume storage.Volume, mappingContext storage.MappingContext) (storage.Volume, error)
	UnmapVolumeFromGroup(ctx context.Context, initiatorGroupName string, targetVolume storage.Volume, mappingContext storage.MappingContext) error
}

// vendorMapperAdapter bridges a vendor-native storage.VendorMapper (Pure,
// NetApp) to the ctx-aware Mapper interface. Vendor methods are ctx-less by
// design; the adapter drops the context.
type vendorMapperAdapter struct {
	vendor storage.VendorMapper
}

func (a *vendorMapperAdapter) CreateOrUpdateInitiatorGroup(_ context.Context, initiatorGroupName string, hbaIdentifiers []string) (storage.MappingContext, error) {
	return a.vendor.CreateOrUpdateInitiatorGroup(initiatorGroupName, hbaIdentifiers)
}

func (a *vendorMapperAdapter) MapVolumeToGroup(_ context.Context, initiatorGroupName string, targetVolume storage.Volume, mappingContext storage.MappingContext) (storage.Volume, error) {
	return a.vendor.MapVolumeToGroup(initiatorGroupName, targetVolume, mappingContext)
}

func (a *vendorMapperAdapter) UnmapVolumeFromGroup(_ context.Context, initiatorGroupName string, targetVolume storage.Volume, mappingContext storage.MappingContext) error {
	return a.vendor.UnmapVolumeFromGroup(initiatorGroupName, targetVolume, mappingContext)
}

// connectorHostForESXi derives the Cinder connector["host"] identity for an
// ESXi host. Cinder drivers assume one connector host value per physical
// host (that is the invariant Nova upholds), so the value must be unique per
// ESXi. Empty input falls back to the CinderMapper default.
func connectorHostForESXi(esxiHostIP string) string {
	if esxiHostIP == "" {
		return ""
	}
	r := strings.NewReplacer(".", "-", ":", "-")
	return "vjailbreak-" + r.Replace(esxiHostIP)
}

// selectMapper picks how target LUNs are exposed to the ESXi host during
// Storage-Accelerated-Copy, based on the ArrayCreds MappingMode:
//
//	""/auto → vendor-native when the provider implements it, else Cinder
//	native  → vendor-native, or an error when the provider lacks it
//	cinder  → Cinder fallback, unconditionally
//
// It returns the mapper plus a human-readable description for logging, e.g.
// "vendor-native (pure)" or "cinder fallback (pure)".
func selectMapper(provider storage.StorageProvider, osClients openstack.OpenstackOperations, mode string, esxiHostIP string) (Mapper, string, error) {
	if provider == nil {
		return nil, "", fmt.Errorf("selectMapper: storage provider not initialized")
	}

	cinderMapper := func() (Mapper, string, error) {
		if osClients == nil {
			return nil, "", fmt.Errorf("selectMapper: OpenStack clients not initialized; required for the Cinder mapping fallback")
		}
		return &cinder.CinderMapper{
			Client: osClients,
			Host:   connectorHostForESXi(esxiHostIP),
			IP:     esxiHostIP,
		}, fmt.Sprintf("cinder fallback (%s)", provider.WhoAmI()), nil
	}

	vendor, isVendorMapper := provider.(storage.VendorMapper)
	switch mode {
	case "", vjailbreakv1alpha1.MappingModeAuto:
		if isVendorMapper {
			return &vendorMapperAdapter{vendor: vendor}, fmt.Sprintf("vendor-native (%s)", provider.WhoAmI()), nil
		}
		return cinderMapper()
	case vjailbreakv1alpha1.MappingModeNative:
		if isVendorMapper {
			return &vendorMapperAdapter{vendor: vendor}, fmt.Sprintf("vendor-native (%s)", provider.WhoAmI()), nil
		}
		return nil, "", fmt.Errorf("MappingMode=native but provider %s has no vendor-native mapper", provider.WhoAmI())
	case vjailbreakv1alpha1.MappingModeCinder:
		return cinderMapper()
	default:
		return nil, "", fmt.Errorf("unknown MappingMode: %q", mode)
	}
}
