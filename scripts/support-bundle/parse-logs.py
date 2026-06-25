#!/usr/bin/env python3
"""
parse-logs.py — turn a vjailbreak log bundle into a rich single-page HTML report.

Surfaces:
  - Migration health card (phase, duration, last error, suggested root cause)
  - VM details (name, OS, disks, networks, target flavor)
  - Per-disk copy progress table (size, throughput, duration, status)
  - Event timeline (snapshot → copy → CBT → virt-v2v → cutover)
  - CBT sync iterations (for hot migrations)
  - Errors categorized by subsystem (DNS, vCenter, NBD, VDDK, virt-v2v, OpenStack, k8s)
  - Pod restart count + exit reasons
  - Network & storage mappings as tables
  - Kubernetes events as sortable table
  - Sticky search box that filters all log content

Usage:
  parse-logs.py /path/to/vjailbreak-bundle-XXX.tar.gz
  parse-logs.py /path/to/extracted-bundle-dir/
"""
import argparse
import html
import json
import re
import sys
import tarfile
import tempfile
from collections import defaultdict
from dataclasses import dataclass, field
from datetime import datetime, timezone, timedelta
from pathlib import Path
from typing import Optional

try:
    import yaml
except ImportError:
    print("ERROR: PyYAML missing. Install with: sudo apt-get install -y python3-yaml", file=sys.stderr)
    sys.exit(1)


# ============================================================
# PATTERNS
# ============================================================

# Timestamps — handle Go-style "2026/05/15 14:05:35" and ISO formats
TIMESTAMP_RE = re.compile(
    r'(?P<ts>'
    r'\d{4}[/-]\d{2}[/-]\d{2}[ T]\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:?\d{2})?'
    r'|\d{2}:\d{2}:\d{2}(?:\.\d+)?'
    r')'
)

# vjailbreak-specific event patterns (from migrate.go in v2v-helper)
EVENT_PATS = [
    ("snapshot.clean",      re.compile(r'Cleaning up snapshots before copy', re.I), "▼"),
    ("snapshot.take",       re.compile(r'Starting NBD server|take.*snapshot', re.I), "📸"),
    ("disk.copy.start",     re.compile(r'Starting full disk copy \[(\d+)/(\d+)\]: (\S+)', re.I), "💾"),
    ("disk.copy.done",      re.compile(r'Disk (\d+)\s*\(([^)]+)\)\s*copied successfully in (\S+)', re.I), "✅"),
    ("cbt.sync",            re.compile(r'Finished copying and syncing changed blocks for disk (\d+) in (\S+)\s*\[Progress:\s*(\d+)/(\d+)\]', re.I), "🔄"),
    ("cbt.no_change",       re.compile(r'Disk (\d+): No changed blocks found', re.I), "·"),
    ("cbt.has_change",      re.compile(r'Disk (\d+): Blocks have Changed', re.I), "🔄"),
    ("cutover.shutdown",    re.compile(r'Shutting down source VM', re.I), "⏻"),
    ("cutover.admin",       re.compile(r'Admin initiated cutover', re.I), "👤"),
    ("virtv2v.start",       re.compile(r'(virt-v2v-in-place|Executing virt-v2v)', re.I), "🔧"),
    ("virtv2v.done",        re.compile(r'virt-v2v-in-place conversion took:\s*(\S+)', re.I), "✅"),
    ("vm.create",           re.compile(r'creat(ing|ed)\s+(target\s+)?(instance|server|VM)', re.I), "🎯"),
    ("migration.complete",  re.compile(r'Migration completed successfully', re.I), "🎉"),
    ("nbd.start",           re.compile(r'(Starting|Restarting) NBD server', re.I), "🔌"),
    ("nbd.stop",            re.compile(r'failed to stop NBD server', re.I), "⚠️"),
    ("power.off",           re.compile(r'(power(ing)?\s*off|VMPowerOff)', re.I), "⏻"),
]

# Error categorization
ERROR_CATEGORIES = [
    ("DNS",        re.compile(r'\b(dns|resolv\.conf|/etc/hosts|no such host|nxdomain|name resolution|lookup\s+\S+:\s)', re.I)),
    ("vCenter",    re.compile(r'\b(vcenter|govmomi|session.*(expired|invalid)|nfc\s+error|esxi)', re.I)),
    ("NBD/VDDK",   re.compile(r'\b(nbd|nbdkit|libnbd|vddk|vmware-vix-disklib)', re.I)),
    ("virt-v2v",   re.compile(r'\b(virt-v2v|libguestfs|guestfish|virtio-win|initramfs)', re.I)),
    ("OpenStack",  re.compile(r'\b(openstack|cinder|neutron|nova|keystone|gophercloud|glance)', re.I)),
    ("Kubernetes", re.compile(r'\b(kubernetes|controller-runtime|reconcile|crd|webhook|api(\s|-)?server)', re.I)),
    ("Network",    re.compile(r'\b(connection\s+(refused|reset|timeout)|i/o timeout|tls handshake|certificate)', re.I)),
    ("Auth",       re.compile(r'\b(unauthor(ized|ised)|forbidden|permission denied|invalid credentials|401|403)', re.I)),
]

# Generic indicators
ERROR_INDICATOR = re.compile(r'\b(error|panic|failed|fatal|exception|nil pointer|segfault|sigsegv|cannot|denied|refused|timeout|unable)\b', re.I)
WARN_INDICATOR = re.compile(r'\b(warn(ing)?|deprecated|retry(ing)?|skipped|stale)\b', re.I)
SUCCESS_INDICATOR = re.compile(r'\b(success(fully)?|completed|finished|✓|✅)\b', re.I)

