# Kong Identity API team questions

Questions to raise during business hours as KongCTL implementation proceeds.
Keep answered items with the team's response and date. Client-side
`managed_by` filtering is accepted and is not a blocker for this work.

## Open questions

### Q1 Directory Token Vault enablement

**Question:** Is `vault_enabled` supported in the US `.com` directory API
today? A full PUT with `vault_enabled: true` succeeds but returns false, and
a fresh GET also returns false. Is this awaiting deployment, gated by an
entitlement, or a bug?

**Observed:** 2026-10-30, US `.com`, an isolated test directory. Reproduced
through KongCTL and direct HTTP. Other declared settings and ownership
metadata were retained by the same API. The cause is unconfirmed.

**Impact:** Directory configuration can proceed with false. KongCTL reports
an error when the API fails to retain a requested vault setting. Enabling
Token Vault cannot be claimed as verified until this is resolved.

**Specification locations in the inspected PR revision:**

- [Boolean definition and disabled default][vault-definition]:
  `components.schemas.DirectoryVaultEnabled`, lines 3579–3583.
- [Create body][vault-create]: `CreateDirectoryBody.vault_enabled`, line 3551.
- [PUT body][vault-replace]: `ReplaceDirectoryBody.vault_enabled`, line 3575.
- [PATCH body][vault-update]: `UpdateDirectoryBody.vault_enabled`, line 3604.
- [Directory response][vault-response]: `KongDirectory.vault_enabled`,
  line 4217.

The source file is
`src/konnect/definitions/kong-identity/v2/openapi.yaml` in platform-api.
These links are pinned to the inspected PR #3877 revision so line numbers
remain meaningful.

### Q2 Default directory lifecycle

**Question:** Does every organization receive a directory named `default`?
When is it created, can it be renamed or deleted, and does it have any
special behavior beyond that name?

**Context:** A single default directory is expected to be the common user
pattern. The isolated test organization initially returned an empty directory
list. The inspected schema exposes name and ID without a default marker.

**Impact:** KongCTL currently resolves an explicitly declared external name
or ID. It does not create or select a default directory silently. The answer
will guide examples and future directory references from child resources.

### Q3 Directory quota and control plane access

**Question:** Is the requirement that `allow_all_control_planes` must be true
for an organization with a one-directory quota permanent? Which organizations
can restrict directory access to an explicit control-plane allowlist?

**Observed:** The test organization rejected directory creation with false
and stated that it must be true when the directory quota is one. Creation
with explicit true succeeded. The [schema default is false][directory-spec].

**Impact:** KongCTL keeps the API default and does not silently broaden
access. The main example and live scenario state true explicitly; the
restricted-access example requires an organization that permits it.

## Questions before implementing child resources

These carry forward the unresolved contracts from the
[triage report](kong-identity-triage-2026-10-30.md#10-prioritized-engineering-requests)
and [platform-api PR 3877](https://github.com/Kong/platform-api/pull/3877).
They do not block the directory configuration slice.

### Q4 Imported API key ownership

**Question:** Can imported principal and group API keys accept `managed_by`
in the initial create request, so the key and its ownership metadata are
created together?

**Impact:** A separate metadata update after creation leaves an orphan if
that update fails and complicates safe recovery.

### Q5 Ownership update semantics

**Question:** For principals, identities, and credentials, which operations
replace the whole `managed_by` map, which merge keys, and how are keys removed?
Can all resources that expose it also update it?

**Known:** Directory PUT retains the complete requested map. Other resource
families still need their individual update contracts verified.

### Q6 Vault and trusted IdP ownership

**Question:** What ownership marker should automation use for vault keys,
credential providers, provider credentials, and trusted IdPs when these
resources do not expose `managed_by`? How should it distinguish records it
may delete from records belonging to another administrator or user?

**Known:** The team states that provider names are unique per directory and
vault keys have unique `kid` values. Those support matching; they do not
establish ownership or permission to prune other records.

### Q7 Shared provider credential matching and updates

**Question:** Can one provider have multiple credentials created by an
administrator? If so, what stable key identifies each one, and do updates
preserve omitted secret material and selectors/priority settings?

**Impact:** This determines whether files can safely manage shared
credentials while leaving user enrollment-derived credentials alone.

### Q8 Groups and imported organization charts

**Question:** Are group memberships unique by member type and ID, are group
cycles rejected, and may manually created memberships coexist with org-chart
imports? Which records may an import overwrite or remove?

**Impact:** KongCTL must avoid treating imported or manually managed state as
its own during sync. Org-chart import remains an explicit operation until
those boundaries are established.

## Answered questions

No team answers have been recorded yet.

[vault-definition]: https://github.com/Kong/platform-api/blob/98137d7be3772b67693cc1ceb0469e77f8ecf0f5/src/konnect/definitions/kong-identity/v2/openapi.yaml#L3579-L3583
[vault-create]: https://github.com/Kong/platform-api/blob/98137d7be3772b67693cc1ceb0469e77f8ecf0f5/src/konnect/definitions/kong-identity/v2/openapi.yaml#L3551-L3552
[vault-replace]: https://github.com/Kong/platform-api/blob/98137d7be3772b67693cc1ceb0469e77f8ecf0f5/src/konnect/definitions/kong-identity/v2/openapi.yaml#L3575-L3576
[vault-update]: https://github.com/Kong/platform-api/blob/98137d7be3772b67693cc1ceb0469e77f8ecf0f5/src/konnect/definitions/kong-identity/v2/openapi.yaml#L3604-L3605
[vault-response]: https://github.com/Kong/platform-api/blob/98137d7be3772b67693cc1ceb0469e77f8ecf0f5/src/konnect/definitions/kong-identity/v2/openapi.yaml#L4217-L4218
[directory-spec]: https://github.com/Kong/platform-api/blob/98137d7be3772b67693cc1ceb0469e77f8ecf0f5/src/konnect/definitions/kong-identity/v2/openapi.yaml#L3511-L3519
