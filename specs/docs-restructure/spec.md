# Feature Specification: vJailbreak Documentation Redesign & Restructure

**Feature Branch**: `docs-restructure`
**Created**: 2026-04-29
**Status**: Draft

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Fix Broken Links & Inaccurate Content (Priority: P1)

A user following the vJailbreak documentation encounters broken links or outdated information (e.g., the 404 GPO migration link, the broken cluster-conversion link, the "Coming Soon" APIs page), causing confusion and lost trust.

**Why this priority**: Broken links and incorrect content are the highest-friction issues — they actively block users and damage credibility. Every broken link is a failed user journey.

**Independent Test**: Navigate to each updated page, verify all links resolve correctly, and confirm content is accurate without needing other changes to be complete.

**Acceptance Scenarios**:

1. **Given** a user reads the FAQ "Can vJailbreak migrate Windows VMs with GPO applied?", **When** they click the GPO migration link, **Then** they are taken to a valid, accessible page (not a 404).
2. **Given** a user visits the cluster-conversion guide URL, **When** the page loads, **Then** the correct cluster conversion content is displayed (no 404 error).
3. **Given** a user visits "Use vjailbreak APIs", **When** the page loads, **Then** they see either complete API documentation or the page is removed and all references to it are cleaned up.

---

### User Story 2 - Accurate Known Limitations Reference (Priority: P1)

A user planning a migration needs to understand what vJailbreak cannot do or does not support so they can plan accordingly, avoid wasted effort, and set correct expectations.

**Why this priority**: Missing limitations cause failed migrations and support escalations. Users need a single authoritative place to check before committing to a migration plan.

**Independent Test**: A user can open the Known Limitations page and find a complete, accurate list of supported/unsupported scenarios without cross-referencing other pages.

**Acceptance Scenarios**:

1. **Given** a user wants to migrate a Windows 2008/2012 VM with network persistence, **When** they check the Known Limitations page, **Then** they see a clear entry stating "Persist Network does not work for Windows Server 2008/2012 and below".
2. **Given** a user plans to assign IPs and also enable persist network, **When** they check Known Limitations, **Then** they see a clear entry stating these two features cannot be used together.
3. **Given** a user has a multi-boot VM, **When** they check Known Limitations, **Then** they see an entry stating multi-boot VMs are not supported.
4. **Given** a user has a VM joined to Active Directory, **When** they check Known Limitations, **Then** they see a detailed entry describing known AD-related issues and behavior.
5. **Given** a user has VMs with LDM (Logical Disk Manager) partitions, **When** they check Known Limitations, **Then** they find the LDM limitation clearly documented (moved from its previous location).
6. **Given** a user plans to assign multiple IPs, **When** they check Known Limitations, **Then** they see an entry noting that multi-IP assignment is limited to preserving only one IP.
7. **Given** a user has low disk space on the migration host, **When** they check Known Limitations, **Then** they see a warning about minimum disk space requirements for virt-v2v-in-place operations.
8. **Given** a user's application cannot sustain a reboot during migration, **When** they check Known Limitations, **Then** they find guidance on the reboot requirement.

---

### User Story 3 - Accurate Prerequisites & Network Requirements (Priority: P2)

A user setting up vJailbreak for the first time reads the Prerequisites page and needs to understand exactly what network connectivity is required — without being given unnecessary requirements that no longer apply.

**Why this priority**: Outdated prerequisites cause users to perform unnecessary steps or misunderstand the deployment model (self-contained VM vs. external dependencies).

**Independent Test**: The Prerequisites network section can be reviewed and confirmed accurate independently by a user setting up a fresh vJailbreak deployment.

**Acceptance Scenarios**:

1. **Given** a user reads the "What network connectivity do I need?" section, **When** they review it, **Then** they see only the currently required connections (vCenter and OpenStack), with a note that additional connectivity is needed only when upgrading vJailbreak.
2. **Given** a user is upgrading vJailbreak (not fresh install), **When** they read the Prerequisites page, **Then** they find the upgrade-specific connectivity requirements clearly distinguished from installation requirements.

