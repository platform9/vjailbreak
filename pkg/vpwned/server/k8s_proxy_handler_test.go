package server

import (
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"strings"
	"testing"
	"time"

	authv1 "k8s.io/api/authentication/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

// clearTokenCache drains the package-level token cache between tests.
func clearTokenCache() {
	tokenCache.Range(func(k, v any) bool {
		tokenCache.Delete(k)
		return true
	})
}

// makeTokenReviewReactor returns a fake reactor for the TokenReview create action.
func makeTokenReviewReactor(authenticated bool, username string) k8stesting.ReactionFunc {
	return func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, &authv1.TokenReview{
			Status: authv1.TokenReviewStatus{
				Authenticated: authenticated,
				User:          authv1.UserInfo{Username: username},
			},
		}, nil
	}
}

// makeTestProxy builds a reverse proxy pointing at a test backend server.
func makeTestProxy(backend *httptest.Server) *httputil.ReverseProxy {
	target, _ := url.Parse(backend.URL)
	return &httputil.ReverseProxy{
		FlushInterval: -1,
		Director: func(req *http.Request) {
			req.URL.Path = strings.TrimPrefix(req.URL.Path, "/vpw/v1/k8s")
			req.URL.Host = target.Host
			req.URL.Scheme = target.Scheme
			req.Header.Del("Authorization")
		},
	}
}

// --- HandleK8sProxy tests ---

func TestHandleK8sProxy_ProxyNotInitialized(t *testing.T) {
	orig := k8sReverseProxy
	k8sReverseProxy = nil
	defer func() { k8sReverseProxy = orig }()

	req := httptest.NewRequest(http.MethodGet, "/vpw/v1/k8s/api/v1/namespaces/migration-system/pods", nil)
	w := httptest.NewRecorder()
	HandleK8sProxy(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", w.Code)
	}
}

