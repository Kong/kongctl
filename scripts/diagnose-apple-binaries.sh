#!/usr/bin/env bash
# Diagnostic only: never execute, sign, or modify the downloaded binaries.
set -euo pipefail
[[ $# -eq 2 ]]
script_dir=$(cd "$(dirname "$0")" && pwd)
assets=$(cd "$1" && pwd)
mkdir -p "$2"
output=$(cd "$2" && pwd)
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
sw_vers > "$output/runner.txt"
uname -m >> "$output/runner.txt"
printf '[]\n' > "$output/hashes.json"
failed=false
for arch in amd64 arm64; do
  mkdir "$work/$arch"
  # Extract only the executable, never arbitrary archive paths or scripts.
  unzip -p "$assets/kongctl_darwin_$arch.zip" kongctl > "$work/$arch/kongctl"
  status=0
  bash "$script_dir/verify-apple-binary.sh" "$work/$arch/kongctl" \
    > "$output/$arch-verification.txt" 2>&1 || status=$?
  codesign --display --verbose=4 "$work/$arch/kongctl" > "$output/$arch-signature.txt" 2>&1
  cdhash=$(sed -n 's/^CDHash=//p' "$output/$arch-signature.txt")
  [[ "$cdhash" =~ ^[0-9a-f]{40}$ ]]
  jq --arg arch "$arch" --arg cdhash "$cdhash" --argjson status "$status" \
    '. + [{arch: $arch, cdhash: $cdhash, verification_exit_code: $status}]' \
    "$output/hashes.json" > "$work/hashes.json"
  cp "$work/hashes.json" "$output/hashes.json"
  printf '%s: verification exit %s; CDHash %s\n' "$arch" "$status" "$cdhash"
  if [[ "$status" -ne 0 ]]; then failed=true; fi
done
if [[ "$failed" == true ]]; then
  # No Apple private credentials are present on this runner. Keep evidence local
  # to its short-lived artifact; don't dump daemon logs into console output.
  /usr/bin/log show --last 5m --style compact --info --debug \
    --predicate 'process == "syspolicyd" OR process == "trustd" OR process == "amfid"' \
    > "$output/security-services.txt" 2>&1 || true
  echo '::warning::A notarization/signature assessment failed; see diagnostic artifacts'
fi
if [[ -n "${GITHUB_STEP_SUMMARY:-}" ]]; then
  {
    echo '### Apple binary diagnostics'
    echo
    echo 'Evidence collection only; this workflow never approves a release.'
    echo
    jq -r '.[] | "- \(.arch): verification exit \(.verification_exit_code); CDHash `\(.cdhash)`"' \
      "$output/hashes.json"
  } >> "$GITHUB_STEP_SUMMARY"
fi
