---
title: Configure Time Zone and NTP Servers
description: Set the vJailbreak appliance time zone and NTP servers from Global Settings, and verify they applied to the host and to system pods
---

The vJailbreak appliance runs in UTC with default public NTP pools unless you change it. Two settings in the UI change that:

- **Time zone** — the appliance's system time zone. It sets the timestamps in migration logs, controller logs, Grafana dashboards, and the schedule of the version-checker cron job.
- **NTP servers** — the time sources the appliance synchronizes against. Needed in air-gapped or restricted networks where the default public pools are unreachable.

Keeping the appliance's clock accurate matters beyond readable logs: clock drift makes Changed Block Tracking timestamps unreliable during hot migrations.

Both settings are applied to the appliance host itself, not to migrated VMs.

## Configure the settings

1. Open **Global Settings**.
2. On the **General** tab, pick a **Timezone**. The dropdown is searchable and lists common IANA zones with their current UTC offset.
3. On the **Advanced** tab, enter **NTP Servers** — hostnames or IPv4 addresses, separated by commas or new lines. For example: `ntp1.corp.local, ntp2.corp.local`.
4. Click **Save**.

Saving writes both values to the `vjailbreak-settings` ConfigMap and then applies them to the host. You will see *"Applying time settings..."* followed by *"Time settings applied successfully."*

### What each combination does

| Time zone | NTP servers | Result |
|---|---|---|
| Set | Set | Host uses that zone, synchronizing against your servers |
| Set | Empty | Host uses that zone, synchronizing against the default public pools |
| Empty | Set | Host stays on UTC, synchronizing against your servers |
| Empty | Empty | Host reverts to UTC and time synchronization is turned off |

Clearing both fields is therefore a full reset to the appliance defaults.

### Validation

The form rejects a save when an entry is malformed:

- A time zone must be one of the listed zones. If the appliance already holds a zone that is not in the list, it appears as `(Legacy) <zone>` so it is not silently discarded.
- Each NTP entry must be a hostname or an IPv4 address. URLs, entries containing `/`, and malformed hostnames are rejected with *"Invalid NTP server "…". Use hostnames or IPv4 addresses, separated by commas or new lines."*

## Both fields lock while migrations run

If any migration is in a non-terminal phase — anything other than Succeeded, Failed, Validation Failed, or Unknown — the **Timezone** and **NTP Servers** fields are disabled, with the tooltip *"Cannot change timezone while migrations are in progress."*

Applying time settings restarts the controller, SDK, and UI pods, which would disrupt a running migration. The migration list is polled every 30 seconds, so the fields unlock shortly after the last migration reaches a terminal phase.

**Reset to Defaults** respects the same rule: while a migration is running it resets every other setting but leaves the time zone and NTP servers at their current values. With no migration running, it resets them along with everything else.

## Verify the settings

On the appliance host:

```bash
# Time zone, and whether the clock is synchronized
timedatectl
timedatectl show --property=NTPSynchronized

# The NTP servers vJailbreak wrote
cat /etc/systemd/timesyncd.conf.d/99-vjailbreak.conf
```

The conf file looks like this:

```ini
[Time]
NTP=ntp1.corp.local ntp2.corp.local
```

From Kubernetes:

```bash
# What was saved
kubectl -n migration-system get configmap vjailbreak-settings \
  -o jsonpath='{.data.TIMEZONE}{"\n"}{.data.NTP_SERVERS}{"\n"}'

# What pods will inherit
kubectl -n migration-system get configmap pf9-env -o jsonpath='{.data.TZ}{"\n"}'

# What a running pod actually has
kubectl -n migration-system exec deploy/migration-controller-manager -- printenv TZ
```

A rolling restart takes a couple of minutes to finish, so give the pods time before concluding that `TZ` did not propagate.

## Configure from the command line

The two values live in the `vjailbreak-settings` ConfigMap:

| Key | Format | Empty means |
|---|---|---|
| `TIMEZONE` | IANA zone, for example `Asia/Calcutta` | Use UTC |
| `NTP_SERVERS` | Hostnames or IPv4 addresses, space separated | Use the default public pools |

```bash
kubectl -n migration-system patch configmap vjailbreak-settings --type merge \
  -p '{"data":{"TIMEZONE":"Asia/Calcutta","NTP_SERVERS":"ntp1.corp.local ntp2.corp.local"}}'
```

:::caution
Editing the ConfigMap only records the values. Nothing reaches the host until the apply step runs, which the UI triggers on **Save**. After editing the ConfigMap directly, open Global Settings and save, or call the apply endpoint yourself:

```bash
curl -X POST http://<vjailbreak-vm-ip>/dev-api/sdk/vpw/v1/settings/apply-time-settings \
  -H 'Content-Type: application/json' -d '{}'
```
:::

Values written by hand skip the form's validation. Invalid NTP entries are dropped when the settings are applied, and only the valid ones reach the conf file.

## Troubleshooting

| Symptom | Cause | What to do |
|---|---|---|
| Timezone and NTP Servers fields are greyed out | A migration is in a non-terminal phase | Wait for it to finish, or cancel it. The fields unlock within about 30 seconds. |
| **Reset to Defaults** left the time zone unchanged | Expected while a migration is running | Reset again once no migration is active |
| Save reports *"Failed to apply time settings on the host … Settings were saved; click Save again to retry the apply."* | The values were stored, but the host apply failed | Click **Save** again. If it keeps failing, check the API response. |
| UI reports success, but `timedatectl` still shows the old zone | The host notification is best-effort and its failure is not surfaced in the UI | Check the logs for `timesettings:` warnings. A zone that is syntactically valid but not installed on the host fails at this step. |
| An NTP server you entered is missing from the conf file | It failed validation and was dropped | Check the logs for `ignoring invalid NTP server entries`. Re-enter it as a plain hostname or IPv4 address. |
| Pods still show the old `TZ` | The rolling restart has not finished | Wait, then re-check with `kubectl rollout status` |
| The conf file is gone after a save | Expected when the NTP Servers field is empty | Re-enter your servers, or leave it empty to use the default pools |

Logs for the apply step:

```bash
kubectl -n migration-system logs deploy/migration-vpwned-sdk | grep timesettings
```

## Notes and limitations

- These settings change the **appliance**, not the VMs being migrated. Migrated guests keep their own time configuration.
- Failures after the conf file is written — host notification, service restart, pod restarts, cron job patch — never fail the request and are not shown in the UI. The success message means the settings were saved and applied as far as possible.
- A time zone that is not installed on the appliance is stored and reported as applied, but the host time zone does not change. Pick a zone from the dropdown to avoid this.
- Invalid NTP entries are silently dropped at apply time. The UI blocks them first, so this affects only values written directly to the ConfigMap.
