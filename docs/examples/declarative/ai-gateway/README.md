# AI Gateway Examples

This directory contains declarative configuration examples for Konnect AI
Gateway resources.

Declare an explicit `name` for each managed gateway and name-bearing child,
including nested children. `ref` is a local reference identifier and does not
supply a missing API name. When upgrading a manifest that relied on this
legacy default, add the existing Konnect API name and keep the local `ref`.

For AI Gateway 2.1 features, set `min_runtime_version: "2.1"` on the
gateway. Set `runtime_auto_upgrade: false` to keep that minimum under
explicit control. Runtime upgrades are applied before child changes that
can require the newer version; downgrades follow child changes.

Model targets accept `input_cost_list`, `output_cost_list`, and
`cache_read_cost_list` under `targets[].config`. Each entry has a `modal`
(such as `text` or `audio`) and a `cost` per million tokens. Policies accept
an optional `condition` expression. MCP `listener` and
`conversion-listener` resources accept `config.allowed_versions` and
`config.cache`, with `tools_list` and `discover` cache hints containing
`ttl_ms` and `cache_scope`. MCP `upstream-server` resources accept
`config.server.upstream_protocol_version`.

Models can expose multiple selector aliases through
`config.route.model.values`, for example `[support, support-alias]`.

Models and agents support conditional access rules under `access.acls` with
AI Gateway runtime 2.2 or later. Configure exactly one of `allow`, `deny`,
`allow_when`, or `deny_when`. The conditional variants accept arrays of CEL
boolean expressions evaluated against the request context, for example:

```yaml
access:
  acls:
    deny_when:
      - "http.headers.x_end_user_id == null"
```

- [ai-gateway.yaml](ai-gateway.yaml) defines a root AI Gateway resource with
  a nested OpenAI provider, env vault, data plane certificate, policies,
  consumer, agent, consumer group, model that targets that provider, and a
  conversion-only MCP Server.
- [ai-gateway-remote.yaml](ai-gateway-remote.yaml) is the same full example
  with the data plane certificate PEM inlined, so it can be loaded directly
  from a remote URL with `-f https://...` and does not require any sibling
  files.
- [openai-llm](openai-llm) creates an AI Gateway, an OpenAI provider, a model,
  and a data plane certificate, then runs a local Docker data plane and sends
  an OpenAI-compatible chat request through it.
- [templates](templates) centralizes model token costs used by AI Rate Limiting
  Advanced, then applies the shared costs and policy across multiple AI model
  resources.
- [config-store-vault.yaml](config-store-vault.yaml) creates a secret in a
  nested Config Store, connects the store to a Konnect Vault with `!ref`, then
  uses a Vault reference for an OpenAI provider authorization header.
- [data-plane-certificates.yaml](data-plane-certificates.yaml) defines AI
  Gateway data plane certificates using both nested
  `data_plane_certificates` and root-level
  `ai_gateway_data_plane_certificates` declarations.
- [runtime-tls.yaml](runtime-tls.yaml) defines a runtime certificate, CA
  certificate, and SNI. It demonstrates a public certificate loaded with
  `!file` and a private key loaded at execution time with
  `!secret {source: !file ...}`. Runtime certificates are distinct from data
  plane certificates.
- [typesafe-models.yaml](typesafe-models.yaml) demonstrates a Typesafe/Jev
  provider and targets, the `decisions` capability, multiple model selector
  aliases, and a skills API model with passthrough format. Requires AI Gateway
  runtime 2.2 or later. Set `JEV_AUTHORIZATION` to the provider's full
  authorization header value before applying it.
- [custom-policies.yaml](custom-policies.yaml) defines a streaming custom
  policy with Lua schema and handler source, plus a policy instance that uses
  it. Requires AI Gateway runtime 2.2 or later and a backend with the custom
  policy API available. See [Custom policies](#custom-policies) for maturity
  and lifecycle details.
- [federated](federated) shows a multi-folder
  layout where a central team owns an AI Gateway and providers, while a peer
  team owns root-level policies, agents, consumers, consumer groups, models, MCP
  Servers, vaults, and data plane certificates that reference the shared
  gateway.

Set `OPENAI_AUTH_HEADER` to the full upstream authorization header value before
applying `ai-gateway.yaml` or the federated example. Set `OPENAI_API_KEY` to
only the token when using `ai-gateway-remote.yaml`; its `!secret` composition
adds the `Bearer ` prefix.

Set `OPENAI_AUTH_HEADER` to the full OpenAI authorization header before
applying `config-store-vault.yaml`. kongctl creates the Config Store secret
from this deferred source without placing its value in the plan. The model
provider uses the public
`{vault://support-secrets/openai-auth-header}` reference, which remains visible
in plans and is resolved by Konnect when the provider uses it.

Before applying `runtime-tls.yaml`, place the matching runtime certificate and
private key at `certs/runtime.pem` and `certs/runtime.key`, and place the CA
certificate at `certs/ca.pem`. The private-key file is intentionally not
included in this repository.

## Policy configuration

Policy `config` is passed through as an arbitrary map. Use the policy schema
for your target runtime version to choose valid options; kongctl does not
validate plugin-specific configuration fields.

## Custom policies

Custom policies currently carry beta maturity in kongctl and require a backend
with custom-policy API support. This designation is specific to kongctl's
custom-policy resources and commands.

Declare definitions under a gateway's `custom_policies` collection, or use
root-level `ai_gateway_custom_policies` with an explicit `ai_gateway` ref.
Both `installed` and `streaming` definitions require `name`, `type`,
`display_name`, and Lua `schema`. Streaming definitions also require Lua
`handler` source. Installed definitions require the plugin implementation to
be available in the runtime independently of the declarative definition.

Create policy instances under `policies`, using the custom definition's name
as their `type`, as shown in `custom-policies.yaml`. Definitions inherit their
gateway's protection and namespace scope. Plan/apply/sync create definitions
before instances that use them. Remove referencing instances before deleting
a definition. Updates send the complete definition because the API uses PUT.
Definition names are immutable; changing a name creates a new definition.

Read definitions through `get` or `list`:

```sh
kongctl get ai-gateway custom-policies --gateway-id GATEWAY_ID
kongctl get ai-gateway custom-policies --gateway-id GATEWAY_ID PLUGIN_NAME
```

Dumps include custom definitions with their Lua schema and handler source.
