---
name: "doc-coauthoring"
description: "Collaborate on documentation authoring and editing workflows for the repository."
argument-hint: "Describe the documentation task, file, or section you want to author or improve."
compatibility: "Applicable to repositories with markdown documentation, readme files, docs directories, or developer guides."
metadata:
  author: "github-spec-kit"
  source: "claude.ai/directory/skills/doc-coauthoring"
user-invocable: true
disable-model-invocation: false
---

## User Input

```text
$ARGUMENTS
```

You **MUST** consider the user input before proceeding (if not empty).

## Pre-Execution Checks

1. Detect key documentation surfaces in the repository:
   - `README.md`, `AGENTS.md`, `CLAUDE.md`, `docs/`, `templates/`, `ui/README.md`, `v2v-helper/README.md`, etc.
   - Existing markdown style, headers, bullet formatting, and code block conventions.
2. Determine the user's goal from the input:
   - Add or improve a user-facing guide.
   - Clarify architecture or workflow docs.
   - Create README or onboarding content.
3. If the user names a specific file, focus on that file and related cross-references.

## Outline

1. Locate the documentation target.
   - If the request is broad, identify the highest-impact docs section to update.
   - If the request is specific, open and analyze the relevant markdown file.
2. Preserve repository style.
   - Use the same heading depth, language tone, and formatting.
   - Maintain existing conventions for sections, lists, tables, and code examples.
3. Generate or expand content with clarity.
   - Use concise technical prose.
   - Include examples, commands, or step-by-step instructions where appropriate.
   - Add summary and purpose at the top of new sections.
4. Cross-link relevant repository files.
   - Reference related docs, scripts, or configuration paths when helpful.
   - Avoid speculative statements about implementation details.
5. Provide a short change summary.
   - Note which documents were updated.
   - Highlight the key improvements or additions.

## Expected behavior

- Focus on accuracy and readability.
- Do not alter unrelated code or configuration files.
- If the user's request is underspecified, ask a clarifying question before editing major docs.
