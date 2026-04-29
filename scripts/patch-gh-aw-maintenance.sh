#!/usr/bin/env bash
set -euo pipefail

# Temporary post-compile patch for gh-aw v0.71.1 generated maintenance workflows.
# Remove this once upstream gh-aw stops writing workflow inputs directly into shell commands.
workflow="${1:-.github/workflows/agentics-maintenance.yml}"

python3 - "$workflow" <<'PY'
from pathlib import Path
import sys

path = Path(sys.argv[1])
if not path.is_file():
    raise SystemExit(f"workflow file not found: {path}")

text = path.read_text()
expr_open = "${{"
expr_close = "}}"
operation_input = f"{expr_open} inputs.operation {expr_close}"
run_url_input = f"{expr_open} inputs.run_url {expr_close}"
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
    if old in text:
        text = text.replace(old, new)
        changed = True
    elif new not in text:
        raise SystemExit(f"expected generated block not found for {label}")

if changed:
    path.write_text(text)
    print(f"patched {path}")
else:
    print(f"{path} already patched")
PY
