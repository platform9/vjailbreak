package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/sirupsen/logrus"
	api "github.com/platform9/vjailbreak/pkg/vpwned/api/proto/v1/service"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// k8sResourceHandler serves authenticated Kubernetes secret and pod operations.
type k8sResourceHandler struct {
	k8sClient client.Client
}

func newK8sResourceHandler(k8sClient client.Client) *k8sResourceHandler {
	return &k8sResourceHandler{k8sClient: k8sClient}
}

// writeJSON writes v as JSON with the given HTTP status code.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		logrus.WithError(err).Error("k8s-resource-handler: failed to encode response")
	}
}

// writeError writes a JSON error object.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// parseNamespaceName extracts {namespace}/{name} from a URL path suffix.
func parseNamespaceName(r *http.Request, basePath string) (namespace, name string, ok bool) {
	suffix := strings.TrimPrefix(r.URL.Path, basePath)
	suffix = strings.Trim(suffix, "/")
	parts := strings.SplitN(suffix, "/", 2)
	if len(parts) == 2 && parts[0] != "" && parts[1] != "" {
		return parts[0], parts[1], true
	}
	return "", "", false
}

// parseNamespace extracts {namespace} from a URL path suffix.
func parseNamespace(r *http.Request, basePath string) (namespace string, ok bool) {
	suffix := strings.TrimPrefix(r.URL.Path, basePath)
	suffix = strings.Trim(suffix, "/")
	parts := strings.SplitN(suffix, "/", 2)
	if len(parts) >= 1 && parts[0] != "" {
		return parts[0], true
	}
	return "", false
}

// secretResponse is the JSON shape returned for a secret.
type secretResponse struct {
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
	Data      map[string][]byte `json:"data"`
	Type      string            `json:"type"`
}

func secretToResponse(s *corev1.Secret) secretResponse {
	return secretResponse{
		Name:      s.Name,
		Namespace: s.Namespace,
		Data:      s.Data,
		Type:      string(s.Type),
	}
}

// handleGetSecret handles GET /vpw/v1/k8s/secrets/{namespace}/{name}
func (h *k8sResourceHandler) handleGetSecret(w http.ResponseWriter, r *http.Request) {
	namespace, name, ok := parseNamespaceName(r, "/vpw/v1/k8s/secrets/")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid path: expected /vpw/v1/k8s/secrets/{namespace}/{name}")
		return
	}

	secret := &corev1.Secret{}
	if err := h.k8sClient.Get(r.Context(), k8stypes.NamespacedName{Namespace: namespace, Name: name}, secret); err != nil {
		if apierrors.IsNotFound(err) {
			writeError(w, http.StatusNotFound, fmt.Sprintf("secret %s/%s not found", namespace, name))
			return
		}
		logrus.WithError(err).Errorf("k8s-resource-handler: get secret %s/%s", namespace, name)
		writeError(w, http.StatusInternalServerError, "failed to get secret")
		return
	}

	writeJSON(w, http.StatusOK, secretToResponse(secret))
}

// secretCreateRequest is the JSON body for creating a secret.
type secretCreateRequest struct {
	Name string            `json:"name"`
	Data map[string][]byte `json:"data"`
	Type string            `json:"type"`
}

// handleCreateSecret handles POST /vpw/v1/k8s/secrets/{namespace}
func (h *k8sResourceHandler) handleCreateSecret(w http.ResponseWriter, r *http.Request) {
	namespace, ok := parseNamespace(r, "/vpw/v1/k8s/secrets/")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid path: expected /vpw/v1/k8s/secrets/{namespace}")
		return
	}

	var req secretCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	secretType := corev1.SecretTypeOpaque
	if req.Type != "" {
		secretType = corev1.SecretType(req.Type)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      req.Name,
			Namespace: namespace,
		},
		Type: secretType,
		Data: req.Data,
	}

	if err := h.k8sClient.Create(r.Context(), secret); err != nil {
		if apierrors.IsAlreadyExists(err) {
			writeError(w, http.StatusConflict, fmt.Sprintf("secret %s/%s already exists", namespace, req.Name))
			return
		}
		logrus.WithError(err).Errorf("k8s-resource-handler: create secret %s/%s", namespace, req.Name)
		writeError(w, http.StatusInternalServerError, "failed to create secret")
		return
	}

	writeJSON(w, http.StatusCreated, secretToResponse(secret))
}

