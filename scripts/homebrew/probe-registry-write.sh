#!/usr/bin/env bash
# Test push authorization without uploading bytes or publishing a manifest/tag.
# Cancel only the empty upload session created by this invocation.
set +x
set -euo pipefail
: "${GH_TOKEN:?}"
: "${GITHUB_ACTOR:?}"
work=$(mktemp -d)
upload_url=''
cleanup() {
  local result=$?
  if [[ -n "$upload_url" ]]; then
    if ! curl --fail --silent --show-error --max-time 60 -X DELETE \
      -H "Authorization: Bearer $token" "$upload_url" > /dev/null; then
      echo '::error::Could not cancel the empty upload session created by this probe' >&2
      result=1
    fi
  fi
  rm -rf "$work"
  exit "$result"
}
trap cleanup EXIT
token=$(curl --fail --silent --show-error --max-time 60 \
  --user "$GITHUB_ACTOR:$GH_TOKEN" \
  'https://ghcr.io/token?service=ghcr.io&scope=repository:kong/kongctl/kongctl:pull,push' | jq -er .token)
status=$(curl --silent --show-error --max-time 60 -X POST \
  -H "Authorization: Bearer $token" -H 'Content-Length: 0' \
  -D "$work/headers" -o "$work/body" -w '%{http_code}' \
  'https://ghcr.io/v2/kong/kongctl/kongctl/blobs/uploads/')
if [[ "$status" != 202 ]]; then
  echo "::error::kongctl Actions cannot start a bottle upload (HTTP $status). Grant this repository Write access under the package's Manage Actions access settings." >&2
  exit 1
fi
location=$(awk 'tolower($1) == "location:" {print $2}' "$work/headers" | tr -d '\r')
[[ "$location" != /* ]] || location="https://ghcr.io$location"
# GHCR returns singular /upload/ even though initiation uses /uploads/.
# Keep cancellation confined to this package's returned upload-session URL.
allowed='^https://ghcr\.io/v2/kong/kongctl/kongctl/blobs/uploads?/[[:alnum:]_=-]+(\?[^[:space:]#]*)?$'
if [[ ! "$location" =~ $allowed ]]; then
  # Describe structure only; an upload URL may contain a credential-like ID.
  case "$location" in
    https://ghcr.io/v2/kong/kongctl/kongctl/blobs/*)
      shape=${location#https://ghcr.io/v2/kong/kongctl/kongctl/blobs/}
      # Only punctuation remains; never log the session identifier or query.
      shape=$(printf '%s' "${shape%%\?*}" | sed -E 's/[[:alnum:]]+/X/g')
      echo "::notice::Redacted upload path shape: $shape" >&2
      echo '::error::Unexpected upload-session path shape on the expected GHCR package' >&2 ;;
    *) echo '::error::Unexpected upload cancellation host/package; refusing to follow it' >&2 ;;
  esac
  exit 1
fi
upload_url=$location
echo 'kongctl Actions has bottle upload access; cancelling the empty test session'
