# AI Gateway 2.2 development

This branch uses public SDK v0.71.0 with the kongctl request-default
patches. Its generation input includes AI Gateway specification 2.2.0.
The original feature references were specification 2.1.15 and
ai-deck-converter v0.19.0; the converter is a runtime reference, not a
kongctl dependency.

```go
replace github.com/Kong/sdk-konnect-go => github.com/Kong/sdk-konnect-go v0.71.1-0.20260930102655-288b6e157cf0
```

The patched public SDK needs no private-module authentication,
`GOPRIVATE` entry, or `scripts/setup-private-sdk.sh` invocation. Existing
private-SDK scripts and CI support remain available for other branches.
Patch provenance and reproduction instructions are in
[KONGCTL_PATCHES.md](https://github.com/Kong/sdk-konnect-go/blob/288b6e157cf052ca2c6a0a70a90d0ca2aaa0e624/KONGCTL_PATCHES.md).

The public schema omits internal-only MCP token-vault configuration,
Event Gateway AWS IAM authentication and some internal policy variants,
and portal AI settings. Those incidental internal SDK surfaces are not
part of the requested AI Gateway 2.2 feature set. Explain and the reviewed
dump-default inventory now follow the public schema.

## Models and providers

[pre-ga-2.2.yaml](pre-ga-2.2.yaml) demonstrates the Typesafe/Jev provider,
Typesafe target configuration, the `decisions` model capability, multiple
`config.route.model.values` aliases, and an API model with `skills` and
`formats: [{type: passthrough}]`. Set `JEV_AUTHORIZATION` before execution.
Required fields remain explicit in the manifest. `explain` includes these
SDK variants, and create/update payloads preserve them.

## Policy configuration

Policy `config` remains an arbitrary map. Headroom compression for
`ai-prompt-compressor`, credential identifiers for
`ai-rate-limiting-advanced`, and mTLS for `opentelemetry` can be supplied
using the runtime team's configuration schema. That schema is not defined
in this OAS and was unavailable for this implementation. No guessed keys
or schema-specific validation have been added. Exact examples and runtime
verification remain pending upstream schema confirmation.

## Custom policies

[custom-policies.yaml](custom-policies.yaml) demonstrates a streaming plugin
definition and a policy instance. Use `custom_policies` under a gateway or
root-level `ai_gateway_custom_policies` with an explicit `ai_gateway` ref.
Installed definitions require `name`, `type: installed`, `display_name`,
and Lua `schema`; streaming definitions also require Lua `handler`.
Custom definitions inherit gateway protection and namespace scope.

Plan/apply/sync support create, read, update, and delete. Definition creation
precedes policy instances whose `type` matches its name. Removing a
definition requires removing or changing the referencing policy instances
first. Updates send the complete definition because the API uses PUT.
Names are immutable API identifiers; changing a name creates a new
definition, and sync may remove the previous definition.

Read commands are available through `get` and `list`:

```sh
kongctl get ai-gateway custom-policies --gateway-id GATEWAY_ID
kongctl get ai-gateway custom-policies --gateway-id GATEWAY_ID PLUGIN_NAME
```

Dumps include custom definitions, including schema and handler source.
Custom policies retain their beta command designation. AI Gateway 2.2
is reported available in production; CI must validate these endpoints
against the production test organization. SDK and mock HTTP tests alone
do not establish backend availability. Ordinary apply only reads requested
child resources; full sync includes custom-policy reconciliation and can fail if its API
is unavailable. Dumps use the existing child-read warning behavior.

Cloud Gateway support is outside this change.

## E2E coverage

The following live scenarios exercise management API behavior:

- `ai-gateway/typesafe-decisions`: Typesafe provider/target, decisions, and
  alias replacement and expansion.
- `ai-gateway/model-passthrough`: passthrough format, route and upstream URL
  updates.
- `ai-gateway/model-skills`: skills API capability and combination with files.
- `ai-gateway/custom-policy`: streaming definitions, instances, and deletion
  dependencies.

Each covers creation, read-back, updates, convergence, dump/reload, and
cleanup. Existing `ai-gateway/runtime-2-1` coverage also verifies multiple
aliases on an OpenAI model. Run one with
`make test-e2e-scenarios SCENARIO=ai-gateway/typesafe-decisions`.

These scenarios follow the AI Gateway suite's default stable classification:
backend unavailability fails CI. They do not establish data-plane behavior.
Policy-schema-specific cases remain pending the upstream schemas described
above; installed custom plugins require separate runtime prerequisites.
