# AI Gateway Examples

This directory contains declarative configuration examples for Konnect AI
Gateway resources.

- [ai-gateway.yaml](ai-gateway.yaml) defines a root AI Gateway resource with
  a nested OpenAI provider, env vault, data plane node, data plane certificate,
  policy, consumer, agent, consumer group, model that targets that provider,
  and a conversion-only MCP Server.
- [data-plane-certificates.yaml](data-plane-certificates.yaml) defines AI
  Gateway data plane certificates using both nested
  `data_plane_certificates` and root-level
  `ai_gateway_data_plane_certificates` declarations.
- [federated](federated) shows a multi-folder
  layout where a central team owns an AI Gateway and providers, while a peer
  team owns root-level policies, agents, consumers, consumer groups, models, MCP
  Servers, vaults, nodes, and data plane certificates that reference the shared
  gateway.

Set `OPENAI_AUTH_HEADER` to the full upstream authorization header value
before applying the example, for example `Bearer ...`.
