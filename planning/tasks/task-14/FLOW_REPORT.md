# Complete Code Flow Report: Ref Field Bug Analysis

## Executive Summary

This report traces the complete execution flow for how the `ref` field is handled throughout the kongctl declarative configuration system. The analysis reveals that refs are processed through separate per-resource-type maps at every stage, allowing duplicate refs across different resource types, which violates the intended global uniqueness requirement.

## 1. Data Structure Foundation

### 1.1 Core Data Structures

**File:** `/internal/declarative/resources/types.go` (Lines 4-25)
```go
type ResourceSet struct {
    Portals                   []PortalResource                   // Separate slice per type
    ApplicationAuthStrategies []ApplicationAuthStrategyResource  // Separate slice per type
    ControlPlanes             []ControlPlaneResource            // Separate slice per type
    APIs                      []APIResource                     // Separate slice per type
    // ... other child resource types
}
```

**Critical Issue:** ResourceSet maintains separate slices for each resource type with no global ref tracking mechanism.

**File:** `/internal/declarative/resources/interfaces.go` (Lines 4-22)
```go
type Resource interface {
    GetRef() string  // Every resource implements this
    // ... other methods
}

type ResourceRef struct {
    Kind string  // Resource type (portal, api, etc.)
    Ref  string  // Reference value
}
```

**Flow Implication:** While ResourceRef contains both Kind and Ref, the validation system only checks uniqueness within each Kind separately.

## 2. Configuration Input Flow

### 2.1 File Parsing Entry Point

**File:** `/internal/declarative/loader/loader.go`

#### 2.1.1 Primary Loading Path
```
LoadFromSources() → loadSingleFile() → parseYAML() → validateResourceSet()
Lines 55-137: Main orchestration
Lines 160-180: Single file processing
Lines 182-260: YAML parsing with tag processing
Lines 132-134: Final validation call
```

#### 2.1.2 Ref Tracking During Loading
**Lines 58-74:** Separate tracking maps created per resource type
```go
// BUG: These separate maps allow cross-type duplicates
portalRefs := make(map[string]string)      // ref → source path  
authStratRefs := make(map[string]string)   // ref → source path
cpRefs := make(map[string]string)          // ref → source path
apiRefs := make(map[string]string)         // ref → source path
```

## 3. Validation Flow - Primary Bug Location

### 3.1 ValidationResourceSet Entry Point

**File:** `/internal/declarative/loader/validator.go`

#### 3.1.1 Main Validation Orchestrator
**Lines 12-53:** `validateResourceSet()` coordinates all validation
```
validateResourceSet() 
  ├── Line 15: resourceRegistry := make(map[string]map[string]bool)
  ├── Lines 18-20: validatePortals()
  ├── Lines 23-25: validateAuthStrategies()  
  ├── Lines 28-30: validateControlPlanes()
  ├── Lines 33-35: validateAPIs()
  ├── Lines 38-40: validateSeparateAPIChildResources()
  └── Lines 43-45: validateCrossReferences()
```

#### 3.1.2 Per-Resource-Type Validation Pattern
Each validation function follows this **BUGGY PATTERN**:

**Portal Validation (Lines 56-84):**
```go
func validatePortals() {
    refs := make(map[string]bool)          // ⚠️ SEPARATE map per type
    registry["portal"] = refs              // ⚠️ Type-specific registry entry
    
    for _, portal := range portals {
        if refs[portal.GetRef()] {          // ⚠️ Only checks within portal refs
            return fmt.Errorf("duplicate portal ref: %s", portal.GetRef())
        }
        refs[portal.GetRef()] = true
    }
}
```

**Auth Strategy Validation (Lines 88-121):**
```go
func validateAuthStrategies() {
    refs := make(map[string]bool)              // ⚠️ SEPARATE map per type
    registry["application_auth_strategy"] = refs  // ⚠️ Type-specific registry
    // Same buggy pattern...
}
```

**Control Plane Validation (Lines 124-156):**
```go
func validateControlPlanes() {
    refs := make(map[string]bool)          // ⚠️ SEPARATE map per type  
    registry["control_plane"] = refs      // ⚠️ Type-specific registry
    // Same buggy pattern...
}
```

