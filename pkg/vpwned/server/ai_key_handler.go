package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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
	aiSecretName       = "vjailbreak-ai-secret"
	aiSecretNS         = "migration-system"
	aiDeploymentName   = "vjailbreak-ai"
)

type aiKeyHandler struct {
	k8sClient client.Client
	rawK8s    kubernetes.Interface
}

type aiKeyRequest struct {
	APIKey   string `json:"api_key"`
	AdminKey string `json:"admin_key"`
}

type aiKeyResponse struct {
	Configured bool `json:"configured"`
}

func (h *aiKeyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.getKey(w, r)
	case http.MethodPost:
		h.saveKey(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *aiKeyHandler) getKey(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	var secret corev1.Secret
	err := h.k8sClient.Get(ctx, types.NamespacedName{Name: aiSecretName, Namespace: aiSecretNS}, &secret)
	configured := err == nil && len(secret.Data["api-key"]) > 0
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(aiKeyResponse{Configured: configured}) //nolint:errcheck
}

func (h *aiKeyHandler) saveKey(w http.ResponseWriter, r *http.Request) {
	var req aiKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.APIKey == "" {
		http.Error(w, "api_key is required", http.StatusBadRequest)
		return
	}

	ctx := context.Background()

	// Fetch existing secret to preserve admin-key if not provided.
	var existing corev1.Secret
	existingErr := h.k8sClient.Get(ctx, types.NamespacedName{Name: aiSecretName, Namespace: aiSecretNS}, &existing)

	secretData := map[string][]byte{
		"api-key": []byte(req.APIKey),
	}

	switch {
	case req.AdminKey != "":
		secretData["admin-key"] = []byte(req.AdminKey)
	case existingErr == nil && len(existing.Data["admin-key"]) > 0:
		// Preserve existing admin key when caller omits it.
		secretData["admin-key"] = existing.Data["admin-key"]
	default:
		http.Error(w, "admin_key is required", http.StatusBadRequest)
		return
	}

	if k8serrors.IsNotFound(existingErr) {
		newSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      aiSecretName,
				Namespace: aiSecretNS,
			},
			Data: secretData,
		}
		if err := h.k8sClient.Create(ctx, newSecret); err != nil {
			logrus.Errorf("ai_key_handler: create secret failed: %v", err)
			http.Error(w, "failed to save API key", http.StatusInternalServerError)
			return
		}
	} else if existingErr == nil {
		existing.Data = secretData
		if err := h.k8sClient.Update(ctx, &existing); err != nil {
			logrus.Errorf("ai_key_handler: update secret failed: %v", err)
			http.Error(w, "failed to update API key", http.StatusInternalServerError)
			return
		}
	} else {
		http.Error(w, "unexpected error", http.StatusInternalServerError)
		return
	}

	// Restart vjailbreak-ai pod so it picks up the new ANTHROPIC_API_KEY env var.
	h.restartAIDeployment(ctx)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(aiKeyResponse{Configured: true}) //nolint:errcheck
}

// restartAIDeployment patches the vjailbreak-ai deployment's pod template annotation
// with the current timestamp, triggering a rolling restart so pods pick up the
// new ANTHROPIC_API_KEY injected from the secret.
func (h *aiKeyHandler) restartAIDeployment(ctx context.Context) {
	if h.rawK8s == nil {
		return
	}
	patch := []byte(fmt.Sprintf(
		`{"spec":{"template":{"metadata":{"annotations":{"kubectl.kubernetes.io/restartedAt":"%s"}}}}}`,
		time.Now().UTC().Format(time.RFC3339),
	))
	_, err := h.rawK8s.AppsV1().Deployments(aiSecretNS).Patch(
		ctx, aiDeploymentName, types.MergePatchType, patch, metav1.PatchOptions{},
	)
	if err != nil {
		logrus.Warnf("ai_key_handler: failed to restart %s deployment: %v", aiDeploymentName, err)
	} else {
		logrus.Infof("ai_key_handler: triggered rolling restart of %s", aiDeploymentName)
	}
}
