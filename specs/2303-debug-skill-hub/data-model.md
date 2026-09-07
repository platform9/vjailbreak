# Data Model: Debug with AI - Skill Distribution Hub

**Date**: 2026-09-07 | **Branch**: `2303-debug-skill-hub`

## Entities

### SkillMetadata

Parsed from the SKILL.md frontmatter at build time. Not stored at runtime — embedded in binary.

| Field | Type | Description |
|-------|------|-------------|
| `version` | string (semver) | Skill version, e.g., `1.0.0` |
| `last_updated` | string (ISO date) | Date skill was last updated, e.g., `2026-09-07` |
| `description` | string | One-line skill description |

### DebugPromptResponse

JSON response body returned by `GET /sdk/vpw/v1/ai/debug-prompt`.

| Field | Type | Description |
|-------|------|-------------|
| `version` | string | Skill version from SKILL.md frontmatter |
| `last_updated` | string | ISO date from SKILL.md frontmatter |
| `prompt` | string | Full plain-text system prompt (subagents inlined as checklist) |

### WatchedFileList

Declared in `Makefile` as `SKILL_WATCHED_FILES`. Not a runtime entity — used only by the CI freshness check.

| Entry | Description |
|-------|-------------|
| `k8s/migration/api/v1alpha1/migration_types.go` | Migration CRD type definitions |
| `k8s/migration/api/v1alpha1/migrationplan_types.go` | MigrationPlan CRD type definitions |
| `v2v-helper/virtv2v/virtv2vops.go` | V2V conversion operations (log format, error codes) |
| `pkg/vpwned/server/ai_handler.go` | AI analysis context assembly (log extraction logic) |

## State Transitions

No stateful entities. The system prompt is immutable per container version. The skill version monotonically increases with each skill update.

## Validation Rules

- `version` in SKILL.md MUST follow semver format `MAJOR.MINOR.PATCH`
- `last_updated` MUST be a valid ISO 8601 date
- The freshness check MUST fail if any watched file's last git-commit date is newer than `last_updated`
- The `prompt` field MUST NOT contain YAML frontmatter (stripped at build time)
- The `prompt` field MUST NOT reference Claude Code subagent dispatch (converted to checklist at build time)
