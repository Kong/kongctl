# Federated AI Gateway Example

This example shows a federated layout for AI Gateway resources where a central
AI platform team owns the shared AI Gateway and upstream model providers, while
a peer team owns root-level policies, agents, consumers, consumer groups,
models, MCP Servers, vaults, and data plane certificates that target the shared
gateway.

## Structure

The shared gateway example uses these team directories:

```text
ai-gateway/federated/
|-- central-team/
|   `-- ai-gateway.yaml
|-- external-peer-team/
|   `-- support-model.yaml
`-- peer-team/
    |-- support-agent.yaml
    |-- support-consumer.yaml
    |-- support-consumer-group.yaml
    |-- support-data-plane-certificate.yaml
    |-- support-mcp-server.yaml
    |-- support-model.yaml
    |-- support-policy.yaml
    `-- support-vault.yaml
```

## Ownership

Supported children have independent namespace and protection. The central and
peer folders share a namespace; the external peer model uses a separate
namespace so its sync cannot prune the peer folder's model. Data plane
certificates remain gateway-scoped and excluded from federation. Existing
unowned children need explicit adoption. See the
[support matrix](../../../../ai-gateway-federation.md#support-matrix).

- `central-team/ai-gateway.yaml` defines the shared AI Gateway, a shared
  key-auth strategy, and two nested model providers: OpenAI
  and Anthropic.
- `peer-team/support-policy.yaml` defines standalone `ai_gateway_policies`
  entries that reference the central gateway with
  `!ref shared-ai-gateway#id`.
- `peer-team/support-agent.yaml` defines a standalone `ai_gateway_agents` entry
  that references the central gateway with `!ref shared-ai-gateway#id` and
  attaches the central auth strategy with an explicit
  `!ref shared-key-auth`.
- `peer-team/support-consumer.yaml` defines a standalone
  `ai_gateway_consumers` entry that references the central gateway with
  `!ref shared-ai-gateway#id` and attaches the policy with an explicit
  `!ref peer-mask-sensitive-data`.
- `peer-team/support-consumer-group.yaml` defines a standalone
  `ai_gateway_consumer_groups` entry that references the central gateway with
  `!ref shared-ai-gateway#id` and attaches the policy with an explicit
  `!ref peer-mask-sensitive-data`.
- `peer-team/support-model.yaml` defines a standalone `ai_gateway_models`
  entry that references the central gateway with `!ref shared-ai-gateway#id`
  and attaches the policy with an explicit `!ref peer-mask-sensitive-data`.
- `peer-team/support-mcp-server.yaml` defines a standalone
  `ai_gateway_mcp_servers` entry that references the central gateway with
  `!ref shared-ai-gateway#id` and attaches the central auth strategy with
  an explicit `!ref shared-key-auth`.
- `peer-team/support-vault.yaml` defines a standalone `ai_gateway_vaults` entry
  that references the central gateway with `!ref shared-ai-gateway#id`.
- `peer-team/support-data-plane-certificate.yaml` defines a standalone
  `ai_gateway_data_plane_certificates` entry that references the central
  gateway with `!ref shared-ai-gateway#id`.
- `external-peer-team/support-model.yaml` defines an `_external` AI Gateway
  stub and points a standalone model at it with
  `!ref external-shared-ai-gateway#id`.

The peer files are standalone in shape, but they still need the parent gateway
declaration in the same declarative load. Run the directory recursively so
kongctl sees both the central gateway and the peer-owned resources. Select
these team directories explicitly; the other subdirectories contain independent
examples:

```sh
kongctl plan \
  -f docs/examples/declarative/ai-gateway/federated/central-team \
  -f docs/examples/declarative/ai-gateway/federated/peer-team --recursive \
  --mode sync
kongctl sync \
  -f docs/examples/declarative/ai-gateway/federated/central-team \
  -f docs/examples/declarative/ai-gateway/federated/peer-team --recursive
```

The external peer file is for a team that does not load the central team's
gateway declaration. Sync the central gateway first, then plan the
external peer file by itself:

```sh
kongctl plan \
  --mode sync \
  -f docs/examples/declarative/ai-gateway/federated/external-peer-team
kongctl sync \
  -f docs/examples/declarative/ai-gateway/federated/external-peer-team
```

Set these environment variables before planning or syncing:

- `OPENAI_AUTH_HEADER`: full OpenAI authorization header value, such as
  `Bearer ...`
- `ANTHROPIC_API_KEY`: Anthropic API key used as the `x-api-key` header value

## Child management examples

Run each example independently, following its README for setup and cleanup:

- [Agents](agents): agent ownership across team namespaces.
- [Auth strategies](auth-strategies): gateway and model auth strategy ownership.
- [Custom policies](custom-policies): custom definitions and policy instances.
- [Policies](policies): gateway and model policy ownership.
- [Providers](providers): model provider and model ownership.
- [Remaining children](remaining): MCP servers, vaults, consumers, consumer
  groups, CA certificates, and SNIs.
