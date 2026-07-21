---
name: vjailbreak-pr-review
description: Reviews vJailbreak GitHub PRs for logic correctness, coding principles, and adherence to the project constitution. Use this skill whenever reviewing a vjailbreak PR, auditing whether a PR follows project rules, or asked to "review PR 123", "check this PR", "look at PR number X" in the vjailbreak codebase. Invoke proactively any time a vJailbreak PR number appears alongside review intent, even if the user just says "can you take a look at #2150" or "does this PR look good".
---

# vjailbreak-pr-review

Reviews vJailbreak PRs for correctness, test coverage, and adherence to the project constitution and coding principles.

## Step 1: Brainstorm Before Reviewing

Before reading a single line of diff, invoke `superpowers:brainstorming` to think through:

- What kind of change is this? (feature, fix, refactor, CRD change, UI-only?)
- Which modules does it touch? (k8s/migration, v2v-helper, pkg/vpwned, pkg/common, ui/)
- What are the highest-risk areas given those modules?
- Which principles are most relevant to this change?

This prevents anchoring on surface-level issues and helps catch the right class of problems.

## Step 2: Fetch the PR

```bash
# Get PR metadata and description
gh pr view <number> --json title,body,author,files,additions,deletions,baseRefName,headRefName

# Get the full diff
gh pr diff <number>

# Get list of changed files
gh pr view <number> --json files --jq '.files[].path'
```

Read the PR description carefully — understanding intent helps distinguish bugs from intentional design.

## Step 3: Review Against Principles

Work through these checks. Severity is ordered: Critical > Important > Quality.

### Critical — NON-NEGOTIABLE (flag every violation)

These come from the project constitution and CLAUDE.md. Flag any violation regardless of how small.

**Generated file protection**
- `deploy/installer.yaml` must never be hand-edited — regenerate via `make generate-manifests`
- `zz_generated.deepcopy.go` files must never be hand-edited — regenerate via `make generate` in `k8s/migration/`
- If CRD types in `k8s/migration/api/v1alpha1/` changed, check that `make generate` was run (deepcopy files updated)

**Test coverage**
- Every modified Go file needs a corresponding `_test.go` — not just new files, also modified ones
- Unit tests must mock external dependencies (no live vCenter, OpenStack, or k8s API calls)
- v2v-helper tests require `CGO_ENABLED=1 GOOS=linux GOARCH=amd64` — flag if CI/Makefile target is missing this
- If untestable code exists (no interfaces, huge functions), check for refactor to enable testability

**Module independence**
- Four independent Go modules: `k8s/migration/`, `v2v-helper/`, `pkg/vpwned/`, `pkg/common/`
- Cross-module imports must use full module path
- No shared `go.sum` files across modules
- New dependencies: `go mod tidy` must be run in the specific module directory

### Important — Strong Violations (flag unless clearly intentional)

**Refactor scope**
- Refactor-as-You-Go applies to Go modules only — do NOT flag missing refactors for UI-only (`ui/`) changes

**Refactor correctness** (when a PR contains a refactor — restructuring for testability, extracting interfaces, splitting functions)
- Refactors must not change observable behavior: same inputs → same outputs, same side effects, same error conditions
- Check: does the refactored code handle all the original code paths? (nil guards, error branches, loop termination, boundary values)
- Check: did any function signature change that callers depend on (even if the PR updated callers)?
- Check: did extraction of a helper function accidentally change evaluation order, short-circuit logic, or variable scope?
- If a refactor introduces an interface, verify all implementations satisfy all the original behaviors, not just the happy path

**Branch hygiene**
- PRs should not be direct commits to main — check `baseRefName`

**Error handling**
- No error handling for scenarios that cannot happen — trust internal code and framework guarantees
- Only validate at system boundaries (user input, external APIs)

**Scope creep**
- PR should not add features, refactors, or improvements beyond what the title/description claims
- No docstrings, comments, or type annotations added to code the PR didn't change

### Code Quality — Flag Meaningful Issues

**Logic correctness**
- Nil/null pointer dereferences
- Race conditions (shared state, goroutine access without locks)
- Off-by-one errors, incorrect boundary conditions
- Error return values ignored when they shouldn't be
- Context cancellation not propagated

**Go-specific patterns**
- Table-driven tests preferred for multiple input/output cases
- Defer for cleanup (file handles, locks)
- Interface usage for external dependencies that need mocking
- Goroutine leaks (channels unbuffered where buffer needed, goroutines without done signals)

**Security**
- No hardcoded credentials, tokens, or secrets
- Input validation at system boundaries (user input, external API responses)
- Sensitive data not logged

**Simplicity**
- Three similar lines of code is better than a premature abstraction
- No helpers or utilities created for one-time operations
- No backwards-compatibility shims for things that can just be changed

## Step 4: Write the Report

Produce `review.md` with clean Markdown that renders well. Each finding gets its own named section separated by `---`. Do NOT use HTML comment markers (`<!-- -->`).

