# APIOps OpenAPI Source of Truth

Use this file when generating or updating declarative `apis` resources from
OpenAPI documents.

## Core Rule

Treat OpenAPI files as the source of truth for:

- `apis[].name`
- `apis[].description`
- `apis[].version`
- `apis[].versions[].version`
- `apis[].versions[].spec`

Prefer `!file` extraction over duplicated literal values so API metadata stays
aligned with spec changes.

## Modeling Pattern

1. Keep OpenAPI files in their existing repository locations.
2. Do not require moving specs under `konnect/resources/` unless the user asks.
3. Store one or more specs per API (one per version when versioned specs are
   separate).
4. Populate API metadata from `info.*` in each OpenAPI file.
5. Keep `versions[].spec` pointing to the full OpenAPI document file.

## Canonical Example

```yaml
apis:
  - ref: sms
    name: !file <path-to-existing-openapi-spec-v1>#info.title
    description: !file <path-to-existing-openapi-spec-v1>#info.description
    version: !file <path-to-existing-openapi-spec-v1>#info.version
    versions:
      - ref: sms-v1
        version: !file <path-to-existing-openapi-spec-v1>#info.version
        spec: !file <path-to-existing-openapi-spec-v1>
      - ref: sms-v2
        version: !file <path-to-existing-openapi-spec-v2>#info.version
        spec: !file <path-to-existing-openapi-spec-v2>
```

## Repository Example

When working in the `kongctl` repository, use this concrete example:

- `docs/examples/declarative/portal/apis.yaml`

## Validation Loop

After generating or updating API config:

```bash
kongctl plan -f <resources-path> --recursive --mode apply -o json
kongctl diff -f <resources-path> --recursive --mode apply -o text
```
