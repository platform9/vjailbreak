// Copyright © 2024 The vjailbreak authors

package openstack

import (
	"context"

	"github.com/gophercloud/gophercloud/v2"
	"github.com/gophercloud/gophercloud/v2/openstack/networking/v2/networks"
	"github.com/pkg/errors"
)

// IsSimpleNetwork checks if a network has the "simple_network" tag indicating it's an L2-only network
func IsSimpleNetwork(ctx context.Context, networkingClient *gophercloud.ServiceClient, networkID string) (bool, error) {
	network, err := networks.Get(ctx, networkingClient, networkID).Extract()
	if err != nil {
		return false, errors.Wrap(err, "failed to get network details")
	}

	// Check if the network has the "simple_network" tag
	for _, tag := range network.Tags {
		if tag == "simple_network" {
			return true, nil
		}
	}
	return false, nil
}
