# Kong Konnect API Detail Listing

**Source**: https://developer.konghq.com/api/

**Date Generated**: 2025-11-07

This document provides detailed endpoint information for each Konnect API.

---

## Core Platform APIs

### [Identity Management (v3)](https://developer.konghq.com/api/konnect/identity/v3/)

Users, teams, organizations, and permissions

**Base URL**: `https://global.api.konghq.com/v3`

**Endpoints**:
- `/organizations/impersonation` - GET, PATCH
- `/authentication-settings` - GET, PATCH
- `/invites` - POST
- `/identity-providers` - GET, POST
- `/identity-providers/{id}` - GET, PATCH, DELETE
- `/identity-provider` - GET, PATCH
- `/identity-provider/team-mappings` - PUT, GET
- `/identity-provider/team-group-mappings` - GET, PATCH
- `/roles` - GET
- `/teams` - GET, POST
- `/teams/{teamId}/users` - GET, POST
- `/teams/{teamId}` - GET, PATCH, DELETE
- `/teams/{teamId}/users/{userId}` - DELETE
- `/teams/{teamId}/assigned-roles` - GET, POST
- `/teams/{teamId}/assigned-roles/{roleId}` - DELETE
- `/users` - GET
- `/users/{userId}` - GET, PATCH, DELETE
- `/users/{userId}/teams` - GET
- `/users/{userId}/assigned-roles` - GET, POST
- `/users/{userId}/assigned-roles/{roleId}` - DELETE
- `/system-accounts` - GET, POST
- `/system-accounts/{accountId}` - GET, PATCH, DELETE
- `/system-accounts/{accountId}/access-tokens` - GET, POST
- `/system-accounts/{accountId}/access-tokens/{tokenId}` - GET, PATCH, DELETE
- `/system-accounts/{accountId}/assigned-roles` - GET, POST
- `/system-accounts/{accountId}/assigned-roles/{roleId}` - DELETE
- `/teams/{teamId}/system-accounts` - GET, POST
- `/teams/{teamId}/system-accounts/{accountId}` - DELETE
- `/system-accounts/{accountId}/teams` - GET
- `/users/me` - GET
- `/organizations/me` - GET
- `/authenticate/{organizationLoginPath}` - GET

### [Organization Management (v3)](https://developer.konghq.com/api/konnect/organizations/v3/)

Organizational-level settings

**Base URL**: `https://us.api.konghq.com/v3`

**Endpoints**:
- `/organizations/{organizationId}/personal-access-tokens` - GET
- `/organizations/{organizationId}/personal-access-token-settings` - GET, PATCH

### [Audit Logging (v2)](https://developer.konghq.com/api/konnect/audit-logs/v2/)

Security monitoring and compliance

**Base URL**: `https://us.api.konghq.com/v2`

**Endpoints**:
- `/audit-log-replay-job` - PUT, GET
- `/audit-log-webhook` - PATCH, GET
- `/audit-log-webhook/status` - GET
- `/audit-log-webhook/jwks.json` - GET
- `/audit-log-destinations` - GET, POST
- `/audit-log-destinations/{auditLogDestinationId}` - GET, PATCH, DELETE

### [Notification Hub (v1)](https://developer.konghq.com/api/konnect/notification-hub/v1/)

Notification management

**Base URL**: `https://global.api.konghq.com/v1`

**Endpoints**:
- `/notifications/inbox` - GET
- `/notifications/inbox/{notificationId}` - GET, PATCH, DELETE
- `/notifications/inbox/bulk` - POST
- `/notifications/configurations` - GET
- `/notifications/configurations/{eventId}/subscriptions` - GET, POST
- `/notifications/configurations/{eventId}/subscriptions/{subscriptionId}` - GET, PATCH, DELETE

---

## Developer Portal & API Management

### [Portal Management (v3)](https://developer.konghq.com/api/konnect/portal-management/v3/)

AIP-compliant portal automation

**Base URL**: `https://us.api.konghq.com/v3`

