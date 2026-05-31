package server

import (
	"bytes"
	"encoding/json"
	"io"
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

// fakeVjailbreakAICapturing captures the request body for inspection.
func fakeVjailbreakAICapturing(t *testing.T, captured *map[string]any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/analyze-migration" {
			http.Error(w, "not found", 404)
			return
		}
		body, _ := io.ReadAll(r.Body)
		var payload map[string]any
		json.Unmarshal(body, &payload)
		*captured = payload
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"root_cause":     "DNS failure",
			"fix_steps":      []string{},
			"summary":        "ok",
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

// T051: additional_context from fetchContext is forwarded to vjailbreak-ai payload.
func TestAIAnalyzeHandler_ForwardsAdditionalContext(t *testing.T) {
	var captured map[string]any
	ai := fakeVjailbreakAICapturing(t, &captured)
	defer ai.Close()

	h := &aiAnalyzeHandler{
		aiURL:      ai.URL,
		httpClient: ai.Client(),
		fetchContext: func(migrationName, namespace string) (map[string]any, error) {
			return map[string]any{
				"migration_cr":       map[string]any{},
				"v2v_logs":           "",
				"controller_logs":    "",
				"debug_logs":         map[string]any{},
				"additional_context": "site note",
				"fetch_warnings":     []string{},
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
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	ctx, ok := captured["context"].(map[string]any)
	if !ok {
		t.Fatalf("forwarded payload missing context field, got: %v", captured)
	}
	if ctx["additional_context"] != "site note" {
		t.Errorf("expected additional_context='site note', got %v", ctx["additional_context"])
	}
}

// T052: assembleMigrationContext returns empty additional_context when ConfigMap absent.
func TestAIAnalyzeHandler_AdditionalContextEmptyWhenConfigMapAbsent(t *testing.T) {
	var captured map[string]any
	ai := fakeVjailbreakAICapturing(t, &captured)
	defer ai.Close()

	h := &aiAnalyzeHandler{
		aiURL:      ai.URL,
		httpClient: ai.Client(),
		fetchContext: func(migrationName, namespace string) (map[string]any, error) {
			// Simulate assembleMigrationContext when ConfigMap is absent:
			// no error, additional_context is empty string
			return map[string]any{
				"migration_cr":       map[string]any{},
				"v2v_logs":           "",
				"controller_logs":    "",
				"debug_logs":         map[string]any{},
				"additional_context": "",
				"fetch_warnings":     []string{},
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
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	ctx, ok := captured["context"].(map[string]any)
	if !ok {
		t.Fatalf("forwarded payload missing context field")
	}
	if ctx["additional_context"] != "" {
		t.Errorf("expected additional_context='', got %v", ctx["additional_context"])
	}
}
