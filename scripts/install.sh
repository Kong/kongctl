#!/bin/sh
# shellcheck shell=sh
set -eu

PROGRAM="kongctl"
REPO="Kong/kongctl"

VERSION="${KONGCTL_VERSION:-}"
INSTALL_DIR="${KONGCTL_INSTALL_DIR:-}"
INSTALL_OS="${KONGCTL_INSTALL_OS:-}"
INSTALL_ARCH="${KONGCTL_INSTALL_ARCH:-}"
RELEASE_BASE_URL="${KONGCTL_RELEASE_BASE_URL:-}"
RELEASE_METADATA_URL="${KONGCTL_RELEASE_METADATA_URL:-}"
ALLOW_FILE_URLS="${KONGCTL_ALLOW_FILE_URLS:-}"
INSTALL_ART="${KONGCTL_INSTALL_ART:-auto}"

TMP_DIR=""
DOWNLOADER=""
CHECKSUM_TOOL=""
EXTRACTOR=""
INSTALL_TARGET=""
INSTALL_DIR_ABS=""
INSTALLED_VERSION=""
CURRENT_VERSION=""
RESOLVED_VERSION="latest stable"
RELEASE_METADATA_FILE=""
LOCK_DIR=""
LOCK_ACQUIRED="0"
LOCK_STALE_AFTER_SECS=600
USE_COLOR="0"

if [ -t 1 ] && [ -z "${NO_COLOR:-}" ]; then
  USE_COLOR="1"
fi

log() {
  printf '%s\n' "$*"
}

warn() {
  if [ "$USE_COLOR" = "1" ]; then
    printf '\033[33m!\033[0m %s\n' "$*" >&2
  else
    printf 'warning: %s\n' "$*" >&2
  fi
}

die() {
  printf 'error: %s\n' "$*" >&2
  exit 1
}

cleanup() {
  release_install_lock
  if [ -n "$TMP_DIR" ] && [ -d "$TMP_DIR" ]; then
    rm -rf "$TMP_DIR"
  fi
}

trap cleanup EXIT
trap 'cleanup; exit 130' INT
trap 'cleanup; exit 143' HUP TERM

usage() {
  cat <<'EOF'
Install kongctl from GitHub Releases.

Usage:
  sh install.sh [flags]

Flags:
  --version VERSION      Install a specific release tag, such as v1.3.0
  --install-dir PATH    Install directory (default: $HOME/.local/bin)
  --os OS               Override OS detection: linux or darwin
  --arch ARCH           Override architecture detection: amd64 or arm64
  --yes                 Accepted for compatibility; install does not prompt
  --help                Show this help

Environment:
  KONGCTL_VERSION
  KONGCTL_INSTALL_DIR
  KONGCTL_INSTALL_OS
  KONGCTL_INSTALL_ARCH
  KONGCTL_INSTALL_ART    auto, always, or never (default: auto)

The installer verifies checksums before extraction and does not use sudo or
modify shell profile files.
EOF
}

color_text() {
  color_code="$1"
  text="$2"

  if [ "$USE_COLOR" = "1" ]; then
    printf '\033[%sm%s\033[0m' "$color_code" "$text"
  else
    printf '%s' "$text"
  fi
}

status() {
  marker="$(color_text 34 "==>")"
  printf '%s %s\n' "$marker" "$*"
}

success() {
  marker="$(color_text 32 "OK")"
  printf '%s %s\n' "$marker" "$*"
}

terminal_width() {
  cols=""

  if command -v tput >/dev/null 2>&1; then
    cols="$(tput cols 2>/dev/null || true)"
  fi

  case "$cols" in
    "" | *[!0-9]*)
      printf '%s\n' 0
      ;;
    *)
      printf '%s\n' "$cols"
      ;;
  esac
}

should_print_install_art() {
  case "$INSTALL_ART" in
    "" | auto | AUTO)
      [ -t 1 ] || return 1
      [ "${TERM:-}" != "dumb" ] || return 1
      ;;
    1 | true | TRUE | yes | YES | y | Y | always | ALWAYS)
      ;;
    0 | false | FALSE | no | NO | n | N | never | NEVER)
      return 1
      ;;
    *)
      die "KONGCTL_INSTALL_ART must be auto, always, or never"
      ;;
  esac
}

