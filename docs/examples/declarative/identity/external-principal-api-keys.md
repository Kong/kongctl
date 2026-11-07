# API keys for imported or externally managed principals

An organization-chart importer can manage the person while KongCTL manages
selected generated API keys. The principal and directory stay external;
KongCTL does not change their provisioning, names, labels or metadata.

Find the person's UUID:

```sh
kongctl get id principals --display-name "Jordan Example" -o json
```

Display names are not unique. Verify the correct person before selecting
their UUID. Principal external references currently require UUIDs; email and
external-ID lookups are not supported.

```sh
export WORKFORCE_PRINCIPAL_ID='<verified-principal-uuid>'
```

Apply `external-principal-api-keys.yaml`, capturing the generated secret in
a private output directory:

```sh
kongctl apply -f external-principal-api-keys.yaml \
  --secret-output-dir ./private-key-material
```

The generated key secret is saved privately and is not included in plans or
declarative dumps. Do not commit these files. Subsequent applies do not issue
a new key when the existing namespace/ref matches.

```sh
kongctl diff --mode apply -f external-principal-api-keys.yaml
```

Read the key's public metadata:

```sh
kongctl get id principal api-keys \
  --principal-id "$WORKFORCE_PRINCIPAL_ID" -o json
```

The key has five immutable management entries: `tool`, `version`,
`kongctl-ref`, `kongctl-namespace`, and `kongctl-create-type`. These identify
generated keys without relying on a unique name or owning the principal.
Matching occurs locally within the principal's complete key collection.
Multiple keys with the same namespace/ref cause a conflict; resolve the
duplicate explicitly before continuing.

Name and expiration changes, explicit deletion, and namespace-scoped sync
are supported. Omitted expiration preserves its value; `expires_at: null`
clears it. With sync, omitting `api_keys` leaves keys alone; `api_keys: []`
retires this source namespace's generated keys. Foreign, unowned, legacy,
and other-namespace keys remain outside this authority.

Dump includes the managed keys beneath external principal UUID references:

```sh
kongctl dump declarative --resources identity.directories \
  --include-child-resources --output-file workforce-dump.yaml
```

The dump can include other resources from the directory. Inspect it before
using it as configuration. An unchanged apply-mode plan should have no drift.

Existing keys cannot be adopted by this feature. The API accepts management
metadata during key creation, but PATCH currently ignores later management
metadata changes. Consequently, changing the namespace/ref requires explicit
key replacement. Key-level `kongctl.protected` is unsupported: the API's
five-entry metadata limit and immutable map do not permit that lifecycle.
Keys under managed principals keep their existing inherited ownership and
protection behavior; group API keys still require managed groups.
