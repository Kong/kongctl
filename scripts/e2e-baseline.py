#!/usr/bin/env python3
"""Report latency and reset metrics for recent successful full .com E2E runs."""

from __future__ import annotations

import argparse
import json
import math
import re
import subprocess
import sys
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
OBSERVATION_SCHEMA_VERSION = 2
COHORTS = ("uncached", "cache-enabled")
LEGACY_ALLOCATION = "modulo-v1"


def valid_allocation(value: Any) -> bool:
    return isinstance(value, str) and re.fullmatch(r"modulo-v1|weighted-v1:[0-9a-f]{64}", value) is not None


def metric_allocation(metric: dict[str, Any]) -> str | None:
    if metric.get("schema_version", 1) not in (1, 2):
        return None
    # Only pre-activation metrics may omit allocation identity.
    if metric.get("schema_version", 1) == 1 and "allocation_id" not in metric:
        return LEGACY_ALLOCATION
    value = metric.get("allocation_id")
    return value if valid_allocation(value) else None


RUN_REQUIRED_FIELDS = {
    "cohort",
    "original_created_at",
    "head_sha",
    "harness_job_seconds",
    "harness_setup_seconds",
    "harness_test_seconds",
    "build_setup_seconds",
    "run_id",
    "run_attempt",
    "url",
    "created_at",
    "workflow_admission_delay_seconds",
    "queue_to_required_status_seconds",
    "build_job_seconds",
    "build_kongctl_seconds",
    "build_scenario_binary_seconds",
    "longest_shard_seconds",
    "shard_spread_seconds",
    "shard_admission_delay_seconds",
    "shards",
    "scenario_durations",
    "reset",
}


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
    if total <= 0 or len(selected) != total or indices != set(range(total)):
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


def run_cohort(jobs: list[dict[str, Any]]) -> str | None:
    build = named_job(jobs, BUILD_JOB)
    if build is None:
        return None
    return "cache-enabled" if named_step(build, "Report Go cache status") else "uncached"