---

### User Story 4 - Troubleshooting Content is Findable (Priority: P2)

A user encounters a virt-v2v failure mentioning `/sysroot/etc/resolv.conf Operation not permitted` and searches the documentation for a solution.

**Why this priority**: Troubleshooting content buried in wrong sections is as bad as not having it — users give up and raise support tickets unnecessarily.

**Independent Test**: Navigate to the Troubleshooting section and find the resolv.conf entry; also verify FAQ and Known Issues pages contain links to it.

**Acceptance Scenarios**:

1. **Given** a user encounters the resolv.conf virt-v2v error, **When** they visit the Troubleshooting section, **Then** they find the resolution steps there (not only in FAQ or Prerequisites).
2. **Given** a user is reading the FAQ, **When** they scan for virt-v2v errors, **Then** they see a link pointing to the Troubleshooting entry for this error.
3. **Given** a user reads the Known Issues page, **When** they scan it, **Then** they see a reference linking to the Troubleshooting page for this resolv.conf error.

---

### User Story 5 - Hotplug Flavor Support Documented (Priority: P2)

A user wants to understand how vJailbreak handles OpenStack flavor assignment, including whether hotplug flavors are supported.

**Why this priority**: Hotplug support changes how users plan their migrations and target flavor selection.

**Independent Test**: The flavor documentation section can be reviewed independently to verify it covers both standard flavor matching and hotplug flavor behavior.

**Acceptance Scenarios**:

1. **Given** a user reads the flavor handling section, **When** they review it, **Then** they see a new subsection explaining hotplug flavor support, how it works, and any prerequisites or limitations.

---

### User Story 6 - CLI Migration Page Reflects Current CRDs (Priority: P2)

A user uses the "Use CLI to Migrate" page to craft migration commands, but the CRD field names or structure shown are outdated compared to the current release.

**Why this priority**: Stale CLI docs cause migration failures when users copy-paste examples with wrong field names.

**Independent Test**: Compare the CLI migration page content against the current release CRD definitions — all fields and examples should match.

**Acceptance Scenarios**:

1. **Given** a user copies a CLI example from the "Use CLI to Migrate" page, **When** they apply it against the current vJailbreak release, **Then** the command succeeds without field errors.
2. **Given** the current vJailbreak release has updated CRD fields, **When** a user reads the CLI page, **Then** all field names, types, and examples match the current CRD schema.

---

### User Story 7 - Settings Documentation Complete (Priority: P3)

A user wants to configure vJailbreak settings but cannot find documentation for all available settings in the current release.

**Why this priority**: Missing settings docs force users to reverse-engineer configuration from source or raise support tickets.

**Independent Test**: Compare documented settings against the current release's complete settings list — no settings should be undocumented.

**Acceptance Scenarios**:

1. **Given** a user reads the vJailbreak settings documentation, **When** they compare it to the current release, **Then** every available setting is documented with name, description, allowed values, and default.

---

### User Story 8 - FAQ Covers GPO and vTPM Migration (Priority: P3)

A user wants to know whether vJailbreak supports GPO-configured VMs and vTPM-enabled VMs before attempting migration.

**Why this priority**: These are common enterprise VM configurations that users frequently ask about.

**Independent Test**: The FAQ page can be checked independently for the presence and accuracy of GPO and vTPM entries with working links.

**Acceptance Scenarios**:

1. **Given** a user visits the FAQ, **When** they look for GPO migration, **Then** they find an entry with a working link to the GPO migration guide.
2. **Given** a user visits the FAQ, **When** they look for vTPM migration, **Then** they find an entry with a working link to the vTPM migration guide.

---

### Edge Cases