**Endpoints**:
- `/portals` - GET, POST
- `/portals/{portalId}` - GET, PATCH, DELETE
- `/portals/{portalId}/custom-domain` - GET, POST, PATCH, DELETE
- `/portals/{portalId}/assets/logo` - GET, PUT
- `/portals/{portalId}/assets/logo/raw` - GET
- `/portals/{portalId}/assets/favicon` - GET, PUT
- `/portals/{portalId}/assets/favicon/raw` - GET
- `/portals/{portalId}/customization` - GET, PUT, PATCH
- `/portals/{portalId}/pages` - GET, POST
- `/portals/{portalId}/pages/{pageId}` - GET, PATCH, DELETE
- `/portals/{portalId}/pages/{pageId}/move` - POST
- `/portals/{portalId}/snippets` - GET, POST
- `/portals/{portalId}/snippets/{snippetId}` - GET, PATCH, DELETE
- `/portals/{portalId}/default-content` - POST
- `/portals/{portalId}/applications` - GET
- `/portals/{portalId}/applications/{applicationId}` - GET, DELETE
- `/portals/{portalId}/applications/{applicationId}/developers` - GET
- `/applications/{applicationId}` - GET
- `/portals/{portalId}/application-registrations` - GET
- `/portals/{portalId}/applications/{applicationId}/registrations` - GET
- `/portals/{portalId}/applications/{applicationId}/registrations/{registrationId}` - GET, PATCH, DELETE
- `/portals/{portalId}/authentication-settings` - GET, PATCH
- `/portals/{portalId}/identity-provider/team-group-mappings` - GET, PATCH
- `/portals/{portalId}/teams` - GET, POST
- `/portals/{portalId}/teams/{teamId}` - GET, PATCH, DELETE
- `/portals/{portalId}/teams/{teamId}/developers` - GET, POST
- `/portals/{portalId}/teams/{teamId}/developers/{developerId}` - DELETE
- `/portals/{portalId}/teams/{teamId}/assigned-roles` - GET, POST
- `/portals/{portalId}/teams/{teamId}/assigned-roles/{roleId}` - DELETE
- `/portals/{portalId}/developers` - GET
- `/portals/{portalId}/developers/{developerId}` - GET, PATCH, DELETE
- `/portals/{portalId}/developers/{developerId}/teams` - GET
- `/portal-roles` - GET
- `/portals/{portalId}/identity-providers` - GET, POST
- `/portals/{portalId}/identity-providers/{id}` - GET, PATCH, DELETE
- `/portals/{portalId}/audit-log-replay-job` - PUT, GET
- `/portals/{portalId}/audit-log-webhook` - PATCH, GET
- `/portals/{portalId}/audit-log-webhook/status` - GET
- `/portals/email-domains` - GET, POST
- `/portals/email-domains/{emailDomain}` - GET, DELETE
- `/portals/{portalId}/email-delivery` - GET, PATCH, DELETE
- `/portals/{portalId}/email-config` - GET, POST, PATCH, DELETE
- `/portals/email-templates` - GET
- `/portals/email-templates/variables` - GET
- `/portals/email-templates/{templateName}` - GET
- `/portals/{portalId}/email-templates` - GET
- `/portals/{portalId}/email-templates/{templateName}` - GET, PATCH, DELETE
- `/portals/{portalId}/email-templates/{templateName}/send-test-email` - POST

### [Developer Portal (v3)](https://developer.konghq.com/api/konnect/dev-portal/v3/)

Portal user data access

**Base URL**: `https://custom.example.com`

