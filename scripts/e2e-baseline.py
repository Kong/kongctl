#!/usr/bin/env python3
"""Report latency and reset metrics for recent successful full .com E2E runs."""

from __future__ import annotations

import argparse
import json
import math
import subprocess
import tempfile
from collections import defaultdict
from datetime import datetime
from pathlib import Path
from typing import Any, Iterable


BUILD_JOB = "Build scenario executables"
HARNESS_JOB = "Test scenario harness"
VERIFY_JOB = "Verify scenario results and coverage"
REQUIRED_JOB = "Publish “E2E Required”"
SCENARIO_JOB_PREFIX = "Run scenarios — "


def gh_json(arguments: list[str]) -> Any:
    process = subprocess.run(
        ["gh", *arguments],
        check=True,
        stdout=subprocess.PIPE,
        text=True,
    )
    return json.loads(process.stdout)


def timestamp(value: str) -> datetime:
    return datetime.fromisoformat(value.replace("Z", "+00:00"))


def elapsed_seconds(start: str, end: str) -> float:
    return (timestamp(end) - timestamp(start)).total_seconds()


def duration(record: dict[str, Any]) -> float:
    return elapsed_seconds(record["startedAt"], record["completedAt"])


def nearest_rank(values: Iterable[float], percentile: float) -> float:
    ordered = sorted(values)
    if not ordered:
        return 0.0
    index = max(0, math.ceil(percentile * len(ordered)) - 1)
    return ordered[index]


def percentiles(values: Iterable[float]) -> dict[str, float]:
    materialized = list(values)
    return {
        "p50": nearest_rank(materialized, 0.50),
        "p75": nearest_rank(materialized, 0.75),
        "p90": nearest_rank(materialized, 0.90),
    }


def select_complete_attempt(metrics: list[dict[str, Any]], run_attempt: int) -> list[dict[str, Any]]:
    selected = [metric for metric in metrics if int(metric["run_attempt"]) == run_attempt]
    totals = {int(metric["shard_total"]) for metric in selected}
    indices = {int(metric["shard_index"]) for metric in selected}
    if len(totals) != 1:
        return []
    total = totals.pop()
    if total <= 0 or indices != set(range(total)):
        return []
    return sorted(selected, key=lambda metric: int(metric["shard_index"]))


def download_metrics(repo: str, run_id: int, run_attempt: int) -> list[dict[str, Any]]:
    artifact_prefix = f"e2e-metrics-{run_id}-{run_attempt}-"
    response = gh_json(
        [
            "api",
            "--method",
            "GET",
            f"repos/{repo}/actions/runs/{run_id}/artifacts",
            "-f",
            "per_page=100",
        ]
    )
    artifacts = response.get("artifacts", [])
    if not any(
        artifact["name"].startswith(artifact_prefix) and not artifact.get("expired", False)
        for artifact in artifacts
    ):
        return []

    with tempfile.TemporaryDirectory(prefix="kongctl-e2e-metrics-") as temp_dir:
        subprocess.run(
            [
                "gh",
                "run",
                "download",
                str(run_id),
                "--repo",
                repo,
                "--pattern",
                f"{artifact_prefix}*",
                "--dir",
                temp_dir,
            ],
            check=True,
            stdout=subprocess.DEVNULL,
        )
        records = [
            json.loads(path.read_text(encoding="utf-8"))
            for path in Path(temp_dir).rglob("e2e-metrics.json")
        ]
        return select_complete_attempt(records, run_attempt)


def named_job(jobs: list[dict[str, Any]], name: str) -> dict[str, Any] | None:
    return next((job for job in jobs if job["name"] == name), None)


def named_step(job: dict[str, Any], name: str) -> dict[str, Any] | None:
    return next((step for step in job.get("steps", []) if step["name"] == name), None)