// secretUpdateRequest is the JSON body for updating a secret.
type secretUpdateRequest struct {
	Data map[string][]byte `json:"data"`
}

// handleUpdateSecret handles PUT /vpw/v1/k8s/secrets/{namespace}/{name}
func (h *k8sResourceHandler) handleUpdateSecret(w http.ResponseWriter, r *http.Request) {
	namespace, name, ok := parseNamespaceName(r, "/vpw/v1/k8s/secrets/")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid path: expected /vpw/v1/k8s/secrets/{namespace}/{name}")
		return
	}

	var req secretUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	secret := &corev1.Secret{}
	if err := h.k8sClient.Get(r.Context(), k8stypes.NamespacedName{Namespace: namespace, Name: name}, secret); err != nil {
		if apierrors.IsNotFound(err) {
			writeError(w, http.StatusNotFound, fmt.Sprintf("secret %s/%s not found", namespace, name))
			return
		}
		logrus.WithError(err).Errorf("k8s-resource-handler: get secret for update %s/%s", namespace, name)
		writeError(w, http.StatusInternalServerError, "failed to get secret")
		return
	}

	secret.Data = req.Data
	if err := h.k8sClient.Update(r.Context(), secret); err != nil {
		logrus.WithError(err).Errorf("k8s-resource-handler: update secret %s/%s", namespace, name)
		writeError(w, http.StatusInternalServerError, "failed to update secret")
		return
	}

	writeJSON(w, http.StatusOK, secretToResponse(secret))
}

// handleDeleteSecret handles DELETE /vpw/v1/k8s/secrets/{namespace}/{name}
func (h *k8sResourceHandler) handleDeleteSecret(w http.ResponseWriter, r *http.Request) {
	namespace, name, ok := parseNamespaceName(r, "/vpw/v1/k8s/secrets/")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid path: expected /vpw/v1/k8s/secrets/{namespace}/{name}")
		return
	}

	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}}
	if err := h.k8sClient.Delete(r.Context(), secret); err != nil {
		if apierrors.IsNotFound(err) {
			writeJSON(w, http.StatusOK, map[string]string{"message": "secret not found, nothing to delete"})
			return
		}
		logrus.WithError(err).Errorf("k8s-resource-handler: delete secret %s/%s", namespace, name)
		writeError(w, http.StatusInternalServerError, "failed to delete secret")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": fmt.Sprintf("secret %s/%s deleted", namespace, name)})
}

// podInfoResponse is the JSON shape for a pod.
type podInfoResponse struct {
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
	Phase     string            `json:"phase"`
	Labels    map[string]string `json:"labels"`
	NodeName  string            `json:"node_name"`
	PodIP     string            `json:"pod_ip"`
}

func podToResponse(p *corev1.Pod) podInfoResponse {
	return podInfoResponse{
		Name:      p.Name,
		Namespace: p.Namespace,
		Phase:     string(p.Status.Phase),
		Labels:    p.Labels,
		NodeName:  p.Spec.NodeName,
		PodIP:     p.Status.PodIP,
	}
}

