#!/usr/bin/env bash
# Test publication with a fake GitHub API; optionally use real signed ZIPs
# and native codesign on a Mac. Never creates or edits a GitHub release.
set -euo pipefail
repo_root=$(cd "$(dirname "$0")/../.." && pwd)
test_dir=$(mktemp -d)
trap 'rm -rf "$test_dir"' EXIT
mkdir -p "$test_dir/bin" "$test_dir/assets" "$test_dir/input"
cp "$repo_root/scripts/tests/apple-signing/gh" "$test_dir/bin/gh"
chmod +x "$test_dir/bin/gh"
arch=${2:-arm64}
if [[ $# -eq 2 ]]; then
  cp "$1/"kongctl_darwin_*.zip "$test_dir/assets/"
else
  cp "$repo_root/scripts/tests/apple-signing/codesign" "$test_dir/bin/"
  chmod +x "$test_dir/bin/codesign"
  printf '#!/bin/sh\nexit 0\n' > "$test_dir/bin/xattr"
  chmod +x "$test_dir/bin/xattr"
  export APPLE_TEAM_ID=ABCDE12345
  export APPLE_SIGNING_IDENTITY='Developer ID Application: Example Inc. (ABCDE12345)'
fi
export PATH="$test_dir/bin:$PATH"
printf '#!/bin/sh\nprintf "kongctl test\\n"\n' > "$test_dir/input/kongctl"
chmod +x "$test_dir/input/kongctl"
for os in darwin linux windows; do
  for build_arch in arm64 amd64; do
    archive="$test_dir/assets/kongctl_${os}_${build_arch}.zip"
    if [[ ! -f "$archive" ]]; then
      (cd "$test_dir/input" && zip -q "$archive" kongctl)
    fi
  done
done
(cd "$test_dir/assets" && shasum -a 256 ./*.zip | sed 's|  ./|  |' > checksums.txt)
export TEST_RELEASE_ASSETS="$test_dir/assets"
export TEST_RELEASE_JSON="$test_dir/release.json"
export TEST_PUBLICATION_LOG="$test_dir/publication.log"
export GITHUB_REPOSITORY=Kong/kongctl RELEASE_TAG=v9.9.9 RELEASE_BUILD_MODE=full
jq -n '{id: 100, tag_name: "v9.9.9", draft: true, prerelease: false,
  assets: (["checksums.txt", "kongctl_darwin_amd64.zip",
    "kongctl_darwin_arm64.zip", "kongctl_linux_amd64.zip",
    "kongctl_linux_arm64.zip", "kongctl_windows_amd64.zip",
    "kongctl_windows_arm64.zip"] | to_entries | map({id: (.key + 1),
      name: .value, state: "uploaded", size: 42, digest: "sha256:fixture",
      updated_at: "2026-09-05T00:00:00Z"}))}' > "$TEST_RELEASE_JSON"
cp "$TEST_RELEASE_JSON" "$test_dir/original.json"
verify() {
  bash "$repo_root/scripts/verify-apple-release.sh" "$arch" "$test_dir/receipts"
}
publish() {
  bash "$repo_root/scripts/publish-apple-release.sh" "$test_dir/receipts"
}
reject() {
  if "$@" > "$test_dir/rejected.log" 2>&1; then
    echo "Expected rejection: $*" >&2
    exit 1
  fi
}
# Reproduce the production failure: tag lookup cannot return a draft.
reject gh api "repos/$GITHUB_REPOSITORY/releases/tags/$RELEASE_TAG"
export TEST_DRAFT_ACCESS=read
reject verify
unset TEST_DRAFT_ACCESS
export TEST_DUPLICATE_RELEASE=true
reject verify
unset TEST_DUPLICATE_RELEASE
verify
other_arch=amd64
[[ "$arch" == amd64 ]] && other_arch=arm64
# Mock the second job's receipt to isolate the publication policy test.
cp "$test_dir/receipts/$arch.json" "$test_dir/receipts/$other_arch.json"
publish
grep -Fx 'PATCH repos/Kong/kongctl/releases/100 draft=false make_latest=false' "$TEST_PUBLICATION_LOG"
rm "$TEST_PUBLICATION_LOG"

# Draft recovery must verify again and can publish only matching receipts.
export RELEASE_BUILD_MODE=draft-recovery
verify
publish
rm "$TEST_PUBLICATION_LOG"
export RELEASE_BUILD_MODE=full

# Changing the release identity is rejected even with the same tag/assets.
jq '.id += 1' "$test_dir/original.json" > "$TEST_RELEASE_JSON"
reject publish
cp "$test_dir/original.json" "$TEST_RELEASE_JSON"

# Asset replacement, deletion and mismatched native receipts fail closed.
jq '.assets[0].id += 100' "$test_dir/original.json" > "$TEST_RELEASE_JSON"
reject publish
jq 'del(.assets[0])' "$test_dir/original.json" > "$TEST_RELEASE_JSON"
reject verify
cp "$test_dir/original.json" "$TEST_RELEASE_JSON"
jq '.assets[0].size += 1' "$test_dir/receipts/$arch.json" \
  > "$test_dir/receipts/$other_arch.json"
reject publish
cp "$test_dir/receipts/$arch.json" "$test_dir/receipts/$other_arch.json"

# A release cannot be published early or recovered as an unverified draft.
jq '.draft = false' "$test_dir/original.json" > "$TEST_RELEASE_JSON"
reject verify
export RELEASE_BUILD_MODE=draft-recovery
reject verify
export RELEASE_BUILD_MODE=recovery
verify
cp "$test_dir/receipts/$arch.json" "$test_dir/receipts/$other_arch.json"
publish
cp "$test_dir/original.json" "$TEST_RELEASE_JSON"
reject verify
export RELEASE_BUILD_MODE=full

# Missing manifest coverage and corrupted downloads cannot pass.
cp "$test_dir/assets/checksums.txt" "$test_dir/original-checksums.txt"
sed '/kongctl_darwin_arm64.zip/d' "$test_dir/original-checksums.txt" \
  > "$test_dir/assets/checksums.txt"
reject verify
cp "$test_dir/original-checksums.txt" "$test_dir/assets/checksums.txt"
printf 'tampered' >> "$test_dir/assets/kongctl_darwin_$arch.zip"
reject verify
[[ ! -e "$TEST_PUBLICATION_LOG" ]]
echo 'Apple release verification/publication policy tests passed'
