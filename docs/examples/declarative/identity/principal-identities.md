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

The preview branch supports declarative plan, diff, apply, sync, delete, dump
and adoption for all four variants, plus get/list inspection. Issue #1416
stays In Progress until merge. Explicit workspace configuration is deferred
under Q25; existing workspace values remain visible in reads.

## Run the example

[principal-identities.yaml](principal-identities.yaml) creates a user, four
identity associations, and an authorization server with one OAuth client.
It selects the existing directory named `default`; change that explicit
selector if your directory has another name. The Gateway consumer must
already exist. Supply its IDs:

```sh
export KONGCTL_IDENTITY_CONTROL_PLANE_ID=CONTROL_PLANE_UUID
export KONGCTL_IDENTITY_CONSUMER_ID=CONSUMER_UUID
```

Save a plan and inspect the changes:

```sh
kongctl plan --mode apply \
  -f docs/examples/declarative/identity/principal-identities.yaml \
  --output-file identity-plan.json
kongctl diff \
  -f docs/examples/declarative/identity/principal-identities.yaml
```

Client creation returns an initial secret once. Apply with private capture:

```sh
identity_secrets_dir="$(mktemp -d)"
kongctl apply --plan identity-plan.json \
  --secret-output-dir "$identity_secrets_dir" --auto-approve
kongctl plan --mode apply \
  -f docs/examples/declarative/identity/principal-identities.yaml
```

The last plan should have no changes. Store the captured credential securely;
the identity association itself stores the public client ID. Plans, dumps
and normal command output omit the issued secret.

The example mixes nested `principals[].identities` with grouped
`identity.identities` declarations. An existing principal can instead be
declared with `_external: {id: PRINCIPAL_UUID}` under its directory. External
principal and identity references require UUIDs and verified membership in
their declared parent. Display names do not select principals. A remote
principal or identity cannot be declared as both managed and external in the
same configuration; reuse its managed ref for additional associations.

## Ownership and updates

Each identity stores five generated `managed_by` values: `tool`, `version`,
`kongctl-namespace`, `kongctl-protected`, and `kongctl-ref`. The parent
principal and directory have independent ownership. Identity operations can
use an external or protected parent without adopting or updating that parent.
Public labels retain their API meaning; omit `managed_by` from manifests.

Matching uses the complete parent collection and the tool/namespace/ref
tuple. Duplicate matches fail before mutation. Updates use a fresh full PUT
payload, preserve omitted public fields, and replace the tool-owned map.
Identity type changes require a separate association. Moving an existing
identity to another local ref or namespace requires explicit adoption:

```sh
kongctl adopt identity principal-identity IDENTITY_UUID \
  --directory-name default --principal-id PRINCIPAL_UUID \
  --ref employee-number --namespace identity-example
```

An already owned identity requires `--overwrite-namespace`, including when
changing only its persisted ref. Adoption retains public association fields
and protection. A protected identity requires a separate unprotection update
before changing its public fields or deleting it.

For sync, omission preserves the collection. An explicit `identities: []`
prunes only identities owned by the source namespace under that principal.
Foreign, unowned, external and protected identities are preserved or block
their own deletion; parent linkage does not adopt them.

Dump with children to export associations:

```sh
kongctl dump declarative --resources identity.directories \
  --include-child-resources > identity-dump.yaml
kongctl plan --mode apply -f identity-dump.yaml
```

Owned resources retain their refs and lifecycle metadata. Unowned principals
and identities export as external references. Workspace-bearing identities
also export as external while Q25 remains unresolved. The API ownership map
is omitted from the manifest. Cross-resource IDs in an export can refer to
existing resources in that environment; edit them when moving environments.

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
affects DELETE. Writes, adoption and fixture cleanup also verify current
membership before changing an association.

## Verified API limits

Creation generated IDs without user-supplied IDs for all four types. Duplicate
custom and OIDC payloads were rejected both under the same principal and
another principal in the directory; a duplicate OAuth-client association was
also rejected. Those checks do not establish uniqueness across directories.
Reconciliation uses persisted refs and complete duplicate checks. A conflicting
external association is never implicitly adopted by its payload.

PUT retained management metadata for all four variants and updated the
tested custom/OIDC association fields. Direct probes also verified retargeting
an OAuth-client identity to a different server/client and an API Gateway
consumer identity to a different control plane/consumer. An AI Gateway OAuth
consumer can be linked through the same `control_plane_consumer` shape;
the response reports `control_plane_type: ai_gateway`, and metadata-only PUT
retains that association. AI-to-API Gateway consumer retargeting also works.
PATCH accepts type-specific field updates but rejects the probed
labels/managed_by request; its schema does not offer those metadata fields.
Safe declarative
updates use a fresh full public payload for PUT, including preservation
of omitted fields. Type changes were rejected in the direct PUT probe.

The test organization rejected its eleventh identity with a quota of ten
(Q23). Applicable quota scope/discovery remains a team question; this is not
a new hard-coded configuration limit. Q22 records the shared cursor-query
serialization mismatch; the preview SDK corrects the three implemented list
request types.

Principal deletion currently succeeds with identities attached (Q24).
Declarative parent deletion requires explicit authorized identity deletions
first, and rechecks for an empty collection immediately before removing the
principal. Late or unplanned children block a saved deletion plan. Credentials
and memberships need their own verified deletion contracts.

OAuth-client deletion removes its identity association, while API Gateway
consumer/control-plane deletion can leave an association with the old IDs
(Q26). Plans order observed identity deletions or retargeting before deleting
referenced resources and save the observed parent scopes for fresh execution
checks. They do not inspect every directory/principal outside the declared
collections. An identity's metadata does not prevent native API deletion
effects outside those scopes.

Workspace PUT probes with text and UUID-shaped values returned 500. No known
valid workspace fixture was available, so Q25 asks for a working example and
the API's format, availability and clearing rules. Explicit workspace writes
and adoption of workspace-bearing identities remain deferred. Existing
importer/JIT/Portal identities stay external until explicitly adopted under
a verified contract.

[Canonical identity schemas](https://github.com/Kong/platform-api/blob/98137d7be3772b67693cc1ceb0469e77f8ecf0f5/src/konnect/definitions/kong-identity/v2/openapi.yaml),
[API questions](../../../kong-identity-api-questions.md),
[metadata storage](../../../contributor/kong-identity-metadata-storage.md).
