// Copyright © 2024 The vjailbreak authors

package openstack

import (
	"fmt"
	"strings"

	"github.com/gophercloud/gophercloud/v2/openstack/compute/v2/flavors"
	"github.com/platform9/vjailbreak/k8s/migration/pkg/constants"
)

// GetClosestFlavour gets the closest flavor for the given CPU and memory requirements.
// If useGPUFlavor is true, it will only consider GPU-enabled flavors.
func GetClosestFlavour(cpu, memory int, allFlavors []flavors.Flavor, useGPUFlavor bool) (*flavors.Flavor, error) {
	// Check if the flavor slice is empty
	if len(allFlavors) == 0 {
		return nil, fmt.Errorf("no flavors available to select from")
	}

	bestFlavor := new(flavors.Flavor)
	bestFlavor.VCPUs = constants.MaxVCPUs
	bestFlavor.RAM = constants.MaxRAM

	// Find the smallest flavor that meets the requirements
	for _, flavor := range allFlavors {
		// If useGPUFlavor is true, filter for GPU-enabled flavors
		if useGPUFlavor && !isGPUFlavor(flavor) {
			continue
		}

		if flavor.VCPUs >= cpu && flavor.RAM >= memory {
			if flavor.VCPUs < bestFlavor.VCPUs ||
				(flavor.VCPUs == bestFlavor.VCPUs && flavor.RAM < bestFlavor.RAM) {
				bestFlavor = &flavor
			}
		}
	}

	if bestFlavor.VCPUs != constants.MaxVCPUs {
		return bestFlavor, nil
	}
	return nil, fmt.Errorf("no suitable flavor found for %d vCPUs and %d MB RAM", cpu, memory)
}

// isGPUFlavor checks if a flavor has GPU-related extra_specs
func isGPUFlavor(flavor flavors.Flavor) bool {
	if flavor.ExtraSpecs == nil {
		return false
	}

	for key := range flavor.ExtraSpecs {
		if key == "pci_passthrough:alias" ||
			strings.Contains(key, "trait:") ||
			key == "resources:VGPU" {
			return true
		}
	}
	return false
}