// handleListPods handles GET /vpw/v1/k8s/pods/{namespace}?labelSelector=key%3Dvalue
func (h *k8sResourceHandler) handleListPods(w http.ResponseWriter, r *http.Request) {
	namespace, ok := parseNamespace(r, "/vpw/v1/k8s/pods/")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid path: expected /vpw/v1/k8s/pods/{namespace}")
		return
	}

	listOpts := []client.ListOption{client.InNamespace(namespace)}

	if sel := r.URL.Query().Get("labelSelector"); sel != "" {
		parsed, err := labels.Parse(sel)
		if err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid labelSelector: %v", err))
			return
		}
		listOpts = append(listOpts, client.MatchingLabelsSelector{Selector: parsed})
	}

	podList := &corev1.PodList{}
	if err := h.k8sClient.List(r.Context(), podList, listOpts...); err != nil {
		logrus.WithError(err).Errorf("k8s-resource-handler: list pods in %s", namespace)
		writeError(w, http.StatusInternalServerError, "failed to list pods")
		return
	}

	result := make([]podInfoResponse, len(podList.Items))
	for i := range podList.Items {
		result[i] = podToResponse(&podList.Items[i])
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"pods": result})
}

// handleGetPod handles GET /vpw/v1/k8s/pods/{namespace}/{name}
func (h *k8sResourceHandler) handleGetPod(w http.ResponseWriter, r *http.Request) {
	namespace, name, ok := parseNamespaceName(r, "/vpw/v1/k8s/pods/")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid path: expected /vpw/v1/k8s/pods/{namespace}/{name}")
		return
	}

	pod := &corev1.Pod{}
	if err := h.k8sClient.Get(r.Context(), k8stypes.NamespacedName{Namespace: namespace, Name: name}, pod); err != nil {
		if apierrors.IsNotFound(err) {
			writeError(w, http.StatusNotFound, fmt.Sprintf("pod %s/%s not found", namespace, name))
			return
		}
		logrus.WithError(err).Errorf("k8s-resource-handler: get pod %s/%s", namespace, name)
		writeError(w, http.StatusInternalServerError, "failed to get pod")
		return
	}

	writeJSON(w, http.StatusOK, podToResponse(pod))
}

// podUpdateLabelsRequest is the JSON body for updating pod labels.
type podUpdateLabelsRequest struct {
	Labels map[string]string `json:"labels"`
}

// handleUpdatePodLabels handles PATCH /vpw/v1/k8s/pods/{namespace}/{name}/labels
func (h *k8sResourceHandler) handleUpdatePodLabels(w http.ResponseWriter, r *http.Request) {
	// Strip the trailing /labels from the path before parsing namespace/name.
	trimmedPath := strings.TrimSuffix(r.URL.Path, "/labels")
	namespace, name, ok := parseNamespaceName(r, "/vpw/v1/k8s/pods/")
	if !ok {
		// Retry with trimmed path by temporarily adjusting the URL.
		parts := strings.SplitN(strings.TrimPrefix(trimmedPath, "/vpw/v1/k8s/pods/"), "/", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			writeError(w, http.StatusBadRequest, "invalid path: expected /vpw/v1/k8s/pods/{namespace}/{name}/labels")
			return
		}
		namespace, name = parts[0], parts[1]
	}
	// Remove the trailing /labels segment from name if present.
	name = strings.TrimSuffix(name, "/labels")

	var req podUpdateLabelsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	pod := &corev1.Pod{}
	if err := h.k8sClient.Get(r.Context(), k8stypes.NamespacedName{Namespace: namespace, Name: name}, pod); err != nil {
		if apierrors.IsNotFound(err) {
			writeError(w, http.StatusNotFound, fmt.Sprintf("pod %s/%s not found", namespace, name))
			return
		}
		logrus.WithError(err).Errorf("k8s-resource-handler: get pod for label update %s/%s", namespace, name)
		writeError(w, http.StatusInternalServerError, "failed to get pod")
		return
	}

	if pod.Labels == nil {
		pod.Labels = make(map[string]string)
	}
	for k, v := range req.Labels {
		pod.Labels[k] = v
	}

	if err := h.k8sClient.Update(r.Context(), pod); err != nil {
		logrus.WithError(err).Errorf("k8s-resource-handler: update pod labels %s/%s", namespace, name)
		writeError(w, http.StatusInternalServerError, "failed to update pod labels")
		return
	}

	writeJSON(w, http.StatusOK, podToResponse(pod))
}

