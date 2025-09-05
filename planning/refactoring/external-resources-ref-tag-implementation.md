# External Resources and !ref Tag Implementation Plan

## Executive Summary

This document outlines the phased implementation of two major features:
1. **!ref YAML Tag**: A universal mechanism for ALL resource references, replacing hardcoded field detection
2. **External Resources**: Support for referencing resources managed by other tools via `_external` blocks

This is a breaking change with no backward compatibility. The project is pre-GA and users expect breaking changes.

## Key Design Principles

1. **Explicit Over Implicit**: All references must use `!ref` tags - no magic field detection
2. **Simple Linear Search**: No ref->resource mappings; loop through resources as needed (config sets are small)
3. **Hard Cutover**: No dual mode support or migration tools
4. **Consistency**: !ref syntax matches !file tag pattern for familiarity

## Phase 1: Core !ref Tag Implementation

### Goal
Replace ALL implicit reference detection with explicit `!ref` tags throughout the codebase.

### 1.1 Create RefTagResolver

**File**: `internal/declarative/tags/ref.go`

```go
package tags

import (
    "fmt"
    "strings"
    "gopkg.in/yaml.v3"
)

// RefPlaceholder represents an unresolved reference during parsing
type RefPlaceholder struct {
    ResourceRef string  // e.g., "getting-started-portal"
    Field       string  // e.g., "id" or "name" 
    SourceLine  int     // YAML line number for error reporting
    SourceFile  string  // File path for error reporting
}

// RefTagResolver handles !ref tags for resource references
type RefTagResolver struct {
    baseDir string
    // Track all unresolved refs for debugging/error reporting
    unresolvedRefs []*RefPlaceholder
}

// NewRefTagResolver creates a new ref tag resolver
func NewRefTagResolver(baseDir string) *RefTagResolver {
    return &RefTagResolver{
        baseDir:        baseDir,
        unresolvedRefs: make([]*RefPlaceholder, 0),
    }
}

// Tag returns the YAML tag this resolver handles
func (r *RefTagResolver) Tag() string {
    return "!ref"
}

// Resolve processes a YAML node with the !ref tag
func (r *RefTagResolver) Resolve(node *yaml.Node) (any, error) {
    // Only support scalar nodes for !ref
    if node.Kind != yaml.ScalarNode {
        return nil, fmt.Errorf("!ref tag must be used with a string, got %v", node.Kind)
    }
    
    // Parse the reference syntax: resource-ref#field
    refStr := node.Value
    resourceRef := refStr
    field := "id" // default field
    
    if idx := strings.Index(refStr, "#"); idx != -1 {
        field = refStr[idx+1:]
        resourceRef = refStr[:idx]
    }
    
    // Validate the reference format
    if resourceRef == "" {
        return nil, fmt.Errorf("!ref tag requires a resource reference")
    }
    
    if field == "" {
        return nil, fmt.Errorf("!ref tag field cannot be empty after #")
    }
    
    // Create and track the placeholder
    placeholder := &RefPlaceholder{
        ResourceRef: resourceRef,
        Field:       field,
        SourceLine:  node.Line,
        SourceFile:  r.baseDir, // Will be set properly by loader
    }
    
    r.unresolvedRefs = append(r.unresolvedRefs, placeholder)
    
    // Return the placeholder - it will be resolved in a later phase
    return placeholder, nil
}

// GetUnresolvedRefs returns all unresolved references for debugging
func (r *RefTagResolver) GetUnresolvedRefs() []*RefPlaceholder {
    return r.unresolvedRefs
}
```

### 1.2 Register RefTagResolver in Loader

**File**: `internal/declarative/loader/loader.go`

Update the `getTagRegistry` method:

```go
func (l *Loader) getTagRegistry() *tags.Registry {
    registry := tags.NewRegistry()
    
    // Register the file tag resolver
    fileResolver := tags.NewFileTagResolver(l.baseDir)
    registry.RegisterResolver(fileResolver)
    
    // Register the ref tag resolver
    refResolver := tags.NewRefTagResolver(l.baseDir)
    registry.RegisterResolver(refResolver)
    
    return registry
}
```

