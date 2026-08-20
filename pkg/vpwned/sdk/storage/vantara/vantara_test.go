// Copyright © 2026 The vjailbreak authors

package vantara

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/platform9/vjailbreak/pkg/vpwned/sdk/storage"
)

// fakeGUM simulates the Configuration Manager REST API surface the provider
// consumes: version, sessions, ldevs (create/label/get/list/delete via async
// jobs), pools, and job polling.
type fakeGUM struct {
	t          *testing.T
	apiVersion string

	mu struct {
		nextLdevID int
		nextJobID  int
		ldevs      map[int]*ldevInfo
		jobs       map[int]map[string]any
		pools      []poolInfo

		createBodies []map[string]any
		labelBodies  map[int]string
		deleted      []int
	}
}

func newFakeGUM(t *testing.T) *fakeGUM {
	f := &fakeGUM{t: t, apiVersion: "1.9.0"}
	f.mu.nextLdevID = 1536
	f.mu.nextJobID = 1
	f.mu.ldevs = map[int]*ldevInfo{}
	f.mu.jobs = map[int]map[string]any{}
	f.mu.labelBodies = map[int]string{}
	f.mu.pools = []poolInfo{{PoolID: 5, PoolName: "dp-pool-1", PoolType: "DP"}}
	return f
}

func (f *fakeGUM) newJob(affected string) int {
	id := f.mu.nextJobID
	f.mu.nextJobID++
	job := map[string]any{
		"jobId":  float64(id),
		"status": "Completed",
		"state":  "Succeeded",
	}
	if affected != "" {
		job["affectedResources"] = []any{affected}
	}
	f.mu.jobs[id] = job
	return id
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func (f *fakeGUM) handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/ConfigurationManager/configuration/version", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"apiVersion": f.apiVersion, "productName": "Configuration Manager REST API"})
	})

	mux.HandleFunc("/ConfigurationManager/v1/objects/sessions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(405)
			return
		}
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Basic ") {
			w.WriteHeader(401)
			return
		}
		writeJSON(w, 200, map[string]any{"token": "tok-123", "sessionId": float64(7)})
	})
	mux.HandleFunc("/ConfigurationManager/v1/objects/sessions/", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{})
	})

	mux.HandleFunc("/ConfigurationManager/v1/objects/jobs/", func(w http.ResponseWriter, r *http.Request) {
		idStr := strings.TrimPrefix(r.URL.Path, "/ConfigurationManager/v1/objects/jobs/")
		var id int
		fmt.Sscanf(idStr, "%d", &id)
		job, ok := f.mu.jobs[id]
		if !ok {
			w.WriteHeader(404)
			return
		}
		writeJSON(w, 200, job)
	})

	mux.HandleFunc("/ConfigurationManager/v1/objects/pools", func(w http.ResponseWriter, r *http.Request) {
		rows := make([]any, 0, len(f.mu.pools))
		for _, p := range f.mu.pools {
			rows = append(rows, map[string]any{"poolId": float64(p.PoolID), "poolName": p.PoolName, "poolType": p.PoolType})
		}
		writeJSON(w, 200, map[string]any{"data": rows})
	})
	mux.HandleFunc("/ConfigurationManager/v1/objects/pools/", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"poolId": float64(5), "poolName": "dp-pool-1", "poolType": "DP"})
	})

	mux.HandleFunc("/ConfigurationManager/v1/objects/ldevs", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			f.mu.createBodies = append(f.mu.createBodies, body)
			id := f.mu.nextLdevID
			f.mu.nextLdevID++
			blocks := int64(0)
			if v, ok := body["blockCapacity"].(float64); ok {
				blocks = int64(v)
			}
			f.mu.ldevs[id] = &ldevInfo{
				LdevID:        id,
				BlockCapacity: blocks,
				PoolID:        5,
				Status:        "NML",
				NaaID:         fmt.Sprintf("60060E80072B9700%016X", id),
			}
			jobID := f.newJob(fmt.Sprintf("/ConfigurationManager/v1/objects/ldevs/%d", id))
			writeJSON(w, 202, map[string]any{"jobId": float64(jobID), "self": "job"})
		case http.MethodGet:
			q, _ := url.ParseQuery(r.URL.RawQuery)
			_ = q
			rows := make([]any, 0, len(f.mu.ldevs))
			for _, l := range f.mu.ldevs {
				rows = append(rows, map[string]any{
					"ldevId": float64(l.LdevID), "label": l.Label, "naaId": l.NaaID,
					"blockCapacity": float64(l.BlockCapacity), "poolId": float64(l.PoolID), "status": l.Status,
				})
			}
			writeJSON(w, 200, map[string]any{"data": rows})
		default:
			w.WriteHeader(405)
		}
	})

	mux.HandleFunc("/ConfigurationManager/v1/objects/ldevs/", func(w http.ResponseWriter, r *http.Request) {
		idStr := strings.TrimPrefix(r.URL.Path, "/ConfigurationManager/v1/objects/ldevs/")
		var id int
		fmt.Sscanf(idStr, "%d", &id)
		l, ok := f.mu.ldevs[id]
		if !ok {
			w.WriteHeader(404)
			return
		}
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, 200, map[string]any{
				"ldevId": float64(l.LdevID), "label": l.Label, "naaId": l.NaaID,
				"blockCapacity": float64(l.BlockCapacity), "poolId": float64(l.PoolID), "status": l.Status,
			})
		case http.MethodPut:
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			if label, ok := body["label"].(string); ok {
				l.Label = label
				f.mu.labelBodies[id] = label
			}
			writeJSON(w, 202, map[string]any{"jobId": float64(f.newJob(""))})
		case http.MethodDelete:
			delete(f.mu.ldevs, id)
			f.mu.deleted = append(f.mu.deleted, id)
			writeJSON(w, 202, map[string]any{"jobId": float64(f.newJob(""))})
		default:
			w.WriteHeader(405)
		}
	})

	return mux
}

