package server

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	aiSecretName = "vjailbreak-ai-secret"
	aiSecretNS   = "migration-system"
)

type aiKeyHandler struct {
	k8sClient client.Client
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
	if req.AdminKey == "" {
		http.Error(w, "admin_key is required", http.StatusBadRequest)
		return
	}

	ctx := context.Background()
	secretData := map[string][]byte{
		"api-key":   []byte(req.APIKey),
		"admin-key": []byte(req.AdminKey),
	}

	var existing corev1.Secret
	err := h.k8sClient.Get(ctx, types.NamespacedName{Name: aiSecretName, Namespace: aiSecretNS}, &existing)
	if k8serrors.IsNotFound(err) {
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
	} else if err == nil {
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(aiKeyResponse{Configured: true}) //nolint:errcheck
}
