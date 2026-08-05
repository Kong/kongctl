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

YAML Tags allow you to load external data or express relationships within the
configuration without hardcoding the values directly into the YAML.

| Tag         | Purpose                                                |
| ----------- | ------------------------------------------------------ |
| `!ref`      | Read a field from a resource in the same loaded input. |
| `!lookup`   | Find an existing Konnect resource during planning.     |
| `!external` | Use the exact alias for `!lookup`.                     |
| `!file`     | Load a local file or a value within a structured file. |
| `!env`      | Read a value from an environment variable.             |

## YAML tag syntax

A tag is attached to a YAML value. It tells `kongctl` how to resolve the input
that follows the tag before using the result as the field value:

```yaml label="Tag anatomy"
target_field: !<tag> <tag-input>
```

### Select a field or load the whole value

Use `#` to select a field from a referenced resource or structured value. The
text before `#` identifies the source, and the text after it identifies the
field or dotted path:

```yaml label="Selecting values"
ai_gateway: !ref my-aigw#id
title: !file ./specs/openapi.yaml#info.title
token: !env APP_CONFIG#credentials.token
```

For `!ref`, omitting `#<field>` selects `id` by default. For `!file`, omitting
`#<path>` loads the entire resolved file value. YAML and JSON files load as
structured values, while text files load as their complete content.

### Compact and full mapping forms

Some tags accept either a compact scalar or a full YAML mapping. These `!file`
values are equivalent:

```yaml label="Equivalent file tags"
# Compact scalar form
display_name: !file ./metadata.yaml#portal.display_name

# Full mapping form
display_name: !file
  path: ./metadata.yaml
  extract: portal.display_name
```

The `!env` mapping form uses `var` and optional `extract` fields. A `!lookup`
mapping contains the fields used to find the resource and can use YAML's
inline flow style or block style.

## Example YAML tags

Reference the ID of an AI Gateway declared in the same input configuration:

```yaml
ai_gateway: !ref my-aigw#id
```

Find an AI Gateway already present in Konnect:

```yaml
ai_gateway: !lookup { name: shared-aigw-name }
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
