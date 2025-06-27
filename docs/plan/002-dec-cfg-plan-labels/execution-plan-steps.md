# Stage 2 Execution Plan: Detailed Steps

## Progress Summary

| Step | Description | Status | Dependencies |
|------|-------------|--------|--------------|
| 1 | Extend API Interfaces | Completed | None |
| 2 | Implement Label Utilities | Completed | None |
| 3 | Create State Client Wrapper | Completed | Step 1 |
| 4 | Implement Config Hash | Completed | Step 2 |
| 5 | Define Plan Types | Completed | None |
| 6 | Implement Reference Resolver | Completed | Step 3 |
| 7 | Implement Dependency Resolution | Completed | Step 5 |
| 8 | Create Planner Core Logic | Completed | Steps 4, 5, 6, 7 |
| 9 | Update Plan Command | Completed | Step 8 |
| 10 | Implement Diff Command | Not Started | Step 5 |
| 11 | Add Integration Tests | Not Started | Steps 9, 10 |

---

## Step 1: Extend API Interfaces

### Status
Completed

### Dependencies
None

### Changes
- **Files**: 
  - `internal/konnect/helpers/portals.go` - Extend PortalAPI
  - `internal/konnect/helpers/auth.go` - Extend AppAuthStrategiesAPI
  - Create additional helper files as needed for other APIs

### Implementation

#### PortalAPI Extensions
```go
// Add to PortalAPI interface
CreatePortal(ctx context.Context, portal kkInternalComps.CreatePortal) (*kkInternalOps.CreatePortalResponse, error)
UpdatePortal(ctx context.Context, id string, portal kkInternalComps.UpdatePortal) (*kkInternalOps.UpdatePortalResponse, error)

// Implement in InternalPortalAPI
func (p *InternalPortalAPI) CreatePortal(
    ctx context.Context,
    portal kkInternalComps.CreatePortal,
) (*kkInternalOps.CreatePortalResponse, error) {
    return p.SDK.Portals.CreatePortal(ctx, portal)
}

func (p *InternalPortalAPI) UpdatePortal(
    ctx context.Context,
    id string,
    portal kkInternalComps.UpdatePortal,
) (*kkInternalOps.UpdatePortalResponse, error) {
    req := kkInternalOps.UpdatePortalRequest{
        PortalID: id,
        UpdatePortal: portal,
    }
    return p.SDK.Portals.UpdatePortal(ctx, req)
}
```

#### AppAuthStrategiesAPI Extensions
```go
// Add to AppAuthStrategiesAPI interface
GetAppAuthStrategy(ctx context.Context, id string) (*kkOps.GetAppAuthStrategyResponse, error)
CreateAppAuthStrategy(ctx context.Context, strategy kkComps.CreateApplicationAuthStrategy) (*kkOps.CreateAppAuthStrategyResponse, error)
UpdateAppAuthStrategy(ctx context.Context, id string, strategy kkComps.UpdateApplicationAuthStrategy) (*kkOps.UpdateAppAuthStrategyResponse, error)

// Implement similar methods for InternalAppAuthStrategiesAPI
```

### Tests
- Mock SDK responses for all CRUD operations
- Error handling for each interface
- Verify correct SDK method delegation

### Commit Message
```
feat(konnect): extend API interfaces with CRUD operations

Extend PortalAPI, AppAuthStrategiesAPI, and other interfaces to support
full CRUD operations needed for plan generation and execution
```

---

## Step 2: Implement Label Utilities

### Status
Completed

### Dependencies
None

### Changes
- Create file: `internal/declarative/labels/labels.go`
- Implement label constants and manipulation functions

### Implementation
```go
package labels

import (
    "fmt"
    "time"
)

// Label keys used by kongctl
const (
    ManagedKey     = "KONGCTL/managed"
    ConfigHashKey  = "KONGCTL/config-hash"
    LastUpdatedKey = "KONGCTL/last-updated"
    ProtectedKey   = "KONGCTL/protected"
)

// NormalizeLabels converts pointer map to non-pointer map
func NormalizeLabels(labels map[string]*string) map[string]string {
    if labels == nil {
        return make(map[string]string)
    }
    
    normalized := make(map[string]string)
    for k, v := range labels {
        if v != nil {
            normalized[k] = *v
        }
    }
    return normalized
}

// DenormalizeLabels converts non-pointer map to pointer map for SDK
func DenormalizeLabels(labels map[string]string) map[string]*string {
    if labels == nil {
        return make(map[string]*string)
    }
    
    denormalized := make(map[string]*string)
    for k, v := range labels {
        v := v // capture loop variable
        denormalized[k] = &v
    }
    return denormalized
}

// AddManagedLabels adds kongctl management labels
func AddManagedLabels(labels map[string]string, configHash string) map[string]string {
    if labels == nil {
        labels = make(map[string]string)
    }
    
    // Preserve existing labels
    result := make(map[string]string)
    for k, v := range labels {
        result[k] = v
    }
    
    // Add management labels
    result[ManagedKey] = "true"
    result[ConfigHashKey] = configHash
    result[LastUpdatedKey] = time.Now().UTC().Format(time.RFC3339)
    
    return result
}

// IsManagedResource checks if resource has managed label
func IsManagedResource(labels map[string]string) bool {
    return labels != nil && labels[ManagedKey] == "true"
}

// GetUserLabels returns labels without KONGCTL prefix
func GetUserLabels(labels map[string]string) map[string]string {
    user := make(map[string]string)
    for k, v := range labels {
        if !IsKongctlLabel(k) {
            user[k] = v
        }
    }
    return user
}

// IsKongctlLabel checks if label key is kongctl-managed
func IsKongctlLabel(key string) bool {
    return len(key) >= 8 && key[:8] == "KONGCTL/"
}

// ValidateLabel ensures label key follows Konnect rules
func ValidateLabel(key string) error {
    if len(key) < 1 || len(key) > 63 {
        return fmt.Errorf("label key must be 1-63 characters: %s", key)
    }
    
    // Check forbidden prefixes
    forbidden := []string{"kong", "konnect", "mesh", "kic", "_"}
    for _, prefix := range forbidden {
        if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
            return fmt.Errorf("label key cannot start with %s: %s", prefix, key)
        }
    }
    
    return nil
}
```

### Tests
- Label normalization with various inputs
- Label validation rules
- KONGCTL label filtering
- Timestamp formatting

