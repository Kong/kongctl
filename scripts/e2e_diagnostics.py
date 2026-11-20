#!/usr/bin/env python3
"""Summarize observational scenario diagnostics; never make retry decisions."""

import argparse
from collections import Counter
import html
import json
from pathlib import Path


def cell(value):
    return html.escape(str(value)).replace("|", "&#124;").replace("\n", " ").replace("\r", " ")


def validate_fields(value, required, optional=None):
    if not isinstance(value, dict):
        raise ValueError("expected an object")
    for key, expected_type in required.items():
        if key not in value or type(value[key]) is not expected_type:
            raise ValueError(f"invalid or missing {key}")
    for key, expected_type in (optional or {}).items():
        if key in value and type(value[key]) is not expected_type:
            raise ValueError(f"invalid {key}")


def validate_record(record):
    # Validate the fields consumed by rendering before accepting the record.
    # Unknown fields remain allowed for compatible schema additions.
    validate_fields(record, {"schema_version": int, "scenario": str, "commands": list})
    if record["schema_version"] != 1:
        raise ValueError("unsupported diagnostics")
    failure = record.get("failure")
    if failure is not None:
        validate_fields(failure, {"phase": str, "cause": str}, {"step": str, "command": str})
    for command in record["commands"]:
        validate_fields(command, {
            "step": str, "command": str, "outcome": str, "duration_ms": int, "attempts": list,
        }, {"retry_stop": str, "attempt_limit": int})
        if command["duration_ms"] < 0 or command.get("attempt_limit", 0) < 0:
            raise ValueError("negative command duration or attempt limit")
        for attempt in command["attempts"]:
            validate_fields(attempt, {"duration_ms": int, "timeout_ms": int}, {
                "kind": str, "timed_out": bool, "exit_code": int, "http_status": int,
                "http_method": str, "http_host": str, "error_class": str,
            })
            if attempt["duration_ms"] < 0 or attempt["timeout_ms"] < 0:
                raise ValueError("negative attempt duration or timeout")


def collect_recoveries(root):
    records = []
    for path in sorted(root.rglob("scenario-recovery.json")):
        record = json.loads(path.read_text())
        validate_fields(record, {"scenario": str, "mode": str, "outcome": str, "eligible": bool}, {"class": str})
        # Raw operation errors remain in the attempt artifacts, never exported
        # into the consolidated report or workflow annotations.
        records.append({key: record.get(key, "") for key in ("scenario", "mode", "outcome", "eligible", "class")})
    return records


def recovery_summary(records):
    if not records:
        return []
    counts = Counter(r["outcome"] for r in records)
    lines = ["", "**Scenario recovery**", "",
             f"First-pass success: {counts['first_pass']}; recovered: {counts['recovered']}; "
             f"exhausted retry: {counts['exhausted']}; reset failed: {counts['reset_failed']}.", "",
             "| Scenario | Mode | Outcome | Eligible | Class |", "| --- | --- | --- | --- | --- |"]
    lines += ["| " + " | ".join(cell(r[k]) for k in ("scenario", "mode", "outcome", "eligible", "class")) + " |"
              for r in records if r["outcome"] != "first_pass"]
    return lines + ["", "First failures and trace IDs are preserved in attempt-1 artifacts and scenario-recovery.json."]


def summarize(root):
    records = []
    unavailable = 0
    for path in sorted(root.rglob("scenario-diagnostics.json")):
        try:
            record = json.loads(path.read_text())
            validate_record(record)
            records.append(record)
        except (OSError, ValueError):
            unavailable += 1

    lines = ["### Scenario diagnostics", "", "Attempt diagnostics are observational; scenario recovery is reported separately.", ""]
    lines.append(f"Captured {len(records)} scenario records; {unavailable} unreadable or unsupported records.")
    lines += ["", "Missing records are not evidence of success (for example, a job killed before capture).", ""]
    try:
        lines += recovery_summary(collect_recoveries(root)) + [""]
    except (OSError, ValueError):
        lines += ["Scenario recovery evidence is unavailable.", ""]
    failures = [record for record in records if record.get("failure")]
    if failures:
        lines += [
            "| Scenario | Step / command | Phase | Attempt cause | HTTP evidence | Last attempt / limit | Attempts | Execution retry stop |",
            "| --- | --- | --- | --- | --- | --- | --- | --- |",
        ]
        for record in failures:
            failure = record["failure"]
            commands = record["commands"]
            command = commands[-1] if commands and commands[-1]["outcome"] == "failed" else {}
            attempts = command.get("attempts", [])
            last = attempts[-1] if attempts else {}
            timing = f'{last.get("duration_ms", "unknown")} / {last.get("timeout_ms", "unknown")} ms'
            evidence = "unknown"
            if failure["phase"] == "http" and last.get("kind") == "http":
                evidence = " ".join(str(last[key]) for key in
                                    ("http_method", "http_host", "http_status", "error_class") if key in last)
            fields = [record["scenario"], f'{failure.get("step", "")} / {failure.get("command", "")}',
                      failure["phase"], failure["cause"], evidence, timing,
                      f'{len(attempts)} / {command.get("attempt_limit", "unknown")}',
                      command.get("retry_stop", "unknown")]
            lines.append("| " + " | ".join(map(cell, fields)) + " |")
    else:
        lines.append("No attempt failures in the captured records.")

    commands = [(record["scenario"], command) for record in records for command in record["commands"]]
    commands.sort(key=lambda item: item[1]["duration_ms"], reverse=True)
    if commands:
        lines += ["", "Slowest commands (including successful commands; up to 15):", "",
                  "| Scenario | Step / command | Outcome | Total duration | Attempts |",
                  "| --- | --- | --- | --- | --- |"]
        for scenario, command in commands[:15]:
            fields = [scenario, f'{command["step"]} / {command["command"]}', command["outcome"],
                      f'{command["duration_ms"]} ms', len(command["attempts"])]
            lines.append("| " + " | ".join(map(cell, fields)) + " |")
    lines += ["", "Full command timings and available HTTP metadata are in scenario-diagnostics.json",
              "alongside each scenario's artifacts. Total duration includes retries and assertions.", ""]
    return "\n".join(lines)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("artifacts", type=Path)
    parser.add_argument("summary", type=Path)
    args = parser.parse_args()
    with args.summary.open("a") as output:
        output.write(summarize(args.artifacts))


if __name__ == "__main__":
    main()
