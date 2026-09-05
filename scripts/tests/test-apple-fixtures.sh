#!/usr/bin/env bash
set -euo pipefail
repo_root=$(cd "$(dirname "$0")/../.." && pwd)
export KONGCTL_SIGNING_VALIDATION_URL='file:///example/kongctl.zip'
export KONGCTL_SIGNING_VALIDATION_SHA256=0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef
for name in kongctl-signing-validation.rb kongctl-signing-validation-cask.rb; do
  rendered=$(ruby "$repo_root/scripts/tests/apple-signing/render-fixture.rb" \
    "$repo_root/scripts/tests/apple-signing/$name")
  grep -Fq "$KONGCTL_SIGNING_VALIDATION_URL" <<< "$rendered"
  grep -Fq "$KONGCTL_SIGNING_VALIDATION_SHA256" <<< "$rendered"
  if grep -Fq 'ENV.fetch' <<< "$rendered"; then
    echo 'unresolved fixture variable' >&2
    exit 1
  fi
  ruby -c <<< "$rendered"
done
echo 'Homebrew signing fixture rendering tests passed'