### Commit Message
```
feat(labels): implement label management utilities

Add functions for label normalization, validation, and KONGCTL-specific
label management
```

---

## Step 3: Create State Client Wrapper

### Status
Completed

### Dependencies
Step 1

### Changes
- Create directory: `internal/declarative/state/`
- Create file: `internal/declarative/state/client.go`

### Implementation
```go
package state

import (
    "context"
    "fmt"
    
    "github.com/kong/kongctl/internal/declarative/labels"
    "github.com/kong/kongctl/internal/konnect/helpers"
    kkInternalComps "github.com/Kong/sdk-konnect-go-internal/models/components"
    kkInternalOps "github.com/Kong/sdk-konnect-go-internal/models/operations"
)

// Client wraps Konnect SDK for state management
type Client struct {
    portalAPI helpers.PortalAPI
}

// NewClient creates a new state client
func NewClient(portalAPI helpers.PortalAPI) *Client {
    return &Client{
        portalAPI: portalAPI,
    }
}

// Portal represents a normalized portal for internal use
type Portal struct {
    kkInternalComps.PortalResponse
    NormalizedLabels map[string]string // Non-pointer labels
}

// ListManagedPortals returns all KONGCTL-managed portals
func (c *Client) ListManagedPortals(ctx context.Context) ([]Portal, error) {
    var allPortals []Portal
    var pageNumber int64 = 1
    const pageSize int64 = 100
    
    for {
        req := kkInternalOps.ListPortalsRequest{
            PageSize:   &pageSize,
            PageNumber: &pageNumber,
        }
        
        resp, err := c.portalAPI.ListPortals(ctx, req)
        if err != nil {
            return nil, fmt.Errorf("failed to list portals: %w", err)
        }
        
        if resp.ListPortalsResponse == nil || len(resp.ListPortalsResponse.Data) == 0 {
            break
        }
        
        // Process and filter portals
        for _, p := range resp.ListPortalsResponse.Data {
            normalized := labels.NormalizeLabels(p.Labels)
            
            if labels.IsManagedResource(normalized) {
                portal := Portal{
                    PortalResponse:   p,
                    NormalizedLabels: normalized,
                }
                allPortals = append(allPortals, portal)
            }
        }
        
        pageNumber++
        
        // Check if we've fetched all pages
        if resp.ListPortalsResponse.Meta != nil && 
           resp.ListPortalsResponse.Meta.Page != nil &&
           resp.ListPortalsResponse.Meta.Page.Total <= float64(pageSize*(pageNumber-1)) {
            break
        }
    }
    
    return allPortals, nil
}

// GetPortalByName finds a managed portal by name
func (c *Client) GetPortalByName(ctx context.Context, name string) (*Portal, error) {
    portals, err := c.ListManagedPortals(ctx)
    if err != nil {
        return nil, err
    }
    
    for _, p := range portals {
        if p.Name == name {
            return &p, nil
        }
    }
    
    return nil, nil // Not found
}

// CreatePortal creates a new portal with management labels
func (c *Client) CreatePortal(ctx context.Context, portal kkInternalComps.CreatePortal, configHash string) (*kkInternalComps.PortalResponse, error) {
    // Add management labels
    normalized := labels.NormalizeLabels(portal.Labels)
    normalized = labels.AddManagedLabels(normalized, configHash)
    portal.Labels = labels.DenormalizeLabels(normalized)
    
    resp, err := c.portalAPI.CreatePortal(ctx, portal)
    if err != nil {
        return nil, fmt.Errorf("failed to create portal: %w", err)
    }
    
    if resp.Portal == nil {
        return nil, fmt.Errorf("create portal response missing portal data")
    }
    
    return resp.Portal, nil
}

// UpdatePortal updates an existing portal with new management labels
func (c *Client) UpdatePortal(ctx context.Context, id string, portal kkInternalComps.UpdatePortal, configHash string) (*kkInternalComps.PortalResponse, error) {
    // Add management labels
    normalized := labels.NormalizeLabels(portal.Labels)
    normalized = labels.AddManagedLabels(normalized, configHash)
    portal.Labels = labels.DenormalizeLabels(normalized)
    
    resp, err := c.portalAPI.UpdatePortal(ctx, id, portal)
    if err != nil {
        return nil, fmt.Errorf("failed to update portal: %w", err)
    }
    
    if resp.Portal == nil {
        return nil, fmt.Errorf("update portal response missing portal data")
    }
    
    return resp.Portal, nil
}
```

### Tests
- Pagination with multiple pages
- Label filtering logic
- Portal name lookup
- Create/update with labels

### Commit Message
```
feat(state): implement Konnect state client wrapper

Add client for fetching and managing KONGCTL-managed resources with
label normalization and filtering
```

---

## Step 4: Implement Config Hash

### Status
Completed

### Dependencies
Step 2

### Changes
- Create file: `internal/declarative/hash/hash.go`
- Implement deterministic configuration hashing

### Implementation
```go
package hash

import (
    "crypto/sha256"
    "encoding/base64"
    "encoding/json"
    "fmt"
    "strings"
    
    kkInternalComps "github.com/Kong/sdk-konnect-go-internal/models/components"
)

// CalculateResourceHash computes a deterministic hash for any resource
func CalculateResourceHash(resource interface{}) (string, error) {
    // Step 1: Use json.Marshal for serialization
    jsonBytes, err := json.Marshal(resource)
    if err != nil {
        return "", fmt.Errorf("failed to marshal resource: %w", err)
    }
    
    // Step 2: Parse into generic map to filter fields
    var data map[string]interface{}
    if err := json.Unmarshal(jsonBytes, &data); err != nil {
        return "", fmt.Errorf("failed to unmarshal for filtering: %w", err)
    }
    
    // Step 3: Filter out system fields and KONGCTL labels
    filtered := filterForHashing(data)
    
    // Step 4: Re-marshal with deterministic ordering
    // json.Marshal on maps already sorts keys alphabetically
    canonicalJSON, err := json.Marshal(filtered)
    if err != nil {
        return "", fmt.Errorf("failed to marshal filtered data: %w", err)
    }
    
    // Step 5: Calculate SHA256 hash
    hash := sha256.Sum256(canonicalJSON)
    return base64.StdEncoding.EncodeToString(hash[:]), nil
}

// Specific resource hash functions for type safety
func CalculatePortalHash(portal kkInternalComps.CreatePortal) (string, error) {
    return CalculateResourceHash(portal)
}

func CalculateAPIHash(api kkInternalComps.CreateAPIRequest) (string, error) {
    return CalculateResourceHash(api)
}

func CalculateAPIVersionHash(version kkInternalComps.CreateAPIVersionRequest) (string, error) {
    return CalculateResourceHash(version)
}

func CalculateAPIDocumentHash(doc kkInternalComps.CreateAPIDocumentRequest) (string, error) {
    return CalculateResourceHash(doc)
}
```

