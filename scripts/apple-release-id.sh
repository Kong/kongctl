#!/usr/bin/env bash
# Resolve exactly one release, including drafts. Draft access needs write scope.
set -euo pipefail
: "${GITHUB_REPOSITORY:?}"
: "${RELEASE_TAG:?}"
[[ "$RELEASE_TAG" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]
gh api --paginate --slurp "repos/$GITHUB_REPOSITORY/releases?per_page=100" |
  jq -er --arg tag "$RELEASE_TAG" '
    [ .[][] | select(.tag_name == $tag) ] |
    if length == 1 and (.[0].id | type == "number") and .[0].id > 0
    then .[0].id
    else error("Expected exactly one accessible release; check tag and draft permissions")
    end'
