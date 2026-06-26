# `kongctl` Declarative Resource Reference

This document is a reference for `kongctl` declarative
configuration. It lists supported resource types and common field-level values.
Resource configurations are provided as YAML files and can be expressed as one
or more files passed to `kongctl` declarative commands.

The definitive schema for the version of `kongctl` you are using is generated
by the CLI itself:

```sh
kongctl explain api
kongctl explain portal.identity_providers --output yaml
kongctl scaffold api
```

Use `kongctl explain` to confirm accepted field names, required fields,
preferred YAML tags, root keys, and nested or root declaration support. Use
`kongctl scaffold` to generate starter YAML for a resource path. If this page
differs from the installed CLI, follow the CLI output.

See the [declarative configuration guide](declarative.md) for information on
the feature, commands, and options.

## File-level defaults (`_defaults`)

Use `_defaults.kongctl` to apply default `namespace` and `protected` metadata
to parent resources in this file. Resource-level `kongctl` values override
these defaults.

```yaml
_defaults:
  kongctl:
    namespace: platform-team
    protected: false
```

## YAML Tags

Use YAML tags in field values to load files or reference other resources.

- `!file`: Load content from a file. Supports `path#extract.path` and
  `path`/`extract` map form.
- `!env`: Load string content from an environment variable. Supports
  `VAR#extract.path` and `var`/`extract` map form.
- `!ref`: Reference another declarative resource by `ref`.
  `resource-ref#field` is supported; the default field is `id`.
- `!ref` is intended for string fields.
- `string (uuid)` and `array[string(uuid)]` annotations in this document
  describe API value types. In declarative config, prefer `!ref` and avoid
  literal UUID values.
- For unmanaged/external resources, prefer `_external.selector` and then
  reference that resource by `!ref` from other fields.
- Large text/spec fields are commonly loaded with `!file`.
- `!file` paths are resolved relative to the config file and must remain
  within the configured base directory boundary.

```yaml
portals:
  - ref: docs-portal
    _external:
      selector:
        matchFields:
          name: "Docs Portal"

apis:
  - ref: billing-api
    publications:
      - ref: billing-publication
        portal_id: !ref docs-portal
```

## Audit Logs

Audit-log webhook destinations are organization-scoped Konnect resources.
Declarative config supports them as external references so managed portal
audit-log webhooks can point at destinations created elsewhere.

```yaml
audit-logs:
  destinations:
    - ref: string
      _external:
        id: string # destination UUID, or use selector
        selector:
          matchFields:
            name: string
```

Only `_external.id` and `_external.selector.matchFields.name` are supported.
Audit-log webhook destinations cannot declare `kongctl` metadata and are not
created, updated, or deleted by declarative apply.

## APIs

