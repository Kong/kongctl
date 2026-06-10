#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
INSTALLER="${ROOT}/scripts/install.sh"

tmp_base="${KONGCTL_INSTALLER_TEST_TMPDIR:-}"
if [[ -z "${tmp_base}" ]]; then
  tmp_base="$(go env GOCACHE 2>/dev/null || true)"
fi
if [[ -z "${tmp_base}" ]]; then
  tmp_base="${TMPDIR:-/tmp}"
fi

mkdir -p "${tmp_base}"
TMP_ROOT="$(mktemp -d "${tmp_base%/}/kongctl-installer-tests.XXXXXX")"
trap 'rm -rf "${TMP_ROOT}"' EXIT

LAST_OUTPUT=""
LAST_STATUS=0
LAST_INSTALL_DIR=""

fail() {
  echo "not ok - $1" >&2
  if [[ -n "${2:-}" && -f "$2" ]]; then
    echo "--- output ---" >&2
    cat "$2" >&2
    echo "--------------" >&2
  fi
  exit 1
}

pass() {
  echo "ok - $1"
}

assert_contains() {
  local file="$1"
  local pattern="$2"
  local name="$3"

  if ! grep -Fq "$pattern" "$file"; then
    fail "$name" "$file"
  fi
}

assert_executable() {
  local file="$1"
  local name="$2"

  if [[ ! -x "$file" ]]; then
    fail "$name"
  fi
}

write_fake_binary() {
  local path="$1"
  local version="$2"
  local os="$3"
  local arch="$4"

  cat > "$path" <<EOF
#!/bin/sh
if [ "\${1:-}" = "version" ] && [ "\${2:-}" = "--full" ]; then
  printf '%s\n' "kongctl fake ${version} ${os}/${arch}"
  exit 0
fi
printf '%s\n' "kongctl fake"
EOF
  chmod 755 "$path"
}

fixture_sha256() {
  local file="$1"

  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$file" | awk '{print $1}'
    return
  fi

  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$file" | awk '{print $1}'
    return
  fi

  if command -v openssl >/dev/null 2>&1; then
    openssl dgst -sha256 "$file" | awk '{print $NF}'
    return
  fi

  fail "sha256sum, shasum, or openssl is required for installer tests"
}

available_checksum_tool() {
  if command -v sha256sum >/dev/null 2>&1; then
    printf '%s\n' "sha256sum"
    return
  fi

  if command -v shasum >/dev/null 2>&1; then
    printf '%s\n' "shasum"
    return
  fi

  if command -v openssl >/dev/null 2>&1; then
    printf '%s\n' "openssl"
    return
  fi

  fail "sha256sum, shasum, or openssl is required for installer tests"
}

append_checksum() {
  local release_dir="$1"
  local asset="$2"
  local checksum

  checksum="$(fixture_sha256 "${release_dir}/${asset}")"
  printf '%s  %s\n' "$checksum" "$asset" >> "${release_dir}/checksums.txt"
}

write_release_metadata() {
  local release_dir="$1"
  local version="$2"
  local checksum_digest

  checksum_digest="$(fixture_sha256 "${release_dir}/checksums.txt")"
  cat > "${release_dir}/release.json" <<EOF
{
  "tag_name": "${version}",
  "assets": [
    {
      "name": "checksums.txt",
      "digest": "sha256:${checksum_digest}"
    }
  ]
}
EOF
}

make_release() {
  local release_dir="$1"
  local version="${2:-v9.9.9}"
  local os
  local arch
  local asset
  local payload

  mkdir -p "$release_dir"
  : > "${release_dir}/checksums.txt"

  for os in linux darwin; do
    for arch in amd64 arm64; do
      asset="kongctl_${os}_${arch}.zip"
      payload="${release_dir}/payload-${os}-${arch}"
      mkdir -p "$payload"
      write_fake_binary "${payload}/kongctl" "$version" "$os" "$arch"
      printf 'fake license\n' > "${payload}/LICENSE"
      printf 'fake readme\n' > "${payload}/README.md"
      (cd "$payload" && zip -q "${release_dir}/${asset}" LICENSE README.md kongctl)
      append_checksum "$release_dir" "$asset"
    done
  done

  write_release_metadata "$release_dir" "$version"
}

