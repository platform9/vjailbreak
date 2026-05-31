package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
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
	body, _ := json.Marshal(aiKeyRequest{APIKey: "sk-ant-abc", AdminKey: "my-admin"})
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
		Data:       map[string][]byte{"api-key": []byte("old"), "admin-key": []byte("old-admin")},
	}
	h := &aiKeyHandler{k8sClient: fakeK8sClientForKeyTest(secret)}
	body, _ := json.Marshal(aiKeyRequest{APIKey: "sk-ant-new", AdminKey: "new-admin"})
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