# Root-cause heuristics
ROOT_CAUSE_RULES = [
    (re.compile(r'/etc/hosts|resolv\.conf|no such host|name resolution', re.I),
     "DNS resolution failure",
     "Verify /etc/hosts on the appliance has entries for vCenter AND every ESXi host. After editing, restart the controller pod."),
    (re.compile(r'vmware-vix-disklib|VDDK', re.I),
     "VDDK libraries missing",
     "Copy VDDK to /home/ubuntu/vmware-vix-disklib-distrib on the appliance and restart v2v-helper."),
    (re.compile(r'session.*expired|nfc error 5', re.I),
     "vCenter session expired or NFC server overloaded",
     "Reduce concurrent v2v-helper pods, or retry. NFC errors often resolve themselves after backoff."),
    (re.compile(r'virt-v2v-in-place.*fail', re.I),
     "virt-v2v conversion failed",
     "Check guest OS support at libguestfs.org/virt-v2v-support.1.html. Verify virtio-win driver was injected for Windows guests."),
    (re.compile(r'nbdkit.*(crash|exited|failed)', re.I),
     "NBD server crashed",
     "Likely VDDK incompatibility or vCenter network blip. Check VDDK version vs vCenter version. Retry usually works."),
    (re.compile(r'cinder|volume.*creat.*fail', re.I),
     "OpenStack volume creation failed",
     "Check Cinder quota and backend health on target cloud."),
    (re.compile(r'context deadline exceeded|i/o timeout', re.I),
     "Timeout — slow vCenter, large disk, or network bottleneck",
     "Check network throughput between appliance and ESXi. For very large disks, raise the operation timeout in vjailbreak-settings."),
    (re.compile(r'permission denied|unauthor|invalid credentials|401|403', re.I),
     "Authentication / authorization failure",
     "Re-verify VMwareCreds and OpenstackCreds. Confirm the vCenter account has VirtualMachine.Provisioning permissions."),
]


# ============================================================
# DATA MODEL
# ============================================================

@dataclass
class Event:
    line_no: int
    timestamp: Optional[str]
    kind: str
    icon: str
    summary: str
    raw: str


@dataclass
class DiskCopy:
    index: str = "?"
    name: str = "?"
    started: Optional[str] = None
    finished: Optional[str] = None
    duration: Optional[str] = None
    cbt_iterations: int = 0
    status: str = "unknown"


@dataclass
class PodInfo:
    name: str
    restart_count: int = 0
    exit_reason: str = ""
    image: str = ""
    status: str = ""
    raw_describe: str = ""


# ============================================================
# HELPERS
# ============================================================

def extract_tarball(path: Path) -> Path:
    if path.is_dir():
        return path
    if not path.exists():
        sys.exit(f"Not found: {path}")
    tmp = Path(tempfile.mkdtemp(prefix="vjbundle-"))
    print(f"  Extracting to {tmp}", file=sys.stderr)
    with tarfile.open(path, 'r:gz') as t:
        # Use filter='data' to silence the Python 3.14 deprecation
        try:
            t.extractall(tmp, filter='data')
        except TypeError:
            t.extractall(tmp)
    children = list(tmp.iterdir())
    if len(children) == 1 and children[0].is_dir():
        return children[0]
    return tmp


def safe_read(p: Path) -> str:
    try:
        return p.read_text(encoding='utf-8', errors='replace')
    except Exception:
        return ""


def safe_yaml(p: Path):
    if not p.exists() or p.stat().st_size == 0:
        return None
    try:
        return yaml.safe_load(safe_read(p))
    except Exception as e:
        return {"_parse_error": str(e)}


def safe_json(p: Path):
    if not p.exists() or p.stat().st_size == 0:
        return None
    try:
        return json.loads(safe_read(p))
    except Exception as e:
        return {"_parse_error": str(e)}


def find_timestamp(line: str) -> Optional[str]:
    m = TIMESTAMP_RE.search(line)
    return m.group("ts") if m else None


# ============================================================
# PARSERS
# ============================================================

def parse_events(text: str) -> list[Event]:
    events = []
    for i, line in enumerate(text.splitlines(), start=1):
        for kind, pat, icon in EVENT_PATS:
            m = pat.search(line)
            if m:
                events.append(Event(
                    line_no=i,
                    timestamp=find_timestamp(line),
                    kind=kind,
                    icon=icon,
                    summary=line.strip()[:200],
                    raw=line.strip(),
                ))
                break
    return events


def parse_disk_copies(text: str) -> dict[str, DiskCopy]:
    """Parse per-disk copy starts, completions, and CBT iterations.

    Keyed by disk *name* (not index) because vjailbreak uses two different
    indexing schemes in its logs:
      - "Starting full disk copy [1/2]:" uses a 1-based progress counter
      - "Disk 0 (disk1) copied successfully" uses a 0-based array index
    Using name as the key avoids conflating them.
    """
    disks: dict[str, DiskCopy] = {}
    for line in text.splitlines():
        ts = find_timestamp(line)
        m = re.search(r'Starting full disk copy \[(\d+)/\d+\]:\s*(\S+)', line)
        if m:
            name = m.group(2)
            disks.setdefault(name, DiskCopy(index=m.group(1), name=name))
            disks[name].started = ts or disks[name].started
            disks[name].status = "copying"
            continue
        m = re.search(r'Disk\s+(\d+)\s*\(([^)]+)\)\s*copied successfully in\s*(\S+)', line)
        if m:
            name = m.group(2)
            disks.setdefault(name, DiskCopy(index=m.group(1), name=name))
            # The "Done" line uses the 0-based array index that CBT logs also use,
            # so it's authoritative — overwrite any earlier 1-based counter from "Starting".
            disks[name].index = m.group(1)
            disks[name].finished = ts or disks[name].finished
            disks[name].duration = m.group(3)
            disks[name].status = "completed"
            continue
        m = re.search(r'Finished copying and syncing changed blocks for disk (\d+)', line)
        if m:
            idx = m.group(1)
            target = next((d for d in disks.values() if d.index == idx), None)
            if target is None:
                target = disks.setdefault(f"disk-idx-{idx}", DiskCopy(index=idx, name=f"disk-idx-{idx}"))
            target.cbt_iterations += 1
    return disks


