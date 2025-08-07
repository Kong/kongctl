# External Resources Flow Analysis Report

## Executive Summary

This report provides a comprehensive analysis of the execution paths, file dependencies, and data flows for implementing external resources schema and configuration in kongctl. The analysis covers the current state architecture and identifies specific integration points where external resources will be implemented.

## Current State Architecture Flow

### 1. Configuration Loading to Resource Resolution Flow

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│  Configuration  │───▶│   YAML Parser   │───▶│   ResourceSet   │
│   Files (.yaml) │    │ (tags/file.go)  │    │  (types.go)     │
└─────────────────┘    └─────────────────┘    └─────────────────┘
                                                        │
                                                        ▼
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   SDK Queries   │◀───│   Reference     │◀───│   Validation    │
│  (client.go)    │    │   Resolution    │    │ (validation.go) │
│                 │    │ (resolver.go)   │    │                 │
└─────────────────┘    └─────────────────┘    └─────────────────┘
                                ▲                       │
                                │                       ▼
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Execution     │◀───│    Planning     │◀───│    Resource     │
│   (apply/sync)  │    │  (planner.go)   │    │   Processing    │
└─────────────────┘    └─────────────────┘    └─────────────────┘
```

### 2. Current Resource Processing Flow

```
ResourceSet Parse
        │
        ▼ 
┌─────────────────────────────────┐
│         For Each Resource Type   │
│  ┌─────┐  ┌─────┐  ┌─────┐     │
│  │ API │  │Portal│  │ CP  │ ... │
│  └─────┘  └─────┘  └─────┘     │
└─────────────────────────────────┘
        │
        ▼
┌─────────────────────────────────┐
│        Validation Phase         │
│  ┌─────────────────────────┐    │
│  │ ValidateRef()           │    │
│  │ Resource.Validate()     │    │
│  │ Length/Pattern checks   │    │
│  └─────────────────────────┘    │
└─────────────────────────────────┘
        │
        ▼
┌─────────────────────────────────┐
│        Planning Phase           │
│  ┌─────────────────────────┐    │
│  │ Compare current state   │    │
│  │ Generate PlannedChanges │    │
│  │ Build dependency graph  │    │
│  └─────────────────────────┘    │
└─────────────────────────────────┘
        │
        ▼
┌─────────────────────────────────┐
│     Reference Resolution        │
│  ┌─────────────────────────┐    │
│  │ Resolve declarative refs │    │
│  │ Map refs to Konnect IDs │    │
│  │ Update PlannedChanges   │    │
│  └─────────────────────────┘    │
└─────────────────────────────────┘
        │
        ▼
┌─────────────────────────────────┐
│         Execution               │
│  ┌─────────────────────────┐    │
│  │ Create/Update/Delete    │    │
│  │ SDK API calls           │    │
│  │ State management        │    │
│  └─────────────────────────┘    │
└─────────────────────────────────┘
```

### 3. Current Reference Resolution Flow

```
┌──────────────────┐
│  PlannedChanges  │
│  (from Planner)  │
└────────┬─────────┘
         │
         ▼
┌──────────────────┐    ┌─────────────────────┐
│  ReferenceResolver│───▶│   Field Detection   │
│  (resolver.go)   │    │   - Scan for refs   │
└────────┬─────────┘    │   - Type mapping    │
         │              └─────────────────────┘
         ▼                         │
┌──────────────────┐              │
│    SDK Queries   │◀─────────────┘
│  - List resources│
│  - Filter by ref │
│  - Get IDs       │
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│  Updated         │
│  PlannedChanges  │
│  (refs → IDs)    │
└──────────────────┘
```

## File Dependencies and Interconnections

### 1. Core File Dependency Map

```
types.go (ResourceSet)
    │
    ├─▶ external_resource.go (NEW)
    │
    ├─▶ api.go (APIResource)
    │
    ├─▶ portal.go (PortalResource)
    │
    └─▶ control_plane.go (ControlPlaneResource)
    
