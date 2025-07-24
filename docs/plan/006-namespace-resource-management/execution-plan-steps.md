# Stage 6: Namespace-Based Resource Management - Implementation Steps

## Progress Summary
**Progress**: 6/15 steps completed (40%)  
**Current Step**: Step 7 - Update Label Constants and Remove Deprecated Labels

## Overview
This document outlines the step-by-step implementation plan for adding 
namespace-based resource management to kongctl.

## Implementation Steps

### Step 1: Add Namespace Field to KongctlMeta
**Status**: Completed

Add the namespace field to the existing KongctlMeta struct, following the 
same pattern as the protected field.

**Files to modify**:
- `internal/declarative/resources/types.go`

**Changes**:
- Add `Namespace string` field to KongctlMeta struct
- Update YAML tags for proper serialization

**Acceptance criteria**:
- KongctlMeta includes namespace field
- Field properly serializes/deserializes from YAML
- Unit tests for struct marshaling

---

### Step 2: Remove KongctlMeta from Child Resources
**Status**: Completed

Remove the kongctl metadata field from all child resource types since Konnect 
doesn't support labels on child resources.

**Files to modify**:
- `internal/declarative/resources/api_version.go`
- `internal/declarative/resources/api_publication.go`
- `internal/declarative/resources/api_implementation.go`
- `internal/declarative/resources/api_document.go`

**Changes**:
- Remove `Kongctl *KongctlMeta` field from all child resource structs
- Add validation to reject kongctl sections in child resources
- Update any tests that reference child resource kongctl fields

**Acceptance criteria**:
- Child resources cannot have kongctl metadata in configuration
- Clear error message if user tries to add kongctl to child resources
- All child resource types updated consistently
- Tests pass without kongctl on child resources

---

### Step 3: Define Namespace Field for Parent Resources
**Status**: Completed

Clarify that namespace field is only valid on parent resources by updating 
configuration documentation and examples.

**Files to modify**:
- Configuration examples in docs
- README sections about namespace usage

**Changes**:
- Document that only parent resources support kongctl.namespace
- Update examples to show namespace only on APIs, Portals, Auth Strategies
- Add note about child resource namespace inheritance

**Acceptance criteria**:
- Clear documentation about parent-only namespace support
- Examples follow correct patterns
- No misleading configurations

---

### Step 4: Define _defaults Configuration Structure
**Status**: Completed

Create the configuration types for file-level defaults, starting with 
namespace support.

**Files to modify**:
- `internal/declarative/resources/types.go`

**Changes**:
- Add DefaultsConfig struct
- Add KongctlDefaults struct
- Update ResourceSet to include _defaults field

**Acceptance criteria**:
- _defaults section can be parsed from YAML
- Structure supports future expansion
- Does not break existing configurations

---

### Step 5: Implement Defaults Parsing in Loader
**Status**: Completed

Update the configuration loader to parse and store the _defaults section 
from YAML files.

**Files to modify**:
- `internal/declarative/loader/loader.go`

**Changes**:
- Parse _defaults section during file loading
- Store defaults in appropriate structure
- Handle missing defaults gracefully

**Acceptance criteria**:
- Loader correctly parses _defaults section
- Defaults are accessible during resource processing
- No impact on files without _defaults

---

### Step 6: Apply Namespace Defaults During Loading
**Status**: Completed

Implement the logic to apply file-level namespace defaults to resources 
that don't explicitly specify a namespace.

**Files to modify**:
- `internal/declarative/loader/loader.go`

**Changes**:
- Add applyNamespaceDefaults function
- Apply defaults after resource parsing
- Use "default" as implicit default when no namespace specified

**Acceptance criteria**:
- Resources inherit namespace from _defaults
- Explicit namespace overrides defaults
- Resources get "default" namespace when none specified

---

### Step 7: Update Label Constants and Remove Deprecated Labels
**Status**: Not Started

Add the namespace label constant and remove deprecated managed/last-updated 
labels to stay within Konnect's 5-label limit.

**Files to modify**:
- `internal/declarative/labels/labels.go`

**Changes**:
- Add `NamespaceKey = "KONGCTL-namespace"` constant
- Remove or deprecate `ManagedKey` and `LastUpdatedKey` constants
- Update `AddManagedLabels` to only add namespace label (and protected when true)
- Update `AddManagedLabelsToPointerMap` similarly
- Replace `IsManagedResource` to check namespace presence instead
- Only add `KONGCTL-protected: true` when resource is actually protected

**Acceptance criteria**:
- Namespace constant follows existing naming pattern
- No more KONGCTL-managed or KONGCTL-last-updated labels added
- Protected label only added when kongctl.protected is true
- Resources identified by namespace presence
- Default case uses only 1 label (namespace)

---

### Step 8: Update Planners for Namespace Handling
**Status**: Not Started

Modify the resource planners to handle the namespace field and pass it 
through to planned changes.

