#!/usr/bin/env python3
"""Deterministic, whole-scenario PR replay routing and result verification."""

import argparse
import importlib.util
import os
from pathlib import Path
import subprocess

SPEC = importlib.util.spec_from_file_location("replay", Path(__file__).with_name("e2e-replay.py"))
REPLAY = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(REPLAY)


def enabled_scenarios(root):
    policy = REPLAY.parse_json((root / "test/e2e/replay-scenarios.json").read_bytes())
    if (set(policy) != {"schema_version", "scenarios"} or policy["schema_version"] != 1
            or not isinstance(policy["scenarios"], list)
            or any(s not in REPLAY.SCENARIOS for s in policy["scenarios"])
            or policy["scenarios"] != sorted(set(policy["scenarios"]))):
        raise ValueError("invalid replay eligibility policy")
    return policy["scenarios"]


def make_plan(root, mode):
    directory = root / "test/e2e/scenarios"
    inventory = sorted(p.relative_to(directory).as_posix() for p in directory.rglob("scenario.yaml"))
    selected = enabled_scenarios(root) if mode == "pr" else []
    for scenario in selected:
        cassette_dir = directory / scenario
        REPLAY.validate_cassette(REPLAY.parse_json((cassette_dir / "replay/cassette.json").read_bytes()),
                                cassette_dir, scenario)
    replay = [s + "/scenario.yaml" for s in selected]
    if not set(replay) <= set(inventory):
        raise ValueError("replay scenario missing from inventory")
    return {"schema_version": 1, "mode": mode, "replay": replay,
            "live": [s for s in inventory if s not in replay]}


def verify_results(plan, result_directory, commit, run_id):
    expected = plan["replay"]
    reports = [REPLAY.parse_json(p.read_bytes()) for p in result_directory.rglob("replay-results.json")]
    if not expected:
        if reports:
            raise ValueError("unexpected replay results for an all-live run")
        return
    if not reports:
        raise ValueError("missing replay results")
    for report in reports:
        if report["commit"] != commit or report["run_id"] != run_id:
            raise ValueError("replay results belong to another execution")
    report = max(reports, key=lambda r: int(r["run_attempt"]))
    summaries = report["scenarios"]
    if sorted(s["scenario"] + "/scenario.yaml" for s in summaries) != expected:
        raise ValueError("replay result coverage mismatch")
    for summary in summaries:
        if (summary["mode"] != "replay" or summary["status"] != "pass"
                or summary["network_isolated"] is not True or summary["source_kind"] != "recorded"
                or summary["interactions"] <= 0):
            raise ValueError("replay did not pass in network isolation")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("command", choices=("plan", "run", "verify"))
    parser.add_argument("--mode", choices=("pr", "live"), default="live")
    parser.add_argument("--root", type=Path, default=REPLAY.ROOT,
                        help="scenario data checkout; never used to load executable code")
    parser.add_argument("--plan", type=Path, default=Path("e2e-routing.json"))
    parser.add_argument("--results", type=Path, default=Path(".e2e-artifacts/pr-replay"))
    args = parser.parse_args()
    if args.command == "plan":
        plan = make_plan(args.root, args.mode)
        REPLAY.write_json(args.plan, plan)
        print(f"Live: {len(plan['live'])}; replay: {len(plan['replay'])}; mode: {plan['mode']}")
        if os.environ.get("GITHUB_OUTPUT"):
            with open(os.environ["GITHUB_OUTPUT"], "a", encoding="utf-8") as output:
                output.write(f"replay_count={len(plan['replay'])}\n")
        return
    plan = REPLAY.parse_json(args.plan.read_bytes())
    if plan != make_plan(args.root, args.mode):
        raise ValueError("routing plan differs from checked-out scenario inventory")
    commit = subprocess.check_output(["git", "-C", str(args.root), "rev-parse", "HEAD"], text=True).strip()
    if args.command == "verify":
        verify_results(plan, args.results, commit, os.environ["GITHUB_RUN_ID"])
        return
    summaries = []
    for name in plan["replay"]:
        scenario = name.removesuffix("/scenario.yaml")
        output = args.results / scenario
        subprocess.run(["bash", "scripts/e2e-replay-isolated.sh", "--scenario", scenario,
                        "--test-binary", "e2e.test", "--output-dir", str(output)], check=True)
        summaries.append(REPLAY.parse_json((output / "summary.json").read_bytes()))
    REPLAY.write_json(args.results / "replay-results.json", {
        "commit": commit, "run_id": os.environ["GITHUB_RUN_ID"],
        "run_attempt": int(os.environ["GITHUB_RUN_ATTEMPT"]), "scenarios": summaries,
    })
    if os.environ.get("GITHUB_STEP_SUMMARY"):
        with open(os.environ["GITHUB_STEP_SUMMARY"], "a", encoding="utf-8") as output:
            output.write("## Offline replay results\n\n")
            output.write("| Scenario | Scenario seconds | Wrapper seconds | Interactions |\n")
            output.write("| --- | ---: | ---: | ---: |\n")
            for summary in summaries:
                output.write(f"| {summary['scenario']} | {summary['scenario_seconds']} | "
                             f"{summary['elapsed_seconds']} | {summary['interactions']} |\n")


if __name__ == "__main__":
    main()