**Endpoints**:
- `/api/v3/api-attributes` - GET
- `/api/v3/apis` - GET
- `/api/v3/apis/{apiIdOrSlug}` - GET
- `/api/v3/apis/{apiIdOrSlug}/actions` - GET
- `/api/v3/apis/{apiIdOrSlug}/applications` - GET
- `/api/v3/apis/{apiIdOrSlug}/documents` - GET
- `/api/v3/apis/{apiIdOrSlug}/documents/{documentIdOrSlug}` - GET
- `/api/v3/apis/{apiIdOrSlug}/specifications` - GET
- `/api/v3/apis/{apiIdOrSlug}/specifications/{specId}` - GET
- `/api/v3/apis/{apiIdOrSlug}/specifications/{specId}/raw` - GET
- `/api/v3/apis/{apiIdOrSlug}/versions` - GET
- `/api/v3/apis/{apiIdOrSlug}/versions/{versionId}` - GET
- `/api/v3/apis/{apiIdOrSlug}/versions/{versionId}/raw` - GET
- `/api/v3/application-auth-strategies` - GET
- `/api/v3/applications` - GET, POST
- `/api/v3/applications/{applicationId}` - GET, DELETE, PATCH
- `/api/v3/applications/{applicationId}/credentials` - GET, POST
- `/api/v3/applications/{applicationId}/credentials/{credentialId}` - PUT, DELETE
- `/api/v3/applications/{applicationId}/granted-scopes` - GET
- `/api/v3/applications/{applicationId}/regenerate-secret` - POST
- `/api/v3/applications/{applicationId}/registrations` - GET, POST
- `/api/v3/applications/{applicationId}/registrations/{registrationId}` - GET, DELETE
- `/api/v3/assets/favicon` - GET
- `/api/v3/assets/logo` - GET
- `/api/v3/customization` - GET
- `/api/v3/developer` - POST
- `/api/v3/developer/authenticate` - POST
- `/api/v3/developer/authenticate/sso` - GET
- `/api/v3/developer/forgot-password` - POST
- `/api/v3/developer/logout` - POST
- `/api/v3/developer/me` - GET
- `/api/v3/developer/refresh` - POST
- `/api/v3/developer/reset-password` - POST
- `/api/v3/developer/verify-email` - POST
- `/api/v3/pages` - GET
- `/api/v3/pages/{pagePath}` - GET
- `/api/v3/portal` - GET
- `/api/v3/search` - GET
- `/api/v3/snippets` - GET
- `/api/v3/snippets/{snippetName}` - GET
- `/api/v3/stats` - POST
- `/api/v3/stats/config` - GET

### [API Builder (v3)](https://developer.konghq.com/api/konnect/api-builder/v3/)

Managing APIs for Developer Portal (beta)

**Base URL**: `https://us.api.konghq.com/v3`

**Endpoints**:
- `/apis` - POST, GET
- `/apis/validate-specification` - POST
- `/apis/{apiId}` - GET, PATCH, DELETE
- `/apis/_computed` - GET
- `/apis/{apiId}/documents` - POST, GET
- `/apis/{apiId}/documents/{documentId}` - GET, PATCH, DELETE
- `/apis/{apiId}/documents/{documentId}/move` - POST
- `/apis/{apiId}/specifications` - POST, GET
- `/apis/{apiId}/specifications/{specId}` - GET, PATCH, DELETE
- `/apis/{apiId}/versions` - POST, GET
- `/apis/{apiId}/versions/{specId}` - GET, PATCH, DELETE
- `/apis/{apiId}/publications/{portalId}` - PUT, GET, DELETE
- `/api-publications` - GET
- `/apis/{apiId}/implementations` - POST
- `/apis/{apiId}/implementations/{implementationId}` - GET, DELETE
- `/api-implementations` - GET
- `/api-attributes` - GET

### [Application Auth Strategies (v2)](https://developer.konghq.com/api/konnect/application-auth-strategies/v2/)

Authentication for APIs

**Base URL**: `https://us.api.konghq.com/v2`

**Endpoints**:
- `/dcr-providers` - POST, GET
- `/dcr-providers/{dcrProviderId}` - GET, PATCH, DELETE
- `/dcr-providers/{dcrProviderId}/verify` - POST
- `/application-auth-strategies` - POST, GET
- `/application-auth-strategies/{authStrategyId}` - GET, PUT, PATCH, DELETE

### [Service Catalog (v1)](https://developer.konghq.com/api/konnect/service-catalog/v1/)

Service catalog

**Base URL**: `https://us.api.konghq.com/v1`

**Endpoints**:
- `/catalog-services` - POST, GET
- `/catalog-services/{id}` - GET, PATCH, DELETE
- `/integrations` - GET
- `/integration-instances` - POST, GET
- `/integration-instances/{id}` - GET, PATCH, DELETE
- `/integration-instances/{id}/auth-credential` - POST, GET, DELETE
- `/integration-instances/{id}/auth-config` - GET, PUT, DELETE
- `/resources` - GET
- `/resources/{id}` - GET
- `/integration-instances/{integrationInstanceId}/resources/{resourceId}` - PATCH
- `/resources/{id}/catalog-services` - GET
- `/resource-mappings` - POST, GET
- `/resource-mappings/{id}` - GET, DELETE
- `/catalog-services/{id}/resources` - GET

