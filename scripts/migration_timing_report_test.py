#!/usr/bin/env python3
# Copyright © 2026 The vjailbreak authors
"""Tests for migration-timing-report.py.

Run with:  python3 scripts/migration_timing_report_test.py
"""

import importlib.util
import json
import os
import sys
import tempfile
import unittest

_HERE = os.path.dirname(os.path.abspath(__file__))
_SPEC = importlib.util.spec_from_file_location(
    "migration_timing_report", os.path.join(_HERE, "migration-timing-report.py")
)
report = importlib.util.module_from_spec(_SPEC)
# dataclasses resolves field types through sys.modules, so the module must be
# registered before it is executed.
sys.modules["migration_timing_report"] = report
_SPEC.loader.exec_module(report)

GIB = 1024 ** 3


def summary_line(**overrides):
    payload = {
        "vm": "win2k19-perf",
        "method": "NBD",
        "migration_type": "cold",
        "disk_count": 1,
        "disk_bytes": 200 * GIB,
        "allocated_bytes": 190 * GIB,
        "total_ms": 1_800_000,
        "failed": False,
        "steps": [
            {"step": "vCenter: Take Snapshot", "count": 1, "total_ms": 60_000, "max_ms": 60_000, "errors": 0},
            {"step": "Data Copy: total (all disks)", "count": 1, "total_ms": 1_020_000, "max_ms": 1_020_000, "errors": 0},
        ],
    }
    payload.update(overrides)
    return f"[TIMING-SUMMARY] {json.dumps(payload)}"


def write_log(contents: str) -> str:
    handle = tempfile.NamedTemporaryFile("w", suffix=".log", delete=False)
    handle.write(contents)
    handle.close()
    return handle.name


class ParseSummaryTest(unittest.TestCase):
    def test_reads_summary_line_among_prose(self):
        log = "\n".join(
            [
                "[2026-08-17T10:00:00Z] Creating volumes in OpenStack",
                summary_line(),
                "[2026-08-17T10:30:00Z] ----- Migration completed successfully -----",
            ]
        )
        run = report.parse_log(write_log(log), "baseline")

        self.assertFalse(run.reconstructed)
        self.assertEqual(run.vm, "win2k19-perf")
        self.assertEqual(run.migration_type, "cold")
        self.assertEqual(run.disk_bytes, 200 * GIB)
        self.assertEqual(run.total_ms, 1_800_000)
        self.assertEqual(run.steps["vCenter: Take Snapshot"].total_ms, 60_000)

    def test_last_summary_wins_and_broken_json_is_skipped(self):
        log = "\n".join(
            [
                "[TIMING-SUMMARY] {not json at all",
                summary_line(total_ms=1),
                summary_line(total_ms=999),
            ]
        )
        run = report.parse_log(write_log(log), "baseline")
        self.assertEqual(run.total_ms, 999)

    def test_falls_back_to_per_call_lines(self):
        log = "\n".join(
            [
                '[TIMING] step="PCD: Create Cinder Volume" duration_ms=1000 err=false',
                '[TIMING] step="PCD: Create Cinder Volume" duration_ms=3000 err=true',
                '[TIMING] step="vCenter: Take Snapshot" duration_ms=60000 err=false',
                "some unrelated log line",
            ]
        )
        run = report.parse_log(write_log(log), "baseline")

        self.assertTrue(run.reconstructed)
        volume = run.steps["PCD: Create Cinder Volume"]
        self.assertEqual(volume.count, 2)
        self.assertEqual(volume.total_ms, 4000)
        self.assertEqual(volume.max_ms, 3000)
        self.assertEqual(volume.errors, 1)
        self.assertEqual(run.total_ms, 64_000)

    def test_timestamp_prefixed_lines_are_matched(self):
        # Lines written to /var/log/pf9 carry an RFC3339 prefix.
        log = '[2026-08-17T10:00:01Z] [TIMING] step="vCenter: Power Off VM" duration_ms=6000 err=false'
        run = report.parse_log(write_log(log), "baseline")
        self.assertEqual(run.steps["vCenter: Power Off VM"].total_ms, 6000)


