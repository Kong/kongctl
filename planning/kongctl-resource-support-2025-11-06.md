# Kong Konnect Resource Support Matrix

- **Report Date:** November 6, 2025
- **Kongctl Version:** Tech Preview
- **SDK Version:** `sdk-konnect-go (latest)`

## Executive Summary

This report provides a comprehensive analysis of resource support across the Kong
Konnect platform and the `kongctl` CLI tool. It catalogs all resources available
in the Kong Konnect SaaS platform and maps their support status in `kongctl`'s
declarative configuration and imperative command interfaces.

### Key Findings

- **Total Kong Konnect Resources:** 78 resource types (69 standard + 9 Event Gateway)
- **Declarative Configuration Support:** 13 resource types (16% coverage)
  - 4 parent resources (with `kongctl` metadata support)
  - 9 child resources
- **Imperative Command Support:** 19 resource types (25% coverage)
  - Parent resources with full get/list operations
  - Child resources accessible via parent context
  - Read-only operations for all imperative commands
- **Event Gateway Resources:** 9 new resource types (0% `kongctl` support)
  - Event Gateway not yet built into `sdk-konnect-go`

---

## Comprehensive Resource Support Matrix

**Notes:**
- Declarative support includes both parent resources 
  (with kongctl metadata) and child resources (children don't support metadata)
- Imperative support is read-only (get/list operations only)
- See the Notes column for specific command syntax and details

### APIs and children (10 resources)

| Resource Name | Declarative | Imperative | Notes |
|--------------|-------------|------------|-------|
| api | ✅ Parent | ✅ Get | Full declarative support as parent resource |
| api_version | ✅ Child | ✅ Get | Via `kongctl get api versions --api-id <id>` |
| api_publication | ✅ Child | ✅ Get | Via `kongctl get api publications --api-id <id>` |
| api_implementation | ✅ Child | ✅ Get | Via `kongctl get api implementations --api-id <id>` |
| api_documentation | ✅ Child | ✅ Get | Via `kongctl get api documents --api-id <id>` |
| api_attributes | ✅ on API resource | ✅ Get Only | Managed via `attributes` field on API resource; `kongctl get api attributes --api-id <id>` for inspection |
| apikeys | ❌ | ❌ | API key credentials for consumers |
| applicationregistrations | ❌ | ❌ | Developer application registrations |
| applications | ❌ | ❌ | Developer applications |
| appauthstrategies | ✅ Parent | ✅ Get | Declarative uses `application_auth_strategy` |

---

### Gateway Manager & core entities (18 resources)

| Resource Name | Declarative | Imperative | Notes |
|--------------|-------------|------------|-------|
| control_planes | ✅ Parent | ✅ Get | Via `kongctl get gateway control-planes` |
| control_plane_groups | ✅ Parent | ✅ Get | Managed via cluster_type in control_planes |
| services | ✅ Child | ✅ Get | Via `kongctl get gateway services --control-plane <id>` |
| routes | ❌ | ✅ Get | Via `kongctl get gateway routes --control-plane <id>` |
| consumers | ❌ | ✅ Get | Via `kongctl get gateway consumers --control-plane <id>` |
| plugins | ❌ | ❌ | |
| upstreams | ❌ | ❌ | |
| targets | ❌ | ❌ | |
| certificates | ❌ | ❌ | |
| cacertificates | ❌ | ❌ | |
| snis | ❌ | ❌ | |
| consumergroups | ❌ | ❌ | |
| vaults | ❌ | ❌ | |
| keys | ❌ | ❌ | |
| keysets | ❌ | ❌ | |
| cloudgateways | ❌ | ❌ | |
| dpnodes | ❌ | ❌ | Data plane nodes |
| dpcertificates | ❌ | ❌ | Data plane certificates |

---

### Event Gateway Resources (9 resources - NEW)

Kong Event Gateway provides event streaming capabilities for Apache Kafka
workloads. This is a new control plane type in Kong Konnect (v1.0.0 spec).

| Resource Name | Declarative | Imperative | Notes |
|--------------|-------------|------------|-------|
| eventgateways | ❌ | ❌ | Event Gateway control plane instances |
| eventgateway-backendclusters | ❌ | ❌ | Kafka backend cluster configurations |
| eventgateway-listeners | ❌ | ❌ | Gateway listeners with policy support |
| eventgateway-virtualclusters | ❌ | ❌ | Virtual Kafka clusters with routing |
| eventgateway-schemaregistries | ❌ | ❌ | Schema registry integrations |
| eventgateway-vaults | ❌ | ❌ | Secret vaults for credential management |
| eventgateway-statickeys | ❌ | ❌ | Static encryption keys |
| eventgateway-nodes | ❌ | ❌ | Event Gateway data plane nodes |
| eventgateway-certificates | ❌ | ❌ | Data plane TLS certificates |

---

### Portal (13 resources)

| Resource Name | Declarative | Imperative | Notes |
|--------------|-------------|------------|-------|
| portals | ✅ Parent | ✅ Get | Full declarative support as parent resource |
| pages | ✅ Child | ✅ Get | Via `kongctl get portal pages --portal-id <id>` |
| snippets | ✅ Child | ✅ Get | Via `kongctl get portal snippets --portal-id <id>` |
| portal_customization | ✅ Child | ❌ | Nested under portals |
| portal_customdomains | ✅ Child | ❌ | Nested under portals, special handling required |
| portal_developers | ❌ | ✅ Get | Via `kongctl get portal developers --portal-id <id>` |
| portal_teams | ❌ | ✅ Get | Via `kongctl get portal teams --portal-id <id>` |
| portal_teammembership | ❌ | ❌ | |
| portal_teamroles | ❌ | ❌ | |
| portal_auditlogs | ❌ | ❌ | Read-only audit logs |
| portal_authsettings | ❌ | ❌ | |
| portal_emails | ❌ | ❌ | |
| applications | ❌ | ✅ Get/Delete | Via `kongctl get portal applications --portal-id <id>` or `kongctl delete portal application --portal-id <id> <application-id>` |
| assets | ❌ | ❌ | Portal asset management |

---

### Identity & Access Management (12 resources)

| Resource Name | Declarative | Imperative | Notes |
|--------------|-------------|------------|-------|
| users | ❌ | ❌ | Organization users |
| teams | ❌ | ❌ | |
| teammembership | ❌ | ❌ | |
| roles | ❌ | ❌ | |
| systemaccounts | ❌ | ❌ | |
| systemaccountsaccesstokens | ❌ | ❌ | |
| systemaccountsroles | ❌ | ❌ | |
| systemaccountsteammembership | ❌ | ❌ | |
| personalaccesstokens | ❌ | ❌ | |
| authentication | ❌ | ❌ | Auth configuration |
| authsettings | ❌ | ❌ | |
| impersonationsettings | ❌ | ❌ | |
| invites | ❌ | ❌ | User invitations |

---

### Consumers (5 resources)

| Resource Name | Declarative | Imperative | Notes |
|--------------|-------------|------------|-------|
| basicauthcredentials | ❌ | ❌ | Basic auth credentials for consumers |
| hmacauthcredentials | ❌ | ❌ | HMAC auth credentials |
| jwts | ❌ | ❌ | JWT credentials |
| mtlsauthcredentials | ❌ | ❌ | mTLS credentials |
| acls | ❌ | ❌ | ACL groups for consumers |

---

### Custom Plugins (4 resources)

| Resource Name | Declarative | Imperative | Notes |
|--------------|-------------|------------|-------|
| schemas | ❌ | ❌ | Schema validation |
| customplugins | ❌ | ❌ | Custom plugin management |
| custompluginschemas | ❌ | ❌ | |
| configstores | ❌ | ❌ | Configuration storage |
| configstoresecrets | ❌ | ❌ | Secrets management |

---

### Organization & Miscellaneous (5 resources)

| Resource Name | Declarative | Imperative | Notes |
|--------------|-------------|------------|-------|
| notifications | ❌ | ❌ | Platform notifications |
| dcrproviders | ❌ | ❌ | Dynamic client registration |
| degraphqlroutes | ❌ | ❌ | GraphQL route configuration |
| partials | ❌ | ❌ | Partial content management |
| partiallinks | ❌ | ❌ | Partial content linking |

---

## Detailed Analysis

### Declarative Configuration Support

#### Parent Resources (4 resources)

Parent resources support the full `kongctl` metadata specification including:
- Namespace isolation for multi-team management
- Protected flag to prevent accidental deletion
- Label-based resource management

**Supported Parent Resources:**
1. **APIs** (`apis`)
   - Core API product management
   - Supports nested child resources (versions, publications, implementations,
     documents)
   - Full lifecycle management

2. **Portals** (`portals`)
   - Developer portal configuration
   - Supports nested child resources (pages, snippets, customizations, domains)
   - Portal customization and theming

3. **Application Auth Strategies** (`application_auth_strategies`)
   - OAuth 2.0 and other auth strategy configuration
   - Referenced by portals for developer authentication

4. **Control Planes** (`control_planes`)
   - Gateway control plane management
   - Support for control plane groups via `cluster_type`
   - Group membership management via `members` array

#### Child Resources (9 resources)

Child resources are defined either nested under their parent or at the root
level with a parent reference. They inherit the namespace from their parent
resource.

**API Child Resources:**
- `api_versions` - API version and specification management
- `api_publications` - Publishing APIs to portals
- `api_implementations` - Backend implementation configuration
- `api_documents` - Additional documentation beyond OpenAPI specs
- `attributes` - API attributes managed via attributes field on API resource (not as separate child resource)

**Portal Child Resources:**
- `portal_pages` - Custom portal pages
- `portal_snippets` - Reusable content snippets
- `portal_customizations` - Theme and appearance customization
- `portal_custom_domains` - Custom domain configuration (special handling)

**Gateway Child Resources:**
- `gateway_services` - Kong Gateway service configuration

#### Special Considerations

**Portal Custom Domains:**
The portal custom domains resource has special handling requirements:
- The Konnect API only returns a subset of fields (hostname, enabled,
  verification method, CNAME status, skip_ca_check)
- Certificate and private key data are write-only
- Pure certificate rotations require workarounds (toggle detectable fields or
  remove/reapply)
- See declarative.md:856-862 for full details

**Control Plane Groups:**
Control plane groups are managed through the `control_planes` resource with:
- `cluster_type: "CLUSTER_TYPE_CONTROL_PLANE_GROUP"`
- `members` array with control plane ID references
- Full membership list replacement on apply/sync

---

### Imperative Command Support

Imperative commands provide read-only operations for resources via
`kongctl get` and `kongctl list` commands. All imperative commands support
JSON, YAML, and text output formats.

#### Parent Resources (6 resources)

These resources can be listed and retrieved individually:

1. **APIs** - `kongctl get apis` / `kongctl get api <id|name>`
2. **Portals** - `kongctl get portals` / `kongctl get portal <id|name>`
3. **Auth Strategies** - `kongctl get auth-strategies` / `kongctl get auth-strategy <id|name>`
4. **Control Planes** - `kongctl get gateway control-planes` / `kongctl get gateway control-plane <id|name>`

#### API Child Resources (5 resources)

These resources are accessed via their parent API:

1. **API Versions** - `kongctl get api versions --api-id <id>`
2. **API Publications** - `kongctl get api publications --api-id <id>`
3. **API Implementations** - `kongctl get api implementations --api-id <id>`
4. **API Documents** - `kongctl get api documents --api-id <id>`
5. **API Attributes** - `kongctl get api attributes --api-id <id>` (read-only inspection; managed via `attributes` field on API resource)

#### Portal Child Resources (5 resources)

These resources are accessed via their parent Portal:

1. **Portal Pages** - `kongctl get portal pages --portal-id <id>` or `--portal-name <name>`
2. **Portal Snippets** - `kongctl get portal snippets --portal-id <id>`
3. **Portal Developers** - `kongctl get portal developers --portal-id <id>`
4. **Portal Teams** - `kongctl get portal teams --portal-id <id>`
5. **Portal Applications** - `kongctl get portal applications --portal-id <id>`

#### Gateway Child Resources (3 resources)

These resources are accessed via their parent Control Plane:

1. **Gateway Services** - `kongctl get gateway services --control-plane <id|name>`
2. **Gateway Routes** - `kongctl get gateway routes --control-plane <id|name>`
3. **Gateway Consumers** - `kongctl get gateway consumers --control-plane <id|name>`

#### Informational Endpoints (Not Counted as Resources)

These commands query read-only informational endpoints and are not manageable resources:

1. **Me** - `kongctl get me` (current user information)
2. **Organization** - `kongctl get organization` (current organization details)

**Note:** These are utility commands for inspecting context, not resource management operations.

#### Command Structure Examples

```bash
# Parent resources (Konnect-first pattern)
kongctl get apis
kongctl get api "Users API"
kongctl get portals
kongctl get portal my-portal
kongctl get auth-strategies

# API child resources
kongctl get api versions --api-id <uuid>
kongctl get api publications --api-id <uuid>
kongctl get api documents --api-id <uuid>
kongctl get api implementations --api-id <uuid>
kongctl get api attributes --api-id <uuid>

# Portal child resources
kongctl get portal pages --portal-name "My Portal"
kongctl get portal pages --portal-id <uuid>
kongctl get portal snippets --portal-id <uuid>
kongctl get portal developers --portal-id <uuid>
kongctl get portal teams --portal-id <uuid>
kongctl get portal applications --portal-id <uuid>

# Gateway resources
kongctl get gateway control-planes
kongctl get gateway control-plane production
kongctl get gateway services --control-plane production
kongctl get gateway routes --control-plane production
kongctl get gateway consumers --control-plane production

# Explicit product pattern
kongctl get konnect apis
kongctl get konnect gateway control-planes

# Special resources
kongctl get me
kongctl get organization

# Output formatting
kongctl get apis -o json
kongctl get apis -o yaml
kongctl get apis -o text  # default
```

#### Interactive Mode

All get commands support an interactive browser mode via the `-i` flag:
```bash
kongctl get -i
```

This launches a terminal UI for browsing resources hierarchically with
real-time navigation and filtering.

---

## Coverage Analysis

### High Priority Gaps

Based on common Kong Konnect usage patterns, the following resources would
benefit from additional kongctl support:

#### Gateway Configuration (High Value for Declarative)
- **Plugins** - Plugin configuration is core to Gateway functionality (no support)
- **Upstreams/Targets** - Load balancing configuration (no support)
- **Certificates/CA Certificates** - TLS/mTLS configuration (no support)
- **Consumer Groups** - Consumer organization and management (no support)
- **Routes** - Currently only imperative support, need declarative
- **Consumers** - Currently only imperative support, need declarative

#### Portal & Developer Experience (Need Declarative)
- **Portal Developers** - Developer account management (imperative only)
- **Portal Teams** - Team-based portal access control (imperative only)
- **Applications** - Developer application lifecycle (imperative only)
- **Portal Customizations** - Currently declarative only, need imperative
- **Portal Custom Domains** - Currently declarative only, need imperative

#### Identity & Access Management
- **Teams/Team Membership** - Organization team management
- **Roles** - RBAC configuration
- **System Accounts** - Service account management

### Medium Priority Gaps

#### Gateway Configuration
- **Vaults** - Secrets management integration
- **Keys/Keysets** - Key management for JWT/encryption
- **Consumer Credentials** - Basic auth, HMAC, JWT, mTLS credentials

#### Configuration Management
- **Config Stores** - Configuration storage
- **Custom Plugins** - Custom plugin deployment

### Low Priority Gaps

Resources that are typically managed through the UI or have specialized use
cases:
- Audit logs (read-only by nature)
- Notifications
- Impersonation settings
- Assets and partials (portal content management)

---

## YAML Tag Support

Declarative configuration supports special YAML tags for enhanced
functionality:

### File Tag (`!file`)
Load content from external files:
```yaml
apis:
  - ref: users-api
    description: !file ./docs/api-description.md
    versions:
      - ref: v1
        spec: !file ./specs/users-v1.yaml
```

**Value Extraction:**
Extract specific fields from structured files:
```yaml
apis:
  - ref: products-api
    name: !file ./specs/products.yaml#info.title
    description: !file ./specs/products.yaml#info.description
```

**Security:**
- Path traversal prevention (no absolute paths or ../)
- File size limits (10MB)
- Files cached during execution for performance

### Reference Tag (`!ref`)
Reference other declarative resources:
```yaml
control_planes:
  - ref: shared-group
    cluster_type: "CLUSTER_TYPE_CONTROL_PLANE_GROUP"
    members:
      - id: !ref prod-us-runtime#id
      - id: !ref prod-eu-runtime#id
```

---

## Best Practices & Recommendations

### For Declarative Configuration Users

1. **Start with Parent Resources**
   - Focus on APIs, Portals, Auth Strategies, and Control Planes
   - These provide the most value with full lifecycle management

2. **Use Namespaces for Isolation**
   - One namespace per team or environment
   - Set namespace in `_defaults` for consistency

3. **Protect Production Resources**
   - Mark critical resources with `protected: true`
   - Prevents accidental deletion

4. **Leverage YAML Tags**
   - Use `!file` to keep specs and docs external
   - Use `!ref` for cross-resource references
   - Improves maintainability and reusability

5. **Plan-Based Workflows**
   - Generate plans for review before applying
   - Store plans for audit trail and rollback
   - Use `--dry-run` for preview without execution

### For Imperative Command Users

1. **Use for Discovery and Debugging**
   - List resources to understand current state
   - Get individual resources for detailed inspection
   - Use `-o json` or `-o yaml` for programmatic processing
   - Access child resources via parent context (--api-id, --portal-id, etc.)

2. **Parent-Child Navigation**
   - Start with parent resources: `kongctl get apis`
   - Navigate to children: `kongctl get api versions --api-id <id>`
   - Use names or IDs for parent resources: `--portal-name "My Portal"` or `--portal-id <uuid>`

3. **Interactive Mode for Exploration**
   - Use `kongctl get -i` to browse resources interactively
   - Navigate resource hierarchies visually
   - Quickly find resource IDs and names
   - Filter and search across resources

4. **Transition to Declarative**
   - Use imperative commands to understand current state
   - Export to declarative format using `kongctl dump declarative`
   - Migrate critical resources to version control
   - Keep imperative for one-off queries and debugging

### For Feature Development

#### Highest Impact - Event Gateway Support (NEW)
1. **Event Gateways (Parent)** - New control plane type for event streaming
2. **Backend Clusters (Child)** - Kafka backend configuration
3. **Virtual Clusters (Child)** - Multi-tenancy and routing

#### High Impact Quick Wins - Standard Resources
1. **Routes (Declarative)** - Already has imperative support, add declarative
2. **Consumers (Declarative)** - Already has imperative support, add declarative
3. **Plugins (Both)** - Critical Gateway feature, no support yet

#### Medium Impact - Extend Declarative
1. **Portal Developers (Declarative)** - Already has imperative support
2. **Portal Teams (Declarative)** - Already has imperative support
3. **Portal Applications (Declarative)** - Already has imperative support

#### Medium Impact - New Support
1. **Upstreams/Targets (Both)** - Load balancing configuration
2. **Certificates (Both)** - TLS configuration

#### Low Impact - Extend Imperative
1. **Portal Customizations (Imperative)** - Already has declarative
2. **Portal Custom Domains (Imperative)** - Already has declarative

---

## Technical Notes

### Declarative Resource Matching

The declarative configuration engine uses the following strategy to match
resources between configuration and Konnect:

1. **Parent Resources** - Matched by name field
2. **Child Resources** - Matched by parent + identifying fields
3. **Namespace Filtering** - Only resources in specified namespaces are
   considered
4. **Label-Based Management** - Uses `KONGCTL-namespace` and
   `KONGCTL-protected` labels

### Imperative Command Implementation

Imperative commands are implemented under:
- `/internal/cmd/root/verbs/get/` - Get command verb implementation
  - `api.go` - API parent command routing
  - `portal.go` - Portal parent command routing
  - `gateway.go` - Gateway resources routing
  - `authstrategy.go` - Auth strategy command routing
  - `me.go` - Current user command
  - `organization.go` - Organization command
- `/internal/cmd/root/verbs/list/` - List command verb implementation
- `/internal/cmd/root/products/konnect/api/` - API and child resources
  - `versions.go`, `publications.go`, `implementations.go`, `documents.go`, `attributes.go`
- `/internal/cmd/root/products/konnect/portal/` - Portal and child resources
  - `pages.go`, `snippets.go`, `developers.go`, `teams.go`, `applications.go`
- `/internal/cmd/root/products/konnect/gateway/` - Gateway resources
  - `controlplane/`, `service/`, `route/`, `consumer/`

### SDK Integration

All kongctl operations use the official `sdk-konnect-go` SDK:
- Location: `../sdk-konnect-go` (sibling directory)
- Documentation: `/docs/sdks/` (per-resource SDK docs)
- 71 resource types fully documented

---


#### Event Gateway Resource Hierarchy

Event Gateways follow a hierarchical model similar to Kong Gateway:

```
Event Gateway (Control Plane)
├── Backend Clusters (Kafka backends)
├── Virtual Clusters (Logical Kafka clusters)
│   ├── Cluster Policies
│   ├── Produce Policies
│   └── Consume Policies
├── Listeners (Gateway entry points)
│   └── Listener Policies
├── Schema Registries
├── Vaults
│   └── Secrets
├── Static Keys
├── Nodes (Data Plane)
└── Certificates (Data Plane)
```

#### Event Gateway Capabilities

1. **Multi-Tenancy** - Virtual clusters provide isolated Kafka environments
2. **Policy-Based Routing** - Cluster, produce, and consume policies control
   traffic flow
3. **Schema Validation** - Integrated schema registry support
4. **Security** - Encryption, ACLs, authentication, and authorization policies
5. **Expression Language** - DSL for dynamic configuration and routing

#### Event Gateway Policy Types

- **ACL Policies** - Access control for topics and consumer groups
- **Schema Validation Policies** - Enforce schema compliance for messages
- **Encryption/Decryption Policies** - Message-level encryption
- **Authentication Policies** - SASL, API key, and other auth mechanisms
- **Authorization Policies** - Fine-grained permission control
- **Rate Limiting Policies** - Throughput and quota management

#### Kongctl Support Status

**Current Status:** No support for Event Gateway resources in kongctl.

**Recommended Implementation Priority:**

1. **High Priority (Parent Resources)**
   - Event Gateways (control plane type)
   - Backend Clusters (critical infrastructure)
   - Virtual Clusters (core multi-tenancy feature)

2. **Medium Priority (Child Resources)**
   - Listeners and Listener Policies
   - Schema Registries
   - Vaults and Secrets

3. **Lower Priority (Operational)**
   - Nodes (read-only status)
   - Certificates (similar to Gateway certificates)
   - Static Keys

**Implementation Approach:**

Event Gateway resources should follow the same pattern as Kong Gateway
resources:
- Declarative configuration for Event Gateways, Backend Clusters, Virtual
  Clusters, and their policies
- Imperative commands for discovery and debugging
- Support for both nested and flat configuration styles
- Namespace isolation for multi-team management

---

## Appendix: Resource Type Naming

Some resources use different names between Kong Konnect SDK and kongctl
declarative configuration:

| Kong Konnect SDK | Kongctl Declarative | Reason |
|------------------|---------------------|---------|
| appauthstrategies | application_auth_strategies | Clarity and consistency |
| apidocumentation | api_documents | Shorter, more intuitive |
| apiimplementation | api_implementations | Plural consistency |
| apipublication | api_publications | Plural consistency |
| apiversion | api_versions | Plural consistency |
| pages | portal_pages | Namespace clarity |
| snippets | portal_snippets | Namespace clarity |
| portalcustomization | portal_customizations | Plural consistency |
| portalcustomdomains | portal_custom_domains | Underscore consistency |
| services | gateway_services | Namespace clarity |
| controlplanegroups | (via control_planes) | Unified model via cluster_type |

---

## Appendix: Complete SDK Resource Listing

### Standard Kong Konnect Resources (69 resources)

1. acls
2. api
3. apiattributes
4. apidocumentation
5. apiimplementation
6. apikeys
7. apipublication
8. apiversion
9. appauthstrategies
10. applicationregistrations
11. applications
12. assets
13. authentication
14. authsettings
15. basicauthcredentials
16. cacertificates
17. certificates
18. cloudgateways
19. configstores
20. configstoresecrets
21. consumergroups
22. consumers
23. controlplanegroups
24. controlplanes
25. customplugins
26. custompluginschemas
27. dcrproviders
28. degraphqlroutes
29. dpcertificates
30. dpnodes
31. hmacauthcredentials
32. impersonationsettings
33. invites
34. jwts
35. keys
36. keysets
37. mtlsauthcredentials
38. notifications
39. pages
40. partiallinks
41. partials
42. personalaccesstokens
43. plugins
44. portalauditlogs
45. portalauthsettings
46. portalcustomdomains
47. portalcustomization
48. portaldevelopers
49. portalemails
50. portals
51. portalteammembership
52. portalteamroles
53. portalteams
54. roles
55. routes
56. schemas
57. services
58. snippets
59. snis
60. systemaccounts
61. systemaccountsaccesstokens
62. systemaccountsroles
63. systemaccountsteammembership
64. targets
65. teammembership
66. teams
67. upstreams
68. users
69. vaults

**Note:** Read-only informational endpoints `me` and `organization` are not included in this count as they are not manageable resources.

### Event Gateway Resources (9 resources - NEW in v1.0.0)

1. eventgateways - Event Gateway control plane instances
2. eventgateway-backendclusters - Backend Kafka cluster configurations
3. eventgateway-listeners - Gateway listeners with policy chains
4. eventgateway-virtualclusters - Virtual Kafka clusters with routing
5. eventgateway-schemaregistries - Schema registry integrations
6. eventgateway-vaults - Secret vault management
7. eventgateway-statickeys - Static encryption keys
8. eventgateway-nodes - Event Gateway data plane nodes
9. eventgateway-certificates - Data plane TLS certificates

**Note:** Event Gateway also includes numerous policy resource types
(cluster-policies, produce-policies, consume-policies, listener-policies) which
are child resources of their respective parent resources.

---

## Conclusion

Kongctl provides comprehensive support for Kong Konnect resource management
through two complementary interfaces:

### Declarative Configuration (18% coverage)
Strong support for core resources (APIs, Portals, Auth Strategies, Control
Planes) with a clean, namespace-aware model suitable for multi-team
environments and CI/CD integration. The stateless, plan-based approach and YAML
tag system provide a modern, Git-friendly infrastructure-as-code experience.

### Imperative Commands (26% coverage)
Extensive read-only access to 21 resource types including all declarative
resources plus child resources for discovery and debugging. The parent-child
command structure provides intuitive navigation through resource hierarchies.

**Note:** Coverage percentage decreased from 30% to 26% with the addition of 9
Event Gateway resources.

### Combined Strengths

Together, these interfaces provide 26% coverage of Kong Konnect resources
(including new Event Gateway resources) with strategic overlap:
- **Declare what matters** - APIs, Portals, Auth Strategies, Control Planes,
  and their critical children
- **Query everything else** - Use imperative commands for discovery, debugging,
  and accessing resources not yet in declarative format
- **Best of both worlds** - Version control critical resources while maintaining
  flexibility to query runtime state

### Future Expansion Priorities

1. **Event Gateway Support (NEW)** - Event Gateways, Backend Clusters, Virtual
   Clusters, Listeners, and Policy management (9 new resources)
2. **Gateway Configuration** - Plugins, Routes (declarative), Consumers
   (declarative), Upstreams, Certificates
3. **Developer Experience (Declarative)** - Portal Developers, Applications,
   Teams (already have imperative)
4. **Bidirectional Parity** - Portal Customizations and Custom Domains
   (imperative), API Attributes (declarative)

The tool successfully balances infrastructure-as-code principles with
operational flexibility, making it suitable for both day-0 provisioning and
day-2 operations.

---

**Report Generated:** November 6, 2025
**Tool Version:** kongctl (Tech Preview)
**Data Sources:**
- `/internal/declarative/resources/` - Declarative resource implementations
- `/internal/cmd/root/verbs/get/` - Imperative command implementations
- `../sdk-konnect-go/docs/sdks/` - Kong Konnect SDK resource documentation
