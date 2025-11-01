# E2E Test Scenario Coverage Analysis for Kongctl Declarative Configuration

**Generated**: 2025-11-01
**Scope**: Declarative configuration features (plan, apply, sync, diff, adopt, dump)
**Test Location**: `test/e2e/scenarios/`

---

## Executive Summary

This document provides a comprehensive field-level analysis of e2e test coverage for
kongctl's declarative configuration features. Based on analysis of 14 test scenarios,
testdata configurations, documentation, and examples, this report reveals **good
coverage of core workflows** but **significant gaps in field-level testing, error
scenarios, and advanced features**.

### Key Findings

- **Overall Coverage**: ~60% across all features
- **Commands**: 83% (5/6 tested) - `diff` command has ZERO coverage
- **Resources**: 89% (12/13 tested) - Portal Custom Domain not tested
- **Metadata Features**: 50% (1/2 tested) - `protected` field has ZERO e2e coverage
- **Field Coverage**: ~50% average across all resource types

### Critical Gaps

1. ⚠️ **protected field** - ZERO e2e coverage despite being documented
2. ⚠️ **Portal Custom Domain** - Entire resource type untested
3. ⚠️ **diff command** - ZERO coverage
4. ⚠️ **Plan artifact workflows** - Two-phase plan/apply not tested
5. ⚠️ **Error scenarios** - Field validation, tag errors largely untested

---

## Table of Contents

