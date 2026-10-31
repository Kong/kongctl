# Kong Identity API team questions

Questions about observed HTTP behavior and API resource contracts. Record each
team response with its date, applicable environment, and API version.

Unless stated otherwise, observations were collected on October 30, 2026
(US Central), using direct HTTP requests to `https://us.api.konghq.com`.
Specification links are pinned to the inspected platform-api PR 3877 revision.
Temporary test resources were deleted after the probes.

## Directory API questions

### Q1 Directory Token Vault enablement

**Question:** Is `vault_enabled` supported in the US directory API today?
Is enablement awaiting deployment, gated by an entitlement, or failing to
persist?

**Observed:** PUT with `vault_enabled: true` succeeded but returned false.
A subsequent GET also returned false. Other supplied directory settings and
`managed_by` entries were retained.

**Specification:** [DirectoryVaultEnabled][vault-definition] is a boolean
that enables Token Vault when true and defaults to disabled. It is referenced
by the [create body][vault-create], [PUT body][vault-replace],
[PATCH body][vault-update], and [directory response][vault-response].

### Q2 Default directory lifecycle

**Question:** Does every organization automatically receive a directory
named `default`? When is it created, can it be renamed or deleted, and does
it have any special behavior beyond that name?

**Observed:** GET `/v2/directories` returned an empty list for one test
organization. The inspected schema exposes name and ID without a default
marker.

### Q3 Directory quota and control plane access

**Question:** Is `allow_all_control_planes: true` permanently required when
an organization has a one-directory quota? Which organizations may restrict
access to an explicit control-plane allowlist, and how is that capability
exposed through the API?

**Observed:** Directory creation with false was rejected with an error
stating that it must be true when the directory quota is one. Creation with
explicit true succeeded. The [schema default is false][directory-spec].

## Resource identity and update contracts

These questions concern the resource contracts in
[platform-api PR 3877](https://github.com/Kong/platform-api/pull/3877).

### Q4 Imported API key ownership

**Question:** Can imported principal and group API keys accept `managed_by`
in the initial create request, so the key and its management metadata are
created atomically?

The inspected imported-key create body does not expose `managed_by`. Is
there another supported way to attach it during creation?

### Q5 Ownership update semantics

**Question:** For principals, identities, and credentials, which operations
replace the whole `managed_by` map, which merge keys, and how are keys removed?
Are management metadata updates supported for every resource that exposes it?

**Observed:** Directory and principal PUT requests retained the supplied
management metadata values. Principal PATCH behaved differently; see Q11.
Omission and removal semantics still need confirmation across resource types.

### Q6 Vault and trusted IdP ownership

**Question:** Do vault keys, credential providers, provider credentials, and
trusted IdPs expose a creator, managing-system identifier, or external
identifier when `managed_by` is unavailable? What is the supported way to
distinguish records created by different administrators or systems?

**Known:** The team states that provider names are unique per directory and
vault keys have unique `kid` values. Those identify records but do not
identify their creator or managing system.

### Q7 Shared provider credential matching and updates

**Question:** Can one provider have multiple credentials created by an
administrator? What uniqueness constraints apply, and which fields besides
the generated ID identify a credential within its provider?

Do updates preserve omitted secret material? Are selectors and priority
mutable, and how are omitted or empty values handled? Do these rules differ
for credentials created through user enrollment?

### Q8 Groups and imported organization charts

**Question:** Are group memberships unique by member type and ID, and are
group cycles rejected? May manually created memberships coexist with
organization-chart imports? Which records may an import overwrite or remove?

## Principal deployment questions

### Q9 Principal type changes

**Question:** Are principal type changes intentionally unsupported, or is the
deployed update implementation behind the specification? Can a principal
change between user/agent or return to null? If type is immutable, should an
attempted change produce a validation error?

**Observed:** Creation retained user, agent, and unclassified/null types.
PUT requesting agent for an existing user succeeded but returned user.
Subsequent GET and list responses also returned user. PATCH with `type: null`
likewise left the type unchanged.

**Specification:** The [type schema][principal-type] is referenced by both
update bodies without an immutable-field restriction.

**Reproduction:** For a disposable principal created as user, PUT the
observed mutable settings back with `"type": "agent"`, or send:

```http
PATCH /v2/directories/{directoryId}/principals/{principalId}
Content-Type: application/json

{"type": null}
```

Both requests succeeded but retained user in the response and subsequent GET.

### Q10 Principal active state is absent

**Question:** When will principal create/GET/list/update responses expose
`active`, and which deployed environments support setting it? If writes are
supported before the field is returned, how can their effective value be
verified?

**Observed:** Requests supplied true and false, but no response included
`active`. This does not establish whether the writes took effect.

**Specification:** The [active schema][principal-active] describes the
boolean state and its update/provisioning restrictions.

**Reproduction:** Create a disposable principal with `"active": true`;
then PUT its observed mutable settings with `"active": false`. Inspect both
responses, GET that principal, and list its directory. None included active
in the tested deployment.

### Q11 Principal PATCH ignores ownership updates

**Question:** Is principal PATCH expected to update `managed_by` today?
The specification exposes it, but a requested version change was ignored.
Are PATCH updates gated separately from PUT updates?

**Observed:** Principal PUT retained changed values in a five-entry
`managed_by` map. PATCH changing only the version entry succeeded but
returned the previous map.

**Reproduction:** After creating a disposable principal with
`managed_by.version` set to `probe-put`, send:

```http
PATCH /v2/directories/{directoryId}/principals/{principalId}
Content-Type: application/json

{"managed_by": {"version": "probe-patched"}}
```

The response and subsequent reads retained `probe-put`. PUT changing the
complete management metadata map did retain the requested values.

## Recorded answers

No team answers have been recorded yet.

[vault-definition]: https://github.com/Kong/platform-api/blob/98137d7be3772b67693cc1ceb0469e77f8ecf0f5/src/konnect/definitions/kong-identity/v2/openapi.yaml#L3579-L3583
[vault-create]: https://github.com/Kong/platform-api/blob/98137d7be3772b67693cc1ceb0469e77f8ecf0f5/src/konnect/definitions/kong-identity/v2/openapi.yaml#L3551-L3552
[vault-replace]: https://github.com/Kong/platform-api/blob/98137d7be3772b67693cc1ceb0469e77f8ecf0f5/src/konnect/definitions/kong-identity/v2/openapi.yaml#L3575-L3576
[vault-update]: https://github.com/Kong/platform-api/blob/98137d7be3772b67693cc1ceb0469e77f8ecf0f5/src/konnect/definitions/kong-identity/v2/openapi.yaml#L3604-L3605
[vault-response]: https://github.com/Kong/platform-api/blob/98137d7be3772b67693cc1ceb0469e77f8ecf0f5/src/konnect/definitions/kong-identity/v2/openapi.yaml#L4217-L4218
[directory-spec]: https://github.com/Kong/platform-api/blob/98137d7be3772b67693cc1ceb0469e77f8ecf0f5/src/konnect/definitions/kong-identity/v2/openapi.yaml#L3511-L3519
[principal-type]: https://github.com/Kong/platform-api/blob/98137d7be3772b67693cc1ceb0469e77f8ecf0f5/src/konnect/definitions/kong-identity/v2/openapi.yaml#L4353-L4363
[principal-active]: https://github.com/Kong/platform-api/blob/98137d7be3772b67693cc1ceb0469e77f8ecf0f5/src/konnect/definitions/kong-identity/v2/openapi.yaml#L4431-L4437
