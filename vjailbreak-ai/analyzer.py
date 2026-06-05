# vjailbreak-ai/analyzer.py
import json
import re
import urllib.parse
from typing import Any

import anthropic

SYSTEM_PROMPT = """\
You are an expert vJailbreak migration failure analyst.
vJailbreak migrates VMs from VMware to OpenStack using virt-v2v, libguestfs, and VDDK.

Analyze the provided migration logs and Kubernetes resource conditions.
Always respond with a JSON object and nothing else. The JSON must have these exact keys:
- root_cause: string (one concise sentence) or null if unknown
- fix_steps: list of strings (ordered remediation steps) or empty list
- summary: string (2-3 sentences explaining the failure)
- confidence: one of "high", "medium", "low", "none"
- doc_references: list of URLs from the provided documentation that were relevant

Set confidence "none" only when you truly cannot identify any likely cause.
"""

FOLLOWUP_SYSTEM_PROMPT = """\
You are an expert vJailbreak migration failure analyst having a conversation.
vJailbreak migrates VMs from VMware to OpenStack using virt-v2v, libguestfs, and VDDK.

The conversation history contains a prior analysis of a failed migration.
Answer the user's follow-up question based on that analysis.
Be concise and specific. Do not repeat the full analysis unless explicitly asked.

IMPORTANT: Respond ONLY in plain prose text. Do NOT output JSON, code blocks, or structured data.
"""

GITHUB_COLLECT_FIRST = [
    "Run `journalctl -u libvirtd -n 200` on the vJailbreak VM and save the output",
    "Download full debug logs using the Download button in the migration logs drawer",
    "Note the ESXi host version and vCenter version",
    "Check whether Changed Block Tracking (CBT) is enabled on the source VM in vCenter",
    "Verify VDDK library path: `ls -la /home/ubuntu/vmware-vix-disklib-distrib/`",
    "Note the vJailbreak version: `kubectl -n migration-system get deployment migration-controller-manager -o jsonpath='{.spec.template.spec.containers[0].image}'`",
]


def extract_error_keywords(logs: str) -> list[str]:
    """Extract up to 10 error/failed lines for RAG query."""
    if not logs:
        return []
    lines = [ln for ln in logs.splitlines() if re.search(r"ERROR|FAILED", ln, re.IGNORECASE)]
    return lines[:10]


def build_user_message(context: dict[str, Any], rag_context: str) -> str:
    migration_cr = context.get("migration_cr", {})
    status = migration_cr.get("status", {})
    conditions = status.get("conditions", [])

    sections = [
        "## Migration CR Status",
        f"Phase: {status.get('phase', 'unknown')}",
        f"Conditions:\n{json.dumps(conditions, indent=2, default=str)}",
        "",
        "## Migration Template Config (credentials redacted)",
        json.dumps(context.get("migration_template", {}), indent=2, default=str),
        "",
        "## MigrationPlan",
        json.dumps(context.get("migration_plan", {}), indent=2, default=str),
        "",
        "## v2v-helper Pod Logs (extracted)",
        context.get("v2v_logs") or "(none)",
        "",
        "## Controller Logs (extracted)",
        context.get("controller_logs") or "(none)",
        "",
    ]

    debug_logs = context.get("debug_logs", {})
    if debug_logs:
        sections.append("## Debug Logs from /var/log/pf9")
        for fname, content in debug_logs.items():
            sections.append(f"### {fname}")
            sections.append(content or "(empty)")
            sections.append("")

    additional_context = context.get("additional_context", "")
    if additional_context:
        sections.append("## Operator-Provided Context")
        sections.append(additional_context)
        sections.append("")

    fetch_warnings = context.get("fetch_warnings", [])
    if fetch_warnings:
        sections.append("## Fetch Warnings (some data may be missing)")
        for w in fetch_warnings:
            sections.append(f"- {w}")
        sections.append("")

    if rag_context:
        sections.append("## Relevant Documentation")
        sections.append(rag_context)
        sections.append("")

    sections.append("Analyze the failure and return the JSON response as described.")
    return "\n".join(sections)


