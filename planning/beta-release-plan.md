# Kongctl Beta Release Plan

**Date:** 2025-11-07
**Status:** Planning - Pending Clarifications
**Target:** Beta Release

---

## Executive Summary

This document outlines the implementation work required to achieve a beta
release of kongctl. The beta release focuses on two primary additions: **Event
Gateway** and **Service Catalog** support, with potential gaps filled in
existing Portal Management and API Builder areas.

### Beta Release Scope

**Primary Focus Areas:**
1. **Event Gateway** (NEW) - 0% → 100% coverage
2. **Service Catalog** (NEW) - 0% → 100% coverage (subset TBD)

**Secondary Review Areas:**
3. **Portal Management** - Identify and fill gaps (pending clarification)
4. **API Builder** - Verify 100% coverage (likely complete)

**Out of Scope:**
- Control Planes Config (v2) / Core-entities → Deferred to Kong decK tool
- Post-beta items: System Accounts, Teams, Users, Org Auth, Team Mapping, Audit
  Logs, Analytics, Mesh

---

## Current State Analysis

### ✅ Complete for Beta

#### 1. Gateway Control Planes (v2)
**Status:** 100% coverage achieved

**Declarative Support:**
- ✅ Control planes (parent resource)
- ✅ Control plane groups (via `cluster_type` field)
- ✅ Group membership management (via `members` array)

**Imperative Support:**
- ✅ `kongctl get gateway control-planes`
- ✅ `kongctl get gateway control-plane <id|name>`

**API Coverage:**
- Control Planes (v2) API - 8 endpoints - ✅ Fully supported
- Control Planes Config (v2) API - 97 endpoints - ❌ Out of scope (decK)

---

### ✅ Likely Complete for Beta (Pending Verification)

#### 2. API Builder (v3)
**Status:** ~100% coverage (requires verification)

**Declarative Support:**
- ✅ APIs (parent resource)
- ✅ API versions (child) - Note: `/specifications` endpoint deprecated
- ✅ API publications (child)
- ✅ API implementations (child)
- ✅ API documents (child)
- ✅ API attributes (managed via `attributes` field on API resource)

**Imperative Support:**
- ✅ `kongctl get apis`
- ✅ `kongctl get api <id|name>`
- ✅ `kongctl get api versions --api-id <id>`
- ✅ `kongctl get api publications --api-id <id>`
- ✅ `kongctl get api implementations --api-id <id>`
- ✅ `kongctl get api documents --api-id <id>`
- ✅ `kongctl get api attributes --api-id <id>`

**API Coverage:**
- API Builder (v3) API - 18 endpoints
- `/apis/{apiId}/specifications` - ❌ Deprecated, ignoring
- All other endpoints - ✅ Covered

**Verification Needed:**
- Confirm all fields on API resource are declaratively managed
- Confirm versions endpoint fully replaces deprecated specifications endpoint

---

### ⚠️ Needs Gap Analysis (Pending Clarification)

#### 3. Portal Management (v3)
**Status:** Partial coverage (~40-50% estimated)

**Current Declarative Support:**
- ✅ Portals (parent resource)
- ✅ Pages (child)
- ✅ Snippets (child)
- ✅ Customization (child)
- ✅ Custom domains (child, with limitations)

**Current Imperative Support:**
- ✅ `kongctl get portals`
- ✅ `kongctl get portal <id|name>`
- ✅ `kongctl get portal pages --portal-id <id>`
- ✅ `kongctl get portal snippets --portal-id <id>`
- ✅ `kongctl get portal developers --portal-id <id>`
- ✅ `kongctl get portal teams --portal-id <id>`
- ✅ `kongctl get portal applications --portal-id <id>`

**Known Gaps:**