**API Validation (Lines 159-240):**
```go
func validateAPIs() {
    apiRefs := make(map[string]bool)       // ⚠️ SEPARATE map per type
    registry["api"] = apiRefs              // ⚠️ Type-specific registry
    
    // Also creates separate maps for child resources
    versionRefs := make(map[string]bool)
    registry["api_version"] = versionRefs
    // ... more separate maps
}
```

### 3.2 Cross-Reference Validation

**Lines 242-285:** `validateCrossReferences()` 
```go
// Uses the same registry with separate type maps
for fieldPath, expectedType := range mappings {
    if !registry[expectedType][fieldValue] {  // ⚠️ Looks up by type only
        return fmt.Errorf("resource %q references unknown %s: %s", 
            refResource.GetRef(), expectedType, fieldValue)
    }
}
```

**Impact:** Cross-reference validation assumes refs are unique within each resource type, not globally.

## 4. Reference Resolution Flow

### 4.1 Planner Integration

**File:** `/internal/declarative/planner/resolver.go`

#### 4.1.1 Created Resource Tracking
**Lines 43-52:** Build map of resources being created in current plan
```go
func ResolveReferences() {
    // BUG: Separate maps per resource type
    createdResources := make(map[string]map[string]string) // resource_type → ref → change_id
    
    for _, change := range changes {
        if change.Action == ActionCreate {
            if createdResources[change.ResourceType] == nil {
                createdResources[change.ResourceType] = make(map[string]string)
            }
            createdResources[change.ResourceType][change.ResourceRef] = change.ID
        }
    }
}
```

#### 4.1.2 Reference Resolution Process
**Lines 62-83:** Resolve each reference
```go
// Determine resource type from field name
resourceType := r.getResourceTypeForField(fieldName)

// Check if this references something being created
if _, inPlan := createdResources[resourceType][ref]; inPlan {
    // Handle internal reference
} else {
    // Resolve from existing resources  
    id, err := r.resolveReference(ctx, resourceType, ref)
}
```

**Lines 141-152:** `getResourceTypeForField()` maps field names to specific resource types
```go
func getResourceTypeForField(fieldName string) string {
    switch fieldName {
    case "default_application_auth_strategy_id", "auth_strategy_ids":
        return "application_auth_strategy"
    case "control_plane_id", "gateway_service.control_plane_id":
        return "control_plane" 
    case "portal_id":
        return ResourceTypePortal
    }
}
```

**Critical Flow Issue:** Resolution assumes each field name maps to a specific resource type, but doesn't handle the case where a ref value could exist in multiple types.

### 4.2 Dependency Resolution

**File:** `/internal/declarative/planner/dependencies.go`

#### 4.2.1 Dependency Graph Construction
**Lines 17-98:** Build dependency graph based on references
```go
func ResolveDependencies(changes []PlannedChange) ([]string, error) {
    // Track all changes by ID
    for _, change := range changes {
        // Add implicit dependencies based on references  
        deps := d.findImplicitDependencies(change, changes)
    }
}
```

**Lines 101-118:** Find dependencies from References field
```go
func findImplicitDependencies(change PlannedChange, allChanges []PlannedChange) []string {
    for _, refInfo := range change.References {
        if refInfo.ID == "[unknown]" {
            // Find the change that creates this resource
            for _, other := range allChanges {
                if other.ResourceRef == refInfo.Ref && other.Action == ActionCreate {
                    dependencies = append(dependencies, other.ID)
                    break  // ⚠️ Takes first match - could be wrong type!
                }
            }
        }
    }
}
```

**Dependency Bug:** When searching for dependencies, it matches purely on `ResourceRef` without considering `ResourceType`, so if multiple resource types have the same ref, it will match the first one found.

## 5. State Management Integration

### 5.1 State Client Resolution

**File:** `/internal/declarative/state/client.go`

The state client provides lookup methods that assume refs are unique within each resource type:

#### 5.1.1 Portal Resolution
**Lines 209-224:** `GetPortalByName()`
```go
func GetPortalByName(ctx context.Context, name string) (*Portal, error) {
    portals, err := c.ListManagedPortals(ctx, []string{"*"})
    for _, p := range portals {
        if p.Name == name {  // Assumes name uniqueness within portals
            return &p, nil
        }
    }
    return nil, nil
}
```

