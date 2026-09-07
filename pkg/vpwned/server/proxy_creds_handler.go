package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	proxyCredsSecretName = "pf9-proxy-creds"
	proxyCredsSecretNS   = "migration-system"
)

var proxyCredsDeployments = []string{"migration-controller-manager", "migration-vpwned-sdk"}

type proxyCredsHandler struct {
	k8sClient client.Client
	rawK8s    kubernetes.Interface
}

type proxyCredsRequest struct {
	Username      string `json:"username"`
	Password      string `json:"password"`
	HTTPSOverride bool   `json:"https_override"`
	HTTPSUsername string `json:"https_username"`
	HTTPSPassword string `json:"https_password"`
}

type proxyCredsResponse struct {
	Configured    bool `json:"configured"`
	HTTPSOverride bool `json:"https_override"`
}

func (h *proxyCredsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.getCreds(w, r)
	case http.MethodPost:
		h.saveCreds(w, r)
	case http.MethodDelete:
		h.deleteCreds(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *proxyCredsHandler) getCreds(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	var secret corev1.Secret
	err := h.k8sClient.Get(ctx, types.NamespacedName{Name: proxyCredsSecretName, Namespace: proxyCredsSecretNS}, &secret)
	switch {
	case err == nil:
		resp := proxyCredsResponse{
			Configured:    len(secret.Data["HTTP_PROXY_USERNAME"]) > 0,
			HTTPSOverride: string(secret.Data["HTTPS_PROXY_OVERRIDE"]) == "true",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp) //nolint:errcheck
	case k8serrors.IsNotFound(err):
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(proxyCredsResponse{}) //nolint:errcheck
	default:
		logrus.Errorf("proxy_creds_handler: get secret failed: %v", err)
		http.Error(w, "failed to fetch proxy credentials", http.StatusInternalServerError)
	}
}

func (h *proxyCredsHandler) saveCreds(w http.ResponseWriter, r *http.Request) {
	var req proxyCredsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Username == "" || req.Password == "" {
		http.Error(w, "username and password are required", http.StatusBadRequest)
		return
	}
	if req.HTTPSOverride && (req.HTTPSUsername == "" || req.HTTPSPassword == "") {
		http.Error(w, "https_username and https_password are required when https_override is true", http.StatusBadRequest)
		return
	}

	// HTTP and HTTPS credentials are always stored as fully independent keys.
	// When there's no override, the shared credentials are duplicated into the
	// HTTPS_* keys here so VjbNet never needs any HTTP/HTTPS fallback logic.
	httpsUsername, httpsPassword := req.Username, req.Password
	if req.HTTPSOverride {
		httpsUsername, httpsPassword = req.HTTPSUsername, req.HTTPSPassword
	}

	data := map[string][]byte{
		"HTTP_PROXY_USERNAME":  []byte(req.Username),
		"HTTP_PROXY_PASSWORD":  []byte(req.Password),
		"HTTPS_PROXY_USERNAME": []byte(httpsUsername),
		"HTTPS_PROXY_PASSWORD": []byte(httpsPassword),
		"HTTPS_PROXY_OVERRIDE": []byte(strconv.FormatBool(req.HTTPSOverride)),
	}

	ctx := context.Background()
	var existing corev1.Secret
	getErr := h.k8sClient.Get(ctx, types.NamespacedName{Name: proxyCredsSecretName, Namespace: proxyCredsSecretNS}, &existing)
	switch {
	case k8serrors.IsNotFound(getErr):
		newSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: proxyCredsSecretName, Namespace: proxyCredsSecretNS},
			Data:       data,
		}
		if err := h.k8sClient.Create(ctx, newSecret); err != nil {
			logrus.Errorf("proxy_creds_handler: create secret failed: %v", err)
			http.Error(w, "failed to save proxy credentials", http.StatusInternalServerError)
			return
		}
	case getErr == nil:
		existing.Data = data
		if err := h.k8sClient.Update(ctx, &existing); err != nil {
			logrus.Errorf("proxy_creds_handler: update secret failed: %v", err)
			http.Error(w, "failed to update proxy credentials", http.StatusInternalServerError)
			return
		}
	default:
		logrus.Errorf("proxy_creds_handler: get secret failed: %v", getErr)
		http.Error(w, "unexpected error", http.StatusInternalServerError)
		return
	}

	h.restartProxyDeployments(ctx)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(proxyCredsResponse{Configured: true, HTTPSOverride: req.HTTPSOverride}) //nolint:errcheck
}

// deleteCreds removes the whole secret - it holds nothing but proxy
// credentials, so "clear" means delete, not clear individual keys.
func (h *proxyCredsHandler) deleteCreds(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: proxyCredsSecretName, Namespace: proxyCredsSecretNS}}
	if err := h.k8sClient.Delete(ctx, secret); err != nil && !k8serrors.IsNotFound(err) {
		logrus.Errorf("proxy_creds_handler: delete secret failed: %v", err)
		http.Error(w, "failed to clear proxy credentials", http.StatusInternalServerError)
		return
	}

	h.restartProxyDeployments(ctx)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(proxyCredsResponse{}) //nolint:errcheck
}

// restartProxyDeployments patches each deployment's pod template annotation with
// the current timestamp, triggering a rolling restart so already-running pods
// pick up the new HTTP(S)_PROXY_USERNAME/PASSWORD values from the secret.
func (h *proxyCredsHandler) restartProxyDeployments(ctx context.Context) {
	if h.rawK8s == nil {
		return
	}
	patch := []byte(fmt.Sprintf(
		`{"spec":{"template":{"metadata":{"annotations":{"kubectl.kubernetes.io/restartedAt":"%s"}}}}}`,
		time.Now().UTC().Format(time.RFC3339),
	))
	for _, name := range proxyCredsDeployments {
		_, err := h.rawK8s.AppsV1().Deployments(proxyCredsSecretNS).Patch(
			ctx, name, types.MergePatchType, patch, metav1.PatchOptions{},
		)
		if err != nil {
			logrus.Warnf("proxy_creds_handler: failed to restart %s deployment: %v", name, err)
		} else {
			logrus.Infof("proxy_creds_handler: triggered rolling restart of %s", name)
		}
	}
}