func newTestProvider(t *testing.T, f *fakeGUM, opts map[string]string) (*VantaraStorageProvider, *httptest.Server) {
	t.Helper()
	srv := httptest.NewTLSServer(f.handler())
	t.Cleanup(srv.Close)

	hostPort := strings.TrimPrefix(srv.URL, "https://")

	p := &VantaraStorageProvider{}
	err := p.Connect(context.Background(), storage.StorageAccessInfo{
		Hostname:            hostPort,
		Username:            "maintenance",
		Password:            "secret",
		SkipSSLVerification: true,
		VendorType:          VendorName,
		ProviderOptions:     opts,
	})
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	return p, srv
}

func TestConnectRejectsOldAPIVersion(t *testing.T) {
	f := newFakeGUM(t)
	f.apiVersion = "1.8.4"
	srv := httptest.NewTLSServer(f.handler())
	defer srv.Close()

	p := &VantaraStorageProvider{}
	err := p.Connect(context.Background(), storage.StorageAccessInfo{
		Hostname:            strings.TrimPrefix(srv.URL, "https://"),
		Username:            "u",
		Password:            "p",
		SkipSSLVerification: true,
	})
	if err == nil || !strings.Contains(err.Error(), "1.9") {
		t.Fatalf("expected API version gate error, got %v", err)
	}
}

