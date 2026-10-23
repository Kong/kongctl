# Federated AI Gateway auth strategies

Key-auth and OpenID Connect strategies support independent namespace
ownership alongside models and providers. Gateway ownership and public
labels no longer authorize strategy updates. Existing unowned strategies
require explicit adoption; sync preserves foreign and unowned strategies.

Declare `kongctl.namespace` and `kongctl.protected` on a root or nested
strategy, or set the source `_defaults.kongctl`. Defaults apply per source;
strategies do not inherit the gateway's namespace or protection.

```yaml
_defaults:
  kongctl: {namespace: security-team}
ai_gateway_auth_strategies:
  - ref: shared-key-auth
    ai_gateway: !lookup {name: shared-ai-gateway}
    name: shared-key-auth
    display_name: Shared key authentication
    type: key-auth
    managed_by: {inventory: shared}
    config:
      key_names: [apikey]
```

Ownership uses `managed_by.kongctl-namespace` and
`managed_by.kongctl-protected`. Writes also stamp `tool: kongctl` and the
executing binary's `version`. These four reserved entries leave one entry
for unrelated metadata within the five-entry API limit. Unrelated metadata
is preserved, including concurrent additions observed immediately before
PUT. Omitted public labels are preserved from that fresh read; an explicit
empty label map clears them. Provenance alone does not establish ownership.

For adoption or transfer, review the current owner, protection, dependencies,
and complete payload. Explicitly update the reserved namespace/protection
through the API, preserving unrelated metadata and credentials. Defaults
and references never adopt or transfer a strategy. Verify the credential
preservation contract before constructing a PUT from redacted GET data.

Apply never prunes. Sync an explicitly empty nested collection to delete the
last strategy in the source's namespace:

```yaml
_defaults:
  kongctl: {namespace: security-team}
ai_gateways:
  - ref: shared
    _external:
      selector:
        matchFields: {name: shared-ai-gateway}
    auth_strategies: []
```

Omission leaves the collection untouched. Scoping strategies never broadens
root gateway pruning. Names are unique per actual gateway, including when
multiple declarations use different gateway aliases. Names and types remain
immutable.

A retained model, agent, or MCP server referencing the strategy prevents
direct deletion, even when that user is foreign or unowned. Combined final
plans order detachments/deletions before target deletion and creation before
new attachments across namespaces and aliases. Gateway owners retain cascade
delete authority with warnings, subject to the gateway's own protection.

Protected strategies require a separate metadata-only unprotection update.
It cannot also change public configuration, unrelated metadata, or secrets.
Saved plans, direct adapter calls, replacements, and secret-only writes
recheck ownership/protection and creation collisions immediately before
writing. The API still has a non-atomic check/write race.

OIDC `client_secret` arrays and `http_proxy_authorization` /
`https_proxy_authorization` use deferred `!secret` sources. Unselected scalar
arrays omit the entire field; selecting a client-secret array selects all
its entries. Mixed selection fails. Proxy authorization values omit
individually. Ownership updates never resolve or silently rotate credentials,
and plans/logs do not contain resolved values.

Live validation verified POST, GET, LIST, metadata-map replacement, and CLI
protection/unprotection for both strategy types. OIDC validation compared two
client-secret entries and both proxy authorization values in fresh runtimes,
with empty local caches synchronized to Konnect's expected config version,
before and after protection and unprotection PUTs. All four synthetic values
remained identical. This is backend credential-preservation evidence; it does
not claim a successful OAuth exchange with an identity service.

Dumps translate ownership to `kongctl`, remove generated metadata, and
preserve public labels and unrelated entries. Protection metadata without an
ownership namespace produces a warning, without adopting the strategy.
Write-only credentials are not reconstructed from responses.

See [the shared contract](ai-gateway-child-ownership.md) and
[runnable manifests](examples/declarative/ai-gateway-federated-auth-strategies).