#### 5.1.2 API Resolution  
**Lines 415-461:** `GetAPIByName()`
```go
func GetAPIByName(ctx context.Context, name string) (*API, error) {
    apis, err := c.ListManagedAPIs(ctx, []string{"*"})
    for _, a := range apis {
        if a.Name == name {  // Assumes name uniqueness within APIs
            return &a, nil
        }
    }
    return nil, nil
}
```

**State Flow Issue:** Each resource type's lookup method only searches within that type, reinforcing the per-type isolation.

## 6. Resource Implementation Pattern

### 6.1 Individual Resource Pattern

Each resource type follows the same pattern for ref handling:

**File:** `/internal/declarative/resources/api_publication.go` (Example)

#### 6.1.1 Ref Interface Implementation
**Lines 29-31:**
```go
func (p APIPublicationResource) GetRef() string {
    return p.Ref
}
```

#### 6.1.2 Reference Mappings
**Lines 50-56:**
```go
func (p APIPublicationResource) GetReferenceFieldMappings() map[string]string {
    return map[string]string{
        "portal_id":         "portal",           // Maps to specific type
        "auth_strategy_ids": "application_auth_strategy",  // Maps to specific type
    }
}
```

**Pattern Impact:** Each resource's reference mappings assume target resource types are distinct and refs are unique within each type.

## 7. Critical Execution Paths

### 7.1 Primary Configuration Loading Path

```
User Configuration Files
    ↓
LoadFromSources() [loader.go:55]
    ↓
loadSingleFile() [loader.go:160]
    ↓  
parseYAML() [loader.go:182]
    ↓
validateResourceSet() [validator.go:12]
    ↓
validatePortals() [validator.go:56] ─────┐
validateAuthStrategies() [validator.go:88] │  
validateControlPlanes() [validator.go:124] │ ← Per-type validation
validateAPIs() [validator.go:159] ─────────┘   (SEPARATE MAPS)
    ↓
validateCrossReferences() [validator.go:242]
    ↓
VALIDATED ResourceSet (with hidden cross-type duplicate bug)
```

### 7.2 Planning and Resolution Path

```
Validated ResourceSet
    ↓
Planner.Plan() [creates PlannedChanges]
    ↓
ReferenceResolver.ResolveReferences() [resolver.go:36]
    ↓
Build createdResources map [resolver.go:44] ← SEPARATE MAPS PER TYPE
    ↓
For each reference:
  getResourceTypeForField() [resolver.go:141] ← Maps to specific type
  Check createdResources[type][ref] [resolver.go:65] ← Type-specific lookup
    ↓
DependencyResolver.ResolveDependencies() [dependencies.go:17]
    ↓
findImplicitDependencies() [dependencies.go:101] ← Could match wrong ref
    ↓
Final PlannedChanges (potentially with wrong dependencies)
```

### 7.3 Error Paths

#### 7.3.1 Validation Error Path
```
validatePortals() detects duplicate "common" in portals → ERROR
BUT
validatePortals() allows "common" in portals + validateAPIs() allows "common" in APIs → ✅ PASSES
```

#### 7.3.2 Reference Resolution Error Path
```
Field "portal_id" with value "common"
    ↓
getResourceTypeForField("portal_id") → "portal"
    ↓
Look in createdResources["portal"]["common"] → FOUND (correct)
BUT if "common" also exists in APIs, dependency resolution might be confused
```

## 8. Data Flow Interconnections

### 8.1 Resource Creation Flow

```
YAML Config → ResourceSet → Validation → Planning → Resolution → Execution
     ↓              ↓            ↓           ↓           ↓           ↓
  Parsing      Separate      Per-type    Type-based   State      Konnect
              Collections    Tracking    Resolution   Client      APIs
```

### 8.2 Cross-Resource Reference Flow

```
API Publication references Portal:
  portal_id: "common" 
       ↓
  GetReferenceFieldMappings() → {"portal_id": "portal"}
       ↓  
  validateResourceReferences() → registry["portal"]["common"] ✓
       ↓
  ResolveReferences() → createdResources["portal"]["common"] ✓
       ↓
  State Resolution → GetPortalByName("common") → Portal ID
```

**Flow works correctly IF refs are unique per type, breaks with cross-type duplicates.**

