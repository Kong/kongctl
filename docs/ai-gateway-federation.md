# AI Gateway federation: release scope and migration

Teams can manage children of a shared AI Gateway from separate declarative
sources and kongctl namespaces. Each participating child owns its namespace
and protection independently of the gateway. A gateway lookup does not claim
the gateway or another team's children.

## Support matrix

| Child family | Independent ownership | Guidance |
| --- | --- | --- |
| Models (`model` and `api`) | Yes | [Models][models] |
| Model providers | Yes | [Providers][providers] |
| Auth strategies | Yes | [Auth strategies][auth] |
| Policies | Yes | [Policies][policies] |
| Custom policy definitions | Yes | [Custom policies][custom] |
| Agents | Yes | [Agents][agents] |
| MCP servers | Yes | [Remaining families][remaining] |
| Vaults | Yes, with AppRole restrictions | [Vault limits][remaining] |
| Consumers | Yes | [Consumers][remaining] |
| Consumer groups | Yes | [Membership][remaining] |
| CA certificates | Yes | [Runtime TLS boundary][remaining] |
| SNIs | Yes | [Runtime TLS boundary][remaining] |
| Consumer credentials | Inherit consumer ownership | [Consumers][remaining] |
| Runtime certificates | Deferred; existing lifecycle | [TLS limit][remaining] |
| Config stores and their secrets | Excluded | [Shared contract][contract] |
| Data plane certificates | Excluded | [Shared contract][contract] |

This is twelve independently owned child families. Provenance write support
alone does not imply federation.

> **Runtime certificates remain platform-managed.**
>
> Teams can independently manage SNIs referencing a shared runtime certificate
> by name. They do not need its private key.
>
> Keep runtime certificates in the platform's configuration. Omit `certificates`
> from team manifests—`certificates: []` is not scoped to the team's namespace.
>
> Certificate updates require an explicitly supplied private key because the
> API lacks a credential-preserving metadata update. Certificate-level
> namespace/protection is unsupported. This does not affect federation for
> the other supported children. See the
> [API limitations](#deferred-operations-and-api-limitations).

## Release notes

- Participating children persist ownership/protection in `managed_by`, with
  `tool: kongctl` and the executing binary's `version`. Public labels remain
  public metadata and do not establish child ownership. Gateway and other
  resource label-based lifecycle behavior is unchanged.
- Apply creates and updates without pruning. Sync scopes each child collection
  by source, namespace, and actual gateway. An omitted collection is untouched;
  an explicitly empty nested collection removes only that namespace's owned,
  unprotected children. Direct `delete -f` removes declared owned children
  without pruning undeclared siblings.
- Cross-namespace and gateway-alias dependency checks retain foreign/unowned
  users and order detachments, creations, deletions, and replacements. Saved
  plans revalidate ownership, protection, collisions, and references at write
  time. API checks and writes are not atomic.
- A gateway owner can cascade-delete the whole gateway, including foreign,
  unowned, and protected children. Warnings describe affected families,
  namespaces, and protected descendants. Gateway protection remains enforced;
  no additional confirmation flow is introduced.
- Unselected write-only object arrays retain public siblings such as header
  names. Secrets are not silently rewritten or embedded in plans/logs. See
  the family guides for credential evidence and its limits.
- Dump translates child ownership into `kongctl` metadata and preserves
  unrelated metadata. Malformed ownership records are warned and skipped
  individually. Sync rejects malformed owned prune candidates during planning.

## Migration checklist

1. Inventory existing children and their intended namespace/protection. A
   matching unowned object fails planning; old public labels do not adopt it.
2. Explicitly adopt or transfer each existing child through the API, using
   the family guide. References and source defaults never grant authority.
   Do not apply a generated dump as an implicit adoption operation.
3. Set each source's `_defaults.kongctl.namespace` or explicit child metadata.
   Review the plan, especially empty collection scope and name collisions.
   Names remain gateway-wide identities within each family.
4. Unprotect with a metadata-only operation before payload updates, credential
   rotation, or deletion. Group owners retain control of the complete optional
   membership set, including foreign consumers; omission preserves membership.
5. Keep the five-entry metadata limit in view. Four entries are generated,
   leaving one unrelated entry. Omission preserves unrelated keys; renaming a
   key currently requires API removal of the old key first. See [the staged
   managed_by migration guide][migration] for later resource enrollment.

## Deferred operations and API limitations

Runtime certificate PUT requires the full private key; GET redacts it and
metadata PATCH returns HTTP 405. Independent ownership remains deferred until
there is a credential-preserving metadata update. Full PUT semantics are
accepted; the missing metadata operation is the blocker.

HashiCorp AppRole vault PUT clears an omitted `secret_id`. Metadata-only writes
against a vault dependent on that redacted value are rejected before mutation.
Creation and eligible unprotected updates with an explicitly selected secret
remain supported; protected records still require separate unprotection. A
public `secret_id_file` configuration follows the existing contract.

Vault references inside credentials are redacted. Retained opaque credential
users can therefore conservatively block vault deletion even if they do not
actually reference it. Explicit user deletion can establish safety;
metadata-only updates cannot. For MCP auth references, prefer names or
explicitly declared `!ref ...#name` references over undeclared raw UUIDs.

Provider basic-header continuity was verified behaviorally. Other credential
checks compare values in fresh synchronized runtime configuration; those
checks demonstrate stored retention, not successful external authentication.
AppRole retention after a rejected write is not proof of a safe AppRole PUT.
See the linked family guides for the exact evidence.

[contract]: ai-gateway-child-ownership.md
[models]: ai-gateway-model-ownership.md
[providers]: ai-gateway-provider-ownership.md
[auth]: ai-gateway-auth-strategy-ownership.md
[policies]: ai-gateway-policy-ownership.md
[custom]: ai-gateway-custom-policy-ownership.md
[agents]: ai-gateway-agent-ownership.md
[remaining]: ai-gateway-remaining-child-ownership.md
[migration]: contributor/managed-by.md
