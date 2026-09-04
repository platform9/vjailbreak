package utils

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

func TestNewVjbNet_Defaults(t *testing.T) {
	n := NewVjbNet()

	if n == nil {
		t.Fatalf("NewVjbNet returned nil")
	}

	if n.timeout != defaultTimeout {
		t.Errorf("expected default timeout %v, got %v", defaultTimeout, n.timeout)
	}

	if n.Client == nil {
		t.Errorf("expected non-nil http.Client")
	}

	if !n.Insecure {
		t.Errorf("expected Insecure to be true by default")
	}

	if n.HTTPProxy != "" || n.HTTPSProxy != "" || n.NoProxy != "" {
		t.Errorf("expected proxy fields to be empty by default")
	}

	if !n.UseProxyFromEnv {
		t.Errorf("expected UseProxyFromEnv to be true by default")
	}

	if n.proxyCfg == nil {
		t.Errorf("expected proxyCfg to be initialized from environment")
	}
}

func TestVjbNet_SettersAndGetters(t *testing.T) {
	n := NewVjbNet()

	// timeout
	customTimeout := 10 * time.Second
	n.SetTimeout(customTimeout)
	if got := n.GetTimeout(); got != customTimeout {
		t.Errorf("GetTimeout = %v, want %v", got, customTimeout)
	}

	// insecure
	n.SetInsecure(false)
	if n.Insecure {
		t.Errorf("expected Insecure to be false after SetInsecure(false)")
	}

	// http proxy
	httpProxy := "http-proxy.example:8080"
	n.SetHTTPProxy(httpProxy)
	if n.HTTPProxy != httpProxy {
		t.Errorf("HTTPProxy = %q, want %q", n.HTTPProxy, httpProxy)
	}

	// https proxy
	httpsProxy := "https-proxy.example:8443"
	n.SetHTTPSProxy(httpsProxy)
	if n.HTTPSProxy != httpsProxy {
		t.Errorf("HTTPSProxy = %q, want %q", n.HTTPSProxy, httpsProxy)
	}

	// no proxy
	noProxy := "no-proxy.example"
	n.SetNoProxy(noProxy)
	if n.NoProxy != noProxy {
		t.Errorf("NoProxy = %q, want %q", n.NoProxy, noProxy)
	}

	// use proxy from env
	n.SetUseProxyFromEnv(false)
	if n.UseProxyFromEnv {
		t.Errorf("expected UseProxyFromEnv to be false after SetUseProxyFromEnv(false)")
	}
}

func TestVjbNet_IsProxyEnabled(t *testing.T) {
	n := NewVjbNet()

	if n.IsProxyEnabled() {
		t.Errorf("expected IsProxyEnabled to be false when no proxies are set")
	}

	n.SetHTTPProxy("http-proxy.example:8080")
	if !n.IsProxyEnabled() {
		t.Errorf("expected IsProxyEnabled to be true when HTTP proxy is set")
	}

	n.SetHTTPProxy("")
	n.SetHTTPSProxy("https-proxy.example:8443")
	if !n.IsProxyEnabled() {
		t.Errorf("expected IsProxyEnabled to be true when HTTPS proxy is set")
	}
}

func TestVjbNet_CreateHTTPClient(t *testing.T) {
	n := NewVjbNet()
	n.SetTimeout(5 * time.Second)
	n.SetInsecure(false)

	if err := n.CreateHTTPClient(); err != nil {
		t.Fatalf("CreateHTTPClient returned error: %v", err)
	}

	client := n.GetClient()
	if client == nil {
		t.Fatalf("expected non-nil http.Client after CreateHTTPClient")
	}

	if client.Timeout != n.timeout {
		t.Errorf("client.Timeout = %v, want %v", client.Timeout, n.timeout)
	}

	// Ensure Transport is set
	if client.Transport == nil {
		t.Errorf("expected non-nil Transport on http.Client")
	}
}

func TestVjbNet_CreateSecureHTTPClient(t *testing.T) {
	n := NewVjbNet()
	n.SetTimeout(7 * time.Second)
	n.SetInsecure(false)

	if err := n.CreateSecureHTTPClient(); err != nil {
		t.Fatalf("CreateSecureHTTPClient returned error: %v", err)
	}

	client := n.GetClient()
	if client == nil {
		t.Fatalf("expected non-nil http.Client after CreateSecureHTTPClient")
	}

	if client.Timeout != n.timeout {
		t.Errorf("client.Timeout = %v, want %v", client.Timeout, n.timeout)
	}

	// Ensure Transport is set
	if client.Transport == nil {
		t.Errorf("expected non-nil Transport on http.Client")
	}
}

