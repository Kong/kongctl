#!/usr/bin/env bash
# Consume receipts from both native verification jobs in this workflow run.
set -euo pipefail
if [[ $# -ne 1 ]]; then
  echo 'usage: publish-apple-release.sh RECEIPT_DIRECTORY' >&2
  exit 2
fi
: "${GITHUB_REPOSITORY:?}"
: "${RELEASE_TAG:?}"
: "${RELEASE_BUILD_MODE:?}"
[[ "$RELEASE_TAG" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]
script_dir=$(cd "$(dirname "$0")" && pwd)
cmp "$1/arm64.json" "$1/amd64.json"
current=$(gh api "repos/$GITHUB_REPOSITORY/releases/tags/$RELEASE_TAG" |
  jq -Se --arg tag "$RELEASE_TAG" -f "$script_dir/apple-release-snapshot.jq")
expected=$(jq -Se . "$1/arm64.json")
if [[ "$current" != "$expected" ]]; then
  echo '::error::Release assets changed after native verification; re-run the gates' >&2
  exit 1
fi
case "$RELEASE_BUILD_MODE" in
  full)
    jq -e '.draft == true' <<< "$current" > /dev/null
    gh release edit "$RELEASE_TAG" --repo "$GITHUB_REPOSITORY" \
      --draft=false --latest=false
    ;;
  recovery)
    jq -e '.draft == false' <<< "$current" > /dev/null
    echo 'Recovery: verified existing published assets without replacing them'
    ;;
  *) echo 'Unexpected release mode' >&2; exit 1 ;;
esac
