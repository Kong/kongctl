# Kong Identity administration

These examples require the Kong Identity SDK preview used by this branch and
access to the regional directory administration API.

[OAuth client declarations and secret rotation](auth-server-clients.md) covers
`auth-server-clients.yaml`, initial private capture, lifecycle, reads, adoption
and explicit secret rotation. Client ownership uses labels independently of
the server.

[Principal identity declarations and inspection](principal-identities.md)
covers `principal-identities.yaml`, OIDC, OAuth-client, Gateway-consumer and
custom associations, independent managed_by ownership, scoped sync and adoption.
Workspace configuration remains deferred under Q25. #1416 stays In Progress
until merge.

[Principal API-key inspection](principal-api-keys.md) covers scoped get/list,
public credential metadata and the API contracts still preventing declarative
ownership updates and adoption. It does not expose credential material.

[Group and direct membership inspection](groups.md) covers group/team get/list,
UUID/name selectors and membership-entry IDs. Declarative writes remain in
progress; inspection alone does not grant authority over imported teams.

The demos center on a directory named `default`, whether platform-provided or
bootstrapped with KongCTL. First inspect the organization:

```sh
kongctl get identity directories
```

If `default` exists, use `external-default.yaml` to resolve it through
`_external.selector.matchFields.name: default`. No adoption is required. The
principal/client tutorials retain that external reference: they do not change
the directory's settings or ownership, or delete it during cleanup.

If `default` is missing, `directories.yaml` bootstraps it declaratively with
the observed new-organization settings: unrestricted control-plane access,
empty allowlist/labels/description, 600-second positive and negative TTLs, and
Vault disabled. KongCTL adds its four ownership entries in `managed_by` and
protects the directory from KongCTL updates/deletion. Bootstrap is a separate
step:

```sh
kongctl plan -f directories.yaml --mode apply --output-file directory-plan.json
kongctl diff --plan directory-plan.json
kongctl apply --plan directory-plan.json --auto-approve
kongctl get identity directory default -o json
```

Then run the same principal/client tutorials. Their external lookup can resolve
a KongCTL-managed directory too. Directory ownership uses `identity-directory`;
each tutorial's principals/identities retain their independent namespace.

`tutorial.yaml` creates user and agent principals in the `ki-tutorial`
namespace under that external directory. Plan and apply it, inspect the
principals, then repeat the plan to verify no changes. Edit a principal's
display name or metadata and apply again. `delete -f tutorial.yaml` removes
the declared principals and retains the default directory. `principals.yaml`
also uses the external default directory.

Start with `external-default.yaml` to verify lookup without managing children.
An explicit directory ID is also supported. Lookup alone fails if `default`
is absent; it never silently creates or adopts a directory. Do not load the
managed bootstrap and external-reference examples together as one manifest.

To start managing an existing unowned default directory, first adopt it:

```sh
kongctl adopt identity directory default --namespace identity-directory
kongctl plan -f directories.yaml --mode apply
```

Review the explicit settings before applying. Applying the bootstrap manifest
without adoption does not take over an existing unowned directory.

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
URLs. Add `--include-child-resources` to include safe client, scope and claim
configuration. Client secrets stay outside declarative dump.
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
evaluate token or userinfo behavior.

### Declarative scopes and claims

`auth-servers.yaml` also declares scopes and claims. Scopes require explicit
`ref` and API `name`; claims additionally require a nonempty string `value`.
Values can contain static text, JSON encoded as a string, or server templates
such as `${ uuidv4 }`. KongCTL preserves that text for the server to evaluate.
`include_in_scopes` accepts scope UUIDs or `!ref scope-ref#id`, including scopes
created in the same saved plan. Every referenced scope must belong to the
same auth server.

Scope settings are `description`, `default`, `include_in_metadata`, and
`enabled`. A default scope is still subject to client authorization; discovery
visibility alone grants no access. Claim settings are `include_in_token`,
`include_in_all_scopes`, `include_in_scopes`, and `enabled`. According to the
API contract, enabled claims excluded from tokens can be retrieved through
userinfo. This example configures those flags without minting a runtime token.

Both child families have no labels or `managed_by`. They inherit the managed
server's namespace and update/delete protection, with no independent
`kongctl` block. Explicitly adopt an existing server before managing its
children. External server declarations support reads/references but cannot
authorize these child writes. Grouped `identity.scopes` and `identity.claims`
declarations are also accepted with `auth_server: issuer-ref`.

An explicitly present child collection is authoritative during sync. Omission
retains that collection; `scopes: []` or `claims: []` removes all members of
that collection under the selected managed server. Use apply for partial
collections. To remove children while retaining their server, sync an empty
child collection under its managed declaration. Empty grouped collections
cannot identify a parent and are rejected for sync.

PATCH preserves omitted optional settings. An empty scope description or
claim `include_in_scopes: []` clears that field. Explicit false flags are
preserved. Names identify resources within their parent; changing a name
creates a different child and sync can prune the old name. Refs remain local.
Duplicate remote claim names cause a clear conflict before mutation or dump.
Use UUID reads to inspect such records and resolve the ambiguity explicitly.

