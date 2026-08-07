# AI Gateway Examples

This directory contains declarative configuration examples for Konnect AI
Gateway resources.

- [ai-gateway.yaml](ai-gateway.yaml) defines a root AI Gateway resource with
  a nested OpenAI provider, env vault, data plane certificate, policies,
  consumer, agent, consumer group, model that targets that provider, and a
  conversion-only MCP Server.
- [ai-gateway-remote.yaml](ai-gateway-remote.yaml) is the same full example
  with the data plane certificate PEM inlined, so it can be loaded directly
  from a remote URL with `-f https://...` and does not require any sibling
  files.
- [config-store-vault.yaml](config-store-vault.yaml) connects a nested Config
  Store to a Konnect Vault with `!ref`, then uses a Vault reference for an
  OpenAI provider authorization header.
- [data-plane-certificates.yaml](data-plane-certificates.yaml) defines AI
  Gateway data plane certificates using both nested
  `data_plane_certificates` and root-level
  `ai_gateway_data_plane_certificates` declarations.
- [federated](federated) shows a multi-folder
  layout where a central team owns an AI Gateway and providers, while a peer
  team owns root-level policies, agents, consumers, consumer groups, models, MCP
  Servers, vaults, and data plane certificates that reference the shared
  gateway.

Set `OPENAI_AUTH_HEADER` to the full upstream authorization header value before
applying `ai-gateway.yaml` or the federated example. Set `OPENAI_API_KEY` to
only the token when using `ai-gateway-remote.yaml`; its `!secret` composition
adds the `Bearer ` prefix.

Before applying `config-store-vault.yaml`, add a secret named
`openai-auth-header` to `support-config-store`. Its value should be the full
OpenAI authorization header, for example `Bearer ...`. Config Store resources
manage the store itself, but do not manage the secrets it contains.
Set `OPENAI_VAULT_REFERENCE` to
`{vault://support-secrets/openai-auth-header}` so the write-only header field
uses the required deferred declaration without putting credential material in
the environment.
