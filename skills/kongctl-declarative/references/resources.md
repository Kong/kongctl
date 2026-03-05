# Declarative Resource Patterns

Use this file as the portable schema reference when the skill is installed
outside the `kongctl` repository.

## Recommended Layout

```text
konnect/resources/
  control-planes.yaml
  portals.yaml
  apis.yaml
```

Match existing repository conventions when a layout already exists.
Do not require OpenAPI specs to be placed under the declarative resources
directory.

## Top-Level Resource Keys

- `apis`
- `application_auth_strategies`
- `catalog_services`
- `control_planes`
- `event_gateways`
- `organization` (contains `teams`)
- `portals`

## Parent and Child Rules

- Parent resources support `kongctl` metadata:
  - `apis`
  - `catalog_services`
  - `portals`
  - `application_auth_strategies`
  - `control_planes`
  - `event_gateways`
- Child resources do not support `kongctl` metadata.
- Put ownership defaults in `_defaults.kongctl`.

## `_defaults` and Ownership

Use `_defaults` to apply namespace and protected values in one file:

```yaml
_defaults:
  kongctl:
    namespace: team-alpha
    protected: false
```

Resource-level `kongctl` values override `_defaults`.

## YAML Tags

- `!file`: Load file content, optionally with extraction.
- `!ref`: Resolve another resource by `ref`, optionally with `#field`.

Examples:

```yaml
name: !file <existing-openapi-path>#info.title
description: !file <existing-openapi-path>#info.description
portal_id: !ref dev-portal#id
```

`!file` paths resolve relative to the config file and must remain within the
`--base-dir` boundary.

## Resource Cheat Sheets

### `control_planes`

Common fields:

- `ref`
- `name`
- `description`
- `cluster_type`
- `auth_type`
- `cloud_gateway`
- `proxy_urls`
- `labels`
- `kongctl`

Example:

```yaml
control_planes:
  - ref: cp-main
    name: "my-control-plane"
    cluster_type: "CLUSTER_TYPE_CONTROL_PLANE"
```

### `portals`

Common fields:

- `ref`
- `name`
- `display_name`
- `description`
- `authentication_enabled`
- `rbac_enabled`
- `default_api_visibility`
- `default_page_visibility`
- `labels`
- `kongctl`

Common child blocks:

- `pages`
- `snippets`
- `customization`
- `auth_settings`
- `custom_domain`
- `teams`
- `email_config`
- `email_templates`
- `assets`

Example:

```yaml
portals:
  - ref: dev-portal
    name: "my-dev-portal"
    display_name: "My Dev Portal"
    default_api_visibility: "private"
    default_page_visibility: "private"
```

### `apis`

Common fields:

- `ref`
- `name`
- `description`
- `version`
- `slug`
- `labels`
- `attributes`
- `spec_content`
- `kongctl`

Common child blocks:

- `versions`
- `publications`
- `implementations`
- `documents`

Example using OpenAPI:

```yaml
apis:
  - ref: payments-api
    name: !file <existing-openapi-path>#info.title
    description: !file <existing-openapi-path>#info.description
    versions:
      - ref: payments-v1
        version: !file <existing-openapi-path>#info.version
        spec: !file <existing-openapi-path>
    publications:
      - ref: payments-pub
        portal_id: !ref dev-portal#id
        visibility: public
```

### `application_auth_strategies`

Common fields:

- `ref`
- `name`
- `display_name`
- `strategy_type` (`key_auth` or `openid_connect`)
- `configs`
- `labels`

Minimal key-auth example:

```yaml
application_auth_strategies:
  - ref: key-auth-main
    name: "my-key-auth"
    display_name: "My Key Auth"
    strategy_type: key_auth
    configs:
      key_auth:
        key_names:
          - X-API-Key
```

### `organization.teams`

Structure:

- `organization`
- `teams` (array of objects with `ref`, `name`, optional metadata fields)

Example:

```yaml
organization:
  teams:
    - ref: platform-team
      name: "Platform Team"
```

## Reference Linking Patterns

- API publication to portal:
  - `portal_id: !ref <portal-ref>#id`
- Publication auth strategy reference:
  - `auth_strategy_ids: [!ref <auth-strategy-ref>#id]`
- Child-to-parent references:
  - prefer `!ref` instead of hard-coded UUIDs

## Schema Discovery Without Local Docs

When field-level uncertainty remains, sample live schema with dump commands:

```bash
kongctl dump declarative --resources=portal --include-child-resources -o yaml
kongctl dump declarative --resources=api --include-child-resources -o yaml
kongctl dump declarative --resources=control_planes -o yaml
```

Then adapt generated shape to the target repository layout and naming.

## Validation Loop

```bash
kongctl plan -f <path> --mode apply -o json
kongctl diff -f <path> --mode apply -o text
```
