#!/usr/bin/env bash
set -euo pipefail

# Temporary post-compile patches for gh-aw generated workflows.
# Remove these once upstream gh-aw stops writing workflow inputs directly into
# shell commands and emits least-privilege safe-output job permissions.
if [ "$#" -gt 0 ]; then
  workflows=("$@")
else
  workflows=(
    ".github/workflows/agentics-maintenance.yml"
    ".github/workflows/benchmark-regression-analysis.lock.yml"
  )
fi

python3 - "${workflows[@]}" <<'PY'
from pathlib import Path
import sys

EXPR_OPEN = "${{"
EXPR_CLOSE = "}}"


def replace_required(text, old, new, label):
    if old in text:
        return text.replace(old, new), True
    if new in text:
        return text, False
    raise SystemExit(f"expected generated block not found for {label}")


def patch_maintenance(text):
    operation_input = f"{EXPR_OPEN} inputs.operation {EXPR_CLOSE}"
    run_url_input = f"{EXPR_OPEN} inputs.run_url {EXPR_CLOSE}"
    replacements = [
        (
            f'''      - name: Record outputs
        id: record
        run: echo "operation={operation_input}" >> "$GITHUB_OUTPUT"
''',
            f'''      - name: Record outputs
        id: record
        env:
          OPERATION: {operation_input}
        run: echo "operation=$OPERATION" >> "$GITHUB_OUTPUT"
''',
            "operation output",
        ),
        (
            f'''      - name: Record outputs
        id: record
        run: echo "run_url={run_url_input}" >> "$GITHUB_OUTPUT"
''',
            f'''      - name: Record outputs
        id: record
        env:
          RUN_URL: {run_url_input}
        run: echo "run_url=$RUN_URL" >> "$GITHUB_OUTPUT"
''',
            "run_url output",
        ),
    ]

    changed = False
    for old, new, label in replacements:
        text, did_change = replace_required(text, old, new, label)
        changed = changed or did_change
    return text, changed


def patch_benchmark_safe_outputs_config(text):
    end_markers = (
        "      - name: Write Safe Outputs Tools\n",
        "      - name: Generate Safe Outputs Tools\n",
    )
    start_markers = (
        "      - name: Write Safe Outputs Config\n",
        "      - name: Generate Safe Outputs Config\n",
    )
    start_marker = next((marker for marker in start_markers if marker in text), None)
    if start_marker is None:
        raise SystemExit("expected generated block not found for benchmark safe-output config")
    start = text.find(start_marker)
    end = min(
        (idx for marker in end_markers if (idx := text.find(marker, start)) != -1),
        default=-1,
    )
    if end == -1:
        raise SystemExit("expected generated block end not found for benchmark safe-output config")

    block = text[start:end]
    patched_block = '''      - name: Write Safe Outputs Config
        env:
          ISSUE_NUMBER: ${{ inputs.issue_number }}
        run: |
          mkdir -p "${RUNNER_TEMP}/gh-aw/safeoutputs"
          mkdir -p /tmp/gh-aw/safeoutputs
          mkdir -p /tmp/gh-aw/mcp-logs/safeoutputs
          jq -n --arg target "${ISSUE_NUMBER}" '{
            add_comment: {
              hide_older_comments: true,
              max: 1,
              target: $target
            },
            create_report_incomplete_issue: {},
            mentions: {
              enabled: false
            },
            missing_data: {},
            missing_tool: {},
            noop: {
              max: 1,
              "report-as-issue": "false"
            },
            report_incomplete: {}
          }' > "${RUNNER_TEMP}/gh-aw/safeoutputs/config.json"
'''

    if '"target":"${{ inputs.issue_number }}"' in block:
        return text[:start] + patched_block + text[end:], True
    if 'jq -n --arg target "${ISSUE_NUMBER}"' in block:
        return text, False
    raise SystemExit("expected generated target interpolation not found for benchmark safe-output config")


def patch_benchmark_permissions(text):
    old = '''    permissions:
      contents: read
      discussions: write
      issues: write
      pull-requests: write
'''
    new = '''    permissions:
      contents: read
      issues: write
'''
    occurrences = text.count(old)
    if occurrences > 0:
        return text.replace(old, new), True
    if new in text:
        return text, False
    raise SystemExit("expected generated safe-output permissions block not found")


def patch_benchmark_handler_config(text):
    raw_config = (
        '          GH_AW_SAFE_OUTPUTS_HANDLER_CONFIG: "{\\"add_comment\\":'
        '{\\"hide_older_comments\\":true,\\"max\\":1,\\"target\\":\\"'
        '${{ inputs.issue_number }}'
        '\\"},\\"create_report_incomplete_issue\\":{},\\"missing_data\\":{},'
        '\\"missing_tool\\":{},\\"noop\\":{\\"max\\":1,\\"report-as-issue\\":'
        '\\"false\\"},\\"report_incomplete\\":{}}"\n'
    )
    issue_number_env = "          ISSUE_NUMBER: ${{ inputs.issue_number }}\n"

    changed = False
    if raw_config in text:
        text = text.replace(raw_config, issue_number_env)
        changed = True
    elif issue_number_env not in text:
        raise SystemExit("expected generated safe-output handler config not found")

    sentinel = "process.env.GH_AW_SAFE_OUTPUTS_HANDLER_CONFIG = JSON.stringify"
    if sentinel in text:
        return text, changed

    old = '''            const { setupGlobals } = require('${{ runner.temp }}/gh-aw/actions/setup_globals.cjs');
            setupGlobals(core, github, context, exec, io, getOctokit);
            const { main } = require('${{ runner.temp }}/gh-aw/actions/safe_output_handler_manager.cjs');
'''
    new = '''            const { setupGlobals } = require('${{ runner.temp }}/gh-aw/actions/setup_globals.cjs');
            setupGlobals(core, github, context, exec, io, getOctokit);
            process.env.GH_AW_SAFE_OUTPUTS_HANDLER_CONFIG = JSON.stringify({
              add_comment: {
                hide_older_comments: true,
                max: 1,
                target: process.env.ISSUE_NUMBER
              },
              create_report_incomplete_issue: {},
              missing_data: {},
              missing_tool: {},
              noop: {
                max: 1,
                "report-as-issue": "false"
              },
              report_incomplete: {}
            });
            const { main } = require('${{ runner.temp }}/gh-aw/actions/safe_output_handler_manager.cjs');
'''
    text, did_change = replace_required(text, old, new, "safe-output handler config")
    return text, changed or did_change


def patch_benchmark(text):
    changed = False
    for patcher in (
        patch_benchmark_safe_outputs_config,
        patch_benchmark_permissions,
        patch_benchmark_handler_config,
    ):
        text, did_change = patcher(text)
        changed = changed or did_change
    return text, changed


def patch_workflow(path):
    text = path.read_text()
    if path.name == "agentics-maintenance.yml":
        text, changed = patch_maintenance(text)
    elif path.name == "benchmark-regression-analysis.lock.yml":
        text, changed = patch_benchmark(text)
    else:
        raise SystemExit(f"unsupported gh-aw workflow patch target: {path}")

    normalized = text.rstrip() + "\n"
    if normalized != text:
        text = normalized
        changed = True

    if changed:
        path.write_text(text)
        print(f"patched {path}")
    else:
        print(f"{path} already patched")


for workflow in sys.argv[1:]:
    path = Path(workflow)
    if not path.is_file():
        raise SystemExit(f"workflow file not found: {path}")
    patch_workflow(path)
PY