### Tests
- Hash consistency with same input
- Hash differences with changed fields
- Label filtering (KONGCTL vs user)
- Nil pointer handling
- Deterministic ordering

### Commit Message
```
feat(hash): implement generic configuration hash calculation

Refactor hash implementation to use a generic approach that leverages
Go's json.Marshal for consistent serialization. This eliminates the need
for per-resource hash functions and automatically adapts to SDK changes.

- Add CalculateResourceHash() as single generic function
- Support all portal-related resources (Portal, API, APIVersion, APIDocument)
- Filter system fields and KONGCTL labels
- Ensure deterministic output with sorted JSON keys
- Add comprehensive tests for determinism and filtering
```

---

## Step 5: Define Plan Types

### Status
Completed

### Dependencies
None

### Changes
- Create directory: `internal/declarative/planner/`
- Create file: `internal/declarative/planner/types.go`

### Implementation
```go
package planner

import (
    "time"
)

// Plan represents a declarative configuration plan
type Plan struct {
    Metadata       PlanMetadata    `json:"metadata"`
    Changes        []PlannedChange `json:"changes"`
    ExecutionOrder []string        `json:"execution_order"`
    Summary        PlanSummary     `json:"summary"`
    Warnings       []PlanWarning   `json:"warnings,omitempty"`
}

// PlanMetadata contains plan generation information
type PlanMetadata struct {
    Version     string    `json:"version"`
    GeneratedAt time.Time `json:"generated_at"`
    Generator   string    `json:"generator"`
}

// PlannedChange represents a single resource change
type PlannedChange struct {
    ID           string                    `json:"id"`
    ResourceType string                    `json:"resource_type"`
    ResourceRef  string                    `json:"resource_ref"`
    ResourceID   string                    `json:"resource_id,omitempty"` // Only for UPDATE/DELETE
    Action       ActionType                `json:"action"`
    Fields       map[string]interface{}    `json:"fields"`
    References   map[string]ReferenceInfo  `json:"references,omitempty"`
    Parent       *ParentInfo               `json:"parent,omitempty"`
    Protection   interface{}               `json:"protection,omitempty"` // bool or ProtectionChange
    ConfigHash   string                    `json:"config_hash"`
    DependsOn    []string                  `json:"depends_on,omitempty"`
}

// ReferenceInfo tracks reference resolution
type ReferenceInfo struct {
    Ref string `json:"ref"`
    ID  string `json:"id"` // May be "<unknown>" for resources in same plan
}

// ParentInfo tracks parent relationships
type ParentInfo struct {
    Ref string `json:"ref"`
    ID  string `json:"id"` // May be "<unknown>" for parents in same plan
}

// ProtectionChange tracks protection status changes
type ProtectionChange struct {
    Old bool `json:"old"`
    New bool `json:"new"`
}

// FieldChange represents a single field modification (for UPDATE)
type FieldChange struct {
    Old interface{} `json:"old"`
    New interface{} `json:"new"`
}

// ActionType represents the type of change
type ActionType string

const (
    ActionCreate ActionType = "CREATE"
    ActionUpdate ActionType = "UPDATE"
    ActionDelete ActionType = "DELETE" // Future
)

// PlanSummary provides overview statistics
type PlanSummary struct {
    TotalChanges      int                `json:"total_changes"`
    ByAction          map[ActionType]int `json:"by_action"`
    ByResource        map[string]int     `json:"by_resource"`
    ProtectionChanges *ProtectionSummary `json:"protection_changes,omitempty"`
}

// ProtectionSummary tracks protection changes
type ProtectionSummary struct {
    Protecting   int `json:"protecting"`
    Unprotecting int `json:"unprotecting"`
}

// PlanWarning represents a warning about the plan
type PlanWarning struct {
    ChangeID string `json:"change_id"`
    Message  string `json:"message"`
}

// NewPlan creates a new plan with metadata
func NewPlan(version, generator string) *Plan {
    return &Plan{
        Metadata: PlanMetadata{
            Version:     version,
            GeneratedAt: time.Now().UTC(),
            Generator:   generator,
        },
        Changes:        []PlannedChange{},
        ExecutionOrder: []string{},
        Summary: PlanSummary{
            ByAction:   make(map[ActionType]int),
            ByResource: make(map[string]int),
        },
        Warnings: []PlanWarning{},
    }
}

// AddChange adds a change to the plan
func (p *Plan) AddChange(change PlannedChange) {
    p.Changes = append(p.Changes, change)
    p.updateSummary()
}

// SetExecutionOrder sets the calculated execution order
func (p *Plan) SetExecutionOrder(order []string) {
    p.ExecutionOrder = order
}

// AddWarning adds a warning to the plan
func (p *Plan) AddWarning(changeID, message string) {
    p.Warnings = append(p.Warnings, PlanWarning{
        ChangeID: changeID,
        Message:  message,
    })
}

// updateSummary recalculates plan statistics
func (p *Plan) updateSummary() {
    p.Summary.TotalChanges = len(p.Changes)
    
    // Reset counts
    p.Summary.ByAction = make(map[ActionType]int)
    p.Summary.ByResource = make(map[string]int)
    protectionSummary := &ProtectionSummary{}
    
    // Count by action and resource type
    for _, change := range p.Changes {
        p.Summary.ByAction[change.Action]++
        p.Summary.ByResource[change.ResourceType]++
        
        // Track protection changes
        switch v := change.Protection.(type) {
        case bool:
            if v && change.Action == ActionCreate {
                protectionSummary.Protecting++
            }
        case ProtectionChange:
            if !v.Old && v.New {
                protectionSummary.Protecting++
            } else if v.Old && !v.New {
                protectionSummary.Unprotecting++
            }
        }
    }
    
    if protectionSummary.Protecting > 0 || protectionSummary.Unprotecting > 0 {
        p.Summary.ProtectionChanges = protectionSummary
    }
}

// IsEmpty returns true if plan has no changes
func (p *Plan) IsEmpty() bool {
    return len(p.Changes) == 0
}
```