class BuildRowsTest(unittest.TestCase):
    def test_rows_cover_both_runs_and_flag_exclusives(self):
        baseline = report.parse_log(write_log(summary_line()), "baseline")
        hotadd = report.parse_log(
            write_log(
                summary_line(
                    method="HotAdd",
                    total_ms=900_000,
                    steps=[
                        {"step": "Data Copy: total (all disks)", "count": 1, "total_ms": 300_000, "max_ms": 300_000, "errors": 0},
                        {"step": "HotAdd: nbdcopy per disk", "count": 1, "total_ms": 280_000, "max_ms": 280_000, "errors": 0},
                    ],
                )
            ),
            "hot-add",
        )

        rows = {row["step"]: row for row in report.build_rows(baseline, hotadd)}

        self.assertEqual(rows["vCenter: Take Snapshot"]["only_in"], "baseline")
        self.assertEqual(rows["HotAdd: nbdcopy per disk"]["only_in"], "hotadd")

        copy = rows["Data Copy: total (all disks)"]
        self.assertEqual(copy["baseline_min"], 17.0)
        self.assertEqual(copy["hotadd_min"], 5.0)
        self.assertEqual(copy["delta_min"], -12.0)
        self.assertEqual(copy["speedup"], 3.4)
        self.assertEqual(copy["only_in"], "")

    def test_baseline_call_order_is_preserved(self):
        baseline = report.parse_log(write_log(summary_line()), "baseline")
        hotadd = report.parse_log(write_log(summary_line(method="HotAdd")), "hot-add")
        steps = [row["step"] for row in report.build_rows(baseline, hotadd)]
        self.assertEqual(steps, ["vCenter: Take Snapshot", "Data Copy: total (all disks)"])


class WarningsTest(unittest.TestCase):
    def warnings_for(self, baseline_kwargs=None, hotadd_kwargs=None):
        baseline = report.parse_log(write_log(summary_line(**(baseline_kwargs or {}))), "baseline")
        hotadd_defaults = {"method": "HotAdd"}
        hotadd_defaults.update(hotadd_kwargs or {})
        hotadd = report.parse_log(write_log(summary_line(**hotadd_defaults)), "hot-add")
        return report.comparability_warnings(baseline, hotadd)

    def test_clean_comparison_has_no_warnings(self):
        self.assertEqual(self.warnings_for(), [])

    def test_hot_migration_baseline_is_flagged(self):
        warnings = self.warnings_for(
            {"migration_type": "hot"}, {"migration_type": "hot"}
        )
        self.assertTrue(any("not cold" in w for w in warnings), warnings)

    def test_mismatched_migration_type_is_flagged(self):
        warnings = self.warnings_for({"migration_type": "hot"})
        self.assertTrue(any("Different migration types" in w for w in warnings), warnings)

    def test_thin_disk_is_flagged(self):
        warnings = self.warnings_for({"allocated_bytes": 30 * GIB})
        self.assertTrue(any("filled" in w for w in warnings), warnings)

    def test_different_vms_flagged(self):
        warnings = self.warnings_for({}, {"vm": "some-other-vm"})
        self.assertTrue(any("Different source VMs" in w for w in warnings), warnings)

    def test_swapped_columns_flagged(self):
        warnings = self.warnings_for({"method": "HotAdd"})
        self.assertTrue(any("columns are swapped" in w for w in warnings), warnings)

    def test_wrong_method_on_hotadd_log_flagged(self):
        warnings = self.warnings_for({}, {"method": "NBD"})
        self.assertTrue(any("not 'HotAdd'" in w for w in warnings), warnings)

    def test_failed_run_flagged(self):
        warnings = self.warnings_for({}, {"failed": True})
        self.assertTrue(any("Run failed" in w for w in warnings), warnings)

    def test_reconstructed_run_flagged(self):
        baseline = report.parse_log(
            write_log('[TIMING] step="vCenter: Take Snapshot" duration_ms=1 err=false'), "baseline"
        )
        hotadd = report.parse_log(write_log(summary_line(method="HotAdd")), "hot-add")
        warnings = report.comparability_warnings(baseline, hotadd)
        self.assertTrue(any("rebuilt from" in w for w in warnings), warnings)


class CSVTest(unittest.TestCase):
    def test_csv_has_a_total_row(self):
        baseline = report.parse_log(write_log(summary_line()), "baseline")
        hotadd = report.parse_log(write_log(summary_line(method="HotAdd", total_ms=900_000)), "hot-add")
        rows = report.build_rows(baseline, hotadd)

        out = tempfile.NamedTemporaryFile("w", suffix=".csv", delete=False)
        out.close()
        report.write_csv(out.name, rows, baseline, hotadd)

        with open(out.name) as handle:
            contents = handle.read()
        self.assertIn("TOTAL (end to end)", contents)
        self.assertIn("2.0", contents)  # 1_800_000 / 900_000


class FormattingTest(unittest.TestCase):
    def test_hhmm(self):
        self.assertEqual(report.hhmm(0), "0:00")
        self.assertEqual(report.hhmm(60_000), "0:01")
        self.assertEqual(report.hhmm(31 * 60_000 + 19_000), "0:31")
        self.assertEqual(report.hhmm(125 * 60_000), "2:05")


class MainTest(unittest.TestCase):
    def test_empty_logs_exit_nonzero(self):
        empty = write_log("nothing to see here\n")
        self.assertEqual(report.main(["--baseline", empty, "--hotadd", empty]), 1)

    def test_happy_path_exits_zero(self):
        baseline = write_log(summary_line())
        hotadd = write_log(summary_line(method="HotAdd", total_ms=900_000))
        self.assertEqual(report.main(["--baseline", baseline, "--hotadd", hotadd]), 0)


if __name__ == "__main__":
    unittest.main(verbosity=2)
