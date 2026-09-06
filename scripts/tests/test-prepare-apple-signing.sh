#!/usr/bin/env bash
set -euo pipefail
repo_root=$(cd "$(dirname "$0")/../.." && pwd)
test_dir=$(mktemp -d)
trap 'rm -rf "$test_dir"' EXIT
export RUNNER_TEMP="$test_dir"
export GITHUB_ENV="$test_dir/github-env"
export APPLE_NOTARY_API_ISSUER_ID=fixture APPLE_NOTARY_API_KEY_ID=fixture
export APPLE_SIGNING_CERTIFICATE_P12_BASE64=fixture
export APPLE_SIGNING_CERTIFICATE_PASSWORD=fixture
export APPLE_TEAM_ID=ABCDE12345 APPLE_SIGNING_IDENTITY=fixture
# Ephemeral synthetic key, not an Apple credential.
openssl genpkey -algorithm EC -pkeyopt ec_paramgen_curve:P-256 \
  -out "$test_dir/fixture.p8"
export APPLE_NOTARY_API_PRIVATE_KEY
APPLE_NOTARY_API_PRIVATE_KEY=$(< "$test_dir/fixture.p8")
bash "$repo_root/scripts/prepare-apple-signing.sh"
cmp "$test_dir/fixture.p8" "$test_dir/kongctl-notary-key.p8"
if [[ "$(uname -s)" == Darwin ]]; then
  mode=$(stat -f '%Lp' "$test_dir/kongctl-notary-key.p8")
else
  mode=$(stat -c '%a' "$test_dir/kongctl-notary-key.p8")
fi
[[ "$mode" == 600 ]]
grep -Fx "APPLE_NOTARY_KEY_PATH=$test_dir/kongctl-notary-key.p8" "$GITHUB_ENV"
for name in APPLE_NOTARY_API_PRIVATE_KEY APPLE_NOTARY_API_ISSUER_ID \
  APPLE_NOTARY_API_KEY_ID APPLE_SIGNING_CERTIFICATE_P12_BASE64 \
  APPLE_SIGNING_CERTIFICATE_PASSWORD APPLE_TEAM_ID APPLE_SIGNING_IDENTITY; do
  if env "$name=" bash "$repo_root/scripts/prepare-apple-signing.sh" \
    > "$test_dir/output" 2>&1; then
    echo "Accepted missing $name" >&2
    exit 1
  fi
  grep -Fx "::error::Missing $name" "$test_dir/output"
done
if APPLE_NOTARY_API_PRIVATE_KEY=invalid \
  bash "$repo_root/scripts/prepare-apple-signing.sh" > "$test_dir/output" 2>&1; then
  echo 'Accepted invalid API private key' >&2
  exit 1
fi
[[ ! -e "$test_dir/kongctl-notary-key.p8" ]]
echo 'Apple credential preparation tests passed'
