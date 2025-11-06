# Kong Konnect Resource Support Matrix
**Report Date:** November 6, 2025
**Kongctl Version:** Tech Preview
**SDK Version:** sdk-konnect-go

## Executive Summary

This report provides a comprehensive analysis of resource support across the Kong
Konnect platform and the kongctl CLI tool. It catalogs all resources available
in the Kong Konnect SaaS platform and maps their support status in kongctl's
declarative configuration and imperative command interfaces.

### Key Findings

- **Total Kong Konnect Resources:** 71 resource types
- **Declarative Configuration Support:** 13 resource types (18% coverage)
  - 4 parent resources (with kongctl metadata support)
  - 9 child resources
- **Imperative Command Support:** 9 resource types (13% coverage)
  - Full CRUD via `get`/`list` commands
  - Read-only operations for most resources

### Coverage Summary

| Support Type | Resources Supported | Coverage |
|-------------|---------------------|----------|
| Kong Konnect SDK | 71 | 100% |
| Declarative (Parent) | 4 | 6% |
| Declarative (Child) | 9 | 13% |
| Declarative (Total) | 13 | 18% |
| Imperative | 9 | 13% |

---

## Resource Categories

Kong Konnect resources can be organized into the following categories:

### API Products Category
Resources related to API product management, documentation, and publishing.

### Gateway Configuration Category
Resources for configuring Kong Gateway instances, including services, routes,
plugins, and certificates.

### Portal & Developer Experience Category
Resources for managing developer portals, customization, and developer
interactions.

### Identity & Access Management Category
Resources for authentication, authorization, users, teams, and system accounts.

### Infrastructure & Operations Category
Resources for control planes, cloud gateways, certificates, and operational
configuration.

---

## Comprehensive Resource Support Matrix

### Legend

- ✅ **Full Support**: Complete CRUD operations available
- 🟡 **Parent**: Declarative parent resource (supports kongctl metadata)
- 🔵 **Child**: Declarative child resource (nested under parent)
- 📖 **Read-Only**: Get/list operations only
- ❌ **Not Supported**: No kongctl support currently

---

### API Products & Documentation (11 resources)

| Resource Name | Kong Konnect SDK | Declarative | Imperative | Notes |
|--------------|------------------|-------------|------------|-------|
| api | ✅ | 🟡 Parent | ✅ Get/List | Full declarative support as parent resource |
| apiversion | ✅ | 🔵 Child | ❌ | Nested under apis or standalone with api reference |
| apipublication | ✅ | 🔵 Child | ❌ | Nested under apis or standalone with api reference |
| apiimplementation | ✅ | 🔵 Child | ❌ | Nested under apis or standalone with api reference |
| apidocumentation | ✅ | 🔵 Child | ❌ | Declarative uses `api_document` name |
| apispecification | ✅ | ❌ | ❌ | API spec content managed via apiversion |
| apiattributes | ✅ | ❌ | ❌ | |
| apikeys | ✅ | ❌ | ❌ | API key credentials for consumers |
| applicationregistrations | ✅ | ❌ | ❌ | Developer application registrations |
| applications | ✅ | ❌ | ❌ | Developer applications |
| appauthstrategies | ✅ | 🟡 Parent | ✅ Get/List | Declarative uses `application_auth_strategy` |

---

### Gateway Configuration (18 resources)

| Resource Name | Kong Konnect SDK | Declarative | Imperative | Notes |
|--------------|------------------|-------------|------------|-------|
| controlplanes | ✅ | 🟡 Parent | ✅ Get/List | Full support including control plane groups |
| controlplanegroups | ✅ | 🟡 Parent | ✅ Get/List | Managed via cluster_type in control_planes |
| services | ✅ | 🔵 Child | ✅ Get/List | Declarative uses `gateway_service` name |
| routes | ✅ | ❌ | ✅ Get/List | Imperative only |
| consumers | ✅ | ❌ | ✅ Get/List | Imperative only |
| plugins | ✅ | ❌ | ❌ | |
| upstreams | ✅ | ❌ | ❌ | |
| targets | ✅ | ❌ | ❌ | |
| certificates | ✅ | ❌ | ❌ | |
| cacertificates | ✅ | ❌ | ❌ | |
| snis | ✅ | ❌ | ❌ | |
| consumergroups | ✅ | ❌ | ❌ | |
| vaults | ✅ | ❌ | ❌ | |
| keys | ✅ | ❌ | ❌ | |
| keysets | ✅ | ❌ | ❌ | |
| cloudgateways | ✅ | ❌ | ❌ | |
| dpnodes | ✅ | ❌ | ❌ | Data plane nodes |
| dpcertificates | ✅ | ❌ | ❌ | Data plane certificates |

---

### Portal & Developer Experience (13 resources)

