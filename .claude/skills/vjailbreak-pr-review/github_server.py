#!/usr/bin/env python3
"""vjailbreak-pr-review eval viewer with GitHub posting support.

Wraps the skill-creator eval viewer and adds:
  POST /api/github  — runs gh pr review / gh pr comment for selected findings
  viewer HTML       — injects "Post to GitHub" section with checkboxes

Never modify the shared skill-creator files; keep all customization here.
"""

import json
import subprocess
import sys
from functools import partial
from http.server import HTTPServer
from pathlib import Path

# ---------------------------------------------------------------------------
# Locate and import the shared eval viewer
# ---------------------------------------------------------------------------

_BASE = (
    Path.home()
    / ".claude/plugins/marketplaces/claude-plugins-official"
    / "plugins/skill-creator/skills/skill-creator/eval-viewer"
)

if not (_BASE / "generate_review.py").exists():
    sys.exit(f"ERROR: skill-creator eval-viewer not found at {_BASE}")

sys.path.insert(0, str(_BASE))
import generate_review  # noqa: E402 (must come after sys.path insert)


# ---------------------------------------------------------------------------
# GitHub CSS / HTML / JS injected into every served page
# ---------------------------------------------------------------------------

_GITHUB_CSS = """
    /* ---- Post to GitHub ---- */
    .github-poster { display: flex; flex-direction: column; gap: 0.625rem; }
    .github-poster .gh-meta { font-size: 0.8125rem; color: var(--text-muted); margin-bottom: 0.25rem; }
    .github-item-label {
      display: flex; align-items: flex-start; gap: 0.5rem; cursor: pointer;
      font-size: 0.875rem; line-height: 1.4; padding: 0.5rem 0.625rem;
      border: 1px solid var(--border); border-radius: var(--radius);
      background: var(--bg); transition: background 0.1s;
    }
    .github-item-label:hover { background: #f0ede4; }
    .github-item-label input[type="checkbox"] {
      margin-top: 0.15rem; flex-shrink: 0; accent-color: var(--accent); width: 1rem; height: 1rem;
    }
    .github-item-label .gh-title { font-weight: 500; }
    .github-item-label .gh-tag {
      font-size: 0.6875rem; font-family: 'Poppins', sans-serif; font-weight: 600;
      text-transform: uppercase; letter-spacing: 0.04em; color: var(--text-muted); margin-left: 0.25rem;
    }
    .github-actions { display: flex; align-items: center; gap: 1rem; margin-top: 0.375rem; }
    .github-post-btn {
      font-family: 'Poppins', sans-serif; padding: 0.5rem 1.25rem;
      background: var(--accent); color: white; border: none; border-radius: var(--radius);
      cursor: pointer; font-size: 0.875rem; font-weight: 600; transition: background 0.15s;
    }
    .github-post-btn:hover:not(:disabled) { background: var(--accent-hover); }
    .github-post-btn:disabled { opacity: 0.5; cursor: not-allowed; }
    .github-post-status { font-size: 0.8125rem; color: var(--text-muted); }
    .gh-results-list { list-style: none; display: flex; flex-direction: column; gap: 0.375rem; margin-top: 0.375rem; }
    .gh-results-list li { font-size: 0.8125rem; padding: 0.375rem 0.625rem; border-radius: 4px; }
    .gh-results-list .gh-ok { background: var(--green-bg); color: var(--green); }
    .gh-results-list .gh-err { background: var(--red-bg); color: var(--red); }
    .gh-results-list .gh-link { color: var(--accent); text-decoration: underline; margin-left: 0.5rem; font-size: 0.75rem; }"""

_GITHUB_HTML_SECTION = """      <!-- Post to GitHub (shown only when findings.json is present) -->
      <div class="section" id="github-section" style="display:none;">
        <div class="section-header">Post to GitHub</div>
        <div class="section-body" id="github-body"></div>
      </div>

      """