def eligible_run(
    run: dict[str, Any],
    jobs: list[dict[str, Any]],
    metrics: list[dict[str, Any]],
) -> dict[str, Any] | None:
    if not metrics or any(metric.get("konnect_environment") != "com" for metric in metrics):
        return None

    build = named_job(jobs, BUILD_JOB)
    harness = named_job(jobs, HARNESS_JOB)
    verify = named_job(jobs, VERIFY_JOB)
    required = named_job(jobs, REQUIRED_JOB)
    scenario_jobs = [job for job in jobs if job["name"].startswith(SCENARIO_JOB_PREFIX)]
    required_jobs = [build, harness, verify, required, *scenario_jobs]
    if any(job is None or job["conclusion"] != "success" for job in required_jobs):
        return None
    if len(scenario_jobs) != len(metrics):
        return None

    assert build is not None and harness is not None and verify is not None and required is not None
    build_kongctl = named_step(build, "Build kongctl")
    build_scenarios = named_step(build, "Build scenario test binary")
    if build_kongctl is None or build_scenarios is None:
        return None

    ready_at = max(timestamp(build["completedAt"]), timestamp(harness["completedAt"]))
    workflow_created_at = timestamp(run["createdAt"])
    first_job_started_at = min(timestamp(job["startedAt"]) for job in jobs if job["startedAt"])
    shard_durations = [float(metric["execution_duration_seconds"]) for metric in metrics]

    reset = defaultdict(int)
    scenarios: list[dict[str, Any]] = []
    for metric in metrics:
        for key, value in metric["reset"].items():
            reset[key] += int(value)
        scenarios.extend(metric["scenario_durations"])

    return {
        "run_id": int(run["databaseId"]),
        "url": run["url"],
        "created_at": run["createdAt"],
        "workflow_admission_delay_seconds": (first_job_started_at - workflow_created_at).total_seconds(),
        "queue_to_required_status_seconds": (timestamp(required["completedAt"]) - workflow_created_at).total_seconds(),
        "build_job_seconds": duration(build),
        "build_kongctl_seconds": duration(build_kongctl),
        "build_scenario_binary_seconds": duration(build_scenarios),
        "longest_shard_seconds": max(shard_durations),
        "shard_spread_seconds": max(shard_durations) - min(shard_durations),
        "shard_admission_delay_seconds": {
            job["name"].removeprefix(SCENARIO_JOB_PREFIX): max(
                0.0,
                (timestamp(job["startedAt"]) - ready_at).total_seconds(),
            )
            for job in scenario_jobs
        },
        "shards": [
            {
                "org_name": metric["org_name"],
                "selected_scenario_count": metric["selected_scenario_count"],
                "execution_duration_seconds": metric["execution_duration_seconds"],
            }
            for metric in metrics
        ],
        "scenario_durations": scenarios,
        "reset": dict(reset),
    }


def collect_runs(repo: str, count: int, scan: int) -> list[dict[str, Any]]:
    candidates = gh_json(
        [
            "run",
            "list",
            "--repo",
            repo,
            "--workflow",
            "e2e.yaml",
            "--status",
            "success",
            "--limit",
            str(scan),
            "--json",
            "databaseId,createdAt,url",
        ]
    )
    selected: list[dict[str, Any]] = []
    for candidate in candidates:
        view = gh_json(
            [
                "run",
                "view",
                str(candidate["databaseId"]),
                "--repo",
                repo,
                "--json",
                "attempt,jobs",
            ]
        )
        run_attempt = int(view["attempt"])
        metrics = download_metrics(repo, int(candidate["databaseId"]), run_attempt)
        if not metrics:
            continue
        record = eligible_run(candidate, view["jobs"], metrics)
        if record is not None:
            selected.append(record)
        if len(selected) == count:
            break
    return selected


def summarize(runs: list[dict[str, Any]]) -> dict[str, Any]:
    metric_names = [
        "workflow_admission_delay_seconds",
        "queue_to_required_status_seconds",
        "build_job_seconds",
        "build_kongctl_seconds",
        "build_scenario_binary_seconds",
        "longest_shard_seconds",
        "shard_spread_seconds",
    ]
    reset_names = [
        "count",
        "duration_ms",
        "list_calls",
        "list_duration_ms",
        "resources_found",
        "delete_calls",
        "delete_duration_ms",
        "resources_deleted",
    ]
    return {
        "run_count": len(runs),
        "metrics": {name: percentiles(run[name] for run in runs) for name in metric_names},
        "reset_per_run": {
            name: percentiles(float(run["reset"].get(name, 0)) for run in runs) for name in reset_names
        },
    }