validation.go
    │
    ├─▶ types.go (interfaces)
    │
    └─▶ external_resource.go (NEW validation)

planner.go
    │
    ├─▶ types.go (ResourceSet)
    │
    ├─▶ resolver.go (ReferenceResolver)
    │
    └─▶ external/resolver.go (NEW)

resolver.go
    │
    ├─▶ client.go (SDK queries)
    │
    └─▶ types.go (resource types)

client.go
    │
    ├─▶ SDK packages (konnect-go)
    │
    └─▶ external/client.go (NEW)
```

### 2. Data Flow Between Components

```
Configuration Data Flow:
YAML/JSON ──▶ ResourceSet ──▶ Individual Resources ──▶ Validation ──▶ Planning

Reference Data Flow:
Resources ──▶ Field Scanning ──▶ SDK Queries ──▶ ID Resolution ──▶ Updated Resources

State Data Flow:
Current State ──▶ Comparison ──▶ PlannedChanges ──▶ Execution ──▶ New State
```

## External Resources Integration Points

### 1. New External Resource Flow (To Be Implemented)

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│  Configuration  │───▶│   YAML Parser   │───▶│   ResourceSet   │
│   Files (.yaml) │    │ (tags/file.go)  │    │  (types.go)     │
└─────────────────┘    └─────────────────┘    └─────────────────┘
                                                        │
                                                        ▼
┌─────────────────┐                        ┌─────────────────┐
│   Validation    │◀───────────────────────│    Resource     │
│ (validation.go) │                        │   Processing    │
│                 │                        │                 │
└─────────┬───────┘                        └─────────────────┘
          │                                         │
          ▼                                         ▼
┌─────────────────┐                        ┌─────────────────┐
│   EXTERNAL      │                        │   Normal Flow   │
│   RESOURCE      │                        │   (existing)    │
│   RESOLUTION    │                        │                 │
│   (NEW PHASE)   │                        │                 │
└─────────┬───────┘                        └─────────┬───────┘
          │                                          │
          ▼                                          │
┌─────────────────┐                                  │
│  External SDK   │                                  │
│  Queries        │                                  │
│  - ID lookups   │                                  │
│  - Selector     │                                  │
│    filtering    │                                  │
│  - Parent       │                                  │
│    resolution   │                                  │
└─────────┬───────┘                                  │
          │                                          │
          ▼                                          │
┌─────────────────┐                                  │
│  Resolved       │──────────────────────────────────┤
│  External       │                                  │
│  Resources      │                                  │
│  (Cache)        │                                  ▼
└─────────────────┘                        ┌─────────────────┐
                                           │    Planning     │
                                           │  (planner.go)   │
                                           └─────────┬───────┘
                                                     │
                                                     ▼
                                           ┌─────────────────┐
                                           │   Reference     │
                                           │   Resolution    │
                                           │ (resolver.go)   │
                                           │ + External Data │
                                           └─────────┬───────┘
                                                     │
                                                     ▼
                                           ┌─────────────────┐
                                           │   Execution     │
                                           │  (apply/sync)   │
                                           └─────────────────┘
```

### 2. External Resource Resolution Detail Flow

```
External Resources Block
         │
         ▼
┌─────────────────────────────────────┐
│         Validation Phase            │
│  ┌─────────────────────────────┐    │
│  │ ID XOR Selector required    │    │
│  │ Valid resource_type         │    │
│  │ Parent relationship check   │    │
│  │ Selector field validation   │    │
│  └─────────────────────────────┘    │
└─────────────────┬───────────────────┘
                  │
                  ▼
┌─────────────────────────────────────┐
│       Dependency Analysis           │
│  ┌─────────────────────────────┐    │
│  │ Build external resource     │    │
│  │   dependency graph          │    │
│  │ Topological sort for order  │    │
│  │ Handle parent dependencies  │    │
│  └─────────────────────────────┘    │
└─────────────────┬───────────────────┘
                  │
                  ▼
┌─────────────────────────────────────┐
│          Resolution Loop            │
│                                     │
│  For each External Resource:        │
│  ┌─────────────────────────────┐    │
│  │ 1. Resolve parent (if any)  │    │
│  │ 2. Direct ID lookup OR      │    │
│  │    Selector-based query     │    │
│  │ 3. Validate single result   │    │
│  │ 4. Cache resolved resource  │    │
│  └─────────────────────────────┘    │
└─────────────────┬───────────────────┘
                  │
                  ▼
┌─────────────────────────────────────┐
│      External Resource Cache        │
│  ┌─────────────────────────────┐    │
│  │ ref → resolved_resource     │    │
│  │ ref → resolved_id           │    │
│  │ Available for normal flow   │    │
│  └─────────────────────────────┘    │
└─────────────────────────────────────┘
```

