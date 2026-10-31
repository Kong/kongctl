# Principal API-key inspection

An API key is a credential presented to authenticate as a directory principal.
It differs from a principal identity, which associates that principal with an
external lookup identifier. Generated keys return their material at creation;
imported keys accept material supplied by the creator. Metadata reads cannot
recover either key's material or establish its creation method.

The preview branch provides scoped metadata inspection:

```sh
kongctl get identity principals --directory-name default
kongctl get identity principal-api-keys \
  --directory-name default --principal-id PRINCIPAL_UUID --page-size 1
kongctl get identity principal-api-key KEY_UUID \
  --directory-name default --principal-id PRINCIPAL_UUID -o yaml
```

Use an existing agent or unclassified principal with keys. The deployed API
returns 404 for a `user` principal's key collection even when principal GET
succeeds (Q27). The command reports that failure; it does not turn it into an
empty collection. Principal display names and key names cannot select records.
An explicit directory UUID works in place of its name.

`list` and explicit `konnect` forms are supported. JSON/YAML contain ID,
expiration, management metadata and timestamps, plus a name if the API returns
one. Current deployments omit name (Q29). Text lists show IDs and expiration;
text details show public metadata. The output type has no credential field.
Complete cursor traversal handles next-page URLs, validates IDs and rejects
failed pages, repeated IDs/cursors and missing collections without returning
partial results.

## Declarative work remains deferred

No API-key declarations or adoption commands are exposed yet. Direct API
probes establish these limits:

- Generated creation retains all five supplied ownership/ref entries in
  `managed_by`. Imported creation rejects that field (Q4).
- PATCH returns success but ignores partial and complete management-map
  changes for both kinds of key. Ref, namespace, protection and adoption cannot
  currently be updated through that operation (Q28).
- PATCH clears expiration when it is omitted. Explicit expiration updates and
  null clearing work, but metadata writes must preserve an existing expiration.
- Named creation is rejected, while named PATCH is ignored. Read responses
  omit name. No name-uniqueness rule is assumed (Q29).
- The runtime endpoint requires mTLS context beyond an administration token.
  Credential continuity and revocation need a supported runtime setup (Q30).
- Principal deletion succeeds with attached keys. The key detail/collection
  then return 404; this does not by itself prove runtime revocation (Q24).

The existing declarative principal deletion check covers identity associations.
It does not yet inspect API-key descendants. Do not rely on an API key's
management map to prevent the API's native parent deletion behavior.

These are API observations, separate from the SDK's request-model limits.
The cursor request fix is included in preview `70a2818dde13`. Expiration is
readable, but its mutable request field also needs a targeted schema/generation
correction before write support. Group keys have separate parent routes and
remain separate work under #2659.

The [engineering question log](../../../kong-identity-api-questions.md) and
[metadata ledger](../../../contributor/kong-identity-metadata-storage.md)
track the outstanding contracts. Do not use a local ref or a parent principal's
ownership as authority to adopt an unowned credential.
