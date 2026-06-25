# Federated AI Gateway Example

This example shows a federated layout for AI Gateway resources where a central
AI platform team owns the shared AI Gateway and upstream providers, while a peer
team owns a root-level model that targets those shared providers.

## Structure

```text
ai-gateway/federated/
|-- central-team/
|   `-- ai-gateway.yaml
|-- external-peer-team/
|   `-- support-model.yaml
`-- peer-team/
    `-- support-model.yaml
```

## Ownership

- `central-team/ai-gateway.yaml` defines the shared AI Gateway and two nested
  providers: OpenAI and Anthropic.
- `peer-team/support-model.yaml` defines a standalone `ai_gateway_models`
  entry that references the central gateway with `!ref shared-ai-gateway#id`.
- `external-peer-team/support-model.yaml` defines an `_external` AI Gateway
  stub and points a standalone model at it with
  `!ref external-shared-ai-gateway#id`.

The peer file is standalone in shape, but it still needs the parent gateway
declaration in the same declarative load. Run the directory recursively so
kongctl sees both the central gateway and the peer-owned model:

```sh
kongctl plan -f docs/examples/declarative/ai-gateway/federated --recursive \
  --mode apply
kongctl apply -f docs/examples/declarative/ai-gateway/federated --recursive
```

The external peer file is for a team that does not load the central team's
gateway declaration. Apply or maintain the central gateway first, then plan the
external peer file by itself:

```sh
kongctl plan \
  -f docs/examples/declarative/ai-gateway/federated/external-peer-team
kongctl apply \
  -f docs/examples/declarative/ai-gateway/federated/external-peer-team
```

Set these environment variables before planning or applying:

- `OPENAI_AUTH_HEADER`: full OpenAI authorization header value, such as
  `Bearer ...`
- `ANTHROPIC_API_KEY`: Anthropic API key used as the `x-api-key` header value
