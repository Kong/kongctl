# Basic-auth username records

A basic-auth credential has two levels: a username record attached to a
directory principal, and up to two password records under that username.
The username is unique within the directory. Passwords have generated IDs
and expiration metadata; they have no name. Expired records still occupy
a password slot in the tested API.

`basic-auth.yaml` declares an agent principal and a username record under
the external `default` directory. Bootstrap a missing default directory with
`directories.yaml` first, as described in the [overview](README.md).
Apply this example separately from the other principal examples:

```sh
kongctl plan --mode apply -f basic-auth.yaml --output-file basic-plan.json
kongctl diff --plan basic-plan.json
kongctl apply --plan basic-plan.json --auto-approve
kongctl get identity principals --directory-name default
```

This creates a username without a password. Password creation, capture and
rotation use the [explicit password commands](passwords.md) and are not
exposed by these declarations. Runtime authentication needs an integration
context (Q30); this example does not establish authentication success.

Use the returned principal UUID to inspect the credential:

```sh
kongctl get identity principal-basic-auths \
  --directory-name default --principal-id PRINCIPAL_UUID --page-size 1
kongctl get identity principal-basic-auth \
  --directory-name default --principal-id PRINCIPAL_UUID \
  --username kongctl-example-order-worker -o yaml
kongctl get identity principal-basic-auth-passwords \
  --directory-name default --principal-id PRINCIPAL_UUID \
  --username kongctl-example-order-worker
```

The username record has four generated `managed_by` entries: `tool`,
`version`, `kongctl-namespace` and `kongctl-protected`. Do not supply that
map yourself. Ownership is independent of the principal/directory: an
external or protected parent does not grant ownership over its credentials.
Principal runtime `metadata` and public labels are separate API fields.

## Matching, protection and sync

Required `username` accepts 1-255 characters and is the remote match key.
`ref` stays local: changing only it does not recreate a username. Changing
the username declares a different resource. Apply preserves the previous
username; scoped sync can remove it only when unprotected and password-free.
Changing the parent does not automatically move a credential between
principals; the API enforces directory-wide username uniqueness.

Set the username's `kongctl.protected: true`, plan and apply to protect it.
Set it back to false in a separate update before deletion. These metadata
updates use fresh GET/full PUT/readback without sending any password value.
The tested API preserves password IDs through username-record PUT; runtime
credential continuity still needs Q30's integration setup.

Nested `principals[].basic_auths` and grouped `identity.basic_auths` are
supported. Grouped declarations require `principal: LOCAL_PRINCIPAL_REF`.
Omitting the collection preserves credentials. An explicit empty collection
selects only its source namespace for sync; other namespaces and unowned
credentials are retained. Use apply for partial collections. Parent sync
scope is separate, so keep a retained principal declared or external.

Saved plans repeat identity, ownership and protection checks before writing.
Username deletion refuses attached passwords, including expired ones.
Principal deletion refuses API keys and requires explicit authorized
username/identity/membership deletions before its fresh empty checks. Parent
ownership never authorizes credential retirement. Observation failures block
deletion; client checks cannot prevent concurrent attachment atomically.

## Adoption, external references and dump

Adopt an unowned username before managing it:

```sh
kongctl adopt identity principal-basic-auth \
  --directory-name default --principal-id PRINCIPAL_UUID \
  --username kongctl-example-order-worker --namespace identity-credentials
```

Existing ownership requires `--overwrite-namespace`. Adoption preserves the
username and protection flag, replacing the management map through full PUT.
It does not adopt or rotate password children. Positional selectors are UUIDs;
`--username` always means the exact username, even if it resembles a UUID.
Read commands also support `list`, explicit `konnect`, JSON/YAML and text
tables for both detail and lists. Password reads never display material.

An external username declaration supports `_external.id` or
`_external.selector.matchFields.username` under a declared principal. It must
exist in that selected principal; external-only collections do not authorize
sync pruning. External principals themselves require explicit UUIDs.

```sh
kongctl dump declarative --resources identity.directories \
  --filter-name default --include-child-resources > identity-dump.yaml
kongctl delete -f basic-auth.yaml --auto-approve
```

Dump uses generated IDs as local username refs and translates ownership into
`kongctl`. Unowned usernames stay external. Passwords and secret material are
excluded: restoring a username does not restore its passwords. The delete
command removes the declared username and principal only if all descendant
checks pass, retaining the external default directory.
An unmanaged directory also exports as an external name lookup, so dump/reload
does not require adopting the platform-provided default.

Password PATCH ignores management-map changes and clears omitted expiration.
PUT requires secret material and expiration; metadata-only adoption has no
verified preserving request (Q35). The preview SDK preserves omitted, dated
and null expiration. Explicit password commands capture creation values and
preserve existing metadata on updates; they do not enroll passwords into
declarative ownership. See the
[API questions](../../../kong-identity-api-questions.md),
[compatibility tracker](../../../contributor/kong-identity-compatibility.md)
and [metadata ledger](../../../contributor/kong-identity-metadata-storage.md).