def parse_cbt_iterations(text: str) -> list[dict]:
    """Parse each CBT sync iteration with its progress counter."""
    iters = []
    for i, line in enumerate(text.splitlines(), start=1):
        m = re.search(r'Finished copying and syncing changed blocks for disk (\d+) in (\S+)\s*\[Progress:\s*(\d+)/(\d+)\]', line)
        if m:
            iters.append({
                "line_no": i,
                "timestamp": find_timestamp(line),
                "disk": m.group(1),
                "duration": m.group(2),
                "progress": f"{m.group(3)}/{m.group(4)}",
            })
    return iters


def categorize_errors(text: str) -> dict[str, list[tuple[int, str]]]:
    """Returns {category_name: [(line_no, line), ...]}"""
    buckets = defaultdict(list)
    for i, line in enumerate(text.splitlines(), start=1):
        if not ERROR_INDICATOR.search(line):
            continue
        matched = False
        for cat_name, cat_pat in ERROR_CATEGORIES:
            if cat_pat.search(line):
                buckets[cat_name].append((i, line.rstrip()))
                matched = True
                break
        if not matched:
            buckets["Other"].append((i, line.rstrip()))
    return dict(buckets)


def suggest_root_cause(text: str) -> Optional[tuple[str, str]]:
    """Best-effort root cause from log content."""
    for pat, cause, action in ROOT_CAUSE_RULES:
        if pat.search(text):
            return (cause, action)
    return None


def parse_pod_describe(text: str) -> PodInfo:
    info = PodInfo(name="?")
    m = re.search(r'^Name:\s+(\S+)', text, re.M)
    if m: info.name = m.group(1)
    m = re.search(r'Restart Count:\s+(\d+)', text)
    if m: info.restart_count = int(m.group(1))
    m = re.search(r'Image:\s+(\S+)', text)
    if m: info.image = m.group(1)
    m = re.search(r'Status:\s+(\S+)', text)
    if m: info.status = m.group(1)
    m = re.search(r'Reason:\s+(\S+)', text)
    if m: info.exit_reason = m.group(1)
    info.raw_describe = text
    return info


def extract_mapping_entries(yaml_data) -> list[dict]:
    """Pull source→target rows from a NetworkMapping or StorageMapping YAML."""
    if not yaml_data or not isinstance(yaml_data, dict):
        return []
    items = yaml_data.get("items", [])
    rows = []
    for it in items:
        name = (it.get("metadata") or {}).get("name", "?")
        spec = it.get("spec") or {}
        # network mappings: spec.networks[].source / .target
        # storage mappings: spec.storages[].source / .target
        for key in ("networks", "storages", "mappings"):
            for entry in spec.get(key, []) or []:
                rows.append({
                    "mapping_name": name,
                    "source": entry.get("source") or entry.get("sourceNetwork") or entry.get("sourceDatastore") or "?",
                    "target": entry.get("target") or entry.get("targetNetwork") or entry.get("targetVolumeType") or "?",
                })
    return rows


def total_duration(events: list[Event]) -> Optional[str]:
    """Estimate total wall-clock duration from earliest to latest event timestamps."""
    ts_lines = [e.timestamp for e in events if e.timestamp]
    if len(ts_lines) < 2:
        return None
    # Try to parse as full datetime
    def parse(t):
        for fmt in ("%Y/%m/%d %H:%M:%S", "%Y-%m-%dT%H:%M:%S", "%Y-%m-%d %H:%M:%S"):
            try:
                return datetime.strptime(t.split('.')[0].rstrip('Z'), fmt)
            except Exception:
                continue
        return None
    dts = [d for d in (parse(t) for t in ts_lines) if d]
    if len(dts) < 2:
        return None
    delta = max(dts) - min(dts)
    s = int(delta.total_seconds())
    if s < 60: return f"{s}s"
    if s < 3600: return f"{s//60}m {s%60}s"
    return f"{s//3600}h {(s%3600)//60}m"


# ============================================================
# COLLECT
# ============================================================