_GITHUB_JS = """
    // ---- Post to GitHub ----
    let currentFindingsData = null;

    function renderGitHub(run) {
      const section = document.getElementById("github-section");
      const body = document.getElementById("github-body");
      currentFindingsData = null;
      const findingsFile = (run.outputs || []).find(f => f.name === "findings.json" && f.type === "text");
      if (!findingsFile) { section.style.display = "none"; return; }
      let fd;
      try { fd = JSON.parse(findingsFile.content); } catch (e) { section.style.display = "none"; return; }
      currentFindingsData = fd;
      section.style.display = "block";
      const verdictLabel = (fd.verdict_flag || "--comment").replace(/^--/, "").replace(/-/g, " ");
      let html = '<div class="github-poster">';
      html += `<div class="gh-meta">PR #${escapeHtml(String(fd.pr || "?"))} &middot; ${escapeHtml(fd.repo || "")}</div>`;
      html += `<label class="github-item-label"><input type="checkbox" id="gh-summary" checked><span><span class="gh-title">Summary / Verdict</span><span class="gh-tag">${escapeHtml(verdictLabel)}</span></span></label>`;
      for (let i = 0; i < (fd.findings || []).length; i++) {
        const f = fd.findings[i];
        html += `<label class="github-item-label"><input type="checkbox" id="gh-finding-${i}" checked><span class="gh-title">${escapeHtml(f.title || "Finding " + (i + 1))}</span></label>`;
      }
      html += `<div class="github-actions"><button class="github-post-btn" id="gh-post-btn" onclick="postToGitHub()">Post Selected to GitHub</button><span class="github-post-status" id="gh-status"></span></div>`;
      html += '<ul class="gh-results-list" id="gh-results"></ul></div>';
      body.innerHTML = html;
    }

    async function postToGitHub() {
      if (!currentFindingsData) return;
      const selected = [];
      if (document.getElementById("gh-summary")?.checked) selected.push("summary");
      for (let i = 0; i < (currentFindingsData.findings || []).length; i++) {
        if (document.getElementById(`gh-finding-${i}`)?.checked) selected.push(`finding_${i}`);
      }
      if (selected.length === 0) { document.getElementById("gh-status").textContent = "Nothing selected."; return; }
      const btn = document.getElementById("gh-post-btn");
      const status = document.getElementById("gh-status");
      btn.disabled = true; status.textContent = "Posting\u2026";
      try {
        const resp = await fetch("/api/github", {
          method: "POST", headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ findings_data: currentFindingsData, selected }),
        });
        const data = await resp.json();
        if (data.error) { status.textContent = "Error: " + data.error; btn.disabled = false; return; }
        const resultsList = document.getElementById("gh-results");
        resultsList.innerHTML = "";
        let allOk = true;
        for (const r of (data.results || [])) {
          const li = document.createElement("li");
          li.className = r.ok ? "gh-ok" : "gh-err";
          li.textContent = (r.ok ? "\\u2713 " : "\\u2717 ") + r.id + ": " + (r.ok ? "posted" : r.output);
          if (r.ok && r.url) {
            const a = document.createElement("a");
            a.href = r.url; a.target = "_blank"; a.className = "gh-link"; a.textContent = "View";
            li.appendChild(a);
          }
          resultsList.appendChild(li);
          if (!r.ok) allOk = false;
        }
        status.textContent = allOk ? "All posted!" : "Some failed \u2014 see below.";
        btn.disabled = false;
      } catch (e) { status.textContent = "Network error: " + e.message; btn.disabled = false; }
    }

    """


def _inject_github(html: str) -> str:
    """Inject GitHub CSS, HTML section, and JS into the base viewer HTML."""
    # CSS: before </style>
    html = html.replace("  </style>", _GITHUB_CSS + "\n  </style>", 1)

    # HTML section: before <!-- Feedback -->
    feedback_anchor = "<!-- Feedback -->"
    if feedback_anchor in html:
        html = html.replace(feedback_anchor, _GITHUB_HTML_SECTION + feedback_anchor, 1)

    # renderGitHub call: after renderOutputs(run);
    html = html.replace(
        "renderOutputs(run);",
        "renderOutputs(run);\n\n      // GitHub posting\n      renderGitHub(run);",
        1,
    )

    # JS functions: before // ---- Feedback
    feedback_js = "// ---- Feedback (saved to server -> feedback.json) ----"
    if feedback_js in html:
        html = html.replace(feedback_js, _GITHUB_JS + feedback_js, 1)

    return html


# ---------------------------------------------------------------------------
# GitHub posting logic
# ---------------------------------------------------------------------------