def markdown_report(repo: str, runs: list[dict[str, Any]], summary: dict[str, Any]) -> str:
    lines = [
        "# Konnect `.com` E2E baseline",
        "",
        f"Repository: `{repo}`  ",
        f"Full successful runs: {len(runs)}",
        "",
        "The report scans successful `e2e.yaml` runs newest-first, retains only runs with a complete",
        "latest-attempt metrics manifest for every `.com` shard, and requires successful build, harness,",
        "scenario, coverage-verification, and required-status jobs. Short gate-only runs are excluded.",
        "Percentiles use the nearest-rank method.",
        "",
        "## Latency",
        "",
        "| Metric | p50 | p75 | p90 |",
        "| --- | ---: | ---: | ---: |",
    ]
    for name, values in summary["metrics"].items():
        lines.append(f"| {name} | {values['p50']:.1f}s | {values['p75']:.1f}s | {values['p90']:.1f}s |")

    lines.extend(
        [
            "",
            "## Reset cost per workflow run",
            "",
            "| Metric | p50 | p75 | p90 |",
            "| --- | ---: | ---: | ---: |",
        ]
    )
    for name, values in summary["reset_per_run"].items():
        suffix = "ms" if name.endswith("_ms") else ""
        lines.append(
            f"| {name} | {values['p50']:.1f}{suffix} | {values['p75']:.1f}{suffix} | "
            f"{values['p90']:.1f}{suffix} |"
        )

    lines.extend(
        [
            "",
            "## Included runs",
            "",
            "| Run | Created | Queue-to-status | Build | Longest shard | Spread | Resets |",
            "| --- | --- | ---: | ---: | ---: | ---: | ---: |",
        ]
    )
    for run in runs:
        lines.append(
            f"| [{run['run_id']}]({run['url']}) | {run['created_at']} | "
            f"{run['queue_to_required_status_seconds']:.0f}s | {run['build_job_seconds']:.0f}s | "
            f"{run['longest_shard_seconds']:.0f}s | {run['shard_spread_seconds']:.0f}s | "
            f"{run['reset'].get('count', 0)} |"
        )

    lines.extend(
        [
            "",
            "## Organization shards",
            "",
            "| Run | Organization | Admission delay | Selected | Execution |",
            "| --- | --- | ---: | ---: | ---: |",
        ]
    )
    for run in runs:
        for shard in run["shards"]:
            org_name = shard["org_name"]
            admission_delay = run["shard_admission_delay_seconds"].get(org_name, 0)
            lines.append(
                f"| {run['run_id']} | `{org_name}` | {admission_delay:.0f}s | "
                f"{shard['selected_scenario_count']} | {shard['execution_duration_seconds']}s |"
            )

    scenario_samples: dict[str, list[float]] = defaultdict(list)
    for run in runs:
        for scenario in run["scenario_durations"]:
            scenario_samples[scenario["scenario"]].append(float(scenario["duration_seconds"]))
    lines.extend(
        [
            "",
            "## Individual scenario durations",
            "",
            "| Scenario | Samples | Median | p90 |",
            "| --- | ---: | ---: | ---: |",
        ]
    )
    scenario_rows = sorted(
        scenario_samples.items(),
        key=lambda item: (-nearest_rank(item[1], 0.50), item[0]),
    )
    for scenario, samples in scenario_rows:
        lines.append(
            f"| `{scenario}` | {len(samples)} | {nearest_rank(samples, 0.50):.2f}s | "
            f"{nearest_rank(samples, 0.90):.2f}s |"
        )
    return "\n".join(lines) + "\n"


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--repo", default="kong/kongctl")
    parser.add_argument("--count", type=int, default=20)
    parser.add_argument("--scan", type=int, default=100)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--json-output", type=Path)
    args = parser.parse_args()
    if args.count < 1 or args.scan < args.count:
        parser.error("--count must be positive and --scan must be at least --count")

    runs = collect_runs(args.repo, args.count, args.scan)
    if len(runs) < args.count:
        raise SystemExit(
            f"found {len(runs)} eligible full .com runs, need {args.count}; "
            "increase --scan or wait for more instrumented runs"
        )
    summary = summarize(runs)
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(markdown_report(args.repo, runs, summary), encoding="utf-8")
    if args.json_output is not None:
        args.json_output.parent.mkdir(parents=True, exist_ok=True)
        args.json_output.write_text(
            json.dumps({"summary": summary, "runs": runs}, indent=2, sort_keys=True) + "\n",
            encoding="utf-8",
        )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
