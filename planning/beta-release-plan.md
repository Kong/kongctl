# Kongctl Beta Release - Implementation Work Items

**Date:** 2025-11-07
**Status:** Planning - Pending Clarifications
**Target:** Beta Release

---

## Summary

This document defines the **delta work** required to achieve a beta release of
kongctl. The focus is on what needs to be **added or changed**, not what
already exists.

### Work Items for Beta

1. **Event Gateway Support** - NEW (can start immediately)
2. **Service Catalog Support** - NEW (pending scope decision)
3. **Portal Management Gaps** - TBD (pending scope decision)
4. **API Builder Verification** - Audit only (likely no work needed)

### Out of Scope for Beta
- Control Planes Config (v2) / Core-entities → Deferred to Kong decK tool
- Post-beta items: System Accounts, Teams, Users, Org Auth, Team Mapping,
  Audit Logs, Analytics, Mesh

---

## Work Item 1: Event Gateway Support (NEW)

**Status:** 🟢 Can start immediately - No blockers

**Scope:** Event Gateway instances (control planes) only for initial beta.
Reevaluate child resources (backend clusters, virtual clusters, listeners, etc.)
after initial implementation.

### What Needs to Be Built

#### 1.1 Declarative Configuration Support

**New Resource:** `event_gateways` (parent resource)

**Files to Create:**
- `/internal/declarative/resources/eventgateway/eventgateway.go` - Resource
  handler
- `/internal/declarative/resources/eventgateway/eventgateway_test.go` - Unit
  tests
- Schema definition for YAML validation

**Implementation Tasks:**
- [ ] Define `EventGateway` resource struct mapping to API schema
- [ ] Implement `Create()` method - POST `/v1/event-gateways`
- [ ] Implement `Read()` method - GET `/v1/event-gateways/{gatewayId}`
- [ ] Implement `Update()` method - PUT `/v1/event-gateways/{gatewayId}`
- [ ] Implement `Delete()` method - DELETE `/v1/event-gateways/{gatewayId}`
- [ ] Implement `List()` method - GET `/v1/event-gateways`
- [ ] Add namespace isolation (KONGCTL-namespace label)
- [ ] Add protected flag support (KONGCTL-protected label)
- [ ] Add field validation
- [ ] Register resource in declarative engine
- [ ] Write unit tests

**API Endpoints (5 total):**
- `GET /v1/event-gateways` - List
- `POST /v1/event-gateways` - Create
- `GET /v1/event-gateways/{gatewayId}` - Read
- `PUT /v1/event-gateways/{gatewayId}` - Update
- `DELETE /v1/event-gateways/{gatewayId}` - Delete

#### 1.2 Imperative Command Support

**New Commands:** `kongctl get event-gateways` and `kongctl get event-gateway
<id|name>`

**Files to Create:**
- `/internal/cmd/root/products/konnect/eventgateway/eventgateway.go` - Command
  implementation
- `/internal/cmd/root/products/konnect/eventgateway/eventgateway_test.go` -
  Command tests

**Implementation Tasks:**
- [ ] Create `eventgateway` package under konnect products
- [ ] Implement `ListEventGateways()` function
- [ ] Implement `GetEventGateway()` function (by ID or name)
- [ ] Add JSON output format support
- [ ] Add YAML output format support
- [ ] Add text/table output format support
- [ ] Add interactive mode support (`-i` flag)
- [ ] Wire commands into `get` verb router
- [ ] Write command tests

#### 1.3 SDK Verification

**Tasks:**
- [ ] Verify `sdk-konnect-go` has Event Gateway v1 client
- [ ] Review Event Gateway schema in SDK
- [ ] Identify any SDK gaps or missing fields
- [ ] If SDK missing: Coordinate with SDK team or implement direct HTTP client

#### 1.4 Integration Testing

**Files to Create:**
- `/test/integration/eventgateway_test.go` - E2E integration tests

**Test Cases:**
- [ ] Create event gateway via declarative config
- [ ] Update event gateway via declarative config
- [ ] Delete event gateway via declarative config
- [ ] List event gateways via imperative command
- [ ] Get single event gateway via imperative command
- [ ] Test namespace isolation
- [ ] Test protected flag behavior
- [ ] Test output formats (json/yaml/text)

#### 1.5 Documentation

**Files to Update:**
- [ ] `docs/declarative.md` - Add event gateway resource documentation
- [ ] `README.md` - Add event gateway to feature list
- [ ] `docs/examples/declarative/event-gateway/` - Create example configs