// k8sResourceProxyGRPC implements the generated K8SResourceProxyServer interface.
type k8sResourceProxyGRPC struct {
	api.UnimplementedK8SResourceProxyServer
	k8sClient client.Client
}

func newK8sResourceProxyGRPC(k8sClient client.Client) *k8sResourceProxyGRPC {
	return &k8sResourceProxyGRPC{k8sClient: k8sClient}
}

func (g *k8sResourceProxyGRPC) GetSecret(ctx context.Context, req *api.GetSecretRequest) (*api.GetSecretResponse, error) {
	secret := &corev1.Secret{}
	if err := g.k8sClient.Get(ctx, k8stypes.NamespacedName{Namespace: req.Namespace, Name: req.Name}, secret); err != nil {
		return nil, err
	}
	data := make(map[string][]byte, len(secret.Data))
	for k, v := range secret.Data {
		data[k] = v
	}
	return &api.GetSecretResponse{
		Name:      secret.Name,
		Namespace: secret.Namespace,
		Secret:    &api.SecretData{Data: data, Type: string(secret.Type)},
	}, nil
}

func (g *k8sResourceProxyGRPC) CreateSecret(ctx context.Context, req *api.CreateSecretRequest) (*api.CreateSecretResponse, error) {
	secretType := corev1.SecretTypeOpaque
	if req.Secret != nil && req.Secret.Type != "" {
		secretType = corev1.SecretType(req.Secret.Type)
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: req.Name, Namespace: req.Namespace},
		Type:       secretType,
	}
	if req.Secret != nil {
		secret.Data = req.Secret.Data
	}
	if err := g.k8sClient.Create(ctx, secret); err != nil {
		return &api.CreateSecretResponse{Success: false, Message: err.Error()}, err
	}
	return &api.CreateSecretResponse{Success: true, Message: "created"}, nil
}

func (g *k8sResourceProxyGRPC) UpdateSecret(ctx context.Context, req *api.UpdateSecretRequest) (*api.UpdateSecretResponse, error) {
	secret := &corev1.Secret{}
	if err := g.k8sClient.Get(ctx, k8stypes.NamespacedName{Namespace: req.Namespace, Name: req.Name}, secret); err != nil {
		return nil, err
	}
	if req.Secret != nil {
		secret.Data = req.Secret.Data
	}
	if err := g.k8sClient.Update(ctx, secret); err != nil {
		return &api.UpdateSecretResponse{Success: false, Message: err.Error()}, err
	}
	return &api.UpdateSecretResponse{Success: true, Message: "updated"}, nil
}

func (g *k8sResourceProxyGRPC) DeleteSecret(ctx context.Context, req *api.DeleteSecretRequest) (*api.DeleteSecretResponse, error) {
	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: req.Name, Namespace: req.Namespace}}
	if err := g.k8sClient.Delete(ctx, secret); err != nil && !apierrors.IsNotFound(err) {
		return &api.DeleteSecretResponse{Success: false, Message: err.Error()}, err
	}
	return &api.DeleteSecretResponse{Success: true, Message: "deleted"}, nil
}

func (g *k8sResourceProxyGRPC) ListPods(ctx context.Context, req *api.ListPodsRequest) (*api.ListPodsResponse, error) {
	listOpts := []client.ListOption{client.InNamespace(req.Namespace)}
	if req.LabelSelector != "" {
		sel, err := labels.Parse(req.LabelSelector)
		if err != nil {
			return nil, fmt.Errorf("invalid label selector %q: %w", req.LabelSelector, err)
		}
		listOpts = append(listOpts, client.MatchingLabelsSelector{Selector: sel})
	}
	podList := &corev1.PodList{}
	if err := g.k8sClient.List(ctx, podList, listOpts...); err != nil {
		return nil, err
	}
	pods := make([]*api.PodInfo, len(podList.Items))
	for i := range podList.Items {
		p := &podList.Items[i]
		pods[i] = &api.PodInfo{
			Name:      p.Name,
			Namespace: p.Namespace,
			Phase:     string(p.Status.Phase),
			Labels:    p.Labels,
			NodeName:  p.Spec.NodeName,
			PodIp:     p.Status.PodIP,
		}
	}
	return &api.ListPodsResponse{Pods: pods}, nil
}