def eligible_run(
    run: dict[str, Any],
    jobs: list[dict[str, Any]],
    metrics: list[dict[str, Any]],
) -> dict[str, Any] | None:
    if not metrics or any(metric.get("konnect_environment") != "com" for metric in metrics):
        return None
    allocations = {metric_allocation(metric) for metric in metrics}
    if len(allocations) != 1 or None in allocations:
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
    harness_setup = named_step(harness, "Setup Go")
    harness_test = named_step(harness, HARNESS_JOB)
    build_setup = named_step(build, "Setup Go")
    if harness_setup is None or harness_test is None or build_setup is None:
        return None

    ready_at = max(timestamp(build["completedAt"]), timestamp(harness["completedAt"]))
    workflow_created_at = timestamp(run["createdAt"])
    if any(timestamp(job["startedAt"]) < workflow_created_at for job in required_jobs):
        # A partial rerun can reuse jobs from an earlier attempt.
        return None
    first_job_started_at = min(timestamp(job["startedAt"]) for job in jobs if job["startedAt"])
    shard_durations = [float(metric["execution_duration_seconds"]) for metric in metrics]

    reset = defaultdict(int)
    scenarios: list[dict[str, Any]] = []
    for metric in metrics:
        for key, value in metric["reset"].items():
            reset[key] += int(value)
        scenarios.extend(metric["scenario_durations"])

    return {
        "cohort": run_cohort(jobs),
        "allocation_id": allocations.pop(),
        "original_created_at": run["original_created_at"],
        "head_sha": run["head_sha"],
        "harness_job_seconds": duration(harness),
        "harness_setup_seconds": duration(harness_setup),
        "harness_test_seconds": duration(harness_test),
        "build_setup_seconds": duration(build_setup),
        "run_id": int(run["databaseId"]),
        "run_attempt": int(metrics[0]["run_attempt"]),
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


def collect_runs(
    repo: str,
    count: int,
    scan: int,
    excluded_run_ids: set[int] | None = None,
    cohort: str = "cache-enabled",
    allocation_id: str = LEGACY_ALLOCATION,
) -> list[dict[str, Any]]:
    if count <= 0:
        return []
    excluded_run_ids = excluded_run_ids or set()
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
        if int(candidate["databaseId"]) in excluded_run_ids:
            continue
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
        if run_cohort(view["jobs"]) != cohort:
            continue
        metrics = download_metrics(repo, int(candidate["databaseId"]), run_attempt)
        if not metrics or any(metric_allocation(metric) != allocation_id for metric in metrics):
            continue
        attempt = gh_json(
            ["api", f"repos/{repo}/actions/runs/{candidate['databaseId']}/attempts/{run_attempt}"]
        )
        if attempt["conclusion"] != "success":
            continue
        candidate = {
            **candidate,
            "original_created_at": candidate["createdAt"],
            "createdAt": attempt["created_at"],
            "head_sha": attempt["head_sha"],
        }
        record = eligible_run(candidate, view["jobs"], metrics)
        if record is not None and record["allocation_id"] == allocation_id:
            selected.append(record)
        if len(selected) == count:
            break
    return selected


def validate_run(run: Any, index: int) -> dict[str, Any]:
    if not isinstance(run, dict):
        raise ValueError(f"observation run {index} must be an object")
    missing = sorted(RUN_REQUIRED_FIELDS - run.keys())
    if missing:
        raise ValueError(f"observation run {index} is missing fields: {', '.join(missing)}")
    if not isinstance(run["run_id"], int) or run["run_id"] <= 0:
        raise ValueError(f"observation run {index} has an invalid run_id")
    if not isinstance(run["run_attempt"], int) or run["run_attempt"] <= 0:
        raise ValueError(f"observation run {index} has an invalid run_attempt")
    if not isinstance(run["created_at"], str):
        raise ValueError(f"observation run {index} has an invalid created_at")
    timestamp(run["created_at"])
    if not isinstance(run["original_created_at"], str):
        raise ValueError(f"observation run {index} has an invalid original_created_at")
    timestamp(run["original_created_at"])
    if not isinstance(run["head_sha"], str) or not run["head_sha"]:
        raise ValueError(f"observation run {index} has an invalid head_sha")
    for field in ("build_setup_seconds", "harness_job_seconds", "harness_setup_seconds", "harness_test_seconds"):
        value = run[field]
        if isinstance(value, bool) or not isinstance(value, (int, float)) or not math.isfinite(value) or value < 0:
            raise ValueError(f"observation run {index} has an invalid {field}")
    if run["cohort"] not in COHORTS:
        raise ValueError(f"observation run {index} has an invalid cohort")
    return run


def load_observations(
    path: Path, repo: str, cohort: str = "cache-enabled", allocation_id: str = LEGACY_ALLOCATION,
) -> list[dict[str, Any]]:
    if not path.exists():
        return []
    try:
        document = json.loads(path.read_text(encoding="utf-8"))
    except json.JSONDecodeError as error:
        raise ValueError(f"parse observations {path}: {error}") from error
    if not isinstance(document, dict):
        raise ValueError(f"observations {path} must contain an object")
    if document.get("schema_version") != OBSERVATION_SCHEMA_VERSION:
        raise ValueError(
            f"observations {path} use unsupported schema_version "
            f"{document.get('schema_version')!r}; expected {OBSERVATION_SCHEMA_VERSION}"
        )
    if document.get("repository") != repo:
        raise ValueError(
            f"observations {path} are for repository {document.get('repository')!r}, expected {repo!r}"
        )
    if document.get("cohort") != cohort:
        raise ValueError(f"observations {path} cohort does not match requested {cohort!r}")
    if document.get("allocation_id", LEGACY_ALLOCATION) != allocation_id:
        raise ValueError(f"observations {path} allocation does not match requested {allocation_id!r}")
    runs = document.get("runs")
    if not isinstance(runs, list):
        raise ValueError(f"observations {path} field runs must be an array")
    validated = [validate_run(run, index) for index, run in enumerate(runs)]
    if any(run["cohort"] != cohort for run in validated):
        raise ValueError(f"observations {path} contain mixed cohorts")
    if any(run.get("allocation_id", LEGACY_ALLOCATION) != allocation_id for run in validated):
        raise ValueError(f"observations {path} contain mixed allocations")
    return validated


def merge_runs(
    saved: Iterable[dict[str, Any]],
    collected: Iterable[dict[str, Any]],
) -> list[dict[str, Any]]:
    by_run_id = {int(run["run_id"]): run for run in saved}
    by_run_id.update({int(run["run_id"]): run for run in collected})
    return sorted(
        by_run_id.values(),
        key=lambda run: (timestamp(run["created_at"]), int(run["run_id"])),
        reverse=True,
    )


def observation_document(
    repo: str, runs: list[dict[str, Any]], cohort: str = "cache-enabled", allocation_id: str = LEGACY_ALLOCATION,
) -> dict[str, Any]:
    return {
        "schema_version": OBSERVATION_SCHEMA_VERSION,
        "repository": repo,
        "cohort": cohort,
        "allocation_id": allocation_id,
        "runs": runs,
    }


def write_json(path: Path, value: Any) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    with tempfile.NamedTemporaryFile(
        mode="w",
        encoding="utf-8",
        dir=path.parent,
        prefix=f".{path.name}.",
        delete=False,
    ) as temporary:
        json.dump(value, temporary, indent=2, sort_keys=True)
        temporary.write("\n")
        temporary_path = Path(temporary.name)
    temporary_path.replace(path)


def summarize(runs: list[dict[str, Any]]) -> dict[str, Any]:
    metric_names = [
        "workflow_admission_delay_seconds",
        "queue_to_required_status_seconds",
        "build_job_seconds",
        "build_kongctl_seconds",
        "build_scenario_binary_seconds",
        "build_setup_seconds",
        "harness_job_seconds",
        "harness_setup_seconds",
        "harness_test_seconds",
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


def markdown_report(
    repo: str,
    runs: list[dict[str, Any]],
    summary: dict[str, Any],
    target_count: int,
    cohort: str = "cache-enabled",
    frozen: bool = False,
    allocation_id: str = LEGACY_ALLOCATION,
) -> str:
    status = "frozen preliminary" if frozen else ("complete" if len(runs) >= target_count else "collecting")
    lines = [
        "# Konnect `.com` E2E baseline",
        "",
        f"Repository: `{repo}`",
        f"Cohort: `{cohort}`",
        f"Allocation: `{allocation_id}`",
        "",
        f"Full successful runs: {len(runs)} of {target_count}",
        f"Status: **{status}**",
        "",
        "The report scans successful `e2e.yaml` runs newest-first and retains only",
        "runs with a complete latest-attempt metrics manifest for every `.com` shard.",
        "Build, harness, scenario, coverage-verification, and required-status jobs",
        "must succeed. Short gate-only runs are excluded.",
        "Percentiles use the nearest-rank method.",
        "Latency starts at the selected attempt's creation time. Jobs reused from an",
        "earlier attempt are excluded. Cache-enabled identifies the cache-reporting",
        "step introduced by #2069 and includes both hits and misses. Keep that step",
        "when changing the cache policy. Each run ID contributes one saved successful",
        "attempt; reruns are not independent samples.",
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
            "| Run / Attempt | Attempt created | Queue-to-status | Build | Longest shard | Spread | Resets |",
            "| --- | --- | ---: | ---: | ---: | ---: | ---: |",
        ]
    )
    for run in runs:
        lines.append(
            f"| [{run['run_id']} / {run['run_attempt']}]({run['url']}/attempts/{run['run_attempt']}) | "
            f"{run['created_at']} | "
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
    parser.add_argument("--cohort", choices=COHORTS, default="cache-enabled")
    parser.add_argument("--allocation-id", default=LEGACY_ALLOCATION,
                        help="modulo-v1 or weighted-v1:<snapshot SHA-256>; never pool allocations")
    parser.add_argument("--frozen", action="store_true", help="report saved observations without collecting")
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--json-output", type=Path)
    parser.add_argument(
        "--observations",
        type=Path,
        help="versioned cumulative observation file to read and update",
    )
    parser.add_argument(
        "--allow-partial",
        action="store_true",
        help="write a collecting report and exit successfully before reaching --count",
    )
    args = parser.parse_args()
    if args.count < 1 or args.scan < 1:
        parser.error("--count and --scan must be positive")
    if not valid_allocation(args.allocation_id):
        parser.error("invalid --allocation-id; use modulo-v1 or weighted-v1:<64 lowercase hex characters>")
    if args.frozen and (args.observations is None or not args.observations.exists()):
        parser.error("--frozen requires an existing --observations file")

    try:
        saved = load_observations(args.observations, args.repo, args.cohort, args.allocation_id) if args.observations else []
    except ValueError as error:
        parser.error(str(error))
    saved_run_ids = {int(run["run_id"]) for run in saved}
    collected = collect_runs(
        args.repo,
        0 if args.frozen else max(0, args.count - len(saved)),
        args.scan,
        excluded_run_ids=saved_run_ids,
        cohort=args.cohort,
        allocation_id=args.allocation_id,
    )
    runs = merge_runs(saved, collected)
    if args.observations is not None:
        write_json(args.observations, observation_document(args.repo, runs, args.cohort, args.allocation_id))
    if len(runs) < args.count and not args.frozen:
        message = (
            f"found {len(runs)} eligible full .com runs, need {args.count}; "
            "increase --scan or collect again before older artifacts expire"
        )
        if not args.allow_partial:
            raise SystemExit(message)
        print(message, file=sys.stderr)
    summary = summarize(runs)
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(
        markdown_report(args.repo, runs, summary, args.count, args.cohort, args.frozen, args.allocation_id), encoding="utf-8"
    )
    if args.json_output is not None:
        write_json(
            args.json_output,
            {
                "schema_version": OBSERVATION_SCHEMA_VERSION,
                "cohort": args.cohort,
                "allocation_id": args.allocation_id,
                "summary": summary,
                "target_run_count": args.count,
                "runs": runs,
            },
        )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