### 1.3 Create Reference Resolution Logic

**File**: `internal/declarative/loader/ref_resolver.go`

```go
package loader

import (
    "fmt"
    "reflect"
    "strings"
    
    "github.com/kong/kongctl/internal/declarative/resources"
    "github.com/kong/kongctl/internal/declarative/tags"
)

// ResolveReferences walks through all resources and resolves RefPlaceholder objects
// This happens AFTER extractNestedResources so all resources are flattened
func ResolveReferences(rs *resources.ResourceSet) error {
    // Track resolution path for circular dependency detection
    resolutionPath := make([]string, 0)
    
    // Resolve references in each resource type
    if err := resolveResourceSlice(rs, &rs.Portals, resolutionPath); err != nil {
        return fmt.Errorf("resolving portal references: %w", err)
    }
    
    if err := resolveResourceSlice(rs, &rs.ApplicationAuthStrategies, resolutionPath); err != nil {
        return fmt.Errorf("resolving auth strategy references: %w", err)
    }
    
    if err := resolveResourceSlice(rs, &rs.ControlPlanes, resolutionPath); err != nil {
        return fmt.Errorf("resolving control plane references: %w", err)
    }
    
    if err := resolveResourceSlice(rs, &rs.APIs, resolutionPath); err != nil {
        return fmt.Errorf("resolving API references: %w", err)
    }
    
    if err := resolveResourceSlice(rs, &rs.APIVersions, resolutionPath); err != nil {
        return fmt.Errorf("resolving API version references: %w", err)
    }
    
    if err := resolveResourceSlice(rs, &rs.APIPublications, resolutionPath); err != nil {
        return fmt.Errorf("resolving API publication references: %w", err)
    }
    
    if err := resolveResourceSlice(rs, &rs.APIImplementations, resolutionPath); err != nil {
        return fmt.Errorf("resolving API implementation references: %w", err)
    }
    
    if err := resolveResourceSlice(rs, &rs.APIDocuments, resolutionPath); err != nil {
        return fmt.Errorf("resolving API document references: %w", err)
    }
    
    if err := resolveResourceSlice(rs, &rs.PortalCustomizations, resolutionPath); err != nil {
        return fmt.Errorf("resolving portal customization references: %w", err)
    }
    
    if err := resolveResourceSlice(rs, &rs.PortalCustomDomains, resolutionPath); err != nil {
        return fmt.Errorf("resolving portal custom domain references: %w", err)
    }
    
    if err := resolveResourceSlice(rs, &rs.PortalPages, resolutionPath); err != nil {
        return fmt.Errorf("resolving portal page references: %w", err)
    }
    
    if err := resolveResourceSlice(rs, &rs.PortalSnippets, resolutionPath); err != nil {
        return fmt.Errorf("resolving portal snippet references: %w", err)
    }
    
    return nil
}

// resolveResourceSlice resolves references in a slice of resources
func resolveResourceSlice(rs *resources.ResourceSet, slice any, path []string) error {
    sliceVal := reflect.ValueOf(slice).Elem()
    
    for i := 0; i < sliceVal.Len(); i++ {
        elem := sliceVal.Index(i)
        if err := resolveResourceFields(rs, elem, path); err != nil {
            return err
        }
    }
    
    return nil
}

// resolveResourceFields walks through struct fields and resolves RefPlaceholders
func resolveResourceFields(rs *resources.ResourceSet, val reflect.Value, path []string) error {
    // Handle pointer types
    if val.Kind() == reflect.Ptr {
        if val.IsNil() {
            return nil
        }
        val = val.Elem()
    }
    
    // Only process structs
    if val.Kind() != reflect.Struct {
        return nil
    }
    
    typ := val.Type()
    
    for i := 0; i < val.NumField(); i++ {
        field := val.Field(i)
        fieldType := typ.Field(i)
        
        // Skip unexported fields
        if !field.CanSet() {
            continue
        }
        
        // Check if this field contains a RefPlaceholder
        if field.CanInterface() {
            if placeholder, ok := field.Interface().(*tags.RefPlaceholder); ok {
                // Resolve the reference
                resolved, err := resolveReference(rs, placeholder, path)
                if err != nil {
                    return fmt.Errorf("field %s: %w", fieldType.Name, err)
                }
                
                // Set the resolved value
                if field.CanSet() {
                    field.Set(reflect.ValueOf(resolved))
                }
            }
        }
        
        // Recursively process nested structs and slices
        switch field.Kind() {
        case reflect.Struct:
            if err := resolveResourceFields(rs, field, path); err != nil {
                return err
            }
        case reflect.Slice:
            for j := 0; j < field.Len(); j++ {
                if err := resolveResourceFields(rs, field.Index(j), path); err != nil {
                    return err
                }
            }
        case reflect.Map:
            // Handle map fields if needed
            iter := field.MapRange()
            for iter.Next() {
                // Maps are tricky with reflection; may need special handling
            }
        }
    }
    
    return nil
}

// resolveReference looks up a reference and extracts the requested field
func resolveReference(rs *resources.ResourceSet, placeholder *tags.RefPlaceholder, path []string) (any, error) {
    // Check for circular dependencies
    refKey := fmt.Sprintf("%s#%s", placeholder.ResourceRef, placeholder.Field)
    for _, p := range path {
        if p == refKey {
            return nil, fmt.Errorf("circular reference detected: %s -> %s", 
                strings.Join(path, " -> "), refKey)
        }
    }
    
    // Add to path for circular detection
    newPath := append(path, refKey)
    
    // Find the resource by ref - linear search through all resource types
    resource := findResourceByRef(rs, placeholder.ResourceRef)
    if resource == nil {
        return nil, fmt.Errorf("resource not found: %s", placeholder.ResourceRef)
    }
    
    // Extract the requested field
    value, err := extractFieldValue(resource, placeholder.Field, newPath, rs)
    if err != nil {
        return nil, fmt.Errorf("extracting field %s from %s: %w", 
            placeholder.Field, placeholder.ResourceRef, err)
    }
    
    return value, nil
}

// findResourceByRef searches all resource types for a matching ref
// Uses linear search as config sets are small
func findResourceByRef(rs *resources.ResourceSet, ref string) any {
    // Search Portals
    for _, p := range rs.Portals {
        if p.Ref == ref {
            return &p
        }
    }
    
    // Search ApplicationAuthStrategies
    for _, a := range rs.ApplicationAuthStrategies {
        if a.Ref == ref {
            return &a
        }
    }
    
    // Search ControlPlanes
    for _, c := range rs.ControlPlanes {
        if c.Ref == ref {
            return &c
        }
    }
    
    // Search APIs
    for _, a := range rs.APIs {
        if a.Ref == ref {
            return &a
        }
    }
    
    // Search APIVersions
    for _, v := range rs.APIVersions {
        if v.Ref == ref {
            return &v
        }
    }
    
    // Search APIPublications
    for _, p := range rs.APIPublications {
        if p.Ref == ref {
            return &p
        }
    }
    
    // Search APIImplementations
    for _, i := range rs.APIImplementations {
        if i.Ref == ref {
            return &i
        }
    }
    
    // Search APIDocuments
    for _, d := range rs.APIDocuments {
        if d.Ref == ref {
            return &d
        }
    }
    
    // Search PortalCustomizations
    for _, c := range rs.PortalCustomizations {
        if c.Ref == ref {
            return &c
        }
    }
    
    // Search PortalCustomDomains
    for _, d := range rs.PortalCustomDomains {
        if d.Ref == ref {
            return &d
        }
    }
    
    // Search PortalPages
    for _, p := range rs.PortalPages {
        if p.Ref == ref {
            return &p
        }
    }
    
    // Search PortalSnippets
    for _, s := range rs.PortalSnippets {
        if s.Ref == ref {
            return &s
        }
    }
    
    return nil
}

// extractFieldValue extracts a field value from a resource using dot notation
func extractFieldValue(resource any, fieldPath string, path []string, rs *resources.ResourceSet) (any, error) {
    // Split field path for nested access (e.g., "metadata.labels.team")
    parts := strings.Split(fieldPath, ".")
    
    val := reflect.ValueOf(resource)
    
    // Handle pointer
    if val.Kind() == reflect.Ptr {
        if val.IsNil() {
            return nil, fmt.Errorf("resource is nil")
        }
        val = val.Elem()
    }
    
    // Navigate through the field path
    for _, part := range parts {
        if val.Kind() == reflect.Struct {
            // Find field by name (case-insensitive to be forgiving)
            field := val.FieldByNameFunc(func(name string) bool {
                return strings.EqualFold(name, part)
            })
            
            if !field.IsValid() {
                return nil, fmt.Errorf("field not found: %s", part)
            }
            
            val = field
            
            // If this field is a RefPlaceholder, we need to resolve it recursively
            if val.CanInterface() {
                if placeholder, ok := val.Interface().(*tags.RefPlaceholder); ok {
                    resolved, err := resolveReference(rs, placeholder, path)
                    if err != nil {
                        return nil, fmt.Errorf("resolving nested reference: %w", err)
                    }
                    val = reflect.ValueOf(resolved)
                }
            }
        } else if val.Kind() == reflect.Map {
            // Handle map access
            key := reflect.ValueOf(part)
            val = val.MapIndex(key)
            if !val.IsValid() {
                return nil, fmt.Errorf("map key not found: %s", part)
            }
        } else if val.Kind() == reflect.Slice || val.Kind() == reflect.Array {
            // Handle array indexing (e.g., "servers.0")
            // This would require parsing the index
            return nil, fmt.Errorf("array indexing not yet implemented: %s", part)
        } else {
            return nil, fmt.Errorf("cannot access field %s on %v", part, val.Kind())
        }
    }
    
    // Return the final value
    if val.CanInterface() {
        return val.Interface(), nil
    }
    
    return nil, fmt.Errorf("cannot extract value")
}
```

