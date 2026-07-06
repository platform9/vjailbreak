# Requirements Quality Checklist: Cinder Fallback for Storage-Accelerated-Copy LUN Mapping

**Purpose**: Validate that the feature specification is complete, unambiguous, and implementation-ready before/while executing tasks.md
**Created**: 2026-07-06
**Feature**: [spec.md](../spec.md)

## Requirement Completeness

- [x] CHK001 Every mapping-mode combination (auto/native/cinder × mapper/non-mapper vendor) has a defined outcome (FR-002, data-model truth table)
- [x] CHK002 Interface split lists the exact method partition and the zero-edit guarantee for Pure/NetApp (FR-001)
- [x] CHK003 Connector dict contents are fully enumerated with types and presence conditions (FR-004, data-model schema)
- [x] CHK004 Order of operations across create/manage/resolve/map/clone/unmap is pinned (FR-008)
- [x] CHK005 Failure surfaces are specified for gRPC (FR-006), reconciler (FR-005), and selector (FR-002)
- [x] CHK006 Cleanup semantics under canceled contexts are specified (FR-011)
- [x] CHK007 Observability requirements name exact greppable log lines (FR-009)

## Requirement Clarity

- [x] CHK008 "Fallback" is precisely scoped: mapping step only; create/delete/NAA/resolve stay vendor-native (FR-007)
- [x] CHK009 Empty mappingMode ≡ auto is stated everywhere it is consumed (selector, reconciler, CRD)
- [x] CHK010 Per-ESXi connector host format and its fallback value are exact strings (FR-004)
- [x] CHK011 Verification explicitly rules out `cinder show` and names valid alternatives (Clarifications)

## Requirement Consistency

- [x] CHK012 spec.md FRs, plan.md design sections, and contracts/interfaces.md agree on all four interface signatures
- [x] CHK013 Corrected file/line references are consistent across plan.md and research.md D9 (ManageExistingVolume :1133, mock via golang/mock, make manifests)
- [x] CHK014 Scope boundary (FC/iSCSI only, NVMe-oF excluded) consistent across spec/plan/research/quickstart

## Acceptance Criteria Quality

- [x] CHK015 Each user story has independently executable acceptance scenarios with observable outcomes (log lines, CR status, array state)
- [x] CHK016 Success criteria are measurable without reading code (SC-001..SC-005)
- [x] CHK017 US2 defines the CI regression gate (Pure + mappingMode: cinder) replacing the unbuildable new-vendor E2E

## Edge Case Coverage

- [x] CHK018 No-initiator and malformed-FC-UID inputs specified (Edge Cases; FR-010 test list)
- [x] CHK019 Concurrency: same-ESXi and different-ESXi parallel migrations addressed (Edge Cases, research D4)
- [x] CHK020 Crash-window export leak documented with mitigation (Edge Cases, research D10)
- [x] CHK021 Idempotent re-map behavior preserved (US2 scenario 3)

## Dependencies & Assumptions

- [x] CHK022 Cinder backend prerequisite shown to be pre-existing (manage step), adding no new deployment requirements (Assumptions)
- [x] CHK023 Policy assumption (admin-or-owner on volume actions) stated (Assumptions)
- [x] CHK024 Upstream evidence recorded for driver contract and arbitrary connectors (research D2/D3)

## Open Items

- [ ] CHK025 Pure `mappingMode: cinder` E2E executed on hardware and logs archived (SC-003) — pending lab run
- [ ] CHK026 Per-backend knob table validated against the first real third-party array onboarded (quickstart §4)

## Notes

- Check items off as completed: `[x]`
- CHK025/CHK026 require lab hardware and are tracked in tasks.md Phases 6–7
