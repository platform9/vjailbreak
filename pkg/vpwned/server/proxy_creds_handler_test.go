package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestProxyCredsHandler_GetAbsent(t *testing.T) {
	h := &proxyCredsHandler{k8sClient: fakeK8sClientForKeyTest()}
	req := httptest.NewRequest(http.MethodGet, "/vpw/v1/proxy/credentials", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp proxyCredsResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Configured {
		t.Error("expected configured=false when secret absent")
	}
}

func TestProxyCredsHandler_GetPresent_SharedOnly(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: proxyCredsSecretName, Namespace: proxyCredsSecretNS},
		Data: map[string][]byte{
			"HTTP_PROXY_USERNAME":  []byte("shared-user"),
			"HTTP_PROXY_PASSWORD":  []byte("shared-pass"),
			"HTTPS_PROXY_USERNAME": []byte("shared-user"),
			"HTTPS_PROXY_PASSWORD": []byte("shared-pass"),
			"HTTPS_PROXY_OVERRIDE": []byte("false"),
		},
	}
	h := &proxyCredsHandler{k8sClient: fakeK8sClientForKeyTest(secret)}
	req := httptest.NewRequest(http.MethodGet, "/vpw/v1/proxy/credentials", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	var resp proxyCredsResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if !resp.Configured {
		t.Error("expected configured=true when secret present")
	}
	if resp.HTTPSOverride {
		t.Error("expected https_override=false for shared-only credentials")
	}
}

func TestProxyCredsHandler_GetPresent_WithOverride(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: proxyCredsSecretName, Namespace: proxyCredsSecretNS},
		Data: map[string][]byte{
			"HTTP_PROXY_USERNAME":  []byte("http-user"),
			"HTTP_PROXY_PASSWORD":  []byte("http-pass"),
			"HTTPS_PROXY_USERNAME": []byte("https-user"),
			"HTTPS_PROXY_PASSWORD": []byte("https-pass"),
			"HTTPS_PROXY_OVERRIDE": []byte("true"),
		},
	}
	h := &proxyCredsHandler{k8sClient: fakeK8sClientForKeyTest(secret)}
	req := httptest.NewRequest(http.MethodGet, "/vpw/v1/proxy/credentials", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	var resp proxyCredsResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if !resp.Configured || !resp.HTTPSOverride {
		t.Errorf("expected configured=true, https_override=true, got %+v", resp)
	}
}

