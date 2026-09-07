# API Contract: Debug Prompt Endpoint

**Date**: 2026-09-07 | **Branch**: `2303-debug-skill-hub`

## Endpoint

```
GET /sdk/vpw/v1/ai/debug-prompt
```

**Authentication**: Same as all vpwned endpoints (existing middleware, no change).

**Response**: `200 OK`, `Content-Type: application/json`

```json
{
  "version": "1.0.0",
  "last_updated": "2026-09-07",
  "prompt": "<full plain-text system prompt with subagents inlined>"
}
```

**Error responses**:

| Status | Condition |
|--------|-----------|
| `500 Internal Server Error` | Embedded prompt file missing or unparseable at startup |

**Side effects**: None. Endpoint is read-only and stateless.

---

## UI Component Contract

### Page: `DebugWithAIPage`

**Route**: `/help/debug-ai`

**Props**: None (fetches data internally via `GET /sdk/vpw/v1/ai/debug-prompt`).

**Tabs**:

| Tab | Content |
|-----|---------|
| Claude Code | Step-by-step install instructions + skill repository link + current version badge |
| Other Agents | Prompt text area (read-only) + "Copy" button + version badge |

**Copy behavior**: Clicking "Copy System Prompt" copies `response.prompt` to clipboard. Button text changes to "Copied!" for 2 seconds, then reverts.

**Version badge**: Displays `v{response.version}` from the API response. If the API is unreachable, badge shows "Version unavailable" — page still renders with static Claude Code instructions.

**Navigation**: Accessible from the main nav under a "Help" section. No authentication gate (same access level as other UI pages).

---

## Build-Time Artifact Contract

### `scripts/build-debug-prompt.sh`

**Input**: `.claude/skills/vjb-debug/SKILL.md` (or equivalent path)

**Output**: `pkg/vpwned/server/debug_prompt_embedded.txt`

**Transformations**:
1. Strip YAML frontmatter block (lines between first `---` and second `---`)
2. Extract `version:` and `last_updated:` values for embedding in the handler
3. Replace subagent dispatch patterns with inline checklist steps
4. Output clean markdown suitable for pasting into any AI chat tool

**Exit codes**:
- `0` — success
- `1` — skill file not found
- `2` — frontmatter missing required fields (`version`, `last_updated`)

---

## CI Freshness Check Contract

### `make check-skill-freshness`

**Input**: `SKILL_WATCHED_FILES` (Makefile variable listing watched paths)

**Logic**:
1. For each watched file, get its last git commit date: `git log -1 --format=%ci -- <file>`
2. Parse `last_updated:` from SKILL.md frontmatter
3. If any watched file's commit date > `last_updated`, exit 1 with message:
   `SKILL STALE: <file> modified after skill last-updated date. Bump version and last-updated in SKILL.md.`

**Exit codes**:
- `0` — skill is current
- `1` — skill is stale (one or more watched files modified after `last_updated`)
- `2` — SKILL.md or watched file missing
