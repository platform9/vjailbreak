package resmgr

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
)

type Resmgr interface {
	ListHosts(ctx context.Context) ([]Host, error)
	ListClusters(ctx context.Context) ([]Cluster, error)
	DeauthHost(ctx context.Context, hostID string) error
	GetHost(ctx context.Context, hostID string) (Host, error)
	GenerateSupportBundle(ctx context.Context, hostID string, label string, upload bool) error
	AssignRoles(ctx context.Context, hostID string, roles []string) error
	RemoveRoles(ctx context.Context, hostID string, roles []string) error
	GetRoles(ctx context.Context, hostID string) ([]string, error)
	AssignHypervisor(ctx context.Context, hostID string, clusterName string) error
	ListHostConfig(ctx context.Context) ([]vjailbreakv1alpha1.HostConfig, error)
	AssignHostConfig(ctx context.Context, hostID string, hostConfigID string) error
	HostExists(ctx context.Context, hostID string) (bool, error)
}

func NewResmgrClient(config Config) Resmgr {
	return &Impl{
		url:           config.DU.URL,
		authenticator: config.Authenticator,
		httpClient:    config.HTTPClient,
		insecure:      config.DU.Insecure,
	}
}

func (h *Host) UnmarshalJSON(data []byte) error {
	// Extension data can be a JSON object, empty string or missing entirely depending on the state of the host.
	// The missing case is handled by the "omitempty" json tag
	// The empty string case is handled in this function
	// If the extension data is present as expected then this function will unmarshal it to extract
	// just the capi_managed extension.
	// To make other extensions available update the PF9CAPIExtensions struct or directly use the RawExtensionData field.
	type H Host
	if err := json.Unmarshal(data, (*H)(h)); err != nil {
		return err
	}
	h.CAPIExtension = PF9CAPIExtensions{}
	if len(h.RawExtensionData) == 0 {
		return nil
	}
	if string(h.RawExtensionData) == "\"\"" {
		return nil
	}
	err := json.Unmarshal(h.RawExtensionData, &h.CAPIExtension)
	if err != nil {
		return err
	}
	return nil
}

func (r Impl) getResmgrReq(ctx context.Context, url, method string, body []byte) (*http.Request, error) {
	auth, err := r.authenticator.Auth(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch auth token: %w", err)
	}
	var req *http.Request
	req, err = http.NewRequestWithContext(ctx, method, url, bytes.NewBuffer(body))

	if err != nil {
		return nil, fmt.Errorf("unable to create request: %w", err)
	}
	req.Header.Set("X-Auth-Token", auth.Token)
	req.Header.Set("Content-Type", "application/json")
	return req, nil
}

// ListHosts fetches all hosts from resmgr along with their role config
func (r Impl) ListHosts(ctx context.Context) ([]Host, error) {
	// resmgr/v2?role_settings=true will fetch hosts and their config in a single API request.
	url := fmt.Sprintf("%s/resmgr/v2/hosts?role_settings=true", r.url)

	req, err := r.getResmgrReq(ctx, url, http.MethodGet, nil)

	if err != nil {
		return nil, fmt.Errorf("unable to create request to fetch hosts: %w", err)
	}

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || err != nil {
		return nil, fmt.Errorf("failed to query the resmgr to list hosts or failed to parse response: (%d) %s", resp.StatusCode, string(body))
	}

	hosts := []Host{}

	err = json.Unmarshal(body, &hosts)
	if err != nil {
		return nil, err
	}

	return hosts, nil
}

func (r Impl) GetHost(ctx context.Context, hostID string) (Host, error) {
	url := fmt.Sprintf("%s/resmgr/v2/hosts/%s", r.url, hostID)
	host := Host{}
	req, err := r.getResmgrReq(ctx, url, http.MethodGet, nil)

	if err != nil {
		return host, fmt.Errorf("unable to create request to get host %s: %w", hostID, err)
	}

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return host, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return host, fmt.Errorf("failed to query the resmgr to get host %s: (%d) %s", hostID, resp.StatusCode, string(body))
	}

	decoder := json.NewDecoder(resp.Body)
	err = decoder.Decode(&host)
	if err != nil {
		return host, err
	}

	return host, nil
}

