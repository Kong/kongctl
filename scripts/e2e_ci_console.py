#!/usr/bin/env python3
"""Fold scenario detail and surface failures in the live CI console.

Reads the raw `go test -v` scenario stream on stdin and rewrites it for the
GitHub Actions job console only: each scenario's verbose output is folded
under a collapsible ::group::, repeated progress heartbeats are dropped, and
any scenario failure gets a ::error:: annotation plus an unfolded one-line
summary. The caller is responsible for also teeing the untouched raw stream
to run.log.
"""

import re
import sys

SCENARIO_RUN = re.compile(r"^=== RUN\s+Test_Scenarios/(.+)$")
SCENARIO_RESULT = re.compile(r"^\s*--- (PASS|FAIL|SKIP): Test_Scenarios/(.+?) \([0-9.]+s\)$")
HEARTBEAT = re.compile(r"^\[\d+/\d+\] elapsed \S+ · current: ")
FAILURE_DETAIL = re.compile(r"scenario failed:")
PANIC = re.compile(r"^panic:")


def scenario_display_name(path):
    path = path.removeprefix("scenarios/").removeprefix("test/e2e/scenarios/")
    return path.removesuffix("/scenario.yaml")


def escape_annotation(message):
    return message.replace("%", "%25").replace("\r", "%0D").replace("\n", "%0A")


def transform_lines(lines):
    """Yield console lines for one raw scenario-run line, preserving order."""
    group_open = False
    pending_detail = None

    for line in lines:
        line = line.rstrip("\n")

        if HEARTBEAT.match(line):
            continue

        if m := SCENARIO_RUN.match(line):
            if group_open:
                yield "::endgroup::"
            pending_detail = None
            yield f"::group::{scenario_display_name(m.group(1))}"
            group_open = True
            yield line
            continue

        if m := SCENARIO_RESULT.match(line):
            if group_open:
                yield "::endgroup::"
                group_open = False
            if m.group(1) == "FAIL":
                detail = pending_detail or line.strip()
                yield f"::error::{escape_annotation(f'{scenario_display_name(m.group(2))}: {detail}')}"
            pending_detail = None
            yield line
            continue

        if PANIC.match(line):
            if group_open:
                yield "::endgroup::"
                group_open = False
            yield f"::error::{escape_annotation(line)}"
            yield line
            continue

        if group_open and pending_detail is None and FAILURE_DETAIL.search(line):
            pending_detail = line.strip()

        yield line

    if group_open:
        yield "::endgroup::"


def main():
    sys.stdin.reconfigure(errors="replace")
    for line in transform_lines(sys.stdin):
        sys.stdout.write(line + "\n")
        sys.stdout.flush()


if __name__ == "__main__":
    main()
