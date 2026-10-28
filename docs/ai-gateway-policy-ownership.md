# Federated AI Gateway policies

Policies support independent namespace ownership alongside models, providers,
and auth strategies. Root and nested policy declarations accept `kongctl`.
The explicit child namespace overrides source `_defaults.kongctl`, then the
literal `default`. Gateway metadata does not supply ownership or protection.

```yaml
_defaults:
  kongctl: {namespace: security-team}
ai_gateway_policies:
  - ref: shared-sanitizer
    ai_gateway: !lookup {name: shared-ai-gateway}
    name: shared-sanitizer
    display_name: Shared sanitizer
    type: ai-sanitizer
    global: false
    enabled: true
    config: {anonymize: [email]}
```

Policies persist `kongctl-namespace` and `kongctl-protected` in `managed_by`,
alongside `tool: kongctl` and the executing binary's `version`. Those four
reserved entries leave room for one unrelated entry within the API's
five-entry limit. Unrelated entries are merged again before PUT. Public labels
remain data; omitted labels preserve fresh observed values, while an explicit
empty map clears them during a payload update. Provenance alone never grants
ownership, and legacy public management labels do not authorize policy writes.

Existing unowned policies require explicit out-of-band API adoption. Review
the current owner, protection, config, global effect, and attachments first.
Set the reserved namespace and protection while preserving unrelated metadata
and the complete policy payload. Transfer from another namespace requires
explicit coordination. Defaults and references never adopt or transfer.
Names remain immutable. Policy type/config updates retain existing API
behavior, subject to protection. For a named replacement, create a new policy
and move users before deleting the old one.

Apply never prunes. Sync scopes policies per source, namespace, and canonical
gateway, including aliases. Omission leaves the collection untouched. To
remove a team's last policy, sync an explicitly empty nested collection:

```yaml
_defaults:
  kongctl: {namespace: security-team}
ai_gateways:
  - ref: shared
    _external:
      selector:
        matchFields: {name: shared-ai-gateway}
    policies: []
```

This never broadens gateway pruning or removes foreign/unowned policies.
Retained agents, consumers, groups, models, and MCP servers prevent direct
deletion of a referenced policy, regardless of ownership. Combined final plans
order detachments or user deletions before target deletion, and replacement
creation before new attachments across namespaces and gateway aliases.
Consumer group membership behavior is unchanged.

`global: true` retains its API meaning: the policy applies gateway-wide.
Independent ownership controls reconciliation, not runtime isolation. A global
policy owner can affect requests belonging to other teams. Gateway owners can
cascade-delete the gateway with warnings; the gateway's own protection still
applies.

Protected policies require a separate metadata-only unprotection update.
Saved plans and direct adapter calls recheck fresh ownership, protection,
identity, metadata capacity, and creation-name collisions. Metadata-only
writes preserve fresh config, conditions, enabled/global flags, and labels.
They reject a request whose payload differs from the planned public fields.
Actual payload changes retain existing full-PUT configuration behavior.

Policy config remains arbitrary JSON under its existing API contract. Live
checks found readable synthetic authentication values in several policy GET
responses. Metadata-only plans therefore retain the manifest's declared config
rather than copying complete observed config into the saved plan; execution
preserves the complete fresh config in memory. This does not add deferred
secret declarations or implicit credential rotation for arbitrary plugins.

Live validation covered POST/GET/LIST/PUT and metadata-map replacement for
15 policy configurations, including a global policy. CLI protection and
unprotection preserved complete backend config and sampled readable synthetic
authentication values; those observed values were absent from saved plans.
These checks demonstrate backend retention, not successful authentication to
external services. Live adoption, stale ownership/protection, public-label
preservation/clearing, metadata overflow, concurrent name collision, and
retained unowned-model deletion checks also passed.

Dumps translate policy ownership into `kongctl`, remove generated metadata,
and preserve unrelated entries and public labels. Orphaned protection warns
without inferring ownership. Dump/reload converges with the existing config
representation. API checks are cooperative and cannot close the non-atomic
check/write race or make paginated lists a snapshot.

Custom policy definitions remain a separate integration under #2487. See the
[shared contract](ai-gateway-child-ownership.md) and
[runnable manifests](examples/declarative/ai-gateway-federated-policies).