### review.md structure

```markdown
## PR #<number> Review — <title>

**Verdict**: APPROVE / REQUEST CHANGES — <one sentence: what the PR does and overall quality>

---

### Finding 1 · MINOR · `path/to/file.go:739`

<explanation of the problem — 2-4 sentences>

```go
// illustrative fix if helpful
```

Fix: <concrete action>

---

### Finding 2 · MISSING TEST · `path/to/other_test.go`

<explanation>

Fix: <concrete action>
```

Severity labels: `CRITICAL`, `LOGIC`, `MISSING TEST`, `QUALITY`, `MINOR`

If no issues found: write only the verdict paragraph. A clean PR needs no findings section.

### Formatting rules

- File references: `path/to/file.go:line` (plain backtick, never bold+backtick)
- Code suggestions: fenced Go/bash blocks
- Each finding: one clear location, one clear problem, one concrete fix

## Step 5: Generate the Posting Script

Produce `send_comments.sh` — one `gh pr comment` per finding, one `gh pr review` for the summary. Each block is labeled so the user can delete any section before running.

```bash
#!/bin/bash
# Review script for PR #<number>
# Delete any labeled block you don't want to post, then: bash send_comments.sh

PR=<number>
REPO=platform9/vjailbreak
VERDICT="--comment"  # change to "--request-changes" if blocking issues found

# ─── Summary ───────────────────────────────────────────────────────────
gh pr review $PR --repo $REPO $VERDICT --body "$(cat <<'EOF'
PR #<number> review — <title>

Verdict: APPROVE / REQUEST CHANGES

<summary paragraph — same content as review.md verdict>
EOF
)"

# ─── Finding 1: MINOR · path/to/file.go:739 ───────────────────────────
# Delete this block to skip
gh pr comment $PR --repo $REPO --body "$(cat <<'EOF'
`path/to/file.go:739` — <short title>

<full explanation>

Fix: <concrete action>
EOF
)"

# ─── Finding 2: MISSING TEST · path/to/other_test.go ──────────────────
# Delete this block to skip
gh pr comment $PR --repo $REPO --body "$(cat <<'EOF'
...
EOF
)"
```

Use `--request-changes` only for Critical violations or blocking logic bugs.

## Step 6: Save Outputs and Launch Eval Viewer

Save the review outputs to the workspace and open the eval viewer in the browser.

```bash
# Create timestamped run directory
WORKSPACE="$HOME/.claude/plugins/marketplaces/claude-plugins-official/plugins/vjailbreak/skills/vjailbreak-pr-review-workspace"
RUN_DIR="$WORKSPACE/pr-<number>/$(date +%Y%m%d_%H%M%S)/outputs"
mkdir -p "$RUN_DIR"

# Write review.md and send_comments.sh (content from Steps 4 and 5)
cat > "$RUN_DIR/review.md" << 'MDEOF'
<review.md content>
MDEOF

cat > "$RUN_DIR/send_comments.sh" << 'SHEOF'
<send_comments.sh content>
SHEOF
chmod +x "$RUN_DIR/send_comments.sh"

# Write eval metadata so viewer shows PR URL as prompt
cat > "$(dirname $RUN_DIR)/eval_metadata.json" << 'JSONEOF'
{"prompt": "Review PR #<number>: https://github.com/platform9/vjailbreak/pull/<number>"}
JSONEOF

# Launch eval viewer (opens browser automatically)
EVAL_VIEWER="$HOME/.claude/plugins/marketplaces/claude-plugins-official/plugins/skill-creator/skills/skill-creator"
cd "$EVAL_VIEWER"
python3 eval-viewer/generate_review.py "$WORKSPACE" --skill-name vjailbreak-pr-review &
```

The viewer starts a local HTTP server and opens `http://localhost:3117` in the browser automatically. The review.md will render as formatted Markdown with syntax-highlighted code blocks. Use the **Post to GitHub** section to post findings directly from the UI.

## Reference: Module Paths

| Module | Directory | Test command |
|--------|-----------|-------------|
| Controller | `k8s/migration/` | `cd k8s/migration && make test` |
| V2V Helper | `v2v-helper/` | `make test-v2v-helper` (requires Linux CGO) |
| API Server | `pkg/vpwned/` | standard `go test ./...` |
| Common | `pkg/common/` | standard `go test ./...` |
| UI | `ui/` | `yarn test` — no Go rules apply |

## Reference: Constitution Principles

| # | Principle | Check |
|---|-----------|-------|
| III | Generated Code Protection | No hand-edits to installer.yaml or zz_generated.deepcopy.go |
| IV | Test-First Development | Unit tests with mocked deps for every touched Go file |
| V | Module Independence | Separate go.sum, full module paths for cross-module imports |
| VII | Code Reuse and Simplicity | No premature abstractions; duplication OK below threshold |
