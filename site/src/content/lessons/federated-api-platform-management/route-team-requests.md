---
title: Route Team Requests
summary: Send requests through each team-owned AI Gateway model.
order: 9
related:
  - label: AI Gateway model documentation
    url: https://developer.konghq.com/ai-gateway/entities/ai-model/
---

## Goal

You will send requests through the routes owned by Product and Engineering and
confirm that each request selects the intended AI Gateway model.

## Understand the routes

The teams now manage three AI Gateway models on the shared Platform gateway:

| Owner       | Route                      | Request `model` value       |
| ----------- | -------------------------- | --------------------------- |
| Product     | `/product`                 | `product-assistant`         |
| Engineering | `/engineering`             | `engineering-assistant`     |
| Engineering | `/engineering/code-review` | `engineering-code-reviewer` |

All three currently use the Platform team's `platform-openai` provider and
target the upstream `gpt-4o` model. The separate routes and model selectors
allow each team to evolve its target, policies, and capabilities independently.

## Set the local proxy URL

Use the HTTPS proxy port exposed by the local data plane:

```shell label="Set the proxy URL"
export AIGW_PROXY_URL="https://localhost:8443"
```

The commands below use `--insecure` because the local proxy certificate is not
issued for `localhost`.

## Call the Product model

The request path and body selector must both match the Product configuration:

```shell rows=10 label="Call the Product model"
curl --insecure --no-progress-meter --fail-with-body \
  --request POST "${AIGW_PROXY_URL}/product/chat/completions" \
  --header "Accept: application/json" \
  --json '{
    "model": "product-assistant",
    "messages": [
      {
        "role": "user",
        "content": "Describe the value of an AI product platform."
      }
    ]
  }'
```

## Call the Engineering model

Send an engineering question through the independently managed Engineering
route:

```shell rows=10 label="Call the Engineering model"
curl --insecure --no-progress-meter --fail-with-body \
  --request POST "${AIGW_PROXY_URL}/engineering/chat/completions" \
  --header "Accept: application/json" \
  --json '{
    "model": "engineering-assistant",
    "messages": [
      {
        "role": "user",
        "content": "Explain why API contracts help engineering teams."
      }
    ]
  }'
```

## Call the independently added model

The more specific route selects the code-review model that Engineering added
without loading the Platform configuration:

```shell rows=10 label="Call the code-review model"
curl --insecure --no-progress-meter --fail-with-body \
  --request POST \
  "${AIGW_PROXY_URL}/engineering/code-review/chat/completions" \
  --header "Accept: application/json" \
  --json '{
    "model": "engineering-code-reviewer",
    "messages": [
      {
        "role": "user",
        "content": "Give me three checks for reviewing an API change."
      }
    ]
  }'
```

Each successful response traveled through the same Platform-owned AI Gateway,
but its route and `model` value selected configuration owned by a satellite
team. If a request returns `name resolution failed`, confirm that its `model`
value exactly matches the corresponding value in the table.