[API Specification](https://developer.konghq.com/api/konnect/api-builder/v3/#/operations/create-api)
[Example](examples/declarative/basic/api.yaml)

```yaml
apis:
  - ref: string
    name: string required (1-255 chars)
    description: string (nullable)
    version: string (1-255 chars, nullable)
    slug: string (pattern: ^[\w-]+$, nullable)
    labels: object [string]string
      key: value
    attributes: object [string]array[string]
      key:
        - value
    versions: # https://developer.konghq.com/api/konnect/api-builder/v3/#/operations/create-api-version
      - ref: string
        version: string
        spec: object required
          content: string required (OpenAPI or AsyncAPI content; json or yaml) # prefer: !file ./specs/api.yaml
    publications: # https://developer.konghq.com/api/konnect/api-builder/v3/#/operations/publish-api-to-portal
      - ref: string
        portal_id: string required (uuid) # prefer: !ref <portal-ref>
        auto_approve_registrations: boolean
        auth_strategy_ids: array[string(uuid)] (nullable, max 1 item) # prefer: !ref values
        visibility: One of (public | private)
    implementations: # https://developer.konghq.com/api/konnect/api-builder/v3/#/operations/create-api-implementation
      - ref: string
        type: string required
        service_reference:
          service:
            id: string required (uuid) # prefer: !ref <gateway-service-ref>
            control_plane_id: string required (uuid) # prefer: !ref <control-plane-ref>
        control_plane_reference:
          control_plane:
            control_plane_id: string required (uuid) # prefer: !ref <control-plane-ref>
    documents: # https://developer.konghq.com/api/konnect/api-builder/v3/#/operations/create-api-document
      - ref: string
        content: string required (markdown) # prefer: !file ./docs/page.md
        title: string
        slug: string (pattern: ^[\w-]+$)
        status: One of (published | unpublished)
        parent_document_id: string (uuid, nullable) # prefer: !ref <document-ref>
        children:
          - ref: string
            content: string required (markdown) # prefer: !file ./docs/page.md
            title: string
            slug: string (pattern: ^[\w-]+$)
            status: One of (published | unpublished)
```

API specifications must be declared on API versions with `versions[].spec` or
root-level `api_versions[].spec`; `apis[].spec_content` is not supported in
declarative configuration.

## Application Auth Strategies

[API Specification](https://developer.konghq.com/api/konnect/application-auth-strategies/v2/#/operations/create-app-auth-strategy)
[Example](examples/declarative/portal/auth-strategies.yaml)

```yaml
application_auth_strategies:
 - ref: string
   name: string required
   display_name: string required
   strategy_type: One of (key_auth | openid_connect) required
   configs: object required
     key-auth: # if strategy_type: key_auth
       key_names: array[string] required (1-10 items)
       ttl: object
         value: integer required (minimum: 1)
         unit: One of (days | weeks | years) required
     openid-connect: # if strategy_type: openid_connect
       issuer: string (url, max 256 chars) required
       credential_claim: array[string] required (max 10 items)
       scopes: array[string] required (max 50 items)
       auth_methods: array[string] required (max 10 items)
   dcr_provider_id: string (uuid, nullable; openid_connect only) # prefer: !ref <dcr-provider-ref>
   labels: object [string]string
     key: value
```

## DCR Providers

[API Specification](https://developer.konghq.com/api/konnect/application-auth-strategies/v2/#/operations/create-dcr-provider)

```yaml
dcr_providers:
 - ref: string
   name: string required
   display_name: string
   provider_type: One of (auth0 | azureAd | curity | okta | http) required
   issuer: string (url, max 256 chars) required
   dcr_config: object required
   labels: object [string]string
     key: value
```

## Catalog Services

[API Specification](https://developer.konghq.com/api/konnect/service-catalog/v1/#/operations/create-catalog-service)
[Example](examples/declarative/catalog/service.yaml)

```yaml
catalog_services:
 - ref: string
   name: string required (1-120 chars, pattern: ^[0-9a-z.-]+$)
   display_name: string required (1-120 chars)
   description: string (max 2048 chars)
   labels: object [string]string
     key: value
   custom_fields: object
     key: value
```

## Dashboards

[Custom Dashboards](https://developer.konghq.com/custom-dashboards/)
[Example](examples/declarative/analytics/dashboards/dashboard.yaml)

Dashboard names do not need to be unique in Konnect, but `kongctl` follows the
same resource matching pattern used elsewhere in declarative configuration.
When planning against live state, it considers dashboards with the matching
`KONGCTL-namespace` label and matches the desired dashboard by name. Avoid
duplicate dashboard names within a kongctl namespace.

Dashboard resources are declared under the `analytics` grouping key.

For dashboards created in the Konnect UI, first run
`kongctl adopt analytics dashboard` with the dashboard ID to apply the
namespace label, then run `kongctl dump declarative` with
`--resources=analytics.dashboards` and `--default-namespace <name>` to generate
declarative configuration. Name-based adoption fails if the name matches
multiple dashboards.

Use the dashboard definition JSON exported from Konnect as the `definition`
value. The field accepts that API-shaped object either inline or loaded from a
JSON/YAML file with `!file`; `kongctl` sends the parsed object as the dashboard
definition without translating it to another schema. `!file` is preferred for
larger dashboard definitions.

```yaml
analytics:
  dashboards:
    - ref: string
      name: string required
      definition: object required # prefer: !file ./definitions/dashboard.json
        tiles: array[object] required
        preset_filters: array[object]
      labels: object [string]string
        key: value
```

When the exported JSON includes the full API response, use `#definition` to
extract the payload expected by the dashboard API:

```yaml
analytics:
  dashboards:
    - ref: traffic-summary
      name: Traffic Summary
      definition: !file ./exports/traffic-summary.json#definition
```

## Control Planes

[API Specification](https://developer.konghq.com/api/konnect/control-planes/v2/#/operations/create-control-plane)
[Example](examples/declarative/control-plane/control-plane.yaml)

```yaml
control_planes:
 - ref: string
   name: string required
   description: string
   cluster_type: >-
     One of (CLUSTER_TYPE_CONTROL_PLANE |
     CLUSTER_TYPE_K8S_INGRESS_CONTROLLER |
     CLUSTER_TYPE_CONTROL_PLANE_GROUP |
     CLUSTER_TYPE_SERVERLESS |
     CLUSTER_TYPE_SERVERLESS_V1)
   auth_type: One of (pinned_client_certs | pki_client_certs)
   cloud_gateway: boolean
   proxy_urls: array[object]
     - host: string required
       port: integer required
       protocol: string required
   labels: object [string]string
     key: value
   _deck:
     files: array[string]
     flags: array[string]
   _external:
     selector:
       matchFields:
         name: string
     requires:
       deck: boolean
   gateway_services:
     - ref: string
       # _external only, Kong Gateway resources are managed by deck
       _external:
         selector:
           matchFields:
             name: string
   # API: create-dataplane-certificate
   data_plane_certificates:
     - ref: string
       cert: string required # prefer: !file ./certs/data-plane.pem
```

Control plane data plane certificates can also be declared as root resources.
The certificate contents identify a certificate within its control plane when
a certificate ID is not available. The `cert` field supports `!file` and
`!env`.

```yaml
control_plane_data_plane_certificates:
 - ref: string
   control_plane: string required # control plane ref
   cert: string required # prefer: !file ./certs/data-plane.pem
```

## AI Gateways

This section covers the root AI Gateway resource backed by the Konnect
`/v1/ai-gateways` API, AI Gateway Providers, AI Gateway Models, and AI Gateway
MCP Servers. Use `kongctl explain ai_gateway --output yaml`,
`kongctl explain ai_gateway_provider --output yaml`,
`kongctl explain ai_gateway.models --output yaml`, and
`kongctl explain ai_gateway.mcp_servers --output yaml` as the authoritative
schemas.

The `ref` value is used as the stable Konnect API `name` when creating an AI
Gateway. Use `display_name` for the human-readable name shown in Konnect.
AI Gateway Providers, Policies, Models, and MCP Servers use their own required
`name` field as the stable Konnect child name. Child entries inherit
management scope from their parent AI Gateway and do not accept `kongctl`
metadata.

For AI Gateway Models, `target_models[].provider` must match an AI Gateway
Provider `name` under the parent gateway. The provider can already exist or be
declared in the same gateway configuration.

For AI Gateway Models and MCP Servers, `policies` entries refer to AI Gateway
Policy names under the same parent gateway. The policy can already exist or be
declared in the same gateway configuration.

For AI Gateway Policies and MCP Servers, root-level declarations must include
`ai_gateway`, while nested declarations inherit the parent gateway. Omit
`policies` or `mcp_servers` to leave existing child resources unmanaged during
sync. Use `policies: []` or `mcp_servers: []` under a specific AI Gateway to
sync-delete that child type for that gateway. Root-level
`ai_gateway_policies: []` and `ai_gateway_mcp_servers: []` are rejected because
they do not identify a parent gateway.

```yaml
ai_gateways:
 - ref: string
   display_name: string required
   description: string
   proxy_urls: array[object]
     - host: string required
       port: integer required
       protocol: string required
   labels: object [string]string
     key: value
   kongctl:
     namespace: string
     protected: boolean
   providers:
     - ref: string
       name: string required
       type: string required
       display_name: string required
       config: object required
       labels: object [string]string
         key: value
       managed_by: object [string]string
         key: value
   policies:
     - ref: string
       name: string required
       type: string required
       display_name: string required
       enabled: boolean
       global: boolean
       config: object required
       labels: object [string]string
         key: value
       managed_by: object [string]string
         key: value
   models:
    - ref: string
      type: model # or api
      name: string required
      display_name: string required
      enabled: boolean
      config:
        route: object required
        model: object required
      formats:
       - type: string required
      target_models:
       - name: string required
         provider: string required # provider name in parent AI Gateway
         config:
           type: string required
      policies: array[object]
      capabilities: array[string]
      labels: object [string]string
        key: value
      acls: object
      managed_by: object
   mcp_servers:
    - ref: string
      type: conversion-only # or conversion-listener, listener,
                            # passthrough-listener, upstream-server
      name: string required
      display_name: string required
      enabled: boolean
      config:
        url: string required
      tools:
       - name: string required
         description: string required
         method: string required
         path: string
      policies: array[string]
      labels: object [string]string
        key: value
      acls: object
      managed_by: object
```

AI Gateway Providers can also be declared as root resources. Root-level
provider declarations must identify the parent AI Gateway with `ai_gateway`.

```yaml
ai_gateway_providers:
 - ref: string
   ai_gateway: string required # AI Gateway ref
   name: string required
   type: string required
   display_name: string required
   config: object required
   labels: object [string]string
     key: value
   managed_by: object [string]string
     key: value
```

AI Gateway Policies can also be declared as root resources. Root-level policy
declarations must identify the parent AI Gateway with `ai_gateway`.

```yaml
ai_gateway_policies:
 - ref: string
   ai_gateway: string required # AI Gateway ref
   name: string required
   type: string required
   display_name: string required
   enabled: boolean
   global: boolean
   config: object required
   labels: object [string]string
     key: value
   managed_by: object [string]string
     key: value
```

AI Gateway Models can also be declared as root resources. Include
`ai_gateway` to point at the parent gateway `ref`.

```yaml
ai_gateway_models:
 - ref: string
   ai_gateway: string required # AI Gateway ref
   type: model # or api
   name: string required
   display_name: string required
   enabled: boolean
   config:
     route: object required
     model: object required
   formats:
    - type: string required
   target_models:
    - name: string required
      provider: string required # provider name in parent AI Gateway
      config:
        type: string required
   policies: array[object]
   capabilities: array[string]
   labels: object [string]string
     key: value
   acls: object
   managed_by: object
```

AI Gateway MCP Servers can also be declared as root resources. Include
`ai_gateway` to point at the parent gateway `ref`.

```yaml
ai_gateway_mcp_servers:
 - ref: string
   ai_gateway: string required # AI Gateway ref
   type: conversion-only # or conversion-listener, listener,
                         # passthrough-listener, upstream-server
   name: string required
   display_name: string required
   enabled: boolean
   config:
     url: string required
   tools:
    - name: string required
      description: string required
      method: string required
      path: string
   policies: array[string]
   labels: object [string]string
     key: value
   acls: object
   managed_by: object
```

## Event Gateways

[API Specification](https://developer.konghq.com/api/konnect/event-gateway/v1/#/operations/create-event-gateway)
[Example](examples/declarative/event-gateway/event-gateway.yaml)

This section is an overview of the Event Gateway resources supported by
`kongctl`. Use `kongctl explain event_gateway --output yaml` as the
authoritative schema for nested Event Gateway resources and fields, and use
`kongctl scaffold event_gateway` to generate starter YAML.

```yaml
event_gateways:
 - ref: string
   name: string required (1-255 chars)
   description: string (max 512 chars)
   min_runtime_version: string (pattern: ^\d+\.\d+$)
   labels: object [string]string
     key: value
   backend_clusters: # https://developer.konghq.com/api/konnect/event-gateway/v1/#/operations/create-event-gateway-backend-cluster
     - ref: string
       name: string required (1-255 chars)
       description: string (max 512 chars)
       authentication: object required
         type: One of (anonymous | sasl_plain | sasl_scram) required
         username: string # required for sasl_plain/sasl_scram
         password: string # required for sasl_plain/sasl_scram
         algorithm: One of (sha256 | sha512) # required for sasl_scram
       insecure_allow_anonymous_virtual_cluster_auth: boolean
       bootstrap_servers: array[string] required (address:port)
       tls: object required
         enabled: boolean required
         insecure_skip_verify: boolean
         ca_bundle: string
         tls_versions: array[One of (tls12 | tls13)]
       metadata_update_interval_seconds: integer (1-43200)
       labels: object [string]string
         key: value
   virtual_clusters: # https://developer.konghq.com/api/konnect/event-gateway/v1/#/operations/create-event-gateway-virtual-cluster
     - ref: string
       name: string required (1-255 chars)
       description: string (max 512 chars)
       destination: object required
         id: string (uuid) # oneOf; declarative: prefer !ref <backend-cluster-ref>
         name: string # oneOf
       authentication: array[object] required (min 1 item)
         - type: One of (anonymous | sasl_plain | sasl_scram | oauth_bearer) required
           mediation: string # required for sasl_plain/oauth_bearer
           principals: array[object] # for sasl_plain terminate mode
           algorithm: One of (sha256 | sha512) # for sasl_scram
           claims_mapping: object # for oauth_bearer
           jwks: object # for oauth_bearer
           validate: object # for oauth_bearer
       namespace:
         mode: One of (hide_prefix | enforce_prefix) required
         prefix: string required
         additional:
           topics: array[object]
           consumer_groups: array[object]
       acl_mode: One of (enforce_on_gateway | passthrough) required
       dns_label: string required (1-63 chars, RFC1035 label)
       labels: object [string]string
         key: value
   listeners: # https://developer.konghq.com/api/konnect/event-gateway/v1/#/operations/create-event-gateway-listener
     - ref: string
       name: string required (1-255 chars)
       description: string (max 512 chars)
       addresses: array[string] required (min 1 item)
       ports: array[integer|string] required (min 1 item)
       labels: object [string]string
         key: value
       policies: # https://developer.konghq.com/api/konnect/event-gateway/v1/#/operations/create-event-gateway-listener-policy
         - ref: string
           type: One of (tls_server | forward_to_virtual_cluster) required
           name: string
           description: string
           enabled: boolean
           labels: object [string]string
             key: value
           config: object required
             certificates: # if type=tls_server
               - certificate: string required
                 key: string required
             versions: # if type=tls_server
               min: One of (TLSv1.2 | TLSv1.3)
               max: One of (TLSv1.2 | TLSv1.3)
             allow_plaintext: boolean # if type=tls_server
             type: One of (sni | port_mapping) # if type=forward_to_virtual_cluster
             sni_suffix: string # if config.type=sni
             advertised_port: integer # if config.type=sni
             broker_host_format:
               type: One of (per_cluster_suffix | shared_suffix) # if config.type=sni
             destination:
               id: string (uuid) # if config.type=port_mapping; oneOf; declarative: prefer !ref <virtual-cluster-ref>
               name: string # if config.type=port_mapping; oneOf
             advertised_host: string # if config.type=port_mapping
             bootstrap_port: One of (none | at_start) # if config.type=port_mapping
             min_broker_id: integer # if config.type=port_mapping
```

Additional nested Event Gateway resources include `schema_registries`,
`static_keys`, `tls_trust_bundles`, `data_plane_certificates`,
`cluster_policies`, `produce_policies`, and `consume_policies`.

## Organization

[API Specification](https://developer.konghq.com/api/konnect/identity/v3/#/)
[Example](examples/declarative/organization/teams.yaml)

```yaml
organization:
 teams:
   # https://developer.konghq.com/api/konnect/identity/v3/#/operations/create-team
   - ref: string
     name: string required
     description: string (max 250 chars)
     labels: object [string]string
       key: value
     roles:
       - ref: string
         role_name: string
         # Prefer: !ref <api-ref> when entity_type_name=APIs.
         entity_id: string (uuid)
         entity_type_name: string
         entity_region: One of (us | eu | au | me | in | sg | *)
```

Organization team roles can also be declared as root resources.

```yaml
organization_team_roles:
  - ref: string
    # Declarative organization team ref, not team name or UUID.
    team: string required
    role_name: string
    # Prefer: !ref <api-ref> when entity_type_name=APIs.
    entity_id: string (uuid)
    entity_type_name: string
    entity_region: One of (us | eu | au | me | in | sg | *)
```

## Portals

[API Specification](https://developer.konghq.com/api/konnect/portal-management/v3/#/operations/create-portal)
[Example](examples/declarative/portal/portal.yaml)

```yaml
portals:
 - ref: string
   name: string required (1-255 chars)
   display_name: string (1-255 chars)
   description: string (max 512 chars, nullable)
   authentication_enabled: boolean (default: true)
   rbac_enabled: boolean (default: false)
   default_api_visibility: One of (public | private)
   default_page_visibility: One of (public | private)
   default_application_auth_strategy_id: string (uuid, nullable) # prefer: !ref <app-auth-strategy-ref>
   auto_approve_developers: boolean (default: false)
   auto_approve_applications: boolean (default: false)
   labels: object [string]string
     key: value
   customization: # https://developer.konghq.com/api/konnect/portal-management/v3/#/operations/replace-portal-customization
     ref: string
     theme:
       name: string
       mode: One of (light | dark | system)
       colors:
         primary: string (hex color, e.g. #0055A4)
     layout: string
     css: string (nullable)
     menu:
       main: array[PortalMenuItem]
       footer_sections: array[PortalFooterMenuSection]
       footer_bottom: array[PortalMenuItem]
     spec_renderer:
       try_it_ui: boolean
       try_it_insomnia: boolean
       infinite_scroll: boolean
       show_schemas: boolean
       hide_internal: boolean
       hide_deprecated: boolean
       allow_custom_server_urls: boolean
     robots: string (nullable)
   auth_settings: # https://developer.konghq.com/api/konnect/portal-management/v3/#/operations/update-portal-authentication-settings
     ref: string
     # OIDC and SAML provider-specific fields are no longer supported here.
     # Move provider config to identity_providers or portal_identity_providers.
     basic_auth_enabled: boolean
     konnect_mapping_enabled: boolean
     idp_mapping_enabled: boolean
   ip_allow_list: # https://developer.konghq.com/api/konnect/portal-management/v3/#/operations/create-portal-ip-allow-list
     ref: string
     allowed_ips: array[string] required # IP addresses or CIDR blocks
   integrations: # https://developer.konghq.com/api/konnect/portal-management/v3/#/operations/upsert-portal-integrations
     ref: string
     google_tag_manager:
       enabled: boolean required
       type: tracking
       config_data:
         id: string required (pattern: ^GTM-[A-Za-z0-9]+$)
         l: string (nullable)
         preview: string (nullable)
         cookies_win: boolean (nullable)
         debug: boolean (nullable)
         npa: boolean (nullable)
         data_layer: string (nullable)
         env_name: string (nullable)
         auth_referrer_policy: string (nullable)
     google_analytics_4:
       enabled: boolean required
       type: analytics
       config_data:
         id: string required (pattern: ^G-[A-Za-z0-9-]+$)
         l: string (nullable)
   identity_providers: # https://developer.konghq.com/api/konnect/portal-management/v3/#/operations/create-portal-identity-provider
     - ref: string
       # Use this child for portal OIDC and SAML provider configuration.
       # At the root of a config, use portal_identity_providers.
       type: One of (oidc | saml) required
       enabled: boolean
       config: object required
         issuer_url: string # OIDC
         client_id: string # OIDC
         client_secret: string # OIDC
         scopes: array[string] # OIDC
         claim_mappings: # OIDC
           name: string
           email: string
           groups: string
         idp_metadata_url: string # SAML
         idp_metadata_xml: string # SAML
   custom_domain: # https://developer.konghq.com/api/konnect/portal-management/v3/#/operations/create-portal-custom-domain
     ref: string
     hostname: string required
     enabled: boolean required
     ssl: object required
       domain_verification_method: One of (http | custom_certificate) required
       custom_certificate: string # when domain_verification_method=custom_certificate
       custom_private_key: string # when domain_verification_method=custom_certificate
       skip_ca_check: boolean
   pages: # https://developer.konghq.com/api/konnect/portal-management/v3/#/operations/create-portal-page
     - ref: string
       slug: string required (max 512 chars)
       content: string required (markdown) # prefer: !file ./docs/page.md
       title: string (max 512 chars)
       visibility: One of (public | private)
       status: One of (published | unpublished)
       description: string (max 160 chars)
       parent_page_id: string (uuid, nullable) # prefer: !ref <page-ref>
       children:
         - ref: string
           slug: string required (max 512 chars)
           content: string required (markdown) # prefer: !file ./docs/page.md
           title: string (max 512 chars)
           visibility: One of (public | private)
           status: One of (published | unpublished)
           description: string (max 160 chars)
           parent_page_id: string (uuid, nullable) # prefer: !ref <page-ref>
   snippets: # https://developer.konghq.com/api/konnect/portal-management/v3/#/operations/create-portal-snippet
     - ref: string
       name: string required (max 512 chars)
       content: string required (markdown) # prefer: !file ./docs/snippet.md
       title: string (max 512 chars)
       visibility: One of (public | private)
       status: One of (published | unpublished)
       description: string (max 160 chars)
   teams: # https://developer.konghq.com/api/konnect/portal-management/v3/#/operations/create-portal-team
     - ref: string
       name: string required
       description: string (max 250 chars)
       roles: # https://developer.konghq.com/api/konnect/portal-management/v3/#/operations/assign-role-to-portal-teams
         - ref: string
           role_name: string
           entity_id: string (uuid)
           entity_type_name: string
           entity_region: One of (us | eu | au | me | in | sg | *)
   email_config: # https://developer.konghq.com/api/konnect/portal-management/v3/#/operations/create-portal-email-config
     ref: string
     domain_name: string (nullable)
     from_name: string (nullable)
     from_email: string (email, nullable)
     reply_to_email: string (email, nullable)
   audit_log_webhook: # https://developer.konghq.com/api/konnect/portal-management/v3/#/operations/update-portal-audit-log-webhook
     ref: string
     enabled: boolean
     audit_log_destination_id: string (uuid) # prefer: !ref
   email_templates: # https://developer.konghq.com/api/konnect/portal-management/v3/#/operations/update-portal-custom-email-template
     <template_name>:
       ref: string
       name: string
       enabled: boolean
       content:
         subject: string (max 1024 chars, nullable)
         title: string (max 1024 chars, nullable)
         body: string (max 4096 chars, nullable)
         button_label: string (max 128 chars, nullable)
   assets:
     logo: string # data URL image (png/jpeg/gif/ico/svg)
     favicon: string # data URL image (png/jpeg/gif/ico/svg)
```

Portal identity providers and integrations can also be declared as root
resources.

```yaml
portal_identity_providers:
 - ref: string
   portal: string required # prefer: !ref <portal-ref>
   type: One of (oidc | saml) required
   enabled: boolean
   config: object required
     issuer_url: string # OIDC
     client_id: string # OIDC
     client_secret: string # OIDC
     scopes: array[string] # OIDC
     claim_mappings: # OIDC
       name: string
       email: string
       groups: string
     idp_metadata_url: string # SAML
     idp_metadata_xml: string # SAML

portal_integrations:
 - ref: string
   portal: string required # prefer: !ref <portal-ref>
   google_tag_manager:
     enabled: boolean required
     type: tracking
     config_data:
       id: string required (pattern: ^GTM-[A-Za-z0-9]+$)
       l: string (nullable)
       preview: string (nullable)
       cookies_win: boolean (nullable)
       debug: boolean (nullable)
       npa: boolean (nullable)
       data_layer: string (nullable)
       env_name: string (nullable)
       auth_referrer_policy: string (nullable)
   google_analytics_4:
     enabled: boolean required
     type: analytics
     config_data:
       id: string required (pattern: ^G-[A-Za-z0-9-]+$)
       l: string (nullable)
```

Portal IP allow lists can also be declared as root resources.

```yaml
portal_ip_allow_lists:
  - ref: string
    portal: string required # prefer: !ref <portal-ref>
    allowed_ips: array[string] required # IP addresses or CIDR blocks
```

In sync mode, omitted `ip_allow_list` configuration is ignored. Include the
`ip_allow_list` block when the portal IP allow list is owned by the config.

Portal audit-log webhooks can also be declared as root resources.

```yaml
portal_audit_log_webhooks:
  - ref: string
    portal: string required # prefer: !ref <portal-ref>
    enabled: boolean
    audit_log_destination_id: string (uuid) # prefer: !ref
```

In sync mode, omitted `audit_log_webhook` configuration is ignored. Include the
`audit_log_webhook` block when the portal webhook is owned by the config.

```yaml
portals:
  - ref: docs-portal
    name: Docs Portal
    audit_log_webhook:
      ref: docs-portal-audit-log-webhook
      enabled: true
      audit_log_destination_id: !ref foo

audit-logs:
  destinations:
    - ref: foo
      _external:
        selector:
          matchFields:
            name: foo
```
