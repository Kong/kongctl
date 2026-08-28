# External API Parent

This example resolves an existing Catalog API by name and manages an API
version beneath it. The API itself remains external and never receives a
create, update, or delete operation.

Create an API named `Shared API` in Konnect, then preview or apply the example:

```bash
kongctl plan -f external-api-version.yaml --mode apply
kongctl apply -f external-api-version.yaml
```

The scalar `!lookup name:Shared API` form keeps the example concise. Mapping
syntax is available when selector values need YAML composition; `!external`
is supported as an exact alias for `!lookup`.