func TestVjbNet_proxy4URL_HTTPAndHTTPS(t *testing.T) {
	n := NewVjbNet()

	httpProxy := "http-proxy.example:8080"
	httpsProxy := "https-proxy.example:8443"
	n.SetHTTPProxy(httpProxy)
	n.SetHTTPSProxy(httpsProxy)

	// Force proxyCfg to be initialized and overridden by setters via proxy4URL
	uHTTP, err := url.Parse("http://some-http-host")
	if err != nil {
		t.Fatalf("failed to parse HTTP URL: %v", err)
	}
	proxyHTTP, err := n.proxy4URL(uHTTP)
	if err != nil {
		t.Fatalf("proxy4URL(http) returned error: %v", err)
	}
	if proxyHTTP == nil {
		t.Fatalf("expected non-nil proxy for HTTP URL")
	}
	if proxyHTTP.Scheme != "http" || proxyHTTP.Host != httpProxy {
		t.Errorf("HTTP proxy = %s://%s, want http://%s", proxyHTTP.Scheme, proxyHTTP.Host, httpProxy)
	}

	uHTTPS, err := url.Parse("https://some-https-host")
	if err != nil {
		t.Fatalf("failed to parse HTTPS URL: %v", err)
	}
	proxyHTTPS, err := n.proxy4URL(uHTTPS)
	if err != nil {
		t.Fatalf("proxy4URL(https) returned error: %v", err)
	}
	if proxyHTTPS == nil {
		t.Fatalf("expected non-nil proxy for HTTPS URL")
	}
	if proxyHTTPS.Scheme != "https" || proxyHTTPS.Host != httpsProxy {
		t.Errorf("HTTPS proxy = %s://%s, want https://%s", proxyHTTPS.Scheme, proxyHTTPS.Host, httpsProxy)
	}
}

func TestVjbNet_proxy4URL_NoProxyConfigPropagation(t *testing.T) {
	n := NewVjbNet()

	httpProxy := "http-proxy.example:8080"
	httpsProxy := "https-proxy.example:8443"
	noProxy := "no-proxy.example"

	n.SetHTTPProxy(httpProxy)
	n.SetHTTPSProxy(httpsProxy)
	n.SetNoProxy(noProxy)

	// Call proxy4URL once to ensure proxyCfg is initialized and updated
	u, err := url.Parse("http://some-http-host")
	if err != nil {
		t.Fatalf("failed to parse URL: %v", err)
	}
	if _, err := n.proxy4URL(u); err != nil {
		t.Fatalf("proxy4URL returned error: %v", err)
	}

	if n.proxyCfg == nil {
		t.Fatalf("expected proxyCfg to be initialized")
	}
	if n.proxyCfg.HTTPProxy != httpProxy {
		t.Errorf("proxyCfg.HTTPProxy = %q, want %q", n.proxyCfg.HTTPProxy, httpProxy)
	}
	if n.proxyCfg.HTTPSProxy != httpsProxy {
		t.Errorf("proxyCfg.HTTPSProxy = %q, want %q", n.proxyCfg.HTTPSProxy, httpsProxy)
	}
	if n.proxyCfg.NoProxy != noProxy {
		t.Errorf("proxyCfg.NoProxy = %q, want %q", n.proxyCfg.NoProxy, noProxy)
	}
}

