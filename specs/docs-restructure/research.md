# Research: vJailbreak Documentation Restructure

## R-001: APIs Page — Remove or Complete?

**Decision**: Remove `using_apis.md`.  
**Rationale**: The page has contained only "Coming Soon!" with no progress toward completion. Empty stub pages actively harm documentation quality. The one inbound link (from `migrating_using_cli_and_kubectl.md`) will be removed.  
**Alternatives considered**: Completing with Swagger/API docs — out of scope; insufficient information available without a separate research effort.

---

## R-002: Cluster-Conversion 404 Root Cause

**Decision**: Fix inbound link in `concepts/cluster-conversion.md`.  
**Root cause**: Link used `how-to/cluster-conversion/` path but the file lives at `guides/Cluster-Conversion/cluster-conversion.md`, generating slug `/guides/cluster-conversion/cluster-conversion/`.  
**Correct link**: `../../guides/cluster-conversion/cluster-conversion/`

---

## R-003: CRD Schema Drift

**Decision**: Update `migrating_using_cli_and_kubectl.md` examples to current CRD schema.

Summary of confirmed drifts (from Go type inspection):

| Location | Issue | Fix |
|----------|-------|-----|
| MigrationTemplate example | `osFamily: linuxGuest` | Change to `osFamily: linux` |
| MigrationTemplate example | `spec.source.datacenter` field present | Remove — field no longer exists in struct |
| MigrationPlan example | Missing optional fields | Document `retry`, `firstBootScript`, `postMigrationAction`, `advancedOptions` |
| MigrationPlanStrategy | Missing time fields | Document `dataCopyStart`, `vmCutoverStart`, `vmCutoverEnd` |

---

## R-004: Settings Page Completeness

**Status**: The `vjailbreak_settings.md` page documents 20 settings matching the current ConfigMap structure seen in the controller source. No missing settings identified from source inspection. Page is current as of v0.4.3.  
**Action**: No changes required beyond verifying during implementation pass.

---

## R-005: Known Limitations — Content Consolidation

Content map for the new `reference/known-limitations.md` page:

| Limitation | Source | Notes |
|------------|--------|-------|
| Windows LDM (Dynamic Disk) | `guides/Troubleshooting/windows-dynamic-disk-ldm-migration-issue.md` | Summary only; link to full troubleshooting page |
| Active Directory-Joined VMs | VJAILB-5 Jira ticket + team input | Describe domain re-join behavior post-migration |
| Persist Network: Windows 2008/2012 | v0.4.3 release notes | Not supported on Windows Server 2012 and below |
| Assign IP + Persist Network conflict | User-provided | Cannot be used together |
| Multi-IP assignment: one IP preserved | User-provided | Only first IP preserved in multi-IP scenarios |
| VMware Tools residual artifacts | `guides/Troubleshooting/vmware_residual_artifacts.md` | Link to page |
| Multi-boot VMs not supported | User-provided | virt-v2v limitation |
| Hotplug flavor requirements | User-provided | Describe how to use hotplug-capable flavors |
| Low disk space for virt-v2v-in-place | User-provided | Recommend minimum free space |
| Application reboot requirement | User-provided | Cold migration causes reboot |

---

## R-006: GPO Link Fix

**Finding**: The current FAQ links to `../guides/how-to/gpo_migration.md` — this uses a `.md` extension which doesn't work for Astro Starlight slugs, and the relative path is wrong.  
**Correct URL**: `../../guides/how-to/gpo_migration/` (matches file at `guides/How-to/gpo_migration.md`, slug `guides/how-to/gpo_migration`)

---

## All NEEDS CLARIFICATION markers resolved

None were present in the spec. All 6 research questions above are resolved.