func TestProxyCredsHandler_PostCreates_SharedOnly(t *testing.T) {
	k8s := fakeK8sClientForKeyTest()
	h := &proxyCredsHandler{k8sClient: k8s}
	body, _ := json.Marshal(proxyCredsRequest{Username: "shared-user", Password: "shared-pass"})
	req := httptest.NewRequest(http.MethodPost, "/vpw/v1/proxy/credentials", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var secret corev1.Secret
	if err := k8s.Get(context.Background(), types.NamespacedName{Name: proxyCredsSecretName, Namespace: proxyCredsSecretNS}, &secret); err != nil {
		t.Fatalf("secret not found: %v", err)
	}
	if string(secret.Data["HTTP_PROXY_USERNAME"]) != "shared-user" || string(secret.Data["HTTP_PROXY_PASSWORD"]) != "shared-pass" {
		t.Errorf("HTTP credentials not stored correctly: %+v", secret.Data)
	}
	if string(secret.Data["HTTPS_PROXY_USERNAME"]) != "shared-user" || string(secret.Data["HTTPS_PROXY_PASSWORD"]) != "shared-pass" {
		t.Errorf("expected shared credentials duplicated into HTTPS_* keys, got %+v", secret.Data)
	}
	if string(secret.Data["HTTPS_PROXY_OVERRIDE"]) != "false" {
		t.Errorf("expected HTTPS_PROXY_OVERRIDE=false, got %q", secret.Data["HTTPS_PROXY_OVERRIDE"])
	}
}

func TestProxyCredsHandler_PostCreates_WithOverride(t *testing.T) {
	k8s := fakeK8sClientForKeyTest()
	h := &proxyCredsHandler{k8sClient: k8s}
	body, _ := json.Marshal(proxyCredsRequest{
		Username:      "http-user",
		Password:      "http-pass",
		HTTPSOverride: true,
		HTTPSUsername: "https-user",
		HTTPSPassword: "https-pass",
	})
	req := httptest.NewRequest(http.MethodPost, "/vpw/v1/proxy/credentials", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var secret corev1.Secret
	if err := k8s.Get(context.Background(), types.NamespacedName{Name: proxyCredsSecretName, Namespace: proxyCredsSecretNS}, &secret); err != nil {
		t.Fatalf("secret not found: %v", err)
	}
	if string(secret.Data["HTTP_PROXY_USERNAME"]) != "http-user" || string(secret.Data["HTTP_PROXY_PASSWORD"]) != "http-pass" {
		t.Errorf("HTTP credentials not stored correctly: %+v", secret.Data)
	}
	if string(secret.Data["HTTPS_PROXY_USERNAME"]) != "https-user" || string(secret.Data["HTTPS_PROXY_PASSWORD"]) != "https-pass" {
		t.Errorf("HTTPS override credentials not stored correctly: %+v", secret.Data)
	}
	if string(secret.Data["HTTPS_PROXY_OVERRIDE"]) != "true" {
		t.Errorf("expected HTTPS_PROXY_OVERRIDE=true, got %q", secret.Data["HTTPS_PROXY_OVERRIDE"])
	}
}

func TestProxyCredsHandler_PostUpdates(t *testing.T) {
	existing := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: proxyCredsSecretName, Namespace: proxyCredsSecretNS},
		Data: map[string][]byte{
			"HTTP_PROXY_USERNAME":  []byte("old-user"),
			"HTTP_PROXY_PASSWORD":  []byte("old-pass"),
			"HTTPS_PROXY_USERNAME": []byte("old-user"),
			"HTTPS_PROXY_PASSWORD": []byte("old-pass"),
			"HTTPS_PROXY_OVERRIDE": []byte("false"),
		},
	}
	k8s := fakeK8sClientForKeyTest(existing)
	h := &proxyCredsHandler{k8sClient: k8s}
	body, _ := json.Marshal(proxyCredsRequest{Username: "new-user", Password: "new-pass"})
	req := httptest.NewRequest(http.MethodPost, "/vpw/v1/proxy/credentials", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var secret corev1.Secret
	if err := k8s.Get(context.Background(), types.NamespacedName{Name: proxyCredsSecretName, Namespace: proxyCredsSecretNS}, &secret); err != nil {
		t.Fatalf("secret not found: %v", err)
	}
	if string(secret.Data["HTTP_PROXY_USERNAME"]) != "new-user" {
		t.Errorf("expected updated username, got %q", secret.Data["HTTP_PROXY_USERNAME"])
	}
}

func TestProxyCredsHandler_PostMissingUsername(t *testing.T) {
	h := &proxyCredsHandler{k8sClient: fakeK8sClientForKeyTest()}
	req := httptest.NewRequest(http.MethodPost, "/vpw/v1/proxy/credentials", bytes.NewBufferString(`{"password":"pass"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestProxyCredsHandler_PostMissingPassword(t *testing.T) {
	h := &proxyCredsHandler{k8sClient: fakeK8sClientForKeyTest()}
	req := httptest.NewRequest(http.MethodPost, "/vpw/v1/proxy/credentials", bytes.NewBufferString(`{"username":"user"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestProxyCredsHandler_PostOverrideMissingFields(t *testing.T) {
	h := &proxyCredsHandler{k8sClient: fakeK8sClientForKeyTest()}
	body, _ := json.Marshal(proxyCredsRequest{Username: "user", Password: "pass", HTTPSOverride: true})
	req := httptest.NewRequest(http.MethodPost, "/vpw/v1/proxy/credentials", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 when https_override is set without https_username/https_password, got %d", w.Code)
	}
}

func TestProxyCredsHandler_Delete_RemovesSecret(t *testing.T) {
	existing := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: proxyCredsSecretName, Namespace: proxyCredsSecretNS},
		Data: map[string][]byte{
			"HTTP_PROXY_USERNAME": []byte("user"),
			"HTTP_PROXY_PASSWORD": []byte("pass"),
		},
	}
	k8s := fakeK8sClientForKeyTest(existing)
	h := &proxyCredsHandler{k8sClient: k8s}
	req := httptest.NewRequest(http.MethodDelete, "/vpw/v1/proxy/credentials", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp proxyCredsResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Configured {
		t.Error("expected configured=false after delete")
	}

	var secret corev1.Secret
	err := k8s.Get(context.Background(), types.NamespacedName{Name: proxyCredsSecretName, Namespace: proxyCredsSecretNS}, &secret)
	if err == nil {
		t.Error("expected secret to be deleted, but it still exists")
	}
}

func TestProxyCredsHandler_Delete_IdempotentWhenAbsent(t *testing.T) {
	h := &proxyCredsHandler{k8sClient: fakeK8sClientForKeyTest()}
	req := httptest.NewRequest(http.MethodDelete, "/vpw/v1/proxy/credentials", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 when deleting an absent secret, got %d", w.Code)
	}
}

func TestProxyCredsHandler_MethodNotAllowed(t *testing.T) {
	h := &proxyCredsHandler{k8sClient: fakeK8sClientForKeyTest()}
	req := httptest.NewRequest(http.MethodPut, "/vpw/v1/proxy/credentials", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}