func TestVjbNet_NoProxyBypassesProxy(t *testing.T) {
	n := NewVjbNet()

	httpProxy := "http-proxy.example:8080"
	noProxyHost := "no-proxy.example:80"

	// Configure HTTP proxy and NoProxy host
	n.SetHTTPProxy(httpProxy)
	n.SetNoProxy("no-proxy.example:80")

	// Disable UseProxyFromEnv so our custom Proxy function is preserved
	n.SetUseProxyFromEnv(false)

	// Warm up proxyCfg so that NoProxy is propagated into it
	warmURL, err := url.Parse("http://warmup-host")
	if err != nil {
		t.Fatalf("failed to parse warmup URL: %v", err)
	}
	if _, err := n.proxy4URL(warmURL); err != nil {
		t.Fatalf("proxy4URL warmup returned error: %v", err)
	}

	// Build client and transport with these settings
	if err := n.CreateHTTPClient(); err != nil {
		t.Fatalf("CreateHTTPClient returned error: %v", err)
	}

	client := n.GetClient()
	if client == nil {
		t.Fatalf("expected non-nil client")
	}

	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("client.Transport is not *http.Transport")
	}
	if transport.Proxy == nil {
		t.Fatalf("expected non-nil Proxy function when HTTPProxy and NoProxy are set")
	}

	// Request to a host that matches NoProxy should bypass proxy (return nil, nil)
	reqURL, err := url.Parse("http://" + noProxyHost)
	if err != nil {
		t.Fatalf("failed to parse URL: %v", err)
	}
	proxyURL, err := transport.Proxy(&http.Request{URL: reqURL})
	if err != nil {
		t.Fatalf("Proxy callback returned error: %v", err)
	}
	if proxyURL != nil {
		t.Errorf("expected nil proxy for NoProxy host, got %v", proxyURL)
	}

	// Request to a different host should use the configured proxy
	otherURL, err := url.Parse("http://other-host.example")
	if err != nil {
		t.Fatalf("failed to parse other URL: %v", err)
	}
	proxyURL, err = transport.Proxy(&http.Request{URL: otherURL})
	if err != nil {
		t.Fatalf("Proxy callback for other host returned error: %v", err)
	}
	if proxyURL == nil {
		t.Fatalf("expected non-nil proxy for non-NoProxy host")
	}
	if proxyURL.Scheme != "http" || proxyURL.Host != httpProxy {
		t.Errorf("proxy for other host = %s://%s, want http://%s", proxyURL.Scheme, proxyURL.Host, httpProxy)
	}
}

func TestVjbNet_NoProxy_CommaSeparatedAndCIDR(t *testing.T) {
	n := NewVjbNet()

	httpProxy := "http-proxy.example:8080"
	noProxy := "example.com,10.0.0.0/8"

	// Configure HTTP proxy and comma-separated NO_PROXY including domain and CIDR
	n.SetHTTPProxy(httpProxy)
	n.SetNoProxy(noProxy)
	n.SetUseProxyFromEnv(false)

	if err := n.CreateHTTPClient(); err != nil {
		t.Fatalf("CreateHTTPClient returned error: %v", err)
	}
	client := n.GetClient()
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("client.Transport is not *http.Transport")
	}
	if transport.Proxy == nil {
		t.Fatalf("expected non-nil Proxy function when HTTPProxy and NoProxy are set")
	}

	tests := []struct {
		name        string
		rawURL      string
		wantNoProxy bool
	}{
		{"no_proxy_exact_domain", "http://example.com", true},
		{"no_proxy_subdomain", "http://sub.example.com", true},
		{"no_proxy_cidr_match", "http://10.1.2.3", true},
		{"use_proxy_other_domain", "http://other.com", false},
		{"use_proxy_other_ip", "http://192.168.1.1", false},
	}

	for _, ttcase := range tests {
		t.Run(ttcase.name, func(t *testing.T) {
			u, err := url.Parse(ttcase.rawURL)
			if err != nil {
				t.Fatalf("failed to parse URL %q: %v", ttcase.rawURL, err)
			}
			proxyURL, err := transport.Proxy(&http.Request{URL: u})
			if err != nil {
				t.Fatalf("Proxy callback returned error: %v", err)
			}
			if ttcase.wantNoProxy {
				if proxyURL != nil {
					t.Fatalf("expected nil proxy for %s, got %v", ttcase.rawURL, proxyURL)
				}
			} else {
				if proxyURL == nil {
					t.Fatalf("expected non-nil proxy for %s", ttcase.rawURL)
				}
				if proxyURL.Scheme != "http" || proxyURL.Host != httpProxy {
					t.Fatalf("proxy for %s = %s://%s, want http://%s", ttcase.rawURL, proxyURL.Scheme, proxyURL.Host, httpProxy)
				}
			}
		})
	}
}

func TestVjbNet_GetClient_Default(t *testing.T) {
	n := NewVjbNet()

	client := n.GetClient()
	if client == nil {
		t.Fatalf("expected non-nil client from GetClient on new VjbNet")
	}

	if _, ok := interface{}(client).(*http.Client); !ok {
		t.Errorf("GetClient did not return *http.Client")
	}
}

