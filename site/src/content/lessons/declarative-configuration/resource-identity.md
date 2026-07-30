---
title: Resource Identity
summary: Understand ref, ID, and name fields in declarative configuration.
order: 2
related:
  - label: Declarative resource reference
    url: https://developer.konghq.com/kongctl/supported-resources/
---

## Goal

You will understand how `ref` identifies resources inside declarative
configuration and connects related resources.

## Local and remote identities

A resource can have several identifiers:

- `ref` identifies it within the files loaded by one `kongctl` command.
- `id` is usually a UUID assigned by Konnect.
- `name` is a Konnect field whose uniqueness rules depend on the resource.

Every declarative resource requires a `ref`:

```yaml
ai_gateways:
  - ref: my-aigw
    name: my-aigw
    display_name: My AI Gateway
```

The `ref` must be unique across the complete input configuration. It is used
while loading and planning, but is not written to Konnect.

## Connect resources with ref

The `!ref` tag reads a field from another declared resource. This relationship
targets the Konnect ID that will belong to `my-aigw`:

```yaml
ai_gateway_models:
  - ref: chat-model
    ai_gateway: !ref my-aigw#id
```

`kongctl` uses references to order dependent operations and replace the tag
with the correct remote value.
