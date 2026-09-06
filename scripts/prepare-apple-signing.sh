#!/usr/bin/env bash
# Run only in a trusted signing job. Never echo credential values.
set +x
set -euo pipefail
for name in APPLE_NOTARY_API_PRIVATE_KEY APPLE_NOTARY_API_ISSUER_ID \
  APPLE_NOTARY_API_KEY_ID APPLE_SIGNING_CERTIFICATE_P12_BASE64 \
  APPLE_SIGNING_CERTIFICATE_PASSWORD APPLE_TEAM_ID APPLE_SIGNING_IDENTITY; do
  if [[ -z "${!name:-}" ]]; then
    echo "::error::Missing $name" >&2
    exit 1
  fi
done
: "${RUNNER_TEMP:?RUNNER_TEMP is required}"
: "${GITHUB_ENV:?GITHUB_ENV is required}"
umask 077
key_path="$RUNNER_TEMP/kongctl-notary-key.p8"
printf '%s\n' "$APPLE_NOTARY_API_PRIVATE_KEY" > "$key_path"
if ! openssl pkey -in "$key_path" -noout; then
  rm -f "$key_path"
  exit 1
fi
echo "APPLE_NOTARY_KEY_PATH=$key_path" >> "$GITHUB_ENV"