### Tests
- Plan creation and initialization
- Change addition and summary updates
- JSON serialization/deserialization

### Commit Message
```
feat(planner): define plan types and structures

Add data structures for representing declarative configuration plans
with metadata, changes, and summaries
```

---

## Step 6: Implement Reference Resolver

### Status
Completed

### Dependencies
Step 3

### Changes
- Create file: `internal/declarative/planner/resolver.go`
- Implement reference to ID resolution

### Implementation
```go
package planner

import (
    "context"
    "fmt"
    
    "github.com/kong/kongctl/internal/declarative/resources"
    "github.com/kong/kongctl/internal/declarative/state"
)

// ReferenceResolver resolves declarative refs to Konnect IDs
type ReferenceResolver struct {
    client *state.Client
}

// NewReferenceResolver creates a new resolver
func NewReferenceResolver(client *state.Client) *ReferenceResolver {
    return &ReferenceResolver{
        client: client,
    }
}

// ResolvedReference contains ref and resolved ID
type ResolvedReference struct {
    Ref string
    ID  string
}

// ResolveResult contains resolved reference information
type ResolveResult struct {
    // Map of change_id -> field -> resolved reference
    ChangeReferences map[string]map[string]ResolvedReference
    // Errors encountered during resolution
    Errors []error
}

// ResolveReferences resolves all references in planned changes
func (r *ReferenceResolver) ResolveReferences(ctx context.Context, changes []PlannedChange) (*ResolveResult, error) {
    result := &ResolveResult{
        ChangeReferences: make(map[string]map[string]ResolvedReference),
        Errors:           []error{},
    }
    
    // Build a map of what's being created in this plan
    createdResources := make(map[string]map[string]string) // resource_type -> ref -> change_id
    for _, change := range changes {
        if change.Action == ActionCreate {
            if createdResources[change.ResourceType] == nil {
                createdResources[change.ResourceType] = make(map[string]string)
            }
            createdResources[change.ResourceType][change.ResourceRef] = change.ID
        }
    }
    
    // Resolve references for each change
    for _, change := range changes {
        changeRefs := make(map[string]ResolvedReference)
        
        // Check fields that might contain references
        for fieldName, fieldValue := range change.Fields {
            if ref, isRef := r.extractReference(fieldName, fieldValue); isRef {
                // Determine resource type from field name
                resourceType := r.getResourceTypeForField(fieldName)
                
                // Check if this references something being created
                if changeID, inPlan := createdResources[resourceType][ref]; inPlan {
                    changeRefs[fieldName] = ResolvedReference{
                        Ref: ref,
                        ID:  "<unknown>", // Will be resolved at execution
                    }
                } else {
                    // Resolve from existing resources
                    id, err := r.resolveReference(ctx, resourceType, ref)
                    if err != nil {
                        result.Errors = append(result.Errors, fmt.Errorf(
                            "change %s: failed to resolve %s reference %q: %w",
                            change.ID, resourceType, ref, err))
                        continue
                    }
                    changeRefs[fieldName] = ResolvedReference{
                        Ref: ref,
                        ID:  id,
                    }
                }
            }
        }
        
        if len(changeRefs) > 0 {
            result.ChangeReferences[change.ID] = changeRefs
        }
    }
    
    return result, nil
}

// extractReference checks if a field value is a reference
func (r *ReferenceResolver) extractReference(fieldName string, value interface{}) (string, bool) {
    // Check if field name suggests a reference
    if !r.isReferenceField(fieldName) {
        return "", false
    }
    
    // Extract string value
    switch v := value.(type) {
    case string:
        if !isUUID(v) {
            return v, true
        }
    case FieldChange:
        if newVal, ok := v.New.(string); ok && !isUUID(newVal) {
            return newVal, true
        }
    }
    
    return "", false
}

// isReferenceField checks if field name indicates a reference
func (r *ReferenceResolver) isReferenceField(fieldName string) bool {
    // Fields that contain references to other resources
    referenceFields := []string{
        "default_application_auth_strategy_id",
        "control_plane_id",
        "portal_id",
        "auth_strategy_ids",
        // Add more as needed
    }
    
    for _, rf := range referenceFields {
        if fieldName == rf || 
           fieldName == "gateway_service."+rf ||
           fieldName == "gateway_service.service_id" {
            return true
        }
    }
    return false
}

// getResourceTypeForField maps field names to resource types
func (r *ReferenceResolver) getResourceTypeForField(fieldName string) string {
    switch fieldName {
    case "default_application_auth_strategy_id", "auth_strategy_ids":
        return "application_auth_strategy"
    case "control_plane_id", "gateway_service.control_plane_id":
        return "control_plane"
    case "portal_id":
        return "portal"
    default:
        return ""
    }
}

// resolveReference looks up a reference in existing resources
func (r *ReferenceResolver) resolveReference(ctx context.Context, resourceType, ref string) (string, error) {
    switch resourceType {
    case "application_auth_strategy":
        return r.resolveAuthStrategyRef(ctx, ref)
    case "control_plane":
        return r.resolveControlPlaneRef(ctx, ref)
    case "portal":
        return r.resolvePortalRef(ctx, ref)
    default:
        return "", fmt.Errorf("unknown resource type: %s", resourceType)
    }
}

// resolveAuthStrategyRef resolves auth strategy ref to ID
func (r *ReferenceResolver) resolveAuthStrategyRef(ctx context.Context, ref string) (string, error) {
    // TODO: Implement when auth strategy API is available
    return "", fmt.Errorf("auth strategy resolution not yet implemented")
}

// resolveControlPlaneRef resolves control plane ref to ID
func (r *ReferenceResolver) resolveControlPlaneRef(ctx context.Context, ref string) (string, error) {
    // TODO: Implement when control plane state client is available
    return "", fmt.Errorf("control plane resolution not yet implemented")
}

// resolvePortalRef resolves portal ref to ID
func (r *ReferenceResolver) resolvePortalRef(ctx context.Context, ref string) (string, error) {
    portal, err := r.client.GetPortalByName(ctx, ref)
    if err != nil {
        return "", err
    }
    if portal == nil {
        return "", fmt.Errorf("portal not found")
    }
    return portal.ID, nil
}

// isUUID checks if string is already a UUID
func isUUID(s string) bool {
    // Simple check - actual implementation would use regex or uuid library
    return len(s) == 36 && s[8] == '-' && s[13] == '-' && s[18] == '-' && s[23] == '-'
}
```