// DeauthHost removes ALL roles from a host
func (r Impl) DeauthHost(ctx context.Context, hostID string) error {
	url := fmt.Sprintf("%s/resmgr/v2/hosts/%s", r.url, hostID)
	req, err := r.getResmgrReq(ctx, url, http.MethodDelete, nil)

	if err != nil {
		return fmt.Errorf("unable to create request to deauth host: %w", err)
	}

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to query the resmgr to deauth host: (%d) %s", resp.StatusCode, string(body))
	}
	return nil
}

func (r Impl) GenerateSupportBundle(ctx context.Context, hostID string, label string, upload bool) error {
	url := fmt.Sprintf("%s/resmgr/v1/hosts/%s/support/bundle", r.url, hostID)
	opts := &bundle{Label: label, Upload: strconv.FormatBool(upload)}
	data, _ := json.Marshal(opts)

	req, err := r.getResmgrReq(ctx, url, http.MethodPost, data)

	if err != nil {
		return fmt.Errorf("unable to create request to generate bundle: %w", err)
	}

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to query the resmgr to generate bundle: (%d) %s", resp.StatusCode,
			string(body))
	}

	return nil
}

// AddRoleVersion tries to adds role details into the resmgr.
// TODO: Consider details as a struct object instead of []byte
func (r Impl) AddRoleVersion(ctx context.Context, details []byte, errIfAlreadyExists bool) error {
	url := fmt.Sprintf("%s/resmgr/v2/roles", r.url)
	req, err := r.getResmgrReq(ctx, url, http.MethodPost, details)

	if err != nil {
		return fmt.Errorf("unable to create request to add role, err: %w", err)
	}

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode == http.StatusConflict {
			if errIfAlreadyExists {
				return fmt.Errorf("failed to add role to resmgr. Role already exists: (%d) %s", resp.StatusCode, string(body))
			}
		} else {
			return fmt.Errorf("failed to add role to resmgr: (%d) %s", resp.StatusCode, string(body))
		}
	}
	return nil
}

func (h *Host) ReadExtensions() (Extensions, error) {
	var extensions Extensions
	err := json.Unmarshal(h.RawExtensionData, &extensions)
	if err != nil {
		return extensions, err
	}
	return extensions, nil
}

func (r Impl) AssignRoles(ctx context.Context, hostID string, roles []string) error {

	if len(roles) == 0 {
		return fmt.Errorf("no roles provided")
	}

	for _, role := range roles {
		if role == "" {
			return fmt.Errorf("empty role provided")
		}
		url := fmt.Sprintf("%s/resmgr/v2/hosts/%s/roles/%s", r.url, hostID, role)
		req, err := r.getResmgrReq(ctx, url, http.MethodPost, nil)
		if err != nil {
			return fmt.Errorf("unable to create request to assign role %s: %w", role, err)
		}
		resp, err := r.httpClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("failed to query the resmgr to assign role %s: (%d) %s", role, resp.StatusCode, string(body))
		}
	}
	return nil
}

func (r Impl) AssignHypervisor(ctx context.Context, hostID string, clusterName string) error {
	url := fmt.Sprintf("%s/resmgr/v2/hosts/%s/roles/hypervisor", r.url, hostID)

	opts := &assignHypervisor{ClusterName: clusterName}
	data, _ := json.Marshal(opts)

	req, err := r.getResmgrReq(ctx, url, http.MethodPut, data)
	if err != nil {
		return fmt.Errorf("unable to create request to assign hypervisor role: %w", err)
	}
	resp, err := r.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to query the resmgr to assign hypervisor role: (%d) %s", resp.StatusCode, string(body))
	}
	return nil
}

