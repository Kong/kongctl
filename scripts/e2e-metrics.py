#!/usr/bin/env python3
"""Build a small, secret-free metrics record from one E2E shard artifact tree."""

from __future__ import annotations

import argparse
import json
import os
import re
from pathlib import Path
from typing import Any


SCENARIO_RESULT_RE = re.compile(
    r"^\s*--- (PASS|FAIL|SKIP): Test_Scenarios/(.+?/scenario\.yaml) \(([0-9.]+)s\)\s*$"
)


def parse_key_values(path: Path) -> dict[str, str]:
    values: dict[str, str] = {}
    for line in path.read_text(encoding="utf-8").splitlines():
        if not line or line.startswith("[") or "=" not in line:
            continue
        key, value = line.split("=", 1)
        values[key] = value
    return values


def parse_manifest(path: Path) -> list[str]:
    lines = path.read_text(encoding="utf-8").splitlines()
    return [line for line in lines[3:] if line]


def parse_scenario_durations(path: Path) -> list[dict[str, Any]]:
    scenarios: list[dict[str, Any]] = []
    for line in path.read_text(encoding="utf-8", errors="replace").splitlines():
        match = SCENARIO_RESULT_RE.match(line)
        if match is None:
            continue
        result, scenario, duration = match.groups()
        scenarios.append(
            {
                "scenario": scenario.removeprefix("test/e2e/scenarios/"),
                "result": result.lower(),
                "duration_seconds": float(duration),
            }
        )
    return scenarios


def reset_details(observation: dict[str, Any]) -> tuple[int, list[dict[str, Any]]]:
    if "regions" in observation:
        regions = observation.get("regions") or []
        duration_ms = sum(int(region.get("duration_ms") or 0) for region in regions)
        details = [detail for region in regions for detail in (region.get("details") or [])]
        return duration_ms, details
    return int(observation.get("duration_ms") or 0), observation.get("details") or []


def collect_reset_metrics(root: Path) -> dict[str, int]:
    totals = {
        "count": 0,
        "duration_ms": 0,
        "list_calls": 0,
        "list_duration_ms": 0,
        "resources_found": 0,
        "delete_calls": 0,
        "delete_duration_ms": 0,
        "resources_deleted": 0,
    }
    for path in root.rglob("observation.json"):
        try:
            observation = json.loads(path.read_text(encoding="utf-8"))
        except (OSError, json.JSONDecodeError):
            continue
        if observation.get("type") != "reset_summary" or not observation.get("executed"):
            continue
        totals["count"] += 1
        duration_ms, details = reset_details(observation)
        totals["duration_ms"] += duration_ms
        for detail in details:
            totals["list_calls"] += int(detail.get("list_calls") or 0)
            totals["list_duration_ms"] += int(detail.get("list_duration_ms") or 0)
            totals["resources_found"] += int(detail.get("total") or 0)
            totals["delete_calls"] += int(detail.get("delete_calls") or 0)
            totals["delete_duration_ms"] += int(detail.get("delete_duration_ms") or 0)
            totals["resources_deleted"] += int(detail.get("deleted") or 0)
    return totals


def collect_metrics(root: Path, environ: dict[str, str]) -> dict[str, Any]:
    manifests = list(root.rglob("assigned-scenarios.txt"))
    results = list(root.rglob("scenario-results.txt"))
    run_logs = list(root.rglob("run.log"))
    if len(manifests) != 1 or len(results) != 1 or len(run_logs) != 1:
        raise ValueError("artifact tree must contain one manifest, scenario results file, and run log")

    result_values = parse_key_values(results[0])
    manifest_values = parse_key_values(manifests[0])
    scenarios = parse_scenario_durations(run_logs[0])
    assigned = parse_manifest(manifests[0])

    return {
        "schema_version": 1,
        "run_id": int(environ.get("GITHUB_RUN_ID", "0")),
        "run_attempt": int(result_values.get("run_attempt", environ.get("GITHUB_RUN_ATTEMPT", "1"))),
        "org_name": result_values.get("org_name", environ.get("KONGCTL_E2E_MATRIX_ORG", "")),
        "konnect_environment": environ.get("KONGCTL_E2E_KONNECT_ENV", "com"),
        "shard_index": int(manifest_values.get("shard_index", result_values.get("shard_index", "0"))),
        "shard_total": int(manifest_values.get("shard_total", result_values.get("shard_total", "0"))),
        "selected_scenario_count": len(assigned),
        "execution_duration_seconds": int(result_values.get("duration_seconds", "0")),
        "scenario_durations": scenarios,
        "reset": collect_reset_metrics(root),
    }


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("artifact_root", type=Path)
    parser.add_argument("output", type=Path)
    args = parser.parse_args()

    metrics = collect_metrics(args.artifact_root, dict(os.environ))
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(metrics, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
