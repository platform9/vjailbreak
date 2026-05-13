---
name: "doc-coauthoring"
description: "Edit and improve public-facing GitHub Pages documentation for the repository, with emphasis on clear structure, readability, and customer-safe public content."
argument-hint: "Describe the GitHub Pages documentation task, section, or public docs area to improve."
compatibility: "Applicable to repositories publishing public documentation through GitHub Pages or static docs sites."
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

1. Detect the public docs publishing surface for this repository:
   - `docs/src/content/docs` and other docs site content used by GitHub Pages
   - `docs/README.md`, `docs/guides/`, `docs/reference/`, `docs/release_docs/`
   - Top-level `README.md` only if it is also part of published public-facing documentation
   - `AGENTS.md` only when it documents public usage or supported workflows
2. Confirm the request is for user-facing documentation, not internal developer docs.
3. Enforce customer-safe public content:
   - Avoid any customer names, internal project names, or proprietary operational details.
   - Keep examples generic and abstract.

## Outline

1. Focus on public GitHub Pages documentation.
   - Prefer content under `docs/` and the docs site entry points.
   - Only update top-level repo docs when they are part of the published public documentation flow.
2. Structure content for clarity and scanability.
   - Use clear section headings: Overview, What it does, Prerequisites, Installation, Usage, Examples, Troubleshooting, References.
   - Use short paragraphs, numbered steps, bullet lists, tables, and callout-style notes.
   - Avoid long dense paragraphs.
3. Organize public docs for fast discovery.
   - Prefer clear top-level nav categories such as Getting Started, Guides, Reference, Troubleshooting, and Release Notes.
   - Keep documentation pages shallow and focused: a page should solve one user question or task.
   - Use labels in the navigation panel that match user intent, not internal code names.
   - Ensure key information is reachable within two clicks from the docs home or side navigation.
4. Make the public docs customer-friendly.
   - Write in simple, direct language.
   - Avoid jargon unless defined clearly.
   - Keep the tone factual, concise, and action-oriented.
5. Keep content detailed but to the point.
   - Include concrete commands and examples only when they help the reader complete a task.
   - Prefer general guidance over overly specific environmental details.
6. Apply standard documentation best practices.
   - Add summaries and quick start guidance for new users.
   - Use consistent terminology and phrase structure.
   - Validate that each section has a purpose and a clear next step.
   - Maintain a public/consumer perspective rather than an internal engineering viewpoint.
6. Preserve repository-specific doc conventions.
   - Match existing markdown styling, heading levels, and path references.
   - Keep the docs site structure intact.

## Expected behavior

- Edit only public-facing documentation that will appear on GitHub Pages or the published docs site.
- Make the docs clearer, more structured, and easier for customers to follow.
- Organize content so users can reach key topics quickly from the navigation panel.
- Do not add internal or customer-specific details.
- Prefer generic examples and standard documentation conventions.
- If the request is too broad, ask a targeted follow-up before editing large public docs sections.
