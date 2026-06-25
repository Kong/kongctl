# AI Gateway Examples

This directory contains declarative configuration examples for Konnect AI
Gateway resources.

- [ai-gateway.yaml](ai-gateway.yaml) defines a root AI Gateway resource with
  a nested OpenAI provider and a model that targets that provider.
- [../ai-gateway-federated](../ai-gateway-federated) shows a multi-folder
  layout where a central team owns an AI Gateway and providers, while a peer
  team owns a root-level model that references the shared gateway.

Set `OPENAI_AUTH_HEADER` to the full upstream authorization header value
before applying the example, for example `Bearer ...`.
