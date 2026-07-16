# vjailbreak-ai/tests/test_server.py
import pytest
from unittest.mock import patch, MagicMock
import sys, os
sys.path.insert(0, os.path.join(os.path.dirname(__file__), '..'))

with patch.dict(os.environ, {"ANTHROPIC_API_KEY": "test", "ADMIN_API_KEY": "test"}):
    import server as _server_module
    from server import app

from fastapi.testclient import TestClient
client = TestClient(app)


def test_health():
    mock_collection = MagicMock()
    mock_collection.count.return_value = 0
    with patch.object(_server_module, "collection", mock_collection):
        r = client.get("/health")
    assert r.status_code == 200


def test_analyze_migration_returns_structure():
    mock_result = {
        "root_cause": "DNS failure",
        "fix_steps": ["add to /etc/hosts"],
        "summary": "DNS issue during disk copy",
        "confidence": "high",
        "doc_references": [],
        "raw_response": "...",
        "github_issue": {"should_open": False},
    }
    with patch("server.analyze_migration", return_value=mock_result):
        r = client.post("/analyze-migration", json={
            "migration_name": "migration-my-vm",
            "namespace": "migration-system",
            "context": {
                "migration_cr": {"status": {"phase": "Failed", "conditions": []}},
                "v2v_logs": "ERROR: DNS failed",
                "controller_logs": "",
                "debug_logs": {},
            },
            "conversation_history": [],
            "question": None,
        })
    assert r.status_code == 200
    body = r.json()
    assert body["root_cause"] == "DNS failure"
    assert body["confidence"] == "high"
    assert "github_issue" in body


def test_analyze_migration_missing_body_returns_422():
    r = client.post("/analyze-migration", json={})
    assert r.status_code == 422
