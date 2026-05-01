// Package proxyclient provides an authenticated HTTP client for the vjailbreak proxy
// service, used by the migration controller to perform secret and pod operations
// without holding direct Kubernetes RBAC permissions on those resources.
package proxyclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	// DefaultProxyServiceURL is the in-cluster DNS name of the vpwned proxy service.
	DefaultProxyServiceURL = "http://migration-vpwned-service.migration-system.svc.cluster.local:80"

	// SATokenPath is the standard Kubernetes ServiceAccount token mount path.
	SATokenPath = "/var/run/secrets/kubernetes.io/serviceaccount/token" //nolint:gosec // filesystem path, not a credential
)

// Client is a thin HTTP client that calls the vjailbreak proxy's authenticated
// /vpw/v1/k8s/* endpoints on behalf of the migration controller.
type Client struct {
	baseURL    string
	httpClient *http.Client
	tokenPath  string
}

// New creates a new proxy Client. If baseURL is empty the default in-cluster service URL is used.
func New(baseURL string) *Client {
	if baseURL == "" {
		baseURL = DefaultProxyServiceURL
	}
	return &Client{
		baseURL:   strings.TrimRight(baseURL, "/"),
		tokenPath: SATokenPath,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// token reads the current ServiceAccount token from disk (refreshed automatically by kubelet).
func (c *Client) token() (string, error) {
	data, err := os.ReadFile(c.tokenPath)
	if err != nil {
		return "", fmt.Errorf("proxyclient: read SA token from %s: %w", c.tokenPath, err)
	}
	return strings.TrimSpace(string(data)), nil
}

// do executes an HTTP request with the SA Bearer token in the Authorization header.
func (c *Client) do(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("proxyclient: marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("proxyclient: create request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	tok, err := c.token()
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+tok)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("proxyclient: %s %s: %w", method, path, err)
	}
	return resp, nil
}

// errFromStatus converts a non-2xx status code into a typed Kubernetes error where possible.
func errFromStatus(statusCode int, body []byte) error {
	msg := string(body)
	switch statusCode {
	case http.StatusNotFound:
		return apierrors.NewNotFound(schema.GroupResource{}, msg)
	case http.StatusConflict:
		return apierrors.NewAlreadyExists(schema.GroupResource{}, msg)
	default:
		return fmt.Errorf("proxyclient: unexpected status %d: %s", statusCode, msg)
	}
}

// doGetJSON makes a GET request to path and JSON-decodes a successful response into dest.
// Extracted to eliminate structural duplication between GetSecret and GetPod.
func (c *Client) doGetJSON(ctx context.Context, path string, dest interface{}) error {
	resp, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck // close errors on HTTP response bodies are not actionable

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("proxyclient: read response body from %s: %w", path, err)
	}
	if resp.StatusCode != http.StatusOK {
		return errFromStatus(resp.StatusCode, body)
	}
	if err := json.Unmarshal(body, dest); err != nil {
		return fmt.Errorf("proxyclient: decode GET response from %s: %w", path, err)
	}
	return nil
}

// ---- Secret operations ----

// secretResponse mirrors the JSON shape returned by the proxy for a secret.
type secretResponse struct {
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
	Data      map[string][]byte `json:"data"`
	Type      string            `json:"type"`
}

func responseToSecret(r *secretResponse) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.Name,
			Namespace: r.Namespace,
		},
		Type: corev1.SecretType(r.Type),
		Data: r.Data,
	}
}

// GetSecret fetches a secret from the proxy and populates out.
func (c *Client) GetSecret(ctx context.Context, namespace, name string, out *corev1.Secret) error {
	var sr secretResponse
	if err := c.doGetJSON(ctx, fmt.Sprintf("/vpw/v1/k8s/secrets/%s/%s", namespace, name), &sr); err != nil {
		return err
	}
	*out = *responseToSecret(&sr)
	return nil
}

// secretCreateRequest mirrors the proxy's create request body.
type secretCreateRequest struct {
	Name string            `json:"name"`
	Data map[string][]byte `json:"data"`
	Type string            `json:"type"`
}