func (g *k8sResourceProxyGRPC) GetPod(ctx context.Context, req *api.GetPodRequest) (*api.GetPodResponse, error) {
	pod := &corev1.Pod{}
	if err := g.k8sClient.Get(ctx, k8stypes.NamespacedName{Namespace: req.Namespace, Name: req.Name}, pod); err != nil {
		return nil, err
	}
	return &api.GetPodResponse{
		Pod: &api.PodInfo{
			Name:      pod.Name,
			Namespace: pod.Namespace,
			Phase:     string(pod.Status.Phase),
			Labels:    pod.Labels,
			NodeName:  pod.Spec.NodeName,
			PodIp:     pod.Status.PodIP,
		},
	}, nil
}

func (g *k8sResourceProxyGRPC) UpdatePodLabels(ctx context.Context, req *api.UpdatePodLabelsRequest) (*api.UpdatePodLabelsResponse, error) {
	pod := &corev1.Pod{}
	if err := g.k8sClient.Get(ctx, k8stypes.NamespacedName{Namespace: req.Namespace, Name: req.Name}, pod); err != nil {
		return nil, err
	}
	if pod.Labels == nil {
		pod.Labels = make(map[string]string)
	}
	for k, v := range req.Labels {
		pod.Labels[k] = v
	}
	if err := g.k8sClient.Update(ctx, pod); err != nil {
		return &api.UpdatePodLabelsResponse{Success: false, Message: err.Error()}, err
	}
	return &api.UpdatePodLabelsResponse{Success: true, Message: "updated"}, nil
}

func (h *k8sResourceHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	switch {
	case strings.HasPrefix(path, "/vpw/v1/k8s/secrets/"):
		suffix := strings.TrimPrefix(path, "/vpw/v1/k8s/secrets/")
		suffix = strings.Trim(suffix, "/")
		segments := strings.SplitN(suffix, "/", 3)
		switch r.Method {
		case http.MethodGet:
			if len(segments) == 2 {
				h.handleGetSecret(w, r)
			} else {
				writeError(w, http.StatusBadRequest, "GET secrets requires {namespace}/{name}")
			}
		case http.MethodPost:
			if len(segments) == 1 {
				h.handleCreateSecret(w, r)
			} else {
				writeError(w, http.StatusBadRequest, "POST secrets requires {namespace}")
			}
		case http.MethodPut:
			if len(segments) == 2 {
				h.handleUpdateSecret(w, r)
			} else {
				writeError(w, http.StatusBadRequest, "PUT secrets requires {namespace}/{name}")
			}
		case http.MethodDelete:
			if len(segments) == 2 {
				h.handleDeleteSecret(w, r)
			} else {
				writeError(w, http.StatusBadRequest, "DELETE secrets requires {namespace}/{name}")
			}
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}

	case strings.HasPrefix(path, "/vpw/v1/k8s/pods/"):
		suffix := strings.TrimPrefix(path, "/vpw/v1/k8s/pods/")
		suffix = strings.Trim(suffix, "/")
		segments := strings.SplitN(suffix, "/", 3)
		switch {
		case r.Method == http.MethodGet && len(segments) == 1:
			h.handleListPods(w, r)
		case r.Method == http.MethodGet && len(segments) == 2:
			h.handleGetPod(w, r)
		case r.Method == http.MethodPatch && len(segments) == 3 && segments[2] == "labels":
			h.handleUpdatePodLabels(w, r)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed or unrecognized path")
		}

	default:
		writeError(w, http.StatusNotFound, "not found")
	}
}
