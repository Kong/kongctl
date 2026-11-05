#!/usr/bin/env bash
# Compile and apply the repository's generator overlays as one operation.
set -euo pipefail

if [ -n "${GH_AW_BIN:-}" ]; then
  compiler=("$GH_AW_BIN")
else
  compiler=(gh aw)
fi
if [ "$("${compiler[@]}" version 2>&1)" != "gh aw version v0.86.2" ]; then
  echo "gh-aw v0.86.2 is required; review generator overlays before upgrading" >&2
  exit 1
fi

"${compiler[@]}" compile --no-check-update "$@"
bash scripts/patch-gh-aw-maintenance.sh
python3 scripts/patch-gh-aw-tokens.py
