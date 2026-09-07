package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDebugPromptHandler_GET(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/vpw/v1/ai/debug-prompt", nil)
	w := httptest.NewRecorder()

	HandleDebugPrompt(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected application/json, got %q", ct)
	}

	var resp struct {
		Version     string `json:"version"`
		LastUpdated string `json:"last_updated"`
		Prompt      string `json:"prompt"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Version == "" {
		t.Error("version field is empty")
	}
	if resp.LastUpdated == "" {
		t.Error("last_updated field is empty")
	}
	if resp.Prompt == "" {
		t.Error("prompt field is empty")
	}
}

func TestDebugPromptHandler_nonGET(t *testing.T) {
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch} {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/vpw/v1/ai/debug-prompt", nil)
			w := httptest.NewRecorder()

			HandleDebugPrompt(w, req)

			if w.Code != http.StatusMethodNotAllowed {
				t.Errorf("expected 405 for %s, got %d", method, w.Code)
			}
		})
	}
}
