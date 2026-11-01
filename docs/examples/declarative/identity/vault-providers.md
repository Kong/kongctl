# Manage Token Vault Credential Providers

A Credential Provider describes an upstream service for which a directory's
Token Vault manages credentials. For example, one GitHub provider can describe
the OAuth application, requested scopes and endpoint settings used to enroll
principals with GitHub. A static-secret provider describes a service accessed
with an existing API key or other secret.

This differs from an auth-server OAuth client: that client requests tokens
from a Kong authorization server. A Vault provider describes access to another
service. Its `client_id`, when present, belongs to that upstream service's OAuth
application. Its OAuth client secret and the resulting upstream credentials are
separate values.

The [provider declarations](vault-providers.yaml) cover static-secret, static
OAuth application and dynamic registration configurations. These declare
provider settings; credential enrollment and runtime token retrieval are
separate operations. [Static shared credentials](vault-shared-credentials.md)
add deferred material and explicit replacement. See the
[public-key example](vault-keys.md) for encryption key registration.

## Prepare the default directory

An existing unmanaged `default` directory must be explicitly adopted. Provider
resources have no independent ownership metadata, so they require a managed
directory in the same declaration:

```sh
kongctl adopt identity directory default --namespace identity-directory
kongctl apply -f docs/examples/declarative/identity/vault-directory.yaml
```

Use the same namespace for directory preparation and provider declarations.
If the directory is missing, the provider example can create it and its
providers together. For an existing disabled Vault, first enable it with a
directory-only declaration: its hidden provider collection must be observable
before planning provider changes. Reads and external directory references
grant no write authority. Adoption does not independently stamp child metadata.

## Declare and inspect providers

Set the upstream application ID and supply its secret through the execution
environment, then adjust the example endpoints and scopes for your service.
Plan validates the secret source without reading its value:

```sh
kongctl plan --mode apply \
  -f docs/examples/declarative/identity/vault-providers.yaml \
  --output-file providers-plan.json
kongctl apply --plan providers-plan.json --auto-approve
kongctl get identity vault-providers --directory-name default -o yaml
```

`VAULT_GITHUB_CLIENT_ID` is public configuration and is read when loading the
file. `VAULT_GITHUB_CLIENT_SECRET` is deferred until execution. Its value is
absent from plans, diffs, dumps and ordinary read output. Keeping its source in
the file does not rewrite an existing secret. Explicitly select a later write:

```sh
kongctl plan --mode apply \
  -f docs/examples/declarative/identity/vault-providers.yaml \
  --write-secret 'github#client_secret' --output-file secret-plan.json
kongctl apply --plan secret-plan.json --auto-approve
```

The selection is stored in the saved plan. Secret updates recheck directory
identity, namespace, protection and provider context even without public drift.
Public reads cannot verify the stored secret value; runtime enrollment is
outside this example's verification boundary.

An initial `enabled: false` needs POST followed by PATCH because the create
API has no enabled field. This creates a brief enabled interval. If the PATCH
fails, inspect the created provider and generate another plan; creation is not
atomically disabled. Later false/true settings use PATCH normally.

Provider names are unique per directory and determine matching; refs are local
identifiers. Changing a ref preserves the matching provider. Changing a name
creates another provider: apply retains the old one, while explicitly scoped
sync registers the replacement before retiring the old one after dependency
checks. Template, static client ID,
credential type, DCR mode and registration endpoint changes require explicit
replacement in this implementation; the verified PATCH schema lacks them.
This is a client compatibility boundary, not a claim of API immutability.

To replace one provider, first remove its entry while keeping the directory
declared, review a sync plan and retire that provider. Then add the entry back
with the new settings and apply. Its credentials must be removed before
retirement; configuration replacement does not migrate credentials. The new
provider receives a new ID. Review the complete declared collection before
sync because it also selects other omitted providers for deletion.

## Sync, dump and delete

Omitting `vault_providers` preserves existing providers. Declaring the
collection authorizes sync to prune providers omitted from it. An explicit
empty list selects all providers for retirement under that managed directory.
Provider updates/deletion inherit directory protection. A provider with any
attached credential requires exact explicitly authorized static credential
retirements first. Saved execution still requires its actual empty collection;
a newly attached or undeclared credential blocks deletion. Unknown or incomplete
observations also block deletion.

