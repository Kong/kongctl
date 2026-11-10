# Resource-specific declarative behavior

[Declarative configuration](../declarative.md)

Read the relevant section for API operation removal, portal webhooks, and
AI Gateway or Kong Identity lifecycle constraints. Field schemas remain in the
[resource reference](../declarative-resource-reference.md).

## Authorizing API Operation Removal

When access control enforcement (ACE) is enabled, Konnect rejects updates that
remove operations from the current API specification unless explicitly
authorized. Set the synthetic `force_delete` field directly on the parent API to
authorize these updates:

```yaml
apis:
  - ref: bookings
    name: Bookings
    version: beta.2
    force_delete: true
    versions:
      - ref: bookings-beta2
        version: beta.2
        spec: !file ./bookings-beta2.yaml
```

This field permits operation removal during updates. It does not request API
deletion or bypass other validation.

This policy defaults to false and is supported only on managed APIs. It applies
to updates of the API and its child versions, including versions declared at the
root with `api: bookings`. It cannot be set on children or in `_defaults`. It
authorizes future updates while present in the manifest; remove it when that
authorization is no longer intended.

The planner records `force_delete: true` on authorized update changes in saved
JSON plans. Executing such a plan preserves that policy without consulting the
original manifest. It maps to Konnect's `force=true` update query parameter, is
never sent in the request body or labels, and is not inferred or exported by
`dump`.

When switching the current version, kongctl creates or updates the target child
version before updating the API. In `sync` mode, deletion of the old current
version waits for the switch to succeed. Keep the old version in `versions` to
retain it; omitting it requests deletion when that collection is in sync scope.
Other Konnect deletion restrictions still apply.

Existing child specifications can also be updated in place. The same
authorization is needed if such an update removes current operations under ACE.
Changing the operations of the current version affects access control even if
historical version records are retained.

## Portal Audit Log Webhooks

Portal audit-log webhooks can reference organization audit-log destinations that
are managed outside kongctl. Declare those destinations under
`audit-logs.destinations` with `_external`, then reference them from a portal
webhook with `!ref`.

```yaml
portals:
  - ref: docs-portal
    name: Docs Portal
    audit_log_webhook:
      ref: docs-portal-audit-log-webhook
      enabled: true
      audit_log_destination_id: !ref foo

audit-logs:
  destinations:
    - ref: foo
      _external:
        selector:
          matchFields:
            name: foo
```

`audit-logs.destinations` supports `_external.id` and
`_external.selector.matchFields.name`. Destination resources cannot declare
`kongctl` metadata and are not created, updated, or deleted by declarative
apply. In sync mode, omitted portal webhook configuration is ignored unless an
`audit_log_webhook` block is explicitly present for that portal. To remove an
existing webhook while retaining the portal, declare `audit_log_webhook: {}`.
`audit_log_webhook: null` is rejected because null is not a reset or delete
signal.

AI Gateway nodes are imperative, read-only resources. Inspect them with
`kongctl get ai-gateway nodes`; omit them from declarative manifests. Use
`kongctl explain ai_gateway` for supported resource schemas.

## AI Gateway MCP server replacements

MCP server names are immutable: a name change in `sync` creates a new server and
deletes the old one. For enabled `listener`, `conversion-listener`, and
`passthrough-listener` servers in the same gateway, kongctl creates the
replacement before deleting the old server when their normalized `config.route`
values match. Auth strategies, policies, and sources needed by the new server
are created first; unused strategies and policies are deleted after the old
server. These dependencies are stored in saved plans as well. If replacement
creation fails, deletion of the old server is blocked.

The route comparison includes paths, hosts, methods, headers, protocols, regex
priority, and route behavior settings. It ignores list order and normalizes
documented API defaults. It preserves case and exact path/regex values. A
partially overlapping or changed route does not qualify. Neither disabled
servers nor the source-only `conversion-only` and `upstream-server` variants
qualify. All matching creations must succeed before a matching deletion
proceeds, including when several servers share a route.

Konnect accepts distinct server names with the same route, including different
authentication and policies. During overlap, either configuration may serve
requests; do not rely on the old route winning. Clients must be compatible with
both configurations during this interval. See Kong's
[route priority documentation][gateway-route-priority].

Successful control-plane creation is not a data-plane readiness check. kongctl
does not wait for all data planes to serve the replacement or preserve MCP
sessions across a rename, so this ordering does not guarantee zero downtime. For
a controlled cutover, first use `apply` to create the replacement while
retaining the old server, verify traffic and authentication on the data planes,
then use `sync` to retire the old server and unused dependencies.

