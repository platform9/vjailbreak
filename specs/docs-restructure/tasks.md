# Tasks: vJailbreak Documentation Redesign & Restructure

**Input**: Design documents from `specs/docs-restructure/`
**Prerequisites**: plan.md ✅, spec.md ✅, research.md ✅

**Tests**: Not applicable — documentation feature; verification is manual link checks and `astro build`.

**Organization**: Tasks grouped by user story. Most tasks touch different files and can run in parallel across stories. Tasks touching the same file are sequential within their phase.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies on incomplete tasks)
- **[Story]**: Which user story this belongs to (US1–US8)
- All paths relative to `docs/src/content/docs/` unless noted otherwise

---

## Phase 1: Setup

**Purpose**: Confirm the Astro docs build is clean before making changes.

- [x] T001 Verify docs build passes with `cd docs && npm ci && npm run build` — resolve any pre-existing errors before starting content changes

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Create the Known Limitations page stub and wire it into navigation. Required early because later phases add cross-links to it.

**⚠️ CRITICAL**: T003 depends on T002. Both must complete before cross-links in Phases 4–10 are valid.

- [x] T002 Create `docs/src/content/docs/reference/known-limitations.md` with frontmatter, section headings for all 10 limitations, and placeholder content for each (will be filled in Phase 4)
- [x] T003 Add `{ label: 'Known Limitations', slug: 'reference/known-limitations' }` to the Reference sidebar items array in `docs/astro.config.mjs` (depends on T002)

**Checkpoint**: `astro build` succeeds; Known Limitations page renders at `/vjailbreak/reference/known-limitations/`

---

## Phase 3: User Story 1 — Fix Broken Links & Inaccurate Content (Priority: P1) 🎯 MVP

**Goal**: Zero broken links on all touched pages; remove the empty "Coming Soon" APIs stub.

**Independent Test**: Click every link in the modified pages; all resolve without 404. Verify `astro build` passes with no broken slug warnings.

- [x] T004 [P] [US1] Fix cluster-conversion internal link in `docs/src/content/docs/concepts/cluster-conversion.md`: change `../../guides/how-to/cluster-conversion/` → `../../guides/cluster-conversion/cluster-conversion/`
- [x] T005 [P] [US1] Delete `docs/src/content/docs/guides/CLI-API/using_apis.md` (empty "Coming Soon" stub)
- [x] T006 [US1] Remove the sentence linking to `using_apis.md` from `docs/src/content/docs/guides/CLI-API/migrating_using_cli_and_kubectl.md` (the line "You can perform the same steps using APIs instead of using kubectl commands...") — depends on T005 to avoid dead link

**Checkpoint**: User Story 1 independently testable. Run `astro build`; visit cluster-conversion page and verify link works; confirm APIs page is gone.

---

## Phase 4: User Story 2 — Accurate Known Limitations Reference (Priority: P1)

**Goal**: A single authoritative Known Limitations page with all 10 specified entries, fully cross-linked.

**Independent Test**: Open `/vjailbreak/reference/known-limitations/` — all 10 entries present, each with a clear heading and description; all cross-links to Troubleshooting pages resolve.

- [x] T007 [US2] Fill all 10 limitation entries in `docs/src/content/docs/reference/known-limitations.md` (created as stub in T002):
  - `## Windows Dynamic Disk (LDM)` — summary of LDM incompatibility + link to `../guides/troubleshooting/windows-dynamic-disk-ldm-migration-issue/`
  - `## Active Directory-Joined VMs` — describe domain re-join behavior post-migration (source: VJAILB-5: AD-joined VMs lose domain trust after migration; require manual re-join or use of unattend.xml)
  - `## Persist Network: Windows Server 2012 and Below` — static network persistence not supported on Windows Server 2012 and earlier
  - `## Assign IP and Persist Network Cannot Be Used Together` — these two options are mutually exclusive; enabling both results in undefined behavior
  - `## Multi-IP Assignment: Only First IP Preserved` — when multiple IPs are configured in the Assign IPs field, only the first IP is preserved on the destination VM
  - `## VMware Tools Removal: Residual Artifacts` — the removal process may leave residual files; link to `../guides/troubleshooting/vmware_residual_artifacts/`
  - `## Multi-Boot VMs Not Supported` — virt-v2v cannot inspect or convert VMs with multiple bootable OS installations
  - `## Hotplug Flavor Requirements` — describe that OpenStack flavors must have hotplug CPU/RAM enabled at the flavor level for hotplug to work post-migration; explain how to verify and assign hotplug-capable flavors
  - `## Low Disk Space for virt-v2v-in-place` — recommend minimum 20 GB free on the vJailbreak VM disk for in-place conversion; migration will fail if disk space is exhausted mid-conversion
  - `## Application Reboot During Migration` — cold migration powers off the source VM; applications must tolerate a reboot; hot migration minimizes downtime but still requires a final cutover reboot

