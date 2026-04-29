# Implementation Plan: vJailbreak Documentation Redesign & Restructure

**Branch**: `docs-restructure` | **Date**: 2026-04-29 | **Spec**: [spec.md](./spec.md)

## Summary

Restructure and update the vJailbreak public documentation to fix broken links, add a new Known Limitations page, move content to correct sections, update CLI migration examples to match current CRDs, and add missing FAQ entries. All changes are content-only edits to Markdown/MDX files in the existing Astro + Starlight documentation site.

## Technical Context

**Content Format**: Markdown (`.md`) with Astro/Starlight frontmatter  
**Site Framework**: Astro 5 + `@astrojs/starlight` 0.32  
**Content Root**: `docs/src/content/docs/`  
**Navigation Config**: `docs/astro.config.mjs` (sidebar defined statically + `autogenerate`)  
**Storage**: File system — one `.md` file per documentation page  
**Testing**: Manual link verification + `astro build` for compile-time errors  
**Target Platform**: GitHub Pages at `https://platform9.github.io/vjailbreak/`  
**Project Type**: Documentation site  
**Performance Goals**: All pages load; zero build errors  
**Constraints**: No new frameworks, no restructuring of Astro project layout; use existing page style (frontmatter + Markdown headings, `:::note`/`:::tip`/`:::caution` callouts)

## Constitution Check

This feature modifies only documentation Markdown files — no Go code, no TypeScript, no Kubernetes controllers. The constitution principles apply as follows:

| Principle | Applicability | Assessment |
|-----------|--------------|------------|
| I. Interface-First Design | N/A — no code | ✅ Pass |
| II. Modular Boundaries | N/A — no code | ✅ Pass |
| III. Test-Driven Quality | Documentation has no automated tests; manual link verification is the equivalent gate | ✅ Pass (manual verify) |
| IV. Clean and Explicit Code | Applies as clear, well-organized prose; no unexplained jargon | ✅ Pass |
| V. Controller-Runtime Discipline | N/A — no controllers | ✅ Pass |
| VI. Observability by Default | N/A | ✅ Pass |
| VII. Simplicity First | Add only what was specified; no new pages beyond what's required; no navigation restructuring beyond adding the Known Limitations entry | ✅ Pass |

**Gate result: PASS** — proceed to Phase 1.

## Project Structure

### Documentation (this feature)

```text
specs/docs-restructure/
├── plan.md              # This file
├── research.md          # Phase 0 output
└── tasks.md             # Phase 2 output (/speckit-tasks command)
```

### Files Modified or Created

```text
docs/src/content/docs/
├── introduction/
│   ├── prerequisites.md          # MODIFY: Trim network section; split upgrade vs. install
│   └── faq.md                    # MODIFY: Fix GPO link; add resolv.conf cross-ref; add vTPM/GPO FAQ; add flavor hotplug subsection
├── reference/
│   └── known-limitations.md      # CREATE NEW: central Known Limitations page
├── guides/
│   ├── Troubleshooting/
│   │   └── troubleshooting.md    # MODIFY: Add resolv.conf entry (moved from FAQ); update LDM cross-ref
│   ├── How-to/
│   │   └── vjailbreak_settings.md  # MODIFY: Verify completeness against latest release settings
│   └── CLI-API/
│       ├── migrating_using_cli_and_kubectl.md  # MODIFY: Update CRD examples to current schema
│       └── using_apis.md         # MODIFY: Remove "Coming Soon" content and redirect (or delete)
└── concepts/
    └── cluster-conversion.md     # MODIFY: Fix broken internal link to cluster-conversion guide
docs/astro.config.mjs             # MODIFY: Add Known Limitations to sidebar
```

## Phase 0: Research

### R-001: APIs Page Decision

**Question**: Should `using_apis.md` be removed entirely, or completed with current API documentation?

**Finding**: The page contains only `Coming Soon!` and has been in this state. The CLI migration page already links to it. The user's question ("what do we need to add?") suggests uncertainty. Based on spec Assumption 4 (remove), the plan defaults to **removing the page** and cleaning up references.

**Action**: Remove `using_apis.md` and update the one inbound link in `migrating_using_cli_and_kubectl.md`. Update the `astro.config.mjs` sidebar if the guide section for CLI-API is manually listed.

**Decision**: Remove page. If the team wants to add API docs later, that is a separate feature.
**Rationale**: Empty "Coming Soon" pages degrade documentation quality and generate 404s when referenced. A clean removal is better than an incomplete stub.
**Alternatives considered**: Completing the API docs — rejected because the required content (full API reference from swagger) would take significant effort and is out of scope.

