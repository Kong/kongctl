# Native organization-chart import and declarative adoption

Research snapshot: November 6, 2026. Scope: native Kong Identity import,
management metadata, and a subsequent declarative dump/apply. Initial research
was read-only. A subsequent authorized direct API probe tested reversible
writes against one synthetic imported principal in `.com`. No import was
replayed; original principal state was restored and verified.

Import followed by a zero-change external-reference snapshot works today.
Direct API tests also establish a path for limited adoption: hydrated PUT
persists ownership metadata while preserving imported provisioning. Imported
names and descriptions remain controlled by the importer. KongCTL currently
rejects imported principals and must implement this narrower boundary before
`--adopt` can provide managed declarative output.

## Current `.com` read-only verification

The configured organization has three imported principals. Existing dump
output, filtered to those principals beneath an external lookup of `default`,
contains three external principal declarations and seven principal identity
records. Planning that file with `--mode apply` produced **zero changes**.
No import, ownership write, deactivation or resource mutation was performed.
The generated records use `_external`; they do not acquire a managed lifecycle.
Local evidence is under `.cache/identity-import-adoption/private/`, including
`verification-summary.json` and `imported-principals-external.yaml`. Files are
private and contain no new ownership markers.

## Authorized `.com` write verification

On November 6, 2026, direct HTTP requests tested one existing synthetic active
imported principal. Every request was followed by a fresh GET. No request
changed activity or provisioning; the original writable fields and ownership
map were restored and verified. No roster import was replayed.

The route for every request below was
`/v2/directories/{directoryId}/principals/{principalId}`.

| Request | Result | Fresh GET |
| --- | --- | --- |
| PATCH `managed_by` with five ownership keys | 200 | Ownership unchanged |
| PATCH `description` | 409 | Unchanged |
| PATCH `display_name` | 409 | Unchanged |
| PATCH user `metadata` | 200 | Persisted |
| PATCH `labels` | 200 | Persisted |
| Hydrated PUT adding ownership keys | 200 | Ownership persisted |
| Hydrated PUT changing `display_name` | 409 | Unchanged |
| Hydrated PUT changing `description` | 409 | Unchanged |
| Hydrated PUT changing user metadata and labels | 200 | All maps persisted |

PATCH name/description failures said:
> display_name and description are managed by the import and cannot be
> modified directly
PUT name/description failures said:
`only metadata and labels can be modified on an imported principal`.

PATCH could not remove the temporary ownership keys either. Hydrated PUT with
an empty original ownership map restored them successfully. Full restoration
was verified across display name, description, type, metadata, labels,
managed_by, active, and provisioned_by. Timestamp changes were expected.

This permits a limited principal-adoption implementation using hydrated PUT.
It does not establish arbitrary principal control: declarative name and
description changes must fail clearly, and deletion/sync pruning must remain
protected until their permissions are established. Type changes, deletion,
reserved identities, team mutations and subsequent import behavior were not
probed in this session. Existing historical observations cover some of these
but must not be represented as fresh verification.

A manifest adopting the imported principal itself has not been implemented
or tested. The earlier principal-only zero-change plan used external
references. Generated child-key management was subsequently implemented and
verified separately below.
Evidence and the probe are under `.cache/identity-import-adoption/`; response
bodies are private. The probe is tied to this synthetic fixture and is not a
general-purpose command to copy into another organization.

## Recommended feature design

A possible interface, after verifying the API permissions, is:

```sh
kongctl import id org-chart \
  --file roster.csv \
  --execute \
  --adopt \
  --namespace workforce \
  --report-output-dir ./import-reports \
  --output-file ./workforce.yaml
```

`--adopt`, its namespace binding, and this import output-file option are a
proposal; they have not been implemented.

1. Validate the CSV and adoption intent before execution. Require an explicit
   namespace. Dry-run performs no ownership writes and does not promise an
   immediately usable post-import manifest for not-yet-created UUIDs.
2. Execute the native import once. Its `users.adopted` count means matching an
   existing user by the API's rules; it does not mean declarative ownership.
3. Discover the imported resources using complete paginated observations.
   Match only rows from this CSV using their external-ID identities, with
   email as a cross-check. Reject missing or duplicate matches; display names
   are not unique. The import response supplies counts, not a row-to-UUID map.
4. Derive valid stable refs from CSV external IDs, retaining existing refs
   when already correctly owned. Validate refs and detect sanitization
   collisions. Ref generation must not depend on mutable display names.
5. Use reusable adoption functions shared with the normal adopt command,
   rather than invoking Cobra or starting CLI subprocesses. Preserve existing
   protections and enforce ownership/ref conflicts. Principal ownership uses
   five management keys; the API map limit is also five. Foreign metadata
   cannot simply be merged with all five or silently discarded to make room.
