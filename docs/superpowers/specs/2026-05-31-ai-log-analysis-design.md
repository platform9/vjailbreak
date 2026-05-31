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
│       calls POST /vpw/v1/ai/analyze via existing axios client
│
├── vjailbreak-api  (Go, pkg/vpwned)
│   └── new handler: POST /vpw/v1/ai/analyze
│       1. fetches migration CR + related CRs from k8s
│       2. fetches debug logs from /debug-logs/ nginx endpoint
│       3. fetches v2v-helper + controller pod logs from k8s logs API
│       4. extracts relevant lines (see Data section)
│       5. POSTs compact payload to vjailbreak-ai (ClusterIP)
│       6. returns structured response to UI
│
└── vjailbreak-ai  (NEW — Python FastAPI, derived from vjailbreak-chat)
    ├── POST /analyze-migration  ← new endpoint
    ├── POST /query              ← existing doc Q&A kept
    ├── POST /crawl              ← existing crawler kept (admin)
    └── ChromaDB + RAG pipeline  ← kept, indexes vjailbreak + virt-v2v docs
        reads ANTHROPIC_API_KEY from env (mounted from k8s Secret)
        ClusterIP only — not exposed externally
```

**Key invariant:** Raw logs never leave the vJailbreak VM except via the Anthropic API call from vjailbreak-ai. The UI never calls vjailbreak-ai directly.

---

## vjailbreak-ai Service

Derived from the vjailbreak-chat project (`~/Downloads/vjailbreak-chat`). **Keep full stack**: ChromaDB, RAG pipeline, crawler, doc Q&A endpoints, Anthropic SDK, security headers, rate limiting, Dockerfile, k8s manifests. Extend with a new `/analyze-migration` endpoint.

### Why Keep RAG

When logs show errors like `virt-v2v: libguestfs error` or `VDDK connection refused`, the RAG pipeline retrieves the relevant troubleshooting doc sections and injects them into the Claude prompt. Without RAG, Claude relies only on training data for vjailbreak-specific error patterns. With RAG, analysis is grounded in actual documentation.

**Crawled knowledge sources** (indexed into ChromaDB at startup / on-demand):

- vJailbreak docs: `https://platform9.github.io/vjailbreak/`
- virt-v2v support: `https://libguestfs.org/virt-v2v-support.1.html`
- virt-v2v manual: `https://libguestfs.org/virt-v2v.1.html`
- nbdkit manual: `https://libguestfs.org/nbdkit.1.html`
- A curated `error-catalog.md` shipped with the service (known vjailbreak error patterns + fixes)

### Endpoints

**Existing (from vjailbreak-chat), kept as-is:**

```
POST /query           — doc Q&A (public, rate-limited)
GET/POST /context     — admin: manage knowledge base (API key required)
POST /crawl           — admin: re-crawl docs (API key required)
GET /health           — health check
GET /stats            — usage stats
```

**New endpoint:**

```
POST /analyze-migration
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
    "debug_logs": { "filename.log": "..extracted.." },
    "additional_context": "..from vjailbreak-ai-context ConfigMap, empty string if absent..",
    "fetch_warnings": ["string", ...]   // log fetch failures — injected as "## Fetch Warnings" in prompt
  },
  "conversation_history": [],   // [] for initial analysis, prior turns for follow-up
  "question": null              // null = initial analysis, string = follow-up question
}

Response:
{
  "root_cause": "string | null",       // null if cannot determine
  "fix_steps": ["string", ...],        // empty if cannot determine
  "summary": "string",
  "confidence": "high" | "medium" | "low" | "none",
  "doc_references": ["url", ...],      // RAG source docs used
  "github_issue": {                    // populated when confidence = "none" or "low"
    "should_open": true | false,
    "title": "string",
    "body": "string",                  // pre-filled issue body with migration details
    "prefill_url": "string",           // github.com/platform9/vjailbreak/issues/new?...
    "collect_first": ["string", ...]   // checklist of data to gather before filing
  },
  "raw_response": "string"
}
```

### k8s Resources

```yaml
Deployment: vjailbreak-ai
  namespace: migration-system
  replicas: 1
  image: platform9/vjailbreak-ai:<tag>
  env:
    - name: ANTHROPIC_API_KEY
      valueFrom:
        secretKeyRef:
          name: vjailbreak-ai-secret
          key: api-key
  resources:
    requests: { cpu: 200m, memory: 512Mi }   # ChromaDB needs more memory
    limits:   { cpu: 1000m, memory: 1Gi }
  volumeMounts:
    - name: chroma-data
      mountPath: /data
  volumes:
    - name: chroma-data
      persistentVolumeClaim:
        claimName: vjailbreak-ai-chroma

PersistentVolumeClaim: vjailbreak-ai-chroma
  storage: 2Gi                               # ChromaDB + indexed docs

Service: vjailbreak-ai
  type: ClusterIP
  port: 8080

Secret: vjailbreak-ai-secret
  keys:
    api-key:   Anthropic API key
    admin-key: vjailbreak-ai admin key (for /crawl, /context endpoints)
  managed by: vJailbreak settings API (created/updated when user saves both keys in UI)
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

### Operator-Provided Additional Context (ConfigMap)

vpwned reads `ConfigMap/vjailbreak-ai-context` (namespace: `migration-system`) on every analysis request and passes `data.additional_context` to vjailbreak-ai. If the ConfigMap is absent or the key is empty, the field is omitted from the prompt silently.

vjailbreak-ai injects this as an **"Operator-Provided Context"** section in the user message, before the RAG documentation section. This lets operators encode site-specific knowledge without changing code:

- Custom VDDK install path (if non-standard)
- ESXi certificate policy (self-signed certs expected)
- OpenStack volume type mappings in use
- Known environment quirks that affect migrations

**k8s resource:**

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: vjailbreak-ai-context
  namespace: migration-system
data:
  additional_context: |
    VDDK is installed at /opt/vmware/vddk (non-standard path).
    All ESXi hosts use self-signed TLS certificates — SSL errors are expected.
```

