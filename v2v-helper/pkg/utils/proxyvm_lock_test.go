package utils

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/platform9/vjailbreak/pkg/common/constants"
)

func newLockServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv
}

func withBaseURL(t *testing.T, url string) {
	t.Helper()
	orig := vpwnedSDKBaseURL
	vpwnedSDKBaseURL = url
	t.Cleanup(func() { vpwnedSDKBaseURL = orig })
}

func withPollTiming(t *testing.T, interval, timeout time.Duration) {
	t.Helper()
	origInterval, origTimeout := proxyVMAttachCheckInterval, proxyVMAttachWaitTimeout
	proxyVMAttachCheckInterval = interval
	proxyVMAttachWaitTimeout = timeout
	t.Cleanup(func() {
		proxyVMAttachCheckInterval = origInterval
		proxyVMAttachWaitTimeout = origTimeout
	})
}

func jsonResponse(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func TestWaitForProxyVMLock_AcquiredImmediately(t *testing.T) {
	srv := newLockServer(t, func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, proxyVMLockAcquireResponse{Acquired: true})
	})
	withBaseURL(t, srv.URL)

	if err := WaitForProxyVMLock(context.Background(), "proxy-geet", "migration-a"); err != nil {
		t.Fatalf("WaitForProxyVMLock() = %v, want nil", err)
	}
}

func TestWaitForProxyVMLock_SendsCorrectPathAndBody(t *testing.T) {
	var gotPath string
	var gotContentType string
	var gotBody proxyVMLockRequest
	srv := newLockServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotContentType = r.Header.Get("Content-Type")
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("failed to decode request body: %v", err)
		}
		jsonResponse(w, proxyVMLockAcquireResponse{Acquired: true})
	})
	withBaseURL(t, srv.URL)

	if err := WaitForProxyVMLock(context.Background(), "proxy-geet", "migration-42"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPath != constants.ProxyVMLockAcquirePath {
		t.Errorf("request path = %q, want %q", gotPath, constants.ProxyVMLockAcquirePath)
	}
	if gotContentType != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", gotContentType)
	}
	if gotBody.ProxyVMName != "proxy-geet" || gotBody.MigrationName != "migration-42" {
		t.Errorf("request body = %+v, want ProxyVMName=proxy-geet MigrationName=migration-42", gotBody)
	}
}

func TestWaitForProxyVMLock_PollsUntilAcquired(t *testing.T) {
	var calls int32
	srv := newLockServer(t, func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n < 3 {
			jsonResponse(w, proxyVMLockAcquireResponse{Acquired: false, Holder: "other-migration"})
			return
		}
		jsonResponse(w, proxyVMLockAcquireResponse{Acquired: true})
	})
	withBaseURL(t, srv.URL)
	withPollTiming(t, 5*time.Millisecond, time.Second)

	if err := WaitForProxyVMLock(context.Background(), "proxy-geet", "migration-a"); err != nil {
		t.Fatalf("WaitForProxyVMLock() = %v, want nil", err)
	}
	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Errorf("server received %d acquire attempts, want 3 (2 losses then a win)", got)
	}
}

func TestWaitForProxyVMLock_TimesOut(t *testing.T) {
	srv := newLockServer(t, func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, proxyVMLockAcquireResponse{Acquired: false, Holder: "other-migration"})
	})
	withBaseURL(t, srv.URL)
	withPollTiming(t, 5*time.Millisecond, 20*time.Millisecond)

	err := WaitForProxyVMLock(context.Background(), "proxy-geet", "migration-a")
	if err == nil {
		t.Fatal("WaitForProxyVMLock() = nil, want a timeout error")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Errorf("error = %q, want it to mention timing out", err.Error())
	}
}

func TestWaitForProxyVMLock_ContextCancelled(t *testing.T) {
	srv := newLockServer(t, func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, proxyVMLockAcquireResponse{Acquired: false, Holder: "other-migration"})
	})
	withBaseURL(t, srv.URL)
	withPollTiming(t, time.Minute, time.Hour) // long enough that only cancellation can end this

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(5 * time.Millisecond)
		cancel()
	}()

	err := WaitForProxyVMLock(ctx, "proxy-geet", "migration-a")
	if err != context.Canceled {
		t.Errorf("WaitForProxyVMLock() = %v, want context.Canceled", err)
	}
}

func TestWaitForProxyVMLock_ServerError(t *testing.T) {
	srv := newLockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	withBaseURL(t, srv.URL)

	err := WaitForProxyVMLock(context.Background(), "proxy-geet", "migration-a")
	if err == nil {
		t.Fatal("WaitForProxyVMLock() = nil, want an error")
	}
	if !strings.Contains(err.Error(), "failed to acquire Proxy VM attach lock") {
		t.Errorf("error = %q, want it to mention failing to acquire the lock", err.Error())
	}
}

func TestReleaseProxyVMLock_Success(t *testing.T) {
	var gotPath string
	var gotBody proxyVMLockRequest
	srv := newLockServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("failed to decode request body: %v", err)
		}
		jsonResponse(w, struct {
			Released bool `json:"released"`
		}{Released: true})
	})
	withBaseURL(t, srv.URL)

	if err := ReleaseProxyVMLock(context.Background(), "proxy-geet", "migration-42"); err != nil {
		t.Fatalf("ReleaseProxyVMLock() = %v, want nil", err)
	}
	if gotPath != constants.ProxyVMLockReleasePath {
		t.Errorf("request path = %q, want %q", gotPath, constants.ProxyVMLockReleasePath)
	}
	if gotBody.ProxyVMName != "proxy-geet" || gotBody.MigrationName != "migration-42" {
		t.Errorf("request body = %+v, want ProxyVMName=proxy-geet MigrationName=migration-42", gotBody)
	}
}

func TestReleaseProxyVMLock_ServerError(t *testing.T) {
	srv := newLockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	withBaseURL(t, srv.URL)

	err := ReleaseProxyVMLock(context.Background(), "proxy-geet", "migration-42")
	if err == nil {
		t.Fatal("ReleaseProxyVMLock() = nil, want an error")
	}
	if !strings.Contains(err.Error(), "failed to release Proxy VM attach lock") {
		t.Errorf("error = %q, want it to mention failing to release the lock", err.Error())
	}
}

func TestReleaseProxyVMLock_NeverHeldIsSafe(t *testing.T) {
	srv := newLockServer(t, func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, struct {
			Released bool `json:"released"`
		}{Released: true})
	})
	withBaseURL(t, srv.URL)

	if err := ReleaseProxyVMLock(context.Background(), "proxy-geet", "migration-never-held-it"); err != nil {
		t.Fatalf("ReleaseProxyVMLock() = %v, want nil", err)
	}
}