6. Apply and verify metadata through a method proven to work for imported
   principals. The current adopter uses hydrated PUT and refuses imported
   state. A verified metadata PATCH would require a shared adoption helper,
   not just removing that guard. Ordinary declarative updates would also need
   a supported method and appropriately narrow writable-field rules.
7. Export fresh observed state for the matched IDs only, keeping `default`
   external. Reuse the existing dump projections and reload the result through
   the real loader/planner. Publish the final private file only after an
   apply-mode plan has zero changes. Unrelated principals and Vault resources
   in the same directory must not enter the exported configuration.
8. Capture import, per-resource adoption and export outcomes separately in the
   private report. These API calls are not one transaction. A partial adoption
   failure must not automatically replay the full-roster import.

Start with principals if the API supports their complete intended workflow.
Importer-created `kong:*` identities, teams and memberships need their own
ownership/write contracts; retain external references until those contracts
are verified. Inactive users omitted from this CSV also need an explicit
policy before they can enter the adopted namespace's sync scope.

Do not use `active: false` as an adoption workaround. It changes authentication
and provisioning, as the API documents. Likewise, removing `_external` from a
dump does not adopt the record: matching requires stored namespace/ref
metadata, and a configuration may otherwise propose duplicate creation or
fail its UUID assertion. See [adoption][client-adopt],
[planner matching][client-plan],
[dump projection][client-dump], [metadata builder][client-metadata], and
[ref validation][client-ref].

For a one-time CSV bootstrap into ordinary managed principals, the existing
[declarative principal CSV path][csv-guide] is available. A converter could map
canonical roster columns into that format without invoking the native
importer. This would not create the importer-managed organization-chart teams
or reporting hierarchy; it is a separate product workflow.

## API and SDK contract

- `POST /v2/directories/{directoryId}/org-chart/import` takes multipart `file`
  and `dry_run` (default `true`; explicitly `false` executes). The specification
  describes user principals, an email custom identity, `hris_import` tagging,
  external IDs, and optional `metadata.cost_center`. It says validation precedes
  writes and the response contains counts and per-row errors, never created
  rows. There is no request ownership/adoption option.
  [Import operation][api-import]
  and [request/report schemas][api-report].
- The report contains users (`created`, `updated`, `unchanged`, `inactive`,
  optional `adopted`), teams (`created`, `deleted`), memberships (`added`,
  `removed`), warnings (`row`, `type`, `detail`) and errors (`row`, optional
  `field`, `reason`). Email adoption is counted separately from creation.
  Teams can be removed when rosters empty; memberships change with managers
  and departures. It provides no resource UUID mapping for a stamping phase.
  [Report schemas][api-report].
- The org-root response documents an importer-controlled `kong:org_root`
  principal identity and a managed team whose `team_manager` identity names
  the root. A user manages at most one team per directory. Group membership
  entries have `id`, `member`, and `created_at`, with no independent
  `managed_by`. Therefore stamping principals alone does not cover the full
  imported chart. [Org root][api-root], [membership entries][api-membership].
- `provisioned_by` is read-only; `imported` is an enum value. Setting principal
  `active: false` explicitly changes provisioning to `konnect_user`; that is
  deactivation, not an ownership-only update.
  [Provisioning schema][api-provisioning],
  [active schema][api-active].
- Principal PATCH accepts `managed_by` as `ManagedByUpdate`; PUT accepts
  `ManagedBy`. PATCH preserves unspecified fields; PUT resets omitted fields
  generally. Neither endpoint description or body schema specifically grants
  or forbids metadata-only PATCH while `provisioned_by: imported`. An exposed
  request property is not proof of that state-dependent backend permission.
  [Endpoints][api-principal], [request bodies][api-principal-bodies].
- `ManagedByUpdate` explicitly says setting a key to `null` removes it and
  permits nullable string values. It has a five-property limit. It does not
  explicitly promise preservation of omitted map entries; the endpoint's
  unspecified-field rule does not settle map replacement versus merging.
  [Management metadata schemas][api-managed-by].
- Principal identities expose management metadata through their common base,
  but identity PATCH's body only lists identity-specific fields, including
  custom `key`/`value`; it has no `managed_by` or `labels` property. Generic
  identity PATCH therefore does not document an ownership-stamping path,
  including for imported reserved identities.
  [Identity PATCH][api-identity-patch],
  [identity schemas][api-identity-bodies].