**Files to modify**:
- `internal/declarative/planner/portal_planner.go`
- `internal/declarative/planner/api_planner.go`
- `internal/declarative/planner/auth_strategy_planner.go`
- `internal/declarative/planner/types.go`

**Changes**:
- Add namespace to PlannedChange struct
- Extract namespace from resources during planning
- Pass namespace through planning pipeline

**Acceptance criteria**:
- Planners correctly extract namespace
- Namespace available in PlannedChange
- Child resources handled appropriately

---

### Step 9: Update Label Handling in Executors
**Status**: Not Started

Update executors to convert namespace field to label and remove deprecated 
label handling.

**Files to modify**:
- `internal/declarative/executor/portal_executor.go`
- `internal/declarative/executor/api_executor.go`
- `internal/declarative/executor/auth_strategy_executor.go`
- `internal/declarative/labels/labels.go`
- `internal/declarative/state/client.go`

**Changes**:
- Modify BuildCreateLabels to accept namespace parameter
- Remove logic that adds KONGCTL-managed and KONGCTL-last-updated
- Add KONGCTL-namespace label from namespace parameter
- Keep KONGCTL-protected label handling unchanged
- Update state client Create/Update methods to use new label functions

**Acceptance criteria**:
- Resources created with only namespace and protected labels
- No more managed or last-updated labels
- Namespace label properly set from kongctl.namespace field
- Updates preserve namespace and protected status

---

### Step 10: Update State Client for Namespace-Based Resource Management
**Status**: Not Started

Update the state client to use namespace presence for resource management 
instead of the deprecated KONGCTL-managed label.

**Files to modify**:
- `internal/declarative/state/client.go`

**Changes**:
- Replace `IsManagedResource()` checks with namespace label presence
- Add namespace parameter to ListManaged* methods  
- Filter by KONGCTL-namespace label instead of KONGCTL-managed
- Consider any resource with namespace label as managed
- Handle multiple namespace filtering

**Acceptance criteria**:
- Resources identified by namespace presence, not managed label
- Can filter by single namespace
- Can filter by multiple namespaces
- Empty namespace list returns no resources
- Backwards compatibility for existing resources (temporary)

---

### Step 11: Group Resources by Namespace in Planner
**Status**: Not Started

Implement namespace grouping logic in the main planner to process each 
namespace independently.

**Files to modify**:
- `internal/declarative/planner/planner.go`

**Changes**:
- Group loaded resources by namespace
- Generate separate plans per namespace
- Maintain namespace isolation

**Acceptance criteria**:
- Resources grouped correctly
- Each namespace planned independently
- No cross-namespace interference

---

### Step 12: Update Command Output for Namespace Visibility
**Status**: Not Started

Enhance command output to clearly show namespace operations and provide 
better visibility.

**Files to modify**:
- `internal/cmd/root/products/konnect/declarative/plan.go`
- `internal/cmd/root/products/konnect/declarative/apply.go`
- `internal/cmd/root/products/konnect/declarative/sync.go`
- `internal/cmd/root/products/konnect/declarative/diff.go`

**Changes**:
- Show namespaces being processed
- Group output by namespace
- Add namespace to resource identifiers

**Acceptance criteria**:
- Clear namespace visibility in output
- Operations grouped by namespace
- Improved user understanding

---

### Step 13: Add Namespace Validation
**Status**: Not Started

Implement validation to ensure namespace consistency and prevent errors.

**Files to modify**:
- `internal/declarative/validator/namespace_validator.go` (new)
- `internal/declarative/loader/loader.go`

**Changes**:
- Create namespace validator
- Validate namespace values are valid
- Check parent-child namespace consistency

**Acceptance criteria**:
- Invalid namespace values cause error
- Clear error messages
- Validation runs during loading

---

### Step 14: Create Integration Tests
**Status**: Not Started

Add comprehensive integration tests for namespace functionality.

**Files to create**:
- `test/integration/namespace_test.go`

**Test scenarios**:
- Single namespace operations
- Multi-namespace operations
- Namespace defaults
- Namespace isolation
- Error cases

**Acceptance criteria**:
- Tests cover all scenarios
- Tests pass reliably
- Good error case coverage

---

### Step 15: Update Documentation and Examples
**Status**: Not Started

Create documentation and examples showing namespace usage.

**Files to create/modify**:
- `docs/examples/declarative/namespace/` (new examples)
- `README.md` updates
- `docs/declarative-config.md` updates

**Content**:
- Namespace concept explanation
- Usage examples
- Best practices
- Limitations

**Acceptance criteria**:
- Clear documentation
- Working examples
- Best practices documented

---

## Summary

Total steps: 15

Implementation order is designed to:
1. Build core infrastructure (Steps 1-7)
2. Integrate with existing systems (Steps 8-11)
3. Enhance user experience (Steps 12-13)
4. Ensure quality (Steps 14-15)

Each step builds on previous work and can be tested independently.