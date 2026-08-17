#!/usr/bin/env python3
# Copyright © 2026 The vjailbreak authors
"""Build the Hot-Add vs non-Hot-Add migration comparison table from v2v-helper logs.

Run the same VM twice — once with StorageCopyMethod=HotAdd and once without —
then feed both migration logs in:

    ./scripts/migration-timing-report.py --baseline runA.log --hotadd runB.log
    ./scripts/migration-timing-report.py --baseline runA.log --hotadd runB.log --csv table.csv

Logs come from either:

    kubectl -n migration-system logs <migration-name>-v2v-helper
    /var/log/pf9/migration-<vmwaremachine-name>.log   (on the appliance)

The script reads the single ``[TIMING-SUMMARY] {json}`` line each run emits. If
that line is missing — the run crashed before MigrateVM returned — it falls back
to aggregating the per-call ``[TIMING] step="..." duration_ms=N err=bool`` lines,
and says so.

Exit status is 0 even when the comparison is not apples-to-apples; the warnings
are printed instead, because a caveat you can read beats a report you cannot get.
"""

from __future__ import annotations

import argparse
import csv
import json
import re
import sys
from dataclasses import dataclass, field
from typing import Optional

SUMMARY_PREFIX = "[TIMING-SUMMARY]"
LINE_PREFIX = "[TIMING]"

# [TIMING] step="PCD: Create Cinder Volume" duration_ms=1043 err=false
LINE_RE = re.compile(
    r'\[TIMING\]\s+step="(?P<step>[^"]*)"\s+duration_ms=(?P<ms>\d+)\s+err=(?P<err>true|false)'
)

GIB = 1024 ** 3


@dataclass
class Step:
    step: str
    count: int = 0
    total_ms: int = 0
    max_ms: int = 0
    errors: int = 0


@dataclass
class Run:
    """One migration's timings, however they were recovered from the log."""

    label: str
    vm: str = ""
    method: str = ""
    migration_type: str = ""
    disk_count: int = 0
    disk_bytes: int = 0
    allocated_bytes: int = 0
    total_ms: int = 0
    failed: bool = False
    # Ordered: summary order is call order, which is what the table should show.
    steps: dict[str, Step] = field(default_factory=dict)
    # True when no [TIMING-SUMMARY] line was found and the numbers were
    # reconstructed from individual [TIMING] lines.
    reconstructed: bool = False


def parse_log(path: str, label: str) -> Run:
    with open(path, "r", errors="replace") as handle:
        text = handle.read()

    run = _parse_summary(text, label)
    if run is not None:
        return run
    return _parse_lines(text, label)


def _parse_summary(text: str, label: str) -> Optional[Run]:
    """Return the run built from the last [TIMING-SUMMARY] line, or None."""
    payload = None
    for line in text.splitlines():
        idx = line.find(SUMMARY_PREFIX)
        if idx == -1:
            continue
        candidate = line[idx + len(SUMMARY_PREFIX):].strip()
        try:
            payload = json.loads(candidate)
        except json.JSONDecodeError:
            continue  # a retried/truncated line — keep looking
    if payload is None:
        return None

    run = Run(
        label=label,
        vm=payload.get("vm", ""),
        method=payload.get("method", ""),
        migration_type=payload.get("migration_type", ""),
        disk_count=payload.get("disk_count", 0),
        disk_bytes=payload.get("disk_bytes", 0),
        allocated_bytes=payload.get("allocated_bytes", 0),
        total_ms=payload.get("total_ms", 0),
        failed=bool(payload.get("failed", False)),
    )
    for raw in payload.get("steps") or []:
        name = raw.get("step", "")
        run.steps[name] = Step(
            step=name,
            count=raw.get("count", 0),
            total_ms=raw.get("total_ms", 0),
            max_ms=raw.get("max_ms", 0),
            errors=raw.get("errors", 0),
        )
    return run


def _parse_lines(text: str, label: str) -> Run:
    """Rebuild a run from per-call [TIMING] lines when the summary is missing."""
    run = Run(label=label, reconstructed=True)
    for match in LINE_RE.finditer(text):
        name = match.group("step")
        ms = int(match.group("ms"))
        failed = match.group("err") == "true"

        step = run.steps.get(name)
        if step is None:
            step = Step(step=name)
            run.steps[name] = step
        step.count += 1
        step.total_ms += ms
        step.max_ms = max(step.max_ms, ms)
        if failed:
            step.errors += 1
    run.total_ms = sum(s.total_ms for s in run.steps.values())
    return run


