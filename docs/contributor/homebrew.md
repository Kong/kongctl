# Homebrew release process

The `Kong/homebrew-kongctl` tap publishes `kongctl` as both a prebuilt cask and
a source-built formula.

## Release ownership

GoReleaser builds and publishes release archives, checksums, container images,
and the cask. It does not generate the source-built formula.

After GoReleaser completes a stable release, the release workflow:

1. clones the tap into Homebrew's tap directory;
2. copies the generated cask into the tap;
3. reads the version, commit, and build date from GoReleaser's metadata;
4. downloads the tagged source archive and calculates its SHA-256 checksum;
5. updates the formula with the source and linker metadata;
6. runs Homebrew style and audit checks and tests both installation methods;
7. commits the cask directly to the tap's default branch, preserving its
   existing publication path;
8. opens a formula-only release pull request in the tap;
9. waits for `brew test-bot` to build bottles natively on Linux, Apple Silicon
   macOS, and Intel macOS; and
10. dispatches the tap's `brew pr-pull` workflow so it publishes the bottles
    and formula metadata to the default branch and closes the pull request.

The GitHub release and tap update cannot be transactional across repositories.
If formula validation or a bottle build fails, the workflow fails visibly. The
new cask remains available through its established release path, while the
default branch retains the previous formula. Rerunning the failed release job
reuses a matching open formula pull request. No person normally needs to merge
the pull request: the release completes only after `brew pr-pull` publishes it
and closes it. An open pull request therefore indicates a failed or interrupted
release that needs attention.

The tap also runs its own CI on Linux, Apple Silicon macOS, and Intel macOS. It
checks fresh formula installation, switching from the cask to the formula, and
a formula upgrade from the previous tap revision. Users with a matching bottle
install the formula without installing the Go build dependency. Homebrew falls
back to the source build when no matching bottle is available or when the user
requests `--build-from-source`.

Do not add an `xattr` hook or otherwise bypass macOS Gatekeeper. Signing and
notarizing prebuilt macOS archives is separate release work.
