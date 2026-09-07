package server

import (
	_ "embed"
	"encoding/json"
	"net/http"
	"strings"
)

//go:embed debug_prompt_embedded.txt
var debugPromptEmbedded string

// debugPromptResponse is the JSON body returned by GET /vpw/v1/ai/debug-prompt.
type debugPromptResponse struct {
	Version     string `json:"version"`
	LastUpdated string `json:"last_updated"`
	Prompt      string `json:"prompt"`
}

var parsedDebugPrompt debugPromptResponse

func init() {
	parsedDebugPrompt = parseDebugPromptEmbedded(debugPromptEmbedded)
}

// parseDebugPromptEmbedded extracts version and last_updated from the first two
// header lines written by scripts/build-debug-prompt.sh, and treats the
// remainder as the prompt body.
func parseDebugPromptEmbedded(content string) debugPromptResponse {
	lines := strings.SplitN(content, "\n", 3)
	var version, lastUpdated, prompt string

	if len(lines) >= 1 {
		version = strings.TrimPrefix(strings.TrimSpace(lines[0]), "version: ")
	}
	if len(lines) >= 2 {
		lastUpdated = strings.TrimPrefix(strings.TrimSpace(lines[1]), "last_updated: ")
	}
	if len(lines) >= 3 {
		prompt = strings.TrimPrefix(lines[2], "\n")
	}

	return debugPromptResponse{
		Version:     version,
		LastUpdated: lastUpdated,
		Prompt:      prompt,
	}
}

// HandleDebugPrompt serves GET /vpw/v1/ai/debug-prompt.
// Returns the embedded skill system prompt with version metadata.
func HandleDebugPrompt(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(parsedDebugPrompt)
}
