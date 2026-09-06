# Apple signing and notarization rollout

Status: automated validation complete; first production rollout pending.

## What validation tests

The `Apple signing validation` workflow builds both macOS architectures with
GoReleaser 2.13.3, signs with Kong's Developer ID Application certificate, and
submits each executable to Apple using the App Store Connect API key.
GoReleaser's open-source cross-platform notarization implementation uses Quill;
neither GoReleaser Pro nor an Apple ID/app-specific password is needed.

The validation uses the actual kongctl executable and release compiler flags.
It creates snapshot ZIPs, but no GitHub release, tag, container, cask update,
formula update, or registry upload. It also packages Linux and macOS bottles
from those upstream executables and tests the production metadata merger.
Verified ZIPs, bottles and metadata are retained as Actions artifacts for seven
days. These are test builds, not replacements for existing release assets.

Run this workflow manually on `main` in `Kong/kongctl`. Other branches and
repositories are rejected before signing jobs start. It has no push or
pull-request trigger and no arbitrary source-ref input. Apple credentials
remain confined to trusted signing steps in the upstream repository.

## Credentials

Keep these secrets available only to `Kong/kongctl`, not the tap repository:

| Secret | Contents |
| --- | --- |
| `APPLE_SIGNING_CERTIFICATE_P12_BASE64` | Base64 of exported Developer ID Application certificate and private key (`.p12`) |
| `APPLE_SIGNING_CERTIFICATE_PASSWORD` | Password chosen when exporting that `.p12` |
| `APPLE_NOTARY_API_PRIVATE_KEY` | Complete raw PEM contents of the downloaded `.p8` file, including header/footer |
| `APPLE_NOTARY_API_KEY_ID` | App Store Connect API key ID |
| `APPLE_NOTARY_API_ISSUER_ID` | App Store Connect team API issuer UUID |

Variables:

| Variable | Contents |
| --- | --- |
| `APPLE_TEAM_ID` | Ten-character Apple Developer team ID |
| `APPLE_SIGNING_IDENTITY` | Full issued certificate name, `Developer ID Application: ... (TEAMID)` |

Do not change the `.p8` secret to base64. The workflow writes its raw PEM into
a mode-600 temporary file and passes the path to GoReleaser. The file is never
uploaded and is removed even on failure. Certificate material is not imported
into a persistent Keychain by this cross-platform signer.

Repository secrets override same-named organization secrets. Environment
secrets override repository secrets in jobs using that environment. Moving
credentials to the organization requires selected-repository access and removal
of stale repository overrides. No secret values belong in PRs, logs, or docs.

## Acceptance checks

1. Required credentials and variables must be nonempty. `notarytool history`
   must authenticate successfully; its output is not published.
2. GoReleaser must sign and submit both architectures, waiting up to 20 minutes
   per executable. A pending first submission may require investigation and
   more time at Apple; do not remove the acceptance gate to work around it.
3. Verify the executable extracted from each final ZIP: valid signature,
   expected certificate identity and team, hardened runtime, secure timestamp,
   and Apple's online notarization requirement.
4. Fresh ARM and Intel runners independently verify downloaded artifacts and
   execute the native binary with a quarantine attribute present.
   They also exercise the actual installer against local signed ZIP fixtures,
   plus an isolated Homebrew test tap for bottle creation/pouring and cask
   installation. Each path must preserve the executable byte-for-byte.
   These fixtures never update the production tap or publish a release.
5. Before production enablement, also test a browser download on a clean Mac.
   CI's synthetic quarantine test does not reproduce every Finder/browser or
   managed-device policy.
   Follow the [clean-Mac acceptance procedure](apple-signing-mac-test.md).

GoReleaser 2.13.3 can log a notarization timeout and still return success.
Therefore its exit code alone is insufficient. `verify-apple-binary.sh` checks
notarization with Apple's `codesign` tool before artifacts are made available.
Bare executables and ZIPs cannot have stapled notarization tickets: initial
assessment needs access to Apple's service. Signing is not a promise of no
prompts on every offline or organization-managed Mac.

Quill in the pinned GoReleaser version leaves the CodeDirectory TeamIdentifier
unset. Apple accepted both such binaries in the first validation run. Verify
team ownership with the Apple certificate chain and leaf certificate's
`subject.OU` code-signing requirement, not by requiring that optional display
field. A populated but unexpected TeamIdentifier is still rejected.

## Stable release integration

The production GoReleaser configuration now includes the same notarization
settings as the validation configuration. The `Release` workflow stages the
following sequence; it has not yet been exercised as a real release:

