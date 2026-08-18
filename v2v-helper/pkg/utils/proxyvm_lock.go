// Copyright © 2025 The vjailbreak authors

package utils

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/pkg/errors"
	"github.com/platform9/vjailbreak/pkg/common/constants"
)

// This file is v2v-helper's only network communication with vpwned-sdk today
// (v2v-helper otherwise just imports pkg/vpwned/sdk/storage as a Go library,
// it doesn't call the running vpwned-sdk pod). vpwned-sdk holds the Proxy VM
// attach lock in memory, so acquiring/releasing it is a plain HTTP call, not
// a Kubernetes read/write.

// proxyVMLockRequest is the JSON body both lock endpoints expect. It must
// match proxyVMLockRequest in pkg/vpwned/server/proxyvm_lock_handler.go.
type proxyVMLockRequest struct {
	ProxyVMName   string `json:"proxyVMName"`
	MigrationName string `json:"migrationName"`
}

// proxyVMLockAcquireResponse is the JSON body the acquire endpoint returns.
type proxyVMLockAcquireResponse struct {
	Acquired bool   `json:"acquired"`
	Holder   string `json:"holder,omitempty"`
}

// proxyVMLockHTTPClient is shared across calls so retries reuse connections
// instead of dialing vpwned-sdk fresh every poll.
var proxyVMLockHTTPClient = &http.Client{Timeout: 10 * time.Second}

// WaitForProxyVMLock blocks until migrationName holds the attach lock for
// proxyVMName, polling vpwned-sdk every constants.ProxyVMAttachCheckInterval
// and giving up after constants.ProxyVMAttachWaitTimeout.
//
// There is no queue on the vpwned-sdk side: every retry just re-races for the
// lock rather than waiting in line. That's fine here -- losing a race just
// means polling again a few seconds later, not lost work.
func WaitForProxyVMLock(ctx context.Context, proxyVMName, migrationName string) error {
	deadline := time.Now().Add(constants.ProxyVMAttachWaitTimeout)
	loggedWaiting := false

	for {
		resp, err := tryAcquireProxyVMLock(ctx, proxyVMName, migrationName)
		if err != nil {
			return errors.Wrap(err, "failed to acquire Proxy VM attach lock")
		}
		if resp.Acquired {
			if loggedWaiting {
				PrintLog(fmt.Sprintf("Acquired attach lock for Proxy VM %s", proxyVMName))
			}
			return nil
		}

		if !loggedWaiting {
			PrintLog(fmt.Sprintf("Migration %s is currently attaching disks to Proxy VM %s, waiting for turn...",
				resp.Holder, proxyVMName))
			loggedWaiting = true
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("timed out after %s waiting for turn to attach disks to Proxy VM %s",
				constants.ProxyVMAttachWaitTimeout, proxyVMName)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(constants.ProxyVMAttachCheckInterval):
		}
	}
}

// ReleaseProxyVMLock releases migrationName's attach lock on proxyVMName.
// Safe to call even if the lock was never acquired or was already lost to
// another migration -- vpwned-sdk only ever releases a lock for its current
// holder, so this can't clear someone else's in-progress attach.
func ReleaseProxyVMLock(ctx context.Context, proxyVMName, migrationName string) error {
	req := proxyVMLockRequest{ProxyVMName: proxyVMName, MigrationName: migrationName}
	if err := postProxyVMLock(ctx, constants.ProxyVMLockReleasePath, req, nil); err != nil {
		return errors.Wrap(err, "failed to release Proxy VM attach lock")
	}
	return nil
}

// tryAcquireProxyVMLock makes a single acquire attempt against vpwned-sdk --
// no waiting, no retries. WaitForProxyVMLock is the entry point most callers
// want; this exists as its own function purely so that loop stays readable.
func tryAcquireProxyVMLock(ctx context.Context, proxyVMName, migrationName string) (proxyVMLockAcquireResponse, error) {
	var resp proxyVMLockAcquireResponse
	req := proxyVMLockRequest{ProxyVMName: proxyVMName, MigrationName: migrationName}
	err := postProxyVMLock(ctx, constants.ProxyVMLockAcquirePath, req, &resp)
	return resp, err
}

// postProxyVMLock POSTs req as JSON to the given vpwned-sdk path and decodes
// the response into out. Pass a nil out to ignore the response body.
func postProxyVMLock(ctx context.Context, path string, req proxyVMLockRequest, out interface{}) error {
	body, err := json.Marshal(req)
	if err != nil {
		return errors.Wrap(err, "failed to marshal proxy VM lock request")
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		constants.VpwnedSDKServiceBaseURL+path, bytes.NewReader(body))
	if err != nil {
		return errors.Wrap(err, "failed to build proxy VM lock request")
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := proxyVMLockHTTPClient.Do(httpReq)
	if err != nil {
		return errors.Wrap(err, "failed to reach vpwned-sdk proxy VM lock endpoint")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("vpwned-sdk proxy VM lock endpoint %s returned status %d", path, resp.StatusCode)
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