Operators update via `kubectl edit configmap vjailbreak-ai-context -n migration-system`. No service restart required — read per request.

### Log Extraction Strategy

Applied to v2v-helper pod logs, controller pod logs, and each debug log file:

1. **All ERROR, FAILED, WARN lines** — no cap
2. **±10 lines of surrounding context** for each error line
3. **Last 200 lines** of each log source (final state regardless of level)
4. Deduplicate overlapping context windows

Controller pod logs: ERROR + WARN only (no last-200, too verbose).

Debug logs fetched server-side from `/debug-logs/migration-{name}/` — same nginx endpoint the UI uses for download today. At most **10 `.log` files** per migration directory are included; extraction (ERROR±10 + last 200 lines) limits each file's contribution.

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

**Result (confidence: high or medium):**
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

References: [vJailbreak Prerequisites ↗]  [virt-v2v DNS docs ↗]

────────────────────────────────────────────────────
[ Ask a follow-up question...              ] [Send]
```

**Result (confidence: none — AI cannot determine root cause):**
```
⚠ Could not determine root cause from available logs.

Before opening an issue, collect the following:
  □ Run `journalctl -u libvirtd` on vJailbreak VM and save output
  □ Collect /var/log/pf9/ debug logs (use Download button above)
  □ Note ESXi host version and vCenter version
  □ Check if CBT is enabled on source VM

[ Open GitHub Issue with pre-filled details ↗ ]

────────────────────────────────────────────────────
[ Ask a follow-up question...              ] [Send]
```

The GitHub Issue button opens `github.com/platform9/vjailbreak/issues/new` with pre-filled title (migration name + phase + error summary) and body (migration CR conditions, extracted error lines, vjailbreak version). User fills in the manually-collected items from the checklist before submitting.

Follow-up messages append to `conversation_history`. Re-clicking "Analyse with AI" resets history and re-runs initial analysis.

### Settings Page

New "AI Configuration" section:
- **Anthropic API Key** — password-type text field
- **Admin API Key** — password-type text field (authenticates `/crawl` and `/context` admin endpoints on vjailbreak-ai)
- Save → `POST /vpw/v1/ai/key` → creates/updates k8s Secret `vjailbreak-ai-secret` with `api-key` and `admin-key`
- Shows masked placeholder if key already configured

---

## Error Handling

| Condition | UI behavior |
|-----------|-------------|
| API key not configured | Tab shows: "Configure Anthropic API key in Settings" with link |
| vjailbreak-ai unreachable | "AI service unavailable. Check vjailbreak-ai deployment." |
| Anthropic API error (quota / 5xx) | Error message with Retry button |
| Migration phase ≠ Failed | "Analyse with AI" button hidden |
| Partial log fetch failure | Include available data; vpwned collects `fetch_warnings []string` and passes them to vjailbreak-ai, which injects a "## Fetch Warnings" section into the user message; do not block analysis |

---

## Testing

### vjailbreak-ai (Python)

- Unit: prompt construction — correct system prompt, RAG chunks injected, conversation history threaded correctly
- Unit: response parsing — structured fields (root_cause, fix_steps, confidence, github_issue) extracted correctly
- Unit: GitHub Issue URL builder — title/body encoding, checklist items populated
- Unit: RAG retrieval — error keywords from logs produce relevant doc chunks
- Mock: Anthropic API client, ChromaDB client

### vjailbreak-api handler (Go)

- Unit: log extraction logic — correct lines selected, context windows, deduplication
- Unit: CR assembly — credential refs stripped from MigrationTemplate
- Unit: excluded CRs (VMwareCreds, OpenstackCreds) never fetched
- Unit: debug log fetch and extraction (max 10 files cap enforced)
- Unit: fetch_warnings collected when pod/debug log fetch fails
- Unit: ai_key_handler GET (absent → configured=false, present → configured=true)
- Unit: ai_key_handler POST create and update paths
- Mock: k8s client, vjailbreak-ai HTTP client

### UI (React)

- Component: tab switching (Logs ↔ AI Analysis)
- Component: button hidden when migration not Failed
- Component: loading / result / error states
- Component: confidence=none path renders checklist + GitHub Issue button
- Component: GitHub Issue button opens correct pre-filled URL
- Component: follow-up chat flow
- Mock: `/vpw/v1/ai/analyze` endpoint

### Integration
- Fixture failed migration CR + sample logs → full analysis pipeline → verify response rendered in UI

---

## Out of Scope (v1)

- Streaming AI responses (show result when complete, not token-by-token)
- Saving/exporting AI analysis results
- Analysis for non-Failed migrations
- Multiple concurrent analyses
- Local LLM support
