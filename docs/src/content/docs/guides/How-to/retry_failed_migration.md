---
title: Retry a Failed Migration
description: Retry a failed migration from the vJailbreak UI, edit its configuration before retrying, or retry several failed migrations at once
---

When a migration ends in the **Failed** phase, you can retry it directly from the vJailbreak UI instead of building the migration again from scratch. A retry reopens the original configuration in the migration form, so you can correct whatever caused the failure — flavor, network or storage mapping, target cluster, cutover settings, or IP assignments — and start again.

## What Retry does

vJailbreak offers two retry actions:

| Action | Where to find it | What it does |
|---|---|---|
| **Retry** | Row action in the **Migrations** table, or the **Retry** button on a migration's detail page | Opens the migration form pre-filled with the failed migration's configuration. You can change settings before submitting. |
| **Retry Selected** | Toolbar of the **Migrations** table, after selecting rows | Retries every selected failed migration with its existing configuration. No editing. |

Use **Retry** when the migration failed because of a configuration problem. Use **Retry Selected** when several migrations failed for the same temporary reason — for example, an ESXi host or a target service was briefly unreachable — and nothing needs to change.

## Prerequisites

- The migration is in the **Failed** phase. The Retry button appears in no other phase.
- The migration is retryable. Migrations for VMs with **RDM (Raw Device Mapping) disks** cannot be retried from the UI, because the shared RDM state prevents an automatic retry. For these VMs the Retry button is visible but disabled.
- The resources the original migration used still exist: its migration plan, migration template, VMware and OpenStack credentials, and the source VM in the inventory. If any of them is missing, the retry form opens with a banner naming the missing item, and Retry stays disabled.

## Retry a single migration

1. Go to **Migrations** and locate the failed migration.
2. Click the **Retry** action in the row, or open the migration's detail page and click **Retry**.
3. The migration form opens in retry mode, titled **Retry Migration**, and loads the original configuration.
4. Review the settings and change what you need. See [What you can edit](#what-you-can-edit).
5. Click **Retry**.

vJailbreak returns you to the migrations list and starts a new migration for that VM using the configuration you submitted.

:::note
Clicking **Cancel** closes the form without changing anything. A cancelled retry never modifies the original migration, plan, or template.
:::


### What you can edit

**Locked during a retry**

- The VM being retried. A retry always applies to exactly one VM, and the VM list shows only that VM.
- Source and destination credentials, and the source cluster.

**Editable**

- **Target PCD cluster.** Changing it **clears the network and storage mappings**, because mappings are specific to a cluster. Select new mappings before submitting.
- Everything else — mappings, flavor, storage copy method, migration options, advanced options, and IP or MAC overrides.

## Retry several migrations at once

1. Go to **Migrations** and select the failed rows using the checkboxes.
2. The **Retry Selected (N)** button appears in the toolbar. It appears **only when every selected migration** is failed and retryable. If the selection includes a migration that is not failed, or a VM with RDM disks, the button is hidden.
3. Click **Retry Selected**. A confirmation dialog appears, stating that the migrations will be retried without changing their configurations and that source VMs will not be modified.
4. Click **Retry** to confirm.

Each selected migration restarts with its existing configuration. Plans, templates, and mappings are left untouched.


## Troubleshooting

| Symptom | Cause | What to do |
|---|---|---|
| Retry is disabled and the tooltip mentions RDM disks | The VM has RDM disks and cannot be retried from the UI | Restart the migration manually. See [Migrating an RDM disk Windows cluster machine using the CLI](../../cli-api/migrating_rdm_disk_windows_cluster_machine_using_cli/). |
| Banner: *"Migration plan … no longer exists"* | The plan was deleted after the migration failed | Create a new migration for that VM |
| Banner naming a VMware or OpenStack credential | The credential was deleted | Recreate the credential, then retry |
| Banner: *"Source VM … is no longer present in the inventory"* | The VM is missing from the VMware inventory, usually after a re-sync or after the VM was removed | Refresh the inventory or add the VM back, then retry |
| **Retry failed** banner after submitting | A step of the retry did not complete | The banner shows the underlying error. If the original plan could not be updated, nothing new was created and the original plan is intact — correct the problem and retry again. |
| Network and storage mappings are empty | Expected after changing the target cluster | Select mappings for the new cluster |

## Limitations

- A retry always creates a plan containing a single VM. You cannot retry several VMs into one shared plan with edits.
- **Retry Selected** cannot change configuration. If a migration fails again after a bulk retry, retry it individually and correct its settings.
