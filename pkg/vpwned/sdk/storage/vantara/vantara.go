// Copyright © 2026 The vjailbreak authors

// Package vantara implements the core StorageProvider for Hitachi Vantara
// VSP-family arrays via the Configuration Manager REST API (GUM/SVP).
//
// The provider deliberately implements ONLY the core surface (auth/session,
// CreateVolume, DeleteVolume, GetVolumeInfo, ListAllVolumes, NAA,
// ResolveCinderVolumeToLUN) — it does NOT implement storage.VendorMapper.
// LUN-to-ESXi mapping is delegated to the Hitachi Cinder driver (HBSD)
// through the CinderMapper fallback (see sdk/storage/cinder), which is
// selected automatically under MappingMode=auto.
//
// REST conventions
//   - Base URL:   https://<host>:<port>/ConfigurationManager/v1/objects
//   - Version:    GET  /ConfigurationManager/configuration/version (>= 1.9)
//   - Session:    POST /objects/sessions with Basic auth -> token, then
//     "Authorization: Session <token>"; sessions expire ~30min, so the
//     provider transparently re-authenticates after 25min.
//   - Async jobs: mutating calls return a jobId; poll /objects/jobs/<id>
//     until status=Completed, then read affectedResources.
//   - NAA:        GET /objects/ldevs/<id> returns naaId directly; the ESXi
//     device is naa.<naaId>. No vendor formula needed.
//
// Cinder (HBSD) interop:
//   - manage_existing accepts {"source-id": <LDEV id>} — the provider
//     advertises this via storage.CinderManageRefBuilder because the default
//     {"source-name": ...} lookup requires dash-free LDEV labels.
//   - On manage, HBSD relabels the LDEV to the Cinder volume UUID with
//     dashes stripped; ResolveCinderVolumeToLUN searches by that label.
package vantara

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"crypto/tls"

	"github.com/platform9/vjailbreak/pkg/vpwned/sdk/storage"
	"k8s.io/klog/v2"
)

const (
	// VendorName is the registry key for this provider.
	VendorName = "vantara"

	// OptionPoolID selects the DP pool (decimal ID) where target LDEVs are
	// created. Optional when the array exposes exactly one DP pool.
	OptionPoolID = "pool_id"
	// OptionRESTPort overrides the Configuration Manager REST port.
	OptionRESTPort = "rest_port"

	defaultRESTPort = "443"

	requiredMajorVersion = 1
	requiredMinorVersion = 9

	// Hitachi LDEV labels are limited to 32 characters.
	maxLdevLabelLen = 32

	// ldev listing page size (REST maximum).
	listPageSize = 16384

	sessionMaxAge = 25 * time.Minute
)

// ldevInfo is the subset of the LDEV object the provider consumes.
type ldevInfo struct {
	LdevID        int    `json:"ldevId"`
	Label         string `json:"label"`
	NaaID         string `json:"naaId"`
	BlockCapacity int64  `json:"blockCapacity"`
	PoolID        int    `json:"poolId"`
	EmulationType string `json:"emulationType"`
	Status        string `json:"status"`
}

type poolInfo struct {
	PoolID   int    `json:"poolId"`
	PoolName string `json:"poolName"`
	PoolType string `json:"poolType"`
}

// VantaraStorageProvider talks to the Configuration Manager REST API.
type VantaraStorageProvider struct {
	client     *http.Client
	baseURL    string // https://<host>:<port>/ConfigurationManager/v1/objects
	versionURL string // https://<host>:<port>/ConfigurationManager/configuration/version

	username string
	password string

	sessionToken string
	sessionID    string
	sessionStart time.Time
	connected    bool

	// poolID is the resolved DP pool for CreateVolume; nil until configured
	// or auto-picked from a single-DP-pool array.
	poolID *int
}

func init() {
	storage.RegisterStorageProvider(VendorName, &VantaraStorageProvider{})
}

// Compile-time contract checks: core provider + manage-ref builder, and
// deliberately NOT a VendorMapper (mapping is Cinder's job for this vendor).
var (
	_ storage.StorageProvider        = (*VantaraStorageProvider)(nil)
	_ storage.CinderManageRefBuilder = (*VantaraStorageProvider)(nil)
	_ storage.CinderBackendPoolAware = (*VantaraStorageProvider)(nil)
)

