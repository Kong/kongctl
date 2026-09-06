#!/usr/bin/env bash
# Exit 0: exact public bottles; 3: manifest absent; 1: conflict/other failure.
set -euo pipefail
[[ $# -eq 2 ]]
directory=$1
version=$2
[[ "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
token=$(curl --fail --silent --show-error --retry 3 --max-time 60 \
  'https://ghcr.io/token?service=ghcr.io&scope=repository:kong/kongctl/kongctl:pull' | jq -er .token)
status=$(curl --silent --show-error --retry 3 --max-time 60 \
  -H "Authorization: Bearer $token" -H 'Accept: application/vnd.oci.image.index.v1+json' \
  -w '%{http_code}' -o "$work/manifest.json" \
  "https://ghcr.io/v2/kong/kongctl/kongctl/manifests/$version")
[[ "$status" != 404 ]] || exit 3
[[ "$status" == 200 ]] || { echo "Registry returned $status" >&2; exit 1; }
expected=$(jq -s -c '[.[] | .[] .bottle.tags[] .sha256] | sort' "$directory/"*.bottle.json)
actual=$(jq -ec '[.manifests[].annotations["sh.brew.bottle.digest"]] | sort' "$work/manifest.json")
[[ "$expected" == "$actual" ]] || {
  echo '::error::Existing registry version differs; refusing to replace published bottles' >&2; exit 1;
}
for digest in $(jq -r '.[] .bottle.tags[] .sha256' "$directory/"*.bottle.json); do
  [[ "$digest" =~ ^[0-9a-f]{64}$ ]]
  actual=$(curl --fail --silent --show-error --location --retry 3 --max-time 180 \
    -H "Authorization: Bearer $token" \
    "https://ghcr.io/v2/kong/kongctl/kongctl/blobs/sha256:$digest" | shasum -a 256)
  [[ "${actual%% *}" == "$digest" ]]
done
echo 'All three bottles are publicly downloadable with the expected checksums'
