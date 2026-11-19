---
title: Migration fails due to NFC memory exhaustion on the source ESXi host
description: Troubleshooting VM data-copy failures (including NFC_COMPRESSION_ERROR) caused by the ESXi host's NFC (Network File Copy) memory limit
---

## Problem

A migration fails during the VM data-copy phase with an error indicating that the ESXi host's **NFC (Network File Copy)** service has run out of memory.

This only affects the **Standard** copy method, which reads disks over NFC through nbdkit's VDDK plugin. [vJailbreak Accelerated Copy](../../../concepts/vjailbreak-accelerated-copy/) (the default since v0.4.10) and [Storage Accelerated Copy](../../../concepts/storage-accelerated-copy/) don't use NFC at all — Accelerated Copy reads disks via `qemu-nbd` on the Proxy VM, and Storage Accelerated Copy uses array-level XCOPY — so neither is affected by this issue.

VMware surfaces this as a session fault named **`NFC_COMPRESSION_ERROR`**:

```text
The session is in a faulted state: NFC_COMPRESSION_ERROR
```

The underlying `hostd`/ESXi-side error behind that fault (in `hostd` logs, or surfaced in the vCenter/ESXi task) is:

```text
Cannot allocate anymore memory as NFC is already using <n> and allocating <m> will make it more than the maximum allocated
```

or

```text
Failed to allocate the requested <n> bytes
```

vJailbreak's own migration failure reason comes from the second form (`[NFC error] NfcFssrvrProcessErrorMsg: ... failed to allocate ...`), which it pattern-matches from the copy-phase debug log. `NFC_COMPRESSION_ERROR` is the same NFC memory exhaustion described below, not a separate compression-specific bug — other NFC clients (for example backup/replication tools) surface it under that name.

## Symptoms

- Migration fails during disk copy, not at an earlier validation or snapshot step.
- The failure is more likely when several migrations (or other NFC consumers, such as backup/replication jobs) run against the **same ESXi host** at the same time.
- Retrying the same VM on a **different** ESXi host in the cluster often succeeds, which points at a per-host resource limit rather than a problem with the VM or with vJailbreak.

## Root Cause

Each ESXi host's `hostd` service enforces a memory limit for the NFC service (`nfcsvc`). The default limit is **100663296 bytes (~96 MB)** on current ESXi releases (older releases default to ~48 MB). Every concurrent NFC session (migrations, clones, backups, replication) draws from this shared pool. Once enough sessions are active, or stale sessions haven't been cleaned up, the host can't allocate a buffer for a new transfer and the copy fails.

This is an **ESXi host-side configuration limit**, not a vJailbreak bug or a VM-specific issue. vJailbreak already caps every Standard-copy read request at a conservative size (2 MiB or less, tighter still if the copy pod's memory budget is tight) as a best-effort way to lower the odds of hitting this limit, and it recognizes the error to report it as a clear migration failure reason. That mitigation reduces the chance of running into the limit; it doesn't remove it; under enough concurrent NFC sessions the host can still exhaust the limit, and the actual fix is applied on the ESXi host, not in vJailbreak.

## Resolution

Raise the NFC memory limit on the affected ESXi host, then retry the migration.

**ESXi 7.0 U2 and later** (NFC settings are managed through the config store):

```bash
# Export the current hostd config
/bin/configstorecli config current get -c esx -g services -k hostd -outfile tmp.json

# Edit tmp.json and increase maxMemory under the nfcsvc section, e.g.:
#   "maxMemory": 150663296
vi tmp.json

# Apply the updated config
/bin/configstorecli config current set -c esx -g services -k hostd -infile tmp.json

# Restart hostd for the change to take effect
/etc/init.d/hostd restart
```

**ESXi versions before 7.0 U2** (edit `hostd`'s config file directly):

```bash
vi /etc/vmware/hostd/config.xml
```

Increase `maxMemory` (bytes) in the `<nfcsvc>` section, then restart `hostd`:

```xml
<nfcsvc>
  <path>libnfcsvc.so</path>
  <enabled>true</enabled>
  <maxMemory>150663296</maxMemory>
  <maxStreamMemory>35651584</maxStreamMemory>
</nfcsvc>
```

```bash
/etc/init.d/hostd restart
```

:::note
`hostd` is the ESXi management agent, not the VMkernel that runs VMs, so restarting it does not power off or affect already-running VMs. It does interrupt any management operation in flight on that host at the time (for example an active clone, backup, or vMotion), so avoid restarting it while one is running.
:::

After raising the limit, re-run the migration. See [Retry a Failed Migration](../../how-to/retry_failed_migration/).

## Prevention

- If several ESXi hosts in the cluster are used as migration sources, proactively raise the NFC memory limit on **all** of them, not just the host that failed, to avoid hitting the same issue on the next VM.
- Avoid running a large number of migrations (or other NFC-consuming operations such as backups/replication) against a single ESXi host at the same time; spreading the load across hosts, or using the [data copy start time](../../../concepts/migration-options/#data-copy-start-time) option to stagger migrations, reduces concurrent NFC sessions per host.
- Size the limit to your migration plan, not just the 150663296-byte example above: NFC service memory on a host is shared across its concurrent connections, so a migration plan that moves many VMs off the same ESXi host at once needs a higher limit than one moving a handful. Red Hat's Forklift/MTV documentation, migrating VMs from the same kind of ESXi NFC service, recommends raising `maxMemory` well beyond the default when moving more than 10 VMs from one host in the same plan, since NFC service memory is otherwise shared across roughly 10 parallel connections; size proactively rather than reusing the example value above for a large plan.

## Notes

- This is a known VMware/ESXi constraint, not specific to vJailbreak. See the [NFC_SESSION_ERROR knowledge base article](https://knowledge.broadcom.com/external/article/389223/virtual-machine-replication-fails-with-e.html), the [NFC_COMPRESSION_ERROR knowledge base article](https://knowledge.broadcom.com/external/article/393216/rubrik-vm-backup-failing-with-nfccompres.html) (reported against a third-party backup product, but the same ESXi-side root cause and fix), and the [`virt-v2v-input-vmware` documentation](https://libguestfs.org/virt-v2v-input-vmware.1.html) for background.
- The NFC memory limit lives on the ESXi host's `hostd` regardless of whether the client connects directly to the host or routes through vCenter; there's no separate, independently-configurable limit for the vCenter-routed path.
