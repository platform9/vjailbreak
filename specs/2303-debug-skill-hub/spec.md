# Feature Specification: Debug with AI - Skill Distribution Hub

**Feature Branch**: `2303-debug-skill-hub`
**Created**: 2026-09-07
**Status**: Draft
**Input**: User description: "Expose vjb-debug skill to end users via global help/docs page in vJailbreak UI, with Claude Code install guide and plain markdown system prompt export for other agents, plus CI freshness enforcement"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Claude Code User Installs Debug Skill (Priority: P1)

A vJailbreak operator or user has a failed migration and wants to debug it using their own Claude Code. They navigate to the vJailbreak UI help page, find the "Debug with AI" section, follow the install instructions, and use the vjb-debug skill with their own Claude Code and API key — at no cost to Platform9.

**Why this priority**: The highest-value path. Users who already have Claude Code or Claude Pro can get superior AI-assisted debugging with zero API cost to Platform9. This is the primary distribution channel for the skill.

**Independent Test**: Can be fully tested by visiting the help page, following the Claude Code tab instructions, installing the skill, and successfully invoking it on a failed migration CR.

**Acceptance Scenarios**:

1. **Given** a user navigates to the vJailbreak UI help page, **When** they click "Debug with AI", **Then** they see a page with instructions for Claude Code users including how to install/access the vjb-debug skill from the repository.
2. **Given** a user is on the Claude Code tab, **When** they follow the instructions, **Then** they can invoke the skill on a failed migration using their own Claude API key without any cost to Platform9.
3. **Given** the skill has a version stamp, **When** the user compares their installed version to the version shown in the UI, **Then** they can determine if their local copy is outdated.

---

### User Story 2 - Non-Claude Agent User Gets System Prompt (Priority: P2)

A user wants to debug a failed migration using a non-Claude AI agent (e.g., ChatGPT, Gemini, a local LLM). They navigate to the same help page, switch to the "Other Agents" tab, copy the pre-built system prompt, paste it into their preferred AI tool, and follow the structured debugging checklist embedded in the prompt.

**Why this priority**: Extends skill value to users who don't use Claude Code. The system prompt encodes the same structured reasoning the skill uses — triage, log investigation, behavior lookup — as a numbered checklist any agent can follow.

**Independent Test**: Can be fully tested by copying the system prompt, pasting it into ChatGPT or Gemini, and asking it to analyze a failed migration (providing logs manually). The checklist in the prompt guides the session.

**Acceptance Scenarios**:

1. **Given** a user is on the "Other Agents" tab, **When** they click "Copy System Prompt", **Then** the full plain-text system prompt (with subagent reasoning inlined as a checklist) is copied to their clipboard.
2. **Given** a user pastes the system prompt into any AI chat tool, **When** they describe their failed migration, **Then** the AI follows the structured triage process embedded in the prompt.
3. **Given** the system prompt is served by the vJailbreak backend, **When** the skill is updated with a new version, **Then** the served prompt reflects the latest version automatically (no manual UI update required).

---

### User Story 3 - Operator Checks Skill Freshness (Priority: P3)

A Platform9 developer modifies a CRD type, log format, or controller behavior. The CI pipeline detects that a watched file changed and fails the build if the skill's version stamp has not been bumped, preventing the skill from going stale relative to the codebase.

**Why this priority**: Without this, the skill's quality degrades silently as the codebase evolves. Users would get incorrect debugging guidance months after a major change. CI enforcement is the mechanism that makes the skill trustworthy long-term.

**Independent Test**: Can be tested by modifying a watched file in a CI run without bumping the skill version — the build must fail. Separately, bumping the version must allow the build to pass.

**Acceptance Scenarios**:

1. **Given** a developer modifies a watched file (CRD type, log format constant, controller behavior), **When** they submit a PR without updating the skill version, **Then** CI fails with a clear message identifying the stale skill.
2. **Given** a developer bumps the skill version stamp after a code change, **When** CI runs, **Then** the freshness check passes.
3. **Given** a PR template is filled out, **When** the developer makes a breaking change, **Then** the template includes a checkbox reminding them to evaluate whether the skill needs updating.

---

### Edge Cases

