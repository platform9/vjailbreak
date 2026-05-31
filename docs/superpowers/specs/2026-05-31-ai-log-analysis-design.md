# AI Log Analysis — Design

**Date**: 2026-05-31  
**Status**: Approved  
**Feature**: "Analyse with AI" for failed migrations in the vJailbreak logs drawer

---

## Problem

When a VM migration fails, operators must manually sift through debug logs, pod logs, and multiple related Kubernetes CRs to diagnose root cause. This is time-consuming and requires deep familiarity with vJailbreak internals.

## Goal

Add an "Analyse with AI" capability inside the existing migration logs drawer. For failed migrations, operators get an instant AI-generated root cause analysis and remediation steps. They can also ask follow-up questions about the failure.

---

## Architecture

```
vJailbreak VM (k3s)
│
├── vjailbreak-ui  (React)
│   └── BaseLogsDrawer → new "AI Analysis" tab
│       calls POST /api/v1/ai/analyze via existing axios client
│
├── vjailbreak-api  (Go, pkg/vpwned)
│   └── new handler: POST /api/v1/ai/analyze
│       1. fetches migration CR + related CRs from k8s
│       2. fetches debug logs from /debug-logs/ nginx endpoint
│       3. fetches v2v-helper + controller pod logs from k8s logs API
│       4. extracts relevant lines (see Data section)
│       5. POSTs compact payload to vjailbreak-ai (ClusterIP)
│       6. returns structured response to UI
│
└── vjailbreak-ai  (NEW — Python FastAPI, derived from vjailbreak-chat)
    └── POST /analyze
        reads ANTHROPIC_API_KEY from env (mounted from k8s Secret)
        calls Anthropic Claude API
        returns structured JSON response
        ClusterIP only — not exposed externally
```

**Key invariant:** Raw logs never leave the vJailbreak VM except via the Anthropic API call from vjailbreak-ai. The UI never calls vjailbreak-ai directly.

---

## vjailbreak-ai Service

Derived from the vjailbreak-chat project (`~/Downloads/vjailbreak-chat`). Remove: ChromaDB, vector embeddings, crawler, doc Q&A endpoints, RAG pipeline. Keep: FastAPI skeleton, Anthropic SDK integration, security headers, rate limiting, Dockerfile, k8s manifests structure.

### Endpoint

```
POST /analyze
Content-Type: application/json

Request:
{
  "migration_name": "vm-prod-01",
  "namespace": "migration-system",
  "context": {
    "migration_cr": { ...full CR status+spec... },
    "migration_plan": { ...spec... },
    "migration_template": { ...spec minus credential refs... },
    "network_mapping": { ...spec... },       // if present
    "storage_mapping": { ...spec... },       // if present
    "v2v_logs": "..extracted log lines..",
    "controller_logs": "..extracted log lines..",
    "debug_logs": { "filename.log": "..extracted.." }
  },
  "conversation_history": [],   // [] for initial analysis, prior turns for follow-up
  "question": null              // null = initial analysis, string = follow-up question
}

Response:
{
  "root_cause": "string",
  "fix_steps": ["string", ...],
  "summary": "string",
  "confidence": "high" | "medium" | "low",
  "raw_response": "string"     // full Claude response for follow-up context
}
```

### k8s Resources

```yaml
Deployment: vjailbreak-ai
  namespace: vjailbreak
  replicas: 1
  image: platform9/vjailbreak-ai:<tag>
  env:
    - name: ANTHROPIC_API_KEY
      valueFrom:
        secretKeyRef:
          name: vjailbreak-ai-secret
          key: api-key
  resources:
    requests: { cpu: 100m, memory: 128Mi }
    limits:   { cpu: 500m, memory: 256Mi }

Service: vjailbreak-ai
  type: ClusterIP
  port: 8080

Secret: vjailbreak-ai-secret
  key: api-key
  managed by: vJailbreak settings API (created/updated when user saves key in UI)
```

---

## Data Collection and Extraction

The vjailbreak-api handler assembles context before forwarding to vjailbreak-ai.

### CRs Fetched (via existing k8s client in pkg/vpwned)

