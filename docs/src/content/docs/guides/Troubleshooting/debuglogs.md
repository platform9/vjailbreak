---
title: "Debug Logs"
description: "Learn how vJailbreak collects and stores migration debug logs directly on the host, and how to download a full debug bundle from the UI."
---


This guide outlines how vJailbreak handles debug log collection for VM migrations. Traditionally, enabling debug logs required editing ConfigMaps and restarting pods. With the current setup, debug logs are automatically collected and stored without any manual intervention. In kubectl logs of the pod, normal logs will be displayed as usual.

## How It Works

- For every migration executed via vJailbreak, debug logs are written to the host system under `/var/log/pf9`.
- A single combined log file is written for the migration at:

  `/var/log/pf9/<migration-name>.log`

- In addition, logs are now **split by category** into a dedicated directory for the migration:

  `/var/log/pf9/<migration-name>/<category>.<timestamp>.log`

  Each category captures the output of a specific part of the migration, so an issue can be traced straight to the relevant subsystem instead of scanning one combined file:

  | Category  | Contents                                             |
  |-----------|-------------------------------------------------------|
  | `nbd`     | NBD/`nbdcopy` disk-copy commands and their output      |
  | `virtv2v` | `virt-v2v` conversion commands and their output        |
  | `general` | Everything else run during the migration               |

- These logs are centrally accessible from the **vjailbreak node**, simplifying the debugging process.

## Log File Location

| Node Type          | Path                                              | Description                                          |
|--------------------|----------------------------------------------------|-------------------------------------------------------|
| vjailbreak-master   | `/var/log/pf9/<migration>.log`                     | Combined log for the migration                        |
| vjailbreak-master   | `/var/log/pf9/<migration>/<category>.<timestamp>.log` | Per-category split logs (`nbd`, `virtv2v`, `general`) |

## Example

If a migration is named `vm-migrate-001`, its logs will be available at:

- `/var/log/pf9/vm-migrate-001.log` — combined log
- `/var/log/pf9/vm-migrate-001/nbd.2026-08-25-10:15:00.log` — disk-copy log
- `/var/log/pf9/vm-migrate-001/virtv2v.2026-08-25-10:20:00.log` — conversion log

in the vjailbreak node.

## Downloading a Debug Bundle from the UI

Instead of SSHing into the vjailbreak node to collect logs manually, you can download a full debug bundle directly from the Migration Details page.

| Location                   | What the button does                                                                                                                      | Screenshot |
|-----------------------------|----------------------------------------------------------------------------------------------------------------------------------------|------------|
| Migration Details → Debug Logs tab | Downloads a `.tar.gz` bundle containing the pod's `stdout`/`stderr` logs, the related Kubernetes resource YAMLs, and the split debug log files described above from `/var/log/pf9`. | _screenshot coming soon_ |

:::note
Screenshot to be added.
:::

The bundle is named `<vm-name>-debug-bundle-<timestamp>.tar.gz` and is downloaded straight from the browser — no extra `kubectl` or SSH access is required. If a collection step fails (e.g. the debug logs directory isn't available on that node), the bundle still downloads and includes a `collection-warnings.txt` file describing what was skipped.