**Checkpoint**: User Story 2 independently testable. All 10 entries present and readable; links to Troubleshooting pages resolve.

---

## Phase 5: User Story 3 — Accurate Prerequisites & Network Requirements (Priority: P2)

**Goal**: Network connectivity section reflects current reality — migration requirements vs. install/upgrade requirements clearly separated.

**Independent Test**: Read the Prerequisites page; confirm the network section has two clearly labeled subsections with accurate content for each scenario.

- [x] T008 [P] [US3] Update the "What network connectivity do I need for vJailbreak?" section in `docs/src/content/docs/introduction/prerequisites.md`:
  - Add intro sentence: "During normal migration operations, vJailbreak only requires connectivity to vCenter, ESXi hosts, and the OpenStack API. The endpoints listed below are needed only when installing or upgrading vJailbreak itself."
  - Restructure section into two sub-headings: `#### Required for Migration (always)` (vCenter, ESXi, OpenStack, ICMP to guest VMs) and `#### Required for Installation and Upgrades Only` (container registries, k3s sources, helm, cloud-init, Virtio ISO, prometheus/cert-manager URLs, health-check endpoints)

**Checkpoint**: User Story 3 independently testable. Prerequisites page clearly distinguishes migration-time vs. install-time connectivity.

---

## Phase 6: User Story 4 — Troubleshooting Content is Findable (Priority: P2)

**Goal**: The resolv.conf virt-v2v error is in the Troubleshooting section; FAQ and Known Issues both link to it.

**Independent Test**: Find the resolv.conf fix in Troubleshooting without going through FAQ; find a link to it from both FAQ and Known Issues.

- [x] T009 [P] [US4] Add a new section `## virt-v2v fails: rename /sysroot/etc/resolv.conf Operation not permitted` to `docs/src/content/docs/guides/Troubleshooting/troubleshooting.md`, copying the full symptom/cause/resolution/notes content from the current FAQ entry (keep FAQ entry as well — see T010)
- [x] T010 [US4] In `docs/src/content/docs/introduction/faq.md`, update the resolv.conf FAQ entry: keep a brief symptom description and resolution summary, then add "See full troubleshooting steps: [virt-v2v resolv.conf error](../../guides/troubleshooting/troubleshooting/#virt-v2v-fails-rename-sysrooteresolvconf-operation-not-permitted)" — depends on T009 being done first so the anchor exists

**Checkpoint**: User Story 4 independently testable. Troubleshooting page has the full resolv.conf entry; FAQ links to it.

---

## Phase 7: User Story 5 — Hotplug Flavor Support Documented (Priority: P2)

**Goal**: The flavor FAQ answer includes a hotplug subsection explaining how to use hotplug-capable OpenStack flavors.

**Independent Test**: Read "How does vJailbreak handle flavors?" in the FAQ — confirm a hotplug subsection is present with usage guidance.

- [x] T011 [US5] In `docs/src/content/docs/introduction/faq.md`, add a `#### Hotplug Flavor Support` subsection under the existing "How does vJailbreak handle flavors of the vm in the target openstack environment?" answer:
  - Explain that if the assigned OpenStack flavor has hotplug CPU/RAM enabled, the migrated VM inherits hotplug capability
  - Note the flavor must be created with `extra_specs` like `hw:cpu_policy=dedicated` and `hw:cpu_max_sockets` configured by the OpenStack admin
  - Add a cross-reference to the Known Limitations entry for hotplug requirements

**Checkpoint**: User Story 5 independently testable. Hotplug subsection present and readable in FAQ.

---

## Phase 8: User Story 6 — CLI Migration Page Reflects Current CRDs (Priority: P2)

**Goal**: All YAML examples in the CLI migration guide match the current CRD schema exactly.

**Independent Test**: Copy any YAML example from the page and apply it with `kubectl apply` against a current vJailbreak cluster — no field validation errors.

