# Stage 2 Execution Plan: Detailed Steps

## Progress Summary

| Step | Description | Status | Dependencies |
|------|-------------|--------|--------------|
| 1 | Extend PortalAPI Interface | Not Started | None |
| 2 | Implement Label Utilities | Not Started | None |
| 3 | Create State Client Wrapper | Not Started | Step 1 |
| 4 | Implement Config Hash | Not Started | Step 2 |
| 5 | Define Plan Types | Not Started | None |
| 6 | Implement Reference Resolver | Not Started | Step 3 |
| 7 | Create Planner Core Logic | Not Started | Steps 4, 5, 6 |
| 8 | Update Plan Command | Not Started | Step 7 |
| 9 | Implement Diff Command | Not Started | Step 5 |
| 10 | Add Integration Tests | Not Started | Steps 8, 9 |

---

## Step 1: Extend PortalAPI Interface

### Status
Not Started

### Dependencies
None

### Changes
- **File**: `internal/konnect/helpers/portals.go`
- Add CreatePortal and UpdatePortal methods to PortalAPI interface
- Implement methods in InternalPortalAPI

### Implementation
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

### Tests
- Mock SDK responses for create/update
- Error handling scenarios

### Commit Message
```
feat(konnect): extend PortalAPI with create and update operations

Add CreatePortal and UpdatePortal methods to support plan execution
```

---

## Step 2: Implement Label Utilities

### Status
Not Started

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
Not Started

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
Not Started

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
    "sort"
    
    "github.com/kong/kongctl/internal/declarative/labels"
    kkInternalComps "github.com/Kong/sdk-konnect-go-internal/models/components"
)

// CalculatePortalHash generates deterministic hash for portal config
func CalculatePortalHash(portal kkInternalComps.CreatePortal) (string, error) {
    // Create hashable structure with sorted fields
    hashable := map[string]interface{}{
        "name":                               portal.Name,
        "display_name":                       portal.DisplayName,
        "description":                        portal.Description,
        "authentication_enabled":             portal.AuthenticationEnabled,
        "rbac_enabled":                      portal.RbacEnabled,
        "default_api_visibility":            portal.DefaultAPIVisibility,
        "default_page_visibility":           portal.DefaultPageVisibility,
        "default_application_auth_strategy_id": portal.DefaultApplicationAuthStrategyID,
        "auto_approve_developers":           portal.AutoApproveDevelopers,
        "auto_approve_applications":         portal.AutoApproveApplications,
    }
    
    // Add user labels only (exclude KONGCTL labels)
    if portal.Labels != nil {
        userLabels := make(map[string]string)
        normalized := labels.NormalizeLabels(portal.Labels)
        
        for k, v := range normalized {
            if !labels.IsKongctlLabel(k) {
                userLabels[k] = v
            }
        }
        
        if len(userLabels) > 0 {
            hashable["user_labels"] = sortedMap(userLabels)
        }
    }
    
    return calculateHash(hashable)
}

// sortedMap returns map with keys in sorted order for deterministic JSON
func sortedMap(m map[string]string) map[string]string {
    keys := make([]string, 0, len(m))
    for k := range m {
        keys = append(keys, k)
    }
    sort.Strings(keys)
    
    sorted := make(map[string]string)
    for _, k := range keys {
        sorted[k] = m[k]
    }
    return sorted
}

// calculateHash generates SHA256 hash from data structure
func calculateHash(data interface{}) (string, error) {
    // Marshal to JSON with sorted keys
    jsonBytes, err := json.Marshal(data)
    if err != nil {
        return "", fmt.Errorf("failed to marshal for hash: %w", err)
    }
    
    // Generate SHA256 hash
    hash := sha256.Sum256(jsonBytes)
    
    // Return base64 encoded string
    return base64.StdEncoding.EncodeToString(hash[:]), nil
}