print_art_start() {
  if [ "$USE_COLOR" = "1" ]; then
    printf '\033[36m'
  fi
}

print_art_end() {
  if [ "$USE_COLOR" = "1" ]; then
    printf '\033[0m'
  fi
}

print_kong_logo_30() {
  print_art_start
  cat <<'EOF'
                  @@
               @@@@@@@@
              @@@@@@@@@@@
             @@@@@ @@@@@@@
            @@@@@@@ @@@@@
           @@@@@@@@@@ @
        @@@@ @@@@@@@@@@
    @@@@@@@@@@ @@@@@@@@@@
   @@@@@@@@@@@@@ @@@@@@@@@
 @@@@@@@@@@@@@@@  @@@@@@@@@@
@@@@@@@@@           @@@@@@@@@@
@@@@@@@@ @@@@@@@@     @@@@@@@@
@@@@@@     @@@@@@@     @@@@@@
EOF
  print_art_end
}

print_kong_logo_48() {
  print_art_start
  cat <<'EOF'
                             @@@
                          @@@@@@@@@
                        @@@@@@@@@@@@@
                       @@@@@@@@@@@@@@@@@
                            @@@@@@@@@@@
                     @@@@@@@ @@@@@@@@@@@@
                    @@@@@@@@@@ @@@@@@@@@@
                   @@@@@@@@@@@@  @@@@@@@
                  @@@@@@@@@@@@@@@  @@
                @@ @@@@@@@@@@@@@@@@
             @@@@@@@ @@@@@@@@@@@@@@@@
           @@@@@@@@@@  @@@@@@@@@@@@@@@
      @@@@@@@@@@@@@@@@@ @@@@@@@@@@@@@@@@
     @@@@@@@@@@@@@@@@@@@@ @@@@@@@@@@@@@@@@
   @@@@@@@@@@@@@@@@@@@@@@@  @@@@@@@@@@@@@@@
 @@@@@@@@@@@@@@@@@@@@@@@@     @@@@@@@@@@@@@@@
@@@@@@@@@@@@@@@@@@@@@@@@       @@@@@@@@@@@@@@@@
@@@@@@@@@@@@@@ @@@@@@@@@@        @@@@@@@@@@@@@@@
@@@@@@@@@@@@   @@@@@@@@@@@@        @@@@@@@@@@@@@
@@@@@@@@@@      @@@@@@@@@@@@        @@@@@@@@@@@
@@@@@@@@@        @@@@@@@@@@@@        @@@@@@@@@@
EOF
  print_art_end
}

print_padded_art_line() {
  padding="$1"
  line="$2"

  printf '%*s%s\n' "$padding" "" "$line"
}

print_art_border() {
  art_width="$1"
  i=0

  print_art_start
  while [ "$i" -lt "$art_width" ]; do
    printf '='
    i=$((i + 1))
  done
  printf '\n'
  print_art_end
}

print_kongctl_ogre_centered() {
  art_width="$1"
  wordmark_width=34
  padding=$(((art_width - wordmark_width) / 2))

  if [ "$padding" -lt 0 ]; then
    padding=0
  fi

  print_art_start
  while IFS= read -r line; do
    print_padded_art_line "$padding" "$line"
  done <<'EOF'
 _                          _   _
| | _____  _ __   __ _  ___| |_| |
| |/ / _ \| '_ \ / _` |/ __| __| |
|   < (_) | | | | (_| | (__| |_| |
|_|\_\___/|_| |_|\__, |\___|\__|_|
                 |___/
EOF
  print_art_end
}

print_install_art() {
  if ! should_print_install_art; then
    return 0
  fi

  cols="$(terminal_width)"
  if [ "$cols" -ne 0 ] && [ "$cols" -lt 48 ]; then
    return 0
  fi

  print_kongctl_ogre_centered 48
  print_art_border 48
  print_kong_logo_48

  printf '\n\n'
}

is_truthy() {
  case "$1" in
    1 | true | TRUE | yes | YES | y | Y)
      return 0
      ;;
    *)
      return 1
      ;;
  esac
}