// Connect validates the REST API version and establishes a session.
func (p *VantaraStorageProvider) Connect(ctx context.Context, accessInfo storage.StorageAccessInfo) error {
	host := strings.TrimSpace(accessInfo.Hostname)
	if host == "" {
		return fmt.Errorf("vantara: hostname is required")
	}

	port := defaultRESTPort
	if o := strings.TrimSpace(accessInfo.ProviderOptions[OptionRESTPort]); o != "" {
		port = o
	}
	// Allow "host:port" in Hostname to win over the default.
	if h, hp, err := net.SplitHostPort(host); err == nil {
		host, port = h, hp
	}

	p.baseURL = fmt.Sprintf("https://%s/ConfigurationManager/v1/objects", net.JoinHostPort(host, port))
	p.versionURL = fmt.Sprintf("https://%s/ConfigurationManager/configuration/version", net.JoinHostPort(host, port))
	p.username = accessInfo.Username
	p.password = accessInfo.Password
	p.client = &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: accessInfo.SkipSSLVerification}, //nolint:gosec // operator-controlled, mirrors other providers
		},
		Timeout: 60 * time.Second,
	}

	if o := strings.TrimSpace(accessInfo.ProviderOptions[OptionPoolID]); o != "" {
		id, err := strconv.Atoi(o)
		if err != nil {
			return fmt.Errorf("vantara: invalid %s %q: %w", OptionPoolID, o, err)
		}
		p.poolID = &id
	}

	// API version gate (Basic auth endpoint).
	verResp, _, err := p.doJSON(ctx, http.MethodGet, p.versionURL, nil, p.basicHeaders())
	if err != nil {
		return fmt.Errorf("vantara: failed to get API version: %w", err)
	}
	apiVersion, _ := verResp["apiVersion"].(string)
	if err := checkAPIVersion(apiVersion, requiredMajorVersion, requiredMinorVersion); err != nil {
		return err
	}
	klog.Infof("vantara: API version %s at %s", apiVersion, p.versionURL)

	if err := p.openSession(ctx); err != nil {
		return err
	}
	return nil
}