def minutes(ms: int) -> float:
    return ms / 60000.0


def hhmm(ms: int) -> str:
    total_minutes = int(round(ms / 60000.0))
    return f"{total_minutes // 60}:{total_minutes % 60:02d}"


def build_rows(baseline: Run, hotadd: Run) -> list[dict]:
    """One row per step, in baseline call order then hot-add-only steps."""
    order: list[str] = list(baseline.steps.keys())
    order += [name for name in hotadd.steps.keys() if name not in baseline.steps]

    rows = []
    for name in order:
        b = baseline.steps.get(name)
        h = hotadd.steps.get(name)
        b_ms = b.total_ms if b else 0
        h_ms = h.total_ms if h else 0
        rows.append(
            {
                "step": name,
                "baseline_calls": b.count if b else 0,
                "baseline_min": round(minutes(b_ms), 2),
                "hotadd_calls": h.count if h else 0,
                "hotadd_min": round(minutes(h_ms), 2),
                "delta_min": round(minutes(h_ms - b_ms), 2),
                "speedup": round(b_ms / h_ms, 2) if h_ms > 0 and b_ms > 0 else "",
                "baseline_errors": b.errors if b else 0,
                "hotadd_errors": h.errors if h else 0,
                "only_in": ""
                if (b and h)
                else ("baseline" if b else "hotadd"),
            }
        )
    return rows


def comparability_warnings(baseline: Run, hotadd: Run) -> list[str]:
    """Reasons the two numbers may not be comparable. Read these before quoting the factor."""
    warnings: list[str] = []

    if baseline.reconstructed or hotadd.reconstructed:
        which = [r.label for r in (baseline, hotadd) if r.reconstructed]
        warnings.append(
            f"No [TIMING-SUMMARY] line in {', '.join(which)} — numbers rebuilt from "
            "individual [TIMING] lines. Phase totals are still exact, but disk size "
            "and end-to-end wall clock are unknown."
        )

    if baseline.failed or hotadd.failed:
        which = [r.label for r in (baseline, hotadd) if r.failed]
        warnings.append(f"Run failed: {', '.join(which)}. Partial timings only.")

    if baseline.vm and hotadd.vm and baseline.vm != hotadd.vm:
        warnings.append(
            f"Different source VMs ({baseline.vm} vs {hotadd.vm}) — this is not a "
            "same-VM comparison."
        )

    if baseline.migration_type and hotadd.migration_type:
        if baseline.migration_type != hotadd.migration_type:
            warnings.append(
                f"Different migration types ({baseline.migration_type} vs "
                f"{hotadd.migration_type}). Hot-Add powers the VM off and does one "
                "full copy with no CBT, so it is only comparable against a COLD run — "
                "a hot run adds changed-block iterations Hot-Add has no equivalent for."
            )
        elif baseline.migration_type != "cold":
            warnings.append(
                f"Both runs are '{baseline.migration_type}', not cold. Hot-Add has no "
                "incremental/CBT phase, so the fair baseline is a cold migration."
            )

    if hotadd.method and hotadd.method != "HotAdd":
        warnings.append(
            f"--hotadd log reports method={hotadd.method!r}, not 'HotAdd'. "
            "Check StorageCopyMethod on that run."
        )
    if baseline.method == "HotAdd":
        warnings.append("--baseline log reports method='HotAdd'. The columns are swapped.")

    if baseline.disk_bytes and hotadd.disk_bytes and baseline.disk_bytes != hotadd.disk_bytes:
        warnings.append(
            f"Provisioned disk size differs: {baseline.disk_bytes / GIB:.1f} GiB vs "
            f"{hotadd.disk_bytes / GIB:.1f} GiB."
        )

    # The trap worth stating on every report: the two paths do not necessarily
    # move the same number of bytes.
    if baseline.disk_bytes and baseline.allocated_bytes:
        fill = baseline.allocated_bytes / baseline.disk_bytes
        if fill < 0.8:
            warnings.append(
                f"Source is only ~{fill * 100:.0f}% filled "
                f"({baseline.allocated_bytes / GIB:.1f} of {baseline.disk_bytes / GIB:.1f} GiB). "
                "Hot-Add serves a raw block device and copies every byte; the VDDK path "
                "skips unallocated extents. On a thin disk this understates Hot-Add. "
                "Use a thick/well-filled disk for a headline number."
            )

    return warnings