### Tests
- Reference resolution with valid refs
- Missing reference handling
- UUID detection
- Reference application

### Commit Message
```
feat(planner): implement reference resolver

Add resolver to convert declarative references to Konnect IDs during
plan generation
```

---

## Step 7: Implement Dependency Resolution

### Status
Completed

### Dependencies
Step 5

### Changes
- Create file: `internal/declarative/planner/dependencies.go`
- Implement dependency graph and topological sort

### Implementation
```go
package planner

import (
    "fmt"
)

// DependencyResolver calculates execution order for plan changes
type DependencyResolver struct{}

// NewDependencyResolver creates a new resolver
func NewDependencyResolver() *DependencyResolver {
    return &DependencyResolver{}
}

// ResolveDependencies builds dependency graph and calculates execution order
func (d *DependencyResolver) ResolveDependencies(changes []PlannedChange) ([]string, error) {
    // Build dependency graph
    graph := make(map[string][]string)     // change_id -> list of dependencies
    inDegree := make(map[string]int)       // change_id -> number of incoming edges
    allChanges := make(map[string]bool)    // set of all change IDs
    
    // Initialize graph
    for _, change := range changes {
        changeID := change.ID
        allChanges[changeID] = true
        
        if _, exists := graph[changeID]; !exists {
            graph[changeID] = []string{}
        }
        if _, exists := inDegree[changeID]; !exists {
            inDegree[changeID] = 0
        }
        
        // Add explicit dependencies
        for _, dep := range change.DependsOn {
            graph[dep] = append(graph[dep], changeID)
            inDegree[changeID]++
        }
        
        // Add implicit dependencies based on references
        deps := d.findImplicitDependencies(change, changes)
        for _, dep := range deps {
            if !contains(change.DependsOn, dep) { // Avoid duplicates
                graph[dep] = append(graph[dep], changeID)
                inDegree[changeID]++
            }
        }
        
        // Parent dependencies
        if change.Parent != nil && change.Parent.ID == "<unknown>" {
            parentDep := d.findParentChange(change.Parent.Ref, change.ResourceType, changes)
            if parentDep != "" && !contains(change.DependsOn, parentDep) {
                graph[parentDep] = append(graph[parentDep], changeID)
                inDegree[changeID]++
            }
        }
    }
    
    // Topological sort using Kahn's algorithm
    queue := []string{}
    for changeID := range allChanges {
        if inDegree[changeID] == 0 {
            queue = append(queue, changeID)
        }
    }
    
    executionOrder := []string{}
    
    for len(queue) > 0 {
        current := queue[0]
        queue = queue[1:]
        executionOrder = append(executionOrder, current)
        
        for _, dependent := range graph[current] {
            inDegree[dependent]--
            if inDegree[dependent] == 0 {
                queue = append(queue, dependent)
            }
        }
    }
    
    // Check for cycles
    if len(executionOrder) != len(allChanges) {
        return nil, fmt.Errorf("circular dependency detected in plan")
    }
    
    return executionOrder, nil
}

// findImplicitDependencies finds dependencies based on references
func (d *DependencyResolver) findImplicitDependencies(change PlannedChange, allChanges []PlannedChange) []string {
    var dependencies []string
    
    // Check references field
    for _, refInfo := range change.References {
        if refInfo.ID == "<unknown>" {
            // Find the change that creates this resource
            for _, other := range allChanges {
                if other.ResourceRef == refInfo.Ref && other.Action == ActionCreate {
                    dependencies = append(dependencies, other.ID)
                    break
                }
            }
        }
    }
    
    return dependencies
}

// findParentChange finds the change that creates the parent resource
func (d *DependencyResolver) findParentChange(parentRef, childResourceType string, changes []PlannedChange) string {
    parentType := d.getParentType(childResourceType)
    
    for _, change := range changes {
        if change.ResourceRef == parentRef && 
           change.ResourceType == parentType && 
           change.Action == ActionCreate {
            return change.ID
        }
    }
    
    return ""
}

// getParentType determines parent resource type from child type
func (d *DependencyResolver) getParentType(childType string) string {
    switch childType {
    case "api_version", "api_publication", "api_implementation":
        return "api"
    case "portal_page":
        return "portal"
    default:
        return ""
    }
}

// contains checks if string is in slice
func contains(slice []string, item string) bool {
    for _, s := range slice {
        if s == item {
            return true
        }
    }
    return false
}
```

### Tests
- Simple dependency chains
- Complex multi-level dependencies
- Circular dependency detection
- Parent-child relationships
- Reference-based dependencies

### Commit Message
```
feat(planner): implement dependency resolution

Add dependency resolver with topological sort to calculate correct
execution order for plan changes
```

---

## Step 8: Create Planner Core Logic

### Status
Completed

### Dependencies
Steps 4, 5, 6, 7

### Changes
- Create file: `internal/declarative/planner/planner.go`
- Implement plan generation logic with new plan structure

