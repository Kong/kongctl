# Homebrew release process

The `Kong/homebrew-kongctl` tap publishes `kongctl` as a source-built formula.
During the cask migration period, it also publishes the deprecated binary cask
so existing cask users continue receiving stable releases.

## Release ownership

GoReleaser builds and publishes release archives, checksums, container images,
and the transitional cask. It does not generate the source-built formula.

After GoReleaser completes a stable release, the release workflow:

1. clones the tap into Homebrew's tap directory;
2. copies the generated cask into the tap;
3. reads the version, commit, and build date from GoReleaser's metadata;
4. downloads the tagged source archive and calculates its SHA-256 checksum;
5. updates the formula with the source and linker metadata;
6. runs Homebrew style and audit checks, exercises the cask-to-formula
   migration, and tests a source installation; and
7. pushes one tap commit only after every check succeeds.

The GitHub release and tap update cannot be transactional across repositories.
If formula validation fails, the workflow fails visibly and leaves the tap at
its previous version. Rerunning the failed release job is safe after correcting
the failure.

The tap also runs its own CI on Linux, Apple Silicon macOS, and Intel macOS. It
checks fresh formula installation, the cask-to-formula transition, and a
formula upgrade from the previous tap revision.

## Cask migration period

The cask remains current during the migration period. Its Homebrew deprecation
warning and caveats direct users to run:

```shell
brew uninstall --cask kongctl
brew install --formula kong/kongctl/kongctl
```

Homebrew schedules the cask for automatic disablement on August 31, 2027, one
year after its deprecation date. Keep publishing current cask releases until
that date unless maintainers announce a different migration deadline.

Do not add an `xattr` hook or otherwise bypass macOS Gatekeeper. Signing and
notarizing prebuilt macOS archives is separate release work.

When the announced migration period ends, remove the `homebrew_casks` block
from `.goreleaser.yml`, remove cask synchronization from the release workflow,
and delete `Casks/kongctl.rb` from the tap. The formula update and validation
steps remain unchanged.