**Example Configurations to Create:**
- [ ] Basic event gateway YAML config
- [ ] Event gateway with namespace and labels
- [ ] Protected event gateway example

### Estimated Effort
- **Declarative:** 3-4 days
- **Imperative:** 2-3 days
- **Testing & Docs:** 2-3 days
- **Total:** 1-2 weeks

---

## Work Item 2: Service Catalog Support (NEW)

**Status:** 🔴 BLOCKED - Pending scope decision

### Scope Decision Required

**Question:** Which Service Catalog resources should be supported for beta?

**Option 1 - Minimal (4 endpoints):**
- Catalog Services only
- Simplest, fastest implementation
- Limited functionality

**Option 2 - Standard (7-9 endpoints) - RECOMMENDED:**
- Catalog Services (4 endpoints)
- Resource Mappings (3 endpoints)
- Integrations (1 endpoint, read-only)
- Core catalog functionality + linking external resources

**Option 3 - Full (16 endpoints):**
- All resources: Catalog Services, Integrations, Integration Instances,
  Resources, Resource Mappings
- Most comprehensive
- Longer implementation time

### What Needs to Be Built (Assuming Standard Option)

#### 2.1 Declarative Configuration Support

**New Resources:**
- `catalog_services` (parent resource)
- `resource_mappings` (child resource, nested under catalog_services)

**Files to Create:**
- `/internal/declarative/resources/catalog/catalogservice.go`
- `/internal/declarative/resources/catalog/resourcemapping.go`
- `/internal/declarative/resources/catalog/catalogservice_test.go`
- `/internal/declarative/resources/catalog/resourcemapping_test.go`

**Implementation Tasks:**
- [ ] Define `CatalogService` resource struct
- [ ] Implement CRUD operations for catalog services (4 endpoints)
- [ ] Define `ResourceMapping` resource struct
- [ ] Implement CRUD operations for resource mappings (3 endpoints)
- [ ] Add namespace isolation and protection
- [ ] Register resources in declarative engine
- [ ] Write unit tests

#### 2.2 Imperative Command Support

**New Commands:**
- `kongctl get catalog-services`
- `kongctl get catalog-service <id|name>`
- `kongctl get catalog-service resource-mappings --service-id <id>`
- `kongctl get integrations` (read-only, informational)

**Files to Create:**
- `/internal/cmd/root/products/konnect/catalog/catalogservice.go`
- `/internal/cmd/root/products/konnect/catalog/resourcemapping.go`
- `/internal/cmd/root/products/konnect/catalog/integration.go`

**Implementation Tasks:**
- [ ] Implement list/get for catalog services
- [ ] Implement list/get for resource mappings (child resource)
- [ ] Implement list for integrations (read-only)
- [ ] Add output format support (json/yaml/text)
- [ ] Add interactive mode support
- [ ] Wire commands into `get` verb router
- [ ] Write command tests

#### 2.3 SDK Verification

**Tasks:**
- [ ] Verify `sdk-konnect-go` has Service Catalog v1 client
- [ ] Review Service Catalog schemas in SDK
- [ ] Identify any SDK gaps

#### 2.4 Integration Testing

**Files to Create:**
- `/test/integration/catalog_test.go`

**Test Cases:**
- [ ] Create catalog service via declarative
- [ ] Create resource mapping via declarative
- [ ] Update catalog service
- [ ] Delete catalog service
- [ ] Test imperative commands
- [ ] Test namespace isolation

#### 2.5 Documentation

**Files to Update/Create:**
- [ ] `docs/declarative.md` - Add catalog resources
- [ ] `README.md` - Add catalog to features
- [ ] `docs/examples/declarative/catalog/` - Example configs

### Estimated Effort (Standard Option)
- **Declarative:** 3-5 days
- **Imperative:** 2-3 days
- **Testing & Docs:** 2-3 days
- **Total:** 1-2 weeks

---

## Work Item 3: Portal Management Gaps (TBD)

**Status:** 🟡 BLOCKED - Pending scope decision

### Scope Decision Required

**Question:** Which Portal Management resources should be added for beta?

**Currently NOT Supported (Potential Additions):**