def collect_bundle(root: Path) -> dict:
    data = {
        "bundle_name": root.name,
        "generated_at": datetime.now(timezone.utc).isoformat(),
        "cluster_overview": safe_read(root / "00-cluster-overview.txt"),
        "settings": safe_read(root / "vjailbreak-settings.yaml"),
        "controller": {},
        "migrations": {},
        "creds_redacted": {},
        "events_k8s": {},
        "mappings": {"network": [], "storage": []},
        "totals": {"errors": 0, "warnings": 0, "log_bytes": 0},
    }

    # Mappings
    nm = safe_yaml(root / "crds" / "networkmapping.yaml")
    sm = safe_yaml(root / "crds" / "storagemapping.yaml")
    data["mappings"]["network"] = extract_mapping_entries(nm)
    data["mappings"]["storage"] = extract_mapping_entries(sm)

    # Controller logs
    ctrl_dir = root / "controller"
    if ctrl_dir.exists():
        for f in ctrl_dir.glob("*.log"):
            txt = safe_read(f)
            buckets = categorize_errors(txt)
            err_count = sum(len(v) for v in buckets.values())
            data["controller"][f.name] = {
                "size": len(txt),
                "lines": txt.count("\n"),
                "error_buckets": buckets,
                "tail": "\n".join(txt.splitlines()[-50:]),
            }
            data["totals"]["errors"] += err_count
            data["totals"]["log_bytes"] += len(txt)

    # Migrations
    mig_dir = root / "migrations"
    if mig_dir.exists():
        for m in sorted(mig_dir.iterdir()):
            if not m.is_dir(): continue
            mig_yaml = safe_yaml(m / "migration.yaml")
            entry = {
                "spec": (mig_yaml or {}).get("spec", {}) if isinstance(mig_yaml, dict) else {},
                "status": (mig_yaml or {}).get("status", {}) if isinstance(mig_yaml, dict) else {},
                "pods": {},
                "events": [],
                "disks": {},
                "cbt_iters": [],
                "error_buckets": defaultdict(list),
                "root_cause": None,
                "duration": None,
            }
            combined_log_text = ""
            for log_f in sorted(m.glob("*.log")):
                txt = safe_read(log_f)
                combined_log_text += txt + "\n"
                buckets = categorize_errors(txt)
                err_count = sum(len(v) for v in buckets.values())
                entry["pods"][log_f.name] = {
                    "size": len(txt),
                    "lines": txt.count("\n"),
                    "error_buckets": buckets,
                    "tail": "\n".join(txt.splitlines()[-200:]),
                    "describe": "",
                    "pod_info": None,
                }
                data["totals"]["errors"] += err_count
                data["totals"]["log_bytes"] += len(txt)

                # Match a describe file
                describe_f = log_f.with_suffix("").with_suffix(".describe.txt")
                # try alternate
                if not describe_f.exists():
                    describe_f = m / f"{log_f.stem.replace('.previous', '')}.describe.txt"
                if describe_f.exists():
                    desc_txt = safe_read(describe_f)
                    entry["pods"][log_f.name]["describe"] = desc_txt
                    entry["pods"][log_f.name]["pod_info"] = parse_pod_describe(desc_txt)

            entry["events"] = parse_events(combined_log_text)
            entry["disks"] = parse_disk_copies(combined_log_text)
            entry["cbt_iters"] = parse_cbt_iterations(combined_log_text)
            # Re-bucket combined errors at migration level
            entry["error_buckets"] = categorize_errors(combined_log_text)
            entry["root_cause"] = suggest_root_cause(combined_log_text)
            entry["duration"] = total_duration(entry["events"])
            data["migrations"][m.name] = entry

    # Redacted creds
    creds_dir = root / "creds-redacted"
    if creds_dir.exists():
        for f in creds_dir.glob("*.json"):
            j = safe_json(f)
            items = (j or {}).get("items", []) if isinstance(j, dict) else []
            data["creds_redacted"][f.stem] = [
                {
                    "name": (it.get("metadata") or {}).get("name", "?"),
                    "spec_keys": sorted((it.get("spec") or {}).keys()),
                }
                for it in items
            ]

    # k8s events
    ev_dir = root / "events"
    if ev_dir.exists():
        for f in ev_dir.glob("*.txt"):
            data["events_k8s"][f.stem] = safe_read(f)

    return data


# ============================================================
# HTML / CSS / JS
# ============================================================