```sh
kongctl diff -f docs/examples/declarative/identity/vault-providers.yaml
kongctl sync -f docs/examples/declarative/identity/vault-providers.yaml
kongctl dump declarative --resources identity.directories \
  --include-child-resources --output-file identity-dump.yaml
```

Dump exports public provider configuration under enabled managed directories.
It omits secrets and pins observed effective endpoints; the API does not expose
whether these were overrides or template defaults. Unreproducible empty scope
lists and invalid empty endpoints cause an export error. Add the required
secret sources before recreating an upstream OAuth application configuration
that depends on them.

Deleting the full example also selects its directory. Only explicitly declared
providers can be retired first; other Vault keys/providers/IdPs and independent
directory resources must satisfy the existing deletion guards. Remove attached
credentials explicitly before selecting their provider for deletion. A future
atomic API dependency guard is tracked in Q41; observations cannot exclude a
concurrent attachment between the final check and DELETE.

## Discover templates

Templates are a built-in, read-only catalog. They are not desired-state
resources and require no directory selector or enabled Vault:

```sh
kongctl get identity credential-provider-templates -o text
kongctl get identity credential-provider-template github -o yaml
kongctl list identity provider-templates --page-size 1 -o json
```

The catalog exposes the template name, display name, secret type, supported
bearer methods, default scopes, endpoint defaults and optional PKCE requirement.
The inspected preview contains eight templates: `github`, `slack`, `google`,
`atlassian-rovo`, `datadog`, `miro`, `custom-oauth`, and `static_secret`.
The `custom-oauth` template lets provider configuration supply endpoint URLs.

`secret_type: authorization_code` describes OAuth enrollment. It does not mean
the catalog includes a secret. `secret_type: static_secret` describes a
pre-issued credential provisioned through a separate credentials API.

## Inspect providers under default

An organization normally has a directory named `default`. Reads do not adopt
it, change its management metadata or enable its Vault:

```sh
kongctl get identity vault-providers --directory-name default -o text
kongctl get identity vault-provider github-production \
  --directory-name default -o json
kongctl get identity vault-provider PROVIDER_UUID \
  --directory-name default -o yaml
```

Provider names are unique within a directory. A positional UUID selects a
route ID; `--provider-name` explicitly selects a name, including one shaped
like a UUID:

```sh
kongctl get identity vault-provider --provider-name PROVIDER_NAME \
  --directory-name default -o json
```

`credential-provider` and `credential-providers` are aliases for these provider
commands. JSON/YAML expose public settings, including enabled state, template,
credential type, scopes, static client ID when present, PKCE/DCR settings,
refresh buffer, effective endpoints and timestamps. Text summarizes the
provider's identity and mode. List operations traverse every cursor page.

The API returns resolved `*_endpoint_effective` values. These do not reveal
whether an endpoint came from an explicit override or the template default.
`dcr_enabled: true` indicates dynamic registration rather than a fixed
administrative client ID. Creating that configuration does not establish that
runtime registration or OAuth consent has completed.

`credential_type: shared` means credentials are administered for the provider;
`user` means principals authorize it individually. Provider inspection does
not read those credential records or mint enrollment URLs. Client secrets,
credential values and enrollment/state capabilities are excluded from output.

## API boundaries

Providers have no `labels` or `managed_by`. Reading a name does not confer
ownership. Provider declarative writes use the explicit parent and collection
authority described above.

A disabled Vault hides its providers behind an API error; the command reports
that error instead of returning an empty collection. Template discovery still
works. Use explicit directory configuration to enable Vault where appropriate;
inspection does not perform that update.

Provider DELETE succeeds with an attached shared credential in the tested
`.tech` environment; a subsequent credential GET returns 404. A provider's mere
existence does not establish that deletion is safe. Q41 asks about conditional
deletion and user-enrolled dependencies. Q39/Q40/Q42 cover replacement,
initial enablement and scope/endpoint clearing. Runtime enrollment requires
supported mTLS context (Q30). Stored client-secret preservation is unverified.

Public PATCH settings, including DCR registration configuration omitted from
the request, persist in a subsequent GET. This does not prove runtime OAuth
enrollment. Some OAuth-related settings also persist on static-secret
providers; Q43 asks which have runtime meaning. Configuration persistence does
not demonstrate successful consent, dynamic client registration, credential
enrollment or runtime token retrieval.

See the [API questions](../../../kong-identity-api-questions.md) and
[metadata ledger](../../../contributor/kong-identity-metadata-storage.md).
