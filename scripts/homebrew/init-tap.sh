#!/usr/bin/env bash
# Temporary local tap on an ephemeral runner; never pushes a repository.
set -euo pipefail
[[ "${GITHUB_ACTIONS:-}" == true ]]
git config --global user.name 'kongctl Release Bot'
git config --global user.email kongctl@konghq.com
brew tap-new kong/kongctl