require_arg() {
  if [ $# -lt 2 ] || [ -z "$2" ]; then
    die "$1 requires a value"
  fi
}

parse_args() {
  while [ "$#" -gt 0 ]; do
    case "$1" in
      --version)
        require_arg "$1" "${2:-}"
        VERSION="$2"
        shift 2
        ;;
      --version=*)
        VERSION="${1#--version=}"
        [ -n "$VERSION" ] || die "--version requires a value"
        shift
        ;;
      --install-dir)
        require_arg "$1" "${2:-}"
        INSTALL_DIR="$2"
        shift 2
        ;;
      --install-dir=*)
        INSTALL_DIR="${1#--install-dir=}"
        [ -n "$INSTALL_DIR" ] || die "--install-dir requires a value"
        shift
        ;;
      --os)
        require_arg "$1" "${2:-}"
        INSTALL_OS="$2"
        shift 2
        ;;
      --os=*)
        INSTALL_OS="${1#--os=}"
        [ -n "$INSTALL_OS" ] || die "--os requires a value"
        shift
        ;;
      --arch)
        require_arg "$1" "${2:-}"
        INSTALL_ARCH="$2"
        shift 2
        ;;
      --arch=*)
        INSTALL_ARCH="${1#--arch=}"
        [ -n "$INSTALL_ARCH" ] || die "--arch requires a value"
        shift
        ;;
      --yes)
        shift
        ;;
      --help | -h)
        usage
        exit 0
        ;;
      --)
        shift
        break
        ;;
      *)
        die "unknown argument: $1"
        ;;
    esac
  done
}

normalize_version() {
  if [ -z "$VERSION" ]; then
    return
  fi

  case "$VERSION" in
    v* | V*)
      VERSION="v${VERSION#?}"
      ;;
    [0-9]*)
      VERSION="v$VERSION"
      ;;
  esac
}

detect_os() {
  if [ -n "$INSTALL_OS" ]; then
    case "$INSTALL_OS" in
      linux | darwin)
        printf '%s\n' "$INSTALL_OS"
        return
        ;;
      windows)
        die "Windows is not supported by this shell installer; download a Windows release asset instead"
        ;;
      *)
        die "unsupported OS: $INSTALL_OS"
        ;;
    esac
  fi

  case "$(uname -s 2>/dev/null || true)" in
    Linux)
      printf '%s\n' "linux"
      ;;
    Darwin)
      printf '%s\n' "darwin"
      ;;
    MINGW* | MSYS* | CYGWIN*)
      die "Windows is not supported by this shell installer; download a Windows release asset instead"
      ;;
    *)
      die "unsupported OS: $(uname -s 2>/dev/null || printf '%s' unknown)"
      ;;
  esac
}

detect_arch() {
  if [ -n "$INSTALL_ARCH" ]; then
    case "$INSTALL_ARCH" in
      amd64 | arm64)
        printf '%s\n' "$INSTALL_ARCH"
        return
        ;;
      *)
        die "unsupported architecture: $INSTALL_ARCH"
        ;;
    esac
  fi

  case "$(uname -m 2>/dev/null || true)" in
    x86_64 | amd64)
      printf '%s\n' "amd64"
      ;;
    arm64 | aarch64)
      printf '%s\n' "arm64"
      ;;
    *)
      die "unsupported architecture: $(uname -m 2>/dev/null || printf '%s' unknown)"
      ;;
  esac
}

set_default_install_dir() {
  if [ -n "$INSTALL_DIR" ]; then
    return
  fi

  if [ -z "${HOME:-}" ]; then
    die "HOME is not set; pass --install-dir"
  fi

  INSTALL_DIR="$HOME/.local/bin"
}

find_downloader() {
  if command -v curl >/dev/null 2>&1; then
    DOWNLOADER="curl"
    return
  fi

  if command -v wget >/dev/null 2>&1; then
    DOWNLOADER="wget"
    return
  fi

  die "curl or wget is required to download release artifacts"
}

