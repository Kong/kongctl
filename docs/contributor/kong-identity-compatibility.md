# Kong Identity compatibility tracking

Track temporary implementation restrictions here and in the resource issues.
API questions
for engineering stay in the separate
[question log](../kong-identity-api-questions.md), without client-specific
implementation requirements.

## Principal type changes

Related API question: Q9. Creation supports user, agent, and unclassified/null
types, but tested PUT/PATCH requests did not change an existing type.

- Initial contract: type is create-only. Preserve it when omitted on update;
  reject requested changes, including a requested reset to null.
- Removal trigger: engineering confirms the intended contract and direct HTTP
  tests verify supported changes in the target environment.
- Required verification: user to agent, agent to user, and classified to null;
  saved plans; omitted-field preservation; no-op reapplication; dump/reload.
- Keep the guard explicitly tied to Q9 rather than treating immutability as a
  permanent API guarantee.

## Principal active state

Related API question: Q10. Tested responses omitted active. The generated
SDK's plain bool cannot distinguish absence from false.

- Initial contract: defer declarative active configuration. Inspection omits
  absent values in JSON/YAML and shows unknown in text. Never dump a fabricated
  false value or write one while preserving unrelated settings.
- Removal trigger: engineering confirms deployment and provisioning rules;
  direct create/GET/list/update tests return observable active state and verify
  true/false transitions.
- Required verification: omitted versus false, activation/deactivation,
  provisioning restrictions, saved plans, and dump/reload.
- Retain response-presence handling for compatibility with older environments
  even after enabling active configuration.

## Principal management metadata PATCH

Related API question: Q11. PUT retained management metadata updates; PATCH
returned success while leaving a requested version change unchanged.

- Initial contract: use a fresh read plus PUT for management metadata changes,
  adoption, and protection updates; preserve omitted settings.
- Removal trigger: engineering confirms PATCH semantics and direct tests verify
  value changes, omitted keys, null removal, and preservation of other keys.
- Required verification: canonical five-entry ownership metadata, protection
  changes, adoption, omitted fields, and saved plans.
- Reassess whether PATCH simplifies the implementation once verified. Working
  PUT support remains valid even if switching is unnecessary.

## Current implementation and regression coverage

The principal resource validates deferred `active` input in
`internal/declarative/resources/identity_principal.go`. Type changes are
rejected in `planner/identity_principal_planner.go` and rechecked against a
fresh observation in `executor/identity_principal_adapter.go`. These guards
reference Q9/Q10 explicitly.

`helpers.IdentityPrincipalReplacement` builds PUT requests without `active`.
The principal executor hydrates omitted fields from a fresh detail response,
then overlays only saved plan fields. Adoption uses the same replacement
builder. Reads preserve response presence independently of SDK defaults.

Coverage lives in the principal loader, planner, executor, adoption, and read
helper tests. `test/e2e/scenarios/identity/directories` exercises the live
principal lifecycle, including user/agent/null creation, repeated no-op plans,
PUT updates, omitted fields, metadata clearing, dump/reload, adoption,
protection, deletion, and scoped sync. Q9/Q10 rejection assertions should be
replaced with supported behavior when those restrictions are removed.

## Auth-server preview SDK boundaries

The public preview SDK exposes `dcr.default_access_token_duration`. Internal
DCR options remain outside the public manifest schema. PATCH writes only
declared public fields, preserving omitted backend options. Verify any future
DCR expansion against its create, partial-update, secret, and read contracts
before exposing it.

The preview includes a targeted SDK serialization patch that preserves an
explicit empty `trusted_origins` array. A nil slice still omits the field.
Keep the SDK wire regression and live clear/replan scenario when regenerating
or upgrading the SDK; remove the patch only when upstream serialization
preserves this distinction. This is a generated-client constraint, not an API
question for the service team.

Auth-server deletion always requests `force=false`. Existing children may
block deletion; no root-level cleanup or declarative delete silently cascades.
Revisit deletion ordering only when child resource support observes and
validates the complete dependency graph.

## Claim names and pagination

Related API questions: Q12 and Q13. Direct HTTP probes accepted duplicate
claim names. Claim list totals exceeded the actual number of claims when
claims referenced multiple scopes.

- Initial read contract: name lookup exhausts the unfiltered collection and
  rejects duplicate matches. UUID lookup remains available. Server template
  strings are returned verbatim.
- Claim pagination continues through an empty terminal page, rather than
  trusting the inflated total as a resource count. It validates page metadata,
  rejects repeated/missing IDs, and exposes no partial result after a failure.
- Removal trigger: engineering documents name collision behavior and corrects
  totals; direct multi-scope tests verify accurate counts and terminal pages.
- Keep duplicate detection even if uniqueness becomes enforced, for records
  created before the constraint. Keep terminal-page traversal compatible with
  deployments that still report inflated totals.
- Required verification: two claims linked to three scopes each, page size one
  and one hundred, empty collections, duplicate names, exact UUID reads, and
  failed or repeated pages. Scope deletion also needs fresh claim/client
  observations because the API silently detaches associations.

The generated claim PATCH request also currently omits explicit empty
`include_in_scopes`. Declarative claim configuration must preserve an empty
list on the wire before exposing association clearing. This serializer issue
is separate from the service API questions.

## Recording resolution

For each resolved question, record the team answer, date, environment/API
version, and direct-call evidence in the question log. Update #1415 and this
tracker, remove or revise the corresponding guard, and add the verification
above in the same change. Do not silently broaden support based solely on a
specification or SDK regeneration.