CSS = """
* { box-sizing: border-box; }
body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Helvetica, Arial, sans-serif;
       margin: 0; background: #0f1115; color: #e6e6e6; }
header { background: linear-gradient(135deg, #1a1d24, #232730); padding: 24px 32px;
         border-bottom: 2px solid #2d65f0; position: sticky; top: 0; z-index: 100; }
header .row { display: flex; align-items: center; justify-content: space-between; gap: 20px; }
header h1 { margin: 0; font-size: 20px; }
header .meta { color: #9aa3b2; font-size: 12px; margin-top: 4px; }
#search { background: #0a0c10; border: 1px solid #2d65f0; color: #e6e6e6; padding: 8px 12px;
          border-radius: 6px; width: 320px; font-size: 13px; }
#search:focus { outline: none; border-color: #5a8af0; }
.container { max-width: 1280px; margin: 0 auto; padding: 24px 32px; }
.cards { display: grid; grid-template-columns: repeat(auto-fit, minmax(180px, 1fr)); gap: 16px; margin-bottom: 28px; }
.card { background: #1a1d24; border: 1px solid #2a2f3a; border-radius: 10px; padding: 18px; }
.card .label { color: #9aa3b2; font-size: 11px; text-transform: uppercase; letter-spacing: 0.6px; }
.card .value { font-size: 28px; font-weight: 600; margin-top: 4px; }
.card.err .value { color: #ff5d6c; }
.card.warn .value { color: #f5b042; }
.card.ok .value { color: #4ade80; }
.card.info .value { color: #5dade2; }
.health { background: linear-gradient(135deg, #1a1d24, #1f242e); border: 1px solid #2d65f0;
          border-radius: 12px; padding: 22px; margin-bottom: 28px; }
.health h2 { margin: 0 0 12px; font-size: 18px; }
.health .grid { display: grid; grid-template-columns: 1fr 1fr; gap: 24px; }
.health .field { font-size: 13px; }
.health .field .k { color: #9aa3b2; font-size: 11px; text-transform: uppercase; letter-spacing: 0.5px; }
.health .field .v { font-size: 15px; margin-top: 2px; }
.health .rootcause { background: #2a1a1a; border-left: 3px solid #ff5d6c; padding: 12px 16px;
                     border-radius: 6px; margin-top: 18px; }
.health .rootcause .label { color: #ff5d6c; font-weight: 600; font-size: 12px; text-transform: uppercase; letter-spacing: 0.5px; }
.health .rootcause .cause { font-size: 15px; margin: 6px 0 8px; font-weight: 500; }
.health .rootcause .action { color: #c9d1d9; font-size: 13px; }
.health.green { border-color: #4ade80; }
.health.red { border-color: #ff5d6c; }
.health.yellow { border-color: #f5b042; }
details { background: #15181f; border: 1px solid #2a2f3a; border-radius: 8px; margin-bottom: 14px; }
details > summary { padding: 14px 18px; cursor: pointer; font-weight: 600;
                    user-select: none; list-style: none; }
details > summary::marker, details > summary::-webkit-details-marker { display: none; }
details > summary::before { content: "▸ "; color: #2d65f0; }
details[open] > summary::before { content: "▾ "; }
details > .body { padding: 0 18px 18px; }
.tag { display: inline-block; padding: 3px 10px; border-radius: 4px; font-size: 11px;
       font-weight: 600; margin-right: 6px; }
.tag.pending, .tag.awaitingdatacopystart, .tag.awaitingcutoverstarttime, .tag.awaitingadmincutover, .tag.warning { background: #4a3a1a; color: #f5b042; }
.tag.validating, .tag.copyingblocks, .tag.copyingchangedblocks, .tag.convertingdisk, .tag.connectingtoesxi, .tag.creatinginitiatorgroup, .tag.creatingvolume, .tag.importingtocinder, .tag.mappingvolume, .tag.rescanningstorage, .tag.storageacceleratedcopyinprogress, .tag.running, .tag.copying { background: #1a3a52; color: #5dade2; }
.tag.succeeded, .tag.completed { background: #1a4a2e; color: #4ade80; }
.tag.failed, .tag.validationfailed { background: #4a1a1a; color: #ff5d6c; }
.tag.unknown { background: #2a2f3a; color: #9aa3b2; }
pre.log { background: #0a0c10; border: 1px solid #2a2f3a; border-radius: 6px;
          padding: 12px; font-size: 12px; line-height: 1.55;
          overflow-x: auto; max-height: 440px; white-space: pre; color: #c9d1d9; }
.line { display: block; }
.line.err { color: #ff5d6c; }
.line.warn { color: #f5b042; }
.line.ok { color: #4ade80; }
.line .lno { color: #6e7681; margin-right: 12px; user-select: none; min-width: 40px; display: inline-block; text-align: right; }
.line .hit { background: #ffd54f; color: #000; padding: 0 2px; border-radius: 2px; }
table { width: 100%; border-collapse: collapse; margin-top: 8px; font-size: 13px; }
th, td { text-align: left; padding: 8px 12px; border-bottom: 1px solid #2a2f3a; }
th { color: #9aa3b2; font-weight: 600; text-transform: uppercase; font-size: 10px; letter-spacing: 0.5px; cursor: pointer; user-select: none; }
th:hover { color: #e6e6e6; }
tr:hover td { background: #181b22; }
code { background: #0a0c10; padding: 2px 6px; border-radius: 3px; font-size: 12px; }
.timeline { position: relative; padding: 8px 0 8px 28px; border-left: 2px solid #2d65f0; margin-left: 8px; }
.timeline .item { position: relative; margin-bottom: 10px; padding: 6px 12px; background: #181b22; border-radius: 6px; font-size: 13px; }
.timeline .item::before { content: ""; position: absolute; left: -34px; top: 12px; width: 12px; height: 12px;
                          background: #2d65f0; border-radius: 50%; border: 2px solid #0f1115; }
.timeline .item .ts { color: #9aa3b2; font-size: 11px; margin-right: 8px; }
.timeline .item .icon { margin-right: 8px; }
.timeline .item.err::before { background: #ff5d6c; }
.timeline .item.warn::before { background: #f5b042; }
.timeline .item.ok::before { background: #4ade80; }
.cat-row { display: flex; align-items: center; padding: 6px 0; }
.cat-row .cat-name { width: 160px; font-weight: 600; }
.cat-row .cat-count { background: #2a2f3a; color: #c9d1d9; padding: 2px 10px; border-radius: 10px; font-size: 12px; }
.cat-row.empty { opacity: 0.4; }
.cat-row .cat-bar { flex: 1; height: 6px; background: #2a2f3a; border-radius: 3px; margin: 0 16px; overflow: hidden; }
.cat-row .cat-bar .fill { height: 100%; background: #ff5d6c; }
.muted { color: #6e7681; font-style: italic; }
.hidden { display: none !important; }
"""