| CR | Fields included | Fields excluded |
|----|----------------|-----------------|
| Migration | full `status` (conditions, phase, syncWarning), full `spec` | — |
| MigrationPlan | full `spec` | — |
| MigrationTemplate | full `spec` | `vmwareRef`, `openstackRef` values (credential references stripped) |
| NetworkMapping | full `spec` | — |
| StorageMapping | full `spec` | — |
| ProxyVM | full `spec` | — |
| VMwareCreds | **not fetched** | all (contains credentials) |
| OpenstackCreds | **not fetched** | all (contains credentials) |
| ArrayCreds | **not fetched** | all (contains credentials) |

### Log Extraction Strategy

Applied to v2v-helper pod logs, controller pod logs, and each debug log file:

1. **All ERROR, FAILED, WARN lines** — no cap
2. **±10 lines of surrounding context** for each error line
3. **Last 200 lines** of each log source (final state regardless of level)
4. Deduplicate overlapping context windows

Controller pod logs: ERROR + WARN only (no last-200, too verbose).

Debug logs fetched server-side from `/debug-logs/migration-{name}/` — same nginx endpoint the UI uses for download today. All files under that directory included.

Typical total: 3k–8k tokens per analysis.

---

## UI Changes

### BaseLogsDrawer

1. Accept optional `migrationPhase` + `migrationName` + `namespace` props.
2. When `migrationPhase === 'Failed'`, show "🤖 Analyse with AI" icon button in the toolbar (right side, alongside filter/copy/download).
3. Wrap log content area in two tabs: **Logs** | **🤖 AI Analysis**.
4. Logs tab: unchanged.

### AI Analysis Tab

Three states:

**Idle:**
```
[ 🤖 Click "Analyse with AI" to diagnose this failed migration ]
```

**Loading:**
```
[ ⟳ Analysing logs with AI... ]
```

**Result:**
```
Root Cause                          [high confidence]
────────────────────────────────────────────────────
ESXi host esxi-02 unreachable during CBT disk copy at 67%.
DNS resolution failed — host not in /etc/hosts.

Fix Steps
────────────────────────────────────────────────────
1. Add "192.168.1.102 esxi-02" to /etc/hosts on vJailbreak VM
2. Verify VDDK library at /home/ubuntu/vmware-vix-disklib-distrib
3. Retry migration

────────────────────────────────────────────────────
[ Ask a follow-up question...              ] [Send]
```

Follow-up messages append to `conversation_history`. Re-clicking "Analyse with AI" resets history and re-runs initial analysis.

### Settings Page

New "AI Configuration" section:
- **Anthropic API Key** — password-type text field
- Save → `POST /api/v1/settings/ai` → creates/updates k8s Secret `vjailbreak-ai-secret`
- Shows masked key if already configured

---

## Error Handling

| Condition | UI behavior |
|-----------|-------------|
| API key not configured | Tab shows: "Configure Anthropic API key in Settings" with link |
| vjailbreak-ai unreachable | "AI service unavailable. Check vjailbreak-ai deployment." |
| Anthropic API error (quota / 5xx) | Error message with Retry button |
| Migration phase ≠ Failed | "Analyse with AI" button hidden |
| Partial log fetch failure | Include available data, note gaps in prompt; do not block analysis |

---

## Testing

### vjailbreak-ai (Python)

- Unit: prompt construction — correct system prompt, conversation history threaded correctly
- Unit: response parsing — structured fields (root_cause, fix_steps, confidence) extracted correctly
- Mock: Anthropic API client

### vjailbreak-api handler (Go)

- Unit: log extraction logic — correct lines selected, context windows, deduplication
- Unit: CR assembly — credential refs stripped from MigrationTemplate
- Unit: excluded CRs (VMwareCreds, OpenstackCreds) never fetched
- Unit: debug log fetch and extraction
- Mock: k8s client, vjailbreak-ai HTTP client

### UI (React)
- Component: tab switching (Logs ↔ AI Analysis)
- Component: button hidden when migration not Failed
- Component: loading / result / error states
- Component: follow-up chat flow
- Mock: `/api/v1/ai/analyze` endpoint

### Integration
- Fixture failed migration CR + sample logs → full analysis pipeline → verify response rendered in UI

---

## Out of Scope (v1)

- Streaming AI responses (show result when complete, not token-by-token)
- Saving/exporting AI analysis results
- Analysis for non-Failed migrations
- Multiple concurrent analyses
- Local LLM support
