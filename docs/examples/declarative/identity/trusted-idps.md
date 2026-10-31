# Directory Trusted IdP inspection

A directory Trusted IdP identifies an OIDC issuer whose tokens the directory
trusts. Its configuration controls JIT principal provisioning, claim mapping
and Token Vault access. It is separate from an OAuth client or an auth-server
identity provider.

This preview provides inspection commands. Use a directory with Token Vault
enabled (`vault_enabled: true`) and an existing Trusted IdP. The API returns
403 while Vault is disabled, including when trust records already exist.
See [directory examples](README.md) for using or managing `default`.
Reading a directory or IdP does not adopt it.

```sh
./kongctl get identity trusted-idps --directory-name default -o json
```

Select an IdP by its UUID or exact issuer URL. Multiple IdPs can share a
display name; display names are not selectors.

```sh
export KONGCTL_IDENTITY_TRUSTED_ISSUER='https://your-issuer.example.com'

./kongctl get identity trusted-idp \
  --directory-name default \
  --issuer-url "$KONGCTL_IDENTITY_TRUSTED_ISSUER" -o yaml

./kongctl get identity trusted-idp-principals \
  --directory-name default \
  --issuer-url "$KONGCTL_IDENTITY_TRUSTED_ISSUER" -o text
```

JSON/YAML include the public trust settings and JIT mapping. Text output is a
table. Both collections follow all cursor pages; `--page-size 1` is useful
when inspecting pagination. A Vault-disabled or failed request is reported as
an error rather than an empty collection.

The associated-principal list includes principals with an OIDC identity whose
issuer matches this IdP, including manually created associations. Association
does not establish that the IdP provisioned or owns the principal.

[trusted-idp-principals.yaml](trusted-idp-principals.yaml) demonstrates that
relationship using the externally referenced `default` directory. Set the
issuer variable above to an existing IdP's exact issuer, then run:

```sh
./kongctl plan --mode apply \
  -f docs/examples/declarative/identity/trusted-idp-principals.yaml

./kongctl apply \
  -f docs/examples/declarative/identity/trusted-idp-principals.yaml

./kongctl get identity trusted-idp-principals \
  --directory-name default \
  --issuer-url "$KONGCTL_IDENTITY_TRUSTED_ISSUER" -o json
```

The example manages a principal and its OIDC association with independent
`managed_by` metadata. Set `claim.value` to the subject you intend to associate.
The Trusted IdP and directory remain externally configured. The new principal
is directly provisioned; this example does not execute an OIDC login or JIT
provisioning flow.

Remove the example's managed resources with:

```sh
./kongctl delete \
  -f docs/examples/declarative/identity/trusted-idp-principals.yaml
```

Declarative trust configuration is pending the ownership/deletion boundary.
Trusted IdPs have no labels or `managed_by` store. The API also deletes them
with their directory even when `force=false`; it cannot expose those records
while Vault is disabled. Those contracts and JWKS PATCH clearing are recorded
in [Q37/Q38](../../../kong-identity-api-questions.md).