// ComparePortalHash checks if portal config matches expected hash
func ComparePortalHash(portal kkInternalComps.PortalResponse, expectedHash string) (bool, error) {
    // Convert response to create structure for hashing
    createPortal := kkInternalComps.CreatePortal{
        Name:                            portal.Name,
        DisplayName:                     &portal.DisplayName,
        Description:                     portal.Description,
        AuthenticationEnabled:           &portal.AuthenticationEnabled,
        RbacEnabled:                    &portal.RbacEnabled,
        DefaultAPIVisibility:           (*kkInternalComps.DefaultAPIVisibility)(&portal.DefaultAPIVisibility),
        DefaultPageVisibility:          (*kkInternalComps.DefaultPageVisibility)(&portal.DefaultPageVisibility),
        DefaultApplicationAuthStrategyID: portal.DefaultApplicationAuthStrategyID,
        AutoApproveDevelopers:          &portal.AutoApproveDevelopers,
        AutoApproveApplications:        &portal.AutoApproveApplications,
        Labels:                         labels.DenormalizeLabels(portal.Labels),
    }
    
    actualHash, err := CalculatePortalHash(createPortal)
    if err != nil {
        return false, err
    }
    
    return actualHash == expectedHash, nil
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
feat(hash): implement configuration hash calculation

Add SHA256-based hashing for detecting configuration drift with
deterministic output and KONGCTL label exclusion
```

---

## Step 5: Define Plan Types

### Status
Not Started

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
    Metadata          PlanMetadata                `json:"metadata"`
    ReferenceMappings map[string]map[string]string `json:"reference_mappings"`
    Changes           []PlannedChange             `json:"changes"`
    Summary           PlanSummary                 `json:"summary"`
}

// PlanMetadata contains plan generation information
type PlanMetadata struct {
    Version     string    `json:"version"`
    GeneratedAt time.Time `json:"generated_at"`
    Generator   string    `json:"generator"`
}

// PlannedChange represents a single resource change
type PlannedChange struct {
    ID            string        `json:"id"`
    ResourceType  string        `json:"resource_type"`
    ResourceRef   string        `json:"resource_ref"`
    ResourceName  string        `json:"resource_name"`
    Action        ActionType    `json:"action"`
    CurrentState  interface{}   `json:"current_state,omitempty"`
    DesiredState  interface{}   `json:"desired_state"`
    FieldChanges  []FieldChange `json:"field_changes,omitempty"`
    ConfigHash    string        `json:"config_hash"`
}

// ActionType represents the type of change
type ActionType string

const (
    ActionCreate ActionType = "CREATE"
    ActionUpdate ActionType = "UPDATE"
    ActionDelete ActionType = "DELETE" // Future
)

// FieldChange represents a single field modification
type FieldChange struct {
    Field    string      `json:"field"`
    OldValue interface{} `json:"old_value"`
    NewValue interface{} `json:"new_value"`
}

// PlanSummary provides overview statistics
type PlanSummary struct {
    TotalChanges int                       `json:"total_changes"`
    ByAction     map[ActionType]int        `json:"by_action"`
    ByResource   map[string]int            `json:"by_resource"`
}

// NewPlan creates a new plan with metadata
func NewPlan(version, generator string) *Plan {
    return &Plan{
        Metadata: PlanMetadata{
            Version:     version,
            GeneratedAt: time.Now().UTC(),
            Generator:   generator,
        },
        ReferenceMappings: make(map[string]map[string]string),
        Changes:          []PlannedChange{},
        Summary: PlanSummary{
            ByAction:   make(map[ActionType]int),
            ByResource: make(map[string]int),
        },
    }
}

// AddChange adds a change to the plan
func (p *Plan) AddChange(change PlannedChange) {
    p.Changes = append(p.Changes, change)
    p.updateSummary()
}

// updateSummary recalculates plan statistics
func (p *Plan) updateSummary() {
    p.Summary.TotalChanges = len(p.Changes)
    
    // Reset counts
    p.Summary.ByAction = make(map[ActionType]int)
    p.Summary.ByResource = make(map[string]int)
    
    // Count by action and resource type
    for _, change := range p.Changes {
        p.Summary.ByAction[change.Action]++
        p.Summary.ByResource[change.ResourceType]++
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
Not Started

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

// ResolveResult contains resolved reference mappings
type ResolveResult struct {
    // Map of resource_type -> ref -> id
    Mappings map[string]map[string]string
    // Errors encountered during resolution
    Errors []error
}

// ResolveReferences resolves all references in a resource set
func (r *ReferenceResolver) ResolveReferences(ctx context.Context, rs *resources.ResourceSet) (*ResolveResult, error) {
    result := &ResolveResult{
        Mappings: make(map[string]map[string]string),
        Errors:   []error{},
    }
    
    // For now, only auth strategies can be referenced
    // Future: Add other resource types as needed
    
    // Resolve auth strategy references in portals
    if err := r.resolvePortalReferences(ctx, rs.Portals, result); err != nil {
        return nil, err
    }
    
    return result, nil
}

// resolvePortalReferences resolves references within portal resources
func (r *ReferenceResolver) resolvePortalReferences(ctx context.Context, portals []resources.PortalResource, result *ResolveResult) error {
    for _, portal := range portals {
        mappings := portal.GetReferenceFieldMappings()
        
        for fieldName, resourceType := range mappings {
            // For portals, only auth strategy field needs resolution
            if fieldName == "default_application_auth_strategy_id" && portal.DefaultApplicationAuthStrategyID != nil {
                ref := *portal.DefaultApplicationAuthStrategyID
                
                // Skip if already an ID (UUID format)
                if isUUID(ref) {
                    continue
                }
                
                // Look up the auth strategy
                // Note: This is placeholder - actual implementation would
                // query auth strategies once that API is available
                id, err := r.resolveAuthStrategyRef(ctx, ref)
                if err != nil {
                    result.Errors = append(result.Errors, fmt.Errorf(
                        "portal %q: failed to resolve %s reference %q: %w",
                        portal.GetRef(), resourceType, ref, err))
                    continue
                }
                
                // Store mapping
                if result.Mappings[resourceType] == nil {
                    result.Mappings[resourceType] = make(map[string]string)
                }
                result.Mappings[resourceType][ref] = id
            }
        }
    }
    
    return nil
}

// resolveAuthStrategyRef resolves auth strategy ref to ID
func (r *ReferenceResolver) resolveAuthStrategyRef(ctx context.Context, ref string) (string, error) {
    // TODO: Implement when auth strategy API is available
    // For now, return error
    return "", fmt.Errorf("auth strategy resolution not yet implemented")
}

// isUUID checks if string is already a UUID
func isUUID(s string) bool {
    // Simple check - actual implementation would use regex or uuid library
    return len(s) == 36 && s[8] == '-' && s[13] == '-' && s[18] == '-' && s[23] == '-'
}

// ApplyResolvedReferences updates resources with resolved IDs
func ApplyResolvedReferences(rs *resources.ResourceSet, mappings map[string]map[string]string) {
    // Update portal auth strategy references
    for i := range rs.Portals {
        portal := &rs.Portals[i]
        
        if portal.DefaultApplicationAuthStrategyID != nil {
            ref := *portal.DefaultApplicationAuthStrategyID
            if !isUUID(ref) {
                if id, ok := mappings["application_auth_strategy"][ref]; ok {
                    portal.DefaultApplicationAuthStrategyID = &id
                }
            }
        }
    }
    
    // Future: Update other resource references
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

## Step 7: Create Planner Core Logic

### Status
Not Started

### Dependencies
Steps 4, 5, 6

### Changes
- Create file: `internal/declarative/planner/planner.go`
- Implement plan generation logic

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
    kkInternalComps "github.com/Kong/sdk-konnect-go-internal/models/components"
)

// Planner generates execution plans
type Planner struct {
    client   *state.Client
    resolver *ReferenceResolver
}

// NewPlanner creates a new planner
func NewPlanner(client *state.Client) *Planner {
    return &Planner{
        client:   client,
        resolver: NewReferenceResolver(client),
    }
}

// GeneratePlan creates a plan from declarative configuration
func (p *Planner) GeneratePlan(ctx context.Context, rs *resources.ResourceSet) (*Plan, error) {
    plan := NewPlan("1.0", fmt.Sprintf("kongctl/%s", build.Version))
    
    // Resolve references first
    resolveResult, err := p.resolver.ResolveReferences(ctx, rs)
    if err != nil {
        return nil, fmt.Errorf("failed to resolve references: %w", err)
    }
    
    // Store reference mappings in plan
    plan.ReferenceMappings = resolveResult.Mappings
    
    // Apply resolved references to resources
    ApplyResolvedReferences(rs, resolveResult.Mappings)
    
    // Generate changes for each resource type
    if err := p.planPortalChanges(ctx, rs.Portals, plan); err != nil {
        return nil, fmt.Errorf("failed to plan portal changes: %w", err)
    }
    
    // Future: Add other resource types
    
    return plan, nil
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
        changeID := fmt.Sprintf("change-%s-%s", "portal", desiredPortal.GetRef())
        
        // Calculate config hash for desired state
        configHash, err := hash.CalculatePortalHash(desiredPortal.CreatePortal)
        if err != nil {
            return fmt.Errorf("failed to calculate hash for portal %q: %w", desiredPortal.GetRef(), err)
        }
        
        current, exists := currentByName[desiredPortal.Name]
        
        if !exists {
            // CREATE action
            change := PlannedChange{
                ID:           changeID,
                ResourceType: "portal",
                ResourceRef:  desiredPortal.GetRef(),
                ResourceName: desiredPortal.Name,
                Action:       ActionCreate,
                DesiredState: convertPortalToCreate(desiredPortal),
                ConfigHash:   configHash,
            }
            plan.AddChange(change)
        } else {
            // Check if update needed by comparing hash
            currentHash := current.NormalizedLabels[labels.ConfigHashKey]
            if currentHash != configHash {
                // UPDATE action
                fieldChanges := calculatePortalFieldChanges(current, desiredPortal)
                
                change := PlannedChange{
                    ID:           changeID,
                    ResourceType: "portal",
                    ResourceRef:  desiredPortal.GetRef(),
                    ResourceName: desiredPortal.Name,
                    Action:       ActionUpdate,
                    CurrentState: convertPortalToResponse(current),
                    DesiredState: convertPortalToUpdate(desiredPortal, current.ID),
                    FieldChanges: fieldChanges,
                    ConfigHash:   configHash,
                }
                plan.AddChange(change)
            }
        }
    }
    
    return nil
}

// convertPortalToCreate converts resource to SDK create type
func convertPortalToCreate(portal resources.PortalResource) kkInternalComps.CreatePortal {
    return portal.CreatePortal
}

// convertPortalToUpdate converts resource to SDK update type
func convertPortalToUpdate(portal resources.PortalResource, id string) map[string]interface{} {
    // Return update structure
    // Note: Actual implementation would use UpdatePortal type when available
    return map[string]interface{}{
        "id":                                id,
        "name":                              portal.Name,
        "display_name":                      portal.DisplayName,
        "description":                       portal.Description,
        "authentication_enabled":            portal.AuthenticationEnabled,
        "rbac_enabled":                      portal.RbacEnabled,
        "default_api_visibility":            portal.DefaultAPIVisibility,
        "default_page_visibility":           portal.DefaultPageVisibility,
        "default_application_auth_strategy_id": portal.DefaultApplicationAuthStrategyID,
        "auto_approve_developers":           portal.AutoApproveDevelopers,
        "auto_approve_applications":         portal.AutoApproveApplications,
    }
}

// convertPortalToResponse extracts response data
func convertPortalToResponse(portal state.Portal) map[string]interface{} {
    return map[string]interface{}{
        "id":                                portal.ID,
        "name":                              portal.Name,
        "display_name":                      portal.DisplayName,
        "description":                       portal.Description,
        "authentication_enabled":            portal.AuthenticationEnabled,
        "rbac_enabled":                      portal.RbacEnabled,
        "default_api_visibility":            portal.DefaultAPIVisibility,
        "default_page_visibility":           portal.DefaultPageVisibility,
        "default_application_auth_strategy_id": portal.DefaultApplicationAuthStrategyID,
        "auto_approve_developers":           portal.AutoApproveDevelopers,
        "auto_approve_applications":         portal.AutoApproveApplications,
        "labels":                           portal.NormalizedLabels,
    }
}

// calculatePortalFieldChanges determines which fields changed
func calculatePortalFieldChanges(current state.Portal, desired resources.PortalResource) []FieldChange {
    var changes []FieldChange
    
    // Compare each field
    if current.Name != desired.Name {
        changes = append(changes, FieldChange{
            Field:    "name",
            OldValue: current.Name,
            NewValue: desired.Name,
        })
    }
    
    if current.DisplayName != getString(desired.DisplayName) {
        changes = append(changes, FieldChange{
            Field:    "display_name",
            OldValue: current.DisplayName,
            NewValue: getString(desired.DisplayName),
        })
    }
    
    // Add other field comparisons...
    
    return changes
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
- Hash comparison logic
- Field change calculation

### Commit Message
```
feat(planner): implement core plan generation logic

Add planner that compares current and desired state to generate
execution plans with CREATE/UPDATE actions
```

---

## Step 8: Update Plan Command

### Status
Not Started

### Dependencies
Step 7

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

## Step 9: Implement Diff Command

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
    
    // Display each change
    for _, change := range plan.Changes {
        switch change.Action {
        case planner.ActionCreate:
            fmt.Fprintf(cmd.OutOrStdout(), "+ %s %q will be created\n",
                change.ResourceType, change.ResourceName)
            
            // Show key fields
            if desired, ok := change.DesiredState.(map[string]interface{}); ok {
                if desc, ok := desired["description"].(string); ok && desc != "" {
                    fmt.Fprintf(cmd.OutOrStdout(), "  description: %q\n", desc)
                }
            }
            
        case planner.ActionUpdate:
            fmt.Fprintf(cmd.OutOrStdout(), "~ %s %q will be updated\n",
                change.ResourceType, change.ResourceName)
            
            // Show field changes
            for _, fc := range change.FieldChanges {
                fmt.Fprintf(cmd.OutOrStdout(), "  %s: %v → %v\n",
                    fc.Field, fc.OldValue, fc.NewValue)
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

## Step 10: Add Integration Tests

### Status
Not Started

### Dependencies
Steps 8, 9

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