| Resource | Endpoints | Type | Notes |
|----------|-----------|------|-------|
| Application Registrations | 3 | GET only | View app registrations |
| Authentication Settings | 2 | GET/PATCH | Portal auth config |
| Identity Providers | 4 | CRUD | Portal IdP config |
| Team Group Mappings | 2 | GET/PATCH | Map IdP groups to teams |
| Assets (logo/favicon) | 6 | GET/PUT | Portal branding |
| Email Domains | 3 | CRUD | Email domain management |
| Email Delivery | 3 | GET/PATCH/DELETE | Email delivery config |
| Email Config | 4 | CRUD | Email configuration |
| Email Templates | 6 | CRUD | Custom email templates |
| Portal Audit Logs | 5 | GET/webhook | Portal audit trail |
| Portal Roles | 1 | GET | Available roles (read-only) |
| Default Content | 1 | POST | Initialize default content |

**Recommendation:** Clarify which resources are critical for beta based on user
workflows.

### What Needs to Be Built (TBD After Scope Decision)

**Implementation approach will depend on which resources are selected. Each
resource follows similar pattern to Work Items 1 and 2:**
1. Add declarative resource handler
2. Add imperative GET commands
3. SDK integration
4. Tests
5. Documentation

**Estimated effort:** 1-3 days per resource (depending on complexity)

---

## Work Item 4: API Builder Verification (Audit Only)

**Status:** 🟢 Can start immediately - Likely no implementation needed

### Verification Tasks

**Goal:** Confirm API Builder is complete for beta (likely already at 100%)

#### 4.1 Field Coverage Audit

**Tasks:**
- [ ] Review API Builder v3 API schema for all resource types
- [ ] Verify `apis` resource has all fields in declarative config
- [ ] Verify `api_versions` resource has all fields
- [ ] Verify `api_publications` resource has all fields
- [ ] Verify `api_implementations` resource has all fields
- [ ] Verify `api_documents` resource has all fields
- [ ] Verify `api_attributes` properly managed via `attributes` field
- [ ] Confirm `/apis/{apiId}/specifications` endpoint is properly ignored
  (deprecated)

#### 4.2 Integration Testing

**Tasks:**
- [ ] Test complete API Builder workflow end-to-end
- [ ] Create API with all fields populated
- [ ] Create version with spec file
- [ ] Create publication to portal
- [ ] Create implementation
- [ ] Create documents
- [ ] Verify all fields round-trip correctly
- [ ] Test imperative commands for all resources

#### 4.3 Documentation Review

**Tasks:**
- [ ] Verify `docs/declarative.md` documents all API Builder resources
- [ ] Verify examples exist for common API Builder scenarios
- [ ] Update any outdated references to deprecated specifications endpoint

### Estimated Effort
- **Audit & Testing:** 2-3 days
- **Doc updates (if needed):** 1 day
- **Total:** 3-4 days

**Expected Outcome:** Likely no implementation work needed, just confirmation
and minor doc updates.

---

## Implementation Phases

### Phase 1: Event Gateway (Can Start Now)
**Duration:** 1-2 weeks

1. SDK verification
2. Declarative implementation
3. Imperative implementation
4. Integration testing
5. Documentation

**Quality Gates:**
- `make build` - Success
- `make lint` - Zero issues
- `make test` - All pass
- `make test-integration` - All pass

### Phase 2: Service Catalog (After Scope Decision)
**Duration:** 1-2 weeks

1. Scope confirmation (BLOCKER)
2. SDK verification
3. Declarative implementation
4. Imperative implementation
5. Integration testing
6. Documentation

**Quality Gates:** Same as Phase 1

### Phase 3: Portal Management Gaps (After Scope Decision)
**Duration:** 0.5-1 week (varies by scope)

1. Scope confirmation (BLOCKER)
2. Incremental implementation per resource
3. Testing
4. Documentation

**Quality Gates:** Same as Phase 1

### Phase 4: API Builder Verification (Can Run in Parallel)
**Duration:** 3-4 days

1. Field coverage audit
2. End-to-end testing
3. Documentation updates (if needed)

**Quality Gates:** Testing only, no build/lint needed unless changes made

---

## Critical Blockers

### 🔴 BLOCKER 1: Service Catalog Scope

**Question:** Which Service Catalog resources should be supported for beta?

**Decision Needed From:** Decision maker / Product owner

**Options:**
1. Minimal: Catalog Services only (4 endpoints)
2. Standard: Catalog Services + Resource Mappings (7 endpoints) - RECOMMENDED
3. Full: All resources (16 endpoints)