func TestCreateVolume(t *testing.T) {
	f := newFakeGUM(t)
	p, _ := newTestProvider(t, f, map[string]string{OptionPoolID: "5"})

	// 1 GiB + 100 bytes forces 512-byte block rounding up.
	size := int64(1<<30 + 100)
	vol, err := p.CreateVolume("myvm-Hard-disk-1-with-a-very-long-name-beyond-32", size)
	if err != nil {
		t.Fatalf("CreateVolume failed: %v", err)
	}

	if len(f.mu.createBodies) != 1 {
		t.Fatalf("expected 1 create call, got %d", len(f.mu.createBodies))
	}
	body := f.mu.createBodies[0]
	if got := int(body["poolId"].(float64)); got != 5 {
		t.Fatalf("poolId not honored: %d", got)
	}
	wantBlocks := int64(1<<30/512 + 1)
	if got := int64(body["blockCapacity"].(float64)); got != wantBlocks {
		t.Fatalf("blockCapacity rounding wrong: got %d want %d", got, wantBlocks)
	}

	if vol.Id != "1536" {
		t.Fatalf("LDEV id not extracted from job affectedResources: %q", vol.Id)
	}
	if len(vol.Name) > maxLdevLabelLen {
		t.Fatalf("label not truncated to %d chars: %q", maxLdevLabelLen, vol.Name)
	}
	if !strings.HasPrefix(vol.NAA, "naa.60060e80") {
		t.Fatalf("NAA not lowercased from naaId: %q", vol.NAA)
	}
	if vol.Size != wantBlocks*512 {
		t.Fatalf("size mismatch: %d", vol.Size)
	}
}

func TestCreateVolumeRequiresPoolWhenAmbiguous(t *testing.T) {
	f := newFakeGUM(t)
	f.mu.pools = append(f.mu.pools, poolInfo{PoolID: 9, PoolName: "dp-pool-2", PoolType: "DP"})
	p, _ := newTestProvider(t, f, nil)

	if _, err := p.CreateVolume("v", 1<<30); err == nil || !strings.Contains(err.Error(), "pool") {
		t.Fatalf("expected pool selection error, got %v", err)
	}
}

func TestCreateVolumeAutoPicksSinglePool(t *testing.T) {
	f := newFakeGUM(t)
	p, _ := newTestProvider(t, f, nil) // no pool option; fake has exactly one DP pool

	if _, err := p.CreateVolume("v", 1<<30); err != nil {
		t.Fatalf("expected single-pool auto-pick, got %v", err)
	}
	if got := int(f.mu.createBodies[0]["poolId"].(float64)); got != 5 {
		t.Fatalf("auto-picked wrong pool: %d", got)
	}
}

func TestResolveCinderVolumeToLUN(t *testing.T) {
	f := newFakeGUM(t)
	p, _ := newTestProvider(t, f, map[string]string{OptionPoolID: "5"})

	// Simulate an LDEV relabelled by HBSD manage: cinder UUID sans dashes.
	cinderID := "9b3c2f1e-8a45-4c3d-9d21-0e5f6a7b8c9d"
	f.mu.ldevs[2000] = &ldevInfo{
		LdevID:        2000,
		Label:         strings.ToUpper(strings.ReplaceAll(cinderID, "-", "")), // case-insensitive match
		NaaID:         "60060E80072B970000000000000007D0",
		BlockCapacity: 2097152,
		PoolID:        5,
		Status:        "NML",
	}

	vol, err := p.ResolveCinderVolumeToLUN(cinderID)
	if err != nil {
		t.Fatalf("ResolveCinderVolumeToLUN failed: %v", err)
	}
	if vol.Id != "2000" {
		t.Fatalf("wrong LDEV resolved: %+v", vol)
	}
	if vol.NAA != "naa.60060e80072b970000000000000007d0" {
		t.Fatalf("NAA mismatch: %q", vol.NAA)
	}

	if _, err := p.ResolveCinderVolumeToLUN("00000000-0000-0000-0000-000000000000"); err == nil {
		t.Fatal("expected error for unknown cinder volume")
	}
}

func TestDeleteVolume(t *testing.T) {
	f := newFakeGUM(t)
	p, _ := newTestProvider(t, f, map[string]string{OptionPoolID: "5"})

	f.mu.ldevs[2001] = &ldevInfo{LdevID: 2001, Label: "doomed", NaaID: "60060E8007000000000000000000AAAA", Status: "NML"}

	if err := p.DeleteVolume("doomed"); err != nil {
		t.Fatalf("DeleteVolume failed: %v", err)
	}
	if len(f.mu.deleted) != 1 || f.mu.deleted[0] != 2001 {
		t.Fatalf("wrong ldev deleted: %v", f.mu.deleted)
	}

	// Idempotent: deleting a missing volume is not an error.
	if err := p.DeleteVolume("doomed"); err != nil {
		t.Fatalf("expected idempotent delete, got %v", err)
	}
}

