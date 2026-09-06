#!/usr/bin/env bash
set -euo pipefail
repo_root=$(cd "$(dirname "$0")/../.." && pwd)
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
for os in darwin linux; do
  for arch in arm64 amd64; do
    printf '%064d  kongctl_%s_%s.zip\n' 1 "$os" "$arch"
  done
done > "$work/checksums.txt"
for tag in arm64_sequoia sequoia x86_64_linux; do
  ruby "$repo_root/scripts/homebrew/render-formula.rb" 1.2.3 "$work/checksums.txt" > "$work/kongctl-$tag.rb"
  file="kongctl--1.2.3.$tag.bottle.tar.gz"
  printf 'synthetic bottle for %s\n' "$tag" > "$work/$file"
  digest=$(shasum -a 256 "$work/$file" | awk '{print $1}')
  jq -n --arg tag "$tag" --arg filename "$file" --arg sha "$digest" \
    '{"kong/kongctl/kongctl": {formula: {pkg_version: "1.2.3"},
      bottle: {root_url: "https://ghcr.io/v2/kong/kongctl", rebuild: 0,
        cellar: "any_skip_relocation",
        tags: {($tag): {local_filename: $filename, sha256: $sha}}}}}' \
    > "$work/kongctl--1.2.3.$tag.bottle.json"
done
validate() { ruby "$repo_root/scripts/homebrew/validate-bottles.rb" "$work" 1.2.3; }
reject() {
  if validate > /dev/null 2>&1; then echo 'Accepted invalid bottle set' >&2; exit 1; fi
}
validate
metadata="$work/kongctl--1.2.3.arm64_sequoia.bottle.json"
cp "$metadata" "$work/original.json"
for expression in \
  '.[] .formula.pkg_version = "9.9.9"' \
  '.[] .bottle.rebuild = 1' \
  '.[] .bottle.cellar = "/opt/homebrew/Cellar"' \
  '.[] .bottle.root_url = "https://example.com"' \
  '.[] .bottle.tags.arm64_sequoia.local_filename = "../escape.tar.gz"' \
  '.[] .bottle.tags = {sequoia: .[] .bottle.tags.arm64_sequoia}'; do
  jq "$expression" "$work/original.json" > "$metadata"
  reject
done
cp "$work/original.json" "$metadata"
printf '\n# different formula\n' >> "$work/kongctl-arm64_sequoia.rb"
reject
cp "$work/kongctl-sequoia.rb" "$work/kongctl-arm64_sequoia.rb"
printf 'tampered' >> "$work/kongctl--1.2.3.arm64_sequoia.bottle.tar.gz"
reject
echo 'Homebrew bottle metadata rejection tests passed'
