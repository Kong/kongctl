# Kong Identity administration

These examples require the Kong Identity SDK preview used by this branch and
access to the regional directory administration API.

`tutorial.yaml` creates a directory plus user and agent principals in the
`ki-tutorial` namespace. It requires a free directory quota slot; run it
separately from the other examples. Plan and apply it, inspect the principals,
then repeat the plan to verify no changes. Edit a principal's display name or
metadata and apply again. `delete -f tutorial.yaml` removes the principals
before their directory. For an existing directory, use `principals.yaml`.

Most users can reference an existing directory, commonly named `default`.
Start with `external-default.yaml`: it resolves the directory by its exact
name, without creating or modifying it. An explicit ID is also supported.
KongCTL does not create an implicit default directory or select one silently.
Other Kong Identity resource types are described below as they are added.

## Authorization servers

`auth-servers.yaml` manages an authorization server through the separate
regional auth-server administration API. It requires an explicit `name` and
`audience`. Optional fields are `description`, `signing_algorithm`,
`trusted_origins`, `dcr.default_access_token_duration`, and public `labels`.
The DCR token duration ranges from 60 through 2592000 seconds.

```sh
kongctl plan -f auth-servers.yaml --mode apply > auth-plan.json
kongctl apply --plan auth-plan.json --auto-approve
kongctl get identity auth-servers
kongctl get identity auth-server kongctl-example-issuer -o json
kongctl dump declarative --resources identity.auth_servers
kongctl delete -f auth-servers.yaml --auto-approve
```

Names are unique within the regional organization. KongCTL matches the exact
name after reading every list page and stores ownership and protection in
`KONGCTL-namespace` and `KONGCTL-protected` labels. Auth servers have no
`managed_by` field. Do not set these reserved labels yourself. An absent
protection label means false, following the existing label convention.

PATCH updates preserve omitted fields. An empty description,
`trusted_origins: []`, or `labels: {}` clears that setting. Set protection to
false in a separate operation before changing protected settings or deleting.
Sync prunes only when `identity.auth_servers` is explicitly present; an empty
collection prunes owned, unprotected servers in the source namespace.
External references support `_external.id` or an exact name selector and do
not authorize mutation.

Adopt an existing server before applying a managed declaration with its name:

```sh
kongctl adopt identity auth-server existing-issuer --namespace identity-team
```

Adoption changes only ownership labels and preserves public fields. Existing
ownership requires `--overwrite-namespace`. Dump includes mutable public
settings and namespace/protection, excluding generated issuer and discovery
URLs. This slice does not configure OAuth clients, scopes, claims, or secrets.
Deletion always uses `force=false`, so existing child resources can block it.
The preview public SDK exposes the DCR token duration; additional internal DCR
settings are outside this manifest's schema.

### Scope and claim inspection

Scope and claim inspection is available under an explicitly selected server:

```sh
kongctl get identity auth-server-scopes --auth-server-name existing-issuer
kongctl get identity auth-server-scope read:data \
  --auth-server-name existing-issuer -o yaml
kongctl get identity auth-server-claims --auth-server-name existing-issuer
kongctl get identity auth-server-claim <claim-id> \
  --auth-server-id <server-id> -o json
```

Specify exactly one server name or UUID. Scope names are unique within the
server; claim names can repeat. An ambiguous claim name fails and requires
the claim UUID. Claim values, including `${ uuidv4 }` and JSON-valued strings,
are returned verbatim. These commands inspect configuration; they do not
evaluate token or userinfo behavior. Declarative child support is still in
progress; the auth-server manifest above currently configures only the root.

## Directories and principals

`directories.yaml` manages a directory's configuration. Run this example
separately from `external-default.yaml`. Creating another directory requires
available organization quota. The API requires `allow_all_control_planes:
true` for organizations with a one-directory quota.

```sh
kongctl get identity directories
kongctl get identity directory default -o json
kongctl get identity principals --directory-name default -o json
kongctl get identity principal <principal-id> --directory-name default -o json
kongctl plan -f external-default.yaml
kongctl plan -f directories.yaml --mode apply > plan.json
kongctl apply --plan plan.json --auto-approve
kongctl dump declarative --resources identity.directories
```

Principal inspection requires an explicit directory name or ID. Principal
display names are not unique, so individual reads require the principal UUID.
JSON/YAML omit `active` when the API does not return it; text shows unknown.
`principals.yaml` declares user, agent, and unclassified principals under an
existing directory. A managed directory may also contain principals. Each
principal has independent namespace and protection settings; ownership of the
directory does not authorize changes to its principals.

