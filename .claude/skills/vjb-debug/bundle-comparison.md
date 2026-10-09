# Comparing Two Debug Bundles (Success vs. Fail, or Fail vs. Fail)

The dominant real-world entry point for this skill is an **offline debug-bundle tarball**
(`/vjb-debug <bundle>.tar.gz`), often with a second bundle from a different run of the same VM
handed over mid-conversation ("here's a successful run of the same VM"). This file covers that
specific workflow — see [support-bundle.md](support-bundle.md) for the bundle layout itself.

## Step 1: Confirm identity BEFORE comparing

**Never assume two bundles are the same VM because a filename looks similar, and never assume
they're different VMs because filenames differ.** Bundle filenames are often just
`<something>-debug-bundle-<timestamp>.tar.gz` — the timestamp differing is expected for two runs
of the same VM, not evidence of anything.

Confirm identity from bundle contents, in priority order:
1. VM UUID / instance UUID in the `Migration` CR YAML (`crds/migration-<name>.yaml`) or in the
   per-migration debug log header.
2. VM MAC address(es) — consistent across runs of the same source VM.
3. Migration/VM name string, if present and not auto-generated per-attempt.

If the user states two bundles are the same VM, take it as given — do not re-litigate it — but
still use it to anchor the comparison (e.g., disk device paths should be compared 1:1 by role, not
by raw device letter, since those can shift between attempts).

## Step 2: Extract both bundles in parallel, not serially

Extracting and building a phase timeline from one bundle is independent of doing the same for the
other. Per SKILL.md's parallel-fan-out rule: dispatch **one agent per bundle**, in the same
message, each producing a structured phase-by-phase timeline (phase, timestamp, key log lines,
errors). Only after both return should the diff happen — do this centrally, not inside either
agent (neither agent has the other's data).

Do not do this serially in the main thread with dozens of individual `tar`/`grep`/`cat` calls
across both bundles interleaved — that was the actual failure mode observed in a real session
(65 solo Bash calls to compare two bundles by hand).

## Step 3: Diff phase-by-phase, not error-string-by-error-string

Comparing only the final error line misses the point where the two runs diverge. For each phase
(Discovery → Mapping → Validate → Data Copy → Convert → Cutover → Post-Migration), compare:
- Did the phase start/end at all in each run?
- Same copy method (Normal NFC / XCOPY / Hot-Add Proxy)? Different methods are a legitimate
  confound — note it, don't assume it's the cause, and don't assume it's irrelevant either. Ask
  the user which variable they want held constant if it's ambiguous.
- Same detected guest OS / same device layout (disk count, boot disk index)? Device paths
  (`/dev/vdb` vs `/dev/vdc`) can shift between attempts even for the same VM — compare by role
  (boot disk, data disk N), not raw letter.
- First point of divergence in log timestamps — this is usually more informative than the final
  error line.

## Step 4: State findings as evidence vs. hypothesis

Per SKILL.md's independent-evidence rule: after the diff, separate what the logs directly show
(e.g., "failed run's copy method field is empty, successful run's is `XCOPY`") from what you
believe that means (e.g., "this suggests the copy method affects X — unconfirmed, needs Y to
verify"). Do not present the second as if it were the first.
