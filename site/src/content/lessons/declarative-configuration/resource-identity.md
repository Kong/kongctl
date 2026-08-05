---
title: Resource Identity
summary: Understand ref, ID, and name fields in declarative configuration.
order: 3
related:
  - label: Declarative resource reference
    url: https://developer.konghq.com/kongctl/supported-resources/
---

## Goal

You will understand how `ref` identifies resources inside declarative
configuration and connects related resources.

## Local and remote identities

A resource can have several identifiers:

<table>
  <thead>
    <tr>
      <th scope="col">Identifier</th>
      <th scope="col">Meaning</th>
    </tr>
  </thead>
  <tbody>
    <tr>
      <td><code>ref</code></td>
      <td>
        <code>kongctl</code> local identifier, never sent to Konnect. Must be
        unique across the entire set of declarative input files per command.
      </td>
    </tr>
    <tr>
      <td><code>id</code></td>
      <td>
        Common Konnect UUID field used to identify resources in URL routes,
        for example <code>ai-gateways/&lt;id&gt;/models</code>.
      </td>
    </tr>
    <tr>
      <td><code>name</code></td>
      <td>
        Common Konnect field with resource-specific uniqueness rules. Not all
        Konnect resources have a <code>name</code>, and not all
        <code>name</code> fields have uniqueness constraints.
      </td>
    </tr>
  </tbody>
</table>

Every declarative resource requires a unique `ref` identifier:

```yaml
ai_gateways:
  - ref: my-aigw
    name: my-aigw
    display_name: My AI Gateway
```

## Cross-resource references with `ref`

The `ref` field lets resources relate to each other within a configuration.

The `!ref` YAML tag reads a field from another declared resource. This
relationship targets the Konnect ID that will belong to `my-aigw`:

```yaml
ai_gateway_models:
  - ref: chat-model
    ai_gateway: !ref my-aigw#id
```

`kongctl` uses these references to order dependent operations and establish
relationships within Konnect.