| Resource Name | Kong Konnect SDK | Declarative | Imperative | Notes |
|--------------|------------------|-------------|------------|-------|
| portals | ✅ | 🟡 Parent | ✅ Get/List | Full declarative support as parent resource |
| pages | ✅ | 🔵 Child | ❌ | Declarative uses `portal_page` name |
| snippets | ✅ | 🔵 Child | ❌ | Declarative uses `portal_snippet` name |
| portalcustomization | ✅ | 🔵 Child | ❌ | Nested under portals |
| portalcustomdomains | ✅ | 🔵 Child | ❌ | Nested under portals, special handling required |
| portaldevelopers | ✅ | ❌ | ❌ | Portal developer accounts |
| portalteams | ✅ | ❌ | ❌ | |
| portalteammembership | ✅ | ❌ | ❌ | |
| portalteamroles | ✅ | ❌ | ❌ | |
| portalauditlogs | ✅ | ❌ | ❌ | Read-only audit logs |
| portalauthsettings | ✅ | ❌ | ❌ | |
| portalemails | ✅ | ❌ | ❌ | |
| assets | ✅ | ❌ | ❌ | Portal asset management |

---

### Identity & Access Management (14 resources)

| Resource Name | Kong Konnect SDK | Declarative | Imperative | Notes |
|--------------|------------------|-------------|------------|-------|
| me | ✅ | ❌ | 📖 Get Only | Current user information |
| users | ✅ | ❌ | ❌ | Organization users |
| teams | ✅ | ❌ | ❌ | |
| teammembership | ✅ | ❌ | ❌ | |
| roles | ✅ | ❌ | ❌ | |
| systemaccounts | ✅ | ❌ | ❌ | |
| systemaccountsaccesstokens | ✅ | ❌ | ❌ | |
| systemaccountsroles | ✅ | ❌ | ❌ | |
| systemaccountsteammembership | ✅ | ❌ | ❌ | |
| personalaccesstokens | ✅ | ❌ | ❌ | |
| authentication | ✅ | ❌ | ❌ | Auth configuration |
| authsettings | ✅ | ❌ | ❌ | |
| impersonationsettings | ✅ | ❌ | ❌ | |
| invites | ✅ | ❌ | ❌ | User invitations |

---

### Consumer Credentials (5 resources)

| Resource Name | Kong Konnect SDK | Declarative | Imperative | Notes |
|--------------|------------------|-------------|------------|-------|
| basicauthcredentials | ✅ | ❌ | ❌ | Basic auth credentials for consumers |
| hmacauthcredentials | ✅ | ❌ | ❌ | HMAC auth credentials |
| jwts | ✅ | ❌ | ❌ | JWT credentials |
| mtlsauthcredentials | ✅ | ❌ | ❌ | mTLS credentials |
| acls | ✅ | ❌ | ❌ | ACL groups for consumers |

---

### Configuration & Schema (4 resources)

| Resource Name | Kong Konnect SDK | Declarative | Imperative | Notes |
|--------------|------------------|-------------|------------|-------|
| schemas | ✅ | ❌ | ❌ | Schema validation |
| customplugins | ✅ | ❌ | ❌ | Custom plugin management |
| custompluginschemas | ✅ | ❌ | ❌ | |
| configstores | ✅ | ❌ | ❌ | Configuration storage |
| configstoresecrets | ✅ | ❌ | ❌ | Secrets management |

---

### Organization & Miscellaneous (6 resources)

| Resource Name | Kong Konnect SDK | Declarative | Imperative | Notes |
|--------------|------------------|-------------|------------|-------|
| organization | ✅ (via me SDK) | ❌ | 📖 Get Only | Current organization info |
| notifications | ✅ | ❌ | ❌ | Platform notifications |
| dcrproviders | ✅ | ❌ | ❌ | Dynamic client registration |
| degraphqlroutes | ✅ | ❌ | ❌ | GraphQL route configuration |
| partials | ✅ | ❌ | ❌ | Partial content management |
| partiallinks | ✅ | ❌ | ❌ | Partial content linking |

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

Imperative commands provide read operations for a subset of resources via
`kongctl get` and `kongctl list` commands.

**Fully Supported Resources:**
- `apis` - List and get individual APIs
- `portals` - List and get individual portals
- `auth-strategies` - List and get auth strategies
- `gateway control-planes` - List and get control planes
- `gateway services` - List and get services (requires --control-plane)
- `gateway routes` - List and get routes (requires --control-plane)
- `gateway consumers` - List and get consumers (requires --control-plane)

**Read-Only Resources:**
- `me` - Get current user information (no list)
- `organization` - Get current organization (no list)

**Command Structure:**
```bash
# Konnect-first pattern (implicit konnect product)
kongctl get apis
kongctl get portals
kongctl get auth-strategies
kongctl get gateway control-planes
kongctl get gateway services --control-plane <name|id>

# Explicit product pattern
kongctl get konnect apis
kongctl get konnect gateway control-planes

# Special resources
kongctl get me
kongctl get organization
```

**Interactive Mode:**
All get commands support an interactive browser mode via the `-i` flag:
```bash
kongctl get -i
```