---

### R-002: Cluster-Conversion 404 Root Cause

**Finding**: The `concepts/cluster-conversion.md` page links to `../../guides/how-to/cluster-conversion/`. The actual file lives at `guides/Cluster-Conversion/cluster-conversion.md`. Astro Starlight generates slugs from directory paths, so the correct URL is `/vjailbreak/guides/cluster-conversion/cluster-conversion/` (case-insensitive slug from directory `Cluster-Conversion`). The `how-to/cluster-conversion/` path does not exist.

**Action**: Update the link in `concepts/cluster-conversion.md` from `../../guides/how-to/cluster-conversion/` to `../../guides/cluster-conversion/cluster-conversion/`.

**Decision**: Fix the path in the concepts page. No file moves required.

---

### R-003: CRD Schema Drift in CLI Migration Page

**Finding by comparing `migrating_using_cli_and_kubectl.md` against current Go types**:

| Field | Current Doc | Current CRD | Fix |
|-------|------------|-------------|-----|
| `MigrationTemplate.spec.osFamily` | `linuxGuest` / `windowsGuest` | `linux` / `windows` (enum) | Update examples |
| `MigrationTemplate.spec.source.datacenter` | Present in doc | **Removed from struct** | Remove from examples |
| `MigrationPlan.spec.retry` | Not shown | `retry: bool` | Add to full reference |
| `MigrationPlan.spec.firstBootScript` | Not shown | `firstBootScript: string` | Add to full reference |
| `MigrationPlan.spec.postMigrationAction` | Not shown | `postMigrationAction: {renameVm, suffix, moveToFolder, folderName}` | Add to full reference |
| `MigrationPlan.spec.advancedOptions` | Not shown | `advancedOptions: {granularVolumeTypes, granularNetworks, granularPorts}` | Add to full reference |
| `MigrationPlanStrategy.dataCopyStart` | Not shown | `dataCopyStart: datetime` | Add to full reference |
| `MigrationPlanStrategy.vmCutoverStart/End` | Not shown | Present | Add to full reference |

**Action**: Update `migrating_using_cli_and_kubectl.md` YAML examples with corrected field values and add documentation for new optional fields.

---

### R-004: Current Settings Completeness Check

**Finding**: The `vjailbreak_settings.md` page documents 20 settings. Cross-referencing against the codebase is required during implementation to verify completeness. Based on the current ConfigMap in `install.yaml` and controller startup, these settings appear current as of v0.4.3.

**Action**: Verify during implementation by checking the controller's settings loading code. If any settings are missing, add them.

---

### R-005: Known Limitations — Content Sources

Content for the new Known Limitations page comes from:
1. The `windows-dynamic-disk-ldm-migration-issue.md` troubleshooting page (LDM — move summary, keep full detail in Troubleshooting with a link)
2. User-provided items (AD, Persist Network, multi-boot, multi-IP, hotplug flavors, disk space, reboot, assign-IP+persist-network)
3. Release notes v0.4.3 (Windows static IP → DHCP conversion, Windows 2008 network persistence, agent scaling on L2)
4. The `network-persistence.md` concepts page (Windows 2008/2012 limitation)

**Format decision**: Known Limitations page will use an `:::caution` or `:::note` callout per limitation group, with clear headings. Cross-links to Troubleshooting pages where detailed remediation exists.

---

## Phase 1: Design

### File-by-File Change Map

#### 1. `reference/known-limitations.md` (NEW FILE)

**Sections** (each as a `##` heading with a brief description and any relevant links):

- **Windows Dynamic Disk (LDM)** — summary + link to `guides/Troubleshooting/windows-dynamic-disk-ldm-migration-issue/`
- **Active Directory-Joined VMs** — describe known post-migration domain behavior; source: VJAILB-5
- **Persist Network: Windows 2008/2012 Not Supported** — static network persistence not supported on Windows Server 2012 and below
- **Assign IP and Persist Network Cannot Be Used Together** — mutually exclusive options
- **Multi-IP Assignment: Only One IP Preserved** — multi-IP input preserves only the first IP
- **VMware Tools Removal: Residual Artifacts** — link to `guides/Troubleshooting/vmware_residual_artifacts/`
- **Multi-Boot VMs Not Supported** — virt-v2v limitation
- **Hotplug Flavor Requirements** — how to use hotplug-capable OpenStack flavors with vJailbreak
- **Low Disk Space for virt-v2v-in-place** — minimum free space requirement
- **Application Reboot During Migration** — cold migration causes a reboot; applications must tolerate it

