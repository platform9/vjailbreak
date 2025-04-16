package resmgr

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/platform9/vjailbreak/k8s/migration/pkg/sdk/du"
	"github.com/platform9/vjailbreak/k8s/migration/pkg/sdk/keystone"
)

type Resmgr interface {
	ListHosts(ctx context.Context) ([]Host, error)
	UpdateHostKubeRoleConfig(ctx context.Context, hostID string, config KubeRoleConfig) error
	ApplyKubeRole(ctx context.Context, hostID string, version string) error
	DeauthHost(ctx context.Context, hostID string) error
	GetKubeRole(ctx context.Context, version string) (KubeRole, error)
	GetHost(ctx context.Context, hostID string) (Host, error)
	AddRoleVersion(ctx context.Context, details []byte, errIfAlreadyExists bool) error
	GenerateSupportBundle(ctx context.Context, hostID string, label string, upload bool) error
	AssignRoles(ctx context.Context, hostID string, roles []string) error
	RemoveRoles(ctx context.Context, hostID string, roles []string) error
	GetRoles(ctx context.Context, hostID string) ([]string, error)
}

type Impl struct {
	url           string
	authenticator keystone.Authenticator
	httpClient    http.Client
}

type Config struct {
	DU            du.Info
	Authenticator keystone.Authenticator
	HTTPClient    http.Client
}

func NewResmgrClient(config Config) Resmgr {
	return &Impl{
		url:           config.DU.URL,
		authenticator: config.Authenticator,
		httpClient:    config.HTTPClient,
	}
}

type KubeRole struct {
	Name            string         `json:"name"`
	DisplayName     string         `json:"display_name"`
	Description     string         `json:"description"`
	RoleVersion     string         `json:"role_version"`
	DefaultSettings KubeRoleConfig `json:"default_settings"`
}

type RoleResponse struct {
	Roles []string `json:"roles"`
}

