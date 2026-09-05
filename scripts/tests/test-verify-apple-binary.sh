#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "$0")/../.." && pwd)
test_dir=$(mktemp -d)
trap 'rm -rf "$test_dir"' EXIT
cp "$repo_root/scripts/tests/apple-signing/codesign" "$test_dir/codesign"
chmod +x "$test_dir/codesign"
export PATH="$test_dir:$PATH"
export APPLE_TEAM_ID=ABCDE12345
export APPLE_SIGNING_IDENTITY='Developer ID Application: Example Inc. (ABCDE12345)'
verifier="$repo_root/scripts/verify-apple-binary.sh"
binary="$test_dir/codesign"

bash "$verifier" "$binary" > /dev/null
if TEST_SIGNATURE_EXIT=1 bash "$verifier" "$binary" > /dev/null 2>&1; then
  echo 'accepted invalid signature' >&2
  exit 1
fi
if TEST_NOTARY_EXIT=1 bash "$verifier" "$binary" > /dev/null 2>&1; then
  echo 'accepted unapproved notarization' >&2
  exit 1
fi
if TEST_TEAM_ID=WRONG12345 bash "$verifier" "$binary" > /dev/null 2>&1; then
  echo 'accepted wrong team' >&2
  exit 1
fi
if APPLE_SIGNING_IDENTITY=Wrong bash "$verifier" "$binary" > /dev/null 2>&1; then
  echo 'accepted wrong signing identity' >&2
  exit 1
fi
if APPLE_TEAM_ID='' bash "$verifier" "$binary" > /dev/null 2>&1; then
  echo 'accepted missing team configuration' >&2
  exit 1
fi
echo 'Apple verification gate tests passed'