func TestHandleK8sProxy_MissingAuthHeader(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	origProxy := k8sReverseProxy
	k8sReverseProxy = makeTestProxy(backend)
	defer func() { k8sReverseProxy = origProxy }()

	req := httptest.NewRequest(http.MethodGet, "/vpw/v1/k8s/api/v1/namespaces/migration-system/pods", nil)
	w := httptest.NewRecorder()
	HandleK8sProxy(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestHandleK8sProxy_NonBearerAuth(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	origProxy := k8sReverseProxy
	k8sReverseProxy = makeTestProxy(backend)
	defer func() { k8sReverseProxy = origProxy }()

	req := httptest.NewRequest(http.MethodGet, "/vpw/v1/k8s/api/v1/namespaces/migration-system/pods", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	w := httptest.NewRecorder()
	HandleK8sProxy(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestHandleK8sProxy_TokenNotAuthenticated(t *testing.T) {
	clearTokenCache()
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	fakeClient := fake.NewSimpleClientset()
	fakeClient.PrependReactor("create", "tokenreviews",
		makeTokenReviewReactor(false, ""))

	origProxy := k8sReverseProxy
	origClient := k8sAuthClient
	k8sReverseProxy = makeTestProxy(backend)
	k8sAuthClient = fakeClient
	defer func() {
		k8sReverseProxy = origProxy
		k8sAuthClient = origClient
	}()

	req := httptest.NewRequest(http.MethodGet, "/vpw/v1/k8s/api/v1/namespaces/migration-system/pods", nil)
	req.Header.Set("Authorization", "Bearer invalidtoken")
	w := httptest.NewRecorder()
	HandleK8sProxy(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestHandleK8sProxy_WrongServiceAccount(t *testing.T) {
	clearTokenCache()
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	fakeClient := fake.NewSimpleClientset()
	fakeClient.PrependReactor("create", "tokenreviews",
		makeTokenReviewReactor(true, "system:serviceaccount:default:some-other-sa"))

	origProxy := k8sReverseProxy
	origClient := k8sAuthClient
	k8sReverseProxy = makeTestProxy(backend)
	k8sAuthClient = fakeClient
	defer func() {
		k8sReverseProxy = origProxy
		k8sAuthClient = origClient
	}()

	req := httptest.NewRequest(http.MethodGet, "/vpw/v1/k8s/api/v1/namespaces/migration-system/pods", nil)
	req.Header.Set("Authorization", "Bearer sometoken")
	w := httptest.NewRecorder()
	HandleK8sProxy(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestHandleK8sProxy_ValidToken_ProxiesToBackend(t *testing.T) {
	clearTokenCache()
	const backendStatus = http.StatusOK
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify Authorization header was stripped before reaching backend.
		if r.Header.Get("Authorization") != "" {
			t.Error("Authorization header should have been stripped by proxy Director")
		}
		w.WriteHeader(backendStatus)
	}))
	defer backend.Close()

	fakeClient := fake.NewSimpleClientset()
	fakeClient.PrependReactor("create", "tokenreviews",
		makeTokenReviewReactor(true, allowedServiceAccount))

	origProxy := k8sReverseProxy
	origClient := k8sAuthClient
	k8sReverseProxy = makeTestProxy(backend)
	k8sAuthClient = fakeClient
	defer func() {
		k8sReverseProxy = origProxy
		k8sAuthClient = origClient
	}()

	req := httptest.NewRequest(http.MethodGet, "/vpw/v1/k8s/api/v1/namespaces/migration-system/pods", nil)
	req.Header.Set("Authorization", "Bearer valid-ui-manager-sa-token")
	w := httptest.NewRecorder()
	HandleK8sProxy(w, req)

	if w.Code != backendStatus {
		t.Errorf("expected %d from backend, got %d", backendStatus, w.Code)
	}
}

// --- Path allowlist tests ---

func TestHandleK8sProxy_AllowedPaths(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
	}{
		// pods — allowed
		{"list pods", http.MethodGet, "/vpw/v1/k8s/api/v1/namespaces/migration-system/pods", http.StatusOK},
		{"get pod", http.MethodGet, "/vpw/v1/k8s/api/v1/namespaces/migration-system/pods/my-pod", http.StatusOK},
		{"stream pod logs", http.MethodGet, "/vpw/v1/k8s/api/v1/namespaces/migration-system/pods/my-pod/log", http.StatusOK},
		{"patch pod (admin cutover)", http.MethodPatch, "/vpw/v1/k8s/api/v1/namespaces/migration-system/pods/my-pod", http.StatusOK},
		// secrets — allowed CRUD
		{"list secrets", http.MethodGet, "/vpw/v1/k8s/api/v1/namespaces/migration-system/secrets", http.StatusOK},
		{"get secret", http.MethodGet, "/vpw/v1/k8s/api/v1/namespaces/migration-system/secrets/my-secret", http.StatusOK},
		{"create secret", http.MethodPost, "/vpw/v1/k8s/api/v1/namespaces/migration-system/secrets", http.StatusOK},
		{"replace secret", http.MethodPut, "/vpw/v1/k8s/api/v1/namespaces/migration-system/secrets/my-secret", http.StatusOK},
		{"delete secret", http.MethodDelete, "/vpw/v1/k8s/api/v1/namespaces/migration-system/secrets/my-secret", http.StatusOK},
		// migrationblueprints — allowed CRUD
		{"list migrationblueprints", http.MethodGet, "/vpw/v1/k8s/apis/vjailbreak.k8s.pf9.io/v1alpha1/namespaces/migration-system/migrationblueprints", http.StatusOK},
		{"get migrationblueprint", http.MethodGet, "/vpw/v1/k8s/apis/vjailbreak.k8s.pf9.io/v1alpha1/namespaces/migration-system/migrationblueprints/my-bp", http.StatusOK},
		{"create migrationblueprint", http.MethodPost, "/vpw/v1/k8s/apis/vjailbreak.k8s.pf9.io/v1alpha1/namespaces/migration-system/migrationblueprints", http.StatusOK},
		{"replace migrationblueprint", http.MethodPut, "/vpw/v1/k8s/apis/vjailbreak.k8s.pf9.io/v1alpha1/namespaces/migration-system/migrationblueprints/my-bp", http.StatusOK},
		{"patch migrationblueprint", http.MethodPatch, "/vpw/v1/k8s/apis/vjailbreak.k8s.pf9.io/v1alpha1/namespaces/migration-system/migrationblueprints/my-bp", http.StatusOK},
		{"delete migrationblueprint", http.MethodDelete, "/vpw/v1/k8s/apis/vjailbreak.k8s.pf9.io/v1alpha1/namespaces/migration-system/migrationblueprints/my-bp", http.StatusOK},
		// blocked — wrong namespace
		{"secrets kube-system forbidden", http.MethodGet, "/vpw/v1/k8s/api/v1/namespaces/kube-system/secrets", http.StatusForbidden},
		{"migrationblueprints default ns forbidden", http.MethodGet, "/vpw/v1/k8s/apis/vjailbreak.k8s.pf9.io/v1alpha1/namespaces/default/migrationblueprints", http.StatusForbidden},
		// blocked — other CRDs not routed through proxy
		{"migrationplans via proxy forbidden", http.MethodGet, "/vpw/v1/k8s/apis/vjailbreak.k8s.pf9.io/v1alpha1/namespaces/migration-system/migrationplans", http.StatusForbidden},
		{"pods default ns forbidden", http.MethodGet, "/vpw/v1/k8s/api/v1/namespaces/default/pods", http.StatusForbidden},
		// blocked — disallowed methods on pods
		{"post pods forbidden", http.MethodPost, "/vpw/v1/k8s/api/v1/namespaces/migration-system/pods", http.StatusForbidden},
		{"delete pod forbidden", http.MethodDelete, "/vpw/v1/k8s/api/v1/namespaces/migration-system/pods/my-pod", http.StatusForbidden},
		// blocked — entirely different resource
		{"clusterroles forbidden", http.MethodGet, "/vpw/v1/k8s/apis/rbac.authorization.k8s.io/v1/clusterroles", http.StatusForbidden},
		// blocked — path traversal
		{"path traversal forbidden", http.MethodGet, "/vpw/v1/k8s/api/v1/namespaces/migration-system/pods/../secrets", http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearTokenCache()
			backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))
			defer backend.Close()

			fakeClient := fake.NewSimpleClientset()
			fakeClient.PrependReactor("create", "tokenreviews",
				makeTokenReviewReactor(true, allowedServiceAccount))

			origProxy := k8sReverseProxy
			origClient := k8sAuthClient
			k8sReverseProxy = makeTestProxy(backend)
			k8sAuthClient = fakeClient
			defer func() {
				k8sReverseProxy = origProxy
				k8sAuthClient = origClient
			}()

			req := httptest.NewRequest(tt.method, tt.path, nil)
			req.Header.Set("Authorization", "Bearer valid-token")
			w := httptest.NewRecorder()
			HandleK8sProxy(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("path %s method %s: expected %d, got %d", tt.path, tt.method, tt.wantStatus, w.Code)
			}
		})
	}
}

// --- validateUIToken tests ---

func TestValidateUIToken_ClientNotInitialized(t *testing.T) {
	clearTokenCache()
	orig := k8sAuthClient
	k8sAuthClient = nil
	defer func() { k8sAuthClient = orig }()

	err := validateUIToken("anytoken")
	if err == nil {
		t.Error("expected error when k8s client is nil")
	}
}

func TestValidateUIToken_NotAuthenticated(t *testing.T) {
	clearTokenCache()
	fakeClient := fake.NewSimpleClientset()
	fakeClient.PrependReactor("create", "tokenreviews",
		makeTokenReviewReactor(false, ""))

	orig := k8sAuthClient
	k8sAuthClient = fakeClient
	defer func() { k8sAuthClient = orig }()

	err := validateUIToken("badtoken")
	if err == nil {
		t.Error("expected error for unauthenticated token")
	}
}

func TestValidateUIToken_WrongServiceAccount(t *testing.T) {
	clearTokenCache()
	fakeClient := fake.NewSimpleClientset()
	fakeClient.PrependReactor("create", "tokenreviews",
		makeTokenReviewReactor(true, "system:serviceaccount:kube-system:default"))

	orig := k8sAuthClient
	k8sAuthClient = fakeClient
	defer func() { k8sAuthClient = orig }()

	err := validateUIToken("wrongsatoken")
	if err == nil {
		t.Error("expected error for wrong service account")
	}
	if err != nil && !strings.Contains(err.Error(), "unauthorized service account") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestValidateUIToken_ValidToken(t *testing.T) {
	clearTokenCache()
	fakeClient := fake.NewSimpleClientset()
	fakeClient.PrependReactor("create", "tokenreviews",
		makeTokenReviewReactor(true, allowedServiceAccount))

	orig := k8sAuthClient
	k8sAuthClient = fakeClient
	defer func() { k8sAuthClient = orig }()

	err := validateUIToken("valid-token")
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestValidateUIToken_CacheHit_SkipsTokenReview(t *testing.T) {
	clearTokenCache()
	callCount := 0
	fakeClient := fake.NewSimpleClientset()
	fakeClient.PrependReactor("create", "tokenreviews", func(action k8stesting.Action) (bool, runtime.Object, error) {
		callCount++
		return true, &authv1.TokenReview{
			Status: authv1.TokenReviewStatus{
				Authenticated: true,
				User:          authv1.UserInfo{Username: allowedServiceAccount},
			},
		}, nil
	})

	orig := k8sAuthClient
	k8sAuthClient = fakeClient
	defer func() { k8sAuthClient = orig }()

	const token = "cached-token"
	if err := validateUIToken(token); err != nil {
		t.Fatalf("first call failed: %v", err)
	}
	if err := validateUIToken(token); err != nil {
		t.Fatalf("second call failed: %v", err)
	}

	if callCount != 1 {
		t.Errorf("TokenReview called %d times, expected 1 (cache should prevent second call)", callCount)
	}
}

func TestValidateUIToken_ExpiredCache_RevalidatesToken(t *testing.T) {
	clearTokenCache()
	// Pre-populate cache with an already-expired entry.
	const token = "expired-token"
	tokenCache.Store(token, time.Now().Add(-2*tokenCacheTTL))

	callCount := 0
	fakeClient := fake.NewSimpleClientset()
	fakeClient.PrependReactor("create", "tokenreviews", func(action k8stesting.Action) (bool, runtime.Object, error) {
		callCount++
		return true, &authv1.TokenReview{
			Status: authv1.TokenReviewStatus{
				Authenticated: true,
				User:          authv1.UserInfo{Username: allowedServiceAccount},
			},
		}, nil
	})

	orig := k8sAuthClient
	k8sAuthClient = fakeClient
	defer func() { k8sAuthClient = orig }()

	if err := validateUIToken(token); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if callCount != 1 {
		t.Errorf("TokenReview called %d times, expected 1 (should re-validate after expiry)", callCount)
	}
}
