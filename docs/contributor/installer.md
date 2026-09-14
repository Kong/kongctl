# kongctl Installer

`scripts/install.sh` is the canonical source for the shell installer published
at:

```shell
https://get.konghq.com/kongctl
```

The hosted file in `Kong/get.konghq.com` is a reviewed copy, not the source of
truth. Update `scripts/install.sh` in this repository first. After changes land
on `main`, the `Sync get.konghq.com installer` workflow opens or updates a pull
request in `Kong/get.konghq.com` that copies the script to the root-level
`kongctl` path.

The sync workflow requires a `GET_KONGHQ_COM_TOKEN` secret with permission to
push branches and open pull requests in `Kong/get.konghq.com`. It must not
auto-merge the hosted-script pull request.

Before merging installer changes, run:

```shell
make test-installer
```

The installer verifies GitHub release archive checksums by default. Stronger
release provenance, such as signed checksums or artifact attestations, should be
added as a future hardening step without weakening checksum verification.

## Uninstall behavior

`sh scripts/install.sh --uninstall` removes only `<install-dir>/kongctl` without
prompting or running the installed binary. Directory precedence is
`--install-dir`, `KONGCTL_INSTALL_DIR`, then `$HOME/.local/bin`. This works for
existing stable and preview installations without a receipt. A downloaded
script works offline using filesystem utilities, before platform detection,
release lookup, download, checksum, extraction, or version execution.

`--yes` remains accepted. `--version`, `--os`, and `--arch` are ignored in this
mode but still require values. All install-only environment settings are
ignored: `KONGCTL_VERSION`, `KONGCTL_INSTALL_OS`, `KONGCTL_INSTALL_ARCH`,
`KONGCTL_INSTALL_ART`, `KONGCTL_RELEASE_BASE_URL`,
`KONGCTL_RELEASE_METADATA_URL`, and `KONGCTL_ALLOW_FILE_URLS`.

Uninstall reuses the installation lock. It refuses target symlinks (including
dangling links), directories, and other non-regular files. Homebrew keg paths
and receipts, plus ownership reported by available `dpkg-query` or `rpm`
commands, cause refusal with package-manager guidance. A custom directory
under `/usr/local` alone is not evidence of package ownership. Homebrew users
should run `brew uninstall kongctl`.

A missing target succeeds without creating an installation directory. Removal
and permission failures are errors; sudo is never invoked. Uninstall preserves
the directory, neighboring files, configuration, credentials, logs, extensions,
user data, and shell profiles. Configuration usually resides under
`${XDG_CONFIG_HOME:-$HOME/.config}/kongctl`. Data and manually added shell setup
require a separate cleanup decision, particularly when another copy is used.

The manual fallback for older scripts is `rm "$HOME/.local/bin/kongctl"`.
For custom installations, remove only `kongctl` from the selected directory.
Inspect `command -v kongctl` or `type -a kongctl` to understand PATH resolution,
but never blindly delete their output. Uninstall selects no targets from PATH;
it reports a remaining PATH copy after removing the requested binary.

`make test-installer` covers isolated uninstall paths, preserved data, unsafe
targets, package ownership, locking, and failures. Uninstall tests restrict
PATH to filesystem tools to verify operation without installation dependencies.
Run the suite on Linux and macOS; the installer remains POSIX shell compatible.

## Publishing and verification

After the canonical change lands on `main`, review and merge the existing sync
workflow's PR in `Kong/get.konghq.com`. Do not bypass that review or auto-merge
the hosted copy. Until publication, the hosted script may lack `--uninstall`;
use the canonical script or the manual fallback above.

After publication, download the hosted script and compare it with the merged
canonical script, check `--help`, and test against an isolated directory:

```shell
curl -fsSL https://get.konghq.com/kongctl -o kongctl-install.sh
cmp kongctl-install.sh scripts/install.sh
sh kongctl-install.sh --help
test_dir="$(mktemp -d)"
printf 'unrunnable preview binary\n' > "$test_dir/kongctl"
curl -fsSL https://get.konghq.com/kongctl |
  sh -s -- --uninstall --install-dir "$test_dir"
test ! -e "$test_dir/kongctl"
sh kongctl-install.sh --uninstall --install-dir "$test_dir"
rmdir "$test_dir"
```

Confirm the removed path, data preservation, and subsequent already-absent
messages. Record the sync PR and verification results with the canonical PR.
