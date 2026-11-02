# Deleting a managed directory

The common examples reference the existing `default` directory externally and
retain it during teardown. This procedure is for deleting a directory that
you already manage and intend to remove, including through scoped sync.

The API's `force=false` does not protect Vault keys, credential providers or
Trusted IdPs from directory deletion. KongCTL therefore requires observable,
empty collections of all three. When `vault_enabled` is false or unknown,
deletion is blocked because the API hides those collections. A 403, missing
API or incomplete response also blocks deletion. KongCTL does not enable
Vault as a side effect of planning or deleting.

Directory retirement also requires a complete Registry Agent reference
inventory. A stored association targeting this directory blocks deletion,
including when its principal has already been deleted. Registry read errors
or unavailable endpoints block retirement. See
[Registry references](registry-references.md) for inspection and explicit
unlinking. Directory ownership grants no authority over Registry links.

## Enable inspection explicitly

First unprotect the directory in a separate metadata-only update if needed,
as described in the [directory overview](README.md). Enabling Vault is another
explicit apply step. For a directory owned in namespace `identity-directory`,
save this as `enable-vault-inspection.yaml`:

```yaml
_defaults:
  kongctl:
    namespace: identity-directory
identity:
  directories:
  - ref: default-directory
    name: default
    vault:
      enabled: true
```

Use the actual managed directory name and namespace. Applying this to an
unowned directory is rejected; a name lookup does not adopt it.

```sh
kongctl plan --mode apply -f enable-vault-inspection.yaml
kongctl apply -f enable-vault-inspection.yaml
kongctl get identity directory default -o json
```

Check that the response retains `vault_enabled: true`. If the deployment does
not support this setting, or the token cannot inspect Vault resources,
directory deletion cannot proceed through the declarative workflow.

These checks apply to declarative deletion. `kongctl api` calls execute the
requested HTTP operation directly, including the API's native cascade behavior.

## Inspect and retire dependencies

Use the returned directory UUID:

```sh
export DIRECTORY_ID=REPLACE_WITH_DIRECTORY_UUID
kongctl api get "/v2/directories/$DIRECTORY_ID/vault/keys" -o json
kongctl api get "/v2/directories/$DIRECTORY_ID/vault/providers" -o json
kongctl get identity trusted-idps --directory-id "$DIRECTORY_ID" -o json
```

The raw API collections can be paginated; follow their returned next links
when inspecting existing records. Retire dependencies explicitly through
their supported lifecycle, including provider credentials and consent where
applicable. Do not infer that a visible resource belongs to a tool or that
deleting a provider proves runtime credential revocation. Directory ownership
does not silently enroll these resources into writable declarations.

KongCTL's deletion guard only accepts an empty, terminal, unfiltered response
for each collection. It does not delete their contents for you. Principals
and groups also require explicit teardown; their own credential, identity,
membership, ownership and protection checks still apply.

## Plan and execute deletion

```sh
kongctl plan --mode delete -f enable-vault-inspection.yaml \
  --output-file directory-delete-plan.json
kongctl diff --plan directory-delete-plan.json
kongctl delete --plan directory-delete-plan.json
```

Direct `delete -f` and scoped sync use the same guard. Saved execution checks
fresh directory settings and all three collections again. Disabling Vault
or attaching a dependency after planning blocks the saved deletion. Keeping
`vault_enabled: true` in a saved plan does not authorize enabling it during
deletion; an explicit update must happen first.

Saved execution also observes Registry references again. A link added after
planning blocks deletion until it is explicitly removed.

These client observations cannot make the backend operation atomic. A new
dependency attached between the final observation and DELETE can still be
affected by the API's cascade behavior. Q37 requests a conditional deletion
contract and inspection while Vault is disabled. See the
[API questions](../../../kong-identity-api-questions.md).