JS = """
// Search box: filter log lines and table rows
(function() {
  const search = document.getElementById('search');
  if (!search) return;
  const allLines = Array.from(document.querySelectorAll('.line'));
  const allRows  = Array.from(document.querySelectorAll('tbody tr'));
  const status = document.getElementById('search-status');

  function highlight(text, q) {
    if (!q) return text;
    const re = new RegExp(q.replace(/[.*+?^${}()|[\\]\\\\]/g, '\\\\$&'), 'gi');
    return text.replace(re, m => `<span class="hit">${m}</span>`);
  }

  function apply() {
    const q = search.value.trim();
    let lineHits = 0, rowHits = 0;
    allLines.forEach(el => {
      const txt = el.dataset.raw || el.textContent;
      if (!el.dataset.raw) el.dataset.raw = txt;
      const t = el.dataset.raw;
      if (!q || t.toLowerCase().includes(q.toLowerCase())) {
        el.classList.remove('hidden');
        // restore + highlight
        const lno = el.querySelector('.lno');
        const lnoTxt = lno ? lno.outerHTML : '';
        el.innerHTML = lnoTxt + highlight(t.replace(/^.*?(?=[^\\s])/, m => ''), q);
        // simpler: just rebuild from raw
        if (q) {
          el.innerHTML = lnoTxt + highlight(t.replace(/^\\s*\\d+\\s+/, ''), q);
        }
        if (q) lineHits++;
      } else {
        el.classList.add('hidden');
      }
    });
    allRows.forEach(el => {
      const t = el.textContent;
      if (!q || t.toLowerCase().includes(q.toLowerCase())) {
        el.classList.remove('hidden');
        if (q) rowHits++;
      } else {
        el.classList.add('hidden');
      }
    });
    if (status) {
      status.textContent = q ? `${lineHits} log lines · ${rowHits} rows` : '';
    }
  }

  search.addEventListener('input', apply);
})();

// Sortable tables: click th to sort
document.querySelectorAll('table').forEach(table => {
  const ths = table.querySelectorAll('thead th');
  ths.forEach((th, idx) => {
    th.addEventListener('click', () => {
      const tbody = table.querySelector('tbody');
      if (!tbody) return;
      const rows = Array.from(tbody.rows);
      const asc = th.dataset.asc !== 'true';
      rows.sort((a, b) => {
        const av = a.cells[idx]?.textContent.trim() || '';
        const bv = b.cells[idx]?.textContent.trim() || '';
        const an = parseFloat(av), bn = parseFloat(bv);
        if (!isNaN(an) && !isNaN(bn)) return asc ? an - bn : bn - an;
        return asc ? av.localeCompare(bv) : bv.localeCompare(av);
      });
      rows.forEach(r => tbody.appendChild(r));
      ths.forEach(t => delete t.dataset.asc);
      th.dataset.asc = String(asc);
    });
  });
});
"""


def fmt_line(lno, line):
    cls = ""
    if ERROR_INDICATOR.search(line): cls = "err"
    elif WARN_INDICATOR.search(line): cls = "warn"
    elif SUCCESS_INDICATOR.search(line): cls = "ok"
    return f'<span class="line {cls}"><span class="lno">{lno}</span>{html.escape(line)}</span>'


def render_health(mname, m):
    phase = (m["status"] or {}).get("phase", "Unknown")
    rc = m.get("root_cause")
    duration = m.get("duration") or "?"
    spec = m.get("spec") or {}
    total_errors = sum(len(v) for v in m["error_buckets"].values())
    has_errors = total_errors > 0
    color = "green" if not has_errors and phase.lower() in ("succeeded", "completed") else ("red" if has_errors else "yellow")
    out = [f'<div class="health {color}">']
    out.append(f'<h2>{html.escape(mname)} <span class="tag {phase.lower()}">{html.escape(phase)}</span></h2>')
    out.append('<div class="grid">')
    out.append(f'<div class="field"><div class="k">Source VM</div><div class="v">{html.escape(str(spec.get("sourceVMName","?")))}</div></div>')
    out.append(f'<div class="field"><div class="k">Migration Type</div><div class="v">{html.escape(str(spec.get("migrationType","?")))}</div></div>')
    out.append(f'<div class="field"><div class="k">Duration (parsed)</div><div class="v">{html.escape(duration)}</div></div>')
    out.append(f'<div class="field"><div class="k">Disks copied</div><div class="v">{len(m.get("disks", {}))}</div></div>')
    out.append(f'<div class="field"><div class="k">CBT iterations</div><div class="v">{len(m.get("cbt_iters", []))}</div></div>')
    out.append(f'<div class="field"><div class="k">Errors in logs</div><div class="v">{total_errors}</div></div>')
    out.append('</div>')
    if rc:
        cause, action = rc
        out.append('<div class="rootcause">')
        out.append('<div class="label">Suggested root cause</div>')
        out.append(f'<div class="cause">{html.escape(cause)}</div>')
        out.append(f'<div class="action">{html.escape(action)}</div>')
        out.append('</div>')
    elif has_errors:
        out.append('<div class="rootcause" style="background:#1f1f1f;border-left-color:#9aa3b2">')
        out.append('<div class="label" style="color:#9aa3b2">No known pattern matched</div>')
        out.append('<div class="action">Review the categorized errors and event timeline below.</div>')
        out.append('</div>')
    out.append('</div>')
    return "\n".join(out)


def render_disks(disks):
    if not disks:
        return "<p class='muted'>No disk copy events parsed from logs.</p>"
    out = ["<table><thead><tr><th>Idx</th><th>Name</th><th>Started</th><th>Finished</th><th>Duration</th><th>CBT Iters</th><th>Status</th></tr></thead><tbody>"]
    for idx, d in sorted(disks.items(), key=lambda kv: kv[0]):
        out.append(f'<tr><td>{html.escape(d.index)}</td><td>{html.escape(d.name)}</td>'
                   f'<td>{html.escape(d.started or "—")}</td>'
                   f'<td>{html.escape(d.finished or "—")}</td>'
                   f'<td>{html.escape(d.duration or "—")}</td>'
                   f'<td>{d.cbt_iterations}</td>'
                   f'<td><span class="tag {d.status}">{html.escape(d.status)}</span></td></tr>')
    out.append("</tbody></table>")
    return "\n".join(out)


def render_timeline(events):
    if not events:
        return "<p class='muted'>No timeline events parsed from logs.</p>"
    out = ['<div class="timeline">']
    for e in events:
        cls = ""
        if "fail" in e.kind or "error" in e.kind: cls = "err"
        elif "complete" in e.kind or "done" in e.kind: cls = "ok"
        ts = f'<span class="ts">{html.escape(e.timestamp)}</span>' if e.timestamp else ''
        out.append(f'<div class="item {cls}">{ts}<span class="icon">{e.icon}</span><span>{html.escape(e.summary)}</span></div>')
    out.append('</div>')
    return "\n".join(out)


