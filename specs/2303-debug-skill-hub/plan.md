# Implementation Plan: Debug with AI - Skill Distribution Hub

**Branch**: `2303-debug-skill-hub` | **Date**: 2026-09-07 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `specs/2303-debug-skill-hub/spec.md`

## Summary

Add a global "Debug with AI" help page to the vJailbreak UI that distributes the vjb-debug skill to end users. Claude Code users get install instructions pointing to the existing skill in the repo. Other-agent users get a one-click copyable plain text system prompt generated at build time from the skill file, served by the existing vpwned API server. A CI freshness gate prevents the skill from going stale when watched source files change.

## Technical Context

**Language/Version**: Go (vpwned), TypeScript/React (UI) — existing stack  
**Primary Dependencies**: controller-runtime, MUI, Vite — existing  
**Storage**: None — system prompt is static, baked into vpwned container at build time  
**Testing**: Vitest (UI unit), Playwright (UI E2E), `go test` (vpwned handler)  
**Target Platform**: vJailbreak appliance (k3s) + browser (desktop)  
**Project Type**: web-service (vpwned endpoint) + web-application (UI page)  
**Performance Goals**: Help page loads in under 2s; prompt copy is instant  
**Constraints**: Zero new API cost to Platform9; page must load even when vjailbreak-ai is unavailable  
**Scale/Scope**: Single new GET endpoint, single new UI page, one CI job step

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-checked after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| III - Generated Code Protection | PASS | No CRD changes; no installer.yaml edits |
| IV - Test-First Development | REQUIRED | vpwned handler needs Go unit test; UI page needs Playwright E2E + Vitest unit test |
| V - Module Independence | PASS | vpwned is its own module; UI is separate; no cross-module imports |
| VIII - Migration Field Parity | N/A | No migration form field added |
| IX - UI K8s Access | PASS | `/sdk/vpw/v1/ai/debug-prompt` is a static endpoint in vpwned, not a K8s resource read — no RBAC change needed |
| X - Upgrade Flow Parity | N/A | No new Deployment or container image; new endpoint added to existing vpwned binary only |
| XI - UI Test Coverage | REQUIRED | New UI page = Playwright E2E required; any pure util/helper = Vitest required |

**Gate result**: PASS with test obligations. No violations.

## Project Structure

### Documentation (this feature)

```text
specs/2303-debug-skill-hub/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── contracts/           # Phase 1 output
│   └── debug-prompt-api.md
└── tasks.md             # Phase 2 output (/speckit-tasks command)
```

### Source Code (repository root)

```text
# Backend (vpwned — Go module at pkg/vpwned/)
pkg/vpwned/server/
├── debug_prompt_handler.go         # NEW: GET /vpw/v1/ai/debug-prompt handler
├── debug_prompt_handler_test.go    # NEW: Go unit tests for handler
└── server.go                       # MODIFY: register new route

# Build scripts
scripts/
└── build-debug-prompt.sh           # NEW: converts skill file → plain system prompt

# Skill file (already exists, add version stamp)
.claude/
└── skills/vjb-debug/SKILL.md       # MODIFY: add version: x.y.z header field

# CI
.github/workflows/
└── packer.yml                      # MODIFY: add skill-freshness check step

# Makefile
Makefile                            # MODIFY: add check-skill-freshness target

# UI (at ui/)
ui/src/
├── App.tsx                         # MODIFY: add /help/debug-ai route
├── config/navigation.tsx           # MODIFY: add "Debug with AI" nav entry
└── features/help/
    └── pages/
        ├── DebugWithAIPage.tsx     # NEW: help page with Claude Code + Other Agents tabs
        └── DebugWithAIPage.test.tsx # NEW: Vitest unit tests

ui/e2e/help/
└── debug-with-ai.spec.ts           # NEW: Playwright E2E spec
```

## Complexity Tracking

> No constitution violations — section not applicable.

---

## Phase 0: Research

*All NEEDS CLARIFICATION items resolved. No external research required — "use existing code/infra/techstack" directive applied throughout.*

See [research.md](research.md).

---

## Phase 1: Design

See [data-model.md](data-model.md) and [contracts/debug-prompt-api.md](contracts/debug-prompt-api.md).
