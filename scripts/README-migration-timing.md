# Hot-Add vs non-Hot-Add migration timing

How to produce the per-call comparison table for "how much faster is a migration
with the Hot-Add proxy" using real measurements instead of estimates.

## What got instrumented

`v2v-helper` times every vCenter and PCD API call plus each coarse phase, and
writes two kinds of line into the migration log:

```
[TIMING] step="PCD: Create Cinder Volume" duration_ms=1043 err=false
[TIMING-SUMMARY] {"vm":"win2k19-perf","method":"HotAdd","migration_type":"cold",...}
```

Timing is always on — the lines are cheap, and it means the table can be
reconstructed from any past run's logs without re-running the migration.

| Piece | Where |
|---|---|
| Recorder + log format | `v2v-helper/pkg/timing/timing.go` |
| vCenter call timings | `v2v-helper/vm/vmops_timed.go` (decorator over `VMOperations`) |
| PCD call timings | `v2v-helper/openstack/openstackops_timed.go` (decorator over `OpenstackOperations`) |
| Coarse phases + Hot-Add sub-steps | `v2v-helper/migrate/timing_hooks.go`, `hotadd_copy.go` |
| Wiring | `v2v-helper/main.go` |
| Report | `scripts/migration-timing-report.py` |

The decorators are pure pass-throughs, so no call site needed editing and every
method on those two interfaces gets a row for free.

## Run protocol

Migrate **one** VM twice, resetting between runs (delete the target VM and its
volumes, remove leftover `vjailbreak-hotadd-snap` snapshots).

| | Run A (baseline) | Run B (Hot-Add) |
|---|---|---|
| `StorageCopyMethod` | default | `HotAdd` |
| `MigrationType` | **cold** | **cold** |
| Everything else | identical | identical |

Hold constant: vCenter, ESXi host, datastore, PCD project, flavor, volume type,
appliance, network. Repeat each run 3x and report the median — one run of each is
an anecdote.

### Two things that will invalidate the comparison

1. **Cold, not hot.** `HotAddCopyDisks` powers the source VM off, takes one
   snapshot and does a single full copy — no CBT, no changed-block iterations.
   The only structurally comparable baseline is a cold migration. The report
   prints a warning if either run was hot.

2. **Thick, well-filled disk.** Hot-Add serves a raw block device through
   `qemu-nbd --format=raw`, which reports no sparseness, so `nbdcopy` moves every
   byte of the provisioned size. The VDDK path reads allocated extents and skips
   holes. On a thin 200 GB disk holding 30 GB, Hot-Add can measure *slower*. The
   report warns when committed/provisioned drops below 80%.

Proxy VM creation and onboarding is a **one-time** cost, not per-migration.
Measure it separately and report it as a footnote rather than folding it into the
per-VM number.

## Collect and report

```bash
# after each run
kubectl -n migration-system logs <migration-name>-v2v-helper > runA.log
# or on the appliance: /var/log/pf9/migration-<vmwaremachine-name>.log
# or pull it via the debug-bundle API: /sdk/vpw/v1/debug-bundle

./scripts/migration-timing-report.py --baseline runA.log --hotadd runB.log --csv table.csv
```

Output is the comparison table (step, both durations, delta, speedup, call
counts), the end-to-end total in minutes and hh:mm, the speed-up factor, and a
caveats list. `table.csv` pastes straight into the existing slide spreadsheet.

Steps present in only one run are marked `*` (the Hot-Add sub-steps have no VDDK
equivalent — that is the overhead Hot-Add pays to buy a faster read). Steps that
returned an error are marked `!`; their duration includes retries and is not
comparable.

## Tests

```bash
make test-v2v-helper                          # Go side (needs Linux CGO)
python3 scripts/migration_timing_report_test.py   # report script
```