- [x] T012 [US6] Fix CRD schema drift in `docs/src/content/docs/guides/CLI-API/migrating_using_cli_and_kubectl.md`:
  - In the MigrationTemplate example: change `osFamily: linuxGuest` → `osFamily: linux` (valid enum: `linux` or `windows`)
  - In the MigrationTemplate example: remove the `spec.source.datacenter` field entirely (field no longer exists in `MigrationTemplateSource` struct)
  - Add a note that `osFamily` accepts `linux` or `windows` (not the legacy `linuxGuest`/`windowsGuest` values)
- [x] T013 [US6] Document new optional MigrationPlan fields in `docs/src/content/docs/guides/CLI-API/migrating_using_cli_and_kubectl.md` — add a new "Optional MigrationPlan Fields" section after the basic example showing:
  - `spec.retry: true/false` — retry migration on failure
  - `spec.firstBootScript: |` — shell script executed on first boot post-migration
  - `spec.postMigrationAction.renameVm`, `.suffix`, `.moveToFolder`, `.folderName`
  - `spec.advancedOptions.granularVolumeTypes`, `.granularNetworks`, `.granularPorts`
  - `spec.migrationStrategy.dataCopyStart`, `.vmCutoverStart`, `.vmCutoverEnd` (RFC3339 datetime)

**Checkpoint**: User Story 6 independently testable. All CLI examples match current CRD types.

---

## Phase 9: User Story 7 — Settings Documentation Complete (Priority: P3)

**Goal**: Every setting in the current release is documented in `vjailbreak_settings.md`.

**Independent Test**: Compare the settings table in the docs against the controller's settings-loading code — no undocumented settings remain.

- [x] T014 [P] [US7] Cross-reference `docs/src/content/docs/guides/How-to/vjailbreak_settings.md` against `k8s/migration/` controller source (search for ConfigMap key reads) — add any settings present in code but missing from the docs table; mark any documented settings that no longer exist in code

**Checkpoint**: User Story 7 independently testable. Settings page is complete and accurate.

---

## Phase 10: User Story 8 — FAQ Covers GPO and vTPM Migration (Priority: P3)

**Goal**: FAQ has entries for GPO and vTPM migrations with working links.

**Independent Test**: Open FAQ, click the GPO migration link and vTPM migration link — both resolve to valid pages.

- [x] T015 [US8] In `docs/src/content/docs/introduction/faq.md`, make three changes in a single edit:
  1. Fix the existing "Can vJailbreak migrate Windows VMs with GPO applied?" link: change `../guides/how-to/gpo_migration.md` → `../../guides/how-to/gpo_migration/`
  2. Add new FAQ entry: "How do I migrate a Windows VM with Group Policy (GPO) applied?" linking to `../../guides/how-to/gpo_migration/`
  3. Add new FAQ entry: "How do I migrate a VM with vTPM (Virtual Trusted Platform Module) enabled?" linking to `../../guides/how-to/vtpm_migration/`

**Checkpoint**: User Story 8 independently testable. Both links resolve; GPO fix removes the 404.

---

## Final Phase: Polish & Cross-Cutting Concerns

**Purpose**: Verify the full build, check all new cross-links, and add Known Limitations link from release notes.

- [x] T016 [P] Add a cross-reference link to the Known Limitations page from the v0.4.3 release notes (`docs/src/content/docs/release_docs/v0.4.3.md`) under "Known Issues" to `/vjailbreak/reference/known-limitations/`
- [x] T017 Run `cd docs && npm run build` and confirm zero build errors and zero broken internal slug warnings
- [x] T018 [P] Manually verify all external links added or modified in this feature are reachable (GPO migration URL, vTPM migration URL, virt-v2v resolv.conf upstream link)

## Phase 11: Known Limitations Follow-up Fixes

**Purpose**: Correct inaccurate content and broken links reported post-implementation.

