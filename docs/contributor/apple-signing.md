# Apple signing and notarization rollout

Status: draft validation, not enabled for production releases.

## What this draft tests

The `Apple signing validation` workflow builds both macOS architectures with
GoReleaser 2.13.3, signs with Kong's Developer ID Application certificate, and
submits each executable to Apple using the App Store Connect API key.
GoReleaser's open-source cross-platform notarization implementation uses Quill;
neither GoReleaser Pro nor an Apple ID/app-specific password is needed.

The validation uses the actual kongctl executable and release compiler flags.
It creates snapshot ZIPs, but no GitHub release, tag, container, cask update,
formula update, or bottle upload. Only verified ZIPs and checksums are retained
as Actions artifacts for seven days. These are test builds, not replacements
for existing release assets.

During development the workflow runs on pushes to `task/apple-notarization`.
This is a trusted repository branch, not a pull-request event. Only reviewed,
trusted code should be pushed there: its workflow can access signing secrets.
Remove the temporary push trigger before merging. Afterward use the manual
workflow on main. There is no arbitrary source-ref input.

## Credentials

Repository or selected-repository organization secrets:

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

## Remaining production integration

Do not merge this as a completed all-installation-method signing rollout.

- Move the validated notarization configuration into the stable release path.
  Require all five secrets; never silently skip signing when one is missing.
- Build/sign before generating archives and checksums. Hold GitHub assets in
  a draft release until native macOS signature/notarization checks pass, then
  publish and let the existing Homebrew release automation proceed.
- Ensure recovery mode cannot bypass verification of newly signed releases.
- Exercise both architecture downloads through the cask and `curl | sh`
  installer, checking they preserve the signed executable byte-for-byte.
- Decide how to handle ad-hoc prereleases, which currently use `goreleaser
  build` and their own archive/publication path. Do not give signing secrets to
  their arbitrary `source_ref` checkout. Keep arbitrary-code previews explicitly
  unsigned or add a separately trusted signing process and approval boundary.
- Sign the independently compiled macOS Homebrew bottles too, as below.
- Remove the temporary trusted-branch push trigger before merging.
- Test on a prerelease first. Do not replace existing 1.15.0 assets or bottles.

## Homebrew is a separate signing boundary

The cask, downloaded release ZIP, and installer script all consume release
executables. They can share GoReleaser's signed and notarized output.

The formula currently builds from source. Its bottles contain different
executables, so signing the release ZIPs does **not** sign those bottles.
Preserve the working bottle infrastructure and add a trusted signing stage
before the final bottle checksums, attestations, and GHCR publication:

1. Test/build unprivileged source bottles as today. Never expose Apple secrets
   to ordinary tap PR test-bot jobs.
2. In an explicitly trusted publication job, validate the release PR, exact
   tested SHA, expected artifacts and source version before signing anything.
3. Sign/notarize both macOS bottle executables using the same Kong identity.
   Repackage and regenerate metadata/checksums with Homebrew, rather than
   editing checksums by hand. Leave Linux unchanged.
4. Test pouring the final signed bottles on both architectures. Homebrew
   relocation or re-signing must not invalidate the Developer ID signature.
   Check the installed binary, not just the pre-packaging file.
5. Publish only the verified, signed artifacts and attest those final bytes.
   Update partial-upload recovery to verify the signed metadata as well.

This will require a separate tap PR and either scoped organization secrets or
an approved signing service. Repository secrets in Kong/kongctl are not
automatically available to Kong/homebrew-kongctl. Do not copy or broaden secret
access without administrator approval.

A formula explicitly built from source on a user's machine cannot carry Kong's
Developer ID signature: the user produces new executable bytes and must never
receive Kong's signing key. The supported downloaded macOS distributions must
be signed; local source builds need to remain clearly distinguished.

## References

- [GoReleaser notarization][goreleaser]
- [Apple notarization workflow][apple]
- [GoReleaser 2.13.3 notary implementation][implementation]

[goreleaser]: https://goreleaser.com/customization/sign/notarize/
[apple]: https://developer.apple.com/documentation/Security/customizing-the-notarization-workflow
[implementation]: https://github.com/goreleaser/goreleaser/blob/v2.13.3/internal/pipe/notary/macos.go
