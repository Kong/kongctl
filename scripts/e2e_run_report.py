#!/usr/bin/env python3
"""Collect compact, redacted E2E evidence and render one run-level report."""
from __future__ import annotations

import argparse
from collections import Counter
from datetime import datetime
import html
import json
import os
from pathlib import Path
import re
import shlex

from e2e_metrics import collect_metrics, parse_key_values


def read(path):
    return json.loads(path.read_text())


def write(path, data):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(data, indent=2, sort_keys=True) + "\n")


def classification(value):
    value = value.lower()
    for token, label in (("tls handshake timeout", "tls_timeout"),
                         ("connection reset", "connection_reset"),
                         ("timeout", "timeout"), ("deadline exceeded", "timeout"),
                         ("eof", "network"), ("transport", "network"),
                         ("connection refused", "network")):
        if token in value:
            return label
    return "unknown"


def duration_ms(value):
    # slog.Duration text, not arbitrary error text.
    units = {"h": 3600000, "m": 60000, "s": 1000, "ms": 1, "µs": .001, "ns": .000001}
    parts = re.findall(r"(\d+(?:\.\d+)?)(ms|µs|ns|h|m|s)", value)
    return round(sum(float(n) * units[u] for n, u in parts)) if parts else None


def http_events(path):
    events = []
    for line_number, line in enumerate(path.read_text(errors="replace").splitlines(), 1):
        if "log_type=http_error" not in line and "event=retry_attempt" not in line:
            continue
        try:
            fields = dict(token.split("=", 1) for token in shlex.split(line) if "=" in token)
        except ValueError:
            continue
        # Transport failures are counted from http_error only. Status retries
        # have no http_error record, so count those from retry_attempt instead.
        if fields.get("log_type") == "http_error":
            kind = classification(fields.get("error", "") + " " + fields.get("error_class", ""))
        elif fields.get("event") == "retry_attempt" and fields.get("status_code", "").isdigit():
            kind = "http_" + fields["status_code"]
        else:
            continue
        method = fields.get("method", "")
        if method not in {"GET", "HEAD", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}:
            method = "HTTP"
        events.append({"class": kind, "operation": "CLI " + method,
                       "duration_ms": duration_ms(fields.get("duration", "")), "line": line_number})
    return events


def collect(root, environ):
    data = collect_metrics(root, environ)
    result = parse_key_values(next(root.rglob("scenario-results.txt")))
    data["results"] = {name: int(result.get(name + "_count", "0"))
                       for name in ("passed", "failed", "skipped", "beta_failed")}
    data["exit_code"] = int(result.get("exit_code", "-1"))
    data["started_at"] = int(result.get("started_at", "0")) or None
    data["finished_at"] = int(result.get("finished_at", "0")) or None
    data["events"] = []
    data["failures"] = []
    data["evidence_warnings"] = []
    seen_logs, seen_reset_events = set(), set()
    diagnostics = list(root.rglob("scenario-diagnostics.json"))
    if not diagnostics:
        data["evidence_warnings"].append("No scenario diagnostics captured")
    for path in diagnostics:
        try:
            record = read(path)
            scenario = record["scenario"].removeprefix("test/e2e/scenarios/").removesuffix("/scenario.yaml")
            if record.get("failure"):
                failure = record["failure"]
                data["failures"].append({"scenario": scenario, "phase": failure.get("phase", "unknown"),
                    "cause": failure.get("cause", "unknown"), "step": failure.get("step", ""),
                    "command": failure.get("command", ""), "evidence": str(path.relative_to(root))})
            for command in record["commands"]:
                for index, attempt in enumerate(command["attempts"], 1):
                    context = {"scenario": scenario, "step": command["step"], "command": command["command"],
                               "execution_attempt": index, "outcome": "command_" + command["outcome"]}
                    if attempt.get("timed_out") and attempt.get("kind") == "subprocess":
                        data["events"].append(dict(context, **{"class": "subprocess_deadline",
                            "operation": "CLI process", "duration_ms": attempt["duration_ms"],
                            "evidence": str(path.relative_to(root))}))
                    elif attempt.get("kind") == "http" and (attempt.get("error_class") or
                                                            attempt.get("http_status", 0) >= 400):
                        data["events"].append(dict(context, **{"class": "timeout" if attempt.get("timed_out")
                            else "network" if attempt.get("error_class") == "network" else "http_error",
                            "operation": "Harness HTTP", "duration_ms": attempt["duration_ms"],
                            "evidence": str(path.relative_to(root))}))
                    if not attempt.get("log_path"):
                        continue
                    log = (path.parent / attempt["log_path"]).resolve()
                    if not log.is_relative_to(root.resolve()):
                        raise ValueError("invalid evidence path")
                    if log in seen_logs:
                        continue
                    seen_logs.add(log)
                    for event in http_events(log):
                        data["events"].append(dict(context, **event, evidence=str(log.relative_to(root.resolve()))))
            for observation in path.parent.rglob("observation.json"):
                reset = read(observation)
                if reset.get("type") != "reset_summary":
                    continue
                for region in reset.get("regions", [reset]):
                    for detail in region.get("details", []):
                        for event in detail.get("events", []):
                            key = (scenario, event["timestamp"], detail["endpoint"], event["operation"], event["attempt"])
                            if key in seen_reset_events:
                                continue
                            seen_reset_events.add(key)
                            data["events"].append({"scenario": scenario, "operation": "Reset " + event["operation"] +
                                " " + detail["endpoint"], "class": event["class"],
                                "duration_ms": event["duration_ms"], "http_attempt": event["attempt"],
                                "outcome": event["outcome"], "evidence": str(observation.relative_to(root))})
        except (OSError, ValueError, KeyError, TypeError):
            data["evidence_warnings"].append("Incomplete scenario evidence: " + path.parent.name)
    # Preserve advisory failure counts without interpreting PASS wrappers as clean results.
    return data


