# Research: Debug with AI - Skill Distribution Hub

**Date**: 2026-09-07 | **Branch**: `2303-debug-skill-hub`

## Decision Log

### 1. Skill File Distribution Mechanism

**Decision**: Serve the system prompt as a static GET endpoint from vpwned (`GET /vpw/v1/ai/debug-prompt`). The prompt is baked into the binary at build time by embedding the generated file via Go `embed`.

**Rationale**: The system prompt changes only when the skill file changes (which triggers a new container build). There is no need for live-reload. Go `embed` is already used elsewhere in the codebase and requires zero new infra.

**Alternatives considered**:
- Serve the raw skill file from a K8s ConfigMap — rejected because ConfigMaps require RBAC and add operational complexity.
- Generate the prompt at request time by reading the skill file from disk — rejected because it creates a runtime dependency on the filesystem layout and complicates container builds.

---

### 2. System Prompt Generation (Subagent Inlining)

**Decision**: A bash script (`scripts/build-debug-prompt.sh`) runs at build time. It strips SKILL.md YAML frontmatter, converts subagent dispatch calls (lines matching `Agent tool` or `subagent_type`) into inline numbered checklist steps, and outputs `pkg/vpwned/server/debug_prompt_embedded.txt`. The Go handler embeds this file.

**Rationale**: The conversion is mechanical (~50 lines). Doing it at build time means the served prompt is always consistent with the skill file. No runtime dependencies.

**Alternatives considered**:
- Hand-maintain a separate system prompt document — rejected because it diverges from the skill over time.
- Use a Go library to parse markdown at runtime — rejected as overkill for a static asset.

---

### 3. Skill Freshness CI Gate

**Decision**: Add a `check-skill-freshness` Make target. It computes a SHA256 of the watched file list, then checks if the skill's `last-updated:` field in SKILL.md is newer than the newest watched file's git commit date. If not, it exits non-zero. CI calls this target in the `build-vpwned` job (which already builds pkg/vpwned/).

**Watched files** (declared in `Makefile` variable `SKILL_WATCHED_FILES`):
```
k8s/migration/api/v1alpha1/migration_types.go
k8s/migration/api/v1alpha1/migrationplan_types.go
v2v-helper/virtv2v/virtv2vops.go
pkg/vpwned/server/ai_handler.go
```

**Rationale**: Tying the check to existing `build-vpwned` CI job means it runs on every PR without a new workflow file. The watched file list is in Makefile (version-controlled), not hardcoded in CI YAML.

**Alternatives considered**:
- Separate CI job — rejected as unnecessary overhead for a lightweight check.
- Check on every file change in the repo — rejected as too noisy; only behavioral/interface files matter.

---

### 4. UI Page Placement

**Decision**: New route `/help/debug-ai` added to `App.tsx`. Navigation entry "Debug with AI" added to `navigation.tsx` under a "Help" group (or appended to existing nav if no group exists). Page uses MUI `Tabs` component (already used throughout the UI) for Claude Code / Other Agents tabs.

**Rationale**: Matches existing UI patterns. No new component library needed.

**Alternatives considered**:
- Modal dialog on the failed migration drawer — rejected because the spec explicitly chose global help page (User said option B, not A).
- Separate docs site — rejected because it can't serve the dynamically-versioned system prompt.

---

### 5. Skill Version Stamp

**Decision**: Add `last-updated: YYYY-MM-DD` and `version: x.y.z` fields to the SKILL.md frontmatter (YAML block at top of file). The UI displays the version returned by the `GET /vpw/v1/ai/debug-prompt` endpoint.

**Rationale**: Frontmatter is already the convention for Claude skill files. Adding two fields is non-breaking.

**Alternatives considered**:
- Separate VERSION file — rejected as a second file to keep in sync.
- Git tag on the skill file — rejected as too complex for contributors.