func TestVjbNet_SetProxyCredentials(t *testing.T) {
	n := NewVjbNet()

	n.SetHTTPProxyCredentials("http-user", "http-pass")
	if n.HTTPProxyUsername != "http-user" || n.HTTPProxyPassword != "http-pass" {
		t.Errorf("SetHTTPProxyCredentials did not set fields, got user=%q pass=%q", n.HTTPProxyUsername, n.HTTPProxyPassword)
	}

	n.SetHTTPSProxyCredentials("https-user", "https-pass")
	if n.HTTPSProxyUsername != "https-user" || n.HTTPSProxyPassword != "https-pass" {
		t.Errorf("SetHTTPSProxyCredentials did not set fields, got user=%q pass=%q", n.HTTPSProxyUsername, n.HTTPSProxyPassword)
	}

	// HTTP and HTTPS credentials must stay fully independent - setting one
	// must never leak into the other.
	if n.HTTPProxyUsername == n.HTTPSProxyUsername {
		t.Errorf("expected independent HTTP/HTTPS proxy usernames, both are %q", n.HTTPProxyUsername)
	}
}

func TestNewVjbNet_CredentialsAutoReadFromEnv(t *testing.T) {
	t.Setenv("HTTP_PROXY_USERNAME", "env-http-user")
	t.Setenv("HTTP_PROXY_PASSWORD", "env-http-pass")
	t.Setenv("HTTPS_PROXY_USERNAME", "env-https-user")
	t.Setenv("HTTPS_PROXY_PASSWORD", "env-https-pass")

	n := NewVjbNet()

	if n.HTTPProxyUsername != "env-http-user" || n.HTTPProxyPassword != "env-http-pass" {
		t.Errorf("expected HTTP proxy credentials from env, got user=%q pass=%q", n.HTTPProxyUsername, n.HTTPProxyPassword)
	}
	if n.HTTPSProxyUsername != "env-https-user" || n.HTTPSProxyPassword != "env-https-pass" {
		t.Errorf("expected HTTPS proxy credentials from env, got user=%q pass=%q", n.HTTPSProxyUsername, n.HTTPSProxyPassword)
	}
}

func TestNewVjbNet_CredentialsEmptyByDefault(t *testing.T) {
	n := NewVjbNet()

	if n.HTTPProxyUsername != "" || n.HTTPProxyPassword != "" {
		t.Errorf("expected empty HTTP proxy credentials by default, got user=%q pass=%q", n.HTTPProxyUsername, n.HTTPProxyPassword)
	}
	if n.HTTPSProxyUsername != "" || n.HTTPSProxyPassword != "" {
		t.Errorf("expected empty HTTPS proxy credentials by default, got user=%q pass=%q", n.HTTPSProxyUsername, n.HTTPSProxyPassword)
	}
}

func TestWithProxyCredentials(t *testing.T) {
	tests := []struct {
		name     string
		rawURL   string
		username string
		password string
		want     string
	}{
		{"empty raw URL returns unchanged", "", "user", "pass", ""},
		{"empty username returns unchanged", "proxy.example:8080", "", "pass", "proxy.example:8080"},
		{"scheme-qualified URL gets userinfo", "http://proxy.example:8080", "user", "pass", "http://user:pass@proxy.example:8080"},
		{"bare host:port gets userinfo via http:// fallback", "proxy.example:8080", "user", "pass", "http://user:pass@proxy.example:8080"},
		{"special characters in password are percent-encoded", "http://proxy.example:8080", "user", "p@ss:w/rd", "http://user:p%40ss%3Aw%2Frd@proxy.example:8080"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := withProxyCredentials(tt.rawURL, tt.username, tt.password)
			if got != tt.want {
				t.Errorf("withProxyCredentials(%q, %q, %q) = %q, want %q", tt.rawURL, tt.username, tt.password, got, tt.want)
			}
		})
	}
}