find_checksum_tool() {
  if command -v sha256sum >/dev/null 2>&1; then
    CHECKSUM_TOOL="sha256sum"
    return
  fi

  if command -v shasum >/dev/null 2>&1; then
    CHECKSUM_TOOL="shasum"
    return
  fi

  if command -v openssl >/dev/null 2>&1; then
    CHECKSUM_TOOL="openssl"
    return
  fi

  die "sha256sum, shasum, or openssl is required to verify release checksums"
}

find_extractor() {
  if command -v unzip >/dev/null 2>&1; then
    EXTRACTOR="unzip"
    return
  fi

  if command -v bsdtar >/dev/null 2>&1; then
    EXTRACTOR="bsdtar"
    return
  fi

  die "unzip or bsdtar is required to extract kongctl release archives"
}

prepare_install_dir() {
  mkdir -p "$INSTALL_DIR" || die "could not create install directory: $INSTALL_DIR"

  INSTALL_DIR_ABS="$(cd "$INSTALL_DIR" && pwd -P)" ||
    die "could not resolve install directory: $INSTALL_DIR"
  INSTALL_TARGET="$INSTALL_DIR_ABS/$PROGRAM"
  LOCK_DIR="$INSTALL_DIR_ABS/.${PROGRAM}-install.lock.d"
}

lock_is_stale() {
  [ -d "$LOCK_DIR" ] || return 1

  pid="$(cat "$LOCK_DIR/pid" 2>/dev/null || true)"
  started_at="$(cat "$LOCK_DIR/started_at" 2>/dev/null || true)"
  now="$(date +%s 2>/dev/null || printf '%s' 0)"

  case "$started_at" in
    "" | *[!0-9]*)
      started_at=0
      ;;
  esac

  if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
    return 1
  fi

  if [ "$started_at" -eq 0 ] || [ "$now" -eq 0 ]; then
    return 0
  fi

  [ $((now - started_at)) -ge "$LOCK_STALE_AFTER_SECS" ]
}

acquire_install_lock() {
  while ! mkdir "$LOCK_DIR" 2>/dev/null; do
    if lock_is_stale; then
      warn "removing stale installer lock at $LOCK_DIR"
      rm -rf "$LOCK_DIR"
      continue
    fi

    die "another $PROGRAM install is already running for $INSTALL_DIR_ABS"
  done

  LOCK_ACQUIRED="1"
  printf '%s\n' "$$" > "$LOCK_DIR/pid"
  date +%s > "$LOCK_DIR/started_at" 2>/dev/null || true
}

release_install_lock() {
  if [ "$LOCK_ACQUIRED" = "1" ] && [ -n "$LOCK_DIR" ]; then
    rm -rf "$LOCK_DIR" 2>/dev/null || true
  fi
  LOCK_ACQUIRED="0"
}

ensure_https_url() {
  case "$1" in
    https://*)
      ;;
    file://*)
      if ! is_truthy "$ALLOW_FILE_URLS"; then
        die "refusing file URL outside installer tests: $1"
      fi
      ;;
    *)
      die "refusing non-HTTPS download URL: $1"
      ;;
  esac
}

download_file() {
  url="$1"
  dest="$2"

  ensure_https_url "$url"

  case "$DOWNLOADER" in
    curl)
      curl -fsSL "$url" -o "$dest"
      ;;
    wget)
      wget -q -O "$dest" "$url"
      ;;
    *)
      die "no downloader selected"
      ;;
  esac
}

download_text() {
  url="$1"

  ensure_https_url "$url"

  case "$DOWNLOADER" in
    curl)
      curl -fsSL "$url"
      ;;
    wget)
      wget -q -O - "$url"
      ;;
    *)
      die "no downloader selected"
      ;;
  esac
}

compute_sha256() {
  file="$1"

  case "$CHECKSUM_TOOL" in
    sha256sum)
      sha256sum "$file" | awk '{print $1}'
      ;;
    shasum)
      shasum -a 256 "$file" | awk '{print $1}'
      ;;
    openssl)
      openssl dgst -sha256 "$file" | awk '{print $NF}'
      ;;
    *)
      die "no checksum tool selected"
      ;;
  esac
}

