# SDK patches owned by kongctl

This directory is the source of truth for kongctl's SDK compatibility patches.
Keep changes here on kongctl's normal development branches and merge them into
`main`. SDK patch branches contain a generated snapshot, not an independent
copy to maintain. No patches run during ordinary kongctl builds.

`manifest.json` lists the OpenAPI overlays, post-generation replacements, and
regression tests. The overlays remove automatic defaults from optional
request fields. The targeted `identity_realms` replacement changes `omitempty`
to `omitzero`, preserving explicit empty arrays while still omitting unset
arrays. It is a generated-code patch, not a Speakeasy OpenAPI extension.

Portal customization patches preserve omitted theme names and Try It UI
audiences. Explicit audience values remain supported; an unset audience must
not become `all` in a PATCH request.

Portal and organization team patches preserve omitted membership management.
A PATCH changing a team's name must not reset `konnect_managed` to `false`.
Explicit `false` and `true` remain supported.

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