make_unsafe_release() {
  local release_dir="$1"
  local payload="${release_dir}/payload"
  local asset="kongctl_linux_amd64.zip"

  mkdir -p "${payload}/bin"
  write_fake_binary "${payload}/bin/kongctl" "v9.9.9" "linux" "amd64"
  : > "${release_dir}/checksums.txt"
  (cd "$payload" && zip -q "${release_dir}/${asset}" bin/kongctl)
  append_checksum "$release_dir" "$asset"
  write_release_metadata "$release_dir" "v9.9.9"
}

make_bad_checksum_release() {
  local release_dir="$1"

  make_release "$release_dir"
  printf '%064d  kongctl_linux_amd64.zip\n' 0 > "${release_dir}/checksums.txt"
  write_release_metadata "$release_dir" "v9.9.9"
}

make_bad_metadata_release() {
  local release_dir="$1"

  make_release "$release_dir"
  cat > "${release_dir}/release.json" <<'EOF'
{
  "tag_name": "v9.9.9",
  "assets": [
    {
      "name": "checksums.txt",
      "digest": "sha256:0000000000000000000000000000000000000000000000000000000000000000"
    }
  ]
}
EOF
}

run_installer() {
  local name="$1"
  local release_dir="$2"
  shift 2

  local case_dir="${TMP_ROOT}/${name}"
  local metadata_env=()
  mkdir -p "$case_dir"
  LAST_OUTPUT="${case_dir}/output.log"
  LAST_INSTALL_DIR="${case_dir}/bin"
  if [[ -f "${release_dir}/release.json" ]]; then
    metadata_env=(KONGCTL_RELEASE_METADATA_URL="file://${release_dir}/release.json")
  fi

  set +e
  env \
    HOME="${case_dir}/home" \
    KONGCTL_ALLOW_FILE_URLS=1 \
    KONGCTL_RELEASE_BASE_URL="file://${release_dir}" \
    PATH="${PATH}" \
    "${metadata_env[@]}" \
    /bin/sh "$INSTALLER" --install-dir "$LAST_INSTALL_DIR" "$@" > "$LAST_OUTPUT" 2>&1
  LAST_STATUS=$?
  set -e
}

expect_success() {
  local name="$1"
  local release_dir="$2"
  shift 2

  run_installer "$name" "$release_dir" "$@"
  if [[ "$LAST_STATUS" -ne 0 ]]; then
    fail "$name" "$LAST_OUTPUT"
  fi
  pass "$name"
}

expect_failure() {
  local name="$1"
  local release_dir="$2"
  local expected="$3"
  shift 3

  run_installer "$name" "$release_dir" "$@"
  if [[ "$LAST_STATUS" -eq 0 ]]; then
    fail "$name should have failed" "$LAST_OUTPUT"
  fi
  assert_contains "$LAST_OUTPUT" "$expected" "$name"
  pass "$name"
}

test_success_matrix() {
  local release_dir="${TMP_ROOT}/release"
  local os
  local arch
  local bin

  make_release "$release_dir"

  for os in linux darwin; do
    for arch in amd64 arm64; do
      expect_success "installs ${os}/${arch}" "$release_dir" --os "$os" --arch "$arch"
      bin="${LAST_INSTALL_DIR}/kongctl"
      assert_executable "$bin" "installed binary is executable for ${os}/${arch}"
      "$bin" version --full | grep -Fq "kongctl fake v9.9.9 ${os}/${arch}" ||
        fail "installed binary reports version for ${os}/${arch}"
    done
  done
}

test_version_pin_and_install_dir() {
  local release_dir="${TMP_ROOT}/release-version"

  make_release "$release_dir" "v1.2.3"
  expect_success "supports version pinning" "$release_dir" --version "1.2.3" --os linux --arch amd64
  assert_contains "$LAST_OUTPUT" "Resolved version: 1.2.3" "version pin is normalized"
  assert_executable "${LAST_INSTALL_DIR}/kongctl" "install-dir override is honored"
}

