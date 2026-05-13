package server

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/sirupsen/logrus"
	"k8s.io/client-go/rest"
)

var k8sReverseProxy *httputil.ReverseProxy

// InitK8sProxy builds a reverse proxy that forwards /vpw/v1/k8s/* requests to the
// Kubernetes API server using the pod's ServiceAccount credentials.
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

	k8sReverseProxy = &httputil.ReverseProxy{
		FlushInterval: -1,
		Transport:     transport,
		Director: func(req *http.Request) {
			// Strip /vpw/v1/k8s prefix so /vpw/v1/k8s/api/v1/... → /api/v1/...
			req.URL.Path = strings.TrimPrefix(req.URL.Path, "/vpw/v1/k8s")
			if req.URL.RawPath != "" {
				req.URL.RawPath = strings.TrimPrefix(req.URL.RawPath, "/vpw/v1/k8s")
			}
			req.URL.Host = target.Host
			req.URL.Scheme = target.Scheme
			req.Header.Del("Authorization")
		},
	}
	return nil
}

// HandleK8sProxy forwards the request to the Kubernetes API server via the
// reverse proxy initialised by InitK8sProxy.
func HandleK8sProxy(w http.ResponseWriter, r *http.Request) {
	if k8sReverseProxy == nil {
		logrus.Warn("k8s proxy not initialised — is the server running in-cluster?")
		http.Error(w, "k8s proxy unavailable", http.StatusServiceUnavailable)
		return
	}
	k8sReverseProxy.ServeHTTP(w, r)
}