release_metadata_url() {
  if [ -n "$RELEASE_METADATA_URL" ]; then
    printf '%s\n' "$RELEASE_METADATA_URL"
    return
  fi

  if [ -n "$VERSION" ]; then
    printf 'https://api.github.com/repos/%s/releases/tags/%s\n' "$REPO" "$VERSION"
    return
  fi

  printf 'https://api.github.com/repos/%s/releases/latest\n' "$REPO"
}

fetch_release_metadata() {
  metadata_url="$(release_metadata_url)"
  metadata_file="$TMP_DIR/release.json"

  if download_text "$metadata_url" > "$metadata_file"; then
    RELEASE_METADATA_FILE="$metadata_file"
    return
  fi

  if [ -n "$RELEASE_METADATA_URL" ]; then
    die "could not download release metadata from $metadata_url"
  fi

  warn "could not fetch GitHub release metadata; checksum file digest verification will be skipped"
}

release_tag_from_metadata() {
  [ -n "$RELEASE_METADATA_FILE" ] || return 1

  sed -n 's/.*"tag_name":[[:space:]]*"\([^"]*\)".*/\1/p' "$RELEASE_METADATA_FILE" | head -n 1
}

release_asset_digest_from_metadata() {
  metadata_asset="$1"
  [ -n "$RELEASE_METADATA_FILE" ] || return 1

  awk -v asset="$metadata_asset" '
    /"name":[[:space:]]*"[^"]+"/ {
      name = $0
      sub(/^.*"name":[[:space:]]*"/, "", name)
      sub(/".*$/, "", name)
      in_asset = (name == asset)
    }

    in_asset && /"digest":[[:space:]]*"sha256:[^"]+"/ {
      digest = $0
      sub(/^.*"digest":[[:space:]]*"sha256:/, "", digest)
      sub(/".*$/, "", digest)
      print tolower(digest)
      found = 1
      exit
    }

    END {
      if (!found) {
        exit 1
      }
    }
  ' "$RELEASE_METADATA_FILE"
}

verify_file_digest_from_metadata() {
  digest_file="$1"
  digest_asset="$2"

  [ -n "$RELEASE_METADATA_FILE" ] || return

  expected="$(release_asset_digest_from_metadata "$digest_asset")" ||
    die "release metadata does not contain SHA256 digest for $digest_asset"
  actual="$(compute_sha256 "$digest_file")"

  if [ "$expected" != "$actual" ]; then
    die "checksum mismatch for $digest_asset"
  fi

  success "Verified release digest for $digest_asset"
}

expected_checksum() {
  checksums_file="$1"
  checksum_asset="$2"

  awk -v asset="$checksum_asset" '
    $2 == asset {
      print $1
      found = 1
      exit
    }
    END {
      if (!found) {
        exit 1
      }
    }
  ' "$checksums_file"
}

verify_checksum() {
  checksums_file="$1"
  archive="$2"
  checksum_asset="$3"

  expected="$(expected_checksum "$checksums_file" "$checksum_asset")" ||
    die "checksums.txt does not contain $checksum_asset"
  actual="$(compute_sha256 "$archive")"

  if [ "$expected" != "$actual" ]; then
    die "checksum mismatch for $checksum_asset"
  fi

  success "Verified checksum for $checksum_asset"
}

list_archive() {
  archive="$1"

  case "$EXTRACTOR" in
    unzip)
      unzip -Z -1 "$archive"
      ;;
    bsdtar)
      bsdtar -tf "$archive"
      ;;
    *)
      die "no extractor selected"
      ;;
  esac
}

validate_archive_entries() {
  archive="$1"
  binary_name="$2"
  entries_file="$TMP_DIR/archive-entries.txt"
  found_binary="0"

  list_archive "$archive" > "$entries_file" || die "could not inspect archive"

  while IFS= read -r entry; do
    case "$entry" in
      "" | /* | ../* | */../* | */.. | *//*)
        die "archive contains unsafe path: $entry"
        ;;
    esac

    case "$entry" in
      "$binary_name")
        found_binary="1"
        ;;
      LICENSE | README.md)
        ;;
      *)
        die "archive contains unexpected path: $entry"
        ;;
    esac
  done < "$entries_file"

  if [ "$found_binary" != "1" ]; then
    die "archive does not contain expected $binary_name binary"
  fi
}