test_completion_output() {
  local release_dir="${TMP_ROOT}/release-completion"

  make_release "$release_dir" "v2.0.0"
  expect_success "prints completion output" "$release_dir" --os linux --arch amd64
  assert_contains "$LAST_OUTPUT" "OK kongctl successfully installed!" "completion success line"
  assert_contains "$LAST_OUTPUT" "Version: kongctl fake v2.0.0 linux/amd64" "completion version line"
  assert_contains "$LAST_OUTPUT" "Location: ${LAST_INSTALL_DIR}/kongctl" "completion location line"
  assert_contains "$LAST_OUTPUT" "Next: Run kongctl --help to get started" "completion next step"
}

test_update_status() {
  local release_dir="${TMP_ROOT}/release-update"
  local case_dir="${TMP_ROOT}/update-status"
  local install_dir="${case_dir}/bin"

  make_release "$release_dir" "v2.0.0"
  mkdir -p "$install_dir"
  write_fake_binary "${install_dir}/kongctl" "v1.0.0" "linux" "amd64"

  set +e
  env \
    HOME="${case_dir}/home" \
    KONGCTL_ALLOW_FILE_URLS=1 \
    KONGCTL_RELEASE_BASE_URL="file://${release_dir}" \
    KONGCTL_RELEASE_METADATA_URL="file://${release_dir}/release.json" \
    PATH="${PATH}" \
    /bin/sh "$INSTALLER" --install-dir "$install_dir" \
      --os linux --arch amd64 > "${case_dir}.log" 2>&1
  LAST_STATUS=$?
  set -e
  if [[ "$LAST_STATUS" -ne 0 ]]; then
    fail "update status install should succeed" "${case_dir}.log"
  fi
  assert_contains "${case_dir}.log" "Updating kongctl from 1.0.0 to 2.0.0" "update status"
  pass "prints update status"
}

test_release_metadata_digest_failure() {
  local release_dir="${TMP_ROOT}/release-bad-metadata"

  make_bad_metadata_release "$release_dir"
  expect_failure "fails on checksum manifest digest mismatch" "$release_dir" \
    "checksum mismatch for checksums.txt" --os linux --arch amd64
}

test_yes_flag_compatibility() {
  local release_dir="${TMP_ROOT}/release-yes"

  make_release "$release_dir"
  expect_success "accepts yes flag" "$release_dir" --yes --os linux --arch amd64
}

test_checksum_failure() {
  local release_dir="${TMP_ROOT}/release-bad-checksum"

  make_bad_checksum_release "$release_dir"
  expect_failure "fails on checksum mismatch" "$release_dir" "checksum mismatch" --os linux --arch amd64
}

test_unsupported_platforms() {
  local release_dir="${TMP_ROOT}/release-unsupported"

  make_release "$release_dir"
  expect_failure "fails on unsupported os" "$release_dir" "unsupported OS" --os solaris --arch amd64
  expect_failure "fails on unsupported arch" "$release_dir" "unsupported architecture" --os linux --arch riscv64
}

