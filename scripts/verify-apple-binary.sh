#!/usr/bin/env bash
# Verify the executable itself, independent of its delivery container.
set -euo pipefail

if [[ $# -ne 1 || ! -f "$1" ]]; then
  echo "usage: bash scripts/verify-apple-binary.sh BINARY" >&2
  exit 2
fi
: "${APPLE_TEAM_ID:?APPLE_TEAM_ID is required}"
: "${APPLE_SIGNING_IDENTITY:?APPLE_SIGNING_IDENTITY is required}"
if [[ ! "$APPLE_TEAM_ID" =~ ^[A-Z0-9]{10}$ ]]; then
  echo "invalid Apple team ID" >&2
  exit 1
fi

binary=$1
codesign --verify --strict --verbose=2 "$binary"
details=$(codesign --display --verbose=4 "$binary" 2>&1)
printf '%s\n' "$details"
grep -Fx "TeamIdentifier=$APPLE_TEAM_ID" <<< "$details"
grep -Fx "Authority=$APPLE_SIGNING_IDENTITY" <<< "$details"
grep -E '^CodeDirectory .*flags=.*runtime' <<< "$details"
grep -E '^Timestamp=.' <<< "$details"

# Unlike spctl's application assessment, this checks a standalone executable.
# Online lookup is required: a CLI/ZIP cannot have a stapled ticket.
# Fail closed even if GoReleaser returned success with notarization pending.
codesign --verify --strict --verbose=2 --check-notarization \
  --test-requirement='notarized' "$binary"
