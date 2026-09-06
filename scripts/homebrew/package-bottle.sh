#!/usr/bin/env bash
# Package the upstream executable, never compile it again. Ephemeral CI only.
set -euo pipefail
if [[ "${GITHUB_ACTIONS:-}" != true || $# -ne 3 ]]; then
  echo 'usage (Actions only): package-bottle.sh ASSETS VERSION OUTPUT' >&2
  exit 2
fi
script_dir=$(cd "$(dirname "$0")" && pwd)
assets=$(cd "$1" && pwd)
version=$2
mkdir -p "$3"
output=$(cd "$3" && pwd)
export HOMEBREW_NO_AUTO_UPDATE=1 HOMEBREW_DEVELOPER=1
# Prove neither packaging nor pouring needs the Homebrew Go toolchain.
while IFS= read -r formula; do
  case "$formula" in
    go|go@*) brew uninstall --formula --force --ignore-dependencies "$formula" ;;
  esac
done < <(brew list --formula)
case "$(uname -s)/$(uname -m)" in
  Darwin/arm64) os=darwin; arch=arm64; tag=arm64_sequoia ;;
  Darwin/x86_64) os=darwin; arch=amd64; tag=sequoia ;;
  Linux/x86_64) os=linux; arch=amd64; tag=x86_64_linux ;;
  *) echo 'Unsupported bottle builder' >&2; exit 1 ;;
esac
(cd "$assets" && shasum -a 256 --check checksums.txt)

# Do not start with a user's installed tap/kongctl or modify their installation.
if brew list --formula kongctl > /dev/null 2>&1 || brew tap | grep -Fxq kong/kongctl; then
  echo 'Expected a fresh runner without kongctl or the production tap' >&2; exit 1
fi
bash "$script_dir/init-tap.sh"
tap_dir=$(brew --repository kong/kongctl)
ruby "$script_dir/render-formula.rb" "$version" "$assets/checksums.txt" \
  > "$tap_dir/Formula/kongctl.rb"
git -C "$tap_dir" add Formula/kongctl.rb
git -C "$tap_dir" -c user.name='kongctl Release Bot' -c user.email='kongctl@konghq.com' \
  commit -m "Package upstream kongctl $version"
brew trust --tap kong/kongctl
# Seed the cache with the already verified archive; snapshots need no release.
# All platforms still package the identical, official-URL formula recipe.
cache_file=$(brew --cache --build-from-source --formula kong/kongctl/kongctl)
[[ "$cache_file" == "$(brew --cache)/downloads/"* ]]
mkdir -p "$(dirname "$cache_file")"
cp "$assets/kongctl_${os}_$arch.zip" "$cache_file"
brew install --formula --build-bottle kong/kongctl/kongctl

reference=$(mktemp -d)
trap 'rm -rf "$reference"' EXIT
unzip -q "$assets/kongctl_${os}_$arch.zip" -d "$reference"
verify_binary() {
  local binary="$1"
  cmp "$reference/kongctl" "$binary"
  if [[ "$os" == darwin ]]; then
    bash "$script_dir/../verify-apple-binary.sh" "$binary"
  fi
  "$binary" version --full
}
verify_binary "$(brew --prefix kong/kongctl/kongctl)/bin/kongctl"
cd "$output"
brew bottle --json --root-url=https://ghcr.io/v2/kong/kongctl kong/kongctl/kongctl
shopt -s nullglob
bottles=(kongctl--*.bottle*.tar.gz)
metadata=(kongctl--*.bottle.json)
[[ ${#bottles[@]} -eq 1 && ${#metadata[@]} -eq 1 ]]
jq -e --arg tag "$tag" '.[] | (.bottle.tags | keys) == [$tag] and .bottle.cellar == "any_skip_relocation"' \
  "${metadata[0]}" > /dev/null
brew uninstall --formula kong/kongctl/kongctl
brew install --formula "./${bottles[0]}"
brew info --json=v2 --formula kong/kongctl/kongctl |
  jq -e '.formulae[0].installed | length == 1 and .[0].poured_from_bottle == true'
verify_binary "$(brew --prefix kong/kongctl/kongctl)/bin/kongctl"
brew test kong/kongctl/kongctl
if brew list --formula | grep -E '^go(@.*)?$'; then
  echo 'Bottle packaging unexpectedly installed Homebrew Go' >&2; exit 1
fi
cp "$tap_dir/Formula/kongctl.rb" "kongctl-$tag.rb"
# Registry provenance points to the actual upstream packaging recipe, not the
# throwaway local tap commit (which has never been pushed to GitHub).
jq --arg revision "$GITHUB_SHA" '.[] .formula |=
  (.tap_git_revision = $revision | .tap_git_path = "scripts/homebrew/kongctl.rb.template")' \
  "${metadata[0]}" > metadata.tmp
mv metadata.tmp "${metadata[0]}"
echo "Verified $tag bottle preserves the upstream executable byte-for-byte"
