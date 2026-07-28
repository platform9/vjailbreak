// Copyright © 2024 The vjailbreak authors

package openstack

import (
	"errors"
	"testing"

	"github.com/gophercloud/gophercloud/v2/openstack/compute/v2/flavors"
	pkgerrors "github.com/pkg/errors"
)

func TestGetPassthroughGPUCount(t *testing.T) {
	tests := []struct {
		name     string
		flavor   flavors.Flavor
		expected int
	}{
		{
			name: "passthrough GPU with count 1",
			flavor: flavors.Flavor{
				ExtraSpecs: map[string]string{
					"pci_passthrough:alias": "nvidia-l4:1",
				},
			},
			expected: 1,
		},
		{
			name: "passthrough GPU with count 2",
			flavor: flavors.Flavor{
				ExtraSpecs: map[string]string{
					"pci_passthrough:alias": "nvidia-a100:2",
				},
			},
			expected: 2,
		},
		{
			name: "no passthrough GPU",
			flavor: flavors.Flavor{
				ExtraSpecs: map[string]string{
					"resources:VGPU": "1",
				},
			},
			expected: 0,
		},
		{
			name: "invalid format",
			flavor: flavors.Flavor{
				ExtraSpecs: map[string]string{
					"pci_passthrough:alias": "nvidia-l4",
				},
			},
			expected: 0,
		},
		{
			name:     "nil extra_specs",
			flavor:   flavors.Flavor{},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getPassthroughGPUCount(tt.flavor)
			if result != tt.expected {
				t.Errorf("getPassthroughGPUCount() = %d, want %d", result, tt.expected)
			}
		})
	}
}

func TestGetVGPUCount(t *testing.T) {
	tests := []struct {
		name     string
		flavor   flavors.Flavor
		expected int
	}{
		{
			name: "vGPU with count 1",
			flavor: flavors.Flavor{
				ExtraSpecs: map[string]string{
					"resources:VGPU": "1",
				},
			},
			expected: 1,
		},
		{
			name: "vGPU with count 2",
			flavor: flavors.Flavor{
				ExtraSpecs: map[string]string{
					"resources:VGPU": "2",
				},
			},
			expected: 2,
		},
		{
			name: "no vGPU",
			flavor: flavors.Flavor{
				ExtraSpecs: map[string]string{
					"pci_passthrough:alias": "nvidia-l4:1",
				},
			},
			expected: 0,
		},
		{
			name: "invalid format",
			flavor: flavors.Flavor{
				ExtraSpecs: map[string]string{
					"resources:VGPU": "invalid",
				},
			},
			expected: 0,
		},
		{
			name:     "nil extra_specs",
			flavor:   flavors.Flavor{},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getVGPUCount(tt.flavor)
			if result != tt.expected {
				t.Errorf("getVGPUCount() = %d, want %d", result, tt.expected)
			}
		})
	}
}

func TestFilterFlavorsByAvailabilityZone(t *testing.T) {
	global := flavors.Flavor{ID: "global"}
	gpuOnly := flavors.Flavor{
		ID:         "gpu-only",
		ExtraSpecs: map[string]string{"resources:VGPU": "1"},
	}
	azTest := flavors.Flavor{
		ID:         "az-test",
		ExtraSpecs: map[string]string{"availability_zone": "vjb-test"},
	}
	azProd := flavors.Flavor{
		ID:         "az-prod",
		ExtraSpecs: map[string]string{"availability_zone": "vjb-prod", "pf9-managed": "true"},
	}
	all := []flavors.Flavor{global, gpuOnly, azTest, azProd}

	tests := []struct {
		name     string
		targetAZ string
		want     []string
	}{
		{
			name:     "vjb-test target keeps global, gpu-only, and az-test",
			targetAZ: "vjb-test",
			want:     []string{"global", "gpu-only", "az-test"},
		},
		{
			name:     "vjb-prod target keeps global, gpu-only, and az-prod",
			targetAZ: "vjb-prod",
			want:     []string{"global", "gpu-only", "az-prod"},
		},
		{
			name:     "unknown AZ keeps only AZ-less flavors",
			targetAZ: "vjb-other",
			want:     []string{"global", "gpu-only"},
		},
		{
			name:     "empty targetAZ disables filtering",
			targetAZ: "",
			want:     []string{"global", "gpu-only", "az-test", "az-prod"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FilterFlavorsByAvailabilityZone(all, tt.targetAZ)
			gotIDs := make([]string, len(got))
			for i, f := range got {
				gotIDs[i] = f.ID
			}
			if len(gotIDs) != len(tt.want) {
				t.Fatalf("got %v, want %v", gotIDs, tt.want)
			}
			for i := range gotIDs {
				if gotIDs[i] != tt.want[i] {
					t.Errorf("got %v, want %v", gotIDs, tt.want)
					return
				}
			}
		})
	}
}

