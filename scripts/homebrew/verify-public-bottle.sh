#!/usr/bin/env bash
# Fresh, unprivileged installation from the public registry and release URLs.
set -euo pipefail
[[ "${GITHUB_ACTIONS:-}" == true && $# -eq 1 ]]
script_dir=$(cd "$(dirname "$0")" && pwd)
recipe=$(cd "$(dirname "$1")" && pwd)/$(basename "$1")
export HOMEBREW_NO_AUTO_UPDATE=1 HOMEBREW_DEVELOPER=1
while IFS= read -r formula; do
  case "$formula" in
    go|go@*) brew uninstall --formula --force --ignore-dependencies "$formula" ;;
  esac
done < <(brew list --formula)
bash "$script_dir/init-tap.sh"
tap_dir=$(brew --repository kong/kongctl)
cp "$recipe" "$tap_dir/Formula/kongctl.rb"
brew trust --tap kong/kongctl
unset HOMEBREW_GITHUB_API_TOKEN HOMEBREW_GITHUB_PACKAGES_TOKEN
unset HOMEBREW_DOCKER_REGISTRY_TOKEN GITHUB_TOKEN GH_TOKEN
brew install --formula --force-bottle kong/kongctl/kongctl
info=$(brew info --json=v2 --formula kong/kongctl/kongctl)
jq -e '.formulae[0].installed | length == 1 and .[0].poured_from_bottle == true' <<< "$info"
binary="$(brew --prefix kong/kongctl/kongctl)/bin/kongctl"
if [[ "$(uname -s)" == Darwin ]]; then
  bash "$script_dir/../verify-apple-binary.sh" "$binary"
fi
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
url=$(jq -er '.formulae[0].urls.stable.url' <<< "$info")
checksum=$(jq -er '.formulae[0].urls.stable.checksum' <<< "$info")
[[ "$url" == https://github.com/Kong/kongctl/releases/download/v*/kongctl_*.zip ]]
[[ "$checksum" =~ ^[0-9a-f]{64}$ ]]
curl --fail --location --retry 3 --max-time 180 --output "$work/archive.zip" "$url"
printf '%s  %s\n' "$checksum" "$work/archive.zip" | shasum -a 256 --check
unzip -q "$work/archive.zip" -d "$work/reference"
cmp "$work/reference/kongctl" "$binary"
brew test kong/kongctl/kongctl
"$binary" version --full
if brew list --formula | grep -E '^go(@.*)?$'; then
  echo 'Public bottle installation unexpectedly installed Go' >&2; exit 1
fi
echo 'Public bottle matches upstream release bytes and needs no Homebrew Go'
