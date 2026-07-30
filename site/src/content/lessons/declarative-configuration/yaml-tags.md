---
title: YAML Tags
summary: Load values and connect resources with kongctl YAML tags.
order: 4
related:
  - label: Declarative configuration documentation
    url: https://developer.konghq.com/kongctl/declarative/
---

## Goal

You will recognize the YAML tags `kongctl` uses for relationships and external
values.

## Extend YAML values

Tags let a configuration load data or resolve a relationship without copying
the final value into the YAML.

| Tag         | Purpose                                                |
| ----------- | ------------------------------------------------------ |
| `!ref`      | Read a field from a resource in the same loaded input. |
| `!lookup`   | Find an existing Konnect resource during planning.     |
| `!external` | Use the exact alias for `!lookup`.                     |
| `!file`     | Load a local file or a value within a structured file. |
| `!env`      | Read a value from an environment variable.             |

## Recognize tag syntax

Reference the ID of a declared AI Gateway:

```yaml
ai_gateway: !ref my-aigw#id
```

Find an AI Gateway already present in Konnect:

```yaml
ai_gateway: !lookup { name: shared-aigw }
```

Load a certificate from a file:

```yaml
cert: !file ./certs/data-plane.pem
```

Read an upstream authorization header without storing it in the file:

```yaml
value: !env OPENAI_AUTH_HEADER
```

Relationship support depends on the resource field. Use `kongctl explain` to
see which tags and selectors a field accepts. External resources and lookups
are explored further in the Federated Management chapter.
