---
title: Federate Engineering Team
summary: Make the Engineering configuration independent with a lookup.
order: 6
related:
  - label: YAML tags lesson
    url: https://kong.github.io/kongctl/declarative-configuration/yaml-tags/
---

## Goal

You will remove Engineering's local dependency on the Platform configuration
without changing any resources in Konnect.

## Replace the local reference

The initial apply loaded all three directories together. That allowed the
relationship in `engineering/model.yaml` to resolve against
`platform/ai-gateway.yaml`:

```yaml label="Find this in engineering/model.yaml"
ai_gateway_models:
  - ref: engineering-assistant
    ai_gateway: !ref platform-aigw#id
    type: model
```

Engineering cannot resolve that `ref` when it loads only its own file. Open
`engineering/model.yaml` in your editor and change the relationship to:

```yaml label="Change it to this"
ai_gateway_models:
  - ref: engineering-assistant
    ai_gateway: !lookup { name: platform-aigw }
    type: model
```

## Apply Engineering independently

Apply only the Engineering configuration:

```shell label="Run this..."
kongctl apply -f engineering/model.yaml
```

Notice how you only give this `apply` operation the single
engineering model declaration file.

The result should be a no-op:

```text label="Example output"
No changes needed. Resources match configuration.
```

The relationship source changed, but the desired Engineering model did not.
During planning, `kongctl` resolves the lookup to the same Platform AI Gateway
ID used by the initial shared apply.