extract_binary() {
  archive="$1"
  binary_name="$2"
  dest="$3"

  case "$EXTRACTOR" in
    unzip)
      unzip -p "$archive" "$binary_name" > "$dest" || die "could not extract $binary_name"
      ;;
    bsdtar)
      bsdtar -xOf "$archive" "$binary_name" > "$dest" || die "could not extract $binary_name"
      ;;
    *)
      die "no extractor selected"
      ;;
  esac

  chmod 755 "$dest"
}

install_binary() {
  source_binary="$1"
  binary_name="$2"

  target="$INSTALL_TARGET"
  tmp_target="$INSTALL_DIR_ABS/.${binary_name}.tmp.$$"

  if [ -d "$target" ]; then
    die "refusing to replace directory: $target"
  fi

  if [ ! -w "$INSTALL_DIR_ABS" ]; then
    die "install directory is not writable: $INSTALL_DIR_ABS"
  fi

  if [ -e "$target" ] || [ -L "$target" ]; then
    warn "replacing existing file: $target"
  fi

  if [ -e "$tmp_target" ] || [ -L "$tmp_target" ]; then
    die "temporary install path already exists: $tmp_target"
  fi

  mv "$source_binary" "$tmp_target" || die "could not stage binary in install directory"
  chmod 755 "$tmp_target"
  mv -f "$tmp_target" "$target" || die "could not install binary to $target"

  INSTALL_TARGET="$target"
}

version_from_binary() {
  binary_path="$1"
  [ -x "$binary_path" ] || return 0

  version_home="$TMP_DIR/version-home"
  version_config_home="$TMP_DIR/version-config"
  mkdir -p "$version_home" "$version_config_home" ||
    die "could not create version check directories"

  output="$(HOME="$version_home" XDG_CONFIG_HOME="$version_config_home" DO_NOT_TRACK=1 "$binary_path" version --full 2>/dev/null || true)"
  printf '%s\n' "$output" | awk '
    {
      for (i = 1; i <= NF; i++) {
        value = $i
        sub(/^[^0-9vV]*/, "", value)
        sub(/[),].*$/, "", value)
        if (value ~ /^[vV]?[0-9][0-9A-Za-z.+-]*$/) {
          sub(/^[vV]/, "", value)
          print value
          exit
        }
      }
    }
  '
}

current_installed_version() {
  version_from_binary "$INSTALL_TARGET"
}

