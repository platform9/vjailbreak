package server

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	authv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	allowedServiceAccount    = "system:serviceaccount:migration-system:ui-manager-sa"
	tokenCacheTTL            = 60 * time.Second
	migrationSystemNamespace = "migration-system"
)

// allowedRoute pairs an HTTP method with a path pattern.
// All patterns are anchored to migration-system namespace.
type allowedRoute struct {
	method  string
	pattern *regexp.Regexp
}

// allowedRoutes is the exact set of K8s API calls the UI makes through this proxy.
var allowedRoutes = []allowedRoute{
	// Pods — read and admin-cutover patch
	{http.MethodGet, regexp.MustCompile(`^/api/v1/namespaces/migration-system/pods(/[^/?]+(/log)?)?$`)},
	{http.MethodPatch, regexp.MustCompile(`^/api/v1/namespaces/migration-system/pods/[^/?]+$`)},
	// Secrets — full CRUD, migration-system only
	{http.MethodGet, regexp.MustCompile(`^/api/v1/namespaces/migration-system/secrets(/[^/?]+)?$`)},
	{http.MethodPost, regexp.MustCompile(`^/api/v1/namespaces/migration-system/secrets$`)},
	{http.MethodPut, regexp.MustCompile(`^/api/v1/namespaces/migration-system/secrets/[^/?]+$`)},
	{http.MethodDelete, regexp.MustCompile(`^/api/v1/namespaces/migration-system/secrets/[^/?]+$`)},
	// MigrationBlueprints — full CRUD via proxy
	{http.MethodGet, regexp.MustCompile(`^/apis/vjailbreak\.k8s\.pf9\.io/v1alpha1/namespaces/migration-system/migrationblueprints(/[^/?]+)?$`)},
	{http.MethodPost, regexp.MustCompile(`^/apis/vjailbreak\.k8s\.pf9\.io/v1alpha1/namespaces/migration-system/migrationblueprints$`)},
	{http.MethodPut, regexp.MustCompile(`^/apis/vjailbreak\.k8s\.pf9\.io/v1alpha1/namespaces/migration-system/migrationblueprints/[^/?]+$`)},
	{http.MethodPatch, regexp.MustCompile(`^/apis/vjailbreak\.k8s\.pf9\.io/v1alpha1/namespaces/migration-system/migrationblueprints/[^/?]+$`)},
	{http.MethodDelete, regexp.MustCompile(`^/apis/vjailbreak\.k8s\.pf9\.io/v1alpha1/namespaces/migration-system/migrationblueprints/[^/?]+$`)},
}

func isAllowedRoute(method, k8sPath string) bool {
	for _, r := range allowedRoutes {
		if r.method == method && r.pattern.MatchString(k8sPath) {
			return true
		}
	}
	return false
}

var (
	k8sReverseProxy *httputil.ReverseProxy
	k8sAuthClient   kubernetes.Interface
	tokenCache      sync.Map // map[string]time.Time — token → cache expiry
)

// InitK8sProxy builds a reverse proxy that forwards /vpw/v1/k8s/* to the K8s
// API server using the pod's ServiceAccount credentials, and initialises a
// Kubernetes client for incoming Bearer token validation.
func InitK8sProxy() error {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return err
	}

	target, err := url.Parse(cfg.Host)
	if err != nil {
		return err
	}

	transport, err := rest.TransportFor(cfg)
	if err != nil {
		return err
	}

	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return err
	}
	k8sAuthClient = client

	k8sReverseProxy = &httputil.ReverseProxy{
		FlushInterval: -1,
		Transport:     transport,
		Director: func(req *http.Request) {
			// Strip /vpw/v1/k8s prefix: /vpw/v1/k8s/api/v1/... → /api/v1/...
			req.URL.Path = strings.TrimPrefix(req.URL.Path, "/vpw/v1/k8s")
			if req.URL.RawPath != "" {
				req.URL.RawPath = strings.TrimPrefix(req.URL.RawPath, "/vpw/v1/k8s")
			}
			req.URL.Host = target.Host
			req.URL.Scheme = target.Scheme
			// Replace browser's Authorization with the pod's SA token.
			req.Header.Del("Authorization")
		},
	}
	return nil
}

// validateUIToken calls the K8s TokenReview API to verify the token is
// authentic and belongs to the ui-manager-sa service account.
// Successful validations are cached for tokenCacheTTL to avoid repeated calls.
func validateUIToken(token string) error {
	if exp, ok := tokenCache.Load(token); ok {
		if time.Now().Before(exp.(time.Time)) {
			return nil
		}
		tokenCache.Delete(token)
	}

	if k8sAuthClient == nil {
		return fmt.Errorf("k8s client not initialized")
	}

	result, err := k8sAuthClient.AuthenticationV1().TokenReviews().Create(
		context.Background(),
		&authv1.TokenReview{Spec: authv1.TokenReviewSpec{Token: token}},
		metav1.CreateOptions{},
	)
	if err != nil {
		return fmt.Errorf("token review request failed: %w", err)
	}
	if !result.Status.Authenticated {
		return fmt.Errorf("token not authenticated")
	}
	if result.Status.User.Username != allowedServiceAccount {
		return fmt.Errorf("unauthorized service account: %s", result.Status.User.Username)
	}

	tokenCache.Store(token, time.Now().Add(tokenCacheTTL))
	return nil
}

// HandleK8sProxy validates the caller's Bearer token (must be ui-manager-sa),
// then forwards the request to the K8s API using the pod's own SA credentials.
func HandleK8sProxy(w http.ResponseWriter, r *http.Request) {
	if k8sReverseProxy == nil {
		logrus.Warn("k8s proxy not initialised — is the server running in-cluster?")
		http.Error(w, "k8s proxy unavailable", http.StatusServiceUnavailable)
		return
	}

	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	token := strings.TrimPrefix(authHeader, "Bearer ")
	if err := validateUIToken(token); err != nil {
		logrus.Warnf("k8s proxy: rejected request: %v", err)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	k8sPath := strings.TrimPrefix(r.URL.Path, "/vpw/v1/k8s")
	if !isAllowedRoute(r.Method, k8sPath) {
		logrus.Warnf("k8s proxy: forbidden %s %s", r.Method, k8sPath)
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	k8sReverseProxy.ServeHTTP(w, r)
}