### 1.4 Update Loader Pipeline

**File**: `internal/declarative/loader/loader.go`

Modify the `parseYAML` method to add reference resolution after extraction:

```go
func (l *Loader) parseYAML(data []byte, source string, rs *resources.ResourceSet) error {
    // ... existing tag processing and unmarshaling ...
    
    // Extract nested resources (existing)
    l.extractNestedResources(&tempResult.ResourceSet)
    
    // NEW: Resolve all !ref tags
    if err := ResolveReferences(&tempResult.ResourceSet); err != nil {
        return fmt.Errorf("resolving references in %s: %w", source, err)
    }
    
    // ... rest of method ...
}
```

### 1.5 Remove Hardcoded Reference Detection

#### Files to Update:

**`internal/declarative/planner/resolver.go`**
- Delete the `isReferenceField()` method
- Delete the `getResourceTypeForField()` method  
- Delete the hardcoded `referenceFields` list
- Simplify `extractReference()` to just check for non-UUID strings

**`internal/declarative/planner/api_planner.go`**
- Remove all special handling for `portal_id` field
- Remove `resolvedPortalID` logic
- Update to use already-resolved values

**`internal/declarative/planner/portal_child_planner.go`**
- Remove all the reference field maps in operation definitions
- These fields will already contain resolved values

