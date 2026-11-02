# Principal and group API keys

An API key is a credential presented to authenticate as a directory principal.
It differs from a principal identity, which associates that principal with an
external lookup identifier. Generated keys return their material at creation;
imported keys accept material supplied by the creator. Metadata reads cannot
recover either key's material or establish its creation method.

The preview branch provides scoped metadata inspection and explicit credential
lifecycle commands. For generated keys managed through plan/apply/sync/dump,
use the [declarative key tutorial](generated-api-keys.md). The explicit workflow
below also supports imported keys, which have no declarative adoption path.
Create parents under the existing default directory:

```sh
kongctl plan --mode apply -f api-key-parents.yaml
kongctl apply -f api-key-parents.yaml --auto-approve
```

If `default` is missing, first follow the separate directory bootstrap in
[the administration guide](README.md). The parent example manages the worker
and ordinary group, and leaves the directory external.

Find the principal and group UUIDs:

```sh
kongctl get identity principals --directory-name default
kongctl get identity groups --directory-name default
kongctl get identity principal api-keys \
  --directory-name default --principal-id PRINCIPAL_UUID --page-size 1
kongctl get identity principal api-key KEY_UUID \
  --directory-name default --principal-id PRINCIPAL_UUID -o yaml
```

Use an existing principal with keys. US `.tech` supports user, agent and
unclassified principals. Earlier US `.com` tests returned 404 for a user's
key collection even when principal GET succeeded (Q27). The command reports
such failures rather than treating them as empty collections. Principal
display names and key names cannot select records.
An explicit directory UUID works in place of its name.

`list` and explicit `konnect` forms are supported. JSON/YAML contain ID,
expiration, management metadata and timestamps, plus a name if the API returns
one. US `.tech` returns names; earlier US `.com` tests omitted them (Q29).
Text lists show IDs, names and expiration; text details show public metadata.
The output type has no credential field.
Complete cursor traversal handles next-page URLs, validates IDs and rejects
failed pages, repeated IDs/cursors and missing collections without returning
partial results.

Group reads use the same pattern with `group-api-keys` and `--group-id`.

## Create, rotate and retire

Use the UUID of the worker from `api-key-parents.yaml`. Omit a secret source
to generate a key. Choose a new capture path outside the checkout:

```sh
kongctl create identity principal api-key \
  --directory-name default --principal-id PRINCIPAL_UUID \
  --name primary --expires-at 2031-01-01T00:00:00Z \
  --secret-output-file ~/worker-key.private.json
```

Normal output contains public metadata, including the generated key UUID.
The new 0600 file contains the material and directory/principal/key IDs.
Existing files and symlinks are rejected before creation. Keep captures private.
If issuance or capture is uncertain, inspect the file and parent inventory
before trying again; there is no automatic second creation or rollback.

Import supplied material with `--secret-env VARIABLE` or `--secret-file PATH`.
Both generated and imported creation require a new `--secret-output-file`.
Material is not a command-line positional value and cannot be changed in place.
For rotation, create a second key, distribute it, then explicitly retire the
old UUID. Administrative metadata checks do not prove runtime authentication.

```sh
kongctl patch identity principal api-key KEY_UUID \
  --directory-name default --principal-id PRINCIPAL_UUID --name replacement
kongctl patch identity principal api-key KEY_UUID \
  --directory-name default --principal-id PRINCIPAL_UUID --no-expiration
kongctl delete identity principal api-key KEY_UUID \
  --directory-name default --principal-id PRINCIPAL_UUID --auto-approve
```

Name-only changes preserve freshly read expiration. Use `--expires-at` to set
a timestamp or `--no-expiration` to clear it. Null name clearing is unavailable.
These commands preserve existing management metadata and add no ownership
markers; the principal's declarative ownership does not automatically own keys.

Group commands use `group-api-key` and `--group-id GROUP_UUID`. Generated group
creation supports `--name`; imported group creation and all group PATCH name
changes are unavailable because the deployed API ignores those fields.
Expiration updates and explicit deletion work for both generated/imported keys.
Key names are never selectors. Delete requires fresh detail, complete parent
collection membership and a final observation that the key is absent.

Remove all created keys before retiring the tutorial parents:

```sh
kongctl delete -f api-key-parents.yaml --auto-approve
```

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
- US `.tech` accepts generated/imported principal-key names and scalar rename
  PATCH,
  but accepts duplicate names under one principal. Earlier US `.com` rejected
  named creation and ignored rename. Name is not a unique match key (Q29).
- The runtime endpoint requires mTLS context beyond an administration token.
  Credential continuity and revocation need a supported runtime setup (Q30).
- Principal deletion succeeds with attached keys. The key detail/collection
  then return 404; this does not by itself prove runtime revocation (Q24).

Declarative principal deletion now checks API-key descendants and refuses any
attached key, regardless of its management map. It also requires explicit
authorized identity/username/membership deletions and fresh empty checks.
These observations do not prevent a concurrent attachment atomically (Q24).

These are API observations, separate from the SDK's request-model limits.
Preview `90596b498bcb` includes the corrected cursor parameters and writable
nullable expiration. No further SDK revision is required for these commands.
Generated-key declarative integration is still being evaluated under #1417
and #2659; imported ownership, adoption and mutable ownership remain limited.

The [engineering question log](../../../kong-identity-api-questions.md) and
[metadata ledger](../../../contributor/kong-identity-metadata-storage.md)
track the outstanding contracts. Do not use a local ref or a parent principal's
ownership as authority to adopt an unowned credential.