Scope deletion is blocked while any observed client or claim explicitly
references it, including records omitted from the manifest. A planned claim
detachment or deletion runs first; execution checks references again. Deleting
the example removes declared claims/scopes before the server, using
`force=false`. Undeclared children remain and can block server deletion.

Include children in a dump to export flags, values, and portable scope links:

```sh
kongctl dump declarative --resources identity.auth_servers \
  --include-child-resources
kongctl plan -f auth-servers.yaml --mode apply > auth-plan.json
kongctl apply --plan auth-plan.json --auto-approve
kongctl delete -f auth-servers.yaml --auto-approve
```

## Directories and principals

The normal examples use the default directory externally. Begin by inspecting
it, then plan the principal tutorial:

```sh
kongctl get identity directories
kongctl get identity directory default -o json
kongctl get identity principals --directory-name default -o json
kongctl get identity principal <principal-id> --directory-name default -o json
kongctl plan -f external-default.yaml
kongctl plan -f tutorial.yaml --mode apply > plan.json
kongctl apply --plan plan.json --auto-approve
kongctl get identity principals --directory-name default -o json
kongctl dump declarative --resources identity.directories
```

Principal inspection requires an explicit directory name or ID. Principal
display names are not unique, so individual reads require the principal UUID.
JSON/YAML omit `active` when the API does not return it; text shows unknown.
`principals.yaml` declares user, agent, and unclassified principals under the
external default directory. A managed directory may also contain principals.
Each principal has independent namespace and protection settings; directory
ownership does not authorize changes to its principals.

Principal matching uses the directory ID plus `kongctl-namespace` and
`kongctl-ref` in `managed_by`. The other generated entries are `tool`,
`version`, and `kongctl-protected`. Display names can repeat. Keep refs stable:
renaming a ref creates a different principal and can remove the old one during
sync. Do not supply `managed_by` yourself.

Updates use a fresh GET plus PUT and preserve omitted public fields. Explicit
empty `description`, `labels: {}`, or `metadata: {}` clears those values.
Use strings for exact numeric identifiers in metadata. The deployed API
rounds some large integer values, including integers in arrays. Configuration
rejects integers that change under the observed numeric conversion (Q31),
before making API requests. Stable values retain their integer type; strings
are never converted to numbers. Writes also check returned metadata and report
an error if it differs, even though the remote write may already have occurred.
`type` is currently create-only: omission preserves the existing type, while
requesting another type or resetting a classified principal to null fails.
Optional `active: true` or `active: false` controls the principal's state.
Inactive principals are treated as nonexistent when authenticating; they
remain visible to administration reads. Omission preserves the existing
state; creation without it uses the documented API default of true. Explicit
null is rejected. The specification prohibits deactivating developer-portal
principals and says setting false changes provisioning to `konnect_user`.
US `.tech` supports the verified lifecycle on directly provisioned principals.

An API deployment that omits active cannot reconcile an explicit declaration:
updates fail before writing, while a create reports unverifiable state after
the record may have been created. Inspect it and generate a fresh plan rather
than blindly retrying. Inspection/dump never infer false from an absent field.
Track deployment differences and type restrictions in the
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
and namespace/protection, and includes active only when the API returns it. Dumps of unowned principals require
explicit adoption before management. Filter to one directory if multiple
remote resources use the same ref.

### Advanced directory administration

`directories.yaml` bootstraps or configures the managed default directory.
`scoped-control-plane.yaml` shows restricted access on a managed default;
it requires an organization that permits restricted directory access.
Organizations with a one-directory quota require
`allow_all_control_planes: true`. A protected default must be unprotected in a
separate operation before changing its public settings.

To manage an existing directory, explicitly adopt it into your namespace
before applying a managed declaration with the same name:

```sh
kongctl adopt identity directory default --namespace identity-directory
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

The preview specification exposes `vault_enabled`. Original US `.com`
probes accepted `true` and returned `false`. KongCTL reports that mismatch as
an error. Keep it false until Token Vault is available in your environment;
the directory lifecycle scenario verifies false, while unit coverage verifies
that a true request is sent and an ignored value is reported.
Native `.tech` revalidation confirms true persists and enables Vault
provider/key lists there. The common bootstrap still uses false to match a
new organization's default settings; availability is environment-specific.

`scoped-control-plane.yaml` shows restricted access with a control plane
reference. Use UUIDs or `!ref gateway#id`, including references to external
control planes. Control planes are created before directory settings that
reference them are applied. Apply each example separately.

Protection follows the usual KongCTL lifecycle. Set `kongctl.protected` to
false and apply that change separately before changing protected settings or
deleting the directory. A protected resource cannot be changed and
unprotected in the same operation.

`sync` only prunes directories when `identity.directories` is explicitly in
scope. To remove every managed, unprotected directory in this namespace:

```yaml
_defaults:
  kongctl:
    namespace: identity-directory
identity:
  directories: []
```

Omitting the collection leaves directories alone during sync. External,
foreign-namespace, and unowned directories are not pruned. `delete -f` can
delete explicitly declared managed directories. KongCTL never requests
forced cascading deletion; the API must permit deletion without losing
descendant resources. Credentials, groups, Token Vault children, and other
directory children remain separate follow-up work.
