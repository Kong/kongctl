# DCR Providers Example

This example demonstrates how to manage Konnect Dynamic Client Registration
(DCR) providers with `kongctl` declarative configuration.

The configuration uses the `dcr-providers-example` namespace so it stays
isolated from other declarative resources in the same Konnect organization.

## Files

- `dcr-providers.yaml` - declares standalone DCR provider resources for
  `okta`, `http`, `auth0`, `azureAd`, and `curity`
- `oidc-auth-strategy-with-dcr-provider.yaml` - declares one Okta DCR
  provider and one OIDC application auth strategy that references it with
  `dcr_provider_id: !ref`

## Usage

Preview the change set:

```bash
kongctl diff -f dcr-providers.yaml
```

Set the `OKTA_DCR_TOKEN`, `HTTP_DCR_API_KEY`,
`AUTH0_INITIAL_CLIENT_SECRET`, `AZURE_AD_INITIAL_CLIENT_SECRET`, and
`CURITY_INITIAL_CLIENT_SECRET` variables before execution. Planning validates
their deferred `!secret` declarations without reading the values.

Apply the configuration:

```bash
kongctl apply -f dcr-providers.yaml --auto-approve
```

Apply the OIDC auth strategy example that references a DCR provider:

```bash
kongctl apply -f oidc-auth-strategy-with-dcr-provider.yaml --auto-approve
```

Check the created DCR providers:

```bash
kongctl get dcr-providers -o yaml
```

Delete the example resources when you are done:

```bash
kongctl delete -f dcr-providers.yaml --auto-approve
```

## Notes

DCR provider `dcr_config` values usually include provider credentials such as
tokens, client IDs, client secrets, or API keys. Write-only credentials use
`!secret` with deferred environment sources in this example.

Some DCR provider configuration fields are write-only. Konnect may omit
secrets such as `dcr_token`, `api_key`, and `initial_client_secret` from get
and list responses. Use `--write-secret` or `--write-secrets` when explicitly
rotating those fields on existing providers.

Azure AD DCR uses the Azure v1 issuer format. The issuer should look like:

```text
https://sts.windows.net/<tenant-uuid>
```

Do not use the Azure v2.0 issuer form and do not add a trailing slash.
Konnect also appears to validate that the tenant UUID is a real Azure tenant,
not just a syntactically valid UUID. Placeholder UUID values may be rejected
even if the issuer URL format is otherwise correct.

The `oidc-auth-strategy-with-dcr-provider.yaml` example shows how to attach an
OIDC application auth strategy to a DCR provider by referencing it through
`dcr_provider_id: !ref <dcr-provider-ref>`.
