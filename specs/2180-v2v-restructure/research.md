# Research: v2v-helper Module Restructure — Phase 1

**Date**: 2026-07-30
**Status**: Complete — no NEEDS CLARIFICATION items

## Findings

### Finding 1: Within-package file splits are zero-impact in Go

**Decision**: Split `migrate/migrate.go` by creating new files that all declare `package migrate`.

**Rationale**: In Go, a package is the unit of compilation — not a file. All files in the same directory sharing the same package declaration form a single compilation unit. Moving a method from `migrate.go` to `nbd_copy.go` (both `package migrate`) requires zero changes to callers, zero import updates, and is invisible to the compiler and tests.

**Alternatives considered**: Moving methods to a sub-package (e.g., `migrate/nbd/`) — rejected because it would require callers to update imports and would change the public API surface.

---

### Finding 2: openstack/ does not currently import pkg/utils/

**Decision**: Moving `pkg/utils/openstackopsutils.go` → `openstack/clients.go` introduces no circular import.

**Rationale**: Verified via codebase investigation — `openstack/openstackops.go` imports only standard library and gophercloud packages. `migrate/` imports `openstack/`; adding the implementation to `openstack/` does not change this direction.

**Alternatives considered**: None — the move is straightforward.

---

### Finding 3: *reporter.Reporter already satisfies reporter.ReporterOps

**Decision**: Changing `Migrate.Reporter` field type from `*reporter.Reporter` to `reporter.ReporterOps` is safe — no callers break.

**Rationale**: `ReporterOps` is defined in `reporter/reporter.go` and is already exported. `Reporter` struct implements all methods. Go's structural typing guarantees `*reporter.Reporter` satisfies the interface without any code change to the `reporter` package.

**Alternatives considered**: None — the interface already exists for this purpose.

---

### Finding 4: Method inventory for migrate.go split

**Decision**: 5-file split (not 4, not 7) based on natural concern groupings.

**Rationale**: Audited all 53 methods/functions in migrate.go. Natural groupings with no micro-files:

| File | Methods | LOC |
|------|---------|-----|
| `migrate.go` | MigrateVM, cleanup, 4 cutover helpers, Migrate struct | ~550 |
| `nbd_copy.go` | LiveReplicateDisks, EnableCBTWrapper, SyncCBT, WaitForAdminCutover | ~450 |
| `network.go` | 15 network methods (port reservation + OS config) | ~760 |
| `conversion.go` | ConvertVolumes, attachAllVolumes, 5 boot detection, performDiskConversion, 3 OS helpers | ~780 |
| `vm_ops.go` | 8 volume lifecycle, 4 storage setup, 5 health/instance, 5 utilities | ~650 |

Single-caller helper functions (`getDiskKeys`, `readGuestWWIDs`, `sanitizeVolumeName`) remain with their only callers per YAGNI — moving them creates 1-function files with no reuse benefit.

---

### Finding 5: Caller sites for OpenStackClients import

**Decision**: Update ~3 import sites.

**Rationale**: Grep confirmed `OpenStackClients` is referenced in:
- `main.go`
- `migrate/vaai_copy.go`
- One or more of the migrate/ split files (whichever holds volume/instance creation calls)

All changes are mechanical import path updates: `pkg/utils` → `openstack`.