There is no automatic delete-first fallback. If an API or deployment rejects
overlap, the create fails and leaves the old server intact. If overlap itself is
unacceptable (for example, mutually exclusive authorization policies),
explicitly retire the old server in a separate sync before adding the new one,
accepting the availability gap. Other route changes and source-only renames
retain ordinary dependency ordering without a replacement guarantee.

[gateway-route-priority]:
https://developer.konghq.com/gateway/entities/route/

## AI Gateway Config Stores and Vaults

AI Gateway Config Stores can back Konnect Vaults declared under the same
gateway. Use `!ref <config-store-ref>#id` for the Vault's
`config.config_store_id` so kongctl orders creation and supplies the remote
Config Store ID:

```yaml
ai_gateways:
  - ref: support-gateway
    name: support-gateway
    display_name: Support Gateway
    config_stores:
      - ref: support-config-store
        name: support-config-store
        display_name: Support-Config-Store
        secrets:
          - ref: support-openai-header
            key: openai-auth-header
            value: !secret {source: !env OPENAI_AUTH_HEADER}
    vaults:
      - ref: support-secrets
        name: support-secrets
        type: konnect
        config:
          config_store_id: !ref support-config-store#id
```

Secret values are write-only. New secrets require `value: !secret` with a
deferred source and are written once during creation. Existing secrets are not
rotated unless `--write-secret <ref>#value` or `--write-secrets` is supplied
while planning. Omit `secrets` to leave a store's secrets unmanaged during sync,
or use `secrets: []` to remove all secrets in that store's sync scope. See the
[Config Store and Vault example][config-store-vault-example] for a complete
provider Vault reference.

[config-store-vault-example]:
../examples/declarative/ai-gateway/config-store-vault.yaml

## AI Gateway Runtime TLS

AI Gateway runtime certificates, CA certificates, and SNIs are gateway child
resources. They are distinct from data plane certificates, which authenticate
data planes to Konnect. Runtime certificate private keys are write-only and must
use `!secret`:

```yaml
ai_gateways:
  - ref: support-gateway
    name: support-gateway
    display_name: Support Gateway
    certificates:
      - ref: runtime-cert
        name: runtime-cert
        cert: !file ./certs/runtime.pem
        key: !secret {source: !file ./certs/runtime-key.pem}
    ca_certificates:
      - ref: partner-ca
        name: partner-ca
        cert: !file ./certs/partner-ca.pem
    snis:
      - ref: support-sni
        name: support-sni
        display_name: Support hostname
        hostname: support.example.com
        certificate: !ref runtime-cert#name
```

Omit one of these child keys during sync to leave that collection unmanaged. Use
`certificates: []`, `ca_certificates: []`, or `snis: []` under a gateway to
sync-delete that collection. Dumps omit private keys and can be planned again
without reporting drift solely because those keys are unavailable.

## AI Gateway 2.3 configuration

Use a gateway running AI Gateway 2.3 for Token Vault, MCP method/tool rate
limits, Baseten targets, Bedrock Mantle endpoints, and the new outbound TLS
configuration on agents and MCP servers. A spec or SDK snapshot alone does not
enable these features on an older backend or data plane.

Datastores support nested `ai_gateways[].datastores` and root-level
`ai_gateway_datastores` declarations with an `ai_gateway` parent. Supported
types are `redis-ce`, `redis-ee`, `vectordb`, and `token-vault`. Each datastore
requires explicit `name`, `display_name`, `type`, and `config` fields.

Semantic models can reference a `vectordb` datastore in the same gateway using
`datastores: [{name: !ref vector-store#name}]`. Declare connection settings on
the datastore and vector search settings (`dimensions`, `distance_metric`, and
`threshold`) under `config.balancer.vectordb`. Planning accounts for connection
settings expanded by the API without requiring them in the model manifest.
Explicit model connection settings, embeddings, search settings, targets, and
datastore references still participate in comparison. The datastore can be
managed outside the loaded files. Dumps omit its expanded connection fields from
the model so later datastore changes remain inherited.

```yaml
ai_gateways:
  - ref: support
    name: support
    display_name: Support
    min_runtime_version: "2.3"
    datastores:
      - ref: credential-store
        name: credential-store
        display_name: Credential storage
        type: token-vault
        config:
          directory: support-credentials
          current_version: v1
          keys:
            - version: v1
              key: !secret {source: !env TOKEN_VAULT_ENCRYPTION_KEY}
```

Providers and MCP servers using `token_vault` require exactly one datastore
reference, such as `datastores: [{name: !ref credential-store#name}]`, to a
`token-vault` datastore under the same gateway. Auth strategies require it
whenever `token_vault` is present, which enables enrollment protection even when
`token_vault.enable_credential_manager` is false. Configure the upstream
credential provider using `token_vault.provider` on providers and MCP servers.
The API's resolved `token_vault.storage` is read-only and is not exported as
writable configuration.