// Disconnect discards the REST session.
func (p *VantaraStorageProvider) Disconnect() error {
	if !p.connected {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if _, _, err := p.doJSON(ctx, http.MethodDelete, p.baseURL+"/sessions/"+p.sessionID, nil, p.sessionHeaders()); err != nil {
		klog.Warningf("vantara: failed to discard session %s: %v", p.sessionID, err)
	}
	p.connected = false
	p.sessionToken = ""
	p.sessionID = ""
	return nil
}

// ValidateCredentials confirms the session works and the pool selection is
// sane. With no pool configured it attempts a single-DP-pool auto-pick.
func (p *VantaraStorageProvider) ValidateCredentials(ctx context.Context) error {
	if err := p.ensureSession(ctx); err != nil {
		return err
	}
	if p.poolID != nil {
		if _, _, err := p.doJSON(ctx, http.MethodGet, fmt.Sprintf("%s/pools/%d", p.baseURL, *p.poolID), nil, p.sessionHeaders()); err != nil {
			return fmt.Errorf("vantara: configured pool %d not found: %w", *p.poolID, err)
		}
		return nil
	}
	if err := p.autoPickPool(ctx); err != nil {
		// Multiple pools is not a credential failure; the pool is usually
		// derived from the Cinder backend mapping (ApplyCinderPoolHint) and
		// CreateVolume demands explicit configuration otherwise.
		klog.Warningf("vantara: %v", err)
	}
	return nil
}

// ApplyCinderPoolHint implements storage.CinderBackendPoolAware. The hint is
// the Cinder backend's pool for this array (openstackMapping.cinderBackendPool
// or the "#pool" suffix of an explicit cinderHost). HBSD's hitachi_pools
// accepts DP pool IDs or names, so the hint is resolved both ways. An
// explicitly configured pool (spec.vantaraConfig.poolId) always wins.
func (p *VantaraStorageProvider) ApplyCinderPoolHint(ctx context.Context, poolHint string) error {
	poolHint = strings.TrimSpace(poolHint)
	if poolHint == "" {
		return nil
	}
	if p.poolID != nil {
		klog.Infof("vantara: explicit pool %d configured; ignoring Cinder pool hint %q", *p.poolID, poolHint)
		return nil
	}
	if err := p.ensureSession(ctx); err != nil {
		return err
	}

	// Numeric hint: treat as a DP pool ID and verify it exists.
	if id, err := strconv.Atoi(poolHint); err == nil {
		if _, _, err := p.doJSON(ctx, http.MethodGet, fmt.Sprintf("%s/pools/%d", p.baseURL, id), nil, p.sessionHeaders()); err != nil {
			return fmt.Errorf("vantara: Cinder pool hint %q does not match a pool on the array: %w", poolHint, err)
		}
		p.poolID = &id
		klog.Infof("vantara: pool %d derived from Cinder backend pool hint", id)
		return nil
	}

	// Otherwise resolve the hint as a DP pool name.
	pools, err := p.listDPPools(ctx)
	if err != nil {
		return fmt.Errorf("vantara: failed to resolve Cinder pool hint %q: %w", poolHint, err)
	}
	for i := range pools {
		if strings.EqualFold(pools[i].PoolName, poolHint) {
			p.poolID = &pools[i].PoolID
			klog.Infof("vantara: pool %d (%s) derived from Cinder backend pool hint", pools[i].PoolID, pools[i].PoolName)
			return nil
		}
	}
	names := make([]string, 0, len(pools))
	for _, pi := range pools {
		names = append(names, fmt.Sprintf("%d(%s)", pi.PoolID, pi.PoolName))
	}
	return fmt.Errorf("vantara: Cinder pool hint %q matches no DP pool on the array (available: %s)", poolHint, strings.Join(names, ", "))
}

// CreateVolume creates a DP LDEV of at least size bytes in the configured
// pool, labels it with volumeName (truncated to 32 chars), and returns the
// volume with its NAA taken from the array's naaId attribute.
func (p *VantaraStorageProvider) CreateVolume(volumeName string, size int64) (storage.Volume, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	if err := p.ensureSession(ctx); err != nil {
		return storage.Volume{}, err
	}
	if p.poolID == nil {
		if err := p.autoPickPool(ctx); err != nil {
			return storage.Volume{}, fmt.Errorf("vantara: no pool configured for LDEV creation: %w (set spec.vantaraConfig.poolId, or let it derive from openstackMapping.cinderBackendPool / the \"#pool\" suffix of cinderHost)", err)
		}
	}

	blocks := size / 512
	if size%512 != 0 {
		blocks++
	}

	body := map[string]any{
		"poolId":        *p.poolID,
		"blockCapacity": blocks,
	}
	job, err := p.invokeJob(ctx, http.MethodPost, p.baseURL+"/ldevs", body)
	if err != nil {
		return storage.Volume{}, fmt.Errorf("vantara: LDEV creation failed: %w", err)
	}
	ldevID, err := ldevIDFromJob(job)
	if err != nil {
		return storage.Volume{}, err
	}

	label := sanitizeLabel(volumeName)
	if _, err := p.invokeJob(ctx, http.MethodPut, fmt.Sprintf("%s/ldevs/%d", p.baseURL, ldevID), map[string]any{"label": label}); err != nil {
		p.bestEffortDeleteLdev(ctx, ldevID)
		return storage.Volume{}, fmt.Errorf("vantara: failed to label LDEV %d: %w", ldevID, err)
	}

	info, err := p.getLdevWithNAA(ctx, ldevID)
	if err != nil {
		p.bestEffortDeleteLdev(ctx, ldevID)
		return storage.Volume{}, err
	}

	klog.Infof("vantara: created LDEV %d label=%s naa=%s blocks=%d pool=%d", ldevID, label, info.NaaID, info.BlockCapacity, *p.poolID)
	return volumeFromLdev(*info), nil
}

// DeleteVolume removes the LDEV whose label matches volumeName. Missing
// volumes are treated as already deleted.
func (p *VantaraStorageProvider) DeleteVolume(volumeName string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	if err := p.ensureSession(ctx); err != nil {
		return err
	}
	info, err := p.findLdevByLabel(ctx, sanitizeLabel(volumeName))
	if err != nil {
		return err
	}
	if info == nil {
		klog.Infof("vantara: volume %q not found; treating delete as complete", volumeName)
		return nil
	}
	if _, err := p.invokeJob(ctx, http.MethodDelete, fmt.Sprintf("%s/ldevs/%d", p.baseURL, info.LdevID), nil); err != nil {
		return fmt.Errorf("vantara: failed to delete LDEV %d: %w", info.LdevID, err)
	}
	return nil
}

// GetVolumeInfo returns information about the LDEV labelled volumeName.
func (p *VantaraStorageProvider) GetVolumeInfo(volumeName string) (storage.VolumeInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	if err := p.ensureSession(ctx); err != nil {
		return storage.VolumeInfo{}, err
	}
	info, err := p.findLdevByLabel(ctx, sanitizeLabel(volumeName))
	if err != nil {
		return storage.VolumeInfo{}, err
	}
	if info == nil {
		return storage.VolumeInfo{}, fmt.Errorf("vantara: volume %q not found", volumeName)
	}
	return volumeInfoFromLdev(*info), nil
}

// ListAllVolumes lists all defined LDEVs on the array.
func (p *VantaraStorageProvider) ListAllVolumes() ([]storage.VolumeInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	if err := p.ensureSession(ctx); err != nil {
		return nil, err
	}
	ldevs, err := p.listLdevs(ctx)
	if err != nil {
		return nil, err
	}
	infos := make([]storage.VolumeInfo, 0, len(ldevs))
	for _, l := range ldevs {
		infos = append(infos, volumeInfoFromLdev(l))
	}
	return infos, nil
}

// GetAllVolumeNAAs returns NAA identifiers for all LDEVs that report one.
func (p *VantaraStorageProvider) GetAllVolumeNAAs() ([]string, error) {
	infos, err := p.ListAllVolumes()
	if err != nil {
		return nil, err
	}
	naas := make([]string, 0, len(infos))
	for _, i := range infos {
		if i.NAA != "" {
			naas = append(naas, i.NAA)
		}
	}
	return naas, nil
}

// ResolveCinderVolumeToLUN locates the LDEV backing a Cinder volume. The
// Hitachi Cinder driver (HBSD) relabels managed LDEVs to the Cinder volume
// UUID with dashes stripped, so that label is the lookup key.
func (p *VantaraStorageProvider) ResolveCinderVolumeToLUN(volumeID string) (storage.Volume, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	if err := p.ensureSession(ctx); err != nil {
		return storage.Volume{}, err
	}
	label := strings.ReplaceAll(volumeID, "-", "")
	info, err := p.findLdevByLabel(ctx, label)
	if err != nil {
		return storage.Volume{}, err
	}
	if info == nil {
		return storage.Volume{}, fmt.Errorf("vantara: no LDEV labelled %q for Cinder volume %s (expected HBSD manage to relabel it)", label, volumeID)
	}
	return volumeFromLdev(*info), nil
}

// WhoAmI returns the provider name.
func (p *VantaraStorageProvider) WhoAmI() string {
	return VendorName
}

// BuildCinderManageRef implements storage.CinderManageRefBuilder. HBSD's
// manage_existing resolves {"source-id": <LDEV id>} unambiguously, whereas
// source-name requires dash-free labels; prefer the id we know from create.
func (p *VantaraStorageProvider) BuildCinderManageRef(vol storage.Volume) map[string]interface{} {
	if vol.Id != "" {
		return map[string]interface{}{"source-id": vol.Id}
	}
	return map[string]interface{}{"source-name": vol.Name}
}

// ---------- internals ----------

func (p *VantaraStorageProvider) openSession(ctx context.Context) error {
	resp, _, err := p.doJSON(ctx, http.MethodPost, p.baseURL+"/sessions", map[string]string{}, p.basicHeaders())
	if err != nil {
		return fmt.Errorf("vantara: failed to create session: %w", err)
	}
	token, _ := resp["token"].(string)
	if token == "" {
		return fmt.Errorf("vantara: session response missing token: %v", resp)
	}
	p.sessionToken = token
	if id, ok := resp["sessionId"].(float64); ok {
		p.sessionID = strconv.Itoa(int(id))
	}
	p.sessionStart = time.Now()
	p.connected = true
	klog.Infof("vantara: session %s established", p.sessionID)
	return nil
}

func (p *VantaraStorageProvider) ensureSession(ctx context.Context) error {
	if p.client == nil {
		return fmt.Errorf("vantara: provider not connected; call Connect first")
	}
	if p.connected && time.Since(p.sessionStart) < sessionMaxAge {
		return nil
	}
	if p.connected {
		klog.Info("vantara: session approaching expiry, re-authenticating")
		_ = p.Disconnect()
	}
	return p.openSession(ctx)
}

func (p *VantaraStorageProvider) basicHeaders() map[string]string {
	auth := base64.StdEncoding.EncodeToString([]byte(p.username + ":" + p.password))
	return map[string]string{
		"Content-Type":  "application/json",
		"Accept":        "application/json",
		"Authorization": "Basic " + auth,
	}
}

func (p *VantaraStorageProvider) sessionHeaders() map[string]string {
	return map[string]string{
		"Content-Type":  "application/json",
		"Accept":        "application/json",
		"Authorization": "Session " + p.sessionToken,
	}
}

// doJSON performs a request and decodes the JSON response (tolerating empty
// bodies). 503s are retried a bounded number of times.
func (p *VantaraStorageProvider) doJSON(ctx context.Context, method, url string, body any, headers map[string]string) (map[string]any, int, error) {
	var lastStatus int
	for attempt := 0; attempt < 3; attempt++ {
		var reqBody io.Reader
		if body != nil {
			raw, err := json.Marshal(body)
			if err != nil {
				return nil, 0, fmt.Errorf("vantara: failed to encode request body: %w", err)
			}
			reqBody = bytes.NewReader(raw)
		}
		req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
		if err != nil {
			return nil, 0, err
		}
		for k, v := range headers {
			req.Header.Set(k, v)
		}

		resp, err := p.client.Do(req)
		if err != nil {
			return nil, 0, fmt.Errorf("vantara: request %s %s failed: %w", method, url, err)
		}
		data, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return nil, resp.StatusCode, fmt.Errorf("vantara: failed to read response: %w", readErr)
		}
		lastStatus = resp.StatusCode

		if resp.StatusCode == http.StatusServiceUnavailable {
			klog.Warningf("vantara: %s %s returned 503, retrying (%d/3)", method, url, attempt+1)
			select {
			case <-ctx.Done():
				return nil, lastStatus, ctx.Err()
			case <-time.After(10 * time.Second):
			}
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, resp.StatusCode, fmt.Errorf("vantara: %s %s failed with status %d: %s", method, url, resp.StatusCode, strings.TrimSpace(string(data)))
		}

		result := map[string]any{}
		if len(bytes.TrimSpace(data)) > 0 {
			if err := json.Unmarshal(data, &result); err != nil {
				return nil, resp.StatusCode, fmt.Errorf("vantara: failed to decode response from %s: %w", url, err)
			}
		}
		return result, resp.StatusCode, nil
	}
	return nil, lastStatus, fmt.Errorf("vantara: %s %s still unavailable (503) after retries", method, url)
}

