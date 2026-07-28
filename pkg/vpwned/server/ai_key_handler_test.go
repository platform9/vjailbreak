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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func fakeK8sClientForKeyTest(objs ...client.Object) client.Client {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
}

func TestAIKeyHandler_GetAbsent(t *testing.T) {
	h := &aiKeyHandler{k8sClient: fakeK8sClientForKeyTest()}
	req := httptest.NewRequest(http.MethodGet, "/vpw/v1/ai/key", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp aiKeyResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Configured {
		t.Error("expected configured=false when secret absent")
	}
}

func TestAIKeyHandler_GetPresent(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: aiSecretName, Namespace: aiSecretNS},
		Data:       map[string][]byte{"api-key": []byte("sk-ant-test")},
	}
	h := &aiKeyHandler{k8sClient: fakeK8sClientForKeyTest(secret)}
	req := httptest.NewRequest(http.MethodGet, "/vpw/v1/ai/key", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	var resp aiKeyResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if !resp.Configured {
		t.Error("expected configured=true when secret present")
	}
}

func TestAIKeyHandler_PostCreates(t *testing.T) {
	h := &aiKeyHandler{k8sClient: fakeK8sClientForKeyTest()}
	body, _ := json.Marshal(aiKeyRequest{APIKey: "sk-ant-abc"})
	req := httptest.NewRequest(http.MethodPost, "/vpw/v1/ai/key", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp aiKeyResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if !resp.Configured {
		t.Error("expected configured=true after POST")
	}
}

func TestAIKeyHandler_PostUpdates(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: aiSecretName, Namespace: aiSecretNS},
		Data:       map[string][]byte{"api-key": []byte("old")},
	}
	h := &aiKeyHandler{k8sClient: fakeK8sClientForKeyTest(secret)}
	body, _ := json.Marshal(aiKeyRequest{APIKey: "sk-ant-new"})
	req := httptest.NewRequest(http.MethodPost, "/vpw/v1/ai/key", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestAIKeyHandler_PostMissingKey(t *testing.T) {
	h := &aiKeyHandler{k8sClient: fakeK8sClientForKeyTest()}
	req := httptest.NewRequest(http.MethodPost, "/vpw/v1/ai/key", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// TestAIKeyHandler_PostCreatesAdminKey verifies that saving a key to a brand-new secret
// also auto-generates an admin-key, so the vjailbreak-ai pod can start without crashing.
func TestAIKeyHandler_PostCreatesAdminKey(t *testing.T) {
	k8s := fakeK8sClientForKeyTest()
	h := &aiKeyHandler{k8sClient: k8s}
	body, _ := json.Marshal(aiKeyRequest{APIKey: "sk-ant-abc"})
	req := httptest.NewRequest(http.MethodPost, "/vpw/v1/ai/key", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(httptest.NewRecorder(), req)

	var secret corev1.Secret
	if err := k8s.Get(context.Background(), types.NamespacedName{Name: aiSecretName, Namespace: aiSecretNS}, &secret); err != nil {
		t.Fatalf("secret not found: %v", err)
	}
	if len(secret.Data["admin-key"]) == 0 {
		t.Error("expected admin-key to be auto-generated on create")
	}
	if string(secret.Data["api-key"]) != "sk-ant-abc" {
		t.Errorf("expected api-key=sk-ant-abc, got %q", secret.Data["api-key"])
	}
}

// TestAIKeyHandler_PostPreservesAdminKey verifies that updating an existing secret
// does not overwrite an already-set admin-key.
func TestAIKeyHandler_PostPreservesAdminKey(t *testing.T) {
	existing := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: aiSecretName, Namespace: aiSecretNS},
		Data: map[string][]byte{
			"api-key":   []byte("old-anthropic-key"),
			"admin-key": []byte("existing-admin-key"),
		},
	}
	k8s := fakeK8sClientForKeyTest(existing)
	h := &aiKeyHandler{k8sClient: k8s}
	body, _ := json.Marshal(aiKeyRequest{APIKey: "sk-ant-new"})
	req := httptest.NewRequest(http.MethodPost, "/vpw/v1/ai/key", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(httptest.NewRecorder(), req)

	var secret corev1.Secret
	if err := k8s.Get(context.Background(), types.NamespacedName{Name: aiSecretName, Namespace: aiSecretNS}, &secret); err != nil {
		t.Fatalf("secret not found: %v", err)
	}
	if string(secret.Data["admin-key"]) != "existing-admin-key" {
		t.Errorf("admin-key must not change on api-key update, got %q", secret.Data["admin-key"])
	}
	if string(secret.Data["api-key"]) != "sk-ant-new" {
		t.Errorf("expected api-key=sk-ant-new, got %q", secret.Data["api-key"])
	}
}

// TestAIKeyHandler_PostGeneratesAdminKeyIfMissing verifies that updating an existing secret
// that lacks admin-key (upgrade path) auto-generates one.
func TestAIKeyHandler_PostGeneratesAdminKeyIfMissing(t *testing.T) {
	existing := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: aiSecretName, Namespace: aiSecretNS},
		Data:       map[string][]byte{"api-key": []byte("old-key")},
	}
	k8s := fakeK8sClientForKeyTest(existing)
	h := &aiKeyHandler{k8sClient: k8s}
	body, _ := json.Marshal(aiKeyRequest{APIKey: "sk-ant-new"})
	req := httptest.NewRequest(http.MethodPost, "/vpw/v1/ai/key", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(httptest.NewRecorder(), req)

	var secret corev1.Secret
	if err := k8s.Get(context.Background(), types.NamespacedName{Name: aiSecretName, Namespace: aiSecretNS}, &secret); err != nil {
		t.Fatalf("secret not found: %v", err)
	}
	if len(secret.Data["admin-key"]) == 0 {
		t.Error("expected admin-key to be generated for existing secret missing it")
	}
}
