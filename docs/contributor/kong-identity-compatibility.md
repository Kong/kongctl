# Kong Identity compatibility tracking

Track temporary implementation restrictions here and in #1415. API questions
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

## Recording resolution

For each resolved question, record the team answer, date, environment/API
version, and direct-call evidence in the question log. Update #1415 and this
tracker, remove or revise the corresponding guard, and add the verification
above in the same change. Do not silently broaden support based solely on a
specification or SDK regeneration.
