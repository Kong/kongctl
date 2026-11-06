# Inspect Token Vault credentials

A Credential Provider describes an upstream service and its OAuth settings.
Its credentials hold access to that service: either shared values/enrollments
configured by an administrator, or individual authorizations obtained by a
principal. These differ from principal API keys, which authenticate a principal
to Kong Identity.

Use an existing provider in a Vault-enabled directory named `default`. The
[provider guide](vault-providers.md) covers existing-directory preparation and
missing-default bootstrap. Reading credentials does not adopt the directory,
provider or credential.

```sh
./kongctl get identity vault providers --directory-name default -o text

./kongctl get identity vault provider credentials \
  --directory-name default --provider-name billing-service -o yaml
```

Provider selection accepts `--provider-id` or exact `--provider-name`.
Credential selection uses its UUID from that collection:

```sh
./kongctl get identity vault provider credential CREDENTIAL_UUID \
  --directory-name default --provider-name billing-service -o json

./kongctl list identity vault provider credentials \
  --directory-name default --provider-name billing-service --page-size 1
```

Shared metadata includes `credential_type`, `status`, nullable `selector` and
`priority`, plus ID/timestamps. A null routing pair is the default credential.
Additional shared credentials can use CEL selectors and distinct priorities;
lower priorities are considered first. Selectors can repeat, so they are not
unique names. Q45 asks for the formal priority/default-slot contract because
the current API description says only one static credential is supported.

User entries include principal ID/display name, active status and nullable
expiration. A principal's own collection shows the provider and template:

```sh
./kongctl get identity principal vault credentials \
  --directory-name default --principal-id PRINCIPAL_UUID -o text

./kongctl get identity principal vault credential CREDENTIAL_UUID \
  --directory-name default --principal-id PRINCIPAL_UUID -o yaml
```

Both commands follow complete cursor inventories. A UUID from another parent
is not selected. JSON/YAML expose public metadata; text provides a summary
table. Credential values, access/refresh tokens and enrollment URLs are
excluded from output. Unknown/incomplete inventories return an error instead
of a partial list.

Provider credential detail GET in the API issues a fresh OAuth enrollment URL.
These metadata commands select through the collection, so reading one pending
credential does not issue a new enrollment capability. A pending record still
needs OAuth consent; inspection does not complete that flow.

[Static shared credential declarations](vault-shared-credentials.md) support
explicit material replacement and scoped retirement. OAuth enrollment and user
revocation are separate workflows.
[Shared OAuth enrollment](vault-oauth-enrollment.md)
provides explicit private URL capture/reissuance and shared retirement; user
enrollment/revocation still needs a supported runtime fixture. Static PUT
requires material again;
omitting its routing fields selects the default slot. OAuth PUT issues a fresh
enrollment URL. Ordinary configuration changes therefore cannot assume secret
preservation. See [API questions](../../../kong-identity-api-questions.md),
Q30/Q45/Q46, for runtime prerequisites and update/cardinality contracts.

Static credential writes also need a registered public encryption key in the
directory. See [Vault keys](vault-keys.md) for key registration and private-key
retention. Enabling Vault or creating a provider alone does not supply a key.
Static declarations use guarded writes, including explicit material replacement
that preserves routing. Public observations cannot verify stored material.

No `labels` or `managed_by` storage exists on provider credentials. This
inspection stamps no metadata and grants no sync or deletion authority
over either shared credentials or user authorizations. Actual user enrollment
and revocation still need verification in a supported runtime context; empty
administrative collections do not establish that behavior.