| Resource | Endpoints | Current Support | Recommendation |
|----------|-----------|-----------------|----------------|
| Applications | 4 endpoints | Imperative GET only | ✅ Keep as-is (user-created) |
| Application Registrations | 3 endpoints | None | ⚠️ Add imperative GET only? |
| Authentication Settings | 2 endpoints (GET/PATCH) | None | ⚠️ Clarify if needed |
| Identity Providers | 4 endpoints | None | ⚠️ Clarify if needed |
| Team Group Mappings | 2 endpoints (GET/PATCH) | None | ⚠️ Clarify if needed |
| Assets (logo/favicon) | 6 endpoints | None | ⚠️ Clarify if needed |
| Email Domains | 3 endpoints | None | ⚠️ Clarify if needed |
| Email Delivery | 3 endpoints (GET/PATCH/DELETE) | None | ⚠️ Clarify if needed |
| Email Config | 4 endpoints | None | ⚠️ Clarify if needed |
| Email Templates | 6 endpoints | None | ⚠️ Clarify if needed |
| Audit Log (Portal) | 5 endpoints | None | ⚠️ Clarify if needed |
| Portal Roles | 1 endpoint (GET) | None | ⚠️ Clarify if needed |
| Default Content | 1 endpoint (POST) | None | ⚠️ Clarify if needed |

**Clarification Needed:**
> **Question for decision maker:** Which Portal Management resources beyond the
> current support (portals, pages, snippets, customization, custom domains,
> applications, developers, teams) should be added for beta? Consider:
> authentication settings, identity providers, email configuration, assets, and
> audit logs.

---

## Primary Beta Implementation Work

### 🚧 Implementation Required: Event Gateway (v1)

**Status:** 0% coverage → Target 100%

**Initial Scope (Decision Maker Guidance):**
- Start with Event Gateway instances (control planes) only
- Reevaluate child resources after initial implementation

#### Event Gateway Instances (Control Planes)

**API Endpoints:**
- `GET /event-gateways` - List all event gateways
- `POST /event-gateways` - Create event gateway
- `GET /event-gateways/{gatewayId}` - Get event gateway details
- `PUT /event-gateways/{gatewayId}` - Update event gateway
- `DELETE /event-gateways/{gatewayId}` - Delete event gateway

**Implementation Requirements:**

1. **Declarative Configuration**
   - New parent resource: `event_gateways`
   - Support all fields from Event Gateway resource schema
   - Namespace isolation (kongctl metadata)
   - Protected flag support
   - Label-based management

2. **Imperative Commands**
   - `kongctl get event-gateways` - List all event gateways
   - `kongctl get event-gateway <id|name>` - Get specific event gateway
   - JSON/YAML/text output formats
   - Interactive mode support (`-i` flag)

3. **SDK Integration**
   - Verify `sdk-konnect-go` has Event Gateway v1 support
   - If not, coordinate with SDK team or implement using raw HTTP client

**Estimated Scope:**
- 5 API endpoints for parent resource
- 1 declarative resource type
- 2 imperative commands (list/get)
- Integration with existing kongctl patterns

#### Event Gateway Child Resources (Post-Initial)

**Deferred until after Event Gateway instances are complete:**

| Resource | Endpoints | Priority | Notes |
|----------|-----------|----------|-------|
| Backend Clusters | 4 endpoints | High | Kafka backend configuration |
| Virtual Clusters | 4 endpoints | High | Multi-tenancy and routing |
| Listeners | 4 endpoints | Medium | Gateway entry points |
| Schema Registries | 4 endpoints | Medium | Schema validation |
| Listener Policies | 5 endpoints | Medium | Policy chains for listeners |
| Virtual Cluster Policies | 15 endpoints | Medium | Consume, produce, cluster policies |
| Data Plane Certificates | 4 endpoints | Low | TLS certificates |
| Static Keys | 3 endpoints | Low | Encryption keys |
| Nodes | 4 endpoints | Low | Read-only node status |

**Total Event Gateway API:** 51 endpoints (5 parent + 46 child resources)

**Decision Point:**
> After implementing Event Gateway instances, reevaluate which child resources
> are critical for beta based on user feedback and use cases.

---

### 🚧 Implementation Required: Service Catalog (v1)

**Status:** 0% coverage → Target 100% (subset TBD)