def _post_to_github(data: dict) -> list[dict]:
    findings_data = data.get("findings_data", {})
    selected = set(data.get("selected", []))
    pr = str(findings_data.get("pr", ""))
    repo = findings_data.get("repo", "")
    verdict_flag = findings_data.get("verdict_flag", "--comment")
    summary_body = findings_data.get("summary_body", "")
    findings = findings_data.get("findings", [])

    if not pr or not repo:
        raise ValueError("findings.json must contain 'pr' and 'repo' fields")

    results = []

    if "summary" in selected:
        cmd = ["gh", "pr", "review", pr, "--repo", repo, verdict_flag, "--body", summary_body]
        r = subprocess.run(cmd, capture_output=True, text=True)
        results.append({
            "id": "summary",
            "ok": r.returncode == 0,
            "url": r.stdout.strip(),
            "output": (r.stdout + r.stderr).strip(),
        })

    for i, finding in enumerate(findings):
        key = f"finding_{i}"
        if key not in selected:
            continue
        cmd = ["gh", "pr", "comment", pr, "--repo", repo, "--body", finding.get("body", "")]
        r = subprocess.run(cmd, capture_output=True, text=True)
        results.append({
            "id": key,
            "ok": r.returncode == 0,
            "url": r.stdout.strip(),
            "output": (r.stdout + r.stderr).strip(),
        })

    return results


# ---------------------------------------------------------------------------
# Extended handler
# ---------------------------------------------------------------------------

class GitHubReviewHandler(generate_review.ReviewHandler):
    """ReviewHandler extended with /api/github endpoint."""

    def do_GET(self) -> None:
        if self.path in ("/", "/index.html"):
            # Generate base HTML then inject GitHub additions
            runs = generate_review.find_runs(self.workspace)
            benchmark = None
            if self.benchmark_path and self.benchmark_path.exists():
                try:
                    benchmark = json.loads(self.benchmark_path.read_text())
                except (json.JSONDecodeError, OSError):
                    pass
            html = generate_review.generate_html(runs, self.skill_name, self.previous, benchmark)
            html = _inject_github(html)
            content = html.encode("utf-8")
            self.send_response(200)
            self.send_header("Content-Type", "text/html; charset=utf-8")
            self.send_header("Content-Length", str(len(content)))
            self.end_headers()
            self.wfile.write(content)
        else:
            super().do_GET()

    def do_POST(self) -> None:
        if self.path == "/api/github":
            length = int(self.headers.get("Content-Length", 0))
            body = self.rfile.read(length)
            try:
                payload = json.loads(body)
                results = _post_to_github(payload)
                resp = json.dumps({"ok": True, "results": results}).encode()
                self.send_response(200)
            except (json.JSONDecodeError, OSError, ValueError) as e:
                resp = json.dumps({"error": str(e)}).encode()
                self.send_response(500)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(resp)))
            self.end_headers()
            self.wfile.write(resp)
        else:
            super().do_POST()


# ---------------------------------------------------------------------------
# Entry point (mirrors generate_review.main)
# ---------------------------------------------------------------------------

def main() -> None:
    import argparse
    import time
    import webbrowser

    parser = argparse.ArgumentParser(description="vjailbreak-pr-review eval viewer with GitHub posting")
    parser.add_argument("workspace", type=Path)
    parser.add_argument("--port", "-p", type=int, default=3117)
    parser.add_argument("--skill-name", "-n", type=str, default="vjailbreak-pr-review")
    parser.add_argument("--previous-workspace", type=Path, default=None)
    parser.add_argument("--benchmark", type=Path, default=None)
    args = parser.parse_args()

    workspace = args.workspace.resolve()
    if not workspace.is_dir():
        sys.exit(f"Error: {workspace} is not a directory")

    runs = generate_review.find_runs(workspace)
    if not runs:
        sys.exit(f"No runs found in {workspace}")

    feedback_path = workspace / "feedback.json"
    previous: dict = {}
    if args.previous_workspace:
        previous = generate_review.load_previous_iteration(args.previous_workspace.resolve())
    benchmark_path = args.benchmark.resolve() if args.benchmark else None

    generate_review._kill_port(args.port)

    handler = partial(
        GitHubReviewHandler,
        workspace,
        args.skill_name,
        feedback_path,
        previous,
        benchmark_path,
    )
    try:
        server = HTTPServer(("127.0.0.1", args.port), handler)
    except OSError:
        server = HTTPServer(("127.0.0.1", 0), handler)

    port = server.server_address[1]
    url = f"http://localhost:{port}"
    print(f"\n  vjailbreak-pr-review Eval Viewer (with GitHub posting)")
    print(f"  URL: {url}\n  Workspace: {workspace}\n")
    webbrowser.open(url)

    try:
        server.serve_forever()
    except KeyboardInterrupt:
        print("\nStopped.")
        server.server_close()


if __name__ == "__main__":
    main()
