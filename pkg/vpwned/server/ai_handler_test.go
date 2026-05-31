package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func fakeVjailbreakAI(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/analyze-migration" {
			http.Error(w, "not found", 404)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"root_cause":     "DNS failure",
			"fix_steps":      []string{"add to /etc/hosts"},
			"summary":        "DNS issue",
			"confidence":     "high",
			"doc_references": []string{},
			"github_issue":   map[string]any{"should_open": false},
		})
	}))
}

func TestAIAnalyzeHandler_RequiresPOST(t *testing.T) {
	h := &aiAnalyzeHandler{aiURL: "http://localhost:0"}
	req := httptest.NewRequest(http.MethodGet, "/vpw/v1/ai/analyze", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestAIAnalyzeHandler_MissingParams(t *testing.T) {
	h := &aiAnalyzeHandler{aiURL: "http://localhost:0"}
	req := httptest.NewRequest(http.MethodPost, "/vpw/v1/ai/analyze", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestAIAnalyzeHandler_ForwardsToAIService(t *testing.T) {
	ai := fakeVjailbreakAI(t)
	defer ai.Close()

	h := &aiAnalyzeHandler{
		aiURL:      ai.URL,
		httpClient: ai.Client(),
		fetchContext: func(migrationName, namespace string) (map[string]any, error) {
			return map[string]any{
				"migration_cr":    map[string]any{},
				"v2v_logs":        "ERROR: disk failed",
				"controller_logs": "",
				"debug_logs":      map[string]any{},
			}, nil
		},
	}

	body, _ := json.Marshal(map[string]string{
		"migration_name": "migration-my-vm",
		"namespace":      "migration-system",
	})
	req := httptest.NewRequest(http.MethodPost, "/vpw/v1/ai/analyze", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}
	if resp["root_cause"] != "DNS failure" {
		t.Errorf("unexpected root_cause: %v", resp["root_cause"])
	}
}