**`internal/declarative/executor/*.go`**
- Remove all `portal_id`, `control_plane_id` resolution logic
- Remove `resolvePortalRef()` calls
- Use values as-is since they're already resolved

### 1.6 Update Resource Validation

**Files**: `internal/declarative/resources/*.go`

Since references are now resolved before validation, update validation logic:
- Remove UUID validation for reference fields
- Fields will contain actual IDs by validation time

## Phase 2: Testing and Example Updates

### 2.1 Update Test Fixtures

All test YAML files need to be converted to use !ref syntax.

**Example conversion**:

Before:
```yaml
api_publications:
  - ref: pub1
    api: user-api
    portal_id: dev-portal
```

After:
```yaml
api_publications:
  - ref: pub1
    api: !ref user-api#ref
    portal_id: !ref dev-portal#id
```

**Files to update**:
- `test/integration/declarative/testdata/*.yaml`
- `test/e2e/testdata/declarative/**/*.yaml`
- All inline YAML in test files

### 2.2 Create New Test Cases

**File**: `test/integration/declarative/ref_tag_test.go`

Test cases to implement:
1. Basic reference resolution
2. Cross-field references (`name: !ref other-portal#name`)
3. Nested field access (`team: !ref api#metadata.labels.team`)
4. Missing reference error
5. Circular dependency detection
6. Invalid field path error

