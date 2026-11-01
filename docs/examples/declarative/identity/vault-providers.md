# Inspect Token Vault Credential Providers

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

The commands here inspect public configuration. Provider declarations and
credential enrollment are separate implementation work. See the
[public-key example](vault-keys.md) for the existing Vault declarative resource.

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
ownership. Provider declarative writes need explicit parent and collection
authority before they can support adoption or sync.

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

See the [API questions](../../../kong-identity-api-questions.md) and
[metadata ledger](../../../contributor/kong-identity-metadata-storage.md).
