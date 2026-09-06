#!/usr/bin/env bash
# Offline HTTP doubles: no credentials or registry requests.
set -euo pipefail
root=$(cd "$(dirname "$0")/../.." && pwd)
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
export GH_TOKEN=fixture GITHUB_ACTOR=fixture
export PROBE_LOG="$work/cancelled" PROBE_STATUS=202 PROBE_CANCEL_EXIT=0
curl() {
  local method=GET headers='' url=''
  while (( $# )); do
    case "$1" in
      -X) method=$2; shift 2 ;;
      -D) headers=$2; shift 2 ;;
      --user|-H|-o|-w|--max-time) shift 2 ;;
      https://*) url=$1; shift ;;
      *) shift ;;
    esac
  done
  case "$method" in
    GET) printf '{"token":"fixture"}\n' ;;
    POST)
      printf 'HTTP/2 %s\r\nlocation: %s\r\n' "$PROBE_STATUS" "$PROBE_LOCATION" > "$headers"
      printf '%s' "$PROBE_STATUS" ;;
    DELETE) printf '%s\n' "$url" >> "$PROBE_LOG"; return "$PROBE_CANCEL_EXIT" ;;
    *) return 99 ;;
  esac
}
export -f curl
export PROBE_LOCATION
for path in upload uploads; do
  for prefix in '' https://ghcr.io; do
    PROBE_LOCATION="$prefix/v2/kong/kongctl/kongctl/blobs/$path/01234567-89ab-cdef-0123-456789abcdef"
    : > "$PROBE_LOG"
    bash "$root/scripts/homebrew/probe-registry-write.sh"
    [[ $(wc -l < "$PROBE_LOG") -eq 1 ]]
    [[ $(< "$PROBE_LOG") == "https://ghcr.io/v2/kong/kongctl/kongctl/blobs/$path/01234567-89ab-cdef-0123-456789abcdef" ]]
  done
done
for PROBE_LOCATION in \
  'https://example.com/v2/kong/kongctl/kongctl/blobs/upload/abcd' \
  '/v2/kong/other/blobs/upload/abcd' \
  '/v2/kong/kongctl/kongctl/blobs/upload/../manifests/latest'; do
  : > "$PROBE_LOG"
  if bash "$root/scripts/homebrew/probe-registry-write.sh"; then exit 1; fi
  [[ ! -s "$PROBE_LOG" ]]
done
PROBE_LOCATION='/v2/kong/kongctl/kongctl/blobs/upload/abcd'
PROBE_CANCEL_EXIT=22
if bash "$root/scripts/homebrew/probe-registry-write.sh"; then exit 1; fi
PROBE_STATUS=403
: > "$PROBE_LOG"
if bash "$root/scripts/homebrew/probe-registry-write.sh"; then exit 1; fi
[[ ! -s "$PROBE_LOG" ]]
echo 'Registry authorization and safe cancellation tests passed'
