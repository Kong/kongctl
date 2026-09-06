# Homebrew release process

## Ownership

`Kong/kongctl` owns all executable artifacts: GoReleaser compilation, Apple
signing/notarization, release ZIPs, Homebrew bottle packaging, GHCR publication,
and final download verification. Apple credentials stay only in this repository.

`Kong/homebrew-kongctl` publishes the formula and cask definitions. It tests
installation and merges verified release metadata; it neither compiles nor
signs executables and has no registry-write role.

The formula and cask both install the platform's upstream release executable.
The formula has no Go dependency. Bottles add Homebrew's package layout and
metadata around those same bytes; creating one does not compile the Go source
again. Homebrew still calls the ZIP fallback `--build-from-source`, but this
recipe only installs the downloaded binary.

## Stable release sequence

1. GoReleaser compiles the release targets and signs/notarizes both macOS
   executables before producing ZIPs and checksums.
2. The ZIPs remain in a GitHub draft until fresh Intel and ARM runners verify
   their signatures, identity, timestamp, notarization and installer behavior.
3. Publish the approved GitHub release.
4. Independently validate and push the generated cask to tap main, preserving
   its existing direct publication path. A later bottle failure does not hold
   back cask users.
5. In this repository, Linux x86-64, macOS Intel and macOS ARM runners install
   the upstream executable into Homebrew's layout and run `brew bottle`.
   Each runner pours its local bottle and compares the installed executable
   byte-for-byte with the release ZIP. Macs also recheck Apple notarization.
6. Validate all three metadata files, hashes, relocatability, versions and
   identical formula recipes. Merge metadata using Homebrew's own tooling.
7. Publish the bottles to the public `ghcr.io/kong/kongctl/kongctl` package
   using `brew pr-upload --upload-only`, then attest the final bottle files.
8. Fresh runners install those public bottles without registry credentials,
   verify that no Homebrew Go dependency appears, and compare the installed
   executable against the actual upstream download.
9. Open a formula-only `release/kongctl-VERSION` PR in the tap. The formula
   already contains the published bottle URLs/checksums.
10. Wait for tap installation checks, dispatch its guarded metadata publisher,
    and verify that the exact formula was merged. No routine manual PR merge
    is required. The publisher checks the expected head SHA and file scope.

The public registry must be readable anonymously. Its existing package must
grant `Kong/kongctl` Actions write access; that is a GitHub package setting,
not permission to copy Apple credentials into the tap. The upstream workflow
uses its own `GITHUB_TOKEN` for packages and attestations. The existing
`TAP_GITHUB_TOKEN` remains scoped to tap contents, pull requests and Actions
dispatch. No new cross-repository Apple signing service or artifact handoff
is required.

## Validation and rollout

The manual Apple validation workflow also creates and pours snapshot bottles
on all three platforms and merges their metadata, without publishing anything
to GHCR or the tap. It uses the production renderer and packaging scripts.
Both validation workflows are manual-only and restricted to upstream `main`.
Run Apple signing validation first on main; packaging-only validation can
then reuse those artifacts. Pre-merge branch artifacts are not selected after
a squash merge because their producer commit is not an ancestor of main.

The packaging-only validation workflow can reuse a previous run's verified
executables when Go source, embedded assets, module dependencies and the build
recipe are unchanged. It records the producer SHA/run, re-verifies native
signatures, and never receives signing credentials or publishes packages.
This avoids recompilation/notarization for packaging-only test iterations.

Confirm package Actions write access in the GitHub package settings before
rollout. Do not use empty uploads as a recurring permission probe: GHCR accepts
upload initiation but rejects cancellation with HTTP 405. The first controlled
release must still exercise actual bottle publication and anonymous downloads.

Coordinate [kongctl #2078] and [tap #15]. Merge the tap PR first, then the
upstream PR, without dispatching a release between them. The new tap publisher
deliberately rejects old source-building release PRs. Existing installations
remain available during this interval.
The upstream workflow checks the tap's release-protocol marker before creating
a new tag, preventing it from accidentally dispatching the old bottle builder.

The tap transition preserves the 1.15.0 bottle block and cask exactly. It does
not replace old assets or retroactively sign them. Only a new release built by
the signed pipeline gets the new signed distribution artifacts. Before that
release, complete clean-Mac browser acceptance and confirm registry write
access. Then verify the real public cask, formula and script installer.

Users retain the existing commands:

```sh
brew install --formula kong/kongctl/kongctl
brew install --cask kong/kongctl/kongctl
```

These are alternatives, not simultaneous installations: uninstall the cask
before switching to the formula, since both expose the `kongctl` executable.
When no compatible bottle exists, the formula installs the supported platform's
upstream ZIP without Go. Unsupported binary platforms still cannot install it
through this formula; building the source manually remains a separate option.

## Failure and retry

GitHub releases, the cask, registry packages and formula updates cannot form
one transaction. If a later step fails, already-published artifacts remain
available and the previous formula remains on tap main.

- Retry failed jobs in the original release run, reusing its verified bottle
  artifacts. Artifacts expire after seven days.
- A completed registry upload is reused only if its manifest contains exactly
  the expected three bottle digests and every blob is anonymously downloadable
  with its expected SHA-256.
- A conflicting or partial existing manifest fails closed. Do not overwrite
  published bottles, use `--keep-old` blindly, or regenerate bytes and pretend
  that they match the old upload. Investigate before planning a new version or
  an explicitly reviewed bottle rebuild.
- A matching open metadata PR is reused. A failed tap check leaves a visible
  PR and failed upstream release job, not a silently successful release.
- `recovery_tag` alone verifies already-published stable releases; it is not
  a bottle rebuild or general resume command. For a signed draft stopped
  before publication, also supply the original `recovery_run_id`. This
  recovers that run's generated cask metadata and finishes the normal bottle
  and tap jobs after fresh native verification. It never recompiles or
  replaces the upstream executables.
- Legacy unsigned macOS releases cannot pass the new Apple verification gate.
  Do not replace their assets to work around that restriction.

See [Apple signing](apple-signing.md) for the native verification requirements,
draft-release recovery and explicitly unsigned arbitrary-ref previews.

[kongctl #2078]: https://github.com/Kong/kongctl/pull/2078
[tap #15]: https://github.com/Kong/homebrew-kongctl/pull/15