// CreateSecret creates a secret via the proxy.
func (c *Client) CreateSecret(ctx context.Context, secret *corev1.Secret) error {
	reqBody := secretCreateRequest{Name: secret.Name, Data: secret.Data, Type: string(secret.Type)}
	resp, err := c.do(ctx, http.MethodPost, fmt.Sprintf("/vpw/v1/k8s/secrets/%s", secret.Namespace), reqBody)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("proxyclient: read CreateSecret response body: %w", err)
	}
	if resp.StatusCode != http.StatusCreated {
		return errFromStatus(resp.StatusCode, body)
	}
	var sr secretResponse
	if err := json.Unmarshal(body, &sr); err != nil {
		return fmt.Errorf("proxyclient: decode CreateSecret response: %w", err)
	}
	*secret = *responseToSecret(&sr)
	return nil
}

// secretUpdateRequest mirrors the proxy's update request body.
type secretUpdateRequest struct {
	Data map[string][]byte `json:"data"`
}

// UpdateSecret updates an existing secret's data via the proxy.
func (c *Client) UpdateSecret(ctx context.Context, secret *corev1.Secret) error {
	resp, err := c.do(ctx, http.MethodPut, fmt.Sprintf("/vpw/v1/k8s/secrets/%s/%s", secret.Namespace, secret.Name), secretUpdateRequest{Data: secret.Data})
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("proxyclient: read UpdateSecret response body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return errFromStatus(resp.StatusCode, body)
	}
	return nil
}

// DeleteSecret deletes a secret via the proxy; returns nil if already gone.
func (c *Client) DeleteSecret(ctx context.Context, namespace, name string) error {
	resp, err := c.do(ctx, http.MethodDelete, fmt.Sprintf("/vpw/v1/k8s/secrets/%s/%s", namespace, name), nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("proxyclient: read DeleteSecret response body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return errFromStatus(resp.StatusCode, body)
	}
	return nil
}

// ---- Pod operations ----

// podInfoResponse mirrors the JSON shape returned by the proxy for a pod.
type podInfoResponse struct {
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
	Phase     string            `json:"phase"`
	Labels    map[string]string `json:"labels"`
	NodeName  string            `json:"node_name"`
	PodIP     string            `json:"pod_ip"`
}

func responseToPod(r *podInfoResponse) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.Name,
			Namespace: r.Namespace,
			Labels:    r.Labels,
		},
		Spec: corev1.PodSpec{NodeName: r.NodeName},
		Status: corev1.PodStatus{
			Phase: corev1.PodPhase(r.Phase),
			PodIP: r.PodIP,
		},
	}
}

// ListPods fetches pods in namespace filtered by the given label selector string (e.g. "vmName=my-vm").
func (c *Client) ListPods(ctx context.Context, namespace, labelSelector string, podList *corev1.PodList) error {
	path := fmt.Sprintf("/vpw/v1/k8s/pods/%s", namespace)
	if labelSelector != "" {
		path += "?labelSelector=" + labelSelector
	}
	resp, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("proxyclient: read ListPods response body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return errFromStatus(resp.StatusCode, body)
	}
	var result struct {
		Pods []podInfoResponse `json:"pods"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("proxyclient: decode ListPods response: %w", err)
	}
	podList.Items = make([]corev1.Pod, len(result.Pods))
	for i, p := range result.Pods {
		pCopy := p
		podList.Items[i] = *responseToPod(&pCopy)
	}
	return nil
}

// GetPod fetches a single pod and populates out.
func (c *Client) GetPod(ctx context.Context, namespace, name string, out *corev1.Pod) error {
	var pr podInfoResponse
	if err := c.doGetJSON(ctx, fmt.Sprintf("/vpw/v1/k8s/pods/%s/%s", namespace, name), &pr); err != nil {
		return err
	}
	*out = *responseToPod(&pr)
	return nil
}

// podUpdateLabelsRequest mirrors the proxy's labels update body.
type podUpdateLabelsRequest struct {
	Labels map[string]string `json:"labels"`
}

// UpdatePodLabels merges the given labels into a pod's label map via the proxy.
func (c *Client) UpdatePodLabels(ctx context.Context, namespace, name string, lbls map[string]string) error {
	resp, err := c.do(ctx, http.MethodPatch, fmt.Sprintf("/vpw/v1/k8s/pods/%s/%s/labels", namespace, name), podUpdateLabelsRequest{Labels: lbls})
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("proxyclient: read UpdatePodLabels response body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return errFromStatus(resp.StatusCode, body)
	}
	return nil
}
