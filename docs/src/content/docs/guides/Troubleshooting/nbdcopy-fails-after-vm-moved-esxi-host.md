---
title: nbdcopy fails during disk copy (often DNS resolution)
description: Troubleshooting disk copy failures caused by missing DNS/hosts entries
---

## Problem

A migration fails during the disk copy (live replicate) phase with an error similar to:

```text
Failed to migrate VM: failed to live replicate disks: failed to copy disk Hard disk 1 (DeviceKey=2000): failed to run nbdcopy: exec: already started.
```

Error signature:
```text
failed to run nbdcopy: exec: already started
```

## Symptoms

- Migration fails during the **nbdcopy** phase.
- Debug logs often show **DNS resolution errors** when attempting to connect to an ESXi host.

## Root Cause

During the disk copy phase, vJailbreak needs to communicate with ESXi hosts. If name resolution for an ESXi host is not available from the vJailbreak VM, the nbdcopy workflow can fail.

This is commonly caused by missing DNS records or missing `/etc/hosts` entries for ESXi hosts.

## Resolution

1. Review the debug logs to confirm DNS/name-resolution errors.

See: [Debug Logs](../debuglogs/).

2. Ensure the vJailbreak VM can resolve ESXi host names.

If you are not using DNS, add a static entry on the vJailbreak VM:

```bash
sudo sh -c 'echo "<esxi-host-ip> <esxi-host-fqdn> <esxi-host-shortname>" >> /etc/hosts'
```

3. Re-run the migration.

## Prevention

- Ensure DNS (or `/etc/hosts`) is configured for **all ESXi hosts** in the cluster, not just vCenter.