### 3. Schema Integration Points

#### 3.1 ResourceSet Extension (types.go)

```go
// BEFORE (current)
type ResourceSet struct {
    Portals    []PortalResource    `yaml:"portals,omitempty"`
    APIs       []APIResource       `yaml:"apis,omitempty"`
    // ... other resources
}

// AFTER (with external resources)
type ResourceSet struct {
    ExternalResources []ExternalResourceResource `yaml:"external_resources,omitempty"` // NEW
    Portals          []PortalResource           `yaml:"portals,omitempty"`
    APIs             []APIResource              `yaml:"apis,omitempty"`
    // ... other resources
}
```

#### 3.2 Validation Integration (validation.go)

```
Current Validation Flow:
ResourceSet ──▶ Individual Resource Validation ──▶ Ref Validation

New Validation Flow:
ResourceSet ──▶ External Resource Validation (NEW) ──┬──▶ Individual Resource Validation ──▶ Ref Validation
                                                     │
External Resource Validation:                        │
  - ID XOR Selector validation                      │
  - Resource type validation                        │
  - Parent field validation                         │
  - Selector match_fields validation                │
```

#### 3.3 Resolution Integration (planner.go + resolver.go)

```
Current Resolution Order:
Planning ──▶ Reference Resolution ──▶ Execution

New Resolution Order:
Planning ──▶ External Resource Resolution (NEW) ──▶ Reference Resolution ──▶ Execution
                          │                                    │
                          ▼                                    │
                   Resolved External                           │
                   Resources Cache ─────────────────────────────┘
```

## New Components and Files

### 1. New File Structure

```
internal/declarative/
├── resources/
│   ├── types.go                    # Modified: Add external_resources field
│   ├── external_resource.go        # NEW: ExternalResourceResource struct
│   └── validation.go               # Modified: Add external resource validation
├── external/                       # NEW PACKAGE
│   ├── types.go                    # NEW: External resource type definitions
│   ├── registry.go                 # NEW: Resource type registry
│   ├── resolver.go                 # NEW: External resource resolver
│   └── client.go                   # NEW: External resource SDK queries
├── planner/
│   ├── planner.go                  # Modified: Add external resource phase
│   └── resolver.go                 # Modified: Access external resource cache
└── state/
    └── client.go                   # Modified: Add external resource methods
```

### 2. New External Resource Components Detail

#### 2.1 External Resource Registry (external/registry.go)

```
┌─────────────────────────────────────┐
│         ResourceTypeRegistry        │
│  ┌─────────────────────────────┐    │
│  │ Map resource_type string to:│    │
│  │  - SDK query interface      │    │
│  │  - Supported fields         │    │
│  │  - Parent relationships     │    │
│  │  - Validation rules         │    │
│  └─────────────────────────────┘    │
└─────────────────────────────────────┘
```

#### 2.2 External Resource Resolver (external/resolver.go)

```
┌─────────────────────────────────────┐
│         ExternalResourceResolver    │
│  ┌─────────────────────────────┐    │
│  │ Process external_resources  │    │
│  │ Build dependency graph      │    │
│  │ Resolve in topological order│   │
│  │ Cache resolved resources    │    │
│  │ Handle resolution errors    │    │
│  └─────────────────────────────┘    │
└─────────────────────────────────────┘
```

