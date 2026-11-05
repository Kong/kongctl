# Kong Identity SDK 0.83 impact

Assessment date: November 5, 2026.

## Branch and dependency

`ki-triage` is rebased onto updated `origin/main` at `d054f056` after
SDK 0.83 merged in #2724. PR #2676 targets `main`. The Identity tutorial was
preserved across the rebase. A local backup branch is
`backup/ki-triage-before-sdk-83-20261105`. A second backup before rebasing
onto updated main is `backup/ki-triage-before-main-20261105`.

The consumer now pins public SDK v0.83.0 plus KongCTL's Identity compatibility
patches at `6f7802e01d27`, version
`v0.83.1-0.20261105173244-6f7802e01d27`. The SDK branch is
[`kongctl/identity-sdk83-compat`](https://github.com/Kong/sdk-konnect-go/tree/kongctl/identity-sdk83-compat),
based on the ordinary patched v0.83.0 commit `18b075cc1edd`. This is a branch
dependency, not a published SDK release; no SDK PR was opened.

The previous Identity preview was `2f51ede0670e`. The initial v0.83.0 baseline
had a different compatibility patch set. Missing preview fixes were not
evidence of changed backend contracts. The new branch restores only Identity
request serialization and includes no Agent Registry additions.

## Clear adaptations

- Auth Server models use `Client`, `Scope`, `GrantType`, `Algorithm`, and
  `TokenEndpointAuthMethod`. The preview's prefixed Go names disappear.
  Response fields become `Client` and `Scope`. Wire fields are unchanged.
- Deletion options become `operations.Force` and
  `operations.DeleteDirectoryQueryParamForce`. Deletion must continue to send
  explicit false; no cascading deletion is introduced.
- The principal-list filter becomes `ListPrincipalsQueryParamFilter`.
  Existing exact display-name filtering keeps its query shape.
- Identity subtype and API-key creation discriminators become generated
  constant fields. Use constructors/getters, and validate the raw saved-plan
  type explicitly; a constant getter cannot validate untrusted input.
- Group names now permit internal hyphens, with an ASCII letter or digit at
  each end. Declarative validation and explain guidance follow that pattern.
- Org-chart imports now default to dry-run in the specification. The existing
  command explicitly supplies true or false, so execution intent is preserved.
  Existing upload tests must continue to prove false reaches the API.

Principals, identity associations, ordinary groups, memberships, basic-auth
usernames, Trusted IdPs, and Vault administration retain their resource
hierarchy and ownership policy. No new metadata field or server-side
`managed_by` filter was found in this comparison. Provider sorting documents
`name` in addition to timestamps; existing inventory matching needs no change.

## Compatibility fixes carried forward

The v0.83.0 baseline lacked several preview compatibility fixes. The new
compatibility branch restores these behaviors:

| Area | SDK 0.83 behavior | Required preserved behavior |
| --- | --- | --- |
| Auth Server PATCH arrays | `omitempty` drops explicit empty lists | Send empty lists to clear values while omitting undeclared lists |
| Client auth method PATCH | Omitted field gains `client_secret_post` | Preserve the current method when omitted |
| Identity cursor queries | `style: form` emits flat object members | Send `page[size]` and `page[after]` |
| Credential expiration | Time pointers conflate null and omission | Preserve omitted, timestamp, and explicit null |

The public spec now has `ExpiresAtWritable`; the former read-only schema
collision is corrected. The generated pointer representation still cannot
express all three expiration states. Do not remove null-clearing behavior or
silently omit expiration to make compilation succeed.

The updated local patch manifest is adapted to SDK 0.83 model names and its
split key-creation files. Its post-generation replacements have been checked
in memory for complete application and idempotence against `18b075cc1edd`.
The SDK source checkout is `/home/rspurgeon/dev/kong/ki-sdk83-compat`,
branch `kongctl/identity-sdk83-compat`. Commit `6f7802e01d27` is pushed and
includes the reproducible patch snapshot from KongCTL `a1daabe1`, provenance
hashes and HTTP serialization regressions. SDK root, model, internal and
package tests pass. `go.mod` uses the remotely resolvable Go pseudo-version;
there is no local-path replacement.

Agent Registry is explicitly excluded from this Identity work. Its readers,
retirement guards, dedicated tests and active API questions have been removed.
Identity operations do not inspect Registry links and need no Registry SDK
additions. This scope decision does not establish backend cleanup semantics.

## Questions and limits

- SDK compatibility patches are carried on the development SDK branch until
  upstream generation preserves those request semantics.
- New spec behavior, including hyphenated group names, has not been exercised
  against the deployed API in this pass. Spec availability does not establish
  deployment availability.

## Validation against the published dependency

- The original helper compilation failure was reproduced with the baseline
  SDK, then eliminated by updating the consumer replacement to the pushed
  compatibility commit. API-key/password code keeps its three-state
  expiration semantics.
- `go mod tidy` and `go mod vendor` refresh the dependency checksums and
  vendored generated models. The built binary reports replacement version
  `v0.83.1-0.20261105173244-6f7802e01d27`.
- `GOFLAGS=-buildvcs=false make build` passes with CGO disabled by the recipe.
  This agent environment makes Go probe `/home/rspurgeon` for VCS status and
  requires that existing stamping workaround. It is independent of the SDK
  compilation error. No SDK-local path or module-file override is used.
- Modernization, stable formatting, and full `GOFLAGS=-buildvcs=false make
  test-all` pass against the published pin: zero-issue lint, unit/race tests,
  integration, installer, SDK-patch, E2E harness and metrics checks. The log is
  `.cache/sdk-83-identity/published-sdk-gates.log`.
- Existing principal/directory deletion tests pass without Registry readers.
  The Registry-only scenario was not replay-enabled; enabled replay membership
  and cassettes are unchanged.
- No live Identity lifecycle scenarios or remote Identity resource mutations
  were run in the SDK migration pass. The SDK branch is pushed; the consumer
  adaptation and Registry exclusion are included in PR #2676.
- The tutorial site adds four Identity lessons before Extensions. Site checks,
  11 unit tests, and the static build pass. Local browser verification is
  blocked by Chromium failing before initial page loading. The walkthrough
  mutation commands are examples, not newly executed live acceptance tests.

Earlier local-SDK validation is in
`.cache/sdk-83-identity/without-registry-gates.log`.

Raw SDK inventories and comparisons are in `.cache/sdk-83-identity/`.
