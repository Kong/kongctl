# Inline External Lookup Tags

This example uses YAML tags to resolve an existing Konnect resource directly
in a relationship field during planning.

The manifest demonstrates both kinds of relationship field:

- `portal_id` is a foreign-key field from the Konnect API schema.
- `portal` is a root-level parent selector added by kongctl.

Both fields use the same planner-time lookup mechanism. Examples use `!lookup`
consistently. `!external` is also supported as an exact alias.

```yaml
portal_id: !lookup {name: Shared Developer Portal}
portal: !lookup {name: Shared Developer Portal}
```

Scalar lookups use `field:value` syntax. Mapping lookups can contain multiple
selectors, all of which must match. A known Konnect ID can be bound directly:

```yaml
portal_id: !lookup {id: 00000000-0000-0000-0000-000000000000}
```

The target resource must already exist. It remains externally managed;
kongctl only manages the API publication and portal page declared in this
example.

Preview or apply the example with:

```bash
kongctl plan -f external-tags.yaml --mode apply
kongctl apply -f external-tags.yaml
```

Use an `_external` resource declaration plus `!ref` instead when the external
resource needs a reusable declarative `ref` or managed children spread across
multiple files.
