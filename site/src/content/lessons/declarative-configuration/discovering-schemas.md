---
title: Discovering Declarative Schemas
summary: Explore supported resources and generate starter YAML.
order: 8
related:
  - label: Declarative resource reference
    url: https://developer.konghq.com/kongctl/supported-resources/
---

## Goal

You will use `kongctl` to discover supported declarative resources, inspect
their fields, and generate starter YAML.

> _Note:_ This chapter shows how `kongctl` self describes the resources it
> supports. Human users _may_ find this useful, but the real power use case
> is coding agents which can use the CLI itself as the basis for generating
> declarative configurations.

## List supported resources

Run `explain` without a resource path to see what `kongctl` supports:

```shell
kongctl explain
```

Resource paths can describe a top-level resource, a child, or a specific
field.

## Explain a resource

Inspect the fields available for an AI Gateway:

```shell
kongctl explain ai_gateway
```

Move deeper into a child resource by extending the path:

```shell
kongctl explain ai_gateway.models
```

Add `--extended` for more field detail, or use `-o json` to retrieve the
machine-readable schema.

## Generate starter YAML

Generate a commented AI Gateway example:

```shell
kongctl scaffold ai_gateway
```

`explain` answers what a resource supports. `scaffold` provides YAML that you
can adapt when you extend the AI Gateway configuration from the previous
lesson.