def print_report(baseline: Run, hotadd: Run, rows: list[dict]) -> None:
    b_total = baseline.total_ms
    h_total = hotadd.total_ms

    print("vJailbreak migration timing: without Hot-Add vs with Hot-Add")
    print("=" * 108)
    for run in (baseline, hotadd):
        print(
            f"{run.label:<9} vm={run.vm or '?'} method={run.method or '?'} "
            f"type={run.migration_type or '?'} disks={run.disk_count or '?'} "
            f"provisioned={run.disk_bytes / GIB:.1f}GiB committed={run.allocated_bytes / GIB:.1f}GiB"
        )
    print("=" * 108)

    header = f"{'Step':<52}{'no-HotAdd':>12}{'HotAdd':>12}{'delta':>10}{'x':>8}{'calls':>12}"
    print(header)
    print("-" * len(header))
    for row in rows:
        calls = f"{row['baseline_calls']}/{row['hotadd_calls']}"
        flag = ""
        if row["only_in"]:
            flag = " *"
        if row["baseline_errors"] or row["hotadd_errors"]:
            flag += " !"
        print(
            f"{row['step'][:50]:<52}{row['baseline_min']:>12.2f}{row['hotadd_min']:>12.2f}"
            f"{row['delta_min']:>10.2f}{str(row['speedup']):>8}{calls:>12}{flag}"
        )
    print("-" * len(header))
    print(
        f"{'TOTAL (end to end)':<52}{minutes(b_total):>12.2f}{minutes(h_total):>12.2f}"
        f"{minutes(h_total - b_total):>10.2f}"
    )
    print(f"{'TOTAL (hh:mm)':<52}{hhmm(b_total):>12}{hhmm(h_total):>12}")
    if h_total > 0 and b_total > 0:
        print(f"\nFactor by which Hot-Add is faster: {b_total / h_total:.2f}x")
    print("\n* step present in only one run    ! step returned an error (time includes retries)")

    warnings = comparability_warnings(baseline, hotadd)
    if warnings:
        print("\nCaveats — read before quoting the factor:")
        for warning in warnings:
            print(f"  - {warning}")


def write_csv(path: str, rows: list[dict], baseline: Run, hotadd: Run) -> None:
    fields = [
        "step",
        "baseline_calls",
        "baseline_min",
        "hotadd_calls",
        "hotadd_min",
        "delta_min",
        "speedup",
        "baseline_errors",
        "hotadd_errors",
        "only_in",
    ]
    with open(path, "w", newline="") as handle:
        writer = csv.DictWriter(handle, fieldnames=fields)
        writer.writeheader()
        writer.writerows(rows)
        writer.writerow(
            {
                "step": "TOTAL (end to end)",
                "baseline_min": round(minutes(baseline.total_ms), 2),
                "hotadd_min": round(minutes(hotadd.total_ms), 2),
                "delta_min": round(minutes(hotadd.total_ms - baseline.total_ms), 2),
                "speedup": round(baseline.total_ms / hotadd.total_ms, 2)
                if hotadd.total_ms
                else "",
            }
        )


def main(argv: Optional[list[str]] = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__.split("\n")[0])
    parser.add_argument("--baseline", required=True, help="v2v-helper log of the run WITHOUT Hot-Add")
    parser.add_argument("--hotadd", required=True, help="v2v-helper log of the run WITH Hot-Add")
    parser.add_argument("--csv", help="also write the table to this CSV path")
    args = parser.parse_args(argv)

    baseline = parse_log(args.baseline, "baseline")
    hotadd = parse_log(args.hotadd, "hot-add")

    if not baseline.steps and not hotadd.steps:
        print(
            "No [TIMING] or [TIMING-SUMMARY] lines in either log. Both runs must be on a "
            "v2v-helper build that includes pkg/timing.",
            file=sys.stderr,
        )
        return 1

    rows = build_rows(baseline, hotadd)
    print_report(baseline, hotadd, rows)
    if args.csv:
        write_csv(args.csv, rows, baseline, hotadd)
        print(f"\nWrote {args.csv}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