### 2.3 Update Documentation

**Files to update**:
- `docs/declarative-configuration.md` - Add !ref section
- `docs/declarative-yaml-tags.md` - Document !ref syntax
- `docs/examples/declarative/**/*.yaml` - Convert examples

## Phase 3: External Resources Implementation

### 3.1 Add External Block Types

**File**: `internal/declarative/resources/types.go`

```go
// ExternalBlock marks a resource as externally managed
type ExternalBlock struct {
    // Direct ID reference
    ID string `yaml:"id,omitempty" json:"id,omitempty"`
    
    // Selector for querying by fields
    Selector *ExternalSelector `yaml:"selector,omitempty" json:"selector,omitempty"`
}

// ExternalSelector defines field matching criteria
type ExternalSelector struct {
    // Field equality matches
    MatchFields map[string]string `yaml:"matchFields" json:"matchFields"`
}

// IsExternal returns true if this resource is externally managed
func (e *ExternalBlock) IsExternal() bool {
    return e != nil
}

// Validate ensures the external block is properly configured
func (e *ExternalBlock) Validate() error {
    if e == nil {
        return nil
    }
    
    if e.ID != "" && e.Selector != nil {
        return fmt.Errorf("_external block cannot have both 'id' and 'selector'")
    }
    
    if e.ID == "" && e.Selector == nil {
        return fmt.Errorf("_external block must have either 'id' or 'selector'")
    }
    
    if e.Selector != nil && len(e.Selector.MatchFields) == 0 {
        return fmt.Errorf("_external selector must have at least one matchField")
    }
    
    return nil
}
```

### 3.2 Update Resource Structures

Add `External` field to each resource type:

```go
type Portal struct {
    Ref         string          `yaml:"ref" json:"ref"`
    Name        string          `yaml:"name,omitempty" json:"name,omitempty"`
    Description string          `yaml:"description,omitempty" json:"description,omitempty"`
    // ... other fields ...
    
    // External resource marker
    External    *ExternalBlock  `yaml:"_external,omitempty" json:"_external,omitempty"`
}
```

Repeat for all resource types.

### 3.3 Implement External Resource Resolution

**File**: `internal/declarative/loader/external_resolver.go`