1. Full releases and recovery must run from `Kong/kongctl` main.
2. Require all five Apple secrets and both variables. Create a temporary API
   key file, then sign/notarize before producing ZIPs and checksums.
3. Upload GitHub assets into a **draft** release. The existing GoReleaser
   container publication still occurs here; Linux container images do not
   depend on Apple signing. This is not an atomic multi-registry transaction.
4. Fresh Intel and ARM Macs download the draft assets, verify checksums,
   signatures and notarization, execute their native binary with quarantine,
   and exercise the installer against those same assets. No Apple private
   credentials are available in these jobs.
5. Each Mac records the release ID and exact asset identities. Both receipts
   must match each other and the current GitHub assets before publication.
   Replacing, deleting, or uploading an asset invalidates that approval.
6. Only then publish the release and update the cask. Package the same upstream
   executables as bottles, verify local and public pouring, and publish the
   completed formula metadata through a tap PR. See [Homebrew](homebrew.md).

The publication-policy test uses an offline GitHub API double; it never
publishes a test release. Mac validation also runs this test with real signed
ZIPs and native verification. Fixture Linux/Windows assets are placeholders,
not tests of those executables or the actual GitHub publication API.

### Failure and recovery

- Missing credentials, rejected/pending notarization, or failed native checks
  must stop publication. Do not remove the gates or manually publish a draft.
- If Apple is still processing a submission, investigate its status, then
  retry the failed verification jobs in the **original run**. Successful
  signing/build jobs need not be repeated. Receipts last seven days.
- The `recovery_tag` input still accepts only already-published stable
  releases. It now rechecks full macOS assets before proceeding, so it cannot
  bypass notarization. Older unsigned releases cannot pass this new gate.
- A changed asset requires both native gates to run again. A missing/expired
  receipt also requires fresh verification. Do not reuse receipts across runs.
- Do not start a normal new release or rerun every job merely to recover a
  draft: version computation can select a new tag. Recovery after a failed
  build, expired run, or partial Homebrew publication still needs maintainer
  investigation; this implementation does not introduce a rebuild/resume
  dispatcher.

Arbitrary-ref ad-hoc prereleases remain explicitly unsigned. Their workflow
uses `goreleaser build`, not the notarization/release pipeline, and its notes
now disclose this. No Apple secrets were added to that workflow. Signing
arbitrary previews needs a separate trusted approval boundary.

## Remaining production integration

Do not merge this as a completed all-installation-method signing rollout.

- Exercise the staged production build/draft/download/publish integration
  against GitHub before approving a stable rollout.
- Exercise both architecture downloads through the cask and `curl | sh`
  installer, checking they preserve the signed executable byte-for-byte.
- If signed ad-hoc previews are required, add a separately trusted signing
  process; do not expose credentials to their arbitrary source checkout.
- Complete the production packaging, public registry and tap integration
  checks. No second compilation or signing step is needed for bottles.
- Validation workflows are manual-only and restricted to upstream `main`.
- Complete signed snapshot validation first, then supervise a new stable
  release. Ad-hoc prereleases remain unsigned and cannot validate this path.
  Do not replace existing 1.15.0 assets or bottles.

## Homebrew packages the same signed executable

The cask, downloaded release ZIP, and installer script all consume release
executables. They can share GoReleaser's signed and notarized output.

The revised formula no longer compiles Go. It installs the upstream release
ZIP, so the cask, ZIP fallback and bottles all consume the same signed bytes.
The bottle runners need only public identity/team variables for verification,
not private signing credentials. Both local pouring and the final public
registry installation must preserve those bytes and their Apple approval.

All packaging and publication jobs live in `Kong/kongctl`. The tap only holds
definitions, checks installation, and merges the finished metadata. Do not
copy Apple secrets into the tap or grant them to ordinary PR test jobs.

Homebrew's `--build-from-source` option selects the formula's install recipe;
in this prebuilt formula that recipe still installs the upstream signed ZIP.
It does not compile Go. Someone manually building the project source produces
a separate unsigned executable and never receives Kong's signing key.

## References

- [GoReleaser notarization][goreleaser]
- [Apple notarization workflow][apple]
- [GoReleaser 2.13.3 notary implementation][implementation]

[goreleaser]: https://goreleaser.com/customization/sign/notarize/
[apple]: https://developer.apple.com/documentation/Security/customizing-the-notarization-workflow
[implementation]: https://github.com/goreleaser/goreleaser/blob/v2.13.3/internal/pipe/notary/macos.go
