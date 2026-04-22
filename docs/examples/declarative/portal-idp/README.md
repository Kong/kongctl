# Portal Identity Provider Example

This example shows how to configure a Developer Portal OIDC identity provider
in declarative mode.

The OIDC issuer URL, client ID, and client secret are loaded from environment
variables with `!env` so they do not need to be stored in plaintext in the
declarative configuration.

Konnect validates the issuer URL against the IdP metadata endpoint. You must
provide a real issuer URL for an OIDC provider you control or have access to.

The configuration uses the `portal-idp-example` namespace so it stays isolated
from other declarative resources in the same Konnect organization.

## Migrating from `auth_settings`

Older portal declarative configs could place OIDC or SAML fields under
`portals[].auth_settings` or `portal_auth_settings`. That shape is no longer
accepted.

Use `auth_settings` only for the portal-level flags:

- `basic_auth_enabled`
- `konnect_mapping_enabled`
- `idp_mapping_enabled`

Move provider-specific values such as `oidc_issuer`, `oidc_client_id`,
`oidc_client_secret`, `oidc_scopes`, `oidc_claim_mappings`, and the SAML
equivalents to `identity_providers` or `portal_identity_providers`.

If you apply an old configuration, `kongctl` now fails validation and tells
you to move that configuration to `identity_providers`.

## Files

- `portal-idp.yaml` - creates a portal and configures a nested OIDC identity
  provider. The `config.issuer_url`, `config.client_id`, and
  `config.client_secret` fields use `!env`.

## Usage

Preview the change:

```bash
PORTAL_IDP_ISSUER_URL='https://your-idp.example.com/oauth2/default' \
PORTAL_IDP_CLIENT_ID='your-client-id' \
PORTAL_IDP_CLIENT_SECRET='your-client-secret' \
kongctl diff -f portal-idp.yaml
```

Apply the configuration:

```bash
PORTAL_IDP_ISSUER_URL='https://your-idp.example.com/oauth2/default' \
PORTAL_IDP_CLIENT_ID='your-client-id' \
PORTAL_IDP_CLIENT_SECRET='your-client-secret' \
kongctl apply -f portal-idp.yaml --auto-approve
```

Check the portal identity provider after apply:

```bash
kongctl get portal identity-providers --portal-name portal-idp-example -o yaml
```

Human-readable diff output redacts `!env` values by design. Use `get` after
apply to confirm the resolved values.

Delete the example when you are done:

```bash
PORTAL_IDP_ISSUER_URL='https://your-idp.example.com/oauth2/default' \
PORTAL_IDP_CLIENT_ID='your-client-id' \
PORTAL_IDP_CLIENT_SECRET='your-client-secret' \
kongctl delete -f portal-idp.yaml --auto-approve
```
