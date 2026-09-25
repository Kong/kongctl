# AI Gateway 2.2 development

This branch uses the patched internal SDK based on v0.5.0. Its generation
input remains AI Gateway specification 2.1.16. The feature references for
this work are specification 2.1.15 and ai-deck-converter v0.19.0; the
converter is a runtime reference, not a new kongctl dependency.

The internal SDK requires access to the private Kong repository:

```go
replace github.com/Kong/sdk-konnect-go => github.com/Kong/sdk-konnect-go-internal v0.5.1-0.20260925165819-ae595d8c3fce
```

For direct module downloads, configure Git authentication with read access
to that repository and include `github.com/Kong/sdk-konnect-go-internal`
in `GOPRIVATE` and `GONOSUMDB`. If `GONOPROXY` is explicitly configured,
include the same module there. Preserve any existing patterns.

Alternatively, run `scripts/setup-private-sdk.sh` with
`GH_TOKEN_PRIVATE_READ` (or `GH_PRIVATE_READ_TOKEN`) set through your normal
secret management. It clones the pinned commit into `.private` and changes
the working copy's replacement to that local checkout. Keep the remote
replacement above in commits. Existing test and E2E CI support requires
the `GH_TOKEN_PRIVATE_READ` repository secret with private SDK read access.

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
Custom policies are marked beta. They are implemented even though their
endpoints were reported unavailable in dev. SDK and mock HTTP tests do
not establish backend availability: live tests need an environment with
the feature enabled. Ordinary apply only reads requested child resources;
full sync includes custom-policy reconciliation and can fail if its API
is unavailable. Dumps use the existing child-read warning behavior.

Cloud Gateway support is outside this change.