**API Resources:**

| Resource | Endpoints | Description |
|----------|-----------|-------------|
| Catalog Services | 4 endpoints | Service catalog entries (main resource) |
| Integrations | 1 endpoint | Available integrations (read-only) |
| Integration Instances | 5 endpoints | Configured integration instances |
| Resources | 3 endpoints | Resources from integrations |
| Resource Mappings | 3 endpoints | Map resources to catalog services |

**Total API:** 16 endpoints across 5 resource types

**Clarification Needed:**
> **Question for decision maker:** Which Service Catalog resources should be
> supported for beta?
>
> **Options:**
> 1. **Minimal:** Catalog Services only (4 endpoints)
> 2. **Standard:** Catalog Services + Resource Mappings (7 endpoints)
> 3. **Full:** All resources - Catalog Services, Integrations, Integration
>    Instances, Resources, Resource Mappings (16 endpoints)
>
> **Recommendation:** Start with Option 2 (Standard) as it provides core
> catalog functionality plus the ability to link external resources.

**Implementation Requirements (Assuming Standard Option):**

1. **Declarative Configuration**
   - New parent resource: `catalog_services`
   - New child resource: `resource_mappings`
   - Support all fields from resource schemas
   - Namespace isolation and protection

2. **Imperative Commands**
   - `kongctl get catalog-services`
   - `kongctl get catalog-service <id|name>`
   - `kongctl get catalog-service resource-mappings --service-id <id>`
   - `kongctl get integrations` (read-only, informational)
   - JSON/YAML/text output formats

3. **SDK Integration**
   - Verify `sdk-konnect-go` has Service Catalog v1 support
   - Implement declarative resource handlers
   - Implement imperative command handlers

**Estimated Scope (Standard Option):**
- 7 API endpoints (4 catalog-services + 3 resource-mappings)
- 2 declarative resource types (1 parent + 1 child)
- 3-4 imperative commands
- Integration with existing kongctl patterns

---

## Implementation Approach

### Phase 1: Event Gateway Instances
**Goal:** Get Event Gateway control planes working end-to-end

1. **SDK Verification & Preparation**
   - Verify Event Gateway v1 support in `sdk-konnect-go`
   - Review schema and field mappings
   - Identify any SDK gaps or issues

2. **Declarative Implementation**
   - Create `/internal/declarative/resources/eventgateway/` package
   - Implement `EventGateway` resource handler
   - Add YAML schema support
   - Implement CRUD operations (create, read, update, delete)
   - Add namespace and label management

3. **Imperative Implementation**
   - Create `/internal/cmd/root/products/konnect/eventgateway/` package
   - Implement `get event-gateways` command
   - Implement `get event-gateway <id|name>` command
   - Add output formatting (json/yaml/text)
   - Add interactive mode support

4. **Testing & Validation**
   - Unit tests for resource handlers
   - Integration tests for E2E workflows
   - Manual testing against Konnect API
   - Documentation updates

**Quality Gates:**
- `make build` - Success
- `make lint` - Zero issues
- `make test` - All pass
- `make test-integration` - All pass
- Manual E2E validation

### Phase 2: Service Catalog (Pending Clarification)
**Goal:** Implement Service Catalog resources per decision maker guidance

1. **Scope Confirmation**
   - Get clarification on which resources to support
   - Review API schemas and relationships
   - Plan resource hierarchy (parent/child)

2. **SDK Verification & Preparation**
   - Verify Service Catalog v1 support in `sdk-konnect-go`
   - Review schema and field mappings

3. **Declarative Implementation**
   - Create `/internal/declarative/resources/catalog/` package
   - Implement resource handlers for selected resources
   - Add YAML schema support
   - Implement CRUD operations

4. **Imperative Implementation**
   - Create `/internal/cmd/root/products/konnect/catalog/` package
   - Implement get commands for selected resources
   - Add output formatting and interactive mode

5. **Testing & Validation**
   - Same quality gates as Phase 1