def build_github_issue(migration_name: str, conditions: list, error_snippet: str) -> dict:
    title = f"Migration failure: {migration_name}"
    body_lines = [
        f"## Migration: `{migration_name}`",
        "",
        "### Migration Conditions",
        "```json",
        json.dumps(conditions, indent=2, default=str)[:3000],
        "```",
        "",
        "### Error Logs (excerpt)",
        "```",
        error_snippet[:3000],
        "```",
        "",
        "### Steps Already Taken",
        "<!-- Describe what you tried -->",
    ]
    body = "\n".join(body_lines)
    params = urllib.parse.urlencode({"title": title, "body": body})
    return {
        "should_open": True,
        "title": title,
        "body": body,
        "prefill_url": f"https://github.com/platform9/vjailbreak/issues/new?{params}",
        "collect_first": GITHUB_COLLECT_FIRST,
    }


def parse_claude_response(
    raw: str, migration_name: str, conditions: list, error_snippet: str
) -> dict:
    """Parse Claude's JSON response; fall back to confidence=none on failure."""
    # Use a more permissive regex to handle JSON embedded in prose text
    json_match = re.search(r"\{.*\}", raw, re.DOTALL)
    if json_match:
        try:
            result = json.loads(json_match.group())
            result["raw_response"] = raw
            confidence = result.get("confidence", "none")
            github_issue = build_github_issue(migration_name, conditions, error_snippet)
            github_issue["should_open"] = confidence in ("none", "low")
            result["github_issue"] = github_issue
            return result
        except json.JSONDecodeError:
            pass

    return {
        "root_cause": None,
        "fix_steps": [],
        "summary": raw[:500],
        "confidence": "none",
        "doc_references": [],
        "raw_response": raw,
        "github_issue": build_github_issue(migration_name, conditions, error_snippet),
    }


def query_rag(chroma_client, error_keywords: list[str]) -> str:
    """Retrieve top-5 relevant doc chunks from ChromaDB."""
    if not error_keywords or chroma_client is None:
        return ""
    try:
        collection = chroma_client.get_collection("vjailbreak_docs")
        query_text = " ".join(error_keywords)
        results = collection.query(query_texts=[query_text], n_results=5)
        chunks = results.get("documents", [[]])[0]
        urls = results.get("metadatas", [[]])[0]
        parts = []
        for i, chunk in enumerate(chunks):
            url = urls[i].get("url", "") if i < len(urls) else ""
            parts.append(f"{chunk}\n(source: {url})" if url else chunk)
        return "\n\n---\n\n".join(parts)
    except Exception:
        return ""


def analyze_migration(request_data: dict, chroma_client) -> dict:
    """Core analysis function. Returns structured response dict."""
    context = request_data.get("context", {})
    conversation_history = request_data.get("conversation_history", [])
    question = request_data.get("question")
    migration_name = request_data.get("migration_name", "unknown")

    all_logs = "\n".join([
        context.get("v2v_logs", ""),
        context.get("controller_logs", ""),
        *context.get("debug_logs", {}).values(),
    ])
    error_keywords = extract_error_keywords(all_logs)
    rag_context = query_rag(chroma_client, error_keywords)

    is_followup = bool(question and conversation_history)

    if is_followup:
        system_prompt = FOLLOWUP_SYSTEM_PROMPT
        user_content = question
    else:
        system_prompt = SYSTEM_PROMPT
        user_content = build_user_message(context, rag_context)

    messages = list(conversation_history) + [{"role": "user", "content": user_content}]

    client = anthropic.Anthropic()
    response = client.messages.create(
        model="claude-haiku-4-5-20251001",
        max_tokens=1024,
        system=system_prompt,
        messages=messages,
    )
    raw = response.content[0].text

    if is_followup:
        return {
            "raw_response": raw,
            "is_followup": True,
            "root_cause": None,
            "fix_steps": [],
            "summary": "",
            "confidence": "high",
            "doc_references": [],
            "github_issue": {"should_open": False},
        }

    conditions = (
        context.get("migration_cr", {}).get("status", {}).get("conditions", [])
    )
    return parse_claude_response(raw, migration_name, conditions, all_logs[:3000])