1. [Current Test Inventory](#section-1-current-test-inventory)
2. [Feature Coverage Matrix](#section-2-feature-coverage-matrix)
3. [Resource Field Coverage Analysis](#section-3-resource-field-coverage-analysis)
4. [Gap Analysis](#section-4-gap-analysis)
5. [Recommended Test Scenarios](#section-5-recommended-test-scenarios)
6. [Summary Statistics](#section-6-summary-statistics)

---

## Section 1: Current Test Inventory

### 1.1 Portal Scenarios (6 scenarios)

#### **portal/api_docs_with_children**

- **Location**: `test/e2e/scenarios/portal/api_docs_with_children/`
- **Purpose**: Test API documents with parent-child relationships
- **Commands**: apply
- **Resources**: Portal, API (SMS, Voice), API Versions, API Documents
- **Fields Exercised**:
  - Portal: name, display_name, description, authentication_enabled,
    rbac_enabled, auto_approve_developers, auto_approve_applications,
    default_api_visibility, default_page_visibility, customization (theme,
    layout, menu), pages (nested hierarchy), snippets
  - API: name, description, version, labels
  - API Documents: title, slug, status, content (via !file tag)
- **Metadata**: None tested
- **YAML Tags**: !file for content loading
- **Patterns**: Hierarchical (nested documents under APIs)

#### **portal/sync**

- **Location**: `test/e2e/scenarios/portal/sync/`
- **Purpose**: Test sync command deletion behavior
- **Commands**: sync
- **Resources**: Portal, API
- **Fields Exercised**:
  - Portal: name, display_name
  - API: name, version
- **Metadata**: None
- **YAML Tags**: !file
- **Patterns**: Hierarchical
- **Special**: Tests DELETE action when resources missing from config

#### **portal/visibility**

- **Location**: `test/e2e/scenarios/portal/visibility/`
- **Purpose**: Test publication visibility mutations
- **Commands**: apply
- **Resources**: Portal, API, API Publications
- **Fields Exercised**:
  - API Publication: visibility (public → private update)
- **Metadata**: None
- **YAML Tags**: !file
- **Patterns**: Hierarchical

#### **portal/default_application_auth_strategy**

- **Location**: `test/e2e/scenarios/portal/default_application_auth_strategy/`
- **Purpose**: Test linking portal to auth strategy
- **Commands**: apply
- **Resources**: Portal, Application Auth Strategy
- **Fields Exercised**:
  - Portal: name, display_name, description,
    default_application_auth_strategy_id (via reference)
  - Auth Strategy: name, display_name, strategy_type,
    configs (key_auth.key_names)
- **Metadata**: None
- **YAML Tags**: None
- **Patterns**: Basic

#### **portal/api_with_attributes**

- **Location**: `test/e2e/scenarios/portal/api_with_attributes/`
- **Purpose**: Test API attributes and slug fields
- **Commands**: apply
- **Resources**: API
- **Fields Exercised**:
  - API: attributes (owner, lifecycle), slug
- **Metadata**: None
- **YAML Tags**: None
- **Patterns**: Uses inputOverlayOps to inject fields

#### **portal/auth-strategy-link**

- **Location**: `test/e2e/scenarios/portal/auth-strategy-link/`
- **Purpose**: Test publication auth strategy linkage
- **Commands**: apply
- **Resources**: Portal, API, API Publications, Auth Strategies
- **Fields Exercised**:
  - API Publication: portal_id, visibility, auth_strategy_ids
  - Auth Strategy: name, display_name, strategy_type, configs
- **Metadata**: None
- **YAML Tags**: None
- **Patterns**: Hierarchical

---

### 1.2 Control Plane Scenarios (4 scenarios)

#### **control-plane/apply**

- **Location**: `test/e2e/scenarios/control-plane/apply/`
- **Purpose**: Basic control plane creation
- **Commands**: apply
- **Resources**: Control Plane
- **Fields Exercised**:
  - Control Plane: name, description, cluster_type
- **Metadata**: None
- **YAML Tags**: None
- **Patterns**: Basic

#### **control-plane/sync**

- **Location**: `test/e2e/scenarios/control-plane/sync/`
- **Purpose**: Test sync with control plane deletion
- **Commands**: sync
- **Resources**: Control Plane
- **Fields Exercised**:
  - Control Plane: name, description, cluster_type
- **Metadata**: None
- **YAML Tags**: None
- **Patterns**: Basic

#### **control-plane/groups**

- **Location**: `test/e2e/scenarios/control-plane/groups/`
- **Purpose**: Test control plane groups and membership
- **Commands**: apply, plan
- **Resources**: Control Plane (regular + groups)
- **Fields Exercised**:
  - Control Plane: name, description, cluster_type
    (CLUSTER_TYPE_CONTROL_PLANE_GROUP), auth_type, proxy_urls, members
- **Metadata**: namespace (via _defaults)
- **YAML Tags**: !ref for member IDs
- **Patterns**: Hierarchical, uses _defaults

#### **control-plane/sync-groups**

- **Location**: `test/e2e/scenarios/control-plane/sync-groups/`
- **Purpose**: Test sync with control plane groups
- **Commands**: sync
- **Resources**: Control Plane groups
- **Fields Exercised**: Same as control-plane/groups
- **Metadata**: namespace
- **YAML Tags**: !ref
- **Patterns**: Hierarchical

---

### 1.3 Adopt Scenarios (2 scenarios)

#### **adopt/full**

- **Location**: `test/e2e/scenarios/adopt/full/`
- **Purpose**: Test adopt command for all parent resources
- **Commands**: adopt, dump, plan
- **Resources**: Portal, API, Control Plane
- **Fields Exercised**: Basic fields only for creation
- **Metadata**: namespace (applied via adopt command)
- **YAML Tags**: None
- **Patterns**: Uses create spec, then adopt, then dump

#### **adopt/create-portal-adopt-dump-plan**

- **Location**: `test/e2e/scenarios/adopt/create-portal-adopt-dump-plan/`
- **Purpose**: Test adopt → dump → plan workflow
- **Commands**: adopt, dump, plan
- **Resources**: Portal
- **Fields Exercised**: name, display_name, authentication_enabled,
  rbac_enabled
- **Metadata**: namespace
- **YAML Tags**: None
- **Patterns**: Workflow validation

---

### 1.4 Namespace Scenarios (1 scenario)

#### **require-namespace/portal**

- **Location**: `test/e2e/scenarios/require-namespace/portal/`
- **Purpose**: Test namespace enforcement flags
- **Commands**: sync (with --require-namespace, --require-any-namespace)
- **Resources**: Portal
- **Fields Exercised**: Basic portal fields
- **Metadata**: namespace (explicit and via _defaults)
- **YAML Tags**: None
- **Patterns**: Tests enforcement failures and successes

---

### 1.5 External Resource Scenarios (1 scenario)

#### **external/api-impl**

- **Location**: `test/e2e/scenarios/external/api-impl/`
- **Purpose**: Test external resource references and API implementations
- **Commands**: apply
- **Resources**: Control Plane, Gateway Service, API, API Implementation
- **Fields Exercised**:
  - Gateway Service: name, url
  - API Implementation: service (control_plane_id, id via !ref)
  - External: _external.selector.matchFields
- **Metadata**: None
- **YAML Tags**: !ref for cross-resource IDs
- **Patterns**: External resource pattern

---

## Section 2: Feature Coverage Matrix

### 2.1 Command Coverage

| Command | Tested? | Scenarios | Coverage Notes |
|---------|---------|-----------|----------------|
| **plan** | ✅ Yes | control-plane/groups, adopt/full, adopt/create-portal-adopt-dump-plan | Only with --mode apply; --mode sync not explicitly tested in isolation |
| **apply** | ✅ Yes | 9 scenarios | Well covered across resource types |
| **sync** | ✅ Yes | portal/sync, control-plane/sync, control-plane/sync-groups, require-namespace/portal | Deletion behavior tested |
| **diff** | ❌ No | None | **NOT tested in e2e scenarios** |
| **adopt** | ✅ Yes | adopt/full, adopt/create-portal-adopt-dump-plan | Tested for portal, API, control plane |
| **dump** | ✅ Yes | adopt/full, adopt/create-portal-adopt-dump-plan | Only declarative format tested |

#### Command Coverage Gaps

- **diff command**: ZERO e2e coverage
- **dump tf-import format**: Not tested
- **Plan artifact execution**: `apply --plan`, `sync --plan` not tested
- **dry-run flag**: Not tested for apply or sync

---

### 2.2 Resource Type Coverage

| Resource Type | Parent/Child | Tested? | Scenarios | Field Coverage |
|---------------|--------------|---------|-----------|----------------|
| **Portal** | Parent | ✅ Yes | 7 scenarios | Good: ~12/20 fields (60%) |
| **API** | Parent | ✅ Yes | 6 scenarios | Moderate: ~7/15 fields (47%) |
| **Application Auth Strategy** | Parent | ✅ Yes | 2 scenarios | Limited: ~5/10 fields (50%) |
| **Control Plane** | Parent | ✅ Yes | 4 scenarios | Limited: ~7/17 fields (41%) |
| **API Version** | Child | ✅ Yes | 3 scenarios | Basic: ~3/6 fields (50%) |
| **API Publication** | Child | ✅ Yes | 3 scenarios | Moderate: ~4/8 fields (50%) |
| **API Implementation** | Child | ✅ Yes | 1 scenario | Basic: ~2/5 fields (40%) |
| **API Document** | Child | ✅ Yes | 2 scenarios | Moderate: ~5/7 fields (71%) |
| **Portal Customization** | Child | ✅ Yes | 1 scenario | Moderate: ~4/9 fields (44%) |
| **Portal Page** | Child | ✅ Yes | 1 scenario | Good: ~6/8 fields (75%) |
| **Portal Snippet** | Child | ✅ Yes | 1 scenario | Good: ~5/5 fields (100%) |
| **Portal Custom Domain** | Child | ❌ No | None | **NOT tested (0%)** |
| **Gateway Service** | Child | ✅ Yes | 1 scenario | Minimal: ~2/8 fields (25%) |

#### Resource Coverage Gaps

- **Portal Custom Domain**: ZERO coverage (entire resource untested)
- Many child resources tested only in 1 scenario
- Field-level coverage averages only ~50% across all resources

---

### 2.3 Kongctl Metadata Coverage

| Feature | Tested? | Scenarios | Coverage Level |
|---------|---------|-----------|----------------|
| **namespace** (explicit) | ✅ Yes | require-namespace/portal, adopt scenarios | Good |
| **namespace** (via _defaults) | ✅ Yes | control-plane/groups, control-plane/sync-groups | Good |
| **namespace** (inheritance) | ✅ Yes | Various child resource scenarios | Implicit |
| **protected** | ❌ No | None | **ZERO e2e coverage** |
| **--require-namespace** flag | ✅ Yes | require-namespace/portal | Good |
| **--require-any-namespace** flag | ✅ Yes | require-namespace/portal | Good |

#### Metadata Coverage Gap

⚠️ **CRITICAL**: The `protected` field has NO e2e testing despite being documented
in `docs/declarative.md` (lines 278-289) and having examples in
`docs/examples/declarative/protected/`. This is a production feature that prevents
accidental resource deletion.

---

### 2.4 YAML Tag Coverage

| Tag | Syntax | Tested? | Scenarios | Coverage |
|-----|--------|---------|-----------|----------|
| **!file** | Path only | ✅ Yes | portal/api_docs_with_children, portal/sync | Good |
| **!file** | Hash extraction (#) | ✅ Yes | portal/api_docs_with_children | Good |
| **!file** | Map format (path + extract) | ❌ No | None | **ZERO coverage** |
| **!ref** | Resource ID reference | ✅ Yes | control-plane/groups, external/api-impl | Good |
| **!ref** | Field extraction (#) | ✅ Yes | control-plane/groups, external/api-impl | Good |

#### YAML Tag Gaps

- **!file map format** (documented in `docs/declarative.md` lines 398-405) not tested:
  ```yaml
  name: !file
    path: ./specs/api.yaml
    extract: info.title
  ```
- **Error scenarios** not tested:
  - Missing file
  - Path traversal attempts
  - File size > 10MB
  - Invalid extraction paths
  - Circular !ref dependencies

---

### 2.5 Configuration Pattern Coverage

| Pattern | Tested? | Scenarios | Notes |
|---------|---------|-----------|-------|
| **Hierarchical** (nested children) | ✅ Yes | portal/api_docs_with_children, portal/sync | Well covered |
| **Flat** (root-level children) | ⚠️ Partial | Some API children tested at root | Limited coverage |
| **_defaults section** | ✅ Yes | control-plane/groups | Only kongctl metadata tested |
| **Mixed** (some nested, some flat) | ❌ No | None | Not tested |

---

## Section 3: Resource Field Coverage Analysis

### 3.1 Portal Resource

**Fields TESTED** (12/20 = 60%):
- ✅ ref
- ✅ name
- ✅ display_name
- ✅ description
- ✅ authentication_enabled
- ✅ rbac_enabled
- ✅ auto_approve_developers
- ✅ auto_approve_applications
- ✅ default_api_visibility
- ✅ default_page_visibility
- ✅ default_application_auth_strategy_id (via reference)
- ✅ customization (theme, layout, menu)

**Fields NOT TESTED** (~8 fields):
- ❌ labels (general purpose labels)
- ❌ published_spec_ids
- ❌ rbac_team_sync_enabled
- ❌ auto_approve_registrations
- ❌ published_spec_labels
- ❌ Other portal-level configuration options

**Child Resources**:
- ✅ pages (well tested with nested hierarchy)
- ✅ snippets (good coverage)
- ✅ customization (good coverage)
- ❌ custom_domain (**ZERO coverage**)

---

### 3.2 API Resource

**Fields TESTED** (7/15 = 47%):
- ✅ ref
- ✅ name
- ✅ description
- ✅ version
- ✅ labels
- ✅ attributes (owner, lifecycle)
- ✅ slug

**Fields NOT TESTED** (~8 fields):
- ❌ deprecated
- ❌ team_id
- ❌ published_spec_ids
- ❌ published_spec_labels
- ❌ Comprehensive attributes (only tested owner + lifecycle)
- ❌ Other metadata fields

**Child Resources**:
- ✅ versions (basic coverage)
- ✅ publications (moderate coverage)
- ✅ implementations (minimal coverage)
- ✅ documents (moderate coverage)

---

### 3.3 Application Auth Strategy

**Fields TESTED** (5/10 = 50%):
- ✅ ref
- ✅ name
- ✅ display_name
- ✅ strategy_type (key_auth, openid_connect)
- ✅ configs.key_auth (key_names)

**Fields NOT TESTED** (~5 fields):
- ❌ labels
- ❌ dcr_providers (for OIDC)
- ❌ Full OIDC configuration options
- ❌ Other strategy types
- ❌ Complex auth configurations

---

### 3.4 Control Plane

**Fields TESTED** (7/17 = 41%):
- ✅ ref
- ✅ name
- ✅ description
- ✅ cluster_type (CONTROL_PLANE, CONTROL_PLANE_GROUP)
- ✅ auth_type (pinned_client_certs)
- ✅ proxy_urls (host, port, protocol)
- ✅ members (for groups)

**Fields NOT TESTED** (~10 fields):
- ❌ labels (general purpose)
- ❌ cloud_gateway settings
- ❌ autoscale configuration
- ❌ network configuration
- ❌ Advanced proxy settings
- ❌ Telemetry settings
- ❌ Other auth_type values
- ❌ Other cluster configurations

**Child Resources**:
- ✅ gateway_services (minimal coverage - only 2/8 fields)

---

### 3.5 API Version

**Fields TESTED** (3/6 = 50%):
- ✅ ref
- ✅ version
- ✅ spec (via !file)

**Fields NOT TESTED** (~3 fields):
- ❌ deprecated
- ❌ notify
- ❌ labels

---

### 3.6 API Publication

**Fields TESTED** (4/8 = 50%):
- ✅ ref
- ✅ portal_id
- ✅ visibility
- ✅ auth_strategy_ids

**Fields NOT TESTED** (~4 fields):
- ❌ auto_approve_registrations
- ❌ application_registration_enabled
- ❌ deprecated
- ❌ labels

---

### 3.7 API Implementation

**Fields TESTED** (2/5 = 40%):
- ✅ ref
- ✅ service (control_plane_id, id)

**Fields NOT TESTED** (~3 fields):
- ❌ Various implementation configuration options
- ❌ Complex service configurations

---

### 3.8 API Document

**Fields TESTED** (5/7 = 71%):
- ✅ ref
- ✅ title
- ✅ slug
- ✅ status
- ✅ content

**Fields NOT TESTED** (~2 fields):
- ❌ parent_document_id (complex hierarchies beyond basic nesting)
- ❌ Additional metadata fields

---

### 3.9 Portal Page

**Fields TESTED** (6/8 = 75%):
- ✅ ref
- ✅ slug
- ✅ title
- ✅ description
- ✅ visibility
- ✅ status
- ✅ content
- ✅ children (nested hierarchy)

**Fields NOT TESTED** (~2 fields):
- ❌ parent_page_id (when defined at root)
- ❌ Advanced page configurations

---

### 3.10 Portal Snippet

**Fields TESTED** (5/5 = 100%):
- ✅ ref
- ✅ name
- ✅ title
- ✅ description
- ✅ visibility
- ✅ status
- ✅ content

**Coverage**: Excellent - all basic fields covered

---

### 3.11 Portal Customization

**Fields TESTED** (4/9 = 44%):
- ✅ ref
- ✅ theme (mode, colors.primary)
- ✅ layout
- ✅ menu (main, footer_sections)

**Fields NOT TESTED** (~5 fields):
- ❌ Many theme customization options
- ❌ Advanced layout settings
- ❌ Custom CSS/branding options
- ❌ Additional customization fields

---

### 3.12 Portal Custom Domain

**Fields TESTED** (0/ALL = 0%):
- ❌ **ENTIRE RESOURCE TYPE NOT TESTED**

**Fields NOT TESTED** (ALL):
- ❌ ref
- ❌ domain
- ❌ certificate settings
- ❌ All configuration options

---

### 3.13 Gateway Service

**Fields TESTED** (2/8 = 25%):
- ✅ ref
- ✅ name
- ✅ url

**Fields NOT TESTED** (~6 fields):
- ❌ protocol
- ❌ host
- ❌ port
- ❌ path
- ❌ retries
- ❌ connect_timeout, read_timeout, write_timeout

---

## Section 4: Gap Analysis

### 4.1 Command Coverage Gaps

#### HIGH PRIORITY

**1. diff command** - ZERO coverage
- No tests for preview functionality
- No tests for diff with configuration files
- No tests for diff with plan artifacts
- No tests for diff output validation

**2. Plan artifact workflows** - NOT TESTED
- Two-phase workflow not validated:
  ```bash
  kongctl plan -f config.yaml --output-file plan.json
  kongctl diff --plan plan.json
  kongctl apply --plan plan.json
  ```
- Plan storage and deferred execution not tested
- Plan artifacts for audit trail not validated

**3. dry-run flag** - NOT TESTED
- `kongctl apply -f config.yaml --dry-run`
- `kongctl sync -f config.yaml --dry-run`
- No validation that --dry-run prevents actual changes

#### MEDIUM PRIORITY

**4. dump formats** - Partial coverage
- `dump tf-import` format not tested
- Multi-resource dump scenarios limited
- Only basic declarative format tested

---

### 4.2 Resource Type Gaps

#### CRITICAL

**1. Portal Custom Domain** - ZERO coverage
- No scenarios test custom domain configuration
- No tests for domain validation
- No tests for certificate management
- No tests for domain lifecycle (create, update, delete)

#### HIGH PRIORITY

**2. Protected resources** - ZERO e2e coverage
- Feature documented in `docs/declarative.md` (lines 278-289)
- Example exists in `docs/examples/declarative/protected/`
- NO e2e tests verify:
  - `protected: true` prevents deletion
  - Error messages when attempting to delete protected resources
  - Unprotecting before deletion workflow
  - Sync behavior with protected resources

**3. Complex control plane configurations**
- Only basic auth_type tested (pinned_client_certs)
- cloud_gateway settings not tested
- Network configurations not tested
- Autoscale settings not tested

**4. API advanced features**
- `deprecated` field not tested
- `team_id` associations not tested
- Complex attribute combinations not tested

---

### 4.3 Field Coverage Gaps by Resource

| Resource | Fields Tested | Fields Not Tested | Coverage % |
|----------|---------------|-------------------|------------|
| Portal | 12 | 8 | 60% |
| API | 7 | 8 | 47% |
| Auth Strategy | 5 | 5 | 50% |
| Control Plane | 7 | 10 | 41% |
| API Version | 3 | 3 | 50% |
| API Publication | 4 | 4 | 50% |
| API Implementation | 2 | 3 | 40% |
| API Document | 5 | 2 | 71% |
| Portal Page | 6 | 2 | 75% |
| Portal Snippet | 5 | 0 | 100% |
| Portal Customization | 4 | 5 | 44% |
| Portal Custom Domain | 0 | ALL | **0%** |
| Gateway Service | 2 | 6 | 25% |

**Average Field Coverage**: ~50%

---

### 4.4 Metadata Feature Gaps

#### CRITICAL

**1. protected field** - ZERO coverage

This is the most critical gap. The `protected` field is documented and has examples,
but has NO e2e validation.

**Missing Test Scenarios**:
- Protected resource deletion attempt (should fail with clear error)
- Unprotecting then deleting (should succeed)
- Sync with protected resources present (should skip deletion)
- _defaults.kongctl.protected inheritance
- Error message validation for protected resources

**Example from docs** (`docs/declarative.md` lines 283-288):
```yaml
portals:
  - ref: production-portal
    name: "Production Portal"
    kongctl:
      protected: true  # Cannot be deleted until protection is removed
```

This feature prevents accidental deletion of critical production resources but
has no automated testing.

---

### 4.5 YAML Tag Gaps

#### HIGH PRIORITY

**1. !file map format** - NOT TESTED

Documented syntax not tested (`docs/declarative.md` lines 398-405):
```yaml
apis:
  - ref: products-api
    name: !file
      path: ./specs/products.yaml
      extract: info.title
```

**2. Error scenarios** - NOT TESTED
- Missing file (should error gracefully with clear message)
- Path traversal attempts (should be blocked for security)
- File size > 10MB (should error with size limit message)
- Invalid extraction path (should error with clear message)
- Circular !ref dependencies (should be detected and prevented)

**3. !ref error scenarios** - NOT TESTED
- Reference to non-existent resource
- Invalid field extraction syntax
- Type mismatches in references

---

### 4.6 Configuration Pattern Gaps

#### MEDIUM PRIORITY

**1. Flat configuration** - Limited coverage
- Child resources defined at root level alongside parents
- Only partially tested with some API children
- Full flat pattern not comprehensively validated

**2. Mixed patterns** - NOT TESTED
- Some children nested under parents, others at root level
- Real-world complex multi-file configurations
- Pattern that teams might actually use in production

**3. _defaults section** - Limited testing
- Only `kongctl` metadata tested in _defaults
- Other potential default fields not explored
- Multi-level defaults not tested

---

### 4.7 Error Validation Gaps

#### HIGH PRIORITY

**1. Field validation errors** - Minimal coverage

Documented in `docs/declarative.md` (lines 778-796) - shows typo detection:
```yaml
portals:
  - ref: my-portal
    name: "My Portal"
    lables:  # ❌ ERROR: Unknown field 'lables'. Did you mean 'labels'?
      team: platform
```

**Common typos that should be tested**:
- `lables` → `labels`
- `descriptin` → `description`
- `displayname` → `display_name`
- `strategytype` → `strategy_type`

**NO e2e tests verify these validation errors.**

**2. Namespace enforcement errors**
- Only basic enforcement tested in require-namespace/portal
- Edge cases not covered:
  - Empty namespace (should error)
  - Very long namespaces
  - Special characters in namespaces

**3. Reference validation errors**
- Invalid references not tested
- Circular dependencies not tested
- Forward references not validated

**4. YAML syntax errors**
- Malformed YAML not tested
- Invalid tag usage not tested
- Schema validation errors not tested

---

## Section 5: Recommended Test Scenarios

### 5.1 HIGH PRIORITY Scenarios

These scenarios address critical gaps in test coverage that could impact
production usage.

---

#### Scenario 1: protected-resources-workflow

**Purpose**: Verify protected field prevents accidental deletion
**Priority**: 🔴 HIGH
**Rationale**: Feature is documented and has examples but ZERO e2e coverage

**Commands**: apply, sync
**Resources**: Portal, API (with protected: true/false)

**Test Steps**:
1. Apply configuration with resources marked `protected: true`
2. Verify resources are created with KONGCTL-protected label
3. Attempt sync with config missing protected resources (should FAIL)
4. Verify clear error message about protected resources
5. Update configuration to set `protected: false`
6. Apply the update
7. Verify KONGCTL-protected label is removed
8. Sync with resources missing (should succeed with deletion)

**Fields to Exercise**:
- Portal: name, display_name, kongctl.protected, kongctl.namespace
- API: name, description, kongctl.protected, kongctl.namespace

**Configuration Pattern**:
```yaml
_defaults:
  kongctl:
    namespace: test-protected
    protected: true

portals:
  - ref: critical-portal
    name: "Production Portal"
    display_name: "Critical Production Portal"
    # Inherits protected: true from defaults

apis:
  - ref: test-api
    name: "Test API"
    kongctl:
      protected: false  # Override default
```

**Expected Behavior**:
- Portal with `protected: true` cannot be deleted by sync
- API with `protected: false` can be deleted by sync
- Clear error message identifies protected resources by name

---

#### Scenario 2: diff-command-coverage

**Purpose**: Test diff command functionality
**Priority**: 🔴 HIGH
**Rationale**: Command has ZERO e2e coverage despite being core feature

**Commands**: diff, apply
**Resources**: Portal, API with modifications

**Test Steps**:
1. Apply initial configuration (portal + API)
2. Modify configuration (change portal display_name, add new API)
3. Run `kongctl diff -f modified-config.yaml`
4. Verify diff output shows:
   - UPDATE for portal (display_name change)
   - CREATE for new API
5. Run `kongctl diff --plan plan.json` with saved plan
6. Verify diff output matches plan contents
7. Verify no changes are made to Konnect (diff is read-only)

**Fields to Exercise**:
- Portal: name, display_name, description, authentication_enabled
- API: name, description, version

**Diff Output Validation**:
- Verify CREATE operations shown for new resources
- Verify UPDATE operations shown with field-level diffs
- Verify DELETE operations shown when using sync mode
- Verify output is human-readable

---

#### Scenario 3: plan-artifact-workflow

**Purpose**: Test two-phase plan generation and execution
**Priority**: 🔴 HIGH
**Rationale**: Documented workflow pattern not validated in e2e tests

**Commands**: plan, apply (with --plan), sync (with --plan), diff (with --plan)
**Resources**: Portal, API, Control Plane

**Test Steps**:

**Phase 1: Plan Generation**
1. Create configuration with portal, API, control plane
2. Generate plan: `kongctl plan -f config.yaml --output-file plan.json`
3. Verify plan.json file is created
4. Verify plan.json is valid JSON
5. Verify plan contains expected operations

**Phase 2: Plan Review**
6. Review plan: `kongctl diff --plan plan.json`
7. Verify diff output shows planned changes
8. Verify no changes made to Konnect

**Phase 3: Plan Execution**
9. Execute plan: `kongctl apply --plan plan.json`
10. Verify resources created successfully
11. Verify resource state matches plan

**Phase 4: Sync with Plan**
12. Generate sync plan (with deletions)
13. Execute sync plan: `kongctl sync --plan plan.json`
14. Verify deletions occur as planned

**Fields to Exercise**: Standard fields for all resources

**Plan Artifact Validation**:
- Plan file is valid JSON
- Plan contains operation types (CREATE, UPDATE, DELETE)
- Plan contains resource details
- Plan can be stored, versioned, and executed later

---

#### Scenario 4: portal-custom-domain

**Purpose**: Test portal custom domain configuration
**Priority**: 🔴 HIGH
**Rationale**: Entire resource type has ZERO coverage

**Commands**: apply, sync
**Resources**: Portal, Portal Custom Domain

**Test Steps**:
1. Create portal
2. Add custom domain configuration:
   - domain name
   - certificate settings
3. Verify custom domain is configured
4. Update custom domain settings
5. Verify updates applied
6. Remove custom domain from config
7. Sync (should delete custom domain)
8. Verify custom domain removed

**Fields to Exercise**:
- Portal Custom Domain: domain, certificate configuration, all fields
- Portal: name, display_name (parent reference)

**Configuration Pattern**:
```yaml
portals:
  - ref: custom-domain-portal
    name: "Custom Domain Portal"
    display_name: "Portal with Custom Domain"

    custom_domains:
      - ref: main-domain
        domain: "api.example.com"
        # Additional certificate/configuration fields
```

---

#### Scenario 5: file-tag-error-scenarios

**Purpose**: Test !file tag error handling
**Priority**: 🔴 HIGH
**Rationale**: Error cases not validated, security implications

**Commands**: apply (expected to fail with clear errors)
**Resources**: API with !file tags (error cases)

**Test Steps**:

**Test 1: Missing File**
1. Configuration references non-existent file
2. Attempt apply
3. Verify clear error message with file path
4. Verify no partial resources created

**Test 2: Path Traversal Attempt**
1. Configuration attempts path traversal: `!file ../../../etc/passwd`
2. Attempt apply
3. Verify security error, path traversal blocked
4. Verify no file access occurred

**Test 3: File Size Limit**
1. Configuration references file > 10MB
2. Attempt apply
3. Verify file size limit error
4. Verify file size shown in error

**Test 4: Invalid Extraction Path**
1. Configuration uses invalid extraction: `!file ./spec.yaml#nonexistent.field`
2. Attempt apply
3. Verify clear error about missing field
4. Verify field path shown in error

**Error Message Requirements**:
- Clear, actionable error messages
- Include file paths in errors
- Security errors don't leak sensitive info
- Suggest corrections where possible

---

### 5.2 MEDIUM PRIORITY Scenarios

These scenarios improve field-level coverage and test additional patterns.

---

#### Scenario 6: comprehensive-field-coverage-portal

**Purpose**: Test all Portal fields comprehensively
**Priority**: 🟡 MEDIUM
**Rationale**: Current coverage is 60% (12/20 fields)

**Commands**: apply, sync
**Resources**: Portal with comprehensive field set

**Fields to Exercise** (in addition to already tested):
- ✅ Already tested: name, display_name, description, authentication_enabled,
  rbac_enabled, auto_approve_developers, auto_approve_applications,
  default_api_visibility, default_page_visibility,
  default_application_auth_strategy_id, customization
- 🆕 **Additional fields to test**:
  - labels (general purpose key-value labels)
  - published_spec_ids
  - published_spec_labels
  - rbac_team_sync_enabled
  - auto_approve_registrations

**Configuration Example**:
```yaml
portals:
  - ref: comprehensive-portal
    name: "comprehensive-test-portal"
    display_name: "Comprehensive Test Portal"
    description: "Portal testing all fields"
    authentication_enabled: true
    rbac_enabled: true
    rbac_team_sync_enabled: true
    auto_approve_developers: false
    auto_approve_applications: false
    auto_approve_registrations: true
    default_api_visibility: "private"
    default_page_visibility: "private"
    labels:
      environment: "test"
      team: "platform"
      cost-center: "engineering"
    published_spec_ids: []
    published_spec_labels:
      spec-type: "openapi"
```

---

#### Scenario 7: comprehensive-field-coverage-api

**Purpose**: Test all API fields comprehensively
**Priority**: 🟡 MEDIUM
**Rationale**: Current coverage is 47% (7/15 fields)

**Commands**: apply, sync
**Resources**: API with comprehensive field set

**Fields to Exercise** (in addition to already tested):
- ✅ Already tested: name, description, version, labels, attributes (owner,
  lifecycle), slug
- 🆕 **Additional fields to test**:
  - deprecated (boolean flag)
  - team_id (team association)
  - published_spec_ids
  - published_spec_labels
  - Comprehensive attributes set (beyond owner + lifecycle)

**Configuration Example**:
```yaml
apis:
  - ref: comprehensive-api
    name: "Comprehensive Test API"
    description: "API testing all fields"
    version: "1.0.0"
    slug: "comprehensive-test-api"
    deprecated: false
    labels:
      environment: "production"
      service: "billing"
    attributes:
      owner: "platform-team"
      lifecycle: "production"
      sla: "99.9"
      documentation: "https://docs.example.com"
      support: "platform@example.com"
    published_spec_ids: []
    published_spec_labels:
      format: "openapi-3.0"
```

---

#### Scenario 8: comprehensive-field-coverage-control-plane

**Purpose**: Test all Control Plane fields comprehensively
**Priority**: 🟡 MEDIUM
**Rationale**: Current coverage is 41% (7/17 fields)

**Commands**: apply, sync
**Resources**: Control Plane with comprehensive field set

**Fields to Exercise** (in addition to already tested):
- ✅ Already tested: name, description, cluster_type, auth_type
  (pinned_client_certs), proxy_urls, members
- 🆕 **Additional fields to test**:
  - labels
  - cloud_gateway settings
  - autoscale configuration
  - Network configuration options
  - Alternative auth_type values
  - Telemetry settings
  - Additional cluster configurations

**Configuration Example**:
```yaml
control_planes:
  - ref: comprehensive-cp
    name: "comprehensive-control-plane"
    description: "Control plane testing all fields"
    cluster_type: "CLUSTER_TYPE_CONTROL_PLANE"
    labels:
      environment: "production"
      region: "us-west-2"
    # Add additional fields as they're identified in SDK
```

---

#### Scenario 9: auth-strategy-variants

**Purpose**: Test all auth strategy types and configurations
**Priority**: 🟡 MEDIUM
**Rationale**: Current coverage is 50% (5/10 fields)

**Commands**: apply, sync
**Resources**: Application Auth Strategies (all types)

**Strategy Types to Test**:
- ✅ Already tested: key_auth, openid_connect (basic)
- 🆕 **Additional testing**:
  - Full OIDC configuration options
  - DCR providers
  - labels field
  - Other strategy types if available

**Configuration Example**:
```yaml
application_auth_strategies:
  - ref: key-auth-strategy
    name: "API Key Authentication"
    display_name: "API Key Auth"
    strategy_type: "key_auth"
    configs:
      key_auth:
        key_names: ["apikey", "x-api-key"]
    labels:
      auth-type: "api-key"

  - ref: oidc-strategy
    name: "OpenID Connect"
    display_name: "OIDC Authentication"
    strategy_type: "openid_connect"
    configs:
      openid_connect:
        issuer: "https://auth.example.com"
        scopes: ["openid", "profile", "email"]
        # Additional OIDC configuration
    dcr_providers: []
    labels:
      auth-type: "oidc"
```

---

#### Scenario 10: api-publication-full-fields

**Purpose**: Test all API publication fields
**Priority**: 🟡 MEDIUM
**Rationale**: Current coverage is 50% (4/8 fields)

**Commands**: apply, sync
**Resources**: API, Portal, API Publications

**Fields to Exercise** (in addition to already tested):
- ✅ Already tested: portal_id, visibility, auth_strategy_ids
- 🆕 **Additional fields to test**:
  - auto_approve_registrations
  - application_registration_enabled
  - deprecated
  - labels

**Configuration Example**:
```yaml
portals:
  - ref: pub-test-portal
    name: "Publication Test Portal"
    display_name: "Publication Test"

apis:
  - ref: pub-test-api
    name: "Publication Test API"

    publications:
      - ref: comprehensive-pub
        portal_id: pub-test-portal
        visibility: "public"
        auth_strategy_ids: []
        auto_approve_registrations: true
        application_registration_enabled: true
        deprecated: false
        labels:
          visibility: "public"
          registration: "open"
```

---

#### Scenario 11: gateway-service-comprehensive

**Purpose**: Test all Gateway Service fields
**Priority**: 🟡 MEDIUM
**Rationale**: Current coverage is 25% (2/8 fields)

**Commands**: apply
**Resources**: Control Plane, Gateway Service

**Fields to Exercise** (in addition to already tested):
- ✅ Already tested: name, url
- 🆕 **Additional fields to test**:
  - protocol, host, port, path
  - retries
  - connect_timeout, read_timeout, write_timeout
  - All service configuration options

**Configuration Example**:
```yaml
control_planes:
  - ref: service-test-cp
    name: "service-test-cp"
    description: "Control plane for gateway service testing"

    gateway_services:
      - ref: comprehensive-service
        name: "comprehensive-gateway-service"
        protocol: "https"
        host: "api.example.com"
        port: 443
        path: "/v1"
        retries: 5
        connect_timeout: 60000
        read_timeout: 60000
        write_timeout: 60000
```

---

#### Scenario 12: flat-configuration-pattern

**Purpose**: Test all child resources defined at root level
**Priority**: 🟡 MEDIUM
**Rationale**: Flat pattern only partially tested

**Commands**: apply, sync
**Resources**: APIs, API Versions, API Publications, API Implementations,
API Documents (all at root level)

**Configuration Pattern**:
```yaml
# All parent resources
portals:
  - ref: flat-portal
    name: "Flat Pattern Portal"

apis:
  - ref: flat-api
    name: "Flat Pattern API"

# All child resources at root with parent references
api_versions:
  - ref: flat-version
    api: flat-api  # Reference to parent
    version: "1.0.0"

api_publications:
  - ref: flat-pub
    api: flat-api  # Reference to parent
    portal: flat-portal  # Reference to portal

api_documents:
  - ref: flat-doc
    api: flat-api  # Reference to parent
    title: "Flat Pattern Document"
```

**Validation**:
- Verify all resources created successfully
- Verify parent-child relationships established correctly
- Verify flat pattern works identically to hierarchical

---

#### Scenario 13: mixed-configuration-pattern

**Purpose**: Test mixed nested and flat configuration patterns
**Priority**: 🟡 MEDIUM
**Rationale**: Real-world usage may mix patterns

**Commands**: apply, sync
**Resources**: Mix of nested and root-level child resources

**Configuration Pattern**:
```yaml
portals:
  - ref: mixed-portal
    name: "Mixed Pattern Portal"

    # Some children nested
    pages:
      - ref: nested-page
        title: "Nested Page"

apis:
  - ref: mixed-api
    name: "Mixed Pattern API"

    # Some children nested
    versions:
      - ref: nested-version
        version: "1.0.0"

# Some children at root
api_publications:
  - ref: root-pub
    api: mixed-api
    portal: mixed-portal

portal_snippets:
  - ref: root-snippet
    portal: mixed-portal
    name: "Root Snippet"
```

**Validation**:
- Verify both patterns work together
- Verify no conflicts between nested and flat
- Verify sync handles both patterns correctly

---

#### Scenario 14: file-tag-map-format

**Purpose**: Test !file map format syntax
**Priority**: 🟡 MEDIUM
**Rationale**: Documented syntax not tested

**Commands**: apply
**Resources**: API using !file map format

**Configuration Example**:
```yaml
apis:
  - ref: map-format-api
    name: !file
      path: ./specs/api.yaml
      extract: info.title
    description: !file
      path: ./specs/api.yaml
      extract: info.description
    version: !file
      path: ./specs/api.yaml
      extract: info.version

    versions:
      - ref: v1
        spec: !file
          path: ./specs/api.yaml
```

**Validation**:
- Verify map format extracts values correctly
- Verify equivalent to hash syntax: `!file ./specs/api.yaml#info.title`
- Verify file caching works with map format

---

### 5.3 LOW PRIORITY Scenarios

These scenarios address edge cases and less critical features.

---

#### Scenario 15: dry-run-validation

**Purpose**: Test --dry-run flag
**Priority**: 🟢 LOW

**Commands**: apply --dry-run, sync --dry-run
**Resources**: Portal, API

**Test Steps**:
1. Run `kongctl apply -f config.yaml --dry-run`
2. Verify plan generated
3. Verify NO changes made to Konnect
4. Run `kongctl sync -f config.yaml --dry-run`
5. Verify deletion plan generated
6. Verify NO deletions occurred

---

#### Scenario 16: dump-tf-import-format

**Purpose**: Test dump tf-import output format
**Priority**: 🟢 LOW

**Commands**: dump tf-import
**Resources**: Portal, API, Control Plane

**Test Steps**:
1. Create resources via apply
2. Run `kongctl dump tf-import --resources=portal,api`
3. Verify output format matches Terraform import syntax
4. Verify all resources included

---

#### Scenario 17: namespace-edge-cases

**Purpose**: Test namespace edge cases
**Priority**: 🟢 LOW

**Commands**: apply, sync
**Resources**: Various with edge-case namespaces

**Test Cases**:
1. Empty namespace (should error)
2. Very long namespace (test limits)
3. Special characters in namespace
4. Namespace inheritance edge cases

---

#### Scenario 18: reference-error-scenarios

**Purpose**: Test !ref error handling
**Priority**: 🟢 LOW

**Commands**: apply (expected to fail)
**Resources**: Resources with invalid !ref

**Test Cases**:
1. Reference to non-existent resource
2. Invalid field extraction: `!ref resource#nonexistent.field`
3. Circular dependencies: A → B → A
4. Type mismatches in references

---

#### Scenario 19: field-validation-errors

**Purpose**: Test field name typo detection
**Priority**: 🟢 LOW

**Commands**: apply (expected to fail with helpful errors)
**Resources**: Resources with common typos

**Test Cases**:
```yaml
# Test case 1: lables → labels
portals:
  - ref: typo-portal
    name: "Typo Test"
    lables:  # Should suggest "labels"
      team: platform

# Test case 2: descriptin → description
apis:
  - ref: typo-api
    name: "Typo API"
    descriptin: "Test"  # Should suggest "description"
```

**Validation**:
- Verify error includes "Unknown field"
- Verify error suggests correct field name
- Verify helpful "Did you mean...?" message

---

#### Scenario 20: api-document-hierarchy

**Purpose**: Test complex document parent-child relationships
**Priority**: 🟢 LOW

**Commands**: apply, sync
**Resources**: API Documents with multiple nesting levels

**Configuration**:
```yaml
apis:
  - ref: doc-hierarchy-api
    name: "Document Hierarchy API"

    documents:
      - ref: parent-doc
        title: "Parent Document"

        children:
          - ref: child-doc-1
            title: "Child Document 1"

            children:
              - ref: grandchild-doc
                title: "Grandchild Document"

          - ref: child-doc-2
            title: "Child Document 2"
```

**Validation**:
- Verify multi-level nesting works
- Verify parent-child relationships correct
- Verify sync deletes cascade properly

---

#### Scenario 21: labels-comprehensive

**Purpose**: Test labels on all parent resources
**Priority**: 🟢 LOW

**Commands**: apply, sync
**Resources**: Portal, API, Auth Strategy, Control Plane (all with labels)

**Configuration**:
```yaml
portals:
  - ref: labeled-portal
    name: "Labeled Portal"
    labels:
      environment: "test"
      team: "platform"

apis:
  - ref: labeled-api
    name: "Labeled API"
    labels:
      service: "billing"
      owner: "team-alpha"

application_auth_strategies:
  - ref: labeled-strategy
    name: "Labeled Strategy"
    strategy_type: "key_auth"
    labels:
      auth-type: "api-key"

control_planes:
  - ref: labeled-cp
    name: "labeled-cp"
    labels:
      region: "us-west-2"
      environment: "production"
```

---

## Section 6: Summary Statistics

### 6.1 Test Coverage Summary

| Category | Total Features | Tested | Not Tested | Coverage % |
|----------|----------------|--------|------------|------------|
| **Commands** | 6 | 5 | 1 (diff) | 83% |
| **Parent Resources** | 4 | 4 | 0 | 100% |
| **Child Resources** | 9 | 8 | 1 (custom domain) | 89% |
| **Kongctl Metadata** | 2 | 1 | 1 (protected) | 50% |
| **YAML Tags** | 4 | 3 | 1 (map format) | 75% |
| **Config Patterns** | 3 | 1.5 | 1.5 | 50% |

**Overall Average Coverage**: ~60%

---

### 6.2 Field Coverage by Resource Type

| Resource Type | Fields Tested | Fields Not Tested | Coverage % |
|---------------|---------------|-------------------|------------|
| Portal | 12 | 8 | 60% |
| API | 7 | 8 | 47% |
| Application Auth Strategy | 5 | 5 | 50% |
| Control Plane | 7 | 10 | 41% |
| API Version | 3 | 3 | 50% |
| API Publication | 4 | 4 | 50% |
| API Implementation | 2 | 3 | 40% |
| API Document | 5 | 2 | 71% |
| Portal Page | 6 | 2 | 75% |
| Portal Snippet | 5 | 0 | 100% |
| Portal Customization | 4 | 5 | 44% |
| Portal Custom Domain | 0 | ALL | **0%** |
| Gateway Service | 2 | 6 | 25% |

**Average Field Coverage**: ~50%

---

### 6.3 Scenarios by Priority

| Priority | Count | Key Focus |
|----------|-------|-----------|
| 🔴 HIGH | 5 scenarios | Protected resources, diff command, plan artifacts, custom domains, error handling |
| 🟡 MEDIUM | 9 scenarios | Comprehensive field coverage, configuration patterns |
| 🟢 LOW | 7 scenarios | Edge cases, additional error scenarios, advanced features |

**Total Recommended Scenarios**: 21

---

### 6.4 Critical Gaps Requiring Immediate Attention

1. **protected field** (ZERO e2e coverage) - Production safety feature
2. **Portal Custom Domain** (ZERO coverage) - Entire resource untested
3. **diff command** (ZERO coverage) - Core command missing
4. **Plan artifact workflows** (NOT tested) - Documented workflow pattern
5. **!file map format** (NOT tested) - Documented syntax variant
6. **Error scenarios** (Minimal coverage) - Field validation, tag errors, references

---

### 6.5 Overall Assessment

**Strengths**:
- ✅ Good coverage of basic workflows (apply, sync)
- ✅ Core parent resources all have at least basic coverage
- ✅ Namespace features well tested
- ✅ YAML tag basics (!file, !ref) well covered
- ✅ Control plane groups well tested

**Weaknesses**:
- ❌ Field-level coverage is shallow (~50% average)
- ❌ Error scenarios largely untested
- ❌ Advanced features (protected, custom domains) have critical gaps
- ❌ Command coverage missing key features (diff, plan artifacts, dry-run)
- ❌ Limited testing of comprehensive field combinations
- ❌ Configuration pattern coverage incomplete

**Impact**:
- Current coverage protects basic happy-path workflows
- Production safety features (protected) are untested
- Error handling and validation largely unvalidated
- Advanced workflows (plan artifacts) not validated

---

### 6.6 Recommendations

#### Immediate Actions (HIGH Priority)

1. **Implement protected resources testing** (Scenario 1)
   - Critical production safety feature
   - Should be tested before GA release

2. **Add diff command coverage** (Scenario 2)
   - Core command with no testing
   - Essential for user workflows

3. **Test plan artifact workflows** (Scenario 3)
   - Documented workflow pattern
   - Important for CI/CD integrations

4. **Add Portal Custom Domain testing** (Scenario 4)
   - Entire resource type missing
   - Prevents regressions in this feature

5. **Test error scenarios** (Scenario 5)
   - Security implications (!file path traversal)
   - User experience (helpful error messages)

#### Short-Term Actions (MEDIUM Priority)

6-14. **Improve field coverage** across all resource types
- Target 80%+ field coverage
- Focus on commonly used fields first
- Test configuration patterns (flat, mixed)

#### Long-Term Actions (LOW Priority)

15-21. **Add edge case and advanced feature testing**
- Complete coverage of less common scenarios
- Validate error messages comprehensively
- Test limits and boundaries

---

### 6.7 Success Metrics

**Target Coverage Goals**:
- Commands: 83% → 100% (add diff command testing)
- Resources: 89% → 100% (add custom domain testing)
- Metadata: 50% → 100% (add protected field testing)
- YAML Tags: 75% → 100% (add map format testing)
- Field Coverage: ~50% → ~85% average across all resources
- Error Scenarios: ~10% → ~80% coverage

**Achieving these goals would bring overall test coverage from ~60% to ~90%**

---

## Appendix A: Test Harness Capabilities

Based on analysis of `test/e2e/harness/`:

### Available Step Types
- **CLI steps**: Execute kongctl commands
- **Assertion steps**: Verify resource state
- **Setup/teardown**: Pre/post test actions
- **Configuration**: YAML-based test definitions

### Assertion Capabilities
- Resource existence checks
- Field value validation
- Label verification
- State comparisons

### Limitations Identified
- No built-in diff output validation (can be added)
- Limited error message validation (can be enhanced)
- File system interaction capabilities (need verification)

---

## Appendix B: References

### Documentation
- `docs/declarative.md` - Declarative configuration guide
- `docs/examples/declarative/` - Example configurations

### Code
- `test/e2e/scenarios/` - Scenario definitions
- `test/e2e/harness/` - Test harness implementation
- `test/e2e/testdata/scenarios/declarative/` - Test data

### Examples Reviewed
- `docs/examples/declarative/basic/` - Basic examples
- `docs/examples/declarative/comprehensive/` - Comprehensive examples
- `docs/examples/declarative/protected/` - Protected resources example
- `docs/examples/declarative/portal/` - Portal examples
- Additional example directories

---

## Document Information

- **Generated**: 2025-11-01
- **Analysis Scope**: Declarative configuration features only
- **Total Scenarios Analyzed**: 14
- **Recommended New Scenarios**: 21
- **Target Coverage Improvement**: 60% → 90%

---

*This document should be updated periodically as test coverage improves.*