- The current module resolves to SDK
  `v0.83.1-0.20261105173244-6f7802e01d27`, replacing `v0.83.0`.
  Its generated principal PATCH body has `map[string]*string` management
  metadata, its import request supports file/dry-run, and its typed report now
  includes optional `adopted`. Older notes saying the SDK lacks that count are
  historical. SDK shape alone cannot establish deployed permissions.
  [Module selection](../../go.mod#L130), [SDK PATCH body][sdk-patch],
  [SDK import request][sdk-import], [SDK report][sdk-report].

## Prior observations and implemented boundaries

Repository compatibility notes record native import retaining an adopted
principal's UUID, management map and unrelated metadata while changing its
name and provisioning to `imported`; ordinary principal PUT/DELETE then
returned 409. These are prior observations, not an imported-specific PATCH
prohibition in the specification. Separately, Q11 records successful PATCH
responses that failed to change principal management metadata, including
partial/full-map and null-removal failures in `.tech`.
[Imported-state observations](kong-identity-compatibility.md#L1432),
[principal PATCH observations](kong-identity-compatibility.md#L131).

The boundary scenario asserts retained metadata and imported provisioning,
rejection by current planner/adopt/saved-write guards, one imported team,
external-reference dump replanning with zero changes, and explicit
`active: false` release during fixture cleanup. These assertions define the
implemented test boundary; CLI rejection is not independent evidence that
the backend rejects metadata PATCH. The lifecycle scenario additionally
asserts email adoption, stable UUIDs and repeated-import unchanged counts.
[Boundary assertions][test-boundaries], [lifecycle assertions][test-lifecycle].

Consequently, zero drift already has an external-reference meaning: preserve
the imported resource without declaring its importer-controlled fields as
desired managed state. A dump cannot turn read-only provenance into a writable
declaration. Full adoption would additionally require supported ownership and
reconciliation rules for teams, identities and memberships, plus resource
discovery because import reports omit IDs. Metadata retention alone supplies
none of those lifecycle guarantees.

## API keys under an imported external principal

Direct `.com` verification on November 6, 2026 used the single-person
imported fixture. Its principal remained active, `provisioned_by: imported`,
and unowned (`managed_by: {}`) throughout. All issued keys were deleted.

The collection route was
`/v2/directories/{directoryId}/principals/{principalId}/api-keys`.
Individual GET/PATCH/DELETE requests appended `/{apiKeyId}`.

| Test | HTTP result | Fresh observation |
| --- | --- | --- |
| POST generated `type: v1` with five ownership keys | 201 | Map persisted |
| GET and list | 200 | Key discoverable; GET omitted secret |
| PATCH name | 200 | Persisted |
| PATCH expiration | 200 | Persisted |
| PATCH expiration to null | 200 | Expiration cleared |
| PATCH `managed_by` map | 200 | Change ignored |
| PATCH ownership key to null | 200 | Key remained |
| POST second key with same ownership tuple | 201 | Two matches |
| DELETE both test keys | 204 | Subsequent GETs returned 404 |

Principal and identity responses were identical before and after the test;
API-key inventory returned to its original state. Generated secrets were
captured only in private local files and never printed.

The API supports an independently owned generated credential beneath an
imported principal. KongCTL's managed-parent requirement is a client design
restriction for this workflow. A safe implementation could stamp namespace,
ref and protection on creation, resolve credentials by local tuple filtering,
and manage name/expiration/deletion without adopting the principal. Duplicate
matches must fail before mutation. Five ownership keys exhaust the API's
five-property limit; the existing extra creation-type marker cannot simply
be added as a sixth property.

Ownership changes and adoption of preexisting unowned keys are not verified
as supported: PATCH accepts but ignores the map. Parent-independent
protection changes would need a supported metadata update method or an
explicit restriction. This test did not exercise runtime authentication,
subsequent roster imports, imported key material, or declarative execution.
Private evidence is in `.cache/identity-import-single/key-probe/`.

## Repeated import and external-principal key implementation

The CLI integration check repeated the exact one-person CSV after creating
an independently owned key. The key UUID and complete public key record
remained unchanged; the principal response also remained unchanged. The
`kong:org_chart_root` identity was recreated with a different UUID/timestamps.
The email and external-ID identities retained their original UUIDs. This
means a full dump's external reference to the old root-marker identity may
be stale after another import, even though a manifest declaring only the
external principal and its managed keys still identifies the same resources.

Generated principal keys now support an external UUID parent. In that mode,
the five stored keys are tool, version, ref, namespace and creation type.
The creation marker is retained instead of adding protection as a sixth key.
`kongctl.namespace` is supported, but key-level protection and adoption are
not. Existing managed-parent keys retain their four-marker convention and
inherited protection. Updates never attempt to mutate ownership metadata.

A live CLI integration check passed generated creation/private capture,
zero-change apply planning, name/expiration updates, diff, dump/no-drift
reload, repeated import, duplicate conflicts, omitted/empty namespace-scoped
sync, and explicit deletion. All test keys were removed. Key-only operations
left the principal and its identities unchanged; the repeated import itself
recreated the org-root marker as described above.

## Questions for engineering

1. Why does PATCH return 200 without persisting `managed_by`, while PUT
   persists the same map on an active imported principal? PATCH null-valued
   keys also did not remove ownership entries.
2. Is updating `managed_by` through PUT on an active imported principal
   supported intentionally? The PUT error for name/description changes says
   only metadata and labels can be modified, but ownership updates succeed.
3. Which changes to imported identities, teams and memberships are supported
   through their administrative APIs while the org-chart importer owns them?

4. Why does API-key PATCH return 200 without persisting `managed_by` changes
   or null-key removal, while the same map persists on key creation? Is there
   a supported way to update credential management metadata after creation?
   Route: `/v2/directories/{directoryId}/principals/{principalId}/api-keys/{apiKeyId}`.

5. Is the `kong:org_chart_root` identity expected to receive a new UUID on
   every import of an identical roster? Principal, email identity and
   external-ID identity UUIDs stayed unchanged in this test.

Requests in questions 1 and 2 use
`/v2/directories/{directoryId}/principals/{principalId}`. Their results were
verified in `.com` with fresh GETs on November 6, 2026. Question 3 was not
probed by the principal-only write test.

[api-import]: https://github.com/Kong/platform-api/blob/8106b8178c4d25130510ea0313d1910be238a8f9/src/konnect/definitions/kong-identity/v2/openapi.yaml#L389
[api-report]: https://github.com/Kong/platform-api/blob/8106b8178c4d25130510ea0313d1910be238a8f9/src/konnect/definitions/kong-identity/v2/openapi.yaml#L4367
[api-root]: https://github.com/Kong/platform-api/blob/8106b8178c4d25130510ea0313d1910be238a8f9/src/konnect/definitions/kong-identity/v2/openapi.yaml#L4849
[api-membership]: https://github.com/Kong/platform-api/blob/8106b8178c4d25130510ea0313d1910be238a8f9/src/konnect/definitions/kong-identity/v2/openapi.yaml#L4162
[api-provisioning]: https://github.com/Kong/platform-api/blob/8106b8178c4d25130510ea0313d1910be238a8f9/src/common/definitions/kong_identity.yaml#L29
[api-active]: https://github.com/Kong/platform-api/blob/8106b8178c4d25130510ea0313d1910be238a8f9/src/konnect/definitions/kong-identity/v2/openapi.yaml#L4586
[api-principal]: https://github.com/Kong/platform-api/blob/8106b8178c4d25130510ea0313d1910be238a8f9/src/konnect/definitions/kong-identity/v2/openapi.yaml#L424
[api-principal-bodies]: https://github.com/Kong/platform-api/blob/8106b8178c4d25130510ea0313d1910be238a8f9/src/konnect/definitions/kong-identity/v2/openapi.yaml#L3768
[api-managed-by]: https://github.com/Kong/platform-api/blob/8106b8178c4d25130510ea0313d1910be238a8f9/src/common/definitions/konnect_properties.yaml#L3
[api-identity-patch]: https://github.com/Kong/platform-api/blob/8106b8178c4d25130510ea0313d1910be238a8f9/src/konnect/definitions/kong-identity/v2/openapi.yaml#L660
[api-identity-bodies]: https://github.com/Kong/platform-api/blob/8106b8178c4d25130510ea0313d1910be238a8f9/src/konnect/definitions/kong-identity/v2/openapi.yaml#L4899
[sdk-patch]: https://github.com/Kong/sdk-konnect-go/blob/6f7802e01d2705181288c90736af6068957540f8/models/components/updateprincipalbody.go#L27
[sdk-import]: https://github.com/Kong/sdk-konnect-go/blob/6f7802e01d2705181288c90736af6068957540f8/models/components/importprincipalsrequest.go#L32
[sdk-report]: https://github.com/Kong/sdk-konnect-go/blob/6f7802e01d2705181288c90736af6068957540f8/models/components/kongimportreport.go#L6
[test-boundaries]: ../../test/e2e/scenarios/identity/org-chart-boundaries/boundaries.py#L132
[test-lifecycle]: ../../test/e2e/scenarios/identity/org-chart-import/lifecycle.py#L192

[client-adopt]: ../../internal/cmd/root/products/konnect/adopt/identity_principal.go#L86
[client-plan]: ../../internal/declarative/planner/identity_principal_planner.go#L109
[client-dump]: ../../internal/cmd/root/verbs/dump/declarative_identity.go#L106
[client-metadata]: ../../internal/declarative/resources/identity_principal.go#L372
[client-ref]: ../../internal/declarative/resources/validation.go#L21
[csv-guide]: ../examples/declarative/identity/README.md