### 8.3 Component Dependency Graph

```
ResourceSet (types.go)
    ↓
Loader (loader.go) ───────┐
    ↓                     ↓
Validator (validator.go)  TagRegistry (tags/)
    ↓                        
Planner ──────────────────┐
    ↓                     ↓
ReferenceResolver     DependencyResolver
    ↓                     ↓
State Client ─────────────┘
    ↓
Konnect APIs
```

**Each component assumes per-type ref uniqueness.**

## 9. Impact Analysis of Ref Handling

### 9.1 Current System Behavior

**Duplicate Ref Scenario:**
```yaml
portals:
  - ref: common
    name: "My Portal"
    
apis:
  - ref: common  # Same ref value
    name: "My API"
```

**Current Flow:**
1. **Loading:** ✅ Loads successfully (separate tracking maps)
2. **Validation:** ✅ Validates successfully (separate registry maps) 
3. **Planning:** ✅ Plans successfully (separate type maps)
4. **Resolution:** ⚠️ **POTENTIAL AMBIGUITY** - field type determines lookup
5. **Execution:** ⚠️ **DEPENDS ON FIELD SPECIFICITY**

### 9.2 Failure Scenarios

#### 9.2.1 Dependency Resolution Ambiguity
If a resource references `ref: "common"` without a type-specific field name, the system cannot determine which "common" resource is intended.

#### 9.2.2 Cross-Reference Conflicts
Future features requiring generic cross-resource references would fail due to ambiguous ref resolution.

#### 9.2.3 User Confusion
Users might accidentally create duplicate refs across types, leading to unexpected behavior that's hard to debug.

## 10. Key Decision Points in Code

### 10.1 Critical Decision Points

1. **Line validator.go:15** - Creation of separate `resourceRegistry` maps
   - **Decision:** Per-type tracking  
   - **Impact:** Allows cross-type duplicates
   - **Alternative:** Global ref map

2. **Line resolver.go:44** - Creation of `createdResources` map structure
   - **Decision:** `map[resourceType][ref]` structure
   - **Impact:** Type-based reference resolution
   - **Alternative:** Global ref registry with type metadata

3. **Line dependencies.go:109** - Reference matching in dependency resolution
   - **Decision:** Match by `ResourceRef` only
   - **Impact:** Could match wrong resource type
   - **Alternative:** Match by both ref and type

4. **Line loader.go:59-74** - Duplicate tracking map creation
   - **Decision:** Separate maps per resource type
   - **Impact:** Cross-type duplicates allowed during loading
   - **Alternative:** Global ref tracking

### 10.2 Design Pattern Issues

The system consistently applies a **per-resource-type isolation pattern** that:
- ✅ Simplifies within-type operations
- ✅ Provides clear resource type boundaries  
- ❌ **Violates global ref uniqueness requirement**
- ❌ **Limits cross-resource reference flexibility**
- ❌ **Creates potential ambiguity in resolution**

## 11. Conclusion

### 11.1 Root Cause

The ref field duplicate bug stems from a systematic architectural decision to maintain **separate reference tracking at every layer**:

1. **Data Structure Level:** ResourceSet uses separate slices
2. **Loading Level:** Separate tracking maps per type  
3. **Validation Level:** Separate registry maps per type
4. **Resolution Level:** Separate created resource maps per type
5. **State Level:** Separate lookup methods per type

### 11.2 Fix Requirements

To implement global ref uniqueness, the following components need modification:

1. **Loader:** Use global ref tracking instead of per-type maps
2. **Validator:** Use global registry instead of type-specific registries  
3. **Resolver:** Maintain global ref awareness while preserving type-based resolution
4. **Dependencies:** Ensure ref matching considers both ref and type appropriately
5. **Tests:** Add comprehensive cross-type ref uniqueness validation

### 11.3 Complexity Assessment

**Low Risk Changes:**
- Loader and Validator modifications (isolated impact)

**Medium Risk Changes:**  
- Reference resolution updates (affects planning)

**Testing Requirements:**
- Comprehensive cross-type duplicate detection tests
- Reference resolution ambiguity tests
- Backward compatibility validation

The bug is well-contained but requires coordinated changes across multiple components to ensure global ref uniqueness while maintaining existing functionality.