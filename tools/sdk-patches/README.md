# SDK patches owned by kongctl

This directory is the source of truth for kongctl's SDK compatibility patches.
Keep changes here on kongctl's normal development branches and merge them into
`main`. SDK patch branches contain a generated snapshot, not an independent
copy to maintain. No patches run during ordinary kongctl builds.

`manifest.json` lists the OpenAPI overlays, post-generation replacements, and
regression tests. The overlays remove automatic defaults from optional
request fields. The targeted `identity_realms` replacement changes `omitempty`
to `omitzero`, preserving explicit empty arrays while still omitting unset
arrays. It is a generated-code patch, not a Speakeasy OpenAPI extension. Newer SDKs
use flexible auth configs that preserve these arrays in additional properties;
the legacy config-file patch is skipped when that file is absent. The wire
regression tests run against both config shapes.

Auth-server PATCH preserves explicit empty `trusted_origins` arrays with
`omitzero`, while nil arrays remain omitted. The patch skips SDK versions
without auth-server management and checks actual wire presence when available.
The same rule preserves explicit empty claim `include_in_scopes` arrays in
PATCH requests. Both patches are optional on SDKs without those APIs.

OAuth client PATCH has the same nil-versus-empty requirement for grant types,
response types, redirect URIs and allowed scopes. A targeted request patch
also removes the automatic endpoint-authentication-method default. An omitted
method must stay omitted in the serialized request; explicit methods remain
supported. This does not fix the deployed API's omission behavior (Q15).
Writers must still preserve the fresh current method explicitly until that
API behavior is resolved. Actual SDK wire tests cover empty, omitted and
populated fields. These patches skip SDKs without auth-server clients.

Directory, principal, principal-identity, principal API-key, basic-auth,
password, group, group membership, Trusted IdP and associated-principal lists
use bracketed
`page[size]` and `page[after]` query names. The shared API parameter omits
`style: deepObject`, so generated form/explode encoding sends flat keys that
the deployed API ignores (Q22). Targeted request-tag replacements correct
these ten operations without changing unrelated SDK resources. A wire
regression verifies the bracketed names and cursor encoding; it fails before
the patches. Skip SDKs without these operations. Remove the replacements when
upstream generation preserves the documented/deployed cursor shape.

Identity credential expiration requires a separate preview-spec correction:
the combined specification merges a writable credential `ExpiresAt` definition
with a read-only IAM timestamp. The preview preparation script gives the six
credential request schemas a dedicated writable nullable date-time reference
(Q36). It leaves the shared IAM response schema unchanged.

For the five optional request fields, targeted replacements use the SDK's
existing `optionalnullable.OptionalNullable[time.Time]` type. The zero value
omits expiration, `optionalnullable.From(&expiration)` supplies a timestamp,
and `optionalnullable.From[time.Time](nil)` sends explicit null. The required
password replacement field stays `*time.Time` with no `omitempty`; nil sends
the required null property. This does not enable nullable wrappers globally
or change existing public SDK fields. Generated examples are updated within
their affected operation sections. Wire regressions cover all three states
for principal/group generated/imported keys and password create/update/replace.
Remove these replacements only when upstream generation preserves those
states. Backend metadata and expiration omission behavior (Q28/Q35) is separate.

Portal customization patches preserve omitted theme names and Try It UI
audiences. Explicit audience values remain supported; an unset audience must
not become `all` in a PATCH request.

Portal and organization team patches preserve omitted membership management.
A PATCH changing a team's name must not reset `konnect_managed` to `false`.
Explicit `false` and `true` remain supported.

The internal-only `CreatePortal.create_default_content` field also stays
omitted when unset. Its tag replacement skips public SDKs where the field is
absent, but fails on unexpected tags or types when the field is present.
The same rule preserves omitted internal `spec_format` query parameters on
API-version create, update, and fetch requests. Explicit JSON/YAML selections
remain supported.

## Prepare and generate a patched SDK

Use a separate, clean SDK checkout based on the release being upgraded. Install
Python 3.9+, mise, and the SDK generation prerequisites. Commit this bundle in
kongctl first so the SDK records an exact source revision.

From kongctl:

```sh
python3 tools/sdk-patches/apply.py prepare /absolute/path/to/sdk-konnect-go
```

The command copies the bundle into the SDK's `kongctl-patches/` directory,
records its kongctl commit and file hashes, registers the overlays in the
Speakeasy workflow, installs regression tests, and adds a post-generation hook
to the SDK Makefile. Existing upstream overlays and fixes remain in place.

From the prepared SDK checkout:

```sh
mise trust
mise install
CGO_ENABLED=0 GOFLAGS=-buildvcs=false mise exec -- make generate.sdk
CGO_ENABLED=0 go test . ./models/... ./internal/... ./pkg/...
make test.unit
```

Generation applies the overlays before generating Go code, then runs the
targeted replacements after upstream post-generation fixes. Replacements fail
if the expected field changes; they also accept an already patched field.
The Speakeasy version is pinned in the manifest. Generation must use that
version rather than whichever executable happens to be on the shell's PATH.

In a linked SDK worktree, Speakeasy may require fetching the baseline generation
snapshot recorded in the release's workflow lockfile before generation.

Commit the snapshot, workflow changes, generated SDK code, and regression tests
to `patch/v<VERSION>/targeted-patch-defaults`, then push the SDK branch. Update
kongctl's `go.mod` replacement to that commit's Go pseudo-version, refresh
`go.sum` and vendoring, and run kongctl's local gates and relevant live scenarios
before pushing kongctl. The SDK branch is consumed by commit; it is not released.

Run the patch-tool checks with:

```sh
python3 -m unittest discover -s tools/sdk-patches -p '*_test.py'
```

Live scenarios can create or delete Konnect resources. Use the configured test
profile and scope cleanup to the scenario's resources.