---

## Runtime Management

### [Control Planes (v2)](https://developer.konghq.com/api/konnect/control-planes/v2/)

Control plane management

**Base URL**: `https://us.api.konghq.com/v2`

**Endpoints**:
- `/control-planes` - GET, POST
- `/control-planes/{id}` - GET, PATCH, DELETE
- `/control-planes/{id}/group-memberships` - GET, PUT
- `/control-planes/{id}/group-memberships/add` - POST
- `/control-planes/{id}/group-memberships/remove` - POST
- `/control-planes/{id}/group-member-status` - GET
- `/control-planes/{id}/group-status` - GET

### [Control Planes Config (v2)](https://developer.konghq.com/api/konnect/control-planes-config/v2/)

Services, routes, certificates

**Base URL**: `https://us.api.konghq.com/v2`

**Endpoints** (97 total endpoints for managing Kong Gateway core entities):
- `/control-planes/{controlPlaneId}/expected-config-hash` - GET
- `/control-planes/{controlPlaneId}/dp-client-certificates` - GET, POST
- `/control-planes/{controlPlaneId}/dp-client-certificates/{certificateId}` - GET, DELETE
- `/control-planes/{controlPlaneId}/nodes` - GET
- `/control-planes/{controlPlaneId}/nodes/eol` - GET
- `/control-planes/{controlPlaneId}/nodes/{nodeId}` - GET, DELETE
- `/control-planes/{controlPlaneId}/core-entities/plugin-schemas` - GET, POST
- `/control-planes/{controlPlaneId}/core-entities/plugin-schemas/{name}` - GET, DELETE, PUT
- `/control-planes/{controlPlaneId}/config-stores` - GET, POST
- `/control-planes/{controlPlaneId}/config-stores/{configStoreId}` - GET, PUT, DELETE
- `/control-planes/{controlPlaneId}/config-stores/{configStoreId}/secrets` - POST, GET
- `/control-planes/{controlPlaneId}/config-stores/{configStoreId}/secrets/{key}` - GET, PUT, DELETE
- Core entities: acls, basic-auths, ca_certificates, certificates, consumer_groups, consumers, custom-plugins, hmac-auths, jwts, key-auths, key-sets, keys, mtls-auths, partials, plugins, routes, services, snis, upstreams, vaults
- Full CRUD operations for all core entities with nested resources

### [Dedicated Cloud Gateways (v2)](https://developer.konghq.com/api/konnect/cloud-gateways/v2/)

Infrastructure configuration

**Base URL**: `https://global.api.konghq.com/v2`

**Endpoints**:
- `/cloud-gateways/availability.json` - GET
- `/cloud-gateways/configurations` - GET, PUT
- `/cloud-gateways/configurations/{configurationId}` - GET
- `/cloud-gateways/networks` - GET, POST
- `/cloud-gateways/networks/{networkId}` - GET, PATCH, DELETE
- `/cloud-gateways/networks/{networkId}/transit-gateways` - GET, POST
- `/cloud-gateways/networks/{networkId}/transit-gateways/{transitGatewayId}` - GET, PATCH, DELETE
- `/cloud-gateways/networks/{networkId}/private-dns` - GET, POST
- `/cloud-gateways/networks/{networkId}/private-dns/{privateDnsId}` - GET, PATCH, DELETE
- `/cloud-gateways/networks/{networkId}/configuration-references` - GET
- `/cloud-gateways/provider-accounts` - GET
- `/cloud-gateways/provider-accounts/{providerAccountId}` - GET
- `/cloud-gateways/custom-domains` - GET, POST
- `/cloud-gateways/custom-domains/{customDomainId}` - GET, DELETE
- `/cloud-gateways/custom-domains/{customDomainId}/online-status` - GET
- `/cloud-gateways/default-resource-quotas` - GET
- `/cloud-gateways/resource-quotas` - GET
- `/cloud-gateways/resource-quotas/{resourceQuotaId}` - GET
- `/cloud-gateways/default-resource-configurations` - GET
- `/cloud-gateways/resource-configurations` - GET
- `/cloud-gateways/resource-configurations/{resourceConfigurationId}` - GET