type KubeRoleConfig struct {
	KubeServiceState            string `json:"KUBE_SERVICE_STATE,omitempty"`
	TransportURL                string `json:"TRANSPORT_URL,omitempty"`
	UseHostname                 string `json:"USE_HOSTNAME,omitempty"`
	Runtime                     string `json:"RUNTIME,omitempty"`
	EnableProfileAgent          string `json:"ENABLE_PROFILE_AGENT,omitempty"`
	Role                        string `json:"ROLE,omitempty"`
	ContainersCidr              string `json:"CONTAINERS_CIDR,omitempty"`
	ServicesCidr                string `json:"SERVICES_CIDR,omitempty"`
	MasterIP                    string `json:"MASTER_IP,omitempty"`
	ExternalDNSName             string `json:"EXTERNAL_DNS_NAME,omitempty"`
	Debug                       string `json:"DEBUG,omitempty"`
	DockerRoot                  string `json:"DOCKER_ROOT,omitempty"`
	ClusterID                   string `json:"CLUSTER_ID,omitempty"`
	ClusterName                 string `json:"CLUSTER_NAME,omitempty"`
	CatapultEnabled             string `json:"CATAPULT_ENABLED,omitempty"`
	FlannelIfaceLabel           string `json:"FLANNEL_IFACE_LABEL,omitempty"`
	FlannelPublicIfaceLabel     string `json:"FLANNEL_PUBLIC_IFACE_LABEL,omitempty"`
	EtcdDataDir                 string `json:"ETCD_DATA_DIR,omitempty"`
	EtcdDiscoveryURL            string `json:"ETCD_DISCOVERY_URL,omitempty"`
	KeystoneEnabled             string `json:"KEYSTONE_ENABLED,omitempty"`
	AuthzEnabled                string `json:"AUTHZ_ENABLED,omitempty"`
	MasterlessEnabled           string `json:"MASTERLESS_ENABLED,omitempty"`
	KeystoneDomain              string `json:"KEYSTONE_DOMAIN,omitempty"`
	AppCatalogEnabled           string `json:"APP_CATALOG_ENABLED,omitempty"`
	Privileged                  string `json:"PRIVILEGED,omitempty"`
	CloudProviderType           string `json:"CLOUD_PROVIDER_TYPE,omitempty"`
	ExtraOpts                   string `json:"EXTRA_OPTS,omitempty"`
	ClusterProjectID            string `json:"CLUSTER_PROJECT_ID,omitempty"`
	RuntimeConfig               string `json:"RUNTIME_CONFIG,omitempty"`
	Pf9NetworkPlugin            string `json:"PF9_NETWORK_PLUGIN,omitempty"`
	CniBridge                   string `json:"CNI_BRIDGE,omitempty"`
	OsRegion                    string `json:"OS_REGION,omitempty"`
	OsUsername                  string `json:"OS_USERNAME,omitempty"`
	OsPassword                  string `json:"OS_PASSWORD,omitempty"`
	OsUserDomainName            string `json:"OS_USER_DOMAIN_NAME,omitempty"`
	OsAuthURL                   string `json:"OS_AUTH_URL,omitempty"`
	OsProjectName               string `json:"OS_PROJECT_NAME,omitempty"`
	OsProjectDomainName         string `json:"OS_PROJECT_DOMAIN_NAME,omitempty"`
	AllowWorkloadsOnMaster      string `json:"ALLOW_WORKLOADS_ON_MASTER,omitempty"`
	MetallbCidr                 string `json:"METALLB_CIDR,omitempty"`
	MetallbEnabled              string `json:"METALLB_ENABLED,omitempty"`
	EtcdEnv                     string `json:"ETCD_ENV,omitempty"`
	MasterVipEnabled            string `json:"MASTER_VIP_ENABLED,omitempty"`
	MasterVipVrouterID          string `json:"MASTER_VIP_VROUTER_ID,omitempty"`
	MasterVipPriority           string `json:"MASTER_VIP_PRIORITY,omitempty"`
	MasterVipIface              string `json:"MASTER_VIP_IFACE,omitempty"`
	BouncerSlowRequestWebhook   string `json:"BOUNCER_SLOW_REQUEST_WEBHOOK,omitempty"`
	K8SAPIPort                  string `json:"K8S_API_PORT,omitempty"`
	MtuSize                     string `json:"MTU_SIZE,omitempty"`
	EtcdVersion                 string `json:"ETCD_VERSION,omitempty"`
	ApiserverStorageBackend     string `json:"APISERVER_STORAGE_BACKEND,omitempty"`
	KubeletCloudConfig          string `json:"KUBELET_CLOUD_CONFIG,omitempty"`
	EnableCas                   string `json:"ENABLE_CAS,omitempty"`
	VaultToken                  string `json:"VAULT_TOKEN,omitempty"`
	EtcdHeartbeatInterval       string `json:"ETCD_HEARTBEAT_INTERVAL,omitempty"`
	EtcdElectionTimeout         string `json:"ETCD_ELECTION_TIMEOUT,omitempty"`
	CalicoIpv4BlockSize         string `json:"CALICO_IPV4_BLOCK_SIZE,omitempty"`
	CalicoIpipMode              string `json:"CALICO_IPIP_MODE,omitempty"`
	CalicoNodeMemoryLimit       string `json:"CALICO_NODE_MEMORY_LIMIT,omitempty"`
	CalicoNodeCPULimit          string `json:"CALICO_NODE_CPU_LIMIT,omitempty"`
	CalicoTyphaMemoryLimit      string `json:"CALICO_TYPHA_MEMORY_LIMIT,omitempty"`
	CalicoTyphaCPULimit         string `json:"CALICO_TYPHA_CPU_LIMIT,omitempty"`
	CalicoControllerMemoryLimit string `json:"CALICO_CONTROLLER_MEMORY_LIMIT,omitempty"`
	CalicoControllerCPULimit    string `json:"CALICO_CONTROLLER_CPU_LIMIT,omitempty"`
	CalicoNatOutgoing           string `json:"CALICO_NAT_OUTGOING,omitempty"`
	CalicoIpv4                  string `json:"CALICO_IPV4,omitempty"`
	CalicoIpv6                  string `json:"CALICO_IPV6,omitempty"`
	CalicoIpv4DetectionMethod   string `json:"CALICO_IPV4_DETECTION_METHOD,omitempty"`
	CalicoIpv6DetectionMethod   string `json:"CALICO_IPV6_DETECTION_METHOD,omitempty"`
	CalicoRouterID              string `json:"CALICO_ROUTER_ID,omitempty"`
	CalicoIpv6PoolCidr          string `json:"CALICO_IPV6POOL_CIDR,omitempty"`
	CalicoIpv6PoolBlockSize     string `json:"CALICO_IPV6POOL_BLOCK_SIZE,omitempty"`
	CalicoIpv6PoolNatOutgoing   string `json:"CALICO_IPV6POOL_NAT_OUTGOING,omitempty"`
	FelixIpv6Support            string `json:"FELIX_IPV6SUPPORT,omitempty"`
	KubeProxyMode               string `json:"KUBE_PROXY_MODE,omitempty"`
	MinNumWorkers               string `json:"MIN_NUM_WORKERS,omitempty"`
	MaxNumWorkers               string `json:"MAX_NUM_WORKERS,omitempty"`
	DockerLiveRestoreEnabled    string `json:"DOCKER_LIVE_RESTORE_ENABLED,omitempty"`
	APIServerFlags              string `json:"API_SERVER_FLAGS,omitempty"`
	ControllerManagerFlags      string `json:"CONTROLLER_MANAGER_FLAGS,omitempty"`
	SchedulerFlags              string `json:"SCHEDULER_FLAGS,omitempty"`
	Ipv6Enabled                 string `json:"IPV6_ENABLED,omitempty"`
	DockerhubID                 string `json:"DOCKERHUB_ID,omitempty"`
	DockerhubPassword           string `json:"DOCKERHUB_PASSWORD,omitempty"`
	RegistryMirrors             string `json:"REGISTRY_MIRRORS,omitempty"`
	CPUManagerPolicy            string `json:"CPU_MANAGER_POLICY,omitempty"`
	TopologyManagerPolicy       string `json:"TOPOLOGY_MANAGER_POLICY,omitempty"`
	ReservedCpus                string `json:"RESERVED_CPUS,omitempty"`
	DockerPrivateRegistry       string `json:"DOCKER_PRIVATE_REGISTRY,omitempty"`
	QuayPrivateRegistry         string `json:"QUAY_PRIVATE_REGISTRY,omitempty"`
	GcrPrivateRegistry          string `json:"GCR_PRIVATE_REGISTRY,omitempty"`
	K8SPrivateRegistry          string `json:"K8S_PRIVATE_REGISTRY,omitempty"`
	DockerCentosRepoURL         string `json:"DOCKER_CENTOS_REPO_URL,omitempty"`
	DockerUbuntuRepoURL         string `json:"DOCKER_UBUNTU_REPO_URL,omitempty"`
}