func TestGetClosestFlavourWithGPU(t *testing.T) {
	allFlavors := []flavors.Flavor{
		{
			ID:    "flavor-1",
			Name:  "m1.small",
			VCPUs: 2,
			RAM:   2048,
		},
		{
			ID:    "flavor-2",
			Name:  "gpu.small",
			VCPUs: 4,
			RAM:   4096,
			ExtraSpecs: map[string]string{
				"pci_passthrough:alias": "nvidia-l4:1",
			},
		},
		{
			ID:    "flavor-3",
			Name:  "vgpu.small",
			VCPUs: 4,
			RAM:   4096,
			ExtraSpecs: map[string]string{
				"resources:VGPU": "1",
			},
		},
		{
			ID:    "flavor-4",
			Name:  "gpu.large",
			VCPUs: 8,
			RAM:   8192,
			ExtraSpecs: map[string]string{
				"pci_passthrough:alias": "nvidia-a100:2",
			},
		},
	}

	tests := []struct {
		name                string
		cpu                 int
		memory              int
		passthroughGPUCount int
		vgpuCount           int
		useGPUFlavor        bool
		expectedFlavorID    string
		expectError         bool
	}{
		{
			name:                "match passthrough GPU count 1",
			cpu:                 2,
			memory:              2048,
			passthroughGPUCount: 1,
			vgpuCount:           0,
			useGPUFlavor:        false,
			expectedFlavorID:    "flavor-2",
			expectError:         false,
		},
		{
			name:                "match vGPU count 1",
			cpu:                 2,
			memory:              2048,
			passthroughGPUCount: 0,
			vgpuCount:           1,
			useGPUFlavor:        false,
			expectedFlavorID:    "flavor-3",
			expectError:         false,
		},
		{
			name:                "match passthrough GPU count 2",
			cpu:                 4,
			memory:              4096,
			passthroughGPUCount: 2,
			vgpuCount:           0,
			useGPUFlavor:        false,
			expectedFlavorID:    "flavor-4",
			expectError:         false,
		},
		{
			name:                "no GPU required - should omit GPU flavors",
			cpu:                 2,
			memory:              2048,
			passthroughGPUCount: 0,
			vgpuCount:           0,
			useGPUFlavor:        false,
			expectedFlavorID:    "flavor-1",
			expectError:         false,
		},
		{
			name:                "no GPU required but useGPUFlavor=true - should select GPU flavor",
			cpu:                 2,
			memory:              2048,
			passthroughGPUCount: 0,
			vgpuCount:           0,
			useGPUFlavor:        true,
			expectedFlavorID:    "flavor-2",
			expectError:         false,
		},
		{
			name:                "GPU requirement not met",
			cpu:                 2,
			memory:              2048,
			passthroughGPUCount: 3,
			vgpuCount:           0,
			useGPUFlavor:        false,
			expectedFlavorID:    "",
			expectError:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flavor, err := GetClosestFlavour(tt.cpu, tt.memory, tt.passthroughGPUCount, tt.vgpuCount, allFlavors, tt.useGPUFlavor)
			
			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if flavor.ID != tt.expectedFlavorID {
				t.Errorf("GetClosestFlavour() = %s, want %s", flavor.ID, tt.expectedFlavorID)
			}
		})
	}
}

// TestGetClosestFlavour_NoSuitableFlavorSentinel verifies that a genuine
// CPU/memory/GPU shape mismatch is matchable with errors.Is so callers can treat
// an unschedulable VM as a skip instead of a fatal error, while an empty flavor
// list (an infrastructure/permissions problem, not a per-VM shape problem)
// deliberately does NOT match the sentinel.
func TestGetClosestFlavour_NoSuitableFlavorSentinel(t *testing.T) {
	candidates := []flavors.Flavor{
		{ID: "small", VCPUs: 2, RAM: 2048},
		{ID: "medium", VCPUs: 4, RAM: 8192},
	}

	tests := []struct {
		name         string
		cpu          int
		memory       int
		allFlavors   []flavors.Flavor
		wantErr      bool
		wantSentinel bool
		wantFlavorID string
	}{
		{
			name:         "shape fits: no error",
			cpu:          2,
			memory:       2048,
			allFlavors:   candidates,
			wantFlavorID: "small",
		},
		{
			name:         "cpu too large: sentinel",
			cpu:          64,
			memory:       2048,
			allFlavors:   candidates,
			wantErr:      true,
			wantSentinel: true,
		},
		{
			name:         "memory too large: sentinel",
			cpu:          2,
			memory:       1048576,
			allFlavors:   candidates,
			wantErr:      true,
			wantSentinel: true,
		},
		{
			name:         "empty flavor list: error but NOT sentinel",
			cpu:          2,
			memory:       2048,
			allFlavors:   nil,
			wantErr:      true,
			wantSentinel: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flavor, err := GetClosestFlavour(tt.cpu, tt.memory, 0, 0, tt.allFlavors, false)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected an error, got nil")
				}
				if got := errors.Is(err, ErrNoSuitableFlavor); got != tt.wantSentinel {
					t.Errorf("errors.Is(err, ErrNoSuitableFlavor) = %v, want %v (err: %v)",
						got, tt.wantSentinel, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if flavor.ID != tt.wantFlavorID {
				t.Errorf("flavor.ID = %q, want %q", flavor.ID, tt.wantFlavorID)
			}
		})
	}
}

// TestErrNoSuitableFlavor_SurvivesWrapping is the crux of the non-blocking
// containment gate: the controller wraps this error with github.com/pkg/errors
// before the trigger loop inspects it, so errors.Is must still match through
// several layers of wrapping. If this breaks, one unschedulable VM silently
// starts aborting the whole MigrationPlan again.
func TestErrNoSuitableFlavor_SurvivesWrapping(t *testing.T) {
	_, err := GetClosestFlavour(64, 2048, 0, 0, []flavors.Flavor{{ID: "small", VCPUs: 2, RAM: 2048}}, false)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	wrapped := pkgerrors.Wrapf(
		pkgerrors.Wrap(err, "failed to determine target flavor for VM vm-07"),
		"failed to create ConfigMap for VM %s", "vm-07")

	if !errors.Is(wrapped, ErrNoSuitableFlavor) {
		t.Errorf("errors.Is failed through pkg/errors wrapping; got %v", wrapped)
	}
}