// invokeJob issues a mutating request and polls the resulting async job to
// completion, returning the final job object.
func (p *VantaraStorageProvider) invokeJob(ctx context.Context, method, url string, body any) (map[string]any, error) {
	headers := p.sessionHeaders()
	headers["Response-Job-Status"] = "Completed"

	resp, _, err := p.doJSON(ctx, method, url, body, headers)
	if err != nil {
		return nil, err
	}
	jobIDFloat, ok := resp["jobId"].(float64)
	if !ok {
		// Some deployments answer synchronously when Response-Job-Status is
		// honored; treat a body without jobId as already complete.
		return resp, nil
	}
	jobURL := fmt.Sprintf("%s/jobs/%d", p.baseURL, int(jobIDFloat))

	wait := time.Second
	for attempt := 0; attempt < 20; attempt++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(wait):
		}
		job, _, err := p.doJSON(ctx, http.MethodGet, jobURL, nil, p.sessionHeaders())
		if err != nil {
			return nil, fmt.Errorf("vantara: failed to poll job: %w", err)
		}
		state, _ := job["state"].(string)
		status, _ := job["status"].(string)
		if state == "Failed" {
			return nil, fmt.Errorf("vantara: job %s failed: %v", jobURL, job["error"])
		}
		if status == "Completed" {
			if state != "" && state != "Succeeded" {
				return nil, fmt.Errorf("vantara: job %s completed with state %s: %v", jobURL, state, job["error"])
			}
			return job, nil
		}
		if wait *= 2; wait > 15*time.Second {
			wait = 15 * time.Second
		}
	}
	return nil, fmt.Errorf("vantara: timed out waiting for job %s", jobURL)
}