```go
package loader

import (
    "context"
    "fmt"
    
    "github.com/kong/kongctl/internal/declarative/resources"
    "github.com/kong/kongctl/internal/declarative/state"
)

// ResolveExternalResources queries and loads external resources from Konnect
func ResolveExternalResources(ctx context.Context, rs *resources.ResourceSet, client *state.Client) error {
    // Process each resource type
    if err := resolveExternalPortals(ctx, rs.Portals, client); err != nil {
        return fmt.Errorf("resolving external portals: %w", err)
    }
    
    // ... repeat for other resource types ...
    
    return nil
}

// resolveExternalPortals queries external portals and populates their data
func resolveExternalPortals(ctx context.Context, portals []resources.Portal, client *state.Client) error {
    for i := range portals {
        portal := &portals[i]
        
        if portal.External == nil {
            continue // Not external
        }
        
        var found *state.Portal
        var err error
        
        if portal.External.ID != "" {
            // Query by ID
            found, err = client.GetPortalByID(ctx, portal.External.ID)
        } else if portal.External.Selector != nil {
            // Query by selector
            found, err = queryPortalBySelector(ctx, client, portal.External.Selector)
        }
        
        if err != nil {
            return fmt.Errorf("resolving external portal %s: %w", portal.Ref, err)
        }
        
        if found == nil {
            return fmt.Errorf("external portal %s not found", portal.Ref)
        }
        
        // Populate the portal data from external source
        // Keep the ref as-is for internal references
        portal.ID = found.ID
        portal.Name = found.Name
        portal.Description = found.Description
        // ... copy other fields ...
    }
    
    return nil
}

// queryPortalBySelector searches for a portal matching the selector
func queryPortalBySelector(ctx context.Context, client *state.Client, selector *resources.ExternalSelector) (*state.Portal, error) {
    // Build filter from selector
    filter := ""
    for field, value := range selector.MatchFields {
        if filter != "" {
            filter += ","
        }
        filter += fmt.Sprintf("%s[eq]=%s", field, value)
    }
    
    portals, err := client.ListPortalsWithFilter(ctx, filter)
    if err != nil {
        return nil, err
    }
    
    if len(portals) == 0 {
        return nil, nil // Not found
    }
    
    if len(portals) > 1 {
        return nil, fmt.Errorf("selector matched %d portals, expected exactly 1", len(portals))
    }
    
    return &portals[0], nil
}
```

### 3.4 Update State Client

**File**: `internal/declarative/state/client.go`

Add methods for querying with filters:

```go
// ListPortalsWithFilter queries portals with a filter string
func (c *Client) ListPortalsWithFilter(ctx context.Context, filter string) ([]Portal, error) {
    // Implementation using Konnect SDK with filter parameter
    // Filter format: "name[eq]=value,description[contains]=text"
}

// GetPortalByID gets a portal by its ID
func (c *Client) GetPortalByID(ctx context.Context, id string) (*Portal, error) {
    // Implementation using Konnect SDK
}
```

### 3.5 Update Planner

**File**: `internal/declarative/planner/planner.go`

Skip planning for external resources:

```go
func (p *Planner) planPortals(ctx context.Context) ([]PlannedChange, error) {
    var changes []PlannedChange
    
    for _, desired := range p.desired.Portals {
        // Skip external resources - they're not managed
        if desired.External != nil && desired.External.IsExternal() {
            continue
        }
        
        // ... existing planning logic ...
    }
    
    return changes, nil
}
```

### 3.6 Update Executor