def stamp(value):
    if not value:
        return None
    return datetime.fromisoformat(value.replace("Z", "+00:00")).timestamp()


def elapsed(start, end):
    return round(end - start) if start is not None and end is not None and end >= start else None


def latest(items, key, order):
    selected = {}
    for item in items:
        if key(item) not in selected or order(item) > order(selected[key(item)]):
            selected[key(item)] = item
    return list(selected.values())


def aggregate(root, metadata, needs):
    warnings = []
    shards = []
    for path in root.rglob("e2e-report-shard.json"):
        try:
            shard = read(path)
            if str(shard["run_id"]) != str(metadata["run"]["id"]):
                raise ValueError("wrong run")
            shards.append(shard)
        except (OSError, ValueError, KeyError, TypeError):
            warnings.append("Unreadable shard report")
    shards = latest(shards, lambda s: s["org_name"], lambda s: s["run_attempt"])
    jobs = latest(metadata.get("jobs", []), lambda j: j["name"], lambda j: j["id"])
    shard_jobs = [j for j in jobs if j["name"].startswith("Run scenarios — ")]
    expected_orgs = {j["name"].removeprefix("Run scenarios — ") for j in shard_jobs}
    required = needs.get("e2e-needed", {}).get("outputs", {})
    if required.get("run_shards") == "true":
        expected_orgs.update(o["org_name"] for o in json.loads(required.get("orgs_json") or "[]"))
    missing = sorted(expected_orgs - {s["org_name"] for s in shards})
    stale = []
    for shard in shards:
        job = next((j for j in shard_jobs if j["name"] == "Run scenarios — " + shard["org_name"]), {})
        shard["job_url"] = job.get("html_url")
        shard["job_result"] = job.get("conclusion", "unknown")
        shard["job_seconds"] = elapsed(stamp(job.get("started_at")), stamp(job.get("completed_at")))
        # Do not silently use old successful evidence when a newer attempt ran.
        if job.get("run_attempt", shard["run_attempt"]) > shard["run_attempt"]:
            missing.append(shard["org_name"] + " (latest attempt missing)")
            stale.append(shard)
        warnings.extend(shard.get("evidence_warnings", []))
    shards = [s for s in shards if s not in stale]
    routing = None
    for path in root.rglob("e2e-routing.json"):
        try:
            candidate = read(path)
            if not all(isinstance(candidate.get(k), list) for k in ("live", "replay")):
                raise ValueError("invalid routing")
            routing = candidate
        except (OSError, ValueError, TypeError, AttributeError):
            warnings.append("Unreadable routing plan")
    replay_candidates = []
    for path in root.rglob("replay-results.json"):
        try:
            record = read(path)
            if str(record.get("run_id")) == str(metadata["run"]["id"]):
                int(record.get("run_attempt", 1))
                if not isinstance(record.get("scenarios"), list):
                    raise ValueError("invalid replay results")
                replay_candidates.append(record)
        except (OSError, ValueError, TypeError, AttributeError):
            warnings.append("Unreadable replay results")
    replay = max(replay_candidates, key=lambda r: int(r.get("run_attempt", 1)), default={})
    replay_job = next((j for j in jobs if j["name"] == "Replay approved scenarios"), {})
    if replay and replay_job.get("run_attempt", 1) > int(replay.get("run_attempt", 1)):
        replay = {}
        warnings.append("Latest replay attempt evidence missing")
    replay_scenarios = replay.get("scenarios", [])
    assigned_replay = len(routing["replay"]) if routing else None
    assigned_live = len(routing["live"]) if routing else None
    live_counts = {k: sum(s["results"][k] for s in shards) for k in ("passed", "failed", "skipped", "beta_failed")}
    replay_counts = {k: sum(s["status"] == status for s in replay_scenarios)
                     for k, status in (("passed", "pass"), ("failed", "fail"), ("skipped", "skip"))}
    required = needs.get("e2e-needed", {}).get("outputs", {})
    verification = needs.get("e2e-verify", {}).get("result", "unknown")
    state = "INCOMPLETE"
    if required.get("required") == "false":
        state = "NOT REQUIRED"
    elif required.get("superseded") == "true":
        state = "SUPERSEDED"
    elif required.get("trusted_e2e_required") == "true" and required.get("run_shards") != "true":
        state = "AWAITING TRUSTED RUN"
    elif any(n.get("result") == "failure" for n in needs.values()):
        state = "FAILED"
    elif any(n.get("result") == "cancelled" for n in needs.values()):
        state = "CANCELLED / INCOMPLETE"
    elif (verification == "success" and routing is not None and not missing and (shards or assigned_live == 0)
          and assigned_live == sum(sum(s["results"].values()) for s in shards)
          and assigned_replay == len(replay_scenarios)
          and not live_counts["failed"] and not replay_counts["failed"]):
        state = "PASSED WITH ADVISORIES" if live_counts["beta_failed"] else "PASSED"
    if routing is None:
        warnings.append("Routing plan unavailable; assigned counts unknown")
    if assigned_replay and len(replay_scenarios) != assigned_replay:
        warnings.append("Replay results incomplete")
    if assigned_live is not None and sum(live_counts.values()) != assigned_live:
        warnings.append("Live results incomplete")
    starts = [s["started_at"] for s in shards if s.get("started_at")]
    ends = [s["finished_at"] for s in shards if s.get("finished_at")]
    live_seconds = elapsed(min(starts), max(ends)) if starts and len(starts) == len(shards) == len(ends) and not missing else None
    replay_job = next((j for j in jobs if j["name"] == "Replay approved scenarios"), {})
    replay_step = next((s for s in replay_job.get("steps", []) if s["name"] == "Run offline replay"), {})
    replay_start, replay_end = stamp(replay_step.get("started_at")), stamp(replay_step.get("completed_at"))
    execution_starts = starts + ([replay_start] if replay_start else [])
    execution_ends = ends + ([replay_end] if replay_end else [])
    all_seconds = elapsed(min(execution_starts), max(execution_ends)) if execution_starts and execution_ends and (
        live_seconds is not None or assigned_live == 0) and (
        assigned_replay == 0 or replay_start is not None and replay_end is not None) else None
    build_job = next((j for j in jobs if j["name"] == "Build scenario executables"), {})
    completed = [stamp(j.get("completed_at")) for j in jobs if j.get("completed_at")
                 and j["name"] != "E2E run summary"]
    build_details = []
    for path in root.rglob("e2e-build-details.md"):
        build_details = [line for line in path.read_text().splitlines()
                         if line.startswith(("Go build cache", "Go fallback", "Scenario routing:"))]
    first_job = min((stamp(j.get("started_at")) for j in jobs if j.get("started_at")), default=None)
    return {"schema_version": 1, "run_id": metadata["run"]["id"], "state": state,
            "reviewed_sha": required.get("status_sha"),
            "run_url": metadata["run"]["html_url"], "head_sha": metadata["run"]["head_sha"],
            "initial_queue_seconds": elapsed(stamp(metadata["run"]["created_at"]), first_job),
            "workflow_through_validation_seconds": elapsed(first_job, max(completed)) if completed else None,
            "build_seconds": elapsed(stamp(build_job.get("started_at")), stamp(build_job.get("completed_at"))),
            "build_details": build_details,
            "scenario_window_seconds": all_seconds, "live_window_seconds": live_seconds,
            "replay_seconds": elapsed(replay_start, replay_end), "live_counts": live_counts,
            "replay_counts": replay_counts, "assigned_live": assigned_live, "assigned_replay": assigned_replay,
            "missing_shards": missing, "warnings": sorted(set(warnings)), "shards": shards,
            "replay_scenarios": replay_scenarios, "routing": routing,
            "checks": {k: v.get("result", "unknown") for k, v in needs.items()},
            "artifacts": [{"name": a["name"], "url": metadata["run"]["html_url"] + "/artifacts/" + str(a["id"])}
                          for a in metadata.get("artifacts", [])]}