- What happens if the vTPM migration guide page does not yet exist — should the FAQ entry link to it or note it as "coming soon"?
- The "VMware Tools removal — add link to residual files page" assumes the residual files page exists and has a stable URL.
- The LDM doc move may leave orphan links from other pages that pointed to its original location — all inbound links must be updated.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The Prerequisites "network connectivity" section MUST be updated to list only currently required connections (vCenter and OpenStack endpoints), removing stale requirements.
- **FR-002**: The Prerequisites page MUST clearly distinguish upgrade-time network requirements from fresh-installation requirements.
- **FR-003**: The virt-v2v resolv.conf error resolution MUST be moved to (or duplicated in) the Troubleshooting section.
- **FR-004**: The FAQ and Known Issues pages MUST each contain a link to the Troubleshooting entry for the resolv.conf error.
- **FR-005**: The flavor handling section MUST include a new subsection documenting hotplug flavor support, including usage and any limitations.
- **FR-006**: The Known Limitations page MUST include an entry for VMs joined to Active Directory with a description of known issues.
- **FR-007**: The FAQ entry "Can vJailbreak migrate Windows VMs with GPO applied?" MUST have its link updated to the correct GPO migration page URL.
- **FR-008**: The Known Limitations page MUST include an entry stating Persist Network does not work for Windows Server 2012 and below.
- **FR-009**: The Known Limitations page MUST include an entry for VMware Tools removal with a link to the residual files documentation page.
- **FR-010**: The Known Limitations page MUST include documentation on hotplug flavor usage and requirements.
- **FR-011**: The Windows LDM limitation MUST be moved from its current location to the Known Limitations section, with a cross-reference link retained in its original location if needed.
- **FR-012**: The Known Limitations page MUST include an entry stating multi-boot VMs are not supported.
- **FR-013**: The Known Limitations page MUST include an entry stating multi-IP assignment preserves only one IP.
- **FR-014**: The Known Limitations page MUST include an entry about low disk space requirements for virt-v2v-in-place operations.
- **FR-015**: The Known Limitations page MUST include a note about application reboot requirements during migration.
- **FR-016**: The Known Limitations page MUST include an entry stating Assign IP and Persist Network cannot be used together.
- **FR-017**: The FAQ MUST include entries for GPO migration and vTPM migration, each with a working link to the respective guide.
- **FR-018**: The "Use CLI to Migrate" page MUST be updated to reflect current CRD field names, types, and examples matching the latest release.
- **FR-019**: The "Use vjailbreak APIs" page MUST either be completed with current API documentation OR removed with all inbound references cleaned up.
- **FR-020**: The vJailbreak settings documentation MUST cover all settings available in the current release, with name, description, allowed values, and defaults.
- **FR-021**: The cluster-conversion URL (`/guides/how-to/cluster-conversion/`) MUST resolve to the correct page without a 404 error.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Zero broken links on any documentation page updated as part of this effort (verified by link checker after changes).
- **SC-002**: All items in the provided change list are present and verifiable in the published documentation.
- **SC-003**: The Known Limitations page contains at least 8 clearly described limitation entries covering all items specified.
- **SC-004**: A user evaluating migration feasibility can find all known limitations in one page without navigating more than 2 clicks from the home page.
- **SC-005**: All CLI migration examples on the "Use CLI to Migrate" page work without modification against the current release.
- **SC-006**: Support ticket volume related to documented known limitations decreases over the 30 days following publication.

## Assumptions

- The residual VMware artifacts page already exists and has a stable, published URL that can be linked to.
- A vTPM migration guide page either already exists or will be created as part of this effort; if it does not yet exist, the FAQ entry will note it as forthcoming.
- The Active Directory limitation details from the linked Jira ticket (VJAILB-5) are accurate and approved for inclusion in public documentation.
- The "Use vjailbreak APIs" page will be removed (not completed) based on the "Coming Soon" status — this assumption requires confirmation.
- The LDM documentation currently exists somewhere in the docs and needs to be moved, not created from scratch.
- The cluster-conversion 404 is a navigation/routing issue, not missing content — the page exists but the link is broken.
- All documentation changes target the GitHub Pages documentation site (`platform9.github.io/vjailbreak`).