### Implementation
```go
package planner

import (
    "context"
    "fmt"
    
    "github.com/kong/kongctl/internal/build"
    "github.com/kong/kongctl/internal/declarative/hash"
    "github.com/kong/kongctl/internal/declarative/labels"
    "github.com/kong/kongctl/internal/declarative/resources"
    "github.com/kong/kongctl/internal/declarative/state"
)

// Planner generates execution plans
type Planner struct {
    client       *state.Client
    resolver     *ReferenceResolver
    depResolver  *DependencyResolver
    changeCount  int
}

// NewPlanner creates a new planner
func NewPlanner(client *state.Client) *Planner {
    return &Planner{
        client:      client,
        resolver:    NewReferenceResolver(client),
        depResolver: NewDependencyResolver(),
        changeCount: 0,
    }
}

// GeneratePlan creates a plan from declarative configuration
func (p *Planner) GeneratePlan(ctx context.Context, rs *resources.ResourceSet) (*Plan, error) {
    plan := NewPlan("1.0", fmt.Sprintf("kongctl/%s", build.Version))
    
    // Generate changes for each resource type
    if err := p.planAuthStrategyChanges(ctx, rs.ApplicationAuthStrategies, plan); err != nil {
        return nil, fmt.Errorf("failed to plan auth strategy changes: %w", err)
    }
    
    if err := p.planPortalChanges(ctx, rs.Portals, plan); err != nil {
        return nil, fmt.Errorf("failed to plan portal changes: %w", err)
    }
    
    // Future: Add other resource types
    
    // Resolve references for all changes
    resolveResult, err := p.resolver.ResolveReferences(ctx, plan.Changes)
    if err != nil {
        return nil, fmt.Errorf("failed to resolve references: %w", err)
    }
    
    // Apply resolved references to changes
    for changeID, refs := range resolveResult.ChangeReferences {
        for i := range plan.Changes {
            if plan.Changes[i].ID == changeID {
                plan.Changes[i].References = make(map[string]ReferenceInfo)
                for field, ref := range refs {
                    plan.Changes[i].References[field] = ReferenceInfo{
                        Ref: ref.Ref,
                        ID:  ref.ID,
                    }
                }
                break
            }
        }
    }
    
    // Resolve dependencies and calculate execution order
    executionOrder, err := p.depResolver.ResolveDependencies(plan.Changes)
    if err != nil {
        return nil, fmt.Errorf("failed to resolve dependencies: %w", err)
    }
    plan.SetExecutionOrder(executionOrder)
    
    // Add warnings for unresolved references
    for _, change := range plan.Changes {
        for field, ref := range change.References {
            if ref.ID == "<unknown>" {
                plan.AddWarning(change.ID, fmt.Sprintf(
                    "Reference %s=%s will be resolved during execution",
                    field, ref.Ref))
            }
        }
    }
    
    return plan, nil
}

// nextChangeID generates semantic change IDs
func (p *Planner) nextChangeID(action ActionType, ref string) string {
    p.changeCount++
    actionChar := "?"
    switch action {
    case ActionCreate:
        actionChar = "c"
    case ActionUpdate:
        actionChar = "u"
    case ActionDelete:
        actionChar = "d"
    }
    return fmt.Sprintf("%d-%s-%s", p.changeCount, actionChar, ref)
}

// planPortalChanges generates changes for portal resources
func (p *Planner) planPortalChanges(ctx context.Context, desired []resources.PortalResource, plan *Plan) error {
    // Fetch current managed portals
    currentPortals, err := p.client.ListManagedPortals(ctx)
    if err != nil {
        return fmt.Errorf("failed to list current portals: %w", err)
    }
    
    // Index current portals by name
    currentByName := make(map[string]state.Portal)
    for _, portal := range currentPortals {
        currentByName[portal.Name] = portal
    }
    
    // Compare each desired portal
    for _, desiredPortal := range desired {
        // Calculate config hash for desired state
        configHash, err := hash.CalculatePortalHash(desiredPortal.CreatePortal)
        if err != nil {
            return fmt.Errorf("failed to calculate hash for portal %q: %w", desiredPortal.GetRef(), err)
        }
        
        current, exists := currentByName[desiredPortal.Name]
        
        if !exists {
            // CREATE action
            p.planPortalCreate(desiredPortal, configHash, plan)
        } else {
            // Check if update needed
            currentHash := current.NormalizedLabels[labels.ConfigHashKey]
            isProtected := current.NormalizedLabels[labels.ProtectedKey] == "true"
            shouldProtect := desiredPortal.GetLabels()[labels.ProtectedKey] == "true"
            
            // Handle protection changes separately
            if isProtected != shouldProtect {
                p.planProtectionChange(current, isProtected, shouldProtect, plan)
                // If unprotecting, we can then update
                if isProtected && !shouldProtect {
                    if currentHash != configHash {
                        p.planPortalUpdate(current, desiredPortal, configHash, plan)
                    }
                }
            } else if currentHash != configHash {
                // Regular update (no protection change)
                p.planPortalUpdate(current, desiredPortal, configHash, plan)
            }
        }
    }
    
    return nil
}

// planPortalCreate creates a CREATE change for a portal
func (p *Planner) planPortalCreate(portal resources.PortalResource, configHash string, plan *Plan) {
    fields := make(map[string]interface{})
    fields["name"] = portal.Name
    if portal.DisplayName != nil {
        fields["display_name"] = *portal.DisplayName
    }
    if portal.Description != nil {
        fields["description"] = *portal.Description
    }
    // Add other fields...
    
    change := PlannedChange{
        ID:           p.nextChangeID(ActionCreate, portal.GetRef()),
        ResourceType: "portal",
        ResourceRef:  portal.GetRef(),
        Action:       ActionCreate,
        Fields:       fields,
        ConfigHash:   configHash,
        DependsOn:    []string{},
    }
    
    // Check if protected
    if portal.GetLabels()[labels.ProtectedKey] == "true" {
        change.Protection = true
    }
    
    plan.AddChange(change)
}

// planPortalUpdate creates an UPDATE change for a portal
func (p *Planner) planPortalUpdate(current state.Portal, desired resources.PortalResource, configHash string, plan *Plan) {
    fields := make(map[string]interface{})
    dependencies := []string{}
    
    // Compare each field and store only changes
    if current.Description != getString(desired.Description) {
        fields["description"] = FieldChange{
            Old: current.Description,
            New: getString(desired.Description),
        }
    }
    
    if current.DisplayName != getString(desired.DisplayName) {
        fields["display_name"] = FieldChange{
            Old: current.DisplayName,
            New: getString(desired.DisplayName),
        }
    }
    
    // Handle auth strategy reference
    desiredAuthID := getString(desired.DefaultApplicationAuthStrategyID)
    if current.DefaultApplicationAuthStrategyID != desiredAuthID {
        fields["default_application_auth_strategy_id"] = FieldChange{
            Old: current.DefaultApplicationAuthStrategyID,
            New: desiredAuthID,
        }
    }
    
    // Add other field comparisons...
    
    // Only create change if there are actual field changes
    if len(fields) > 0 {
        change := PlannedChange{
            ID:           p.nextChangeID(ActionUpdate, desired.GetRef()),
            ResourceType: "portal",
            ResourceRef:  desired.GetRef(),
            ResourceID:   current.ID,
            Action:       ActionUpdate,
            Fields:       fields,
            ConfigHash:   configHash,
            DependsOn:    dependencies,
        }
        
        // Check if already protected
        if current.NormalizedLabels[labels.ProtectedKey] == "true" {
            change.Protection = true
        }
        
        plan.AddChange(change)
    }
}

// planProtectionChange creates a separate UPDATE for protection status
func (p *Planner) planProtectionChange(portal state.Portal, wasProtected, shouldProtect bool, plan *Plan) {
    change := PlannedChange{
        ID:           p.nextChangeID(ActionUpdate, portal.Name+"-protection"),
        ResourceType: "portal",
        ResourceRef:  portal.Name,
        ResourceID:   portal.ID,
        Action:       ActionUpdate,
        Fields:       map[string]interface{}{}, // No field changes allowed
        Protection: ProtectionChange{
            Old: wasProtected,
            New: shouldProtect,
        },
        ConfigHash: portal.NormalizedLabels[labels.ConfigHashKey],
        DependsOn:  []string{},
    }
    
    plan.AddChange(change)
}

// planAuthStrategyChanges generates changes for auth strategies
func (p *Planner) planAuthStrategyChanges(ctx context.Context, desired []resources.ApplicationAuthStrategyResource, plan *Plan) error {
    // Similar logic to portals but for auth strategies
    // TODO: Implement when auth strategy state client is available
    
    // For now, just create all as new
    for _, strategy := range desired {
        configHash, err := hash.CalculateAuthStrategyHash(strategy.CreateApplicationAuthStrategy)
        if err != nil {
            return fmt.Errorf("failed to calculate hash for auth strategy %q: %w", strategy.GetRef(), err)
        }
        
        fields := make(map[string]interface{})
        fields["name"] = strategy.Name
        if strategy.DisplayName != nil {
            fields["display_name"] = *strategy.DisplayName
        }
        fields["strategy_type"] = strategy.StrategyType
        fields["configs"] = strategy.Configs
        
        change := PlannedChange{
            ID:           p.nextChangeID(ActionCreate, strategy.GetRef()),
            ResourceType: "application_auth_strategy",
            ResourceRef:  strategy.GetRef(),
            Action:       ActionCreate,
            Fields:       fields,
            ConfigHash:   configHash,
            DependsOn:    []string{},
        }
        
        plan.AddChange(change)
    }
    
    return nil
}

// getString dereferences string pointer or returns empty
func getString(s *string) string {
    if s == nil {
        return ""
    }
    return *s
}
```

