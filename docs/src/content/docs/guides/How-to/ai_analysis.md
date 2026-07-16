---
title: AI-Powered Migration Failure Analysis
description: Use the built-in AI assistant to diagnose failed migrations and get remediation steps
---

vJailbreak includes an AI analysis feature that inspects failed migration logs, Kubernetes resource conditions, and known failure patterns to identify root causes and suggest fix steps — without manual log triage.

:::note
AI analysis is experimental and requires an Anthropic API key configured in **Settings → AI**.
:::

## Prerequisites

- vJailbreak v0.4.8 or later
- A failed migration (phase: `Failed` or `ValidationFailed`)
- An Anthropic API key — get one at [console.anthropic.com](https://console.anthropic.com)

## Setup

### 1. Configure the Anthropic API key

1. Navigate to **Settings → AI** in the vJailbreak UI.
2. Enter your Anthropic API key (`sk-ant-...`).
3. Enter the vJailbreak admin key (generated at first boot — check `install.sh` output or deployment notes).
4. Click **Save API Keys**.

The key is stored in a Kubernetes Secret in `migration-system` and never exposed after saving. The AI service restarts automatically to pick up the new key.

### 2. Verify the AI service is running

```bash
kubectl -n migration-system get pods -l app=vjailbreak-ai
```

The pod should be in `Running` state. If not, check logs:

```bash
kubectl -n migration-system logs -l app=vjailbreak-ai
```

## Using AI Analysis

### From the migration detail page

1. Open a failed migration from the **Migrations** list.
2. Click the **AI Analysis** tab (marked *Experimental*).
3. Click **Analyse with AI**.

The AI collects:
- Migration CR conditions (primary signal)
- v2v-helper pod exit code and logs
- Controller logs
- Credential validation status
- MigrationPlan spec and template config

It then returns a structured result:

| Field | Description |
|-------|-------------|
| **Root Cause** | One-sentence description of the failure |
| **Fix Steps** | Ordered remediation steps (max 5) |
| **Confidence** | `high` / `medium` / `low` / `none` |
| **Doc References** | Links to relevant documentation |

### From the migration logs drawer

In the migration logs drawer, an **AI Analysis** tab appears alongside the **Logs** tab. Click it to run the same analysis without leaving the log view.

### Follow-up questions

After the initial analysis, ask follow-up questions in the text field at the bottom. The AI retains the analysis context for the conversation.

Example questions:
- "What does exit code 137 mean and how do I fix it?"
- "How do I check if CBT is enabled?"
- "Can I retry without changing the VDDK version?"

### Feedback

Use the **thumbs up / thumbs down** buttons to rate the analysis. Feedback helps improve future analyses.

### Opening a GitHub issue

If the root cause is identified, click **Open GitHub Issue** to pre-fill an issue with the migration conditions and error excerpt. If confidence is `none`, the AI provides a checklist of data to collect before filing the issue.

## Confidence levels

| Level | Meaning |
|-------|---------|
| `high` | Pattern matched exactly; fix is known and confirmed by logs |
| `medium` | Phase is clear but exact cause uncertain, or logs are partial |
| `low` | Phase identified only; logs missing or too ambiguous |
| `none` | Cannot determine phase or cause from available signals |

When confidence is `low` or `none`, the first fix step is always diagnostic (gather more data before acting).

## Known patterns the AI detects

| Symptom | Root cause | Fix |
|---------|-----------|-----|
| `"exec: already started"` | nbdkit/VDDK process init race or stale socket | Verify VDDK path, clean up and retry |
| Pod exit code 137 | OOM kill | Larger VM flavor or reduce concurrent migrations |
| Pod exit code 139 | Segfault in virt-v2v / libguestfs | Check VDDK version vs ESXi version compatibility |
| `"Failed to connect"` + VMware/ESXi | DNS not resolving for ESXi host | Add ESXi entries to `/etc/hosts` on vJailbreak VM |
| `"permission denied"` or `"401"` + OpenStack | Expired or wrong credentials | Revalidate OpenStack credentials |
| `"No space left on device"` | Scratch disk too small for VMDK | Expand `/var` or configure larger scratch space |
| `"CBT"` / `"Changed Block Tracking"` not enabled | Hot migration prerequisite missing | Enable CBT on source VM in vCenter |
| `"VDDK error"` + error code | VDDK library mismatch or license issue | Verify VDDK version matches ESXi; check VDDK path |

## Troubleshooting the AI feature

### "Anthropic API key not configured"

Navigate to **Settings → AI** and save a valid API key.

### "AI service unavailable"

```bash
kubectl -n migration-system get pods -l app=vjailbreak-ai
kubectl -n migration-system logs -l app=vjailbreak-ai --tail=50
```

Check that the `ANTHROPIC_API_KEY` environment variable is populated in the pod — it is injected from the `vjailbreak-ai-secret` Secret.

### Analysis returns confidence "none"

The AI could not identify the root cause from available logs. Collect the following before filing an issue:

- Full debug logs (use the **Download** button in the migration logs drawer)
- `journalctl -u libvirtd -n 200` from the vJailbreak VM
- ESXi host version and vCenter version
- Whether CBT is enabled on the source VM
- VDDK library path: `ls -la /home/ubuntu/vmware-vix-disklib-distrib/`
- vJailbreak version: `kubectl -n migration-system get deployment migration-controller-manager -o jsonpath='{.spec.template.spec.containers[0].image}'`
