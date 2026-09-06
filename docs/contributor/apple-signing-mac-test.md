# Clean-Mac signing acceptance

Owner: a developer with access to a Mac. No Apple private keys are needed.
This checks the browser-download path, complementing native CI installation
tests. Repeat public installation tests after the first signed release.

## Choose and download the test build

1. Use a clean Mac or disposable macOS VM that has not approved this test
   executable. Keep Gatekeeper enabled and connect to the internet. Do not
   reset system-wide security settings or uninstall an existing kongctl.
2. Open a successful `Apple signing validation` run in `Kong/kongctl` Actions.
   Record the run URL and commit. In its Artifacts section, download
   `apple-signed-validation` through Safari or Chrome while signed into GitHub.
   Save the ZIP; disable automatic opening of downloaded archives if necessary.
3. Record its GitHub artifact digest and verify the downloaded outer ZIP:

   ```sh
   shasum -a 256 /path/to/apple-signed-validation.zip
   ```

   The result must match the artifact's SHA-256 digest. Extract the outer ZIP
   using Finder. These artifacts expire after seven days. After merge, obtain
   a fresh set by manually running Apple signing validation on `main`.

## Verify the native archive without changing security settings

Open Terminal in the extracted artifact directory. These commands only inspect
the downloaded files and create a disposable test directory. Run them one at
a time and stop if a checksum or verification command fails:

```sh
sw_vers
uname -m
spctl --status
case "$(uname -m)" in
  arm64) archive=kongctl_darwin_arm64.zip ;;
  x86_64) archive=kongctl_darwin_amd64.zip ;;
  *) echo 'Unsupported Mac architecture'; return 1 ;;
esac
shasum -a 256 --check checksums.txt
mac_test_dir=$(mktemp -d "$PWD/kongctl-mac-test.XXXXXX")
ditto -x -k "$archive" "$mac_test_dir"
cd "$mac_test_dir"
xattr -l ./kongctl
codesign --verify --strict --verbose=2 ./kongctl
codesign -d --verbose=4 ./kongctl 2>&1
codesign --verify --strict --check-notarization -R '=notarized' ./kongctl
./kongctl version --full
```

Expected results:

- All archive checksums match and Gatekeeper reports assessments enabled.
- `com.apple.quarantine` is present before execution. If absent, record the
  browser test as inconclusive; a synthetic attribute test is a separate test.
- Signature verification and the notarization requirement succeed.
- Signing authority is `Developer ID Application: Kong Inc. (FX44YY62GV)`.
  Quill may leave the optional `TeamIdentifier` field unset; CI separately
  verifies the Apple certificate chain and certificate team OU.
- The version command reports this run's snapshot version without an
  unidentified-developer, damaged-app or cannot-verify-malware block.

Do not clear quarantine, disable Gatekeeper, click Open Anyway or use a
right-click Open override. If there is any prompt, capture its exact text and
stop before approving it. A normal first-open confirmation should be recorded
separately from a signing/notarization failure; do not silently count it as
the expected no-prompt result.

Send back the run URL, Mac version/architecture, checksum result, xattr and
codesign output, version output and any prompt screenshot. Do not send keys or
credentials. Leave the disposable test directory intact until results have
been reviewed. No existing installation or security settings are changed.

## First public release

After the coordinated tap/upstream merge and first signed release, repeat the
browser test with the actual release ZIP. On separate clean test installations,
verify the public cask, formula bottle and shell installer. Confirm the bottle
is poured without Go and all three paths preserve the signed release binary.
The validation artifact test does not replace these production checks.
