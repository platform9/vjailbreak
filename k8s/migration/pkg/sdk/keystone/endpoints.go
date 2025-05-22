// Copyright © 2021 The Platform9 Systems Inc.

package keystone

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"go.uber.org/zap"
)

// Type definition for struct encapsulating endpoint manager APIs.
type EndpointManagerAPI struct {
	Client  *http.Client
	BaseURL string
	Token   string
}

// Type definition for services information that is reported as
// part of the "get endpoints" request.
type EndpointsInfo struct {
	Endpoints []struct {
		ID        string `json:"id"`
		Interface string `json:"interface"`
		RegionID  string `json:"region_id"`
		ServiceID string `json:"serviceID"`
		URL       string `json:"url"`
		Enabled   bool   `json:"enabled"`
		Region    string `json:"region"`
		Links     struct {
			Self string `json:"self"`
		} `json:"links"`
	} `json:"endpoints"`
	Links struct {
		Next     interface{} `json:"next"`
		Self     string      `json:"self"`
		Previous interface{} `json:"previous"`
	} `json:"links"`
}

// Fetches the endpoint for a given region.
func GetEndpointForRegion(
	url string, // DU url
	auth AuthInfo, // Auth info
	region string, // region name
	serviceID string, // ID for regionInfo service
) (string, error) {
	zap.S().Debug("Fetching endpoint for region: ", region)

	// Form the URL
	url = fmt.Sprintf("%s/keystone/v3/endpoints", url)

	// Generate the http client object
	client := &http.Client{}

	// Create the context to invoke the service manager API.
	eAPI := EndpointManagerAPI{client, url, auth.Token}

	// Invoke the actual "get services" API.
	endpoint, err := eAPI.GetEndpointForRegionAPI(region, serviceID)
	if err != nil {
		return "", err
	}

	zap.S().Debug("Endpoint found: ", endpoint)
	return endpoint, nil
}

func (eAPI *EndpointManagerAPI) GetEndpointForRegionAPI(
	regionName string,
	serviceID string,
) (string, error) {
	zap.S().Debug("Fetching endpoints for region ", regionName)
	req, _ := http.NewRequest("GET", eAPI.BaseURL, nil)

	// Add keystone token in the header.
	req.Header.Add("X-Auth-Token", eAPI.Token)

	// Add the query parameter "serviceID"
	q := req.URL.Query()
	q.Add("serviceID", serviceID)
	req.URL.RawQuery = q.Encode()

	resp, err := eAPI.Client.Do(req)
	if err != nil {
		zap.S().Errorf("Failed to fetch endpoint information for region %s, Error: %s", regionName, err)
		return "", fmt.Errorf("failed to fetch endpoint information for region %s, Error: %s", regionName, err)
	}
	defer resp.Body.Close()

	endpointsInfo := EndpointsInfo{}
	// Response is received as slice of endpoints.
	err = json.NewDecoder(resp.Body).Decode(&endpointsInfo)
	if err != nil {
		zap.S().Errorf("Failed to decode endpoint information, Error: %s", err)
		return "", fmt.Errorf("failed to decode endpoint information, Error: %s", err)
	}

	var endpointURL string
	for _, endpoint := range endpointsInfo.Endpoints {
		// There will be multiple regions. Filter based on region name and
		// interface which is going to give exact endpoint for a region.
		if (endpoint.Region == regionName) && (endpoint.Interface == "internal") {
			zap.S().Debug("endpoint: ", endpoint.URL)
			u, err := url.Parse(endpoint.URL)
			if err != nil {
				zap.S().Errorf("Failed to parse endpoint information, Error: %s", err)
				return "", fmt.Errorf("failed to parse endpoint information, Error: %s", err)
			}
			endpointURL = u.Host
			zap.S().Debug("URL: ", endpointURL)
			break
		}
	}

	return endpointURL, nil
}
