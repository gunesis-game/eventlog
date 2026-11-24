#!/usr/bin/env python3
"""Apply repository-wide Actions run cleanup and per-workflow success/failure retention."""
from __future__ import annotations

import json
import os
import re
import sys
from urllib.error import HTTPError
from urllib.parse import quote
from urllib.request import Request, urlopen

API = "https://api.github.com"
API_VERSION = "2022-11-28"
KEEP = 4
REPOSITORY_WIDE_DELETE_CONCLUSIONS = {"cancelled", "skipped"}
RETAINED_CONCLUSIONS = {"success", "failure"}


def request(url: str, token: str, method: str = "GET"):
    req = Request(
        url,
        method=method,
        headers={
            "Accept": "application/vnd.github+json",
            "Authorization": f"Bearer {token}",
            "X-GitHub-Api-Version": API_VERSION,
        },
    )
    try:
        with urlopen(req, timeout=30) as response:
            body = response.read()
            return json.loads(body.decode("utf-8")) if body else None, response.headers.get("Link", "")
    except HTTPError as error:
        if method == "DELETE" and error.code == 404:
            return None, ""
        detail = error.read().decode("utf-8", errors="replace")
        raise RuntimeError(f"GitHub API {method} {url} failed: HTTP {error.code}: {detail}") from error


def runs_to_delete(runs: list[dict], keep: int = KEEP) -> list[dict]:
    """Keep four eventual success/failure runs, reserving slots for active runs."""
    completed = [run for run in runs if run.get("status") == "completed" or run.get("conclusion") is not None]
    active_count = sum(1 for run in runs if run.get("status") != "completed" and run.get("conclusion") is None)
    completed_slots = max(0, keep - active_count)
    eligible = [run for run in completed if run.get("conclusion") in RETAINED_CONCLUSIONS]
    eligible.sort(key=lambda run: run.get("created_at", ""), reverse=True)
    retained_ids = {run.get("id") for run in eligible[:completed_slots]}
    return [
        run for run in completed
        if run.get("conclusion") not in RETAINED_CONCLUSIONS or run.get("id") not in retained_ids
    ]


def delete_repository_wide_runs(repository: str, token: str) -> list[int]:
    """Delete every cancelled or skipped run repository-wide, across all workflows."""
    url = f"{API}/repos/{repository}/actions/runs?per_page=100"
    candidates = []
    # Fetch every page before deleting: deleting during offset pagination can shift
    # records between pages and cause older runs to be missed.
    while url:
        page, link_header = request(url, token)
        candidates.extend(
            run for run in page.get("workflow_runs", [])
            if run.get("conclusion") in REPOSITORY_WIDE_DELETE_CONCLUSIONS
        )
        next_match = re.search(r'<([^>]+)>;\s*rel="next"', link_header)
        url = next_match.group(1) if next_match else ""
    deleted = []
    for run in candidates:
        run_id = run.get("id")
        request(f"{API}/repos/{repository}/actions/runs/{run_id}", token, method="DELETE")
        deleted.append(run_id)
    return deleted


def main() -> int:
    repository = os.environ.get("GITHUB_REPOSITORY", "")
    token = os.environ.get("GITHUB_TOKEN", "")
    workflow_ref = os.environ.get("GITHUB_WORKFLOW_REF", "")
    current_run_id = os.environ.get("TARGET_RUN_ID") or os.environ.get("GITHUB_RUN_ID", "")
    target_workflow_id = os.environ.get("TARGET_WORKFLOW_ID", "")
    if not repository or not token or not current_run_id:
        raise RuntimeError("GITHUB_REPOSITORY, GITHUB_TOKEN, and a current/target run ID are required")

    globally_deleted = delete_repository_wide_runs(repository, token)

    if target_workflow_id:
        workflow_file = f"workflow ID {target_workflow_id}"
        workflow_endpoint = quote(target_workflow_id, safe="")
    else:
        if not workflow_ref:
            raise RuntimeError("GITHUB_WORKFLOW_REF is required when TARGET_WORKFLOW_ID is not supplied")
        prefix = repository + "/"
        ref_without_branch = workflow_ref.split("@", 1)[0]
        if not ref_without_branch.startswith(prefix):
            raise RuntimeError(f"Cannot resolve workflow file from GITHUB_WORKFLOW_REF={workflow_ref!r}")
        workflow_path = ref_without_branch[len(prefix):]
        if not workflow_path.startswith(".github/workflows/"):
            raise RuntimeError(f"Unexpected workflow path: {workflow_path!r}")
        workflow_file = workflow_path.rsplit("/", 1)[-1]
        workflow_endpoint = quote(workflow_file, safe="")

    url = f"{API}/repos/{repository}/actions/workflows/{workflow_endpoint}/runs?per_page=100"
    runs = []
    while url:
        page, link_header = request(url, token)
        runs.extend(page.get("workflow_runs", []))
        next_match = re.search(r'<([^>]+)>;\s*rel="next"', link_header)
        url = next_match.group(1) if next_match else ""

    if not any(str(run.get("id")) == current_run_id for run in runs):
        runs.append({"id": int(current_run_id), "created_at": "9999-12-31T23:59:59Z", "status": "in_progress", "conclusion": None})

    deleted = []
    for run in runs_to_delete(runs):
        run_id = run.get("id")
        request(f"{API}/repos/{repository}/actions/runs/{run_id}", token, method="DELETE")
        deleted.append(run_id)

    eligible = sorted(
        (run for run in runs if run.get("conclusion") in RETAINED_CONCLUSIONS),
        key=lambda run: run.get("created_at", ""),
        reverse=True,
    )
    older_success_failure = sum(1 for run in eligible[KEEP:])
    print(
        f"Workflow {workflow_file}: examined {len(runs)} runs; retained the newest "
        f"{min(KEEP, len(eligible))} success/failure runs; deleted {older_success_failure} "
        f"older success/failure and {len(runs_to_delete(runs)) - older_success_failure} "
        f"other completed runs. Repository-wide skipped/cancelled runs deleted: {len(globally_deleted)}."
    )
    if deleted:
        print("Deleted run IDs: " + ", ".join(str(run_id) for run_id in deleted))
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except (RuntimeError, ValueError) as error:
        print(f"ERROR: {error}", file=sys.stderr)
        raise SystemExit(1)