func TestBuildCinderManageRef(t *testing.T) {
	p := &VantaraStorageProvider{}

	ref := p.BuildCinderManageRef(storage.Volume{Name: "lbl", Id: "1536"})
	if ref["source-id"] != "1536" {
		t.Fatalf("expected source-id ref, got %v", ref)
	}

	ref = p.BuildCinderManageRef(storage.Volume{Name: "lbl"})
	if ref["source-name"] != "lbl" {
		t.Fatalf("expected source-name fallback, got %v", ref)
	}
}

func TestGetAllVolumeNAAs(t *testing.T) {
	f := newFakeGUM(t)
	p, _ := newTestProvider(t, f, map[string]string{OptionPoolID: "5"})

	f.mu.ldevs[1] = &ldevInfo{LdevID: 1, Label: "a", NaaID: "60060E8007000000000000000000000A", Status: "NML"}
	f.mu.ldevs[2] = &ldevInfo{LdevID: 2, Label: "b", NaaID: "", Status: "NML"} // no NAA -> skipped

	naas, err := p.GetAllVolumeNAAs()
	if err != nil {
		t.Fatalf("GetAllVolumeNAAs failed: %v", err)
	}
	if len(naas) != 1 || naas[0] != "naa.60060e8007000000000000000000000a" {
		t.Fatalf("unexpected NAAs: %v", naas)
	}
}

func TestApplyCinderPoolHint(t *testing.T) {
	t.Run("numeric hint is verified and applied", func(t *testing.T) {
		f := newFakeGUM(t)
		p, _ := newTestProvider(t, f, nil)
		if err := p.ApplyCinderPoolHint(context.Background(), "5"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if p.poolID == nil || *p.poolID != 5 {
			t.Fatalf("pool not applied from numeric hint: %v", p.poolID)
		}
	})

	t.Run("name hint resolves case-insensitively", func(t *testing.T) {
		f := newFakeGUM(t)
		p, _ := newTestProvider(t, f, nil)
		if err := p.ApplyCinderPoolHint(context.Background(), "DP-POOL-1"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if p.poolID == nil || *p.poolID != 5 {
			t.Fatalf("pool not resolved from name hint: %v", p.poolID)
		}
	})

	t.Run("unknown name errors and lists available pools", func(t *testing.T) {
		f := newFakeGUM(t)
		p, _ := newTestProvider(t, f, nil)
		err := p.ApplyCinderPoolHint(context.Background(), "no-such-pool")
		if err == nil || !strings.Contains(err.Error(), "dp-pool-1") {
			t.Fatalf("expected error listing available pools, got %v", err)
		}
	})

	t.Run("explicit pool option wins over hint", func(t *testing.T) {
		f := newFakeGUM(t)
		f.mu.pools = append(f.mu.pools, poolInfo{PoolID: 9, PoolName: "dp-pool-2", PoolType: "DP"})
		p, _ := newTestProvider(t, f, map[string]string{OptionPoolID: "5"})
		if err := p.ApplyCinderPoolHint(context.Background(), "9"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if p.poolID == nil || *p.poolID != 5 {
			t.Fatalf("explicit pool overridden by hint: %v", p.poolID)
		}
	})

	t.Run("empty hint is a no-op", func(t *testing.T) {
		f := newFakeGUM(t)
		p, _ := newTestProvider(t, f, nil)
		if err := p.ApplyCinderPoolHint(context.Background(), "  "); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if p.poolID != nil {
			t.Fatalf("empty hint must not set a pool: %v", p.poolID)
		}
	})
}

func TestVantaraDoesNotImplementVendorMapper(t *testing.T) {
	var p storage.StorageProvider = &VantaraStorageProvider{}
	if _, ok := p.(storage.VendorMapper); ok {
		t.Fatal("vantara must NOT implement VendorMapper — mapping is delegated to the Cinder fallback")
	}
}
