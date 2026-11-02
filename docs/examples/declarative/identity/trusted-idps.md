# Manage directory Trusted IdPs

A directory Trusted IdP identifies an OIDC issuer whose tokens the directory
trusts. Its configuration controls JIT principal provisioning, claim mapping
and Token Vault access. It is separate from an OAuth client or an auth-server
identity provider.

Use a directory with Token Vault enabled (`vault.enabled: true` in YAML) and
an existing Trusted IdP. The API returns 403 while Vault is disabled,
including when trust records already exist.
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

./kongctl get identity trusted-idp principals \
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

./kongctl get identity trusted-idp principals \
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

## Declare trust configuration

[trusted-idps.yaml](trusted-idps.yaml) declares all public trust settings,
including the claim names used for JIT provisioning. Set
`IDENTITY_WORKFORCE_ISSUER` to your OIDC issuer's exact URL. The API validates
reachable discovery/signing keys; a placeholder hostname cannot be applied.
An explicit `jwks_uri` overrides discovery. Neither URL is a secret.

Trusted IdPs have no labels or `managed_by` store. Their namespace and
update/delete protection therefore come from an explicitly managed directory.
For an existing unmanaged `default`, prepare it first:

```sh
./kongctl adopt identity directory default --namespace identity-directory
./kongctl apply -f docs/examples/declarative/identity/vault-directory.yaml
```

Use the same namespace for directory preparation and trust declarations. An
existing disabled Vault must be enabled in a separate directory-only step so
its current trust collection can be observed before planning changes. If
`default` is missing, the trust example creates it and the trust record in one
plan. External directory references authorize reads and independently owned
principals, but cannot authorize trust writes.

```sh
./kongctl plan --mode apply \
  -f docs/examples/declarative/identity/trusted-idps.yaml \
  --output-file trust-plan.json
./kongctl apply --plan trust-plan.json --auto-approve
./kongctl get identity trusted-idps --directory-name default -o yaml
./kongctl diff -f docs/examples/declarative/identity/trusted-idps.yaml
```

Matching uses directory plus exact `issuer_url`. Ref is local; display names
are nonunique. Changing an issuer creates another trust record. Apply retains
the old issuer; explicitly scoped sync can retire it after creating its
replacement and checking associations. It does not retarget any identities.

Omitted fields preserve fresh public settings during an update. Explicit
`display_name: null` clears the display name; `jwks_uri: null` clears its
override and selects discovery. `jit: {}` clears claim mapping; omission keeps
it. Mapping is a complete object, so supplying a replacement mapping removes
its omitted entries. Boolean fields accept explicit false, not null. Updates
use hydrated full PUT because the deployed PATCH does not clear a null JWKS
override (Q38). The API has no conditional version check, so concurrent writers
can still race with the fresh read.

JIT mapping describes runtime provisioning. It does not grant ownership of
any resulting principals or groups, and applying the configuration does not
exercise an OIDC login, token exchange or JIT provisioning flow.

## Sync, dump and retire

Omitting `trusted_idps` preserves the collection. Declaring it authorizes sync
to prune omitted issuers under that managed directory; `trusted_idps: []`
selects all trust records there. Review that complete collection carefully.

```sh
./kongctl plan --mode sync \
  -f docs/examples/declarative/identity/trusted-idps.yaml
./kongctl dump declarative --resources identity.directories \
  --include-child-resources --output-file identity-dump.yaml
```

Dump includes public trust configuration only beneath enabled managed
directories. Null overrides and empty mappings are exported explicitly so
reload preserves them. Reading or dumping an unmanaged directory does not
adopt it or grant trust ownership.

A trust record cannot be retired while it has associated principals,
including inactive ones. Explicitly delete or retarget every matching OIDC
identity first. Independently authorized detachments in the same plan are
ordered before retirement; otherwise planning fails. A saved plan checks
fresh associations again before deletion. Trust ownership never authorizes
deleting a principal or identity. Unknown/unobservable associations block
deletion rather than guessing their origin.

For resources managed in different namespaces, retain each resource's existing
namespace. An explicitly empty identity collection uses the source file's
default namespace for sync selection; it does not inherit an overridden
principal namespace. Use a separate source file with that identity namespace
as `_defaults.kongctl.namespace`, and explicitly retain the managed directory's
namespace. This follows ordinary independent-child sync behavior.

Native trust deletion leaves associated principals and OIDC identities
readable; runtime behavior after deletion is an open API question (Q44).
Client observations cannot exclude a concurrent association before DELETE.
The API also deletes trust records with their directory even when
`force=false`, and hides them while Vault is disabled. Directory deletion
therefore requires explicitly authorized trust retirements followed by fresh
empty Vault inventories. See the
[API questions](../../../kong-identity-api-questions.md), Q37/Q38/Q44.
