#!/usr/bin/env bash

set -euo pipefail

PUBLIC_SDK_MODULE="${PUBLIC_SDK_MODULE:-github.com/Kong/sdk-konnect-go}"
INTERNAL_SDK_MODULE="${INTERNAL_SDK_MODULE:-github.com/Kong/sdk-konnect-go-internal}"
PRIVATE_SDK_DIR="${PRIVATE_SDK_DIR:-.private/sdk-konnect-go-internal}"

die() {
  echo "setup-private-sdk: $*" >&2
  exit 1
}

write_output() {
  local name="$1"
  local value="$2"

  if [ -n "${GITHUB_OUTPUT:-}" ]; then
    printf '%s=%s\n' "${name}" "${value}" >> "${GITHUB_OUTPUT}"
  fi
}

append_go_env_pattern() {
  local current="$1"
  local pattern="$2"

  case ",${current}," in
    *",${pattern},"*)
      printf '%s\n' "${current}"
      ;;
    ",,")
      printf '%s\n' "${pattern}"
      ;;
    *)
      printf '%s,%s\n' "${current}" "${pattern}"
      ;;
  esac
}

export_private_go_env() {
  local current_goprivate
  local current_gonosumdb

  current_goprivate="${GOPRIVATE:-$(go env GOPRIVATE 2>/dev/null || true)}"
  current_gonosumdb="${GONOSUMDB:-$(go env GONOSUMDB 2>/dev/null || true)}"

  GOPRIVATE="$(append_go_env_pattern "${current_goprivate}" "${INTERNAL_SDK_MODULE}")"
  GONOSUMDB="$(append_go_env_pattern "${current_gonosumdb}" "${INTERNAL_SDK_MODULE}")"
  export GOPRIVATE GONOSUMDB

  if [ -n "${GITHUB_ENV:-}" ]; then
    {
      printf 'GOPRIVATE=%s\n' "${GOPRIVATE}"
      printf 'GONOSUMDB=%s\n' "${GONOSUMDB}"
    } >> "${GITHUB_ENV}"
  fi
}

sdk_replacement() {
  PUBLIC_SDK_MODULE="${PUBLIC_SDK_MODULE}" python3 -c '
import json
import os
import sys

public_module = os.environ["PUBLIC_SDK_MODULE"]
data = json.load(sys.stdin)

for replacement in data.get("Replace") or []:
    old = replacement.get("Old", {})
    new = replacement.get("New", {})
    if old.get("Path") == public_module:
        print("{}\t{}".format(new.get("Path", ""), new.get("Version", "")))
        break
' < <(go mod edit -json)
}

checkout_ref_for_version() {
  local version="$1"

  if [[ "${version}" =~ -([0-9a-fA-F]{12,})(\+incompatible)?$ ]]; then
    printf '%s\n' "${BASH_REMATCH[1]}"
    return
  fi

  printf '%s\n' "${version}"
}

clone_private_sdk() {
  local token="$1"
  local checkout_ref="$2"
  local auth_header
  local parent_dir

  parent_dir="$(dirname "${PRIVATE_SDK_DIR}")"
  mkdir -p "${parent_dir}"

  if [ -e "${PRIVATE_SDK_DIR}" ]; then
    die "private SDK directory already exists: ${PRIVATE_SDK_DIR}"
  fi

  auth_header="$(printf 'x-access-token:%s' "${token}" | base64 | tr -d '\n')"

  git -c "http.https://github.com/.extraheader=AUTHORIZATION: basic ${auth_header}" \
    clone "https://github.com/Kong/sdk-konnect-go-internal.git" "${PRIVATE_SDK_DIR}"

  git -C "${PRIVATE_SDK_DIR}" \
    -c "http.https://github.com/.extraheader=AUTHORIZATION: basic ${auth_header}" \
    fetch --tags --force origin
  git -C "${PRIVATE_SDK_DIR}" checkout --detach "${checkout_ref}"
}

main() {
  local replacement
  local replacement_path
  local replacement_version
  local token
  local checkout_ref
  local local_replace_path
  local sdk_commit

  replacement="$(sdk_replacement)"
  if [ -z "${replacement}" ]; then
    echo "No ${PUBLIC_SDK_MODULE} replacement found; using public SDK."
    write_output "sdk_mode" "public"
    write_output "sdk_module" "${PUBLIC_SDK_MODULE}"
    return
  fi

  IFS=$'\t' read -r replacement_path replacement_version <<< "${replacement}"
  if [ "${replacement_path}" != "${INTERNAL_SDK_MODULE}" ]; then
    echo "${PUBLIC_SDK_MODULE} replacement points to ${replacement_path}; private SDK setup not needed."
    write_output "sdk_mode" "public"
    write_output "sdk_module" "${replacement_path}"
    return
  fi

  if [ -z "${replacement_version}" ]; then
    die "${PUBLIC_SDK_MODULE} replacement points to ${INTERNAL_SDK_MODULE} without a pinned version"
  fi

  token="${GH_TOKEN_PRIVATE_READ:-${GH_PRIVATE_READ_TOKEN:-}}"
  if [ -z "${token}" ]; then
    die "GH_TOKEN_PRIVATE_READ is required when go.mod replaces ${PUBLIC_SDK_MODULE} with ${INTERNAL_SDK_MODULE}"
  fi

  export_private_go_env

  checkout_ref="$(checkout_ref_for_version "${replacement_version}")"
  clone_private_sdk "${token}" "${checkout_ref}"

  local_replace_path="./${PRIVATE_SDK_DIR#./}"
  go mod edit -replace="${PUBLIC_SDK_MODULE}=${local_replace_path}"

  sdk_commit="$(git -C "${PRIVATE_SDK_DIR}" rev-parse HEAD)"

  echo "Configured ${PUBLIC_SDK_MODULE} to use ${INTERNAL_SDK_MODULE} at ${sdk_commit}."
  write_output "sdk_mode" "internal"
  write_output "sdk_module" "${INTERNAL_SDK_MODULE}"
  write_output "sdk_version" "${replacement_version}"
  write_output "sdk_commit" "${sdk_commit}"
}

main "$@"