### Tests
- Plan generation with various scenarios
- CREATE vs UPDATE detection
- Protection change isolation
- Semantic ID generation
- Minimal field storage
- Reference resolution integration
- Dependency calculation

### Commit Message
```
feat(planner): implement core plan generation logic

Add planner that generates execution plans with semantic IDs, minimal
field storage, protection handling, and dependency resolution
```

---

## Step 9: Update Plan Command

### Status
Completed

### Dependencies
Step 8

### Changes
- Update: `internal/cmd/root/products/konnect/declarative/declarative.go`
- Integrate planner with plan command

### Implementation
```go
// Update runPlan function
func runPlan(cmd *cobra.Command, _ []string) error {
    ctx := cmd.Context()
    
    // Get configuration
    cfg := config.GetCurrent()
    kkClient, err := helper.GetKonnectSDK(cfg, logger)
    if err != nil {
        return fmt.Errorf("failed to initialize Konnect client: %w", err)
    }
    
    // Load declarative configuration
    filenames, _ := cmd.Flags().GetStringSlice("filename")
    recursive, _ := cmd.Flags().GetBool("recursive")
    
    loader := loader.New("")
    loader.SetSources(filenames, recursive)
    
    resourceSet, err := loader.Load()
    if err != nil {
        return fmt.Errorf("failed to load configuration: %w", err)
    }
    
    // Create planner
    portalAPI := &helpers.InternalPortalAPI{SDK: kkClient.GetInternalSDK()}
    stateClient := state.NewClient(portalAPI)
    planner := planner.NewPlanner(stateClient)
    
    // Generate plan
    plan, err := planner.GeneratePlan(ctx, resourceSet)
    if err != nil {
        return fmt.Errorf("failed to generate plan: %w", err)
    }
    
    // Handle output
    outputFile, _ := cmd.Flags().GetString("output-file")
    
    if outputFile != "" {
        // Save to file
        planJSON, err := json.MarshalIndent(plan, "", "  ")
        if err != nil {
            return fmt.Errorf("failed to marshal plan: %w", err)
        }
        
        if err := os.WriteFile(outputFile, planJSON, 0644); err != nil {
            return fmt.Errorf("failed to write plan file: %w", err)
        }
        
        fmt.Fprintf(cmd.OutOrStdout(), "Plan saved to: %s\n", outputFile)
    }
    
    // Display summary
    fmt.Fprintf(cmd.OutOrStdout(), "\nPlan Summary:\n")
    fmt.Fprintf(cmd.OutOrStdout(), "Total changes: %d\n", plan.Summary.TotalChanges)
    
    for action, count := range plan.Summary.ByAction {
        fmt.Fprintf(cmd.OutOrStdout(), "  %s: %d\n", action, count)
    }
    
    if plan.IsEmpty() {
        fmt.Fprintln(cmd.OutOrStdout(), "\nNo changes detected. Infrastructure is up to date.")
    } else {
        fmt.Fprintf(cmd.OutOrStdout(), "\nRun 'kongctl diff --plan %s' to review changes.\n", outputFile)
    }
    
    return nil
}
```

### Tests
- Plan generation integration test
- Output file writing
- Summary display

### Commit Message
```
feat(plan): integrate plan generation with plan command

Update plan command to generate execution plans using the planner
and save to specified output file
```

---

## Step 10: Implement Diff Command

### Status
Not Started

### Dependencies
Step 5

### Changes
- Update: `internal/cmd/root/verbs/diff/diff.go`
- Implement diff display logic

