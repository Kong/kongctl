#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 4 ]]; then
  echo "usage: $0 FORMULA_PATH VERSION COMMIT BUILD_DATE" >&2
  exit 2
fi

formula_path=$1
version=${2#v}
commit=$3
build_date=$4

if [[ ! -f "$formula_path" ]]; then
  echo "formula not found: $formula_path" >&2
  exit 1
fi

if [[ ! "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "invalid stable version: $version" >&2
  exit 1
fi

if [[ ! "$commit" =~ ^[0-9a-f]{8,40}$ ]]; then
  echo "invalid commit: $commit" >&2
  exit 1
fi

if [[ ! "$build_date" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(\.[0-9]+)?Z$ ]]; then
  echo "invalid UTC build date: $build_date" >&2
  exit 1
fi

source_url="https://github.com/Kong/kongctl/archive/refs/tags/v${version}.tar.gz"
temp_dir=$(go env GOTMPDIR)
if [[ -z "$temp_dir" ]]; then
  temp_dir=$(go env GOCACHE)
fi
if [[ -z "$temp_dir" ]]; then
  echo "go did not provide a temporary or cache directory" >&2
  exit 1
fi
mkdir -p "$temp_dir/kongctl-homebrew"
source_archive=$(mktemp "$temp_dir/kongctl-homebrew/source.XXXXXX")
trap 'rm -f "$source_archive"' EXIT

curl --fail --location --retry 3 --silent --show-error \
  "$source_url" \
  --output "$source_archive"

if command -v sha256sum >/dev/null 2>&1; then
  source_sha=$(sha256sum "$source_archive" | awk '{print $1}')
else
  source_sha=$(shasum -a 256 "$source_archive" | awk '{print $1}')
fi

python3 - "$formula_path" "$source_url" "$source_sha" "$commit" "$build_date" <<'PY'
import os
import re
import sys
from pathlib import Path

formula_path = Path(sys.argv[1])
source_url, source_sha, commit, build_date = sys.argv[2:]
content = formula_path.read_text()

replacements = (
    (r'^  url "[^"]+"$', f'  url "{source_url}"'),
    (r'^  sha256 "[0-9a-f]{64}"$', f'  sha256 "{source_sha}"'),
    (r'^      -X main\.commit=[0-9a-f]+$', f'      -X main.commit={commit}'),
    (r'^      -X main\.date=\S+$', f'      -X main.date={build_date}'),
    (r'^    assert_match "[0-9a-f]+", output$', f'    assert_match "{commit}", output'),
)

for pattern, replacement in replacements:
    content, count = re.subn(pattern, replacement, content, flags=re.MULTILINE)
    if count != 1:
        raise SystemExit(f"expected one formula match for {pattern!r}, found {count}")

temporary_path = formula_path.with_suffix(".rb.tmp")
temporary_path.write_text(content)
os.replace(temporary_path, formula_path)
PY

echo "Updated $formula_path to kongctl $version ($commit, $build_date)"