### [Event Gateway (v1)](https://developer.konghq.com/api/konnect/event-gateway/v1/)

Kafka protocol proxy

**Base URL**: `https://us.api.konghq.com/v1`

**Endpoints**:
- `/event-gateways` - GET, POST
- `/event-gateways/{gatewayId}` - GET, PUT, DELETE
- `/event-gateways/{gatewayId}/listeners` - GET, POST
- `/event-gateways/{gatewayId}/listeners/{eventGatewayListenerId}` - GET, PUT, DELETE
- `/event-gateways/{gatewayId}/virtual-clusters` - GET, POST
- `/event-gateways/{gatewayId}/virtual-clusters/{virtualClusterId}` - GET, PUT, DELETE
- `/event-gateways/{gatewayId}/backend-clusters` - GET, POST
- `/event-gateways/{gatewayId}/backend-clusters/{backendClusterId}` - GET, PUT, DELETE
- `/event-gateways/{gatewayId}/schema-registries` - GET, POST
- `/event-gateways/{gatewayId}/schema-registries/{schemaRegistryId}` - GET, PUT, DELETE
- `/event-gateways/{gatewayId}/nodes` - GET
- `/event-gateways/{gatewayId}/nodes/{eventGatewayNodeId}` - GET
- `/event-gateways/{gatewayId}/nodes/{eventGatewayNodeId}/status` - GET
- `/event-gateways/{gatewayId}/nodes/{eventGatewayNodeId}/errors` - GET
- `/event-gateways/{gatewayId}/listeners/{eventGatewayListenerId}/policies` - GET, POST
- `/event-gateways/{gatewayId}/listeners/{eventGatewayListenerId}/policies/{policyId}` - GET, PUT, PATCH, DELETE
- `/event-gateways/{gatewayId}/listeners/{eventGatewayListenerId}/policies/{policyId}/move` - POST
- `/event-gateways/{gatewayId}/virtual-clusters/{virtualClusterId}/consume-policies` - GET, POST
- `/event-gateways/{gatewayId}/virtual-clusters/{virtualClusterId}/consume-policies/{policyId}` - GET, PUT, PATCH, DELETE
- `/event-gateways/{gatewayId}/virtual-clusters/{virtualClusterId}/consume-policies/{policyId}/move` - POST
- `/event-gateways/{gatewayId}/virtual-clusters/{virtualClusterId}/produce-policies` - GET, POST
- `/event-gateways/{gatewayId}/virtual-clusters/{virtualClusterId}/produce-policies/{policyId}` - GET, PUT, PATCH, DELETE
- `/event-gateways/{gatewayId}/virtual-clusters/{virtualClusterId}/produce-policies/{policyId}/move` - POST
- `/event-gateways/{gatewayId}/virtual-clusters/{virtualClusterId}/cluster-policies` - GET, POST
- `/event-gateways/{gatewayId}/virtual-clusters/{virtualClusterId}/cluster-policies/{policyId}` - GET, PUT, PATCH, DELETE
- `/event-gateways/{gatewayId}/virtual-clusters/{virtualClusterId}/cluster-policies/{policyId}/move` - POST
- `/event-gateways/{gatewayId}/data-plane-certificates` - GET, POST
- `/event-gateways/{gatewayId}/data-plane-certificates/{certificateId}` - GET, PUT, DELETE
- `/event-gateways/{gatewayId}/static-keys` - GET, POST
- `/event-gateways/{gatewayId}/static-keys/{staticKeyId}` - GET, DELETE

### [Mesh Manager (v0)](https://developer.konghq.com/api/konnect/mesh-control-planes/v0/)

Global mesh control plane

**Base URL**: `https://us.api.konghq.com/v1`

