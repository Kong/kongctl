# Federated AI Gateway custom policy definitions

Installed and streaming custom definitions support independent namespace
ownership alongside models, providers, auth strategies, and policy instances.
This feature remains beta and requires a backend with custom-policy support.
Live validation used gateways with minimum runtime 2.2. An installed definition
does not deploy a plugin binary to runtimes; streaming definitions retain the
existing schema/handler contract.

Root and nested definitions accept `kongctl.namespace` and
`kongctl.protected`. Explicit child values override source `_defaults.kongctl`,
then the literal `default`. Gateway metadata does not supply child ownership
or protection.

```yaml
_defaults:
  kongctl: {namespace: definitions-team}
ai_gateway_custom_policies:
  - ref: shared-plugin
    ai_gateway: !lookup {name: shared-ai-gateway}
    name: shared-plugin
    display_name: Shared installed definition
    type: installed
    labels: {team: security}
    schema: |
      return {
        name = "shared-plugin",
        fields = { { config = { type = "record", fields = {} } } },
      }
```

Definitions persist `kongctl-namespace` and `kongctl-protected` in `managed_by`,
alongside `tool: kongctl` and the executing binary's `version`. Kongctl
generates the complete map and replaces outside entries. Direct `managed_by`
input is rejected. Public labels remain data: omission preserves fresh observed
labels, while an explicit empty map clears them. Neither provenance nor legacy
public management labels authorize ownership.

Existing unowned definitions require explicit out-of-band API adoption.
Review the owner, protection, full schema/handler, metadata, labels, and policy
instances before setting the reserved namespace/protection. Preserve the full
PUT payload during adoption or transfer. Defaults and references never adopt
or transfer a definition. Names and installed/streaming types are immutable.
The backend rejected changes to either identity field with HTTP 400 in both
variants. Create a differently named definition and migrate instances when a
replacement is needed.

Apply never prunes. Sync scopes definitions per source, namespace, and actual
gateway, including aliases. Omission leaves the collection untouched. To remove
a team's last definition, sync an explicitly empty nested collection:

```yaml
_defaults:
  kongctl: {namespace: definitions-team}
ai_gateways:
  - ref: shared
    _external:
      selector:
        matchFields: {name: shared-ai-gateway}
    custom_policies: []
```

This leaves foreign/unowned definitions, policy instances, the gateway, and
other collections untouched. A retained policy instance whose `type` names the
definition blocks direct deletion, including foreign and unowned instances.
Combined final plans order replacement definition creation before instance
type updates and old-definition deletion. Detaching or deleting instances in
another namespace is allowed only through that instance owner's source.

Protected definitions require a separate metadata-only unprotection update.
Schema, handler, display name, labels, namespace, and identity cannot change in
that write. Saved plans and direct adapter calls recheck current ownership,
planned protection, identity, metadata capacity, and creation-name collisions.
Unselected public labels are preserved from a fresh detail read. The gateway
owner retains whole-gateway cascade authority with warnings; the gateway's own
protection remains enforced.

Schema and handler remain explicit, readable Lua strings with full-PUT update
semantics. Installed definitions omit handler; streaming definitions require it.
No credential rotation or write-only omission contract is introduced. Live
POST/GET/LIST/PUT probes verified metadata persistence, map replacement, labels,
and complete definition retention for both variants. CLI checks covered
adoption, saved-plan ownership/protection drift, label preservation and
clearing, unprotection, schema/handler updates, canonical metadata, late
creation collision, retained unowned-instance deletion blocking, and isolated
last-owned deletion. Disposable resources were removed.

Dumps translate ownership into `kongctl`, strip generated entries, and preserve
public labels, schema, and handler. Protection without an ownership namespace
produces a warning without adopting the definition. Dump/reload converges under
the same namespace contract. API checks remain cooperative: fresh reads cannot
close the non-atomic check/write race or make paginated observations a snapshot.

See [the shared contract](ai-gateway-child-ownership.md) and
[runnable manifests](examples/declarative/ai-gateway-federated-custom-policies).