func (r Impl) RemoveRoles(ctx context.Context, hostID string, roles []string) error {
	if len(roles) == 0 {
		return fmt.Errorf("no roles provided")
	}

	for _, role := range roles {
		if role == "" {
			return fmt.Errorf("empty role provided")
		}
		url := fmt.Sprintf("%s/resmgr/v2/hosts/%s/roles/%s", r.url, hostID, role)
		req, err := r.getResmgrReq(ctx, url, http.MethodDelete, nil)
		if err != nil {
			return fmt.Errorf("unable to create request to remove role %s: %w", role, err)
		}
		resp, err := r.httpClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("failed to query the resmgr to remove role %s: (%d) %s", role, resp.StatusCode, string(body))
		}
	}
	return nil
}

func (r Impl) GetRoles(ctx context.Context, hostID string) ([]string, error) {
	url := fmt.Sprintf("%s/resmgr/v2/hosts/%s", r.url, hostID)
	req, err := r.getResmgrReq(ctx, url, http.MethodGet, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to create request to get roles: %w", err)
	}

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to query the resmgr to get roles: (%d) %s", resp.StatusCode, string(body))
	}

	/*
			{
		    "id": "762b8cd2-f99c-4a71-8ab3-c178d2be977d",
		    "roles": [
		        "pf9-neutron-base",
		        "pf9-ostackhost-neutron",
		        "pf9-neutron-ovn-controller",
		        "pf9-neutron-ovn-metadata-agent",
		        "pf9-ceilometer",
		        "pf9-support",
		        "pf9-glance-role"
		    ],
			...
			}
	*/
	var roleResponse RoleResponse
	decoder := json.NewDecoder(resp.Body)
	err = decoder.Decode(&roleResponse)
	if err != nil {
		return nil, err
	}

	return roleResponse.Roles, nil
}

func (r Impl) AuthorizeHost(ctx context.Context, hostID string, token string, version string, role string) error {
	url := fmt.Sprintf("%s/resmgr/v2/hosts/%s/roles/%s", r.url, hostID, role)
	req, err := r.getResmgrReq(ctx, url, http.MethodPost, nil)
	if err != nil {
		return fmt.Errorf("unable to create request to authorize host: %w", err)
	}

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to query the resmgr to authorize host: (%d) %s", resp.StatusCode, string(body))
	}
	return nil
}

func (r Impl) ListClusters(ctx context.Context) ([]Cluster, error) {
	url := fmt.Sprintf("%s/resmgr/v2/clusters", r.url)
	req, err := r.getResmgrReq(ctx, url, http.MethodGet, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to create request to list clusters: %w", err)
	}

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to query the resmgr to list clusters: (%d) %s", resp.StatusCode, string(body))
	}

	var clusters []Cluster
	decoder := json.NewDecoder(resp.Body)
	err = decoder.Decode(&clusters)
	if err != nil {
		return nil, err
	}

	return clusters, nil
}

func (r Impl) ListHostConfig(ctx context.Context) ([]vjailbreakv1alpha1.HostConfig, error) {
	url := fmt.Sprintf("%s/resmgr/v2/hostconfigs", r.url)
	req, err := r.getResmgrReq(ctx, url, http.MethodGet, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to create request to list host config: %w", err)
	}

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to query the resmgr to list host config: (%d) %s", resp.StatusCode, string(body))
	}

	var hostConfig []vjailbreakv1alpha1.HostConfig
	decoder := json.NewDecoder(resp.Body)
	err = decoder.Decode(&hostConfig)
	if err != nil {
		return nil, err
	}

	return hostConfig, nil
}

func (r Impl) AssignHostConfig(ctx context.Context, hostID string, hostConfigID string) error {
	url := fmt.Sprintf("%s/resmgr/v2/hosts/%s/hostconfig/%s", r.url, hostID, hostConfigID)
	data, _ := json.Marshal(hostConfigID)

	req, err := r.getResmgrReq(ctx, url, http.MethodPut, data)
	if err != nil {
		return fmt.Errorf("unable to create request to assign host config: %w", err)
	}
	resp, err := r.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to query the resmgr to assign host config: (%d) %s", resp.StatusCode, string(body))
	}
	return nil
}

func (r Impl) HostExists(ctx context.Context, hostID string) (bool, error) {
	_, err := r.GetHost(ctx, hostID)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
