# Expand models and authorize callers

Start from the existing deployment. Preserve its gateway identity, namespace,
certificate and original model alias. Read the installed contracts:

```sh
kongctl explain ai_gateways.auth_strategies
kongctl explain ai_gateways.consumers.credentials
kongctl explain ai_gateways.models.access
```

For a simple platform demo, use three aliases and two API-key consumers:
`full-access` may use all three, while `limited-access` may use only the
original model. Choose two additional upstream model IDs available to the
account. Keep one `/v1/chat/completions` endpoint and select the model via
the request body. API keys demonstrate caller access without an external
identity provider; use OIDC when the user asks for federated identity.

## Add caller identity, then attach it to each model

Merge this fragment into the existing `ai_gateways` entry. It is not a
standalone manifest. The caller keys are distinct from `OPENAI_API_KEY`:

```yaml
auth_strategies:
  - ref: client-key-auth
    name: client-key-auth
    display_name: Client Key Authentication
    type: key-auth
    config:
      key_names: [apikey]
      hide_credentials: true
consumers:
  - ref: full-access
    name: full-access
    display_name: Full Access
    custom_id: full-access
    type: api-key
    credentials:
      - ref: full-access-key
        name: full-access-key
        display_name: Full Access Key
        type: api-key
        ttl: 0
        api_key: !secret {source: !env FULL_ACCESS_API_KEY}
  - ref: limited-access
    name: limited-access
    display_name: Limited Access
    custom_id: limited-access
    type: api-key
    credentials:
      - ref: limited-access-key
        name: limited-access-key
        display_name: Limited Access Key
        type: api-key
        ttl: 0
        api_key: !secret {source: !env LIMITED_ACCESS_API_KEY}
```

For the existing model, add:

```yaml
access:
  auth_strategies:
    - !ref client-key-auth
  acls:
    allow: [full-access, limited-access]
```

For each additional model, use `allow: [full-access]` with the same auth
strategy. Copy the existing model's supported shape with a distinct `ref`,
`name` and `config.route.model.values` alias, then set the selected upstream
target. Do not overwrite the original model list when adding models.

Use `!ref` for the auth strategy relationship. For model access, kongctl
normalizes this reference to the strategy's **name**; do not substitute a
literal UUID. ACL entries are **consumer or group names**. Choose either
`allow` or `deny` for a model; the schema permits only one. Creating an auth
strategy alone does not protect a model: attach it through
`access.auth_strategies`.
Authentication identifies the caller; ACLs decide which model it may use.
See [AI Consumers][consumers] and [AI Consumer Groups][groups].

For group-based variants, inspect
`kongctl explain ai_gateways.consumer_groups`. Declarative consumer
membership and authenticated OIDC claims have different setup needs;
verify group names and claim mapping against the intended caller token.
Do not copy an SE environment's issuer, audience or group prefixes.

## Verify the complete access matrix

Prepare a bounded verification script using the same endpoint, model
aliases and `apikey` header. Read caller keys from the environment; do not
print them, enable shell tracing or embed them in saved examples.

| Caller | Original model | Added model 1 | Added model 2 |
| --- | --- | --- | --- |
| Missing key | 401 | 401 | 401 |
| Invalid key | 401 | 401 | 401 |
| Full access | 200 + completion | 200 + completion | 200 + completion |
| Limited access | 200 + completion | 403 | 403 |

These are the intended results for this key-auth/allow-ACL example. Make
both status and response semantics executable assertions. An upstream 401,
routing 404 or server failure does not count as an authorization denial.
First establish the allowed requests, then verify missing/invalid
credentials and denied model access. Do not accept any non-200 as proof
that the policy works.

For example, after checking the expected HTTP status, assert the expected
gateway rejection message in the JSON body:

```sh
jq -e --arg expected "$expected_message" \
  '.message == $expected and .error == null' "$response_file" >/dev/null
```

Kong's [Key Auth implementation][key-auth] reports a missing key as
`No API key found in request` and an invalid key as `Unauthorized`.
[ACL validation][acl] expects `You cannot consume this service`. Use these
as starting assertions and confirm the signatures for the selected runtime
during rehearsal; an unknown body fails the check and needs diagnosis.

Check the verifier offline with synthetic responses before using it live:
a 401 provider error must fail even though its status matches the missing
key case, and a 200 without a completion must fail. Use request parameters
supported by the selected upstream models; token-budget options can differ.

Make an unexpected acceptance fail the self-test process explicitly:

```sh
if assert_response "$provider_response" 401 401 missing; then
  echo "Verifier accepted a provider error as missing client credentials" >&2
  exit 1
fi
```

Adapt that call to the verifier's interface. A bare `! assert_response ...`
does not enforce failure under `set -e`: Bash exempts negated commands from
errexit. Check the self-test by temporarily making the response assertion
always succeed in a disposable copy; the self-test must then fail.

If configuration propagation delays a check, wait a bounded interval and
retry the affected case; stop after the stated bound. Record the model,
caller label, status and outcome. Do not persist credential-bearing request
logs. Re-run this matrix after the approved pipeline changes the gateway.

## Credential lifecycle

Supply distinct caller keys when creating these credentials, using deferred
`!secret`. This avoids depending on a server-generated key that may be
returned only once. Consumer `api_key` is create-only: rotating it requires
a new credential identity and deliberate retirement of the old credential.
Changing its environment variable on an unchanged apply is not rotation.

Existing provider secrets also do not reconcile through ordinary drift.
For a requested provider rotation, inspect `kongctl plan --help` and use
the specific `--write-secret` selector while generating a new reviewed
plan. The resulting plan owns that write intent; do not add secret-selection
flags to `apply --plan`. Avoid blanket `--write-secrets` for routine CI.

[consumers]: https://developer.konghq.com/ai-gateway/entities/ai-consumer/
[groups]: https://developer.konghq.com/ai-gateway/entities/ai-consumer-group/
[key-auth]:
  https://github.com/Kong/kong/blob/master/kong/plugins/key-auth/handler.lua
[acl]: https://developer.konghq.com/how-to/configure-oidc-with-acl-auth/
