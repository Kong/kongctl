#!/usr/bin/env python3
"""Measure PR queue overlap with main validation using read-only GitHub APIs."""

from __future__ import annotations

import argparse
import base64
import json
import subprocess
from datetime import datetime, timezone
from typing import Any


def api(path: str) -> list[dict[str, Any]]:
    result = subprocess.run(
        ["gh", "api", "--paginate", "--slurp", path],
        check=True, capture_output=True, text=True,
    )
    return json.loads(result.stdout)


def timestamp(value: str) -> float:
    return datetime.fromisoformat(value.replace("Z", "+00:00")).timestamp()


def measure(runs: list[dict[str, Any]], jobs: dict[int, list[dict[str, Any]]]) -> dict[str, Any]:
    main_intervals = []
    pr_intervals = []
    for run in runs:
        # First attempts only: rerun creation timestamps do not describe their queue.
        if run.get("run_attempt", 1) != 1:
            continue
        admission = next((job for job in jobs[run["id"]]
                          if job["name"] == "Determine scenario requirement" and job.get("started_at")), None)
        if not admission:
            continue
        title = run.get("display_title", "")
        start = timestamp(admission["started_at"])
        if title.startswith("main · scenarios ·") and run["status"] == "completed":
            main_intervals.append((start, timestamp(run["updated_at"]), run["id"]))
        elif title.startswith(("readiness:", "PR #")) or run.get("event") == "pull_request":
            pr_intervals.append((timestamp(run["created_at"]), start, run["id"]))
    overlaps = []
    for queued, admitted, pr in pr_intervals:
        for start, end, main in main_intervals:
            seconds = max(0, min(admitted, end) - max(queued, start))
            if seconds:
                overlaps.append({"pr_run": pr, "main_run": main, "overlap_seconds": seconds})
    return {"main_runs": len(main_intervals), "pr_runs": len(pr_intervals),
            "overlapping_pr_runs": len({row["pr_run"] for row in overlaps}),
            "queue_overlap_seconds": sum(row["overlap_seconds"] for row in overlaps),
            "overlaps": overlaps}


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--repo", default="Kong/kongctl")
    parser.add_argument("--since", required=True, help="Inclusive UTC date, e.g. 2026-11-20")
    args = parser.parse_args()
    pages = api(f"repos/{args.repo}/actions/workflows/e2e.yaml/runs?per_page=100&created=%3E%3D{args.since}")
    if any(page["total_count"] > 1000 for page in pages):
        raise SystemExit("GitHub caps run search at 1000 results; use a shorter observation window.")
    runs = [run for page in pages for run in page["workflow_runs"]]
    jobs = {run["id"]: [job for page in api(
        f"repos/{args.repo}/actions/runs/{run['id']}/attempts/1/jobs?per_page=100") for job in page["jobs"]]
        for run in runs if run.get("run_attempt", 1) == 1}
    report = measure(runs, jobs)
    file = api(f"repos/{args.repo}/contents/state.json?ref=automation/e2e-main-state")[0]
    state = json.loads(base64.b64decode(file["content"]))
    success = state.get("success")
    report["last_success"] = success
    report["validation_age_hours"] = (
        (datetime.now(timezone.utc).timestamp() - timestamp(success["at"])) / 3600 if success else None
    )
    report["since"] = args.since
    print(json.dumps(report, indent=2))


if __name__ == "__main__":
    main()