Skip operations on external resources (they're already validated to exist during resolution).

## Phase 4: Edge Cases and Validation

### 4.1 Error Messages

Enhance error reporting with source location:

```go
type RefResolutionError struct {
    SourceFile  string
    SourceLine  int
    Reference   string
    Field       string
    Underlying  error
}

func (e RefResolutionError) Error() string {
    return fmt.Sprintf("%s:%d: cannot resolve !ref %s#%s: %v",
        e.SourceFile, e.SourceLine, e.Reference, e.Field, e.Underlying)
}
```

### 4.2 Complex Field Access

Support for:
- Nested fields: `!ref portal#customization.theme.primaryColor`
- Map access: `!ref api#metadata.labels.team`
- Special fields: `!ref resource#__typename` for resource type

### 4.3 Performance Considerations

- Cache external resource queries within a single operation
- Batch API calls where the SDK supports it
- Consider parallelizing external queries for different resource types

## Implementation Checklist

### Phase 1 Checklist
- [ ] Create `internal/declarative/tags/ref.go`
- [ ] Register RefTagResolver in loader
- [ ] Create `internal/declarative/loader/ref_resolver.go`
- [ ] Update loader pipeline to call ResolveReferences
- [ ] Remove hardcoded reference detection from planner/resolver.go
- [ ] Remove portal_id handling from api_planner.go
- [ ] Remove reference maps from portal_child_planner.go
- [ ] Remove resolution logic from executor files
- [ ] Update resource validation to not expect UUIDs

### Phase 2 Checklist  
- [ ] Convert test fixtures to use !ref
- [ ] Update integration tests
- [ ] Create ref_tag_test.go with comprehensive tests
- [ ] Update documentation examples
- [ ] Add !ref documentation section

### Phase 3 Checklist
- [ ] Add ExternalBlock type
- [ ] Update all resource structs with External field
- [ ] Create external_resolver.go
- [ ] Add state client methods for filtered queries
- [ ] Update planner to skip external resources
- [ ] Update executor to skip external resources

### Phase 4 Checklist
- [ ] Implement detailed error messages with location
- [ ] Add support for nested field access
- [ ] Optimize external resource queries
- [ ] Add caching for external lookups
- [ ] Comprehensive edge case testing

## Example Configurations After Implementation

### Basic Internal References
```yaml
portals:
  - ref: dev-portal
    name: Developer Portal
    
apis:
  - ref: user-api
    name: User Management API
    
api_publications:
  - ref: publish-user
    api: !ref user-api#ref         # Instead of: api: user-api
    portal_id: !ref dev-portal#id   # Instead of: portal_id: dev-portal
```

### Cross-Field References
```yaml
portals:
  - ref: main-portal
    name: Main Portal
    description: Primary developer portal
    
  - ref: backup-portal  
    name: !ref main-portal#name     # Copy name from main-portal
    description: Backup portal with same name
```

### External Resources (Phase 3)
```yaml
# External portal managed by another team
portals:
  - ref: core-portal
    _external:
      selector:
        matchFields:
          name: "Core Team Portal"
          
# External API with managed children
apis:
  - ref: billing-api
    _external:
      id: 550e8400-e29b-41d4-a716-446655440000
    
    # These are managed by kongctl even though parent is external  
    documents:
      - ref: billing-guide
        title: Billing API Guide
        content: "How to use billing API"
        
# Reference to external resource
api_publications:
  - ref: publish-billing
    api: !ref billing-api#ref
    portal_id: !ref core-portal#id
```

## Success Metrics

1. All existing integration tests pass with new !ref syntax
2. No hardcoded reference detection remains in codebase
3. Clear error messages with file:line for reference errors
4. Documentation fully updated with examples
5. External resources can be queried and referenced (Phase 3)
6. Performance acceptable with multiple external queries

## Risks and Mitigations

| Risk | Mitigation |
|------|------------|
| Breaking existing configs | Clear documentation, pre-GA status communicated |
| Performance with many refs | Linear search is fine for small configs, can optimize later if needed |
| Complex circular dependencies | Track resolution path, fail fast with clear error |
| External resource changes | Cache during single operation, document eventual consistency |
| Missing external resources | Fail fast with clear error identifying the selector/ID |

## Notes for Implementers

1. Start with Phase 1.1 (RefTagResolver) and get basic tests working
2. Use existing FileTagResolver as a template for structure
3. The RefPlaceholder type is key - it's a marker that gets replaced
4. Resolution happens AFTER flattening - this is critical for parent-child refs
5. Keep the linear search for now - premature optimization is bad
6. Error messages should always include file and line number when possible
7. Test with existing examples early to catch issues
8. External resources (Phase 3) can be implemented independently
9. Consider keeping resolver instances for debugging unresolved refs

## Future Enhancements (Not in Scope)

- Array indexing support (`!ref api#servers.0.url`)
- Reference validation at parse time
- Optimization with ref->resource mapping if performance becomes issue
- Support for partial references (wildcards)
- GraphQL-style field selection for external resources
- Dry-run mode showing all external queries that would be made