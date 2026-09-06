#!/usr/bin/env bash
# Merge Homebrew-generated metadata, without publishing or recompiling.
set -euo pipefail
[[ "${GITHUB_ACTIONS:-}" == true && $# -eq 2 ]]
script_dir=$(cd "$(dirname "$0")" && pwd)
directory=$(cd "$1" && pwd)
version=$2
ruby "$script_dir/validate-bottles.rb" "$directory" "$version"
export HOMEBREW_NO_AUTO_UPDATE=1 HOMEBREW_DEVELOPER=1
bash "$script_dir/init-tap.sh"
tap_dir=$(brew --repository kong/kongctl)
cp "$directory/kongctl-arm64_sequoia.rb" "$tap_dir/Formula/kongctl.rb"
git -C "$tap_dir" add Formula/kongctl.rb
git -C "$tap_dir" -c user.name='kongctl Release Bot' -c user.email='kongctl@konghq.com' \
  commit -m "Stage upstream kongctl $version formula"
brew trust --tap kong/kongctl
cd "$directory"
brew bottle --merge --write --no-commit --no-all-checks ./*.bottle.json
cp "$tap_dir/Formula/kongctl.rb" kongctl.rb