See the [Token Vault
example](../examples/declarative/ai-gateway/token-vault.yaml) for a complete MCP
passthrough listener and model provider sharing a datastore. The named Token
Vault directory and provider registration must already exist; these declarations
configure their use, rather than create the registration. See the [example
prerequisites](../examples/declarative/ai-gateway/README.md) for enrollment and
encryption-key setup.

Encryption keys and datastore credentials use deferred `!secret` sources.
Existing write-only values require `--write-secret` or `--write-secrets` for an
intentional rewrite. Keep previous key versions while rotating keys so existing
credentials remain decryptable. Dump preserves key version metadata; it cannot
recover encryption key material. Konnect-managed datastores are excluded from
dump and sync pruning. Datastore deletion checks usage and requires dependent
resources to detach or be deleted first.

Agent and MCP `config.certificates` and `config.ca_certificates` contain
certificate names. Use `!ref my-cert#name` for declarations in the same gateway.
MCP `rate_limiting.limits` entries target exactly one `method` or `tool`, with a
positive `limit` and a `window_size` of 1, 60, 3600, or 86400 seconds. Limits
are counted independently on each AI Gateway node. Rate limiting applies to
listener-style MCP servers, not upstream-only server definitions.

MCP server and tool ACLs also accept `allow_when` and `deny_when` arrays of CEL
boolean expressions with runtime 2.3. These fields can be used under
`access.acls`, `access.default_tool_acls`, and tool-level `access.acls`. Server
`access.acls` must specify exactly one of `allow`, `deny`, `allow_when`, or
`deny_when`. Existing manifests combining these fields must choose one rule
before applying. Tool-level `access.acls` and server `access.default_tool_acls`
allow combined fields.

Auth strategy configs use the SDK-generated plugin schemas, including OIDC
`credential_claims` and upstream header mappings. `explain` lists the supported
configuration fields for each authentication strategy.

Policies accept an `expressions` map for runtime 2.3 policy configuration
expressions. Gateway PUT updates preserve existing server `managed_by` metadata.
This field is reserved for Kong tools and cannot be declared in manifests.

Use `kongctl get ai-gateway datastores --gateway-name support` to list stores.

## Kong Identity

Most organizations already have a directory named `default`. Reference it
externally when managing principals and their independently owned children;
adopt it explicitly before managing directory settings or inherited Vault
children. External references do not transfer ownership. See the
[workflow examples][identity-examples].

Principals and identities persist namespace/ref ownership in `managed_by`,
with complete local list filtering and duplicate detection. Basic-auth
usernames and ordinary groups also have independent ownership; direct
memberships inherit their group. Auth Servers and OAuth clients use labels.
Scopes/claims inherit their managed server's authority. Parent linkage alone
does not authorize changes to an independently owned child.

Principal type is create-only. Omitted public fields are preserved through
fresh reads before full PUT. `metadata_mode: merge` preserves unmentioned
metadata at execution; `sync` replaces the map. Merge plans use format 1.2,
which older executors reject. Imported principals remain external; native
org-chart import controls their names and descriptions. Retained management
metadata does not grant the full declarative principal lifecycle.

Generated keys under managed principals/groups inherit parent authority.
Keys under external principal UUIDs instead carry their own immutable
namespace/ref markers. Their matching, updates, scoped sync and deletion do
not adopt the principal. Duplicate matches stop reconciliation. Existing key
adoption, ownership changes and key-level protection remain outside this
supported workflow. Initial key/client-secret
issuance requires `--secret-output-dir`; capture files are private and are
never overwritten. See [secret handling](secrets.md).

Vault keys, providers, static shared credentials and Trusted IdPs require a
managed directory. Enable an existing Vault in a separate step before
planning children, because disabled Vaults hide those collections. Public-key
`kid`, provider name, credential default/priority slot and IdP issuer URL are
their respective remote identities; local refs do not imply remote names.
Static credential changes require explicit secret-write selection. Consent,
OAuth user authorizations and runtime token retrieval are separate workflows.

Apply preserves undeclared resources. Sync only prunes explicitly scoped
collections; omission preserves them. Parent deletion requires independently
authorized child retirements and fresh empty-dependency checks. Passwords,
importer-owned teams/nested groups and workspace-specific identity settings
are outside the supported declarative lifecycle.

`kongctl import id org-chart` processes a complete roster and can deactivate
previously imported people omitted from the file. It does not add KongCTL
ownership. Declarative `!csv` input instead expands ordinary managed
principals and supported children; see [CSV loading](files.md#principal-csv).

[identity-examples]: ../examples/declarative/identity/README.md