### Phase 3: Portal Management Gap Fill (Pending Clarification)
**Goal:** Fill any critical gaps in Portal Management for beta

1. **Scope Confirmation**
   - Get clarification on which Portal resources to add
   - Prioritize based on user value

2. **Incremental Implementation**
   - Add declarative support for selected resources
   - Add imperative GET commands for selected resources
   - Follow existing portal patterns

3. **Testing & Validation**
   - Same quality gates as Phase 1 & 2

### Phase 4: API Builder Verification
**Goal:** Confirm API Builder is complete for beta

1. **Field Coverage Audit**
   - Review all fields on API resource schema
   - Confirm declarative support for all fields
   - Verify versions fully replaced deprecated specifications

2. **Testing & Validation**
   - Test all API Builder workflows
   - Verify field coverage with real-world examples

---

## Resource Count Summary

### Current State
| Area | Resources | Declarative | Imperative |
|------|-----------|-------------|------------|
| API Builder | 6 | 6 (100%) | 6 (100%) |
| Portal Management | 13 | 5 (38%) | 8 (62%) |
| Gateway Control Planes | 2 | 2 (100%) | 2 (100%) |
| Event Gateway | 9 | 0 (0%) | 0 (0%) |
| Service Catalog | 5 | 0 (0%) | 0 (0%) |

### Beta Target (After Phase 1 - Event Gateway Instances)
| Area | Resources | Declarative | Imperative |
|------|-----------|-------------|------------|
| API Builder | 6 | 6 (100%) | 6 (100%) |
| Portal Management | 13 | TBD | TBD |
| Gateway Control Planes | 2 | 2 (100%) | 2 (100%) |
| Event Gateway | 1 (instances only) | 1 (100%) | 1 (100%) |
| Service Catalog | TBD | TBD | TBD |

---

## Open Questions & Blockers

### Critical Path Blockers

1. **Service Catalog Scope** 🔴 **BLOCKING Phase 2**
   > Which Service Catalog resources should be supported for beta?
   > - Catalog Services only?
   > - Catalog Services + Resource Mappings?
   > - All resources (Integrations, Integration Instances, Resources)?

2. **Portal Management Gaps** 🟡 **BLOCKING Phase 3**
   > Which Portal Management resources beyond current support should be added?
   > - Authentication settings?
   > - Identity providers?
   > - Email configuration (domains, delivery, config, templates)?
   > - Assets (logo, favicon)?
   > - Audit logs?
   > - Portal roles?
   > - Default content?

### Non-Blocking Questions

3. **Event Gateway Child Resources** 🟢 **Post Phase 1**
   > After Event Gateway instances are implemented, which child resources
   > should be prioritized?
   > - Backend Clusters?
   > - Virtual Clusters?
   > - Listeners?
   > - Policies?

4. **Application Registrations** 🟢 **Portal related**
   > Should we add imperative GET support for application registrations?
   > (Currently no support, but applications have GET support)

---

## Success Criteria for Beta

### Functional Requirements
- ✅ Event Gateway instances fully supported (declarative + imperative)
- ✅ Service Catalog resources supported per final scope decision
- ✅ Portal Management gaps filled per final scope decision
- ✅ API Builder verified complete (all fields supported)
- ✅ Gateway Control Planes confirmed complete

### Quality Requirements
- ✅ All quality gates passing (`make build && make lint && make test`)
- ✅ Integration tests covering all new resources
- ✅ Documentation updated (README, declarative.md, planning docs)
- ✅ Example configurations for all new resources
- ✅ E2E validation against real Konnect environment

### User Experience Requirements
- ✅ Consistent command patterns across all resources
- ✅ Interactive mode support for all get commands
- ✅ Multi-format output (json/yaml/text) for all commands
- ✅ Namespace isolation for all declarative resources
- ✅ Protection flags for all parent resources
- ✅ YAML tags (!file, !ref) working with all resources

---

## Next Steps

### Immediate Actions (Can Start Now)

1. **Phase 1: Event Gateway Instances**
   - ✅ Begin implementation (no blockers)
   - Verify SDK support
   - Implement declarative + imperative support
   - Test and validate

