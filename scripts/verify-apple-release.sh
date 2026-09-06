#!/usr/bin/env bash
# Read-only gate, on a fresh native Mac without Apple private credentials.
set -euo pipefail
if [[ $# -ne 2 || ! "$1" =~ ^(arm64|amd64)$ ]]; then
  echo 'usage: verify-apple-release.sh ARCH RECEIPT_DIRECTORY' >&2
  exit 2
fi
: "${GITHUB_REPOSITORY:?}"
: "${RELEASE_TAG:?}"
: "${RELEASE_BUILD_MODE:?}"
[[ "$RELEASE_TAG" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]
arch=$1
receipt_dir=$2
mkdir -p "$receipt_dir"
receipt_dir=$(cd "$receipt_dir" && pwd)
script_dir=$(cd "$(dirname "$0")" && pwd)
verify_dir=$(mktemp -d)
trap 'rm -rf "$verify_dir"' EXIT
endpoint="repos/$GITHUB_REPOSITORY/releases/tags/$RELEASE_TAG"
snapshot() {
  gh api "$endpoint" | jq -Se --arg tag "$RELEASE_TAG" \
    -f "$script_dir/apple-release-snapshot.jq"
}
snapshot > "$verify_dir/before.json"
case "$RELEASE_BUILD_MODE" in
  full) jq -e '.draft == true' "$verify_dir/before.json" > /dev/null ;;
  recovery) jq -e '.draft == false' "$verify_dir/before.json" > /dev/null ;;
  *) echo 'Unexpected release mode' >&2; exit 1 ;;
esac
mkdir "$verify_dir/assets"
gh release download "$RELEASE_TAG" --repo "$GITHUB_REPOSITORY" \
  --dir "$verify_dir/assets"
cd "$verify_dir/assets"
shasum -a 256 --check checksums.txt
# Explicitly require both Darwin archives to be covered by the manifest.
for darwin_arch in arm64 amd64; do
  checksum=$(shasum -a 256 "kongctl_darwin_$darwin_arch.zip")
  grep -Fx "$checksum" checksums.txt > /dev/null
done
unzip -q "kongctl_darwin_$arch.zip" -d unpacked
binary="$PWD/unpacked/kongctl"
bash "$script_dir/verify-apple-binary.sh" "$binary"
xattr -w com.apple.quarantine '0083;00000000;KongReleaseVerification;' "$binary"
"$binary" version --full
# Exercise the real installer against the very same release assets.
bash "$script_dir/tests/test-signed-apple-install.sh" "$PWD" "$arch"
snapshot > "$verify_dir/after.json"
cmp "$verify_dir/before.json" "$verify_dir/after.json"
cp "$verify_dir/after.json" "$receipt_dir/$arch.json"
echo "Verified $RELEASE_TAG on $arch; release assets are unchanged"
