# Federated MCP servers, vaults, consumers, groups, CA certificates, and SNIs

These six families use the [shared ownership contract][contract] on root and
nested declarations. Their independent `kongctl` namespace and protection
override source defaults, then fall back to `default` and `false`. Gateway
ownership and public labels do not authorize writes. Existing resources need
explicit API adoption; references never adopt their targets.

`managed_by` contains four generated entries: `tool: kongctl`, the executing
binary's `version`, `kongctl-namespace`, and `kongctl-protected`. Kongctl
generates the complete map and replaces outside entries. Direct `managed_by`
input is rejected. Omitted labels preserve fresh public labels; `labels: {}`
clears them. Binary upgrades alone do not schedule writes.

Apply never prunes. Sync reconciles only the source/namespace collection scope
on the canonical gateway, including aliases. An omitted collection preserves
it; an explicitly empty nested collection removes only that namespace's owned,
unprotected children. Gateway pruning scope does not expand. Unprotect first
with a metadata-only apply before changing payloads, rotating secrets, or
deleting a child.

Saved plans and direct writes recheck fresh ownership, protection, metadata
capacity, and creation-name collisions. MCP, vault, CA, and SNI deletion also
rechecks current reference users immediately before deleting, after preceding
detachments have executed. These checks retain the documented API check/write
race; they are not atomic backend ownership enforcement.

## MCP servers and vault credentials

All five MCP server types and all seven vault providers participate. MCP
source/listener dependencies resolve across namespaces and gateway aliases.
A foreign or unowned retained listener prevents source deletion. Combined
plans create sources before listeners and detach/delete listeners before
sources. Route replacements retain the existing create-before-delete behavior.

Four MCP write-only paths require deferred `!secret` declarations or supported
public vault references:

- `config.upstream.auth.secret_access_key`
- `config.upstream.auth.session_token`
- `config.proxy.auth.password`
- `config.server.tools_list_auth.client_secret`

Vault write-only fields keep their existing deferred-secret contract. Public
AWS IDs, regions, proxy usernames, and other public siblings survive
metadata-only PUTs. Unselected credentials are omitted without resolving or
rewriting their sources. Values never appear in plans or logs.

Live API checks covered sixteen vault configurations, both consumer types,
all MCP types, groups, CA certificates, and SNIs. Fresh synchronized Kong AI
Gateway 2.2.0 runtimes demonstrated retention of distinct synthetic MCP AWS,
proxy, and tools OAuth credentials and configured vault authentication
credentials after protection/unprotection PUTs. Fifteen distinct values belong
to updated records. A sixteenth AppRole value remained intact after its unsafe
metadata write was rejected before mutation, rather than through a successful
AppRole metadata PUT. Runtime configuration
observation proves stored credential continuity; external AWS, OAuth, proxy,
and vault authentication was not exercised.

The API redacts vault references inside write-only credentials, including the
public `{vault://name/path}` expression. An absent reference therefore cannot
prove a vault unused. Direct vault deletion fails when a retained provider,
OIDC strategy, MCP server, agent, or vault has opaque credential slots that
could reference it. A combined deletion of those users can establish safety;
a metadata-only update or omitted secret cannot. Public references in readable
configuration are checked independently. This conservative restriction can
block deletion even when a particular credential does not use that vault.
Improved backend reference visibility would allow a narrower restriction.

### HashiCorp AppRole PUT limitation

The backend clears an omitted AppRole `secret_id` on PUT, while GET redacts
it. A live fresh runtime rejected the resulting configuration because neither
`approle_secret_id` nor `approle_secret_id_file` was set. Vault metadata PATCH
is unavailable. kongctl therefore rejects such updates before writing.

Creating an owned AppRole vault with an explicitly selected secret remains
supported. Later unprotected updates must explicitly select
`/config/secret_id`, or use the existing public `secret_id_file` configuration.
A vault using only redacted `secret_id` cannot safely protect/unprotect through
a metadata-only PUT until the backend supports preserving that credential.
There is no implicit resubmission or automatic credential rotation. Existing
protected records retain the separate unprotection requirement; the restriction
is deliberately not bypassed by selecting a secret during unprotection.

## Consumers and group membership

Consumer ownership controls credential creation, rotation, and deletion.
Credentials retain their existing create-only values and inherit consumer
namespace/lifecycle; they have no independent adoption or protection. New
protected consumers can receive their initially declared credentials in the
same execution. Later credential writes require unprotecting the consumer.
Deleting a consumer removes its credentials and invalidates those API keys.

The group owner still controls the complete optional `consumers` member set,
including foreign-owned consumers. Omission preserves membership; `[]` clears
it. Membership changes neither adopt/delete consumers nor modify credentials.
There is no consumer-side membership declaration or per-member ownership.
Consumer deletion removes its memberships through the existing backend
behavior. Protection changes alone do not issue membership mutations.

## Runtime TLS boundary

CA certificates and SNIs have independent ownership. An SNI's `certificate` is a
reference to the runtime certificate name, never an ownership source. Retained
foreign/unowned SNI references prevent unsafe certificate deletion; combined
reassignment/deletion orders before certificate removal. Public CA and SNI
configuration references are observed across namespaces before direct deletion.
Dumps retain associations and public labels, translate child ownership to
`kongctl`, and omit write-only credentials.

Runtime certificates remain on their existing lifecycle. Their PUT requires
the complete private key, GET does not return it, and metadata-only PATCH
returns HTTP 405. Full PUT semantics are accepted; missing PATCH support is
the API blocker. Federation is deferred until a credential-preserving metadata
update is available. No automatic private-key resubmission or relaxed
unprotection rule is introduced. Config stores, their secrets, and data plane
certificates also remain excluded.

The gateway owner may cascade-delete all descendants, including foreign,
unowned, and protected children, with warnings about affected families,
namespaces, and protected descendants. The gateway's own protection remains
enforced, with no additional confirmation flow.

[contract]: ai-gateway-child-ownership.md