### Pending Clarifications (Required Before Starting)

2. **Service Catalog Scope Decision**
   - 🔴 **REQUIRED:** Get decision on which resources to support
   - Review API schemas once scope confirmed
   - Plan implementation approach

3. **Portal Management Gap Analysis**
   - 🟡 **REQUIRED:** Get decision on which gaps to fill
   - Prioritize based on user value
   - Plan incremental implementation

### Parallel Work (Can Progress Independently)

4. **API Builder Verification**
   - Audit field coverage
   - Test against real-world scenarios
   - Confirm specifications endpoint properly deprecated

5. **Documentation Preparation**
   - Start planning docs for Event Gateway
   - Prepare example configurations
   - Update architecture diagrams

---

## Timeline Estimate (Rough)

**Assumptions:**
- 1 developer working on implementation
- Scope decisions made within 1 week
- No major SDK or API issues discovered

| Phase | Estimated Duration | Dependencies |
|-------|-------------------|--------------|
| Phase 1: Event Gateway Instances | 1-2 weeks | None (can start now) |
| Phase 2: Service Catalog | 1-2 weeks | Scope clarification |
| Phase 3: Portal Management Gaps | 0.5-1 week | Scope clarification |
| Phase 4: API Builder Verification | 0.5 week | None (parallel) |
| Testing & Polish | 1 week | All phases complete |

**Total Estimated Duration:** 4-6 weeks from start to beta-ready

**Note:** Timeline assumes no major blockers, SDK issues, or scope changes.

---

## Appendix: API Endpoint Reference

### Event Gateway (v1) - Full API
**Base URL:** `https://us.api.konghq.com/v1`

**Event Gateway Instances (Phase 1 Scope):**
- `GET /event-gateways` - List event gateways
- `POST /event-gateways` - Create event gateway
- `GET /event-gateways/{gatewayId}` - Get event gateway
- `PUT /event-gateways/{gatewayId}` - Update event gateway
- `DELETE /event-gateways/{gatewayId}` - Delete event gateway

**Child Resources (Future Phases):**
- Backend Clusters: 4 endpoints
- Virtual Clusters: 4 endpoints
- Listeners: 4 endpoints
- Schema Registries: 4 endpoints
- Nodes: 4 endpoints (read-only)
- Data Plane Certificates: 4 endpoints
- Static Keys: 3 endpoints
- Listener Policies: 5 endpoints
- Virtual Cluster Policies: 15 endpoints (consume, produce, cluster)

**Total:** 51 endpoints

### Service Catalog (v1) - Full API
**Base URL:** `https://us.api.konghq.com/v1`

**Catalog Services:**
- `POST /catalog-services` - Create catalog service
- `GET /catalog-services` - List catalog services
- `GET /catalog-services/{id}` - Get catalog service
- `PATCH /catalog-services/{id}` - Update catalog service
- `DELETE /catalog-services/{id}` - Delete catalog service

**Integrations (Read-only):**
- `GET /integrations` - List available integrations

**Integration Instances:**
- `POST /integration-instances` - Create integration instance
- `GET /integration-instances` - List integration instances
- `GET /integration-instances/{id}` - Get integration instance
- `PATCH /integration-instances/{id}` - Update integration instance
- `DELETE /integration-instances/{id}` - Delete integration instance

**Resources:**
- `GET /resources` - List resources
- `GET /resources/{id}` - Get resource
- `PATCH /integration-instances/{instanceId}/resources/{resourceId}` - Update
  resource

**Resource Mappings:**
- `POST /resource-mappings` - Create resource mapping
- `GET /resource-mappings` - List resource mappings
- `GET /resource-mappings/{id}` - Get resource mapping
- `DELETE /resource-mappings/{id}` - Delete resource mapping

**Total:** 16 endpoints

---

**Document Status:** Draft - Pending Scope Clarifications
**Last Updated:** 2025-11-07
**Next Review:** After clarifications received
