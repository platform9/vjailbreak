// Copyright © 2026 The vjailbreak authors

package migrate

import (
	"context"
	"testing"

	gomock "github.com/golang/mock/gomock"
	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	"github.com/platform9/vjailbreak/pkg/vpwned/sdk/storage"
	"github.com/platform9/vjailbreak/pkg/vpwned/sdk/storage/cinder"
	"github.com/platform9/vjailbreak/v2v-helper/openstack"
)

// fakeCoreProvider implements only the core StorageProvider interface —
// the shape of a new "Cinder-only" vendor.
type fakeCoreProvider struct{ name string }

func (f *fakeCoreProvider) Connect(context.Context, storage.StorageAccessInfo) error { return nil }
func (f *fakeCoreProvider) Disconnect() error                                        { return nil }
func (f *fakeCoreProvider) ValidateCredentials(context.Context) error                { return nil }
func (f *fakeCoreProvider) CreateVolume(string, int64) (storage.Volume, error) {
	return storage.Volume{}, nil
}
func (f *fakeCoreProvider) DeleteVolume(string) error { return nil }
func (f *fakeCoreProvider) GetVolumeInfo(string) (storage.VolumeInfo, error) {
	return storage.VolumeInfo{}, nil
}
func (f *fakeCoreProvider) ListAllVolumes() ([]storage.VolumeInfo, error) { return nil, nil }
func (f *fakeCoreProvider) GetAllVolumeNAAs() ([]string, error)           { return nil, nil }
func (f *fakeCoreProvider) ResolveCinderVolumeToLUN(string) (storage.Volume, error) {
	return storage.Volume{}, nil
}
func (f *fakeCoreProvider) WhoAmI() string { return f.name }

// fakeVendorProvider additionally implements storage.VendorMapper (the shape
// of Pure/NetApp) and records calls so adapter forwarding can be asserted.
type fakeVendorProvider struct {
	fakeCoreProvider
	createCalls int
	mapCalls    int
	unmapCalls  int
	lastGroup   string
	lastHBAs    []string
	lastVolume  storage.Volume
	lastMCtx    storage.MappingContext
}

func (f *fakeVendorProvider) CreateOrUpdateInitiatorGroup(group string, hbas []string) (storage.MappingContext, error) {
	f.createCalls++
	f.lastGroup, f.lastHBAs = group, hbas
	return storage.MappingContext{"hosts": []string{"esx01"}}, nil
}

func (f *fakeVendorProvider) MapVolumeToGroup(group string, vol storage.Volume, mctx storage.MappingContext) (storage.Volume, error) {
	f.mapCalls++
	f.lastGroup, f.lastVolume, f.lastMCtx = group, vol, mctx
	return vol, nil
}

func (f *fakeVendorProvider) UnmapVolumeFromGroup(group string, vol storage.Volume, mctx storage.MappingContext) error {
	f.unmapCalls++
	f.lastGroup, f.lastVolume, f.lastMCtx = group, vol, mctx
	return nil
}

func (f *fakeVendorProvider) GetMappedGroups(storage.Volume, storage.MappingContext) ([]string, error) {
	return nil, nil
}

// Compile-time shape checks: the split must keep both fakes valid providers
// and only the vendor fake a VendorMapper.
var (
	_ storage.StorageProvider = (*fakeCoreProvider)(nil)
	_ storage.StorageProvider = (*fakeVendorProvider)(nil)
	_ storage.VendorMapper    = (*fakeVendorProvider)(nil)
)

