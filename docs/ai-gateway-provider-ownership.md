# Federated AI Gateway model providers

Model providers participate in independent namespace ownership alongside
models. This changes existing provider reconciliation: a managed gateway or
legacy public kongctl labels do not authorize provider mutation. Existing
providers without ownership require explicit adoption before updating them.
Unowned providers remain untouched by sync.

Declare `kongctl.namespace` and `kongctl.protected` on a root or nested
provider, or use source `_defaults.kongctl`. The source default takes
precedence over the literal `default`; gateway metadata is not inherited.

```yaml
_defaults:
  kongctl:
    namespace: providers-team
ai_gateway_model_providers:
  - ref: shared-openai
    ai_gateway: !lookup {name: shared-ai-gateway}
    name: shared-openai
    display_name: Shared OpenAI
    type: openai
    config:
      auth:
        type: basic
        headers:
          - name: Authorization
            value: !secret {source: !env OPENAI_AUTHORIZATION}
```

The provider stores ownership/protection in reserved lowercase `managed_by`
keys `kongctl-namespace` and `kongctl-protected`, alongside `tool: kongctl`
and the executing binary's `version`. These four entries leave room for one
unrelated entry within the five-entry API limit. All four keys are reserved
in manifests. Unrelated metadata is merged again immediately before PUT.
Public labels remain user data. Omitted labels are preserved from a fresh
execution-time read, including labels added since planning. An explicit
empty label map clears public labels.

Ownership is cooperative client behavior. To adopt an existing provider,
review its current owner, dependencies, protection, complete public payload,
and API-specific credential preservation contract. Through an explicit
out-of-band API update, set `kongctl-namespace` to the intended namespace
and `kongctl-protected` to `true` or `false`, preserving unrelated metadata
and credentials. Defaults and gateway references never perform this step.
Transfer from another namespace requires the same explicit coordination.
Do not copy redacted GET data into a PUT without verifying credential
preservation for that provider/authentication variant.

Apply never prunes. For last-provider deletion, sync an external gateway
with an explicitly empty collection in the owning source:

```yaml
_defaults:
  kongctl: {namespace: providers-team}
ai_gateways:
  - ref: shared
    _external:
      selector:
        matchFields: {name: shared-ai-gateway}
    model_providers: []
```

Omitting `model_providers` leaves the collection unmanaged. Foreign and
unowned providers, the gateway, and other collections remain untouched.
A retained model in any namespace, including an unowned model or a semantic
embedding-provider reference, prevents unsafe direct provider deletion.
Combined detach/delete plans order those operations across gateway aliases.
Provider names and types remain immutable; create a differently named
provider and move users before deleting the old one.

Protected providers must be unprotected in a separate metadata-only update.
That update cannot change public payload fields, unrelated metadata, or
secrets. Saved plans and direct adapter calls recheck current ownership,
protection, name/type, merged capacity, and creation-name collisions.
The API does not provide an atomic ownership check/write transaction.
The gateway owner can still cascade-delete the gateway with warnings.

Header and parameter values, Azure client secrets, AWS secret keys/session
tokens, and GCP service-account JSON use `!secret` declarations. Parameter
values are write-only even for public inputs such as `api-version`; put
those values in a deferred source too. Select `--write-secret` or
`--write-secrets` for explicit updates. Unselected object-array values are
omitted while names/locations remain. No ownership change resolves or
rotates credentials implicitly.

Live validation covers ownership metadata on all 20 provider types. Typesafe
requires a gateway minimum runtime of 2.2; the other tested types accept 2.1.
A behavioral probe verified identical basic-header credentials before and
after a CLI metadata-only PUT. For basic headers and parameters, Azure client
secrets, AWS keys/session tokens (including nested SageMaker credentials),
and GCP service-account JSON, fresh runtimes synchronized to Konnect's
expected config version retained identical credential fields after both
protection and unprotection PUTs. Values were compared in memory, never
printed or embedded in plans. These runtime checks verify backend credential
preservation; they do not claim successful OAuth exchanges with cloud
identity services using synthetic credentials.

Dumps translate provider ownership to `kongctl` and remove generated
`managed_by` entries. They retain public labels and unrelated metadata;
write-only credentials are not reconstructed from API responses. If a
provider has protection metadata without an ownership namespace, dump warns
before omitting that ineffective protection key. It does not infer ownership
or adopt the provider.

See [the shared ownership contract](ai-gateway-child-ownership.md) and
[runnable manifests](examples/declarative/ai-gateway-federated-providers).
