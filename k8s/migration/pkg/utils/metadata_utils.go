// Package utils provides utility functions for handling migration-related operations.
package utils

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/servers"
	"github.com/pkg/errors"
)

// InstanceMetadata contains metadata about the current instance
type InstanceMetadata struct {
	UUID string `json:"uuid"`
}

// GetCurrentInstanceMetadata retrieves metadata about the current instance from the OpenStack metadata service
func GetCurrentInstanceMetadata() (*InstanceMetadata, error) {
	// Try to get instance metadata from OpenStack metadata service
	client := &http.Client{}
	req, err := http.NewRequest("GET", "http://169.254.169.254/openstack/latest/meta_data.json", nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create metadata request")
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch instance metadata")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code from metadata service: %d", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read metadata response")
	}

	var metadata InstanceMetadata
	if err := json.Unmarshal(body, &metadata); err != nil {
		return nil, errors.Wrap(err, "failed to parse metadata")
	}

	if metadata.UUID == "" {
		return nil, errors.New("instance UUID not found in metadata")
	}

	return &metadata, nil
}

// VerifyCredentialsMatchCurrentEnvironment verifies if the provided credentials can access the current instance
func VerifyCredentialsMatchCurrentEnvironment(providerClient *gophercloud.ProviderClient) (bool, error) {
	// Get current instance metadata
	metadata, err := GetCurrentInstanceMetadata()
	if err != nil {
		return false, errors.Wrap(err, "failed to get current instance metadata")
	}

	// Create a compute client
	computeClient, err := openstack.NewComputeV2(providerClient, gophercloud.EndpointOpts{})
	if err != nil {
		return false, errors.Wrap(err, "failed to create compute client")
	}

	// Try to get the current instance using the provided credentials
	_, err = servers.Get(computeClient, metadata.UUID).Extract()
	if err != nil {
		// If we get a 404, the credentials don't have access to this instance
		if strings.Contains(err.Error(), "Resource not found") ||
			strings.Contains(err.Error(), "No server with a name or ID") {
			return false, nil
		}
		return false, errors.Wrap(err, "failed to verify instance access")
	}

	return true, nil
}