def render_cbt(iters):
    if not iters:
        return "<p class='muted'>No CBT sync iterations parsed (cold migration or no incremental data).</p>"
    out = ["<table><thead><tr><th>#</th><th>Timestamp</th><th>Disk</th><th>Duration</th><th>Progress</th></tr></thead><tbody>"]
    for i, it in enumerate(iters, start=1):
        out.append(f'<tr><td>{i}</td><td>{html.escape(it["timestamp"] or "—")}</td>'
                   f'<td>{html.escape(it["disk"])}</td><td>{html.escape(it["duration"])}</td>'
                   f'<td>{html.escape(it["progress"])}</td></tr>')
    out.append("</tbody></table>")
    return "\n".join(out)


def render_error_categories(buckets):
    if not buckets:
        return "<p class='muted'>No errors found.</p>"
    total = sum(len(v) for v in buckets.values())
    out = []
    # Sorted bar
    cat_order = [name for name, _ in ERROR_CATEGORIES] + ["Other"]
    for cat in cat_order:
        lines = buckets.get(cat, [])
        n = len(lines)
        pct = (n / total * 100) if total else 0
        empty = "empty" if n == 0 else ""
        out.append(f'<div class="cat-row {empty}"><div class="cat-name">{html.escape(cat)}</div>'
                   f'<div class="cat-bar"><div class="fill" style="width:{pct:.0f}%"></div></div>'
                   f'<div class="cat-count">{n}</div></div>')
    # Expandable details per category
    for cat in cat_order:
        lines = buckets.get(cat, [])
        if not lines: continue
        out.append(f'<details><summary>{html.escape(cat)} — {len(lines)} errors</summary><div class="body"><pre class="log">')
        for lno, line in lines[:50]:
            out.append(fmt_line(lno, line))
        if len(lines) > 50:
            out.append(f'<span class="line muted">... and {len(lines)-50} more</span>')
        out.append('</pre></div></details>')
    return "\n".join(out)


def render_pod_info(pod):
    pi = pod.get("pod_info")
    if not pi: return ""
    rc_color = "ok" if pi.restart_count == 0 else "err"
    out = ['<div class="cards" style="margin:12px 0">']
    out.append(f'<div class="card"><div class="label">Image</div><div class="value" style="font-size:13px">{html.escape(pi.image or "?")}</div></div>')
    out.append(f'<div class="card {rc_color}"><div class="label">Restarts</div><div class="value">{pi.restart_count}</div></div>')
    if pi.exit_reason:
        out.append(f'<div class="card err"><div class="label">Exit Reason</div><div class="value" style="font-size:14px">{html.escape(pi.exit_reason)}</div></div>')
    out.append('</div>')
    return "\n".join(out)


def render_mappings(rows, title):
    if not rows:
        return f"<p class='muted'>No {title} mappings defined.</p>"
    out = [f"<table><thead><tr><th>Mapping</th><th>Source</th><th>Target</th></tr></thead><tbody>"]
    for r in rows:
        out.append(f'<tr><td>{html.escape(r["mapping_name"])}</td>'
                   f'<td><code>{html.escape(str(r["source"]))}</code></td>'
                   f'<td><code>{html.escape(str(r["target"]))}</code></td></tr>')
    out.append("</tbody></table>")
    return "\n".join(out)


def render_k8s_events(events_txt):
    """Parse `kubectl get events` text-table output into a sortable HTML table."""
    if not events_txt.strip():
        return "<p class='muted'>(no events)</p>"
    lines = events_txt.strip().splitlines()
    if not lines: return "<p class='muted'>(no events)</p>"
    # Header is the first line
    header = lines[0]
    # Find column starts by header positions
    cols = re.split(r'\s{2,}', header.strip())
    rows = []
    for line in lines[1:]:
        if not line.strip(): continue
        parts = re.split(r'\s{2,}', line.strip(), maxsplit=len(cols)-1)
        if len(parts) < len(cols):
            parts += [""] * (len(cols) - len(parts))
        rows.append(parts)
    out = ["<table><thead><tr>"]
    for c in cols: out.append(f"<th>{html.escape(c)}</th>")
    out.append("</tr></thead><tbody>")
    for r in rows:
        tcol = ""
        for cell in r:
            if "Warning" in cell or "Error" in cell or "Failed" in cell: tcol = "err"
            elif "Normal" in cell: tcol = "ok"
        cls = f' class="line {tcol}"' if tcol else ""
        out.append(f"<tr{cls}>" + "".join(f"<td>{html.escape(c)}</td>" for c in r) + "</tr>")
    out.append("</tbody></table>")
    return "\n".join(out)


