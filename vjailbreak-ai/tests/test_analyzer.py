# vjailbreak-ai/tests/test_analyzer.py
import pytest
from unittest.mock import MagicMock, patch
import sys, os
sys.path.insert(0, os.path.join(os.path.dirname(__file__), '..'))

from analyzer import (
    extract_error_keywords,
    build_user_message,
    build_github_issue,
    parse_claude_response,
)


def test_extract_error_keywords_finds_errors():
    logs = "INFO starting\nERROR disk copy failed: connection refused\nINFO retrying\nFAILED VDDK connect\n"
    result = extract_error_keywords(logs)
    assert any("disk copy failed" in kw for kw in result)
    assert any("VDDK connect" in kw for kw in result)


def test_extract_error_keywords_empty():
    assert extract_error_keywords("") == []


def test_extract_error_keywords_caps_at_ten():
    lines = [f"ERROR error{i}" for i in range(20)]
    result = extract_error_keywords("\n".join(lines))
    assert len(result) <= 10


def test_build_github_issue_structure():
    result = build_github_issue(
        migration_name="migration-my-vm-abc12",
        conditions=[{"type": "Failed", "message": "disk copy failed at 67%"}],
        error_snippet="ERROR: disk copy failed",
    )
    assert result["should_open"] is True
    assert "migration-my-vm-abc12" in result["title"]
    assert "disk copy failed at 67%" in result["body"]
    assert "github.com/platform9/vjailbreak/issues/new" in result["prefill_url"]
    assert len(result["collect_first"]) >= 3


def test_build_user_message_includes_all_sections():
    context = {
        "migration_cr": {"status": {"phase": "Failed", "conditions": []}},
        "migration_plan": {"spec": {}},
        "migration_template": {"spec": {"networkMapping": "net1"}},
        "v2v_logs": "ERROR: disk copy failed",
        "controller_logs": "WARN: timeout",
        "debug_logs": {"migration.log": "ERROR: DNS failed"},
    }
    msg = build_user_message(context, rag_context="See DNS troubleshooting guide")
    assert "ERROR: disk copy failed" in msg
    assert "WARN: timeout" in msg
    assert "ERROR: DNS failed" in msg
    assert "DNS troubleshooting guide" in msg


def test_build_user_message_includes_additional_context():
    context = {
        "migration_cr": {"status": {"phase": "Failed", "conditions": []}},
        "v2v_logs": "ERROR: disk failed",
        "controller_logs": "",
        "debug_logs": {},
        "additional_context": "Our VDDK is installed at /opt/vmware/vddk. ESXi hosts use self-signed certs.",
    }
    msg = build_user_message(context, rag_context="")
    assert "Our VDDK is installed at /opt/vmware/vddk" in msg
    assert "ESXi hosts use self-signed certs" in msg


def test_build_user_message_no_additional_context():
    context = {
        "migration_cr": {"status": {"phase": "Failed", "conditions": []}},
        "v2v_logs": "",
        "controller_logs": "",
        "debug_logs": {},
    }
    msg = build_user_message(context, rag_context="")
    assert "Operator-Provided Context" not in msg


def test_build_user_message_includes_fetch_warnings():
    context = {
        "migration_cr": {"status": {"phase": "Failed", "conditions": []}},
        "v2v_logs": "",
        "controller_logs": "",
        "debug_logs": {},
        "fetch_warnings": ["v2v pod logs unavailable: context deadline exceeded", "debug logs unavailable: connection refused"],
    }
    msg = build_user_message(context, rag_context="")
    assert "Fetch Warnings" in msg
    assert "v2v pod logs unavailable" in msg
    assert "debug logs unavailable" in msg


def test_build_user_message_no_fetch_warnings_when_empty():
    context = {
        "migration_cr": {"status": {"phase": "Failed", "conditions": []}},
        "v2v_logs": "ERROR: something",
        "controller_logs": "",
        "debug_logs": {},
        "fetch_warnings": [],
    }
    msg = build_user_message(context, rag_context="")
    assert "Fetch Warnings" not in msg


def test_parse_claude_response_valid_json():
    raw = '{"root_cause": "DNS failure", "fix_steps": ["add to /etc/hosts"], "summary": "DNS issue", "confidence": "high", "doc_references": []}'
    result = parse_claude_response(raw, "migration-test", [], "")
    assert result["root_cause"] == "DNS failure"
    assert result["confidence"] == "high"
    assert result["github_issue"]["should_open"] is False


def test_parse_claude_response_json_in_prose():
    raw = 'Based on the logs: {"root_cause": "VDDK missing", "fix_steps": ["install VDDK"], "summary": "missing libs", "confidence": "high", "doc_references": []} Done.'
    result = parse_claude_response(raw, "migration-test", [], "")
    assert result["root_cause"] == "VDDK missing"


def test_parse_claude_response_unparseable_gives_none_confidence():
    raw = "I cannot determine the root cause from the provided logs."
    result = parse_claude_response(raw, "migration-test", [], "some errors")
    assert result["confidence"] == "none"
    assert result["github_issue"]["should_open"] is True


def test_parse_claude_response_low_confidence_includes_github_issue():
    raw = '{"root_cause": "unclear", "fix_steps": [], "summary": "unclear", "confidence": "low", "doc_references": []}'
    result = parse_claude_response(raw, "migration-test", [{"type": "Failed"}], "errors")
    assert result["github_issue"]["should_open"] is True


def test_build_github_issue_url_encoding():
    import urllib.parse
    result = build_github_issue(
        migration_name="migration-test-vm",
        conditions=[],
        error_snippet="some error",
    )
    assert result["prefill_url"].startswith("https://github.com/platform9/vjailbreak/issues/new")
    parsed = urllib.parse.urlparse(result["prefill_url"])
    params = urllib.parse.parse_qs(parsed.query)
    assert "title" in params
    assert "body" in params
    assert len(result["collect_first"]) >= 5
