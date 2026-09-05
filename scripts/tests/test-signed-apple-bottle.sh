#!/usr/bin/env bash
# Disposable GitHub macOS runner only. Does not access the production tap.
set -euo pipefail
if [[ "${GITHUB_ACTIONS:-}" != true || $# -ne 2 ]]; then
  echo 'Run only on an ephemeral Actions runner with ARCHIVE_DIR ARCH' >&2
  exit 2
fi
repo_root=$(cd "$(dirname "$0")/../.." && pwd)
archive_dir=$(cd "$1" && pwd)
arch=$2
[[ "$arch" =~ ^(arm64|amd64)$ ]]
export HOMEBREW_NO_AUTO_UPDATE=1 HOMEBREW_DEVELOPER=1
export KONGCTL_SIGNING_VALIDATION_URL="file://$archive_dir/kongctl_darwin_$arch.zip"
KONGCTL_SIGNING_VALIDATION_SHA256=$(shasum -a 256 "$archive_dir/kongctl_darwin_$arch.zip" | awk '{print $1}')
export KONGCTL_SIGNING_VALIDATION_SHA256

# Homebrew deliberately clears arbitrary environment variables while loading
# formulae. Render ordinary Ruby string literals before giving it the fixtures.
render_fixture() {
  ruby "$repo_root/scripts/tests/apple-signing/render-fixture.rb" "$1" > "$2"
}

tap=kong/signing-validation
formula="$tap/kongctl-signing-validation"
brew tap-new "$tap"
tap_dir=$(brew --repository "$tap")
render_fixture "$repo_root/scripts/tests/apple-signing/kongctl-signing-validation.rb" \
  "$tap_dir/Formula/kongctl-signing-validation.rb"
brew install --formula --build-bottle "$formula"
bash "$repo_root/scripts/verify-apple-binary.sh" "$(brew --prefix "$formula")/bin/kongctl"

test_dir=$(mktemp -d)
trap 'rm -rf "$test_dir"' EXIT
cd "$test_dir"
brew bottle --json "$formula"
shopt -s nullglob
bottles=(kongctl-signing-validation--*.bottle*.tar.gz)
[[ ${#bottles[@]} -eq 1 ]]
brew uninstall --formula "$formula"
brew install --formula "./${bottles[0]}"
brew info --json=v2 --formula "$formula" \
  | jq -e '.formulae[0].installed | length == 1 and .[0].poured_from_bottle == true'
installed="$(brew --prefix "$formula")/bin/kongctl"
bash "$repo_root/scripts/verify-apple-binary.sh" "$installed"
unzip -q "$archive_dir/kongctl_darwin_$arch.zip" -d reference
cmp reference/kongctl "$installed"
"$installed" version --full
echo 'Bottle creation and pouring preserved the signed executable byte-for-byte'

# Check cask installation independently of the formula link, with no
# quarantine-removal or ad-hoc re-signing workaround.
brew uninstall --formula "$formula"
mkdir -p "$tap_dir/Casks"
render_fixture "$repo_root/scripts/tests/apple-signing/kongctl-signing-validation-cask.rb" \
  "$tap_dir/Casks/kongctl-signing-validation.rb"
brew install --cask "$tap/kongctl-signing-validation"
installed="$(brew --prefix)/bin/kongctl"
cmp reference/kongctl "$installed"
bash "$repo_root/scripts/verify-apple-binary.sh" "$installed"
"$installed" version --full
brew uninstall --cask "$tap/kongctl-signing-validation"
echo 'Cask installation preserved the signed executable byte-for-byte'