type PF9CAPIExtensions struct {
	CapiManaged struct {
		Status string `json:"status"`
		Data   struct {
			Managed bool `json:"managed"`
		} `json:"data,omitempty"`
	} `json:"pf9_capi,omitempty"`
}

type Host struct {
	ID   string `json:"id"`
	Info struct {
		Hostname         string      `json:"hostname"`
		OSFamily         string      `json:"os_family"`
		Arch             string      `json:"arch"`
		OSInfo           string      `json:"os_info"`
		Responding       bool        `json:"responding"`
		LastResponseTime interface{} `json:"last_response_time"`
	} `json:"info,omitempty"`
	Roles        []string `json:"roles,omitempty"`
	RoleStatus   string   `json:"role_status,omitempty"`
	RoleSettings struct {
		Pf9Kube KubeRoleConfig `json:"pf9-kube"`
	} `json:"role_settings,omitempty"`
	RawExtensionData json.RawMessage   `json:"extensions,omitempty"`
	CAPIExtension    PF9CAPIExtensions `json:"-"`
	Message          string            `json:"message,omitempty"`
}

// Type definition for payload to be sent to bundle generation request.
type bundle struct {
	Upload string `json:"upload"`
	Label  string `json:"label"`
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

func (r Impl) GetKubeRole(ctx context.Context, version string) (KubeRole, error) {
	url := fmt.Sprintf("%s/resmgr/v2/roles/pf9-kube/", r.url)
	if version != "" {
		url = fmt.Sprintf("%s?version=%s", url, version)
	}
	role := KubeRole{}
	req, err := r.getResmgrReq(ctx, url, http.MethodGet, nil)

	if err != nil {
		return role, fmt.Errorf("unable to create request to get pf9-kube role version %s: %w", version, err)
	}

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return role, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return role, fmt.Errorf("failed to query the resmgr to get pf9-kube role version %s: (%d) %s", version, resp.StatusCode, string(body))
	}

	decoder := json.NewDecoder(resp.Body)
	err = decoder.Decode(&role)
	if err != nil {
		return role, err
	}

	return role, nil
}

// ListHosts fetches all hosts from resmgr along with their pf9-kube role config
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

// UpdateHostKubeRoleConfig updates the pf9-kube role config for a host without upgrading the role version
func (r Impl) UpdateHostKubeRoleConfig(ctx context.Context, hostID string, config KubeRoleConfig) error {
	url := fmt.Sprintf("%s/resmgr/v1/hosts/%s/roles/pf9-kube", r.url, hostID)
	body, _ := json.Marshal(config)
	req, err := r.getResmgrReq(ctx, url, http.MethodPut, body)

	if err != nil {
		return fmt.Errorf("unable to create request to update host role config: %w", err)
	}

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to query the resmgr to update host role config: (%d) %s", resp.StatusCode, string(body))
	}
	return nil
}

// ApplyKubeRole applies just the pf9-kube role to a host
// This API can be used for upgrading pf9-kube role version
func (r Impl) ApplyKubeRole(ctx context.Context, hostID string, version string) error {
	url := fmt.Sprintf("%s/resmgr/v1/hosts/%s/roles/pf9-kube", r.url, hostID)
	if version != "" {
		url = fmt.Sprintf("%s/versions/%s", url, version)
	}
	req, err := r.getResmgrReq(ctx, url, http.MethodPut, nil)

	if err != nil {
		return fmt.Errorf("unable to create request to apply kube role: %w", err)
	}

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to query the resmgr to apply kube role: (%d) %s", resp.StatusCode, string(body))
	}
	return nil
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