func TestSelectMapper(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockOps := openstack.NewMockOpenstackOperations(ctrl)

	core := &fakeCoreProvider{name: "hpe"}
	vendorProv := &fakeVendorProvider{fakeCoreProvider: fakeCoreProvider{name: "pure"}}

	tests := []struct {
		name       string
		provider   storage.StorageProvider
		mode       string
		wantCinder bool
		wantErr    bool
		wantDesc   string
	}{
		{"empty mode with vendor mapper -> native", vendorProv, "", false, false, "vendor-native (pure)"},
		{"auto with vendor mapper -> native", vendorProv, vjailbreakv1alpha1.MappingModeAuto, false, false, "vendor-native (pure)"},
		{"empty mode without vendor mapper -> cinder", core, "", true, false, "cinder fallback (hpe)"},
		{"auto without vendor mapper -> cinder", core, vjailbreakv1alpha1.MappingModeAuto, true, false, "cinder fallback (hpe)"},
		{"native with vendor mapper -> native", vendorProv, vjailbreakv1alpha1.MappingModeNative, false, false, "vendor-native (pure)"},
		{"native without vendor mapper -> error", core, vjailbreakv1alpha1.MappingModeNative, false, true, ""},
		{"cinder forces fallback on vendor mapper", vendorProv, vjailbreakv1alpha1.MappingModeCinder, true, false, "cinder fallback (pure)"},
		{"cinder on core-only provider", core, vjailbreakv1alpha1.MappingModeCinder, true, false, "cinder fallback (hpe)"},
		{"unknown mode -> error", core, "bogus", false, true, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mapper, desc, err := selectMapper(tt.provider, mockOps, tt.mode, "10.4.2.17")
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got mapper %T (%s)", mapper, desc)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if desc != tt.wantDesc {
				t.Fatalf("description mismatch: got %q want %q", desc, tt.wantDesc)
			}
			cm, isCinder := mapper.(*cinder.CinderMapper)
			if isCinder != tt.wantCinder {
				t.Fatalf("mapper type mismatch: got %T, wantCinder=%v", mapper, tt.wantCinder)
			}
			if isCinder {
				if cm.Host != "vjailbreak-10-4-2-17" {
					t.Fatalf("cinder connector host not derived from ESXi IP: %q", cm.Host)
				}
				if cm.IP != "10.4.2.17" {
					t.Fatalf("cinder connector ip not set: %q", cm.IP)
				}
			} else {
				if _, ok := mapper.(*vendorMapperAdapter); !ok {
					t.Fatalf("expected vendorMapperAdapter, got %T", mapper)
				}
			}
		})
	}

	t.Run("nil provider -> error", func(t *testing.T) {
		if _, _, err := selectMapper(nil, mockOps, "", "10.4.2.17"); err == nil {
			t.Fatal("expected error for nil provider")
		}
	})

	t.Run("nil openstack clients on cinder path -> error", func(t *testing.T) {
		if _, _, err := selectMapper(core, nil, vjailbreakv1alpha1.MappingModeCinder, "10.4.2.17"); err == nil {
			t.Fatal("expected error for nil openstack clients")
		}
	})

	t.Run("nil openstack clients on native path is fine", func(t *testing.T) {
		if _, _, err := selectMapper(vendorProv, nil, "", "10.4.2.17"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestVendorMapperAdapterForwards(t *testing.T) {
	fake := &fakeVendorProvider{fakeCoreProvider: fakeCoreProvider{name: "pure"}}
	mapper, _, err := selectMapper(fake, nil, "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := context.Background()
	hbas := []string{"iqn.1998-01.com.vmware:esx01"}

	mctx, err := mapper.CreateOrUpdateInitiatorGroup(ctx, "grp", hbas)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fake.createCalls != 1 || fake.lastGroup != "grp" || len(fake.lastHBAs) != 1 {
		t.Fatalf("CreateOrUpdateInitiatorGroup not forwarded: %+v", fake)
	}
	if _, ok := mctx["hosts"]; !ok {
		t.Fatalf("vendor mapping context not passed through: %v", mctx)
	}

	vol := storage.Volume{Name: "v1"}
	if _, err := mapper.MapVolumeToGroup(ctx, "grp", vol, mctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fake.mapCalls != 1 || fake.lastVolume.Name != "v1" {
		t.Fatalf("MapVolumeToGroup not forwarded: %+v", fake)
	}

	if err := mapper.UnmapVolumeFromGroup(ctx, "grp", vol, mctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fake.unmapCalls != 1 {
		t.Fatalf("UnmapVolumeFromGroup not forwarded: %+v", fake)
	}
}

func TestCinderPoolHintFromArrayCreds(t *testing.T) {
	mk := func(pool, host string) vjailbreakv1alpha1.ArrayCreds {
		ac := vjailbreakv1alpha1.ArrayCreds{}
		ac.Spec.OpenStackMapping.CinderBackendPool = pool
		ac.Spec.OpenStackMapping.CinderHost = host
		return ac
	}
	tests := []struct {
		name string
		ac   vjailbreakv1alpha1.ArrayCreds
		want string
	}{
		{"dedicated pool field wins", mk("poolA", "pcd@hbsd-1#poolB"), "poolA"},
		{"pool parsed from cinderHost suffix", mk("", "pcd@hbsd-1#dp-pool-1"), "dp-pool-1"},
		{"service host without pool yields empty", mk("", "55f61998@hbsd-1"), ""},
		{"trailing hash yields empty", mk("", "pcd@hbsd-1#"), ""},
		{"nothing configured yields empty", mk("", ""), ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cinderPoolHintFromArrayCreds(tt.ac); got != tt.want {
				t.Fatalf("cinderPoolHintFromArrayCreds() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestConnectorHostForESXi(t *testing.T) {
	tests := []struct{ in, want string }{
		{"10.4.2.17", "vjailbreak-10-4-2-17"},
		{"fd00::17", "vjailbreak-fd00--17"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := connectorHostForESXi(tt.in); got != tt.want {
			t.Fatalf("connectorHostForESXi(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
