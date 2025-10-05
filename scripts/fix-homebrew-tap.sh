#!/usr/bin/env bash
set -euo pipefail

REPO_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")"/.. && pwd)
DEFAULT_TAP_PATH="${REPO_DIR}/dist/homebrew/homebrew-kongctl"
TAP_PATH="${1:-${DEFAULT_TAP_PATH}}"

if ! command -v brew >/dev/null 2>&1; then
  echo "brew is not installed on this runner; cannot fix tap style" >&2
  exit 1
fi

if [[ ! -d "${TAP_PATH}" ]]; then
  echo "Tap directory not found: ${TAP_PATH}" >&2
  exit 1
fi

pushd "${TAP_PATH}" >/dev/null

files=()
if [[ -f "Casks/kongctl.rb" ]]; then
  files+=("Casks/kongctl.rb")
fi
if [[ -f "kongctl.rb" ]]; then
  files+=("kongctl.rb")
fi

if [[ ${#files[@]} -eq 0 ]]; then
  echo "No Homebrew files found to fix in ${TAP_PATH}" >&2
  popd >/dev/null
  exit 0
fi

brew style --fix "${files[@]}"

popd >/dev/null