Principal matching uses the directory ID plus `kongctl-namespace` and
`kongctl-ref` in `managed_by`. The other generated entries are `tool`,
`version`, and `kongctl-protected`. Display names can repeat. Keep refs stable:
renaming a ref creates a different principal and can remove the old one during
sync. Do not supply `managed_by` yourself.

Updates use a fresh GET plus PUT and preserve omitted public fields. Explicit
empty `description`, `labels: {}`, or `metadata: {}` clears those values.
`type` is currently create-only: omission preserves the existing type, while
requesting another type or resetting a classified principal to null fails.
`active` configuration is deferred and is never inferred as false. Track
removal of these temporary limits in the
[compatibility tracker](../../../contributor/kong-identity-compatibility.md).

To adopt an existing principal, select its UUID and persist the desired ref:

```sh
kongctl adopt identity principal <principal-id> --directory-name default \
  --ref jane-doe --namespace identity-team
kongctl plan -f principals.yaml --mode apply
kongctl dump declarative --resources identity.directories \
  --include-child-resources
```

Adoption preserves public fields and protection. A conflicting ref in the same
directory and namespace is rejected. As with directories, taking over existing
ownership requires `--overwrite-namespace`.

Principal sync is scoped to explicitly declared collections and independent
namespaces. Omitting `principals` retains existing principals; `principals: []`
under a directory prunes only owned principals in the source default namespace.
Foreign namespaces and unowned principals remain untouched. An empty grouped
`identity.principals: []` cannot identify a parent and is rejected for sync.
Dump includes principals with `--include-child-resources`, preserves owned refs
and namespace/protection, and omits active. Dumps of unowned principals require
explicit adoption before management. Filter to one directory if multiple
remote resources use the same ref.

To manage an existing directory, explicitly adopt it into your namespace
before applying a managed declaration with the same name:

```sh
kongctl adopt identity directory default --namespace identity-team
```

Adoption preserves its settings, public labels, and protection status. It
replaces `managed_by` with KongCTL's ownership and version metadata. Use
`--overwrite-namespace` only when intentionally taking over existing namespace
ownership. Adoption alone does not alter directory configuration.

Each managed directory requires an explicit API `name` in addition to its
local `ref`. The API name is unique in its organization and region. KongCTL
matches that name and checks ownership locally after reading every list page.
It stores `tool`, `version`, `kongctl-namespace`, and `kongctl-protected` in
`managed_by`. Do not supply `managed_by` in a manifest. Public `labels` remain
independent of ownership metadata.

All mutable API settings are supported: `description`,
`allow_all_control_planes`, `allowed_control_planes`, `ttl_secs`,
`negative_ttl_secs`, `vault_enabled`, and `labels`. Cache TTLs range from 300
through 86400 seconds. Omitted settings are preserved during updates; an
explicit empty description, labels map, or control-plane list clears that
setting. Omitted settings on creation use the documented API defaults.
Updates use a fresh read and full replacement because PATCH cannot update
`description` or `managed_by`.

The preview specification exposes `vault_enabled`, but the US `.com` API
currently accepts `true` and returns `false`. KongCTL reports that mismatch
as an error. Keep it false until Token Vault is available in your environment;
the directory lifecycle scenario verifies false, while unit coverage verifies
that a true request is sent and an ignored value is reported.

`scoped-control-plane.yaml` shows restricted access with a control plane
reference. Use UUIDs or `!ref gateway#id`, including references to external
control planes. New control planes are created before directories that
reference them. Apply each example separately.

Protection follows the usual KongCTL lifecycle. Set `kongctl.protected` to
false and apply that change separately before changing protected settings or
deleting the directory. A protected resource cannot be changed and
unprotected in the same operation.

`sync` only prunes directories when `identity.directories` is explicitly in
scope. To remove every managed, unprotected directory in this namespace:

```yaml
_defaults:
  kongctl:
    namespace: identity-team
identity:
  directories: []
```

Omitting the collection leaves directories alone during sync. External,
foreign-namespace, and unowned directories are not pruned. `delete -f` can
delete explicitly declared managed directories. KongCTL never requests
forced cascading deletion; the API must permit deletion without losing
descendant resources. Credentials, groups, Token Vault children, and other
directory children remain separate follow-up work.