**Impact:** Blocks Phase 2 start

**Recommendation:** Choose Option 2 (Standard) for balance of functionality and
implementation time.

---

### 🟡 BLOCKER 2: Portal Management Gaps

**Question:** Which Portal Management resources should be added for beta?

**Decision Needed From:** Decision maker / Product owner

**Resources to Consider:**
- Application Registrations (imperative GET only)
- Authentication Settings (declarative + imperative)
- Identity Providers (declarative + imperative)
- Team Group Mappings (declarative + imperative)
- Assets - logo/favicon (declarative + imperative)
- Email configuration (domains, delivery, config, templates)
- Portal Audit Logs (imperative GET only)
- Portal Roles (imperative GET only - read-only)
- Default Content (imperative POST only)

**Impact:** Blocks Phase 3 start

**Recommendation:** Clarify based on critical user workflows. Consider starting
with authentication settings and identity providers as they're core to portal
functionality.

---

## Timeline Estimate

**Assumptions:**
- 1 developer working on implementation
- Scope decisions made within 1 week
- No major SDK or API issues discovered

| Phase | Duration | Dependencies | Can Start |
|-------|----------|--------------|-----------|
| Phase 1: Event Gateway | 1-2 weeks | None | ✅ Immediately |
| Phase 2: Service Catalog | 1-2 weeks | Scope decision | ⚠️ Blocked |
| Phase 3: Portal Gaps | 0.5-1 week | Scope decision | ⚠️ Blocked |
| Phase 4: API Builder Audit | 3-4 days | None | ✅ Immediately (parallel) |
| Final Testing & Polish | 1 week | All phases complete | After phases 1-3 |

**Best Case (parallel work):** 3-4 weeks
**Realistic Case (sequential + blockers):** 4-6 weeks
**Worst Case (scope changes, SDK issues):** 6-8 weeks

---

## Success Criteria

### Functional Completeness
- [ ] Event Gateway instances: Full declarative + imperative support
- [ ] Service Catalog: Full support per scope decision
- [ ] Portal Management: Gaps filled per scope decision
- [ ] API Builder: Verified 100% complete

### Quality Standards
- [ ] All quality gates passing (`make build && make lint && make test &&
  make test-integration`)
- [ ] Integration tests for all new resources
- [ ] Documentation complete and accurate
- [ ] Example configurations for all new resources
- [ ] Manual E2E validation against live Konnect environment

### User Experience
- [ ] Consistent command patterns across all resources
- [ ] Interactive mode support (`-i`) for all commands
- [ ] Multi-format output (json/yaml/text) for all commands
- [ ] Namespace isolation for all declarative resources
- [ ] Protected flag support for all parent resources
- [ ] YAML tags (!file, !ref) working with all resources

---

## Next Actions

### Can Start Immediately ✅

1. **Begin Phase 1: Event Gateway Implementation**
   - No blockers
   - Verify SDK support
   - Start declarative implementation
   - Start imperative implementation

2. **Begin Phase 4: API Builder Verification**
   - Can run in parallel with Phase 1
   - Audit field coverage
   - Run integration tests

### Requires Decision 🔴

3. **Service Catalog Scope Decision**
   - Product owner decision required
   - Recommendation: Option 2 (Standard)
   - Needed to unblock Phase 2

4. **Portal Management Gaps Decision**
   - Product owner decision required
   - List of 11 potential resources provided above
   - Needed to unblock Phase 3

---

## Appendix: Current State (For Reference)

### Already Complete for Beta
- **Gateway Control Planes (v2):** 100% complete (declarative + imperative)
- **API Builder (v3):** ~100% complete (pending verification)
- **Portal Management (v3):** Core resources complete (portals, pages, snippets,
  customization, custom domains, developers, teams, applications)

### Not Complete for Beta
- **Event Gateway (v1):** 0% complete → **Work Item 1**
- **Service Catalog (v1):** 0% complete → **Work Item 2**
- **Portal Management (v3):** Gaps exist → **Work Item 3**

### Explicitly Out of Scope
- Control Planes Config (v2) - 97 endpoints for core entities → Deferred to
  decK tool
- Post-beta items: System Accounts, Teams, Users, Org Auth, Team Mapping,
  Audit Logs, Analytics, Mesh

---

**Document Status:** Draft - Ready for Review & Decision
**Last Updated:** 2025-11-07
**Next Review:** After scope decisions received from decision maker
