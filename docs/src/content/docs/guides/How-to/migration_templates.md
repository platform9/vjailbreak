---
title: Migration Templates
description: Save a migration configuration as a reusable template, then apply it to new migrations instead of filling in the form every time
---

Operators who migrate many VMs usually reuse the same configuration: the same source vCenter, the same destination cluster, the same network and storage mappings, the same copy method and cutover policy. A **Migration Template** saves that configuration once so every later migration can start from it.

A template stores everything the migration form asks for **except which VMs to migrate**. You pick the VMs fresh each time.

Templates only pre-fill the form. They do not change how a migration runs — a migration started from a template behaves exactly like one configured by hand.

## Where templates live

Open **Migrations** in the vJailbreak UI. It has two tabs, each with a count badge:

- **Migrations** — the existing migration list.
- **Templates** — saved templates.

The Templates tab toolbar sits inline with the tabs and offers:

| Control | Behavior |
|---|---|
| **Search** | Matches the template name and description |
| **Filter** (funnel icon) | Filters by migration type — Hot copy, Cold copy, or Mock copy. Choose **Clear filter** to remove it. |
| **Sort** | **Newest** (default) or **Name** |
| **Grid / list toggle** | Card grid (default) or a dense table |

## Prerequisites

At least one VMware credential and one PCD credential must exist. Until both are present, **Create New Template** and **Save as template** are disabled.

## What a template saves

| Group | Saved values |
|---|---|
| Source | VMware credential, source vCenter cluster |
| Destination | PCD credential, target PCD cluster |
| Mappings | Network mappings, storage mappings, datastore-to-array-credential mappings |
| Copy | Storage copy method (standard, vJailbreak Accelerated Copy with its proxy VM, or Storage Accelerated Copy), migration type (hot, cold, or mock), data copy start time |
| Cutover | Cutover option — immediate, time window, or admin-initiated — with its start and end times |
| Placement | Security groups, server group, DHCP fallback, disconnect source network |
| Post-migration | First boot script, post-migration actions (rename VM, move to folder) |
| Metadata | Preserve source tags, custom metadata key-value pairs |
| Advanced | Network persistence, remove VMware Tools, periodic sync and its interval, image profiles, network conflict acknowledgement, GPU flavor, data-only mode, guest OS family |

**Not saved**, by design:

- The VM selection, and anything attached to a specific VM — assigned IPs, MAC preservation, and per-VM flavor.

## Create a template

There are three ways to create one.

### From a migration you are configuring

1. Click **Start Migration** and fill in the form.
2. Click **Save as template** at the bottom left of the form.
3. Enter a name, optionally a description, and click **Save template**.

The form stays open — saving a template does not start or cancel the migration you were configuring.

### From the Templates tab

Use this when your goal is the template itself, not a migration.

1. Open the **Templates** tab and click **Create New Template**.
2. The migration form opens in **Create Template** mode. It differs from the normal form in three ways:
   - There is no **Select VMs** step.
   - The mapping step lists every network and datastore in the selected source cluster, instead of only those used by selected VMs.
   - Partial mappings are allowed — you do not have to map everything for the template to be valid.
3. Click **Create Template**, name it, and save.

No migration or migration plan is created by this flow.

### By cloning

Click the clone icon on a template card or list row. The copy is named `<original name> (copy)`, or `<original name> (copy) 2` if that name is taken. The original is untouched.

### Naming rules

Template names must be unique.

## Use a template

Click **Use** on a template card, list row, or in its detail drawer. The **Start Migration** form opens with the template's configuration applied, and every field remains editable.

You still choose the VMs. Because mappings are saved for the whole cluster while a migration maps only what the selected VMs use, vJailbreak keeps the template's mappings aside and applies them as they become relevant:

- Before you select any VM, saved mappings are not shown — none of their sources are in play yet.
- As you select VMs, each saved mapping whose source network or datastore appears, and whose target still exists on the destination, is applied automatically.
- De-selecting a VM removes the mappings only that VM needed. Re-selecting it brings them back.
- A mapping you delete by hand stays deleted, and is not re-applied when the VM list changes.

Submitting the form creates an ordinary migration. The template is not modified, and no link is kept between the two.

## View template details

Click a card or row (anywhere except an action button) to open the detail drawer:

- Name, description, and created date
- **Source & Destination** — source credentials, destination credentials, tenant or project, target cluster
- **Network & Storage Mappings** — every mapping pair, plus the storage copy method
- **Migration Options** — migration mode, cutover, guest OS
- **Advanced options** — each option that is set, with its value

The drawer also carries **Use**, **Edit**, **Clone**, and **Delete** actions.

## Edit a template

Templates are editable in place, so infrastructure changes do not force you to delete and recreate them.

1. Click the pencil icon on the card or list row, or **Edit** in the detail drawer.
2. The form opens in **Edit Template** mode, pre-filled the same way **Use** pre-fills it. As in Create Template mode, there is no VM step.
3. Change what you need and click **Save Changes**.

The same template is updated — no duplicate is created, and the name and position in the list stay the same. You can also change the name and description in the save dialog.

If someone else changed the same template while your form was open, the save fails with an error instead of silently overwriting their change. Reopen the template and reapply your edits.

## Delete a template

Click the trash icon on the card or list row, or **Delete** in the detail drawer, then confirm. Deleting a template does not affect migrations already created from it — they hold their own copy of the configuration and continue to run and display normally.

## Templates from the command line

Saved templates are `MigrationBlueprint` custom resources in the `migration-system` namespace:

```bash
kubectl -n migration-system get migrationblueprints
kubectl -n migration-system get migrationblueprint <name> -o yaml
```

These are separate from the internal, per-session `MigrationTemplate` objects that the migration form creates and cleans up on its own. See [vJailbreak CRDs](../../../reference/reference/#migrationblueprint) for the field list.

## Troubleshooting

| Symptom | Cause | What to do |
|---|---|---|
| **Create New Template** and **Save as template** are disabled | No VMware or no PCD credential exists | Add both credentials first |
| Cluster dropdowns are empty after clicking **Use** | The saved source or target cluster no longer exists, or its name changed | Select the clusters manually.
| Saved mappings do not appear after **Use** | Expected until VMs are selected | Select VMs. Mappings apply as their source networks and datastores come into scope. |
| A mapping never reappears after being removed | Mappings deleted by hand are not re-applied | Add the mapping again manually |
| Saving reports *"A template named … already exists"* | Names are unique, ignoring case | Pick a different name |
| **Save Changes** fails on an edit | The template changed elsewhere since the form opened | Reopen the template and reapply the change |
| Templates tab shows "No templates match" | A search term or type filter is active | Clear the search box, or choose **Clear filter** in the filter menu |

## Limitations

See [Known Limitations → Migration Templates](../../../reference/known-limitations/#migration-templates).

## References

- [Migration Options](../../../concepts/migration-options/)
- [Network & Storage Mapping](../../../concepts/network-storage-mapping/)
- [vJailbreak CRDs](../../../reference/reference/)
