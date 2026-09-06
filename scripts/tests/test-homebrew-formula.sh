#!/usr/bin/env bash
set -euo pipefail
repo_root=$(cd "$(dirname "$0")/../.." && pwd)
test_dir=$(mktemp -d)
trap 'rm -rf "$test_dir"' EXIT
for os in darwin linux; do
  for arch in arm64 amd64; do
    printf '%064d  kongctl_%s_%s.zip\n' 1 "$os" "$arch"
  done
done > "$test_dir/checksums.txt"
render() { ruby "$repo_root/scripts/homebrew/render-formula.rb" "$@"; }
render 1.2.3 "$test_dir/checksums.txt" > "$test_dir/kongctl.rb"
ruby -c "$test_dir/kongctl.rb"
grep -Fq 'releases/download/v1.2.3/kongctl_linux_arm64.zip' "$test_dir/kongctl.rb"
grep -Fq 'bin.install "kongctl"' "$test_dir/kongctl.rb"
if grep -Eq 'depends_on|system "go"|archive/refs/tags' "$test_dir/kongctl.rb"; then
  echo 'Formula unexpectedly requires a compiler/source archive' >&2; exit 1
fi
if grep -q '^  version ' "$test_dir/kongctl.rb"; then
  echo 'Stable version is redundant with its release URL' >&2; exit 1
fi
render 1.2.3-apple-validation-abcdef "$test_dir/checksums.txt" > "$test_dir/snapshot.rb"
grep -Fqx '  version "1.2.3-apple-validation-abcdef"' "$test_dir/snapshot.rb"
reject() {
  if render "$@" > /dev/null 2>&1; then
    echo 'Accepted invalid formula input' >&2; exit 1
  fi
}
reject '1.2.3"; system("evil")' "$test_dir/checksums.txt"
reject 1.2.3 "$test_dir/checksums.txt" https://example.com
cp "$test_dir/checksums.txt" "$test_dir/duplicate.txt"
head -1 "$test_dir/checksums.txt" >> "$test_dir/duplicate.txt"
reject 1.2.3 "$test_dir/duplicate.txt"
head -3 "$test_dir/checksums.txt" > "$test_dir/missing.txt"
reject 1.2.3 "$test_dir/missing.txt"
echo 'Prebuilt Homebrew formula tests passed'