// getLdevWithNAA fetches an LDEV, retrying briefly until naaId is populated.
func (p *VantaraStorageProvider) getLdevWithNAA(ctx context.Context, ldevID int) (*ldevInfo, error) {
	var info *ldevInfo
	for attempt := 0; attempt < 5; attempt++ {
		resp, _, err := p.doJSON(ctx, http.MethodGet, fmt.Sprintf("%s/ldevs/%d", p.baseURL, ldevID), nil, p.sessionHeaders())
		if err != nil {
			return nil, fmt.Errorf("vantara: failed to get LDEV %d: %w", ldevID, err)
		}
		info = ldevFromMap(resp)
		if info.NaaID != "" {
			return info, nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	return nil, fmt.Errorf("vantara: LDEV %d has no naaId after creation; cannot derive ESXi device path", ldevID)
}

// listLdevs pages through all defined LDEVs.
func (p *VantaraStorageProvider) listLdevs(ctx context.Context) ([]ldevInfo, error) {
	var all []ldevInfo
	head := 0
	for {
		url := fmt.Sprintf("%s/ldevs?ldevOption=defined&count=%d&headLdevId=%d", p.baseURL, listPageSize, head)
		resp, _, err := p.doJSON(ctx, http.MethodGet, url, nil, p.sessionHeaders())
		if err != nil {
			return nil, fmt.Errorf("vantara: failed to list LDEVs: %w", err)
		}
		rows, _ := resp["data"].([]any)
		for _, row := range rows {
			m, ok := row.(map[string]any)
			if !ok {
				continue
			}
			all = append(all, *ldevFromMap(m))
		}
		if len(rows) < listPageSize || len(rows) == 0 {
			return all, nil
		}
		head = all[len(all)-1].LdevID + 1
	}
}

// findLdevByLabel returns the first LDEV whose label matches (case-insensitive),
// or nil when none matches.
func (p *VantaraStorageProvider) findLdevByLabel(ctx context.Context, label string) (*ldevInfo, error) {
	ldevs, err := p.listLdevs(ctx)
	if err != nil {
		return nil, err
	}
	for i := range ldevs {
		if ldevs[i].Label != "" && strings.EqualFold(ldevs[i].Label, label) {
			return &ldevs[i], nil
		}
	}
	return nil, nil
}

func (p *VantaraStorageProvider) listDPPools(ctx context.Context) ([]poolInfo, error) {
	resp, _, err := p.doJSON(ctx, http.MethodGet, p.baseURL+"/pools?poolType=DP", nil, p.sessionHeaders())
	if err != nil {
		return nil, fmt.Errorf("failed to list DP pools: %w", err)
	}
	rows, _ := resp["data"].([]any)
	pools := make([]poolInfo, 0, len(rows))
	for _, row := range rows {
		m, ok := row.(map[string]any)
		if !ok {
			continue
		}
		var pi poolInfo
		if id, ok := m["poolId"].(float64); ok {
			pi.PoolID = int(id)
		}
		pi.PoolName, _ = m["poolName"].(string)
		pi.PoolType, _ = m["poolType"].(string)
		pools = append(pools, pi)
	}
	return pools, nil
}

func (p *VantaraStorageProvider) autoPickPool(ctx context.Context) error {
	pools, err := p.listDPPools(ctx)
	if err != nil {
		return err
	}
	switch len(pools) {
	case 0:
		return fmt.Errorf("no DP pools found on the array")
	case 1:
		p.poolID = &pools[0].PoolID
		klog.Infof("vantara: auto-selected the only DP pool %d (%s)", pools[0].PoolID, pools[0].PoolName)
		return nil
	default:
		names := make([]string, 0, len(pools))
		for _, pi := range pools {
			names = append(names, fmt.Sprintf("%d(%s)", pi.PoolID, pi.PoolName))
		}
		return fmt.Errorf("multiple DP pools found (%s); set an explicit pool", strings.Join(names, ", "))
	}
}

func (p *VantaraStorageProvider) bestEffortDeleteLdev(ctx context.Context, ldevID int) {
	if _, err := p.invokeJob(ctx, http.MethodDelete, fmt.Sprintf("%s/ldevs/%d", p.baseURL, ldevID), nil); err != nil {
		klog.Warningf("vantara: best-effort cleanup of LDEV %d failed: %v", ldevID, err)
	}
}

func ldevFromMap(m map[string]any) *ldevInfo {
	info := &ldevInfo{}
	if v, ok := m["ldevId"].(float64); ok {
		info.LdevID = int(v)
	}
	info.Label, _ = m["label"].(string)
	info.NaaID, _ = m["naaId"].(string)
	if v, ok := m["blockCapacity"].(float64); ok {
		info.BlockCapacity = int64(v)
	}
	if v, ok := m["poolId"].(float64); ok {
		info.PoolID = int(v)
	}
	info.EmulationType, _ = m["emulationType"].(string)
	info.Status, _ = m["status"].(string)
	return info
}

func ldevIDFromJob(job map[string]any) (int, error) {
	resources, _ := job["affectedResources"].([]any)
	if len(resources) == 0 {
		return 0, fmt.Errorf("vantara: job completed without affectedResources: %v", job)
	}
	res, _ := resources[0].(string)
	idx := strings.LastIndex(res, "/")
	if idx < 0 || idx+1 >= len(res) {
		return 0, fmt.Errorf("vantara: unexpected affected resource %q", res)
	}
	id, err := strconv.Atoi(res[idx+1:])
	if err != nil {
		return 0, fmt.Errorf("vantara: unexpected LDEV id in affected resource %q: %w", res, err)
	}
	return id, nil
}

func volumeFromLdev(l ldevInfo) storage.Volume {
	return storage.Volume{
		Name: l.Label,
		Size: l.BlockCapacity * 512,
		Id:   strconv.Itoa(l.LdevID),
		NAA:  naaFromID(l.NaaID),
	}
}

func volumeInfoFromLdev(l ldevInfo) storage.VolumeInfo {
	return storage.VolumeInfo{
		Name: l.Label,
		Size: l.BlockCapacity * 512,
		NAA:  naaFromID(l.NaaID),
	}
}

func naaFromID(naaID string) string {
	if naaID == "" {
		return ""
	}
	return "naa." + strings.ToLower(naaID)
}

// sanitizeLabel truncates a volume name to Hitachi's 32-char LDEV label limit.
func sanitizeLabel(name string) string {
	name = strings.TrimSpace(name)
	if len(name) > maxLdevLabelLen {
		return name[:maxLdevLabelLen]
	}
	return name
}

func checkAPIVersion(apiVersion string, majorRequired, minorRequired int) error {
	parts := strings.Split(apiVersion, ".")
	if len(parts) < 2 {
		return fmt.Errorf("vantara: invalid API version format: %q", apiVersion)
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return fmt.Errorf("vantara: invalid major version %q", parts[0])
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return fmt.Errorf("vantara: invalid minor version %q", parts[1])
	}
	if !((major == majorRequired && minor >= minorRequired) || major > majorRequired) {
		return fmt.Errorf("vantara: Configuration Manager REST API %d.%d+ required, array reports %s", majorRequired, minorRequired, apiVersion)
	}
	return nil
}