**Endpoints** (55 total endpoints for mesh management):
- `/mesh/control-planes/{cpId}` - GET, DELETE, PATCH
- `/mesh/control-planes/{cpId}/_resources` - GET
- `/mesh/control-planes/{cpId}/global-insight` - GET
- `/mesh/control-planes/{cpId}/meshes/{mesh}/{resourceType}/{resourceName}/_rules` - GET
- `/mesh/control-planes/{cpId}/meshes/{mesh}/dataplanes/{name}/_config` - GET
- `/mesh/control-planes/{cpId}/meshes/{mesh}/{policyType}/{policyName}/_resources/dataplanes` - GET
- Mesh policy types: meshaccesslogs, meshcircuitbreakers, meshfaultinjections, meshhealthchecks, meshhttproutes, meshloadbalancingstrategies, meshmetrics, meshpassthroughs, meshproxypatches, meshratelimits, meshretries, meshtcproutes, meshtimeouts, meshtlses, meshtraces, meshtrafficpermissions
- Additional resources: meshes, meshgateways, hostnamegenerators, meshexternalservices, meshmultizoneservices, meshservices, meshglobalratelimits, meshopas
- `/mesh/control-planes` - GET, POST

---

## Analytics & Monitoring

### [Analytics Dashboards (v2)](https://developer.konghq.com/api/konnect/analytics-dashboards/v2/)

Dashboard tiles with saved queries

**Base URL**: `https://us.api.konghq.com/v2`

**Endpoints**:
- `/dashboards` - GET, POST
- `/dashboards/{dashboardId}` - GET, PUT, DELETE

### [API Request (v2)](https://developer.konghq.com/api/konnect/analytics-requests/v2/)

Near real-time request records

**Base URL**: `https://us.api.konghq.com/v2`

**Endpoints**:
- `/api-requests` - POST

### [Metrics Endpoint (4.0)](https://developer.konghq.com/api/konnect/metrics/4.0/)

Metrics extraction

**Base URL**: `https://us.api.konghq.com/v1`

**Endpoints**:
- `/metrics` - POST

---

## Misc

### [Consumers (v1)](https://developer.konghq.com/api/konnect/consumers/v1/)

Centralized consumer management

**Base URL**: `https://us.api.konghq.com/v1`

**Endpoints**:
- `/realms` - GET, POST
- `/realms/{realmId}` - GET, PATCH, DELETE
- `/realms/{realmId}/consumers` - GET, POST
- `/realms/{realmId}/consumers/{consumerId}` - GET, PATCH, DELETE
- `/realms/{realmId}/consumers/{consumerId}/move` - POST
- `/realms/{realmId}/consumers/{consumerId}/keys` - GET, POST
- `/realms/{realmId}/consumers/{consumerId}/keys/{keyId}` - GET, DELETE

### [Search (v1)](https://developer.konghq.com/api/konnect/ksearch/v1/)

Konnect search functionality

**Base URL**: `https://us.api.konghq.com/v1`

**Endpoints**:
- `/search` - GET
- `/search/types` - GET

### [CMEK (v1)](https://developer.konghq.com/api/konnect/cmek/v1/)

Customer Managed Encryption Keys

**Base URL**: `https://us.api.konghq.com/v0`

**Endpoints**:
- `/blobs/{blobId}` - GET, PUT
- `/materials/encryption` - POST
- `/materials/decryption` - POST
- `/cmeks` - GET
- `/cmeks/{cmekId}` - GET, PUT, DELETE

### [Kong Identity (v1)](https://developer.konghq.com/api/konnect/kong-identity/v1/)

Beta identity features

**Base URL**: `https://us.api.konghq.com/v1`

**Endpoints**:
- `/auth-servers` - GET, POST
- `/auth-servers/{authServerId}` - GET, PATCH, DELETE
- `/auth-servers/{authServerId}/claims` - GET, POST
- `/auth-servers/{authServerId}/claims/{claimId}` - GET, PATCH, DELETE
- `/auth-servers/{authServerId}/scopes` - GET, POST
- `/auth-servers/{authServerId}/scopes/{scopeId}` - GET, PATCH, DELETE
- `/auth-servers/{authServerId}/clients` - GET, POST
- `/auth-servers/{authServerId}/clients/{clientId}` - GET, PATCH, PUT, DELETE
- `/auth-servers/{authServerId}/clients/{clientId}/test-claim` - POST
- `/auth-servers/{authServerId}/clients/{clientId}/tokens` - GET, DELETE
- `/auth-servers/{authServerId}/clients/{clientId}/tokens/{tokenId}` - GET, DELETE

