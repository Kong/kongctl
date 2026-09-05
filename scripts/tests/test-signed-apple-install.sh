#!/usr/bin/env bash
# Uses signed validation archives, never the current public release.
set -euo pipefail
if [[ $# -ne 2 || ! -d "$1" || ! "$2" =~ ^(arm64|amd64)$ ]]; then
  echo "usage: bash scripts/tests/test-signed-apple-install.sh ARCHIVE_DIR ARCH" >&2
  exit 2
fi
repo_root=$(cd "$(dirname "$0")/../.." && pwd)
archive_dir=$(cd "$1" && pwd)
arch=$2
test_dir=$(mktemp -d)
trap 'rm -rf "$test_dir"' EXIT

checksum=$(shasum -a 256 "$archive_dir/checksums.txt" | awk '{print $1}')
jq -n --arg digest "sha256:$checksum" \
  '{tag_name:"v0.0.0-apple-validation",assets:[{name:"checksums.txt",digest:$digest}]}' \
  > "$test_dir/release.json"

# These existing installer fixture options avoid publishing a test release.
# Exercise the same script that is delivered through curl | sh.
KONGCTL_ALLOW_FILE_URLS=1 \
  KONGCTL_RELEASE_BASE_URL="file://$archive_dir" \
  KONGCTL_RELEASE_METADATA_URL="file://$test_dir/release.json" \
  KONGCTL_INSTALL_ART=never \
  sh "$repo_root/scripts/install.sh" --os darwin --arch "$arch" \
    --install-dir "$test_dir/installed"

unzip -q "$archive_dir/kongctl_darwin_$arch.zip" -d "$test_dir/extracted"
cmp "$test_dir/extracted/kongctl" "$test_dir/installed/kongctl"
bash "$repo_root/scripts/verify-apple-binary.sh" "$test_dir/installed/kongctl"
"$test_dir/installed/kongctl" version --full
echo 'Installer preserved the signed executable byte-for-byte'