**Frontmatter**:
```yaml
---
title: Known Limitations
description: Known limitations and unsupported configurations in vJailbreak
---
```

**Sidebar placement**: Under `Reference` section in `astro.config.mjs`, after `vJailbreak CRDs`.

---

#### 2. `introduction/prerequisites.md` (MODIFY)

**Change**: Split the "What network connectivity do I need?" section into two sub-sections:
- **For migration (always required)**: vCenter, ESXi, OpenStack endpoints + NFC port 902
- **For initial installation or upgrades only**: Everything else (container registries, k3s sources, helm, cloud-init, etc.)

Add a note: "During migration, vJailbreak only requires connectivity to vCenter, ESXi hosts, and the OpenStack API. The additional endpoints listed below are needed only when installing or upgrading vJailbreak itself."

---

#### 3. `introduction/faq.md` (MODIFY)

Changes:
1. **Fix GPO link**: Change `../guides/how-to/gpo_migration.md` → `../../guides/how-to/gpo_migration/`
2. **Add resolv.conf cross-reference**: After the existing resolv.conf answer, add "See also: [Troubleshooting: virt-v2v resolv.conf error](../../guides/troubleshooting/troubleshooting/)"
3. **Add hotplug flavor subsection**: Under "How does vJailbreak handle flavors...", add a `####` heading "Hotplug Flavor Support" describing the feature
4. **Add GPO FAQ entry**: "How do I migrate a Windows VM with GPO applied?" → link to `guides/how-to/gpo_migration/`
5. **Add vTPM FAQ entry**: "How do I migrate a VM with vTPM enabled?" → link to `guides/how-to/vtpm_migration/`

---

#### 4. `guides/Troubleshooting/troubleshooting.md` (MODIFY)

Changes:
1. **Move resolv.conf content from FAQ**: Add a new `## virt-v2v fails with rename: /sysroot/etc/resolv.conf Operation not permitted` section duplicating (or linking to) the FAQ content. The FAQ entry will be simplified to a brief description + link here.
2. **Add LDM cross-reference**: The existing link to `windows-dynamic-disk-ldm-migration-issue/` stays; add a note "See also: [Known Limitations: LDM](../../reference/known-limitations/)"

---

#### 5. `guides/CLI-API/migrating_using_cli_and_kubectl.md` (MODIFY)

Changes per R-003:
1. Update `MigrationTemplate` example: remove `datacenter` field, change `osFamily: linuxGuest` → `osFamily: linux`
2. Add documentation for optional MigrationPlan fields: `retry`, `firstBootScript`, `postMigrationAction`, `advancedOptions`, `dataCopyStart`, `vmCutoverStart`, `vmCutoverEnd`
3. Remove reference link to `using_apis.md` (or update if APIs page is replaced with redirect)

---

#### 6. `guides/CLI-API/using_apis.md` (MODIFY → Remove content / redirect)

Replace content with a redirect note pointing to the CLI migration guide and noting that a full API reference is forthcoming. Do not delete the file to avoid breaking the slug — instead replace body with a brief note and a link. This preserves the URL without a 404.

Alternative approach: Delete the file and remove it from the sidebar. Since the Guide section uses `autogenerate`, removing the file is sufficient.

**Decision**: Delete the file and clean up the one inbound link in `migrating_using_cli_and_kubectl.md`.

---

#### 7. `concepts/cluster-conversion.md` (MODIFY)

Fix the link: `../../guides/how-to/cluster-conversion/` → `../../guides/cluster-conversion/cluster-conversion/`

---

#### 8. `docs/astro.config.mjs` (MODIFY)

Add Known Limitations to the Reference sidebar:
```js
{
  label: 'Reference',
  items: [
    { label: 'vJailbreak CRDs', slug: 'reference/reference' },
    { label: 'Compatibility Matrix', slug: 'reference/compatibility' },
    { label: 'Known Limitations', slug: 'reference/known-limitations' },
  ],
},
```

---

### Dependency Order

```
1. Create known-limitations.md          (no deps)
2. Fix cluster-conversion.md link       (no deps)
3. Fix using_apis.md (delete)           (no deps)
4. Update prerequisites.md             (no deps)
5. Update faq.md                        (depends on: troubleshooting section exists for resolv.conf link)
6. Update troubleshooting.md            (no deps; faq can link here after)
7. Update migrating_using_cli_and_kubectl.md  (no deps from other changes)
8. Update astro.config.mjs             (depends on: known-limitations.md created)
9. Verify vjailbreak_settings.md        (verify only, no deps)
```

---

## Complexity Tracking

No constitution violations.
