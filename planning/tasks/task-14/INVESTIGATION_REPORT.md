# Investigation Report: Duplicate Ref Bug in Declarative Configuration

## Overview

This investigation identified a critical bug in the kongctl declarative configuration system where the 'ref' field is not enforced to be globally unique across all resource types. The current implementation maintains separate reference maps per resource type, allowing duplicate `ref` values across different resource types, which violates the intended design.

## Intended vs Current Behavior

### Intended Behavior
- The `ref` field should be a **unique string identifier across ALL resources** in a ResourceSet
- Cross-resource references should work using these globally unique `ref` values
- Duplicate `ref` values should be rejected regardless of resource type

### Current Behavior
- Each resource type maintains its own separate map of ref → resource
- Duplicate `ref` values are only detected within the same resource type
- A portal with `ref: "common"` and an API with `ref: "common"` both pass validation
- This creates potential ambiguity in cross-resource references

## Key Files and Code Locations

### 1. ResourceSet Definition
**File:** `/internal/declarative/resources/types.go`
- Lines 4-25: Defines ResourceSet struct with individual slices for each resource type
- No global ref tracking mechanism present

### 2. Resource Interface
**File:** `/internal/declarative/resources/interfaces.go`
- Lines 4-16: Resource interface defines `GetRef() string` method
- Lines 18-22: ResourceRef struct used for cross-references
- Each resource implements GetRef() individually

### 3. Primary Bug Location - Validator
**File:** `/internal/declarative/loader/validator.go`
- Lines 15-16: Creates separate `resourceRegistry` with per-type maps
- Lines 59, 94, 130: Each resource type gets its own map in `registry[<type>]`

```go
// BUG: Separate maps per resource type allow duplicates across types
resourceRegistry := make(map[string]map[string]bool)
registry["portal"] = refs              // Portal refs tracked separately
registry["application_auth_strategy"] = refs  // Auth strategy refs tracked separately  
registry["control_plane"] = refs       // Control plane refs tracked separately
```

### 4. Duplicate Detection in Loader
**File:** `/internal/declarative/loader/loader.go`
- Lines 59-74: Separate tracking maps for each resource type
- Lines 354-395: Individual duplicate checking per resource type

```go
// BUG: Separate tracking allows "common" to exist as both portal and API ref
portalRefs := make(map[string]string)     // Only tracks portal refs
apiRefs := make(map[string]string)        // Only tracks API refs
cpRefs := make(map[string]string)         // Only tracks control plane refs
```

### 5. Reference Resolution Bug
**File:** `/internal/declarative/planner/resolver.go`
- Lines 44-52: Creates separate maps per resource type for created resources
- Lines 141-151: getResourceTypeForField() maps field names to specific resource types

```go
// BUG: Resolution assumes refs are unique within type, not globally
createdResources := make(map[string]map[string]string) // resource_type -> ref -> change_id
```

## Specific Bug Manifestations

### 1. Loader Bug
The loader creates separate tracking maps for each resource type:
```go
portalRefs := make(map[string]string)      // Only portals
authStratRefs := make(map[string]string)   // Only auth strategies  
cpRefs := make(map[string]string)          // Only control planes
apiRefs := make(map[string]string)         // Only APIs
```
This allows the same ref value like `"common"` to exist across multiple resource types.

### 2. Validator Bug
The validator creates a registry with separate maps per type:
```go
resourceRegistry := make(map[string]map[string]bool)
registry["portal"] = refs           // Separate map for portal refs
registry["api"] = apiRefs           // Separate map for API refs
```

### 3. Reference Resolution Bug
The resolver tracks created resources by type, assuming refs are unique within each type:
```go
createdResources[change.ResourceType][change.ResourceRef] = change.ID
```
This could cause ambiguous resolution if multiple resource types use the same ref.

## Test Case Demonstrating the Bug

Created test file: `/planning/tasks/task-14/bug-demo.yaml`
```yaml
# This should FAIL validation but currently PASSES
portals:
  - ref: common
    name: "My Portal"

apis:  
  - ref: common  # Same ref as portal - should fail but doesn't
    name: "My API"

control_planes:
  - ref: common  # Same ref again - should fail but doesn't  
    name: "My Control Plane"
```

## Current Test Coverage

### Tests That Pass (But Shouldn't)
The current validation only catches duplicates within the same resource type:
- `duplicate-refs.yaml` only tests duplicate portal refs
- Test files in `loader_test.go` and `validator_test.go` only test within-type duplicates

### Missing Test Coverage
No tests exist for:
- Cross-resource-type ref uniqueness
- Global ref validation
- Ambiguous cross-references

## Impact Analysis

### High Impact Issues
1. **Cross-Reference Ambiguity**: When resolving a reference to `ref: "common"`, which resource should be selected?
2. **Data Integrity**: Violates the stated requirement that refs are globally unique identifiers
3. **Future Extensibility**: Makes it difficult to add new cross-resource reference features

### Medium Impact Issues  
1. **User Confusion**: Users might accidentally create duplicate refs across types
2. **Debugging Difficulty**: Hard to trace which resource a reference points to
3. **Configuration Validation**: Silent acceptance of invalid configurations

### Current Mitigation
- The bug manifests primarily in validation - the system still functions
- Most cross-references are currently type-specific (e.g., `portal_id` references portals)
- Limited cross-resource referencing reduces immediate impact

## Code Patterns

### Resource Definition Pattern
Each resource type follows this pattern:
```go
type APIResource struct {
    Ref     string       `yaml:"ref" json:"ref"`
    // ... other fields
}

func (a APIResource) GetRef() string {
    return a.Ref
}
```

### Validation Pattern  
Each resource type uses this validation pattern:
```go
refs := make(map[string]bool)  // Type-specific map
registry["resource_type"] = refs

for _, resource := range resources {
    if refs[resource.GetRef()] {
        return fmt.Errorf("duplicate %s ref: %s", resourceType, resource.GetRef())
    }
    refs[resource.GetRef()] = true
}
```

## Dependencies and Related Code

### Reference Field Mappings
**File:** `/internal/declarative/resources/api_publication.go`
- Cross-resource references use field mappings
- Current mappings are type-specific (e.g., `portal_id` → `portal`)

### State Management
**File:** `/internal/declarative/state/client.go`
- State client resolves refs to Konnect IDs
- Resolution assumes type-specific ref uniqueness

### Dependency Resolution
**File:** `/internal/declarative/planner/dependencies.go`
- Dependency resolver tracks resource creation order
- May be affected by duplicate refs in dependency chains

## Conclusion

The bug is well-contained to the validation and loading layers but represents a violation of the intended design. The current per-resource-type ref tracking should be replaced with global ref tracking to ensure true uniqueness across all resources in a ResourceSet. The fix would require:

1. Implementing global ref tracking in the loader
2. Updating validator to use global ref registry  
3. Ensuring reference resolution handles global uniqueness
4. Adding comprehensive tests for cross-type ref conflicts

The impact is currently limited because most cross-references are type-specific, but this bug prevents the system from supporting truly global resource references as originally intended.