func TestVjbNet_proxy4URL_CredentialsEmbeddedIndependently(t *testing.T) {
	n := NewVjbNet()

	httpProxy := "http-proxy.example:8080"
	httpsProxy := "https-proxy.example:8443"
	n.SetHTTPProxy(httpProxy)
	n.SetHTTPSProxy(httpsProxy)
	n.SetHTTPProxyCredentials("http-user", "http-pass")
	n.SetHTTPSProxyCredentials("https-user", "https-pass")

	uHTTP, err := url.Parse("http://some-http-host")
	if err != nil {
		t.Fatalf("failed to parse HTTP URL: %v", err)
	}
	proxyHTTP, err := n.proxy4URL(uHTTP)
	if err != nil {
		t.Fatalf("proxy4URL(http) returned error: %v", err)
	}
	if proxyHTTP == nil || proxyHTTP.User == nil {
		t.Fatalf("expected HTTP proxy URL with userinfo, got %v", proxyHTTP)
	}
	if gotUser := proxyHTTP.User.Username(); gotUser != "http-user" {
		t.Errorf("HTTP proxy username = %q, want %q", gotUser, "http-user")
	}
	if gotPass, _ := proxyHTTP.User.Password(); gotPass != "http-pass" {
		t.Errorf("HTTP proxy password = %q, want %q", gotPass, "http-pass")
	}
	if proxyHTTP.Host != httpProxy {
		t.Errorf("HTTP proxy host = %q, want %q", proxyHTTP.Host, httpProxy)
	}

	uHTTPS, err := url.Parse("https://some-https-host")
	if err != nil {
		t.Fatalf("failed to parse HTTPS URL: %v", err)
	}
	proxyHTTPS, err := n.proxy4URL(uHTTPS)
	if err != nil {
		t.Fatalf("proxy4URL(https) returned error: %v", err)
	}
	if proxyHTTPS == nil || proxyHTTPS.User == nil {
		t.Fatalf("expected HTTPS proxy URL with userinfo, got %v", proxyHTTPS)
	}
	if gotUser := proxyHTTPS.User.Username(); gotUser != "https-user" {
		t.Errorf("HTTPS proxy username = %q, want %q", gotUser, "https-user")
	}
	if gotPass, _ := proxyHTTPS.User.Password(); gotPass != "https-pass" {
		t.Errorf("HTTPS proxy password = %q, want %q", gotPass, "https-pass")
	}

	// Credentials must never cross over between HTTP and HTTPS.
	if proxyHTTP.User.Username() == proxyHTTPS.User.Username() {
		t.Errorf("expected independent HTTP/HTTPS proxy usernames on resolved URLs")
	}
}

func TestVjbNet_proxy4URL_NoCredentials_UserInfoAbsent(t *testing.T) {
	n := NewVjbNet()

	httpProxy := "http-proxy.example:8080"
	n.SetHTTPProxy(httpProxy)
	// Deliberately not setting any credentials.

	u, err := url.Parse("http://some-http-host")
	if err != nil {
		t.Fatalf("failed to parse URL: %v", err)
	}
	proxyURL, err := n.proxy4URL(u)
	if err != nil {
		t.Fatalf("proxy4URL returned error: %v", err)
	}
	if proxyURL == nil {
		t.Fatalf("expected non-nil proxy URL")
	}
	if proxyURL.User != nil {
		t.Errorf("expected no userinfo on resolved proxy URL when no credentials are set, got %v", proxyURL.User)
	}
}

// TestVjbNet_ProxyCredentials_NeverLoggedInPlaintext guards the core safety
// property behind embedding credentials directly in VjbNet: the debug log
// lines in getNetTransport/proxy4URL print v.proxyCfg/v.HTTPProxy/
// v.HTTPSProxy, which must stay credential-free even once HTTP(S) proxy
// credentials are configured and actively used to resolve a proxy decision.
func TestVjbNet_ProxyCredentials_NeverLoggedInPlaintext(t *testing.T) {
	n := NewVjbNet()

	n.SetHTTPProxy("http-proxy.example:8080")
	n.SetHTTPSProxy("https-proxy.example:8443")
	const secretPassword = "super-secret-password"
	n.SetHTTPProxyCredentials("http-user", secretPassword)
	n.SetHTTPSProxyCredentials("https-user", secretPassword)
	n.SetUseProxyFromEnv(false)

	if err := n.CreateHTTPClient(); err != nil {
		t.Fatalf("CreateHTTPClient returned error: %v", err)
	}
	client := n.GetClient()
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("client.Transport is not *http.Transport")
	}

	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	os.Stdout = w

	httpURL, _ := url.Parse("http://some-http-host")
	if _, err := transport.Proxy(&http.Request{URL: httpURL}); err != nil {
		os.Stdout = origStdout
		t.Fatalf("Proxy(http) returned error: %v", err)
	}
	httpsURL, _ := url.Parse("https://some-https-host")
	if _, err := transport.Proxy(&http.Request{URL: httpsURL}); err != nil {
		os.Stdout = origStdout
		t.Fatalf("Proxy(https) returned error: %v", err)
	}

	w.Close()
	os.Stdout = origStdout

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("failed to read captured stdout: %v", err)
	}

	if strings.Contains(buf.String(), secretPassword) {
		t.Fatalf("proxy debug logging leaked the password; captured output:\n%s", buf.String())
	}
}