def render(data):
    out = []
    out.append("<!DOCTYPE html><html><head><meta charset='utf-8'>")
    out.append(f"<title>vjailbreak report — {html.escape(data['bundle_name'])}</title>")
    out.append(f"<style>{CSS}</style></head><body>")

    # Header with search box
    out.append("<header><div class='row'>")
    out.append("<div>")
    out.append(f"<h1>{html.escape(data['bundle_name'])}</h1>")
    out.append(f"<div class='meta'>generated {data['generated_at']}</div>")
    out.append("</div>")
    out.append("<div><input id='search' type='text' placeholder='Search logs and tables...'>"
               "<div id='search-status' style='font-size:11px;color:#9aa3b2;margin-top:4px;text-align:right'></div></div>")
    out.append("</div></header>")

    out.append("<div class='container'>")

    # Top summary cards
    n_mig = len(data["migrations"])
    n_errs = data["totals"]["errors"]
    n_creds = sum(len(v) for v in data["creds_redacted"].values())
    log_mb = data["totals"]["log_bytes"] / (1024*1024)
    out.append("<div class='cards'>")
    out.append(f"<div class='card info'><div class='label'>Migrations</div><div class='value'>{n_mig}</div></div>")
    out.append(f"<div class='card err'><div class='label'>Errors total</div><div class='value'>{n_errs}</div></div>")
    out.append(f"<div class='card ok'><div class='label'>Creds (redacted)</div><div class='value'>{n_creds}</div></div>")
    out.append(f"<div class='card'><div class='label'>Logs collected</div><div class='value'>{log_mb:.1f} <span style='font-size:14px;color:#9aa3b2'>MB</span></div></div>")
    out.append("</div>")

    # Per-migration deep dive
    for mname, m in data["migrations"].items():
        out.append(render_health(mname, m))

        out.append(f'<details open><summary>Disks ({len(m.get("disks", {}))})</summary><div class="body">')
        out.append(render_disks(m.get("disks", {})))
        out.append('</div></details>')

        out.append(f'<details><summary>Event timeline ({len(m.get("events", []))})</summary><div class="body">')
        out.append(render_timeline(m.get("events", [])))
        out.append('</div></details>')

        out.append(f'<details><summary>CBT sync iterations ({len(m.get("cbt_iters", []))})</summary><div class="body">')
        out.append(render_cbt(m.get("cbt_iters", [])))
        out.append('</div></details>')

        out.append(f'<details open><summary>Errors by category ({sum(len(v) for v in m["error_buckets"].values())})</summary><div class="body">')
        out.append(render_error_categories(m["error_buckets"]))
        out.append('</div></details>')

        for pod_name, pod in m["pods"].items():
            if pod["lines"] == 0: continue
            n_errs_pod = sum(len(v) for v in pod["error_buckets"].values())
            out.append(f'<details><summary>{html.escape(pod_name)} — {pod["lines"]} lines, {n_errs_pod} errors</summary><div class="body">')
            out.append(render_pod_info(pod))
            out.append("<h4>Tail (last 200 lines)</h4><pre class='log'>")
            for i, line in enumerate(pod["tail"].splitlines(), start=1):
                out.append(fmt_line(i, line))
            out.append("</pre></div></details>")

    # Cluster-wide
    if data["mappings"]["network"] or data["mappings"]["storage"]:
        out.append('<details><summary>Network & storage mappings</summary><div class="body">')
        out.append("<h4>Network mappings</h4>")
        out.append(render_mappings(data["mappings"]["network"], "network"))
        out.append("<h4 style='margin-top:18px'>Storage mappings</h4>")
        out.append(render_mappings(data["mappings"]["storage"], "storage"))
        out.append('</div></details>')

    if data["events_k8s"]:
        out.append('<details><summary>Kubernetes events</summary><div class="body">')
        for ns, txt in data["events_k8s"].items():
            out.append(f"<h4>{html.escape(ns)}</h4>")
            out.append(render_k8s_events(txt))
        out.append('</div></details>')

    if data["controller"]:
        out.append('<details><summary>Controller logs</summary><div class="body">')
        for name, c in data["controller"].items():
            n_errs_c = sum(len(v) for v in c["error_buckets"].values())
            out.append(f'<details><summary>{html.escape(name)} — {c["lines"]} lines, {n_errs_c} errors</summary><div class="body">')
            out.append(render_error_categories(c["error_buckets"]))
            out.append("<h4 style='margin-top:14px'>Tail (last 50 lines)</h4><pre class='log'>")
            for i, line in enumerate(c["tail"].splitlines(), start=1):
                out.append(fmt_line(i, line))
            out.append("</pre></div></details>")
        out.append('</div></details>')

    if data["creds_redacted"]:
        out.append('<details><summary>Credentials inventory (all redacted)</summary><div class="body">')
        out.append("<table><thead><tr><th>Kind</th><th>Name</th><th>Fields</th></tr></thead><tbody>")
        for kind, items in data["creds_redacted"].items():
            for it in items:
                fields = ", ".join(it["spec_keys"])
                out.append(f"<tr><td><code>{html.escape(kind)}</code></td><td>{html.escape(it['name'])}</td><td><code>{html.escape(fields)}</code></td></tr>")
        out.append("</tbody></table></div></details>")

    out.append('<details><summary>Cluster overview</summary><div class="body">')
    out.append(f"<pre class='log'>{html.escape(data['cluster_overview'])}</pre>")
    out.append('</div></details>')

    if data["settings"]:
        out.append('<details><summary>vjailbreak-settings</summary><div class="body">')
        out.append(f"<pre class='log'>{html.escape(data['settings'])}</pre>")
        out.append('</div></details>')

    out.append("</div>")
    out.append(f"<script>{JS}</script>")
    out.append("</body></html>")
    return "\n".join(out)


def main():
    p = argparse.ArgumentParser()
    p.add_argument("bundle", help=".tar.gz bundle or extracted directory")
    p.add_argument("-o", "--output", help="Output HTML file (default: report.html next to bundle)")
    args = p.parse_args()

    bundle_path = Path(args.bundle).resolve()
    root = extract_tarball(bundle_path)
    data = collect_bundle(root)
    html_out = render(data)

    out_path = Path(args.output) if args.output else bundle_path.parent / "report.html"
    out_path.write_text(html_out, encoding="utf-8")
    print(f"✅ Report: {out_path}")
    print(f"   Migrations: {len(data['migrations'])}  Errors: {data['totals']['errors']}  Logs: {data['totals']['log_bytes']/1024:.1f} KB")


if __name__ == "__main__":
    main()
