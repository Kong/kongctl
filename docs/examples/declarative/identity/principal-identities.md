# Principal identities

A directory principal can represent a user or agent. An identity links it to
an identifier used by an external authentication or lookup system. A
principal can have several identities; these associations are separate from
API-key or password credentials and from a Trusted IdP's configuration.

| Identity type | Association fields | Meaning |
| --- | --- | --- |
| `oidc` | `issuer`, `claim.name`, `claim.value` | Match a claim from a specific issuer. |
| `auth_server_client` | `auth_server_id`, `client_id` | Associate an OAuth client with the principal. Client IDs are opaque strings. |
| `control_plane_consumer` | `control_plane_id`, `consumer_id`, optional `workspace_id` | Associate a Gateway consumer. The response also reports read-only `control_plane_type`. |
| `custom` | `key`, `value` | Associate an application-defined lookup identifier. |

This increment supports `get` and `list` for all four variants. Declarative
configuration, adoption and mutation workflows remain In Progress in #1416.
The API has retained `labels` and all five `managed_by` ownership/ref values
on direct create, GET, list and PUT probes for each variant. This establishes
the next implementation's metadata capability; the read commands do not
stamp or adopt anything.

## Inspect associations

Choose the directory by name or UUID and the principal by UUID. Display names
are not unique, so they cannot select a principal. First inspect principals:

```sh
kongctl get identity principals --directory-name default
```

Then list its identities or get one by its generated UUID:

```sh
kongctl get identity principal-identities \
  --directory-name default --principal-id PRINCIPAL_UUID --page-size 1
kongctl get identity principal-identity IDENTITY_UUID \
  --directory-id DIRECTORY_UUID --principal-id PRINCIPAL_UUID -o yaml
```

The explicit product form is also supported, for example
`kongctl get konnect identity principal-identities ...`. JSON/YAML includes
the relevant type fields, labels, management metadata and timestamps. Text
lists show ID/type; text details show the public configuration. Read-only
`control_plane_type` distinguishes `api_gateway` and `ai_gateway` where the
API returns that field. Workspace omission is preserved.

Every list traverses the complete cursor collection. A failed page, repeated
ID/cursor, missing collection or unknown response variant rejects the entire
observation. Detail reads first prove membership in the selected principal's
complete collection: direct API GET currently returns an identity through
another existing principal's path (Q21). The same observed API discrepancy
affects DELETE, so future writers and fixture cleanup must also verify current
membership rather than trusting a detail response alone.

## Verified API limits and remaining work

Creation generated IDs without user-supplied IDs for all four types. Duplicate
custom and OIDC payloads were rejected both under the same principal and
another principal in the directory; a duplicate OAuth-client association was
also rejected. Those checks do not establish uniqueness across directories.
Future reconciliation will use persisted refs and complete duplicate checks.

PUT retained management metadata for all four variants and updated the
tested custom/OIDC association fields. Retargeting OAuth-client/consumer
associations still needs verification before declarative updates. PATCH
accepts type-specific field updates but rejects the probed labels/managed_by
request; its schema does not offer those metadata fields. Safe declarative
updates will need a fresh full public payload for PUT, including preservation
of omitted fields. Type changes were rejected in the direct PUT probe.

The test organization rejected its eleventh identity with a quota of ten
(Q23). Applicable quota scope/discovery remains a team question; this is not
a new hard-coded configuration limit. Q22 records the shared cursor-query
serialization mismatch; the preview SDK corrects the three implemented list
request types.

Next work in #1416 is declarative nested/grouped loading, persisted-ref
matching, typed and external references, ownership/protection, saved plans,
dump, adoption, safe scoped sync/delete, and lifecycle scenarios. Existing
importer/JIT/Portal identities must remain outside management until explicitly
adopted under a verified contract.

[Canonical identity schemas](https://github.com/Kong/platform-api/blob/98137d7be3772b67693cc1ceb0469e77f8ecf0f5/src/konnect/definitions/kong-identity/v2/openapi.yaml),
[API questions](../../../kong-identity-api-questions.md),
[metadata storage](../../../contributor/kong-identity-metadata-storage.md).
