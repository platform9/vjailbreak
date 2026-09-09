# Tasks: Debug with AI - Skill Distribution Hub

**Input**: Design documents from `specs/2303-debug-skill-hub/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/

**Tests**: Included — Constitution Principle IV mandates unit tests for all new Go code and Principle XI mandates Playwright E2E for all new UI pages.

**Organization**: Tasks grouped by user story for independent implementation and testing.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: User story label (US1, US2, US3)

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Prepare the skill file and build script scaffold that all phases depend on.

- [X] T001 Add `version: 1.0.0` and `last-updated: 2026-09-07` fields to SKILL.md frontmatter in `.claude/skills/vjb-debug/SKILL.md` (or equivalent path if different; search repo for the skill file first)
- [X] T002 Create `scripts/build-debug-prompt.sh`: (a) read `.claude/skills/vjb-debug/SKILL.md` **and every file it references via relative markdown links** (e.g. `[copy-methods.md](copy-methods.md)`, `[migration-lifecycle.md](migration-lifecycle.md)`, etc.) — resolve each link relative to the skill directory and inline its content in place; (b) strip the YAML frontmatter block (first `---` … second `---`); (c) emit two metadata header lines as the first two lines of output: `version: <value>` and `last_updated: <YYYY-MM-DD>` (underscore form — mapped from YAML `last-updated:` key); (d) scan for subagent dispatch patterns (`subagent_type:` keys and `Agent(` call signatures in fenced code blocks) and replace each with an inline numbered checklist step; (e) write output to `pkg/vpwned/server/debug_prompt_embedded.txt`; exit 1 if skill file missing, exit 2 if required frontmatter fields (`version`, `last-updated`) absent. Also add a `generate-debug-prompt` target to `Makefile` that runs `scripts/build-debug-prompt.sh`, and add `generate-debug-prompt` as a prerequisite of the `build-vpwned` target so `debug_prompt_embedded.txt` always exists before `go build ./...` runs

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Backend endpoint and embedded prompt — required before US1 and US2 UI work can be validated end-to-end.

**⚠️ CRITICAL**: US1 UI work can start in parallel (it needs no API for the Claude Code tab), but US2 and all E2E tests require this phase complete.

- [X] T003 Write Go unit tests FIRST for `pkg/vpwned/server/debug_prompt_handler.go` in `pkg/vpwned/server/debug_prompt_handler_test.go`: test GET returns 200 with JSON body containing `version`, `last_updated`, and `prompt` fields; test non-GET method returns 405; run `cd pkg/vpwned && go test ./server/... -run TestDebugPrompt` — must FAIL before T004
- [X] T004 [P] Implement `pkg/vpwned/server/debug_prompt_handler.go`: embed `debug_prompt_embedded.txt` using Go `//go:embed`; parse `version` and `last_updated` from the first two lines of the embedded file; serve JSON `DebugPromptResponse{Version, LastUpdated, Prompt}` on GET; return 405 on other methods
- [X] T005 Register `GET /vpw/v1/ai/debug-prompt` route in `pkg/vpwned/server/server.go` following the existing `mux.Handle` pattern (see line ~236 in server.go)
- [X] T006 Run `cd pkg/vpwned && go test ./server/... -run TestDebugPrompt` — tests from T003 must now PASS

---

## Phase 3: User Story 1 - Claude Code Install Page (Priority: P1) 🎯 MVP

**Goal**: End users can navigate to `/help/debug-ai` and follow instructions to install vjb-debug skill with Claude Code.

**Independent Test**: Navigate to `/help/debug-ai` in the UI; the "Claude Code" tab renders with install instructions, repository link, and version badge — no API key or vjailbreak-ai service required.

### Tests for User Story 1

> **Write tests FIRST — ensure they FAIL before implementing the page**

- [X] T007 [P] Write Playwright E2E spec `ui/e2e/help/debug-with-ai.spec.ts`: test that `/help/debug-ai` route renders; "Claude Code" tab is active by default; install instructions are visible; repository link is present; stub `GET /dev-api/sdk/vpw/v1/ai/debug-prompt` with `page.route()` returning `{version:"1.0.0", last_updated:"2026-09-07", prompt:"test prompt"}`; run `cd ui && yarn pw:run ui/e2e/help/debug-with-ai.spec.ts` — must FAIL
- [X] T008 [P] Write Vitest unit test `ui/src/features/help/pages/DebugWithAIPage.test.tsx`: test that `DebugWithAIPage` renders "Claude Code" tab as default; version badge shows mocked version; "Other Agents" tab exists; run `cd ui && yarn test` — must FAIL

### Implementation for User Story 1

- [X] T009 [P] [US1] Add `/help/debug-ai` route to `ui/src/App.tsx` following the existing route registration pattern (see routes around line 468-490)
- [X] T010 [P] [US1] Add "Debug with AI" navigation entry to `ui/src/config/navigation.tsx` under a "Help" section (or append to existing nav if no help group)
- [X] T011 [US1] Create `ui/src/features/help/pages/DebugWithAIPage.tsx`: MUI `Tabs` component with "Claude Code" tab (default active) and "Other Agents" tab; Claude Code tab contains: install instructions (numbered steps), link to `.claude/skills/vjb-debug/` in the public repo, version badge showing `v{version}` fetched from `GET /dev-api/sdk/vpw/v1/ai/debug-prompt` (gracefully degrades to "Version unavailable" if API unreachable)
- [X] T012 [US1] Run Playwright E2E (T007) and Vitest unit test (T008) — both must now PASS

**Checkpoint**: `/help/debug-ai` loads, Claude Code tab shows install instructions and version badge, page works even when backend is unavailable.

---

## Phase 4: User Story 2 - Other Agents System Prompt Tab (Priority: P2)

**Goal**: Users with any AI tool (ChatGPT, Gemini, etc.) can copy the full system prompt from the UI in one click.

**Independent Test**: On `/help/debug-ai`, click "Other Agents" tab; a read-only text area shows the system prompt; "Copy System Prompt" button copies it to clipboard; button text changes to "Copied!" for 2 seconds.

### Tests for User Story 2

> **Write tests FIRST — ensure they FAIL before implementing**

- [X] T013 [P] Add to `ui/e2e/help/debug-with-ai.spec.ts` (extend T007 spec): test that clicking "Other Agents" tab shows prompt text; "Copy System Prompt" button exists and clicking it changes label to "Copied!" for 2 seconds; prompt text matches the mocked API response's `prompt` field
- [X] T014 [P] Add to `ui/src/features/help/pages/DebugWithAIPage.test.tsx` (extend T008): test copy button calls `navigator.clipboard.writeText` with prompt text; button label reverts after timeout. Additionally, add unit tests for `ui/src/api/ai/debugPrompt.ts` (Constitution Principle XI — pure utility requires Vitest coverage): mock `fetch`, assert the fetch function calls `/dev-api/sdk/vpw/v1/ai/debug-prompt`, parses `{version, last_updated, prompt}` from a successful response, and rejects/throws on a non-200 status — either inline in the same test file or in a sibling `ui/src/api/ai/debugPrompt.test.ts`. Run tests — must FAIL

### Implementation for User Story 2

- [X] T015 [P] [US2] Create `ui/src/api/ai/debugPrompt.ts`: typed fetch function for `GET /dev-api/sdk/vpw/v1/ai/debug-prompt` returning `DebugPromptResponse {version, last_updated, prompt}`; follows existing pattern in `ui/src/api/ai/aiAnalysis.ts`
- [X] T016 [US2] Add "Other Agents" tab content to `DebugWithAIPage.tsx`: read-only MUI `TextField` or `Typography` block showing `response.prompt`; "Copy System Prompt" MUI `Button` that calls `navigator.clipboard.writeText(prompt)`, changes to "Copied!" for 2s then reverts; loading and error states handled
- [X] T017 [US2] Run E2E and unit tests (T013, T014) — both must now PASS

**Checkpoint**: Both tabs functional; system prompt copyable; page fully works for Claude Code and Other Agents users.

---

## Phase 5: User Story 3 - CI Freshness Gate (Priority: P3)

**Goal**: CI fails when a watched source file changes but the skill version stamp is not bumped.

**Independent Test**: Modify a watched file in a test branch without bumping `last-updated` in SKILL.md; run `make check-skill-freshness` — must exit non-zero with a clear message.

### Implementation for User Story 3

*(No separate test tasks — the CI gate itself is the test. Verified by running `make check-skill-freshness` with a stale skill.)*

- [X] T018 [P] [US3] Add `SKILL_WATCHED_FILES` variable and `check-skill-freshness` target to `Makefile`: for each file in the list, get its last git commit date via `git log -1 --format=%ci -- <file>`; compare to `last-updated:` in SKILL.md frontmatter; exit 1 with message `SKILL STALE: <file> modified after skill last-updated date. Bump version and last-updated in SKILL.md.` if any file is newer; initial watched file list per research.md:
  - `k8s/migration/api/v1alpha1/migration_types.go`
  - `k8s/migration/api/v1alpha1/migrationplan_types.go`
  - `v2v-helper/virtv2v/virtv2vops.go`
  - `pkg/vpwned/server/ai_handler.go`
- [X] T019 [P] [US3] Add `skill-freshness` step to the `build-vpwned` job in `.github/workflows/packer.yml`: run `make check-skill-freshness` before the build step; this ensures every PR touching watched files is gated
- [X] T020 [P] [US3] Add PR template checkbox to `.github/pull_request_template.md` (create file if absent): `- [ ] I checked whether this change requires a vjb-debug skill update (see SKILL.md last-updated)`
- [X] T021 [US3] Verify freshness check: manually run `make check-skill-freshness` with current state — should PASS (T001 set last-updated to today); then set `last-updated` to a past date in SKILL.md, run again — should FAIL with correct message; restore correct date

**Checkpoint**: CI blocks stale skill; PR template reminds contributors; freshness enforced without manual process.

---

## Phase 6: Polish & Cross-Cutting Concerns

- [X] T022 [P] Run `cd pkg/vpwned && go build ./...` to confirm the new handler compiles with the embed directive and generates no lint warnings
- [X] T023 [P] Run full UI test suite `cd ui && yarn test` and `cd ui && yarn pw:run` to confirm no regressions in existing tests
- [X] T024 Add a brief entry to the vJailbreak docs site or repo README pointing to `/help/debug-ai` so users can discover the debug skill hub

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — start immediately
- **Foundational (Phase 2)**: Depends on T001 (SKILL.md must have version stamp before build script runs); T002 must complete before T004 can embed the output
- **US1 (Phase 3)**: T009, T010, T011 can start immediately (Claude Code tab is static); T007, T008 tests written first (TDD); T012 validation depends on T011
- **US2 (Phase 4)**: T015, T016 depend on T005 (endpoint registered); T013, T014 tests written before T015, T016
- **US3 (Phase 5)**: T018, T019, T020 are fully independent of UI phases; can start after T001 (version stamp format established)
- **Polish (Phase 6)**: Depends on all story phases complete

### User Story Dependencies

- **US1 (P1)**: Static tab — can start immediately after Phase 1. No dependency on backend endpoint for Claude Code tab content.
- **US2 (P2)**: Depends on Phase 2 complete (backend endpoint serves the prompt); extends the page built in US1.
- **US3 (P3)**: Independent of UI — can be worked in parallel with US1/US2 after T001.

### Within Each User Story

- Tests written and verified to FAIL before implementation (Constitution Principle IV)
- UI API layer (T015) before UI component integration (T016)
- Route/nav (T009, T010) before page component (T011) — page must be reachable

### Parallel Opportunities

- T001 and setup work are serial prerequisites
- T003 (Go tests) and T007 (Playwright tests) and T008 (Vitest tests) can be written in parallel after T001
- T004 and T002 can run in parallel (different files)
- T009, T010, T011 (UI routing/nav/page) can run in parallel
- T018, T019, T020 (Makefile/CI/PR template) can all run in parallel

---

## Parallel Example: US1 + US3 concurrently

```bash
# Developer A: US1 UI work
Task: T007 - Write Playwright E2E test (fails)
Task: T008 - Write Vitest unit test (fails)
Task: T009, T010, T011 - Route + nav + page (tests now pass)

# Developer B: US3 CI work (fully independent)
Task: T018 - Makefile check-skill-freshness target
Task: T019 - packer.yml CI step
Task: T020 - PR template
Task: T021 - Verify check manually
```

---

## Implementation Strategy

### MVP First (User Story 1 Only — ~1 day)

1. Complete Phase 1: T001, T002
2. Complete Phase 2: T003–T006 (backend endpoint with embed)
3. Complete Phase 3: T007–T012 (Claude Code tab, route, nav)
4. **STOP and VALIDATE**: visit `/help/debug-ai`, follow install instructions, confirm skill installs
5. Ship — users with Claude Code can debug immediately at zero Platform9 API cost

### Incremental Delivery

1. MVP (US1) → Claude Code users unblocked
2. US2 → Other agent users unblocked (extend same page)
3. US3 → Skill stays fresh as codebase evolves

### Parallel Team Strategy

With two developers after Phase 1+2:
- Dev A: US1 + US2 (UI page, both tabs)
- Dev B: US3 (Makefile, CI, PR template)

---

## Notes

- [P] tasks = different files, no shared state — safe to parallelize
- [Story] label maps each task to its user story for traceability
- Constitution Principle IV: all Go tests written BEFORE implementation; verified to fail first
- Constitution Principle XI: Playwright E2E for `/help/debug-ai` page; Vitest for component units
- `page.route()` stubs REQUIRED in Playwright spec — no real vpwned calls in tests
- Version stamp format in SKILL.md: `version: x.y.z` and `last-updated: YYYY-MM-DD` in YAML frontmatter block
- Go `//go:embed` directive in `debug_prompt_handler.go` requires `debug_prompt_embedded.txt` to exist at compile time — `scripts/build-debug-prompt.sh` must run before `go build`; wire this into Makefile `build-vpwned` target