def cell(value):
    return html.escape(str(value)).replace("|", "&#124;").replace("\n", " ").replace("\r", " ").replace("[", "&#91;").replace("]", "&#93;")


def seconds(value):
    if value is None:
        return "unknown"
    return f"{int(value)//60}m {int(value)%60:02d}s"


def render(report):
    lines = ["# E2E run summary — " + report["state"], "",
             "Runtime and result counts cover the selected attempts. Missing evidence is never treated as success.", "",
             "| Mode | Assigned | Passed | Failed | Skipped | Advisory failures |",
             "| --- | ---: | ---: | ---: | ---: | ---: |"]
    if report["state"] == "AWAITING TRUSTED RUN":
        lines[2:2] = ["A maintainer must trigger a trusted run: fork PR code cannot receive E2E secrets automatically.",
                      "", "Reviewed SHA required: `" + cell(report.get("reviewed_sha") or "unknown") + "`", ""]
    for label, assigned, counts in (("Live Konnect", report["assigned_live"], report["live_counts"]),
                                    ("Recorded replay", report["assigned_replay"], report["replay_counts"])):
        lines.append(f"| {label} | {assigned if assigned is not None else 'unknown'} | {counts['passed']} | "
                     f"{counts['failed']} | {counts['skipped']} | {counts.get('beta_failed', 0)} |")
    lines += ["", "**Runtime**", "", "| Measurement | Duration |", "| --- | ---: |"]
    for label, key in (("Scenario execution window (live + replay)", "scenario_window_seconds"),
                       ("Live scenario window", "live_window_seconds"), ("Replay execution step", "replay_seconds"),
                       ("Build job (including setup)", "build_seconds"),
                       ("Initial queue wait", "initial_queue_seconds"),
                       ("Workflow elapsed through validation", "workflow_through_validation_seconds")):
        lines.append(f"| {label} | {seconds(report[key])} |")
    lines += ["", "Execution windows span the first scenario runner start to the last finish; they exclude build, "
              "initial queueing, and artifact upload, but include launch skew and waits between selected attempts. "
              "Shard runner durations include reset and retries. Parallel durations are not summed as wall time. "
              "Workflow elapsed starts at the first job, includes inter-job waits, and excludes this reporting job.",
              "", "**Live shards**", "", "| Shard | Attempt | Passed / assigned | Skipped | Runner | Whole job | Result |",
              "| --- | ---: | ---: | ---: | ---: | ---: | --- |"]
    for s in sorted(report["shards"], key=lambda s: s["org_name"]):
        name = cell(s["org_name"])
        if s.get("job_url"):
            name = f"[{name}]({s['job_url']})"
        lines.append(f"| {name} | {s['run_attempt']} | {s['results']['passed']} / {s['selected_scenario_count']} | "
                     f"{s['results']['skipped']} | {seconds(s['execution_duration_seconds'])} | "
                     f"{seconds(s['job_seconds'])} | {cell(s['job_result'])} |")
    if report["missing_shards"]:
        lines += ["", "**Missing shard evidence:** " + ", ".join(map(cell, report["missing_shards"]))]
    failures = [(s, f) for s in report["shards"] for f in s.get("failures", [])]
    if failures:
        lines += ["", "**Scenario failure evidence**", "", "| Shard / scenario | Step / command | Phase | Cause |",
                  "| --- | --- | --- | --- |"]
        lines += ["| " + " | ".join(map(cell, [s["org_name"] + " / " + f["scenario"],
                  f["step"] + " / " + f["command"], f["phase"], f["cause"]])) + " |" for s, f in failures[:20]]
        lines.append("\nFailure evidence includes advisory scenarios; blocking/advisory counts are shown above.")
    events = [(s, e) for s in report["shards"] for e in s.get("events", [])]
    lines += ["", f"**Timeouts and transient events: {len(events)} observed**", "",
              "Counts are failed HTTP attempts or process deadlines, not unique root causes. "
              "`recovered` confirms the reset operation later succeeded; `command_passed` only confirms command completion. "
              "Partial/missing logs can undercount events.", ""]
    if events:
        counts = Counter(e["class"] for _, e in events)
        lines += [", ".join(f"{cell(kind)}: {count}" for kind, count in sorted(counts.items())),
                  "", "<details><summary>Transient event details and evidence</summary>", ""]
        lines += ["| Shard / scenario | Operation / step / command | Event | Duration | Outcome | Evidence |",
                  "| --- | --- | --- | ---: | --- | --- |"]
        for shard, event in events[:50]:
            artifact = next((a for a in report["artifacts"] if a["name"] ==
                f"e2e-artifacts-{report['run_id']}-{shard['run_attempt']}-{shard['org_name']}"), None)
            evidence = cell(event["evidence"])
            if artifact:
                evidence = f"[artifact]({artifact['url']}) · `{evidence}`"
            timing = "unknown" if event.get("duration_ms") is None else f"{event['duration_ms']/1000:.3f}s"
            lines.append("| " + " | ".join([cell(shard["org_name"] + " / " + event["scenario"]),
                cell(" / ".join(str(event[k]) for k in ("operation", "step", "command") if event.get(k))), cell(event["class"]), timing, cell(event["outcome"]), evidence]) + " |")
        if len(events) > 50:
            lines.append(f"\nShowing 50 of {len(events)} events; all events are in the report JSON artifact.")
        lines += ["", "</details>"]
    else:
        lines.append("No transient events found in the available evidence.")
    if report["warnings"]:
        lines += ["", "**Evidence limitations:**"] + ["- " + cell(w) for w in report["warnings"]]
    lines += ["", "<details><summary>Coverage verification and skipped scenarios</summary>", ""]
    lines += [f"- {cell(k)}: {cell(v)}" for k, v in report["checks"].items()]
    for shard in report["shards"]:
        lines += ["- Skipped: " + cell(s["scenario"]) + " (see scenario artifact for reason)"
                  for s in shard["scenario_durations"] if s["result"] == "skip"]
    lines += ["", "</details>", "", "<details><summary>Slowest live scenarios</summary>", ""]
    scenarios = [(s["org_name"], c) for s in report["shards"] for c in s["scenario_durations"]]
    lines += [f"- {cell(org)} / {cell(s['scenario'])}: {seconds(s['duration_seconds'])}"
              for org, s in sorted(scenarios, key=lambda pair: pair[1]["duration_seconds"], reverse=True)[:10]]
    lines += ["", "</details>", "", "<details><summary>Routing, artifacts, and comparisons</summary>", "",
              "Recorded replay runs approved scenarios offline against cassettes. Live scenarios use Konnect. "
              "The routing JSON artifact contains the complete assignment lists.", "",
              "Compare runtimes only across compatible environments, routing/allocation, and scenario counts. "
              "No historical delta is claimed without a compatible baseline.", ""]
    lines += ["- " + cell(detail) for detail in report.get("build_details", [])]
    lines += [f"- [{cell(a['name'])}]({a['url']})" for a in report["artifacts"] if "binaries" not in a["name"]]
    lines += ["", "The e2e-run-summary artifact contains this Markdown, machine-readable results and event evidence, "
              "and job timing metadata. StepSecurity network monitoring remains in its own job summaries.", "", "</details>", ""]
    return "\n".join(lines)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("mode", choices=["collect", "render"])
    parser.add_argument("root", type=Path)
    parser.add_argument("output", type=Path)
    parser.add_argument("--metadata", type=Path)
    parser.add_argument("--needs", type=Path)
    args = parser.parse_args()
    if args.mode == "collect":
        write(args.output, collect(args.root, os.environ))
    else:
        report = aggregate(args.root, read(args.metadata), read(args.needs))
        write(args.output / "report.json", report)
        (args.output / "summary.md").write_text(render(report))


if __name__ == "__main__":
    main()
