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

assert_not_contains() {
  local file="$1"
  local pattern="$2"
  local name="$3"

  if grep -Fq "$pattern" "$file"; then
    fail "$name" "$file"
  fi
}

assert_before() {
  local file="$1"
  local first="$2"
  local second="$3"
  local name="$4"
  local first_line
  local second_line

  first_line="$(grep -nF "$first" "$file" | head -n 1 | cut -d: -f1 || true)"
  second_line="$(grep -nF "$second" "$file" | head -n 1 | cut -d: -f1 || true)"

  if [[ -z "$first_line" || -z "$second_line" || "$first_line" -ge "$second_line" ]]; then
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

test_prerelease_tag_version_pin() {
  local release_dir="${TMP_ROOT}/release-prerelease"
  local tag="prerelease-gh-1391-ad-hoc-20260625-1842-gabcdef1"

  make_release "$release_dir" "$tag"
  expect_success "supports non-semver prerelease version pinning" "$release_dir" \
    --version "$tag" --os linux --arch amd64
  assert_contains "$LAST_OUTPUT" "Resolved version: $tag" "prerelease tag is not normalized"
  assert_executable "${LAST_INSTALL_DIR}/kongctl" "prerelease install succeeds"
}

test_completion_output() {
  local release_dir="${TMP_ROOT}/release-completion"

  make_release "$release_dir" "v2.0.0"
  expect_success "prints completion output" "$release_dir" --os linux --arch amd64
  assert_contains "$LAST_OUTPUT" "OK kongctl successfully installed!" "completion success line"
  assert_contains "$LAST_OUTPUT" "Version: kongctl fake v2.0.0 linux/amd64" "completion version line"
  assert_contains "$LAST_OUTPUT" "Location: ${LAST_INSTALL_DIR}/kongctl" "completion location line"
  assert_contains "$LAST_OUTPUT" "Next: Run kongctl --help to get started" "completion next step"
  assert_not_contains "$LAST_OUTPUT" "@@@@@@@@@@" "install art is hidden when stdout is not a terminal"
  assert_not_contains "$LAST_OUTPUT" "| | _____" "install wordmark is hidden when stdout is not a terminal"
}

test_install_art_output() {
  local release_dir="${TMP_ROOT}/release-art"
  local case_dir="${TMP_ROOT}/install-art"
  local install_dir="${case_dir}/bin"
  local wordmark_tail

  make_release "$release_dir" "v2.1.0"
  mkdir -p "$case_dir"
  wordmark_tail="$(printf '%24s|___/' '')"

  set +e
  env \
    HOME="${case_dir}/home" \
    KONGCTL_ALLOW_FILE_URLS=1 \
    KONGCTL_INSTALL_ART=always \
    KONGCTL_RELEASE_BASE_URL="file://${release_dir}" \
    KONGCTL_RELEASE_METADATA_URL="file://${release_dir}/release.json" \
    NO_COLOR=1 \
    PATH="${PATH}" \
    TERM=dumb \
    /bin/sh "$INSTALLER" --install-dir "$install_dir" \
      --os linux --arch amd64 > "${case_dir}.log" 2>&1
  LAST_STATUS=$?
  set -e
  if [[ "$LAST_STATUS" -ne 0 ]]; then
    fail "install art output should succeed" "${case_dir}.log"
  fi
  assert_contains "${case_dir}.log" "@@@@@@@@@@" "install art output"
  assert_contains "${case_dir}.log" "       _                          _   _" "centered install wordmark output"
  assert_contains "${case_dir}.log" "$wordmark_tail" "centered install wordmark descender"
  assert_contains "${case_dir}.log" "================================================" "install art border"
  assert_before "${case_dir}.log" "       _                          _   _" "================================================" \
    "install wordmark is before border"
  assert_before "${case_dir}.log" "================================================" "@@@@" "install border is before logo"
  assert_contains "${case_dir}.log" "OK kongctl successfully installed!" "install art completion"
  pass "prints install art when requested"
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

# Uninstall cases use only filesystem tools: no downloader, platform detection,
# checksum/extraction tools, or package managers from the host are available.
setup_uninstall() {
  local name="$1"
  local tool
  CASE_DIR="${TMP_ROOT}/uninstall-${name}"
  mkdir -p "${CASE_DIR}/tools" "${CASE_DIR}/home/.local/bin"
  for tool in mkdir rm cat date; do
    ln -s "$(command -v "$tool")" "${CASE_DIR}/tools/$tool"
  done
  UNINSTALL_DIR="${CASE_DIR}/home/.local/bin"
  printf 'unrunnable preview binary\n' > "${UNINSTALL_DIR}/kongctl"
}

run_uninstall() {
  LAST_OUTPUT="${CASE_DIR}/output.log"
  LAST_STATUS=0
  env -i HOME="${CASE_DIR}/home" PATH="${UNINSTALL_PATH:-${CASE_DIR}/tools}" \
    KONGCTL_INSTALL_DIR="${UNINSTALL_ENV_DIR:-}" \
    KONGCTL_VERSION=preview KONGCTL_INSTALL_OS=unsupported \
    KONGCTL_INSTALL_ARCH=unsupported KONGCTL_INSTALL_ART=invalid \
    KONGCTL_RELEASE_BASE_URL=https://invalid.invalid \
    KONGCTL_RELEASE_METADATA_URL=https://invalid.invalid \
    /bin/sh "$INSTALLER" --uninstall "$@" < /dev/null > "$LAST_OUTPUT" 2>&1 || LAST_STATUS=$?
}

uninstall_succeeded() {
  [[ "$LAST_STATUS" -eq 0 ]] || fail "$1" "$LAST_OUTPUT"
  assert_contains "$LAST_OUTPUT" "were preserved" "$1 preserves data message"
}

uninstall_failed() {
  [[ "$LAST_STATUS" -ne 0 ]] || fail "$1 should fail" "$LAST_OUTPUT"
  assert_contains "$LAST_OUTPUT" "$1" "$1"
}

test_uninstall_paths() {
  setup_uninstall paths
  run_uninstall --version --uninstall
  uninstall_failed "requires a value"
  [[ -f "${UNINSTALL_DIR}/kongctl" ]] || fail "invalid options preserve target"
  mkdir -p "${CASE_DIR}/home/.config/kongctl/extensions"
  printf 'credentials\n' > "${CASE_DIR}/home/.config/kongctl/token.json"
  printf 'extension\n' > "${CASE_DIR}/home/.config/kongctl/extensions/example"
  printf 'profile\n' > "${CASE_DIR}/home/.profile"
  printf 'neighbor\n' > "${UNINSTALL_DIR}/neighbor"
  run_uninstall --yes --version ignored --os unsupported --arch unsupported
  uninstall_succeeded "default offline uninstall"
  [[ ! -e "${UNINSTALL_DIR}/kongctl" && -d "$UNINSTALL_DIR" ]] || fail "remove only binary"
  assert_contains "$LAST_OUTPUT" "Removed ${UNINSTALL_DIR}/kongctl" "removed path"
  assert_contains "${UNINSTALL_DIR}/neighbor" neighbor "preserve neighbor"
  assert_contains "${CASE_DIR}/home/.config/kongctl/token.json" credentials "preserve credentials"
  assert_contains "${CASE_DIR}/home/.config/kongctl/extensions/example" extension "preserve extension"
  assert_contains "${CASE_DIR}/home/.profile" profile "preserve shell profile"
  run_uninstall
  uninstall_succeeded "repeat uninstall"
  assert_contains "$LAST_OUTPUT" "Already absent" "repeat removal"
  run_uninstall --install-dir "${CASE_DIR}/missing/bin"
  uninstall_succeeded "missing directory"
  [[ ! -e "${CASE_DIR}/missing" ]] || fail "must not create missing directory"

  local custom="${CASE_DIR}/custom path"
  mkdir -p "$custom"
  printf 'custom\n' > "${custom}/kongctl"
  printf 'default\n' > "${UNINSTALL_DIR}/kongctl"
  UNINSTALL_ENV_DIR="$custom" run_uninstall
  uninstall_succeeded "environment directory"
  [[ ! -e "${custom}/kongctl" && -f "${UNINSTALL_DIR}/kongctl" ]] || fail "environment precedence"
  printf 'custom\n' > "${custom}/kongctl"
  UNINSTALL_ENV_DIR="$custom" run_uninstall --install-dir "$UNINSTALL_DIR"
  uninstall_succeeded "flag precedence"
  [[ -f "${custom}/kongctl" && ! -e "${UNINSTALL_DIR}/kongctl" ]] || fail "flag precedence"
  run_uninstall --install-dir="$custom"
  uninstall_succeeded "equals flag and spaces"
  [[ ! -e "${custom}/kongctl" ]] || fail "custom path with spaces"
  printf '#!/bin/sh\necho executed > "%s"\nexit 1\n' "${CASE_DIR}/executed" > "${custom}/kongctl"
  chmod 555 "${custom}/kongctl"
  run_uninstall --install-dir "$custom"
  uninstall_succeeded "read-only executable removal"
  [[ ! -e "${CASE_DIR}/executed" && ! -e "${custom}/kongctl" ]] || fail "must not execute installed binary"
  pass "uninstall paths, precedence, offline operation, and preserved data"
}

test_uninstall_unsafe_targets() {
  setup_uninstall unsafe
  mv "${UNINSTALL_DIR}/kongctl" "${CASE_DIR}/original"
  ln -s "${CASE_DIR}/original" "${UNINSTALL_DIR}/kongctl"
  run_uninstall
  uninstall_failed "refusing to remove symlink"
  [[ -L "${UNINSTALL_DIR}/kongctl" && -f "${CASE_DIR}/original" ]] || fail "preserve symlink target"
  rm "${CASE_DIR}/original"
  run_uninstall
  uninstall_failed "refusing to remove symlink"
  [[ -L "${UNINSTALL_DIR}/kongctl" ]] || fail "preserve dangling link"
  rm "${UNINSTALL_DIR}/kongctl"
  mkdir "${UNINSTALL_DIR}/kongctl"
  printf 'keep\n' > "${UNINSTALL_DIR}/kongctl/child"
  run_uninstall
  uninstall_failed "refusing to remove directory"
  [[ -f "${UNINSTALL_DIR}/kongctl/child" ]] || fail "preserve directory contents"
  pass "uninstall refuses symlinks and directories"
}

test_uninstall_path_and_packages() {
  setup_uninstall packages
  mkdir "${CASE_DIR}/other"
  write_fake_binary "${CASE_DIR}/other/kongctl" preview linux amd64
  UNINSTALL_PATH="${UNINSTALL_DIR}:${CASE_DIR}/other:${CASE_DIR}/tools" run_uninstall
  uninstall_succeeded "another PATH copy"
  assert_contains "$LAST_OUTPUT" "remains installed on PATH: ${CASE_DIR}/other/kongctl" "remaining PATH copy"
  [[ -x "${CASE_DIR}/other/kongctl" ]] || fail "preserve PATH copy"

  local keg="${CASE_DIR}/prefix/Cellar/kongctl/1.0/bin"
  mkdir -p "$keg"
  printf 'brew\n' > "${keg}/kongctl"
  run_uninstall --install-dir "$keg"
  uninstall_failed "use brew uninstall kongctl"
  [[ -f "${keg}/kongctl" ]] || fail "preserve Homebrew keg"
  mkdir "${CASE_DIR}/alias"
  ln -s "$keg" "${CASE_DIR}/alias/bin"
  run_uninstall --install-dir "${CASE_DIR}/alias/bin"
  uninstall_failed "use brew uninstall kongctl"

  local custom="${CASE_DIR}/usr/local/bin"
  mkdir -p "$custom"
  printf 'manual\n' > "${custom}/kongctl"
  run_uninstall --install-dir "$custom"
  uninstall_succeeded "custom usr/local directory"
  printf 'receipt managed\n' > "${custom}/kongctl"
  printf '{}\n' > "${custom}/../INSTALL_RECEIPT.json"
  run_uninstall --install-dir "$custom"
  uninstall_failed "use brew uninstall kongctl"

  local manager
  for manager in dpkg-query rpm; do
    printf '#!/bin/sh\nexit 0\n' > "${CASE_DIR}/tools/$manager"
    chmod +x "${CASE_DIR}/tools/$manager"
    printf 'package\n' > "${UNINSTALL_DIR}/kongctl"
    run_uninstall
    uninstall_failed "package-managed installation"
    [[ -f "${UNINSTALL_DIR}/kongctl" ]] || fail "preserve package file"
    rm "${CASE_DIR}/tools/$manager"
  done
  pass "uninstall reports other copies and protects package installations"
}

test_uninstall_lock_and_errors() {
  setup_uninstall lock
  local lock="${UNINSTALL_DIR}/.kongctl-install.lock.d"
  mkdir "$lock"
  printf '%s\n' "$$" > "${lock}/pid"
  date +%s > "${lock}/started_at"
  run_uninstall
  uninstall_failed "already running"
  [[ -f "${UNINSTALL_DIR}/kongctl" && -d "$lock" ]] || fail "preserve active lock and binary"
  printf 'not-a-pid\n' > "${lock}/pid"
  printf '1\n' > "${lock}/started_at"
  run_uninstall
  uninstall_succeeded "stale lock recovery"
  [[ ! -e "$lock" ]] || fail "release uninstall lock"

  printf 'keep\n' > "${UNINSTALL_DIR}/kongctl"
  if [[ "$(id -u)" -ne 0 ]]; then
    chmod 555 "$UNINSTALL_DIR"
    run_uninstall
    chmod 755 "$UNINSTALL_DIR"
    uninstall_failed "could not acquire installer lock"
    assert_not_contains "$LAST_OUTPUT" "already running" "accurate permission error"
    chmod 000 "$UNINSTALL_DIR"
    run_uninstall --install-dir "${UNINSTALL_DIR}/missing/bin"
    chmod 755 "$UNINSTALL_DIR"
    uninstall_failed "not searchable"
  fi

  # Inject a removal failure even when tests run as root; cleanup still works.
  rm "${CASE_DIR}/tools/rm"
  cat > "${CASE_DIR}/tools/rm" <<EOF
#!/bin/sh
if [ "\${2:-}" = "${UNINSTALL_DIR}/kongctl" ]; then
  echo 'Permission denied' >&2
  exit 1
fi
exec "$(command -v rm)" "\$@"
EOF
  chmod +x "${CASE_DIR}/tools/rm"
  run_uninstall
  uninstall_failed "could not remove"
  [[ -f "${UNINSTALL_DIR}/kongctl" && ! -e "$lock" ]] || fail "failed removal preserves binary and releases lock"
  pass "uninstall locking and permission/removal errors"
}

test_uninstall_paths
test_uninstall_unsafe_targets
test_uninstall_path_and_packages
test_uninstall_lock_and_errors
test_success_matrix
test_version_pin_and_install_dir
test_prerelease_tag_version_pin
test_completion_output
test_install_art_output
test_yes_flag_compatibility
test_update_status
test_release_metadata_digest_failure
test_checksum_failure
test_unsupported_platforms
test_missing_dependencies
test_unsafe_archive
test_existing_directory_refusal
test_path_shadow_warning
