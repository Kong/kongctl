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
supported. Writers explicitly preserve the fresh current method; this remains valid
against the corrected development API. Actual SDK wire tests cover empty, omitted and
populated fields. These patches skip SDKs without auth-server clients.

Directory, principal, principal-group, principal-identity, principal API-key,
basic-auth, password, group, group key, group membership, Trusted IdP,
associated-principal, Vault key, Vault provider, provider credential,
principal Vault credential and
credential-provider template lists use bracketed
`page[size]` and `page[after]` query names. The shared API parameter omits
`style: deepObject`, so generated form/explode encoding sends flat keys that
the deployed API ignores (Q22). Targeted request-tag replacements correct
these seventeen operations without changing unrelated SDK resources. Provider
credential, principal Vault-credential and group-key lists use the same
correction; an unfiltered empty-dependency guard does not require page arguments.
A wire regression verifies the bracketed names and cursor encoding; it fails
before the patches. Skip SDKs without these operations. Remove replacements
when upstream generation preserves the documented/deployed cursor shape.

SDK 0.83 includes a dedicated writable nullable expiration schema for the six
Identity credential requests. No preview-spec correction is needed for that
schema separation. Generated time pointers still cannot distinguish omitted
expiration from explicit null, so the targeted Go model corrections below
remain necessary. SDK 0.83 also splits the key-creation variants into separate
model files; the replacements follow those files and preserve their generated
constant discriminators.

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

Portal AI settings patches preserve omitted MCP write enablement. Updating
one feature must not reset unrelated optional settings. Required top-level and
feature enablement retain their API defaults. SDK v0.86.1 removes Portal Agent
and AI Search schemas; the obsolete Portal Agent patches and tests are removed
with maintainer approval. Earlier bundle revisions cover those older schemas.

Portal and organization team patches preserve omitted membership management.
A PATCH changing a team's name must not reset `konnect_managed` to `false`.
Explicit `false` and `true` remain supported.

AI Gateway update patches preserve omitted `runtime_auto_upgrade`. Updating
ownership labels or other gateway fields must not enable runtime auto-upgrade.
Explicit `false` and `true` remain supported. The overlay only changes the
update request; creation and response defaults remain unchanged. Actual SDK
wire tests exercise omitted and explicit values through struct construction
and JSON decoding. Adoption also explicitly preserves observed runtime values.

AI Gateway pgvector model patches preserve omitted connection settings,
including SSL fields. A referenced datastore supplies these settings; inline
SDK defaults must not turn omitted fields into explicit model overrides.
Explicit values, including values equal to API defaults, remain supported.
The planner separately accounts for datastore-expanded response fields and
inline API defaults.

AI Gateway OpenID Connect auth strategies patch only
`upstream_access_token_header` in the generated request and response config
structs. That field uses `optionalnullable.OptionalNullable[string]` with
`omitempty` and no serializer default. Omission stays omitted, explicit null
stays null, and explicit strings (including `authorization:bearer`) are kept.
Its getter still returns `*string`; inspect the field's `IsSet`/`IsNull` methods
when presence matters. Go constructors assigning this field use
`optionalnullable.From` instead of a bare pointer.

All other OIDC fields retain their generated types, getters, JSON tags, and
SDK defaults. Nested models and unknown-field handling remain unchanged. The
patch does not change the global SDK serializer. Existing `post_generation`
entries replace the import, the two field declarations, their getters, and
the two documentation type cells. The shared field replacement declares
`count: 2`; all other replacements require exactly one match by default.
Missing, extra, or partially patched matches fail before any files are written.

Wire regressions compare all non-header output against fixtures recorded with
the unmodified v0.84.0 OIDC models. They cover minimal configs, existing access
control/principal/header mappings, nested Redis/JIT/token-exchange settings,
and explicit values through create, update, and response serialization. Each
case also checks omitted/null/default/custom header values. The baseline
fixture intentionally includes existing SDK defaults and empty-array omission;
only `upstream_access_token_header` is excluded from the baseline comparison.

Review changed generated text before updating the literal replacements when
upgrading the SDK. Remove this correction when upstream preserves this field's
presence without inventing a default.

Typed construction tests use `kongctl_sdk_patched` because Speakeasy compiles
an intermediate unpatched SDK before the post-generation hook runs.
`make generate.sdk` runs them automatically through `make test.kongctl-patches`
immediately after the patch hook. Normal wire tests also run without the tag.

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
CGO_ENABLED=0 go test -tags=kongctl_sdk_patched .
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
kongctl's `go.mod` requirement to that commit's Go pseudo-version, refresh
`go.sum` and vendoring, and run kongctl's local gates and relevant live scenarios
before pushing kongctl. The SDK branch is consumed by commit; it is not released.
Require the public patched version directly rather than selecting it only with
`replace`: downstream Go extension modules do not inherit replacement directives.
Run `go mod tidy` in `docs/examples/extensions/go` and `make test-extension-go`
to verify the example inherits the patched SDK and builds with committed metadata.

Run the patch-tool checks with:

```sh
python3 -m unittest discover -s tools/sdk-patches -p '*_test.py'
```

Live scenarios can create or delete Konnect resources. Use the configured test
profile and scope cleanup to the scenario's resources.

Development Identity APIs accept `managed_by` when importing principal/group
keys, and support explicit null for key names on PATCH. Targeted model patches
add the imported-key map and use `optionalnullable.OptionalNullable[string]`
for update names. Wire tests verify imported metadata, null-key removal, and
omitted/null/scalar names. The client retains its existing ownership convention;
API mutability alone does not authorize adoption or ownership transfer.
