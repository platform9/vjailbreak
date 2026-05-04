---
name: "mcp-builder"
description: "Build or update an MCP-compatible project workflow and generate Model Context Protocol artifacts."
argument-hint: "Describe the MCP build task, target artifact, or repository integration need."
compatibility: "Applicable to repositories with Model Context Protocol / MCP workflows, agent metadata, or .claude/MCP integration."
metadata:
  author: "github-spec-kit"
  source: "claude.ai/directory/skills/mcp-builder"
user-invocable: true
disable-model-invocation: false
---

## User Input

```text
$ARGUMENTS
```

You **MUST** consider the user input before proceeding (if not empty).

## Pre-Execution Checks

1. Determine repository root and detect MCP-related files:
   - `.claude/mcp.yml`, `mcp.yaml`, `mcp.json`, `.mcp/`, `.specify/`, `AGENTS.md`, or any `Model Context Protocol` references.
   - Detect existing skill or agent metadata such as `.claude/skills/` and `.specify/extensions.yml`.
2. If the user specifies a build target, focus the output on that target and preserve unrelated files.
3. If there is no MCP metadata present, prepare a minimal, correct integration plan rather than inventing broad repo changes.

## Outline

1. Identify the MCP workflow scope.
   - If MCP support already exists, review its configuration, agents, and hook definitions.
   - If MCP support is absent, determine the smallest useful artifact set to add (e.g. `.claude/mcp.yml`, AGENTS metadata, `.specify/extensions.yml`).
2. Validate the target repository conventions.
   - Prefer existing `.claude/` and `.specify/` formats.
   - Align generated artifacts with the repository's current agent / documentation style.
3. Generate or update MCP artifacts.
   - If generating new content, create clear, minimal MCP config and helper metadata.
   - If updating existing content, preserve current structure and add only the required MCP schema or hooks.
4. Provide actionable guidance when MCP cannot be fully inferred.
   - Explain how to wire the repository to MCP tooling.
   - List expected files, metadata fields, and references.
5. Keep the change limited to MCP support and avoid broad unrelated refactors.

## Expected behavior

- Prefer `MCP` file names and metadata conventions over generic scaffolding.
- Use repository-relevant paths and file names when suggesting or creating artifacts.
- If the repository already includes `.claude/skills/`, do not overwrite unrelated skill definitions.
- Document the new or updated artifacts in a concise summary.
