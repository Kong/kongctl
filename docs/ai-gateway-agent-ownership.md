# Federated AI Gateway agents

Both `a2a` and `http` agents support independent namespace ownership and
protection through `kongctl` on root or nested declarations. An explicit child
namespace overrides source `_defaults.kongctl`, then the literal `default`.
Gateway ownership and public labels do not authorize agent mutation.

Agents persist `kongctl-namespace` and `kongctl-protected` in `managed_by`,
alongside `tool: kongctl` and the executing binary's `version`. Those four
reserved entries leave one unrelated entry within the API's five-entry limit.
Writes preserve unrelated metadata and reject overflow. Omitted public labels
preserve freshly observed labels; an explicit empty map clears them.
Provenance alone confers no ownership and does not schedule version-only writes.

Existing unowned agents require explicit out-of-band API adoption. Review the
owner, protection, complete public config, credentials, and policy/auth
attachments first. Update the reserved namespace/protection while retaining
unrelated metadata and the full public payload. Coordinate ownership transfers
explicitly. Neither gateway references nor policy/auth references adopt or
transfer their targets. Names are immutable; type changes remain supported
when unprotected, as confirmed for both types by the live API.

Apply never prunes. Sync scopes agents by source, namespace, and canonical
gateway, including aliases. Omission leaves the collection untouched. An
explicit empty nested `agents: []` removes only that source's owned agents;
it never broadens gateway pruning. See the [runnable example][example].

Policy and auth references retain existing attachment behavior. Dependency
checks observe all users across namespaces, including foreign/unowned agents.
A retained reference blocks direct target deletion. Combined final plans order
agent detachments/deletions before target deletion and target creation before
attachment, including across gateway aliases. No referenced resource is adopted.
Consumer group membership behavior remains unchanged.

Saved plans and direct writes revalidate fresh ownership, protection, identity,
metadata capacity, and creation-name collisions. Protection requires a separate
metadata-only unprotection update. Shared full-PUT handling preserves fresh
public API defaults/config and labels for metadata-only writes, while rejecting
payload injection and concurrent secret rotation during unprotection.
The gateway owner may cascade-delete foreign, unowned, or protected agents
with warnings; the gateway's own protection remains enforced.

Three reviewed write-only fields now use deferred secret declarations:

- `config.upstream.auth.secret_access_key`
- `config.upstream.auth.session_token`
- `config.proxy.auth.password`

Use `!secret` with a configured source or a supported public vault reference;
inline credential values are rejected. Public AWS access-key IDs, regions,
proxy usernames, and proxy endpoints remain ordinary fields. Secret selection
and rotation are explicit; metadata updates neither resolve nor rewrite an
unselected credential. Explain/scaffold describe these deferred fields without
changing MCP's separate secret contract.

Live validation confirmed POST/GET/LIST/PUT metadata persistence and map
replacement for both types. Fresh Kong AI Gateway 2.2.0 runtimes, with empty
local caches and Konnect's expected configuration version, retained synthetic
AWS key/session and proxy password values after CLI protection/unprotection.
Each type used distinct values, compared only in memory.

A2A values were observed directly in the runtime. The native HTTP agent
projection did not expose those authentication fields in its runtime config.
For equivalent backend evidence, the disposable HTTP record was temporarily
switched to A2A through the API-supported type update, supplying only the
redacted public payload. Its original credentials then appeared in a fresh
runtime, including after metadata PUTs performed while its type was HTTP.
The probe restored the HTTP type between phases without rewriting credentials.
This verifies stored credential retention for HTTP, not native HTTP runtime
usage or authentication against external AWS/proxy services. Disposable
gateways, containers, and TLS material were removed.

Dumps translate ownership into `kongctl`, preserve public labels and unrelated
metadata, and omit generated metadata. Protection without ownership warns
without inferring adoption. Dump/reload converges without credential rotation.
API checks remain cooperative: they cannot close the non-atomic check/write
race or turn paginated lists into a snapshot.

[example]: examples/declarative/ai-gateway-federated-agents
