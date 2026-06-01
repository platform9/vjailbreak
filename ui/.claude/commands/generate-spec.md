# Generate Feature Specification & E2E Test Index

You are a senior engineer tasked with generating a detailed technical specification
for the **$ARGUMENTS** feature in this codebase. This spec will serve two purposes:

1. Onboard Claude (or any engineer) instantly when $ARGUMENTS-related changes are needed
2. Drive a comprehensive E2E test case suite

---

## Instructions — follow this sequence strictly

### Step 1 — Locate the feature

Search the codebase for all files related to the **$ARGUMENTS** feature:

- Match by folder name, file name, component name, and route path
- List every file found with its path and a one-line description
- Confirm the feature boundary with me before proceeding

Do not read file contents yet. Output the file list and wait for my approval.

---

### Step 2 — Deep codebase audit (no writing yet)

After approval, read every file in the $ARGUMENTS feature. For each file, extract:

- Component name, responsibility, and line count
- Props interface and their types
- Internal state shape and what drives state changes
- Events emitted / callbacks exposed
- API calls made: endpoint, method, payload shape, response shape
- Validation rules (field-level and form-level)
- Error states and how they are surfaced to the user
- Loading states and skeleton/fallback UI
- User flows: what sequence of actions does this component enable
- Dependencies on other features, shared components, or global state

Output the audit summary and wait for my approval before writing any files.

---

### Step 3 — Generate the specification (after audit approval)

Write a markdown file saved as `docs/specs/$ARGUMENTS-feature.md` with these sections:

#### 1. Feature overview

- Purpose and business context of the $ARGUMENTS feature
- High-level user journey (start to finish)
- Key actors and entry points

#### 2. Architecture map

- Module/folder structure with one-line description per file
- Component hierarchy tree (parent → child relationships)
- Data flow diagram in text form (what feeds what)
- Shared dependencies (contexts, stores, hooks used across the feature)

#### 3. Component specifications

For every component, document:

- Purpose (one sentence)
- Props (name, type, required/optional, description)
- Internal state (name, type, initial value, what triggers change)
- User interactions (action → what happens)
- Rendered output variants (default, loading, error, empty)
- Edge cases and known constraints

#### 4. API contract

For every API call in the feature:

- Endpoint and HTTP method
- Request payload (field, type, required/optional)
- Success response shape
- Error response shape and codes
- How errors are handled in the UI
- Any polling, retry, or timeout logic

#### 5. Validation rules

- Every form field with its validation rules listed explicitly
- Cross-field validation dependencies
- When validation triggers (on blur, on submit, on change)
- Error message copy for each rule

#### 6. State and data flow

- Global state slices used (Redux/Zustand/Context — whatever applies)
- How state is initialized, mutated, and cleaned up
- Any derived state or memoized selectors
- State reset conditions (on cancel, on complete, on error)

#### 7. User flows (for E2E test generation)

Document every user flow as a numbered sequence:

- Flow name
- Preconditions
- Step-by-step user actions
- Expected system response at each step
- Success condition
- Failure conditions and fallback behavior

Include flows for:

- Happy path (full successful $ARGUMENTS flow)
- Validation failures
- API errors at each stage
- Cancellation and resumption
- Edge cases (empty states, single item, maximum items)

#### 8. Known constraints and assumptions

- Browser/environment requirements
- Feature flags or conditional behavior
- Performance considerations
- Accessibility requirements

---

### Step 4 — E2E test case index

After the spec is approved, generate a second file saved as
`docs/specs/$ARGUMENTS-e2e-test-index.md` with:

For every user flow in Section 7, produce a test case entry:

- Test case ID (e.g. FEAT-001 where FEAT is a short uppercase prefix for $ARGUMENTS)
- Test name
- Preconditions
- Test steps (numbered — action + expected result per step)
- Pass criteria
- Fail criteria
- Priority (critical / high / medium / low)
- Tags (happy-path, error-handling, validation, edge-case)

Group test cases by category:

- Smoke tests (must pass before any release)
- Happy path tests
- Validation tests
- Error handling tests
- Edge case tests

---

## Output files

- `docs/specs/$ARGUMENTS-feature.md` — full technical specification
- `docs/specs/$ARGUMENTS-e2e-test-index.md` — E2E test case index

Start with Step 1. List all files related to the $ARGUMENTS feature and wait
for my approval before reading any file contents.