test_missing_dependencies() {
  local release_dir="${TMP_ROOT}/release-deps"
  local empty_path="${TMP_ROOT}/empty-path"
  local tool_path="${TMP_ROOT}/tool-path"
  local checksum_tool

  make_release "$release_dir"
  mkdir -p "$empty_path" "$tool_path"

  set +e
  env \
    HOME="${TMP_ROOT}/missing-downloader-home" \
    KONGCTL_ALLOW_FILE_URLS=1 \
    KONGCTL_RELEASE_BASE_URL="file://${release_dir}" \
    PATH="$empty_path" \
    /bin/sh "$INSTALLER" --yes --install-dir "${TMP_ROOT}/missing-downloader-bin" \
      --os linux --arch amd64 > "${TMP_ROOT}/missing-downloader.log" 2>&1
  LAST_STATUS=$?
  set -e
  if [[ "$LAST_STATUS" -eq 0 ]]; then
    fail "missing downloader should fail" "${TMP_ROOT}/missing-downloader.log"
  fi
  assert_contains "${TMP_ROOT}/missing-downloader.log" "curl or wget is required" "missing downloader"
  pass "fails when downloader is missing"

  ln -s "$(command -v curl)" "${tool_path}/curl"
  checksum_tool="$(available_checksum_tool)"
  ln -s "$(command -v "$checksum_tool")" "${tool_path}/${checksum_tool}"
  set +e
  env \
    HOME="${TMP_ROOT}/missing-extractor-home" \
    KONGCTL_ALLOW_FILE_URLS=1 \
    KONGCTL_RELEASE_BASE_URL="file://${release_dir}" \
    KONGCTL_RELEASE_METADATA_URL="file://${release_dir}/release.json" \
    PATH="$tool_path" \
    /bin/sh "$INSTALLER" --yes --install-dir "${TMP_ROOT}/missing-extractor-bin" \
      --os linux --arch amd64 > "${TMP_ROOT}/missing-extractor.log" 2>&1
  LAST_STATUS=$?
  set -e
  if [[ "$LAST_STATUS" -eq 0 ]]; then
    fail "missing extractor should fail" "${TMP_ROOT}/missing-extractor.log"
  fi
  assert_contains "${TMP_ROOT}/missing-extractor.log" "unzip or bsdtar is required" "missing extractor"
  pass "fails when extractor is missing"
}

test_unsafe_archive() {
  local release_dir="${TMP_ROOT}/release-unsafe"

  make_unsafe_release "$release_dir"
  expect_failure "refuses unexpected archive paths" "$release_dir" "unexpected path" --os linux --arch amd64
}

test_existing_directory_refusal() {
  local release_dir="${TMP_ROOT}/release-existing-dir"
  local case_dir="${TMP_ROOT}/existing-dir"
  local install_dir="${case_dir}/bin"

  make_release "$release_dir"
  mkdir -p "${install_dir}/kongctl"

  set +e
  env \
    HOME="${case_dir}/home" \
    KONGCTL_ALLOW_FILE_URLS=1 \
    KONGCTL_RELEASE_BASE_URL="file://${release_dir}" \
    KONGCTL_RELEASE_METADATA_URL="file://${release_dir}/release.json" \
    PATH="${PATH}" \
    /bin/sh "$INSTALLER" --yes --install-dir "$install_dir" \
      --os linux --arch amd64 > "${case_dir}.log" 2>&1
  LAST_STATUS=$?
  set -e
  if [[ "$LAST_STATUS" -eq 0 ]]; then
    fail "existing directory should fail" "${case_dir}.log"
  fi
  assert_contains "${case_dir}.log" "refusing to replace directory" "existing directory refusal"
  pass "refuses to replace existing directory"
}

test_path_shadow_warning() {
  local release_dir="${TMP_ROOT}/release-shadow"
  local case_dir="${TMP_ROOT}/path-shadow"
  local shadow_dir="${case_dir}/shadow"
  local install_dir="${case_dir}/bin"

  make_release "$release_dir"
  mkdir -p "$shadow_dir"
  write_fake_binary "${shadow_dir}/kongctl" "v0.0.1" "linux" "amd64"

  set +e
  env \
    HOME="${case_dir}/home" \
    KONGCTL_ALLOW_FILE_URLS=1 \
    KONGCTL_RELEASE_BASE_URL="file://${release_dir}" \
    KONGCTL_RELEASE_METADATA_URL="file://${release_dir}/release.json" \
    PATH="${shadow_dir}:${PATH}" \
    /bin/sh "$INSTALLER" --yes --install-dir "$install_dir" \
      --os linux --arch amd64 > "${case_dir}.log" 2>&1
  LAST_STATUS=$?
  set -e
  if [[ "$LAST_STATUS" -ne 0 ]]; then
    fail "path shadow install should succeed" "${case_dir}.log"
  fi
  assert_contains "${case_dir}.log" "may shadow" "path shadow warning"
  pass "warns about PATH shadowing"
}

test_success_matrix
test_version_pin_and_install_dir
test_completion_output
test_yes_flag_compatibility
test_update_status
test_release_metadata_digest_failure
test_checksum_failure
test_unsupported_platforms
test_missing_dependencies
test_unsafe_archive
test_existing_directory_refusal
test_path_shadow_warning