- [ ] T019 Fix "Low Disk Space for virt-v2v-in-place" entry in `docs/src/content/docs/reference/known-limitations.md`: remove the incorrect 20 GB claim; reference the virt-v2v documentation which states 1 GB minimum free in the temp directory (`/var/tmp` or `$VIRT_V2V_TMPDIR`)
- [ ] T020 Fix broken links in known-limitations.md: change relative paths for "VMware Tools Removal" and "Windows Dynamic Disks" sections to absolute HTTPS URLs (same format as release notes) to avoid base-URL resolution issues
- [ ] T021 Rewrite "Active Directory-Joined VMs" entry in known-limitations.md: cover domain controller migration failure (C00002E2 error), USN rollback risk, and workarounds per [MS KB: domain-controller-not-start-c00002e2-error](https://learn.microsoft.com/en-us/troubleshoot/windows-server/active-directory/domain-controller-not-start-c00002e2-error)

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Setup)**: No dependencies — start immediately
- **Phase 2 (Foundational)**: Depends on Phase 1 passing — blocks cross-links in all later phases
- **Phase 3 (US1)**: Depends on Phase 2; T006 depends on T005
- **Phase 4 (US2)**: Depends on Phase 2 (stub from T002); T007 is the single content task
- **Phases 5–10**: All depend on Phase 2; all independent of each other except:
  - T010 (US4) depends on T009 (US4)
  - T013 (US6) depends on T012 (US6)
  - All faq.md edits (T010, T011, T015) must be sequential (same file)
- **Final Phase**: Depends on all user story phases complete

### User Story Dependencies

- **US1 (P1)**: No deps on other stories
- **US2 (P1)**: No deps on other stories
- **US3–US6 (P2)**: All independent of each other; can run in parallel after Phase 2
- **US7–US8 (P3)**: Independent; can run after Phase 2

### Same-File Sequencing (cannot parallelize)

| File | Tasks (in order) |
|------|-----------------|
| `introduction/faq.md` | T010 → T011 → T015 |
| `guides/CLI-API/migrating_using_cli_and_kubectl.md` | T006 → T012 → T013 |

---

## Parallel Opportunities

```
After Phase 2 completes, these can run concurrently:

Batch A (all different files):
  T004 [US1] concepts/cluster-conversion.md
  T005 [US1] delete using_apis.md
  T007 [US2] reference/known-limitations.md (fill content)
  T008 [US3] introduction/prerequisites.md
  T009 [US4] guides/Troubleshooting/troubleshooting.md
  T012 [US6] migrating_using_cli_and_kubectl.md (CRD fix)
  T014 [US7] vjailbreak_settings.md

Then sequentially (same-file chains):
  T006 after T005 (same file as T012/T013)
  T013 after T012
  T010 after T009 (faq.md)
  T011 after T010 (faq.md)
  T015 after T011 (faq.md)
```

---

## Implementation Strategy

### MVP First (User Stories 1 + 2 Only)

1. Complete Phase 1: Verify build
2. Complete Phase 2: Create Known Limitations stub + sidebar entry
3. Complete Phase 3: Fix all broken links
4. Complete Phase 4: Fill Known Limitations content
5. **STOP and VALIDATE**: All broken links fixed, Known Limitations page live
6. Ship as first increment — highest user impact, zero regressions

### Incremental Delivery

- Increment 1: Phases 1–4 (US1 + US2) → Broken links fixed, Known Limitations live
- Increment 2: Phases 5–8 (US3–US6) → Prerequisites, troubleshooting, hotplug, CLI docs updated
- Increment 3: Phases 9–10 (US7–US8) → Settings verified, FAQ complete
- Final: Polish phase

### Total Task Count

| Phase | Tasks | User Story |
|-------|-------|------------|
| Setup | 1 | — |
| Foundational | 2 | — |
| Phase 3 | 3 | US1 (P1) |
| Phase 4 | 1 | US2 (P1) |
| Phase 5 | 1 | US3 (P2) |
| Phase 6 | 2 | US4 (P2) |
| Phase 7 | 1 | US5 (P2) |
| Phase 8 | 2 | US6 (P2) |
| Phase 9 | 1 | US7 (P3) |
| Phase 10 | 1 | US8 (P3) |
| Polish | 3 | — |
| **Total** | **18** | — |

---

## Notes

- `[P]` tasks = different files, no conflicting dependencies — safe to run in parallel
- `[Story]` label maps each task to its user story for traceability
- faq.md has the most changes (4 separate story contributions) — edit sequentially in one session to avoid merge conflicts
- migrating_using_cli_and_kubectl.md has 3 tasks (T006, T012, T013) — do in order in one editing session
- No new Astro components, no new directories beyond `reference/known-limitations.md`
- Run `astro build` after Phase 2 and after each user story phase to catch broken slugs early