---

## Coverage Analysis

### High Priority Gaps

Based on common Kong Konnect usage patterns, the following resources would
benefit from kongctl support:

#### Gateway Configuration (High Value)
- **Plugins** - Plugin configuration is core to Gateway functionality
- **Upstreams/Targets** - Load balancing configuration
- **Certificates/CA Certificates** - TLS/mTLS configuration
- **Consumer Groups** - Consumer organization and management
- **Routes** - Currently only imperative support

#### Portal & Developer Experience
- **Portal Developers** - Developer account management
- **Portal Teams** - Team-based portal access control
- **Applications** - Developer application lifecycle

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

2. **Interactive Mode for Exploration**
   - Use `kongctl get -i` to browse resources interactively
   - Navigate resource hierarchies
   - Quickly find resource IDs and names

3. **Transition to Declarative**
   - Use imperative commands to understand current state
   - Export to declarative format using `kongctl dump declarative`
   - Migrate critical resources to version control

### For Feature Development

#### High Impact Quick Wins
1. **Routes (Declarative)** - Already has imperative support, add declarative
2. **Plugins (Both)** - Critical Gateway feature
3. **Consumers (Declarative)** - Already has imperative support

#### Medium Impact
1. **Upstreams/Targets (Both)** - Load balancing configuration
2. **Certificates (Both)** - TLS configuration
3. **Portal Developers/Applications (Both)** - Developer experience

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
- `/internal/cmd/root/verbs/get/` - Get command implementation
- `/internal/cmd/root/verbs/list/` - List command implementation
- `/internal/cmd/root/products/konnect/` - Konnect-specific resource commands

### SDK Integration

All kongctl operations use the official `sdk-konnect-go` SDK:
- Location: `../sdk-konnect-go` (sibling directory)
- Documentation: `/docs/sdks/` (per-resource SDK docs)
- 71 resource types fully documented

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
| apispecification | (via api_versions) | Spec is part of version |
| apiversion | api_versions | Plural consistency |
| pages | portal_pages | Namespace clarity |
| snippets | portal_snippets | Namespace clarity |
| portalcustomization | portal_customizations | Plural consistency |
| portalcustomdomains | portal_custom_domains | Underscore consistency |
| services | gateway_services | Namespace clarity |
| controlplanegroups | (via control_planes) | Unified model via cluster_type |

---

## Appendix: Complete SDK Resource Listing

All 71 Kong Konnect SDK resources (alphabetical):

1. acls
2. api
3. apiattributes
4. apidocumentation
5. apiimplementation
6. apikeys
7. apipublication
8. apispecification
9. apiversion
10. appauthstrategies
11. applicationregistrations
12. applications
13. assets
14. authentication
15. authsettings
16. basicauthcredentials
17. cacertificates
18. certificates
19. cloudgateways
20. configstores
21. configstoresecrets
22. consumergroups
23. consumers
24. controlplanegroups
25. controlplanes
26. customplugins
27. custompluginschemas
28. dcrproviders
29. degraphqlroutes
30. dpcertificates
31. dpnodes
32. hmacauthcredentials
33. impersonationsettings
34. invites
35. jwts
36. keys
37. keysets
38. me
39. mtlsauthcredentials
40. notifications
41. pages
42. partiallinks
43. partials
44. personalaccesstokens
45. plugins
46. portalauditlogs
47. portalauthsettings
48. portalcustomdomains
49. portalcustomization
50. portaldevelopers
51. portalemails
52. portals
53. portalteammembership
54. portalteamroles
55. portalteams
56. roles
57. routes
58. schemas
59. services
60. snippets
61. snis
62. systemaccounts
63. systemaccountsaccesstokens
64. systemaccountsroles
65. systemaccountsteammembership
66. targets
67. teammembership
68. teams
69. upstreams
70. users
71. vaults

---

## Conclusion

Kongctl provides strong declarative configuration support for the core Kong
Konnect resources (APIs, Portals, Auth Strategies, Control Planes) with a
clean, namespace-aware model suitable for multi-team environments and CI/CD
integration.

The current 18% declarative coverage and 13% imperative coverage represent a
solid foundation focused on the most commonly used resources. Future expansion
should prioritize:

1. **Gateway Configuration** - Plugins, Routes (declarative), Upstreams,
   Certificates
2. **Developer Experience** - Portal Developers, Applications, Teams
3. **Consumer Management** - Consumer Groups, Credentials

The tool's stateless, plan-based approach and YAML tag system provide a modern,
Git-friendly infrastructure-as-code experience for Kong Konnect users.

---

**Report Generated:** November 6, 2025
**Tool Version:** kongctl (Tech Preview)
**Data Sources:**
- `/internal/declarative/resources/` - Declarative resource implementations
- `/internal/cmd/root/verbs/get/` - Imperative command implementations
- `../sdk-konnect-go/docs/sdks/` - Kong Konnect SDK resource documentation
