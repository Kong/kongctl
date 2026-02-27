# `kongctl` Declarative Resource Reference

This document is the reference for `kongctl` declarative configuration. It
lists supported resource types and their field-level values.
Resource configurations are provided as YAML files and can be expressed as one or more
files passed to `kongctl` declarative commands. 

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
   spec_content: string (OpenAPI or AsyncAPI content; json or yaml)
   versions: # https://developer.konghq.com/api/konnect/api-builder/v3/#/operations/create-api-version
     - ref: string
       version: string
       spec: object required
         content: string required (OpenAPI or AsyncAPI content; json or yaml)
   publications: # https://developer.konghq.com/api/konnect/api-builder/v3/#/operations/publish-api-to-portal
     - ref: string
       portal_id: string required (uuid)
       auto_approve_registrations: boolean
       auth_strategy_ids: array[string(uuid)] (nullable, max 1 item)
       visibility: One of (public | private)
   implementations: # https://developer.konghq.com/api/konnect/api-builder/v3/#/operations/create-api-implementation
     - ref: string
       service: # oneOf variant
         id: string required (uuid)
         control_plane_id: string required (uuid)
       control_plane: # oneOf variant
         control_plane_id: string required (uuid)
   documents: # https://developer.konghq.com/api/konnect/api-builder/v3/#/operations/create-api-document
     - ref: string
       content: string required (markdown)
       title: string
       slug: string (pattern: ^[\w-]+$)
       status: One of (published | unpublished)
       parent_document_id: string (uuid, nullable)
       children:
         - ref: string
           content: string required (markdown)
           title: string
           slug: string (pattern: ^[\w-]+$)
           status: One of (published | unpublished)
```

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
   dcr_provider_id: string (uuid, nullable; openid_connect only)
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

## Control Planes

[API Specification](https://developer.konghq.com/api/konnect/control-planes/v2/#/operations/create-control-plane)
[Example](examples/declarative/control-plane/control-plane.yaml)

```yaml
control_planes:
 - ref: string
   name: string required
   description: string
   cluster_type: One of (CLUSTER_TYPE_CONTROL_PLANE | CLUSTER_TYPE_K8S_INGRESS_CONTROLLER | CLUSTER_TYPE_CONTROL_PLANE_GROUP | CLUSTER_TYPE_SERVERLESS)
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
```

## Event Gateways

[API Specification](https://developer.konghq.com/api/konnect/event-gateway/v1/#/operations/create-event-gateway)
Example: Not published yet.

```yaml
event_gateways:
 # NOTE: This section documents only the Event Gateway resources currently supported by kongctl.
 # Not all resources in the Event Gateway API are supported yet.
 - ref: string
   name: string required (1-255 chars)
   description: string (max 512 chars)
   min_runtime_version: string (pattern: ^\\d+\\.\\d+$)
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
         id: string (uuid) # oneOf
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
               id: string (uuid) # if config.type=port_mapping; oneOf
               name: string # if config.type=port_mapping; oneOf
             advertised_host: string # if config.type=port_mapping
             bootstrap_port: One of (none | at_start) # if config.type=port_mapping
             min_broker_id: integer # if config.type=port_mapping
```

## Organization

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
   default_application_auth_strategy_id: string (uuid, nullable)
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
     basic_auth_enabled: boolean
     oidc_auth_enabled: boolean (deprecated)
     saml_auth_enabled: boolean (deprecated)
     oidc_team_mapping_enabled: boolean
     konnect_mapping_enabled: boolean
     idp_mapping_enabled: boolean
     oidc_issuer: string (deprecated)
     oidc_client_id: string (deprecated)
     oidc_client_secret: string (deprecated)
     oidc_scopes: array[string] (deprecated)
     oidc_claim_mappings:
       name: string (deprecated)
       email: string (deprecated)
       groups: string (deprecated)
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
       content: string required (markdown)
       title: string (max 512 chars)
       visibility: One of (public | private)
       status: One of (published | unpublished)
       description: string (max 160 chars)
       parent_page_id: string (uuid, nullable)
       children:
         - ref: string
           slug: string required (max 512 chars)
           content: string required (markdown)
           title: string (max 512 chars)
           visibility: One of (public | private)
           status: One of (published | unpublished)
           description: string (max 160 chars)
           parent_page_id: string (uuid, nullable)
   snippets: # https://developer.konghq.com/api/konnect/portal-management/v3/#/operations/create-portal-snippet
     - ref: string
       name: string required (max 512 chars)
       content: string required (markdown)
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
