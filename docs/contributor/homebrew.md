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
   and
7. pushes one tap commit only after every check succeeds.

The GitHub release and tap update cannot be transactional across repositories.
If formula validation fails, the workflow fails visibly and leaves the tap at
its previous version. Rerunning the failed release job is safe after correcting
the failure.

The tap also runs its own CI on Linux, Apple Silicon macOS, and Intel macOS. It
checks fresh formula installation, switching from the cask to the formula, and
a formula upgrade from the previous tap revision.

Do not add an `xattr` hook or otherwise bypass macOS Gatekeeper. Signing and
notarizing prebuilt macOS archives is separate release work.
