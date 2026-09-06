#!/usr/bin/env python3
"""Generate deterministic scenario weights from saved, successful baselines."""

from __future__ import annotations

import argparse
import hashlib
import json
import math
import statistics
from collections import defaultdict
from pathlib import Path


def normalize(path: str) -> str:
    for prefix in ("test/e2e/scenarios/", "scenarios/"):
        if path.startswith(prefix):
            path = path[len(prefix):]
    return path.rstrip("/")


def generate(paths: list[Path], scenario_root: Path) -> dict:
    active = {p.relative_to(scenario_root).as_posix() for p in scenario_root.rglob("scenario.yaml")}
    if not active:
        raise ValueError(f"no scenarios found in {scenario_root}")
    runs = {}
    sources = {}
    for path in sorted(set(paths)):
        raw = path.read_bytes()
        document = json.loads(raw)
        if document.get("schema_version") != 2 or document.get("repository") != "kong/kongctl":
            raise ValueError(f"unsupported observations: {path}")
        sources[path.as_posix()] = hashlib.sha256(raw).hexdigest()
        for run in document["runs"]:
            run_id = run["run_id"]
            previous = runs.get(run_id)
            if previous is not None:
                if previous["run_attempt"] == run["run_attempt"]:
                    if previous["scenario_durations"] != run["scenario_durations"]:
                        raise ValueError(f"conflicting observations for run {run_id}")
                    continue
                if previous["run_attempt"] > run["run_attempt"]:
                    continue
            runs[run_id] = run
    durations = defaultdict(list)
    for run in runs.values():
        seen = set()
        for record in run["scenario_durations"]:
            path = normalize(record["scenario"])
            if path in seen:
                raise ValueError(f"duplicate scenario {path} in run {run['run_id']}")
            seen.add(path)
            seconds = record["duration_seconds"]
            if record["result"] != "pass" or path not in active:
                continue
            if isinstance(seconds, bool) or not isinstance(seconds, (int, float)):
                raise ValueError(f"invalid duration for {path}")
            if not math.isfinite(seconds) or seconds <= 0:
                continue
            durations[path].append(seconds * 1000)
    weights = {
        path: {"duration_ms": max(1, round(statistics.median(values))), "samples": len(values)}
        for path, values in sorted(durations.items()) if len(values) >= 10
    }
    fallback = max(1, round(statistics.median(w["duration_ms"] for w in weights.values()))) if weights else 1000
    return {
        "schema_version": 1,
        "default_duration_ms": fallback,
        "sources": list(sources),
        "source_sha256": sources,
        "scenarios": weights,
    }


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("observations", nargs="+", type=Path)
    parser.add_argument("--scenario-root", type=Path, default=Path("test/e2e/scenarios"))
    parser.add_argument("--output", type=Path, default=Path("test/e2e/baselines/scenario-weights.json"))
    args = parser.parse_args()
    try:
        weights = generate(args.observations, args.scenario_root)
    except (ValueError, KeyError, OSError) as error:
        parser.error(str(error))
    args.output.write_text(json.dumps(weights, indent=2, sort_keys=True) + "\n")


if __name__ == "__main__":
    main()