#### 2.3 External Resource Client (external/client.go)

```
┌─────────────────────────────────────┐
│         ExternalResourceClient      │
│  ┌─────────────────────────────┐    │
│  │ Generic query interface     │    │
│  │ Resource-specific adapters  │    │
│  │ ID lookup methods           │    │
│  │ Selector filtering methods  │    │
│  │ Parent-child queries        │    │
│  └─────────────────────────────┘    │
└─────────────────────────────────────┘
```

## Impact Analysis on Existing Flows

### 1. Configuration Loading Impact

**Files Affected:**
- `internal/declarative/resources/types.go` - Add external_resources field
- `internal/declarative/tags/file.go` - No changes needed (uses reflection)

**Impact Level:** LOW
- Only schema addition, no logic changes
- YAML parsing automatically handles new field

### 2. Validation Impact

**Files Affected:**
- `internal/declarative/resources/validation.go` - Add external resource validation
- `internal/declarative/resources/external_resource.go` - New validation methods

**Impact Level:** MEDIUM
- New validation rules for external resources
- Integration with existing validation interface
- Additional error cases to handle

### 3. Planning Impact

**Files Affected:**
- `internal/declarative/planner/planner.go` - Add external resource resolution phase

**Impact Level:** HIGH
- New phase added before existing planning
- Integration with dependency graph
- Error handling for resolution failures
- Performance impact of additional SDK calls

### 4. Reference Resolution Impact

**Files Affected:**
- `internal/declarative/planner/resolver.go` - Access to external resource cache

**Impact Level:** MEDIUM
- Integration with resolved external resources
- Enhanced reference resolution capabilities
- Additional lookup sources

### 5. SDK Integration Impact

**Files Affected:**
- `internal/declarative/state/client.go` - Add external resource query methods
- `internal/declarative/external/client.go` - New external resource client

**Impact Level:** HIGH
- New generic query interface
- Resource-specific SDK adapters
- Enhanced error handling
- Performance considerations for multiple queries

## Integration Sequence and Dependencies

### Phase 1: Schema and Validation Foundation
1. Add external_resources field to ResourceSet
2. Create ExternalResourceResource struct
3. Implement validation rules
4. Add basic parsing capability

### Phase 2: Resolution Infrastructure
1. Create external resource registry
2. Implement external resource resolver
3. Add SDK client adapters
4. Create resolution cache system

### Phase 3: Integration with Existing Flows
1. Integrate external resource resolution in planner
2. Connect resolved resources to reference resolution
3. Add error handling and recovery
4. Performance optimization

### Phase 4: Testing and Documentation
1. Unit tests for new components
2. Integration tests for full flow
3. Error case testing
4. Performance testing
5. Documentation updates

## Risk Analysis and Mitigation

### 1. Performance Risks
**Risk:** Multiple SDK queries slow down planning
**Mitigation:** Caching, batch operations, parallel queries

### 2. Complexity Risks
**Risk:** Dependency resolution becomes too complex
**Mitigation:** Clear separation of concerns, comprehensive testing

### 3. Error Handling Risks
**Risk:** Poor error messages for resolution failures
**Mitigation:** Structured error types with context, user-friendly messages

### 4. Extensibility Risks
**Risk:** Hard to add new resource types
**Mitigation:** Registry-based architecture, configuration-driven mappings

## Conclusion

The implementation of external resources requires careful integration at multiple points in the existing flow:

1. **Schema Extension**: Minimal impact to ResourceSet and validation
2. **New Resolution Phase**: Significant new component before planning
3. **Enhanced Reference Resolution**: Integration with resolved external resources
4. **SDK Integration**: New generic query interface and adapters

The architecture maintains consistency with existing patterns while providing the flexibility needed for external resource resolution across different resource types and hierarchical relationships. The phased implementation approach minimizes risk and allows for iterative development and testing.