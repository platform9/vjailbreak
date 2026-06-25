#!/usr/bin/env python3
"""Unit tests for parse-logs.py.

Run from this directory:
  python3 -m unittest test_parse_logs.py -v
"""
import importlib.util
import unittest
from pathlib import Path

# parse-logs.py has a hyphen so we load it via importlib
_spec = importlib.util.spec_from_file_location(
    "parse_logs", Path(__file__).parent / "parse-logs.py"
)
parse_logs = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(parse_logs)


class TestParseDisks(unittest.TestCase):
    def test_basic_two_disks_completed(self):
        text = (
            "2026/05/15 14:00:01 Starting full disk copy [1/2]: disk1 (DeviceKey=2000)\n"
            "2026/05/15 14:00:18 Disk 0 (disk1) copied successfully in 15.2s\n"
            "2026/05/15 14:00:20 Starting full disk copy [2/2]: disk2 (DeviceKey=2001)\n"
            "2026/05/15 14:00:30 Disk 1 (disk2) copied successfully in 9.8s\n"
        )
        disks = parse_logs.parse_disk_copies(text)
        self.assertEqual(set(disks.keys()), {"disk1", "disk2"})
        self.assertEqual(disks["disk1"].duration, "15.2s")
        self.assertEqual(disks["disk1"].status, "completed")
        self.assertEqual(disks["disk2"].duration, "9.8s")

    def test_done_line_authoritative_for_index(self):
        # Regression for the bug where 1-based progress counter ("[2/2]") won
        # over 0-based array index ("Disk 1"), breaking CBT attribution.
        text = (
            "Starting full disk copy [2/2]: disk2 (DeviceKey=2001)\n"
            "Disk 1 (disk2) copied successfully in 9.8s\n"
        )
        disks = parse_logs.parse_disk_copies(text)
        self.assertEqual(disks["disk2"].index, "1")  # 0-based, not "2"

    def test_cbt_iterations_attributed_to_correct_disk(self):
        text = (
            "Starting full disk copy [1/2]: disk1 (DeviceKey=2000)\n"
            "Disk 0 (disk1) copied successfully in 15.2s\n"
            "Starting full disk copy [2/2]: disk2 (DeviceKey=2001)\n"
            "Disk 1 (disk2) copied successfully in 9.8s\n"
            "Finished copying and syncing changed blocks for disk 1 in 0.5s [Progress: 1/4]\n"
            "Finished copying and syncing changed blocks for disk 1 in 0.4s [Progress: 2/4]\n"
        )
        disks = parse_logs.parse_disk_copies(text)
        self.assertEqual(disks["disk1"].cbt_iterations, 0)
        self.assertEqual(disks["disk2"].cbt_iterations, 2)

    def test_empty_text_returns_empty_dict(self):
        self.assertEqual(parse_logs.parse_disk_copies(""), {})


class TestParseCBT(unittest.TestCase):
    def test_iterations_extracted(self):
        text = (
            "Finished copying and syncing changed blocks for disk 1 in 1.2s [Progress: 1/20]\n"
            "Finished copying and syncing changed blocks for disk 1 in 0.8s [Progress: 2/20]\n"
        )
        iters = parse_logs.parse_cbt_iterations(text)
        self.assertEqual(len(iters), 2)
        self.assertEqual(iters[0]["disk"], "1")
        self.assertEqual(iters[0]["duration"], "1.2s")
        self.assertEqual(iters[0]["progress"], "1/20")

    def test_no_cbt_means_empty_list(self):
        self.assertEqual(parse_logs.parse_cbt_iterations("nothing here"), [])


class TestCategorizeErrors(unittest.TestCase):
    def test_dns_bucket(self):
        text = "ERROR: failed to lookup esxi01.example.com: no such host\n"
        self.assertIn("DNS", parse_logs.categorize_errors(text))

    def test_vcenter_bucket(self):
        text = "ERROR: vcenter session expired\n"
        self.assertIn("vCenter", parse_logs.categorize_errors(text))

    def test_vddk_bucket(self):
        text = "failed to start nbdkit: vmware-vix-disklib not found\n"
        self.assertIn("NBD/VDDK", parse_logs.categorize_errors(text))

    def test_non_error_lines_excluded(self):
        text = "info: starting up\n"
        self.assertEqual(parse_logs.categorize_errors(text), {})

    def test_unknown_pattern_goes_to_other(self):
        text = "ERROR: something totally unfamiliar broke\n"
        self.assertIn("Other", parse_logs.categorize_errors(text))


class TestRootCause(unittest.TestCase):
    def test_dns_root_cause(self):
        rc = parse_logs.suggest_root_cause("ERROR: no such host\n")
        self.assertIsNotNone(rc)
        cause, action = rc
        self.assertIn("DNS", cause)
        self.assertIn("/etc/hosts", action)

    def test_vddk_root_cause(self):
        rc = parse_logs.suggest_root_cause("vmware-vix-disklib missing")
        self.assertIsNotNone(rc)
        self.assertIn("VDDK", rc[0])

    def test_clean_logs_no_suggestion(self):
        self.assertIsNone(parse_logs.suggest_root_cause("everything is fine"))


class TestPodDescribe(unittest.TestCase):
    def test_extracts_fields(self):
        text = (
            "Name:         v2v-helper-myvm-abc123\n"
            "Status:       Running\n"
            "Restart Count: 3\n"
            "Image:        platform9/v2v-helper:v0.1\n"
            "Reason:       Error\n"
        )
        info = parse_logs.parse_pod_describe(text)
        self.assertEqual(info.name, "v2v-helper-myvm-abc123")
        self.assertEqual(info.restart_count, 3)
        self.assertEqual(info.image, "platform9/v2v-helper:v0.1")
        self.assertEqual(info.exit_reason, "Error")


class TestEvents(unittest.TestCase):
    def test_disk_copy_start_detected(self):
        events = parse_logs.parse_events(
            "Starting full disk copy [1/2]: disk1 (DeviceKey=2000)\n"
        )
        self.assertTrue(any(e.kind == "disk.copy.start" for e in events))

    def test_migration_complete_detected(self):
        events = parse_logs.parse_events(
            "Migration completed successfully at 2026-05-15T14:01:25Z\n"
        )
        self.assertTrue(any(e.kind == "migration.complete" for e in events))

    def test_timestamp_extracted_from_go_log_format(self):
        events = parse_logs.parse_events("2026/05/15 14:00:01 Starting NBD server\n")
        self.assertTrue(any(e.timestamp == "2026/05/15 14:00:01" for e in events))


class TestMappings(unittest.TestCase):
    def test_network_mapping_rows(self):
        yaml_data = {
            "items": [{
                "metadata": {"name": "prod-nm"},
                "spec": {"networks": [{"source": "vSwitch0", "target": "provider-net"}]},
            }]
        }
        rows = parse_logs.extract_mapping_entries(yaml_data)
        self.assertEqual(len(rows), 1)
        self.assertEqual(rows[0]["source"], "vSwitch0")
        self.assertEqual(rows[0]["target"], "provider-net")

    def test_empty_yaml_returns_empty_list(self):
        self.assertEqual(parse_logs.extract_mapping_entries(None), [])
        self.assertEqual(parse_logs.extract_mapping_entries({}), [])


if __name__ == "__main__":
    unittest.main()