### Implementation
```go
func runDiff(cmd *cobra.Command, _ []string) error {
    ctx := cmd.Context()
    
    var plan *planner.Plan
    
    // Check if plan file provided
    planFile, _ := cmd.Flags().GetString("plan")
    
    if planFile != "" {
        // Load existing plan
        planData, err := os.ReadFile(planFile)
        if err != nil {
            return fmt.Errorf("failed to read plan file: %w", err)
        }
        
        plan = &planner.Plan{}
        if err := json.Unmarshal(planData, plan); err != nil {
            return fmt.Errorf("failed to parse plan file: %w", err)
        }
    } else {
        // Generate new plan
        // (Similar to plan command logic)
        return fmt.Errorf("inline plan generation not yet implemented - use --plan flag")
    }
    
    // Display diff
    outputFormat, _ := cmd.Flags().GetString("output")
    
    if outputFormat == "json" {
        // JSON output
        encoder := json.NewEncoder(cmd.OutOrStdout())
        encoder.SetIndent("", "  ")
        return encoder.Encode(plan)
    }
    
    // Human-readable output
    if plan.IsEmpty() {
        fmt.Fprintln(cmd.OutOrStdout(), "No changes detected.")
        return nil
    }
    
    fmt.Fprintf(cmd.OutOrStdout(), "Plan: %d to add, %d to change\n\n",
        plan.Summary.ByAction[planner.ActionCreate],
        plan.Summary.ByAction[planner.ActionUpdate])
    
    // Display each change in execution order
    for _, changeID := range plan.ExecutionOrder {
        // Find the change
        var change *planner.PlannedChange
        for i := range plan.Changes {
            if plan.Changes[i].ID == changeID {
                change = &plan.Changes[i]
                break
            }
        }
        if change == nil {
            continue
        }
        
        switch change.Action {
        case planner.ActionCreate:
            fmt.Fprintf(cmd.OutOrStdout(), "+ [%s] %s %q will be created\n",
                change.ID, change.ResourceType, change.ResourceRef)
            
            // Show key fields
            for field, value := range change.Fields {
                if str, ok := value.(string); ok && str != "" {
                    fmt.Fprintf(cmd.OutOrStdout(), "  %s: %q\n", field, str)
                }
            }
            
            // Show protection status
            if prot, ok := change.Protection.(bool); ok && prot {
                fmt.Fprintln(cmd.OutOrStdout(), "  protection: enabled")
            }
            
        case planner.ActionUpdate:
            fmt.Fprintf(cmd.OutOrStdout(), "~ [%s] %s %q will be updated\n",
                change.ID, change.ResourceType, change.ResourceRef)
            
            // Check if this is a protection change
            if pc, ok := change.Protection.(planner.ProtectionChange); ok {
                if pc.Old && !pc.New {
                    fmt.Fprintln(cmd.OutOrStdout(), "  protection: enabled → disabled")
                } else if !pc.Old && pc.New {
                    fmt.Fprintln(cmd.OutOrStdout(), "  protection: disabled → enabled")
                }
            }
            
            // Show field changes
            for field, value := range change.Fields {
                if fc, ok := value.(planner.FieldChange); ok {
                    fmt.Fprintf(cmd.OutOrStdout(), "  %s: %v → %v\n",
                        field, fc.Old, fc.New)
                }
            }
        }
        
        // Show dependencies
        if len(change.DependsOn) > 0 {
            fmt.Fprintf(cmd.OutOrStdout(), "  depends on: %v\n", change.DependsOn)
        }
        
        // Show references
        if len(change.References) > 0 {
            fmt.Fprintln(cmd.OutOrStdout(), "  references:")
            for field, ref := range change.References {
                if ref.ID == "<unknown>" {
                    fmt.Fprintf(cmd.OutOrStdout(), "    %s: %s (to be resolved)\n", field, ref.Ref)
                } else {
                    fmt.Fprintf(cmd.OutOrStdout(), "    %s: %s → %s\n", field, ref.Ref, ref.ID)
                }
            }
        }
        
        fmt.Fprintln(cmd.OutOrStdout())
    }
    
    return nil
}

// Add flags in NewDiffCmd
cmd.Flags().String("plan", "", "Path to plan file")
cmd.Flags().StringP("output", "o", "text", "Output format (text or json)")
```

### Tests
- Diff display formatting
- JSON output
- Empty plan handling

### Commit Message
```
feat(diff): implement diff command for plan visualization

Add diff command to display plan changes in human-readable or JSON format
```

---

## Step 11: Add Integration Tests

### Status
Not Started

### Dependencies
Steps 9, 10

### Changes
- Create directory: `test/integration/declarative/`
- Create file: `test/integration/declarative/plan_test.go`

### Implementation
```go
//go:build integration
// +build integration

package declarative_test

import (
    "context"
    "encoding/json"
    "os"
    "path/filepath"
    "testing"
    
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    
    "github.com/kong/kongctl/test/cmd"
)

func TestPlanGeneration(t *testing.T) {
    // Create test configuration
    configDir := t.TempDir()
    configFile := filepath.Join(configDir, "portal.yaml")
    
    config := `
portals:
  - ref: test-portal
    name: "Test Portal"
    description: "Integration test portal"
`
    require.NoError(t, os.WriteFile(configFile, []byte(config), 0644))
    
    // Generate plan
    planFile := filepath.Join(t.TempDir(), "plan.json")
    
    output, err := cmd.RunKongctl(
        "plan",
        "-f", configFile,
        "--output-file", planFile,
    )
    
    require.NoError(t, err)
    assert.Contains(t, output, "Plan saved to:")
    
    // Verify plan file
    planData, err := os.ReadFile(planFile)
    require.NoError(t, err)
    
    var plan map[string]interface{}
    require.NoError(t, json.Unmarshal(planData, &plan))
    
    // Check plan structure
    assert.Equal(t, "1.0", plan["metadata"].(map[string]interface{})["version"])
    assert.NotEmpty(t, plan["changes"])
    
    // Run diff
    output, err = cmd.RunKongctl(
        "diff",
        "--plan", planFile,
    )
    
    require.NoError(t, err)
    assert.Contains(t, output, "Test Portal")
}

func TestPlanWithReferences(t *testing.T) {
    // Test plan generation with cross-references
    // Similar structure but with auth strategy references
}

func TestEmptyPlan(t *testing.T) {
    // Test when no changes needed
}
```

### Tests
- End-to-end plan generation
- Plan file format validation
- Diff command integration
- Reference resolution

### Commit Message
```
test(declarative): add integration tests for plan generation

Add comprehensive integration tests for plan and diff commands with
various configuration scenarios
```