- What happens when a user's installed skill version is ahead of the UI-displayed version (e.g., they cloned main)? The UI should display the latest released version, not HEAD.
- What happens when the vjailbreak-ai service is unavailable? The debug help page must still load and be usable (it does not depend on vjailbreak-ai at all).
- What happens when a user has no Claude Code or Claude subscription? The "Other Agents" path must work as a complete fallback requiring no Anthropic account.
- What happens when the skill references a subagent that non-Claude agents can't invoke? The system prompt must contain all reasoning inline — no external dependencies.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The vJailbreak UI MUST include a global "Debug with AI" help page accessible from the main navigation, independent of any specific migration's state.
- **FR-002**: The help page MUST provide a "Claude Code" tab with step-by-step instructions for installing or accessing the vjb-debug skill from the public repository.
- **FR-003**: The help page MUST provide an "Other Agents" tab with a ready-to-copy plain text system prompt that encodes the skill's full structured reasoning process as a numbered checklist.
- **FR-004**: The "Copy System Prompt" button MUST copy the prompt to the user's clipboard in a single click.
- **FR-005**: The system prompt MUST be served by the vJailbreak backend so that when the skill is updated, the served prompt updates automatically without any UI code change.
- **FR-006**: The skill MUST carry a version stamp (e.g., `version: x.y.z`) visible both in the skill file and on the UI help page, so users can compare their local copy to the current version.
- **FR-007**: The CI pipeline MUST include a freshness check that fails when a watched file changes but the skill version stamp is not bumped.
- **FR-008**: The list of watched files triggering the freshness check MUST be explicitly declared and version-controlled (not hardcoded in CI script logic).
- **FR-009**: The PR template MUST include a checkbox reminding contributors to evaluate whether a code change requires a skill update.
- **FR-010**: The system prompt served to non-Claude users MUST inline all multi-step reasoning previously handled by Claude Code subagents (triage, log investigation, behavior lookup) as a self-contained numbered checklist, with no external tool dependencies.
- **FR-011**: The help page MUST NOT require the vjailbreak-ai API key or service to be configured — it must be fully functional regardless of AI service availability.

### Key Entities

- **vjb-debug Skill**: The Claude Code skill file (SKILL.md) residing in the repository. Has a version stamp, referenced reasoning steps, and spawns subagents in Claude Code context.
- **System Prompt**: A plain text representation of the skill's reasoning, auto-generated at build time from the skill file. Metadata stripped, subagent calls inlined as checklist steps.
- **Skill Version**: A semantic version string embedded in the skill file header. Used by CI for freshness enforcement and displayed in the UI for user reference.
- **Watched File List**: An explicit, version-controlled list of source files whose changes require a skill version bump (CRD types, log format constants, controller behavior files).

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A user with no prior knowledge of the vjb-debug skill can find the "Debug with AI" help page and complete the Claude Code install path in under 5 minutes.
- **SC-002**: A user with no Claude subscription can copy the system prompt and get structured debugging guidance from any AI chat tool in under 2 minutes.
- **SC-003**: Zero additional API cost is incurred by Platform9 for each user who uses the Claude Code or Other Agents path (users supply their own API credentials).
- **SC-004**: CI catches 100% of skill freshness violations: any PR that modifies a watched file without bumping the skill version fails the build before merge.
- **SC-005**: The system prompt served by the backend is always consistent with the skill file in the repository — no manual sync step required after a skill update.
- **SC-006**: The help page loads successfully even when the vjailbreak-ai service is unavailable or unconfigured.

## Assumptions

- The vjb-debug skill file is already public in the vJailbreak repository and licensed under BSL 1.1 — no additional legal review needed for distribution via the UI.
- Users of the Claude Code path are assumed to have Claude Code installed and either a Claude Pro subscription or an Anthropic API key.
- The "Other Agents" system prompt assumes the user will manually provide migration context (logs, CR YAML) to their chosen AI tool — no automated log injection is in scope for this feature.
- The vJailbreak backend (vpwned) serves the system prompt as a static endpoint baked into the container at build time from the skill file — live-reloading from the cluster filesystem is out of scope.
- Skill version stamps follow semantic versioning (x.y.z). The CI freshness check compares the last-modified date of watched files against the skill file's `last-updated` field — not a database or external registry.
- The subagent-inlining converter (skill metadata strip + subagent call → checklist) is a build-time script (~50 lines) run in CI, not a runtime process.
- Mobile support for the help page is out of scope for v1; the page is designed for desktop browser use.
- The feature does not modify or replace the existing AIAnalysisTab (managed AI analysis using vjailbreak-ai). Both paths coexist: the managed path for users who have API keys configured, the skill distribution path for users who prefer self-service.