classify_existing_kongctl() {
  existing_path="$1"

  case "$existing_path" in
    /opt/homebrew/* | /usr/local/* | /home/linuxbrew/.linuxbrew/*)
      printf '%s\n' "Homebrew-managed"
      ;;
    *)
      printf '%s\n' "other"
      ;;
  esac
}

warn_path_shadowing() {
  binary_name="$1"
  existing="$(command -v "$binary_name" 2>/dev/null || true)"

  if [ -n "$existing" ] && [ "$existing" != "$INSTALL_TARGET" ]; then
    manager="$(classify_existing_kongctl "$existing")"
    if [ "$manager" = "Homebrew-managed" ]; then
      warn "existing Homebrew-managed $binary_name found at $existing; PATH order may prefer it over $INSTALL_TARGET"
    else
      warn "another $binary_name found at $existing; it may shadow $INSTALL_TARGET"
    fi
  fi
}

verify_installed_binary() {
  verify_home="$TMP_DIR/verify-home"
  verify_config_home="$TMP_DIR/verify-config"
  output=""

  mkdir -p "$verify_home" "$verify_config_home" ||
    die "could not create verification directories"

  if ! output="$(HOME="$verify_home" XDG_CONFIG_HOME="$verify_config_home" DO_NOT_TRACK=1 "$INSTALL_TARGET" version --full 2>&1)"; then
    printf '%s\n' "$output" >&2
    die "installed binary did not run successfully: $INSTALL_TARGET"
  fi

  INSTALLED_VERSION="$output"
}

make_temp_dir() {
  TMP_DIR="$(mktemp -d 2>/dev/null || mktemp -d -t kongctl-install)" ||
    die "could not create temporary directory"
}

release_base_url() {
  if [ -n "$RELEASE_BASE_URL" ]; then
    printf '%s\n' "${RELEASE_BASE_URL%/}"
    return
  fi

  if [ -n "$VERSION" ]; then
    printf 'https://github.com/%s/releases/download/%s\n' "$REPO" "$VERSION"
    return
  fi

  printf 'https://github.com/%s/releases/latest/download\n' "$REPO"
}

resolve_display_version() {
  tag="$(release_tag_from_metadata || true)"
  if [ -n "$tag" ]; then
    printf '%s\n' "${tag#v}"
    return
  fi

  if [ -n "$VERSION" ]; then
    printf '%s\n' "${VERSION#v}"
    return
  fi

  printf '%s\n' "latest stable"
}

platform_label() {
  case "$os/$arch" in
    linux/amd64)
      printf '%s\n' "Linux (x64)"
      ;;
    linux/arm64)
      printf '%s\n' "Linux (ARM64)"
      ;;
    darwin/amd64)
      printf '%s\n' "macOS (Intel)"
      ;;
    darwin/arm64)
      printf '%s\n' "macOS (Apple Silicon)"
      ;;
    *)
      printf '%s\n' "$os/$arch"
      ;;
  esac
}

install_dir_on_path() {
  case ":${PATH:-}:" in
    *":$INSTALL_DIR_ABS:"*)
      return 0
      ;;
    *)
      return 1
      ;;
  esac
}

print_completion() {
  printf '\n'
  success "$PROGRAM successfully installed!"
  printf '\n'
  printf '  Version: %s\n' "$INSTALLED_VERSION"
  printf '\n'
  printf '  Location: %s\n' "$INSTALL_TARGET"
  printf '\n'

  if ! install_dir_on_path; then
    printf '  Tip: Add %s to PATH:\n' "$INSTALL_DIR_ABS"
    printf "       export PATH=\"%s:\$PATH\"\n" "$INSTALL_DIR_ABS"
    printf '\n'
  fi

  printf '  Next: Run %s --help to get started\n' "$PROGRAM"
  printf '\n'
  success "Installation complete!"
}

main() {
  parse_args "$@"
  normalize_version
  set_default_install_dir

  os="$(detect_os)"
  arch="$(detect_arch)"
  archive_asset="${PROGRAM}_${os}_${arch}.zip"
  binary_name="$PROGRAM"
  base_url="$(release_base_url)"

  find_downloader
  find_checksum_tool
  find_extractor
  make_temp_dir
  prepare_install_dir
  acquire_install_lock
  fetch_release_metadata
  RESOLVED_VERSION="$(resolve_display_version)"
  CURRENT_VERSION="$(current_installed_version)"

  archive="$TMP_DIR/$archive_asset"
  checksums="$TMP_DIR/checksums.txt"
  extracted="$TMP_DIR/$binary_name"

  print_install_art

  if [ -n "$CURRENT_VERSION" ] && [ "$RESOLVED_VERSION" != "latest stable" ] &&
    [ "$CURRENT_VERSION" != "$RESOLVED_VERSION" ]; then
    status "Updating $PROGRAM from $CURRENT_VERSION to $RESOLVED_VERSION"
  elif [ -n "$CURRENT_VERSION" ]; then
    status "Updating $PROGRAM"
  else
    status "Installing $PROGRAM"
  fi
  status "Detected platform: $(platform_label)"
  status "Resolved version: $RESOLVED_VERSION"
  log "  Location: $INSTALL_DIR"
  log ""

  status "Downloading $PROGRAM"
  download_file "$base_url/$archive_asset" "$archive"
  download_file "$base_url/checksums.txt" "$checksums"
  verify_file_digest_from_metadata "$checksums" "checksums.txt"
  verify_checksum "$checksums" "$archive" "$archive_asset"
  validate_archive_entries "$archive" "$binary_name"
  extract_binary "$archive" "$binary_name" "$extracted"
  status "Installing $PROGRAM to $INSTALL_TARGET"
  install_binary "$extracted" "$binary_name"
  warn_path_shadowing "$binary_name"
  verify_installed_binary
  print_completion
}

main "$@"
