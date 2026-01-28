# DECLARATIVE RESOURCE IMPLEMENTATION GUIDE

## PURPOSE
Technical guide for coding agents to implement new resources in kongctl's declarative configuration engine. Details exact code locations, patterns, and requirements.

## RESOURCE TYPES

### PARENT RESOURCES
- Support full lifecycle: CREATE, UPDATE, DELETE
- Have kongctl metadata (namespace, protection)
- Can have nested child resources
- Managed via labels: KONGCTL-NAMESPACE, KONGCTL-PROTECTED
- Examples: Portal, API, ControlPlane, ApplicationAuthStrategy

### CHILD RESOURCES
- Scoped to parent resource
- NO kongctl metadata support
- Typically do not expose `KONGCTL_*` labels in Konnect responses. Deletion/managed checks must rely on the
  parent reference or namespace propagated through the plan instead of label lookups.
- Identified by parent + moniker (slug, name, version, etc.)
- Examples: PortalPage, APIDocument, APIVersion, PortalCustomization

### SINGLETON CHILD RESOURCES
- Special case: always exist for parent, only UPDATE supported
- No CREATE/DELETE operations
- Example: PortalCustomization

### PSEUDO RESOURCES (TOOL-LOCAL CONFIG)
- Some declarative keys (prefixed with `_`) represent **kongctl-owned configuration**, not Konnect resources.
- Example: `control_planes[]. _deck` (deck integration). These are **not** part of the Konnect API surface.
- Implementation pattern:
  - Add a field on the parent resource struct (e.g., `ControlPlaneResource`) with `yaml:"_deck"`.
  - Validate the pseudo-resource in the parent resource `Validate()` method.
  - Emit a `PlannedChange` with `ResourceType` set to a pseudo-type (e.g., `_deck`)
    and `ActionExternalTool`.
  - Update plan summaries to include `by_external_tools`.
  - Add dependencies to ensure external tool steps run in the correct order.

---

## IMPLEMENTATION CHECKLIST

### LOGGING & DIAGNOSTICS
- Always add verbose `slog` debug statements when introducing a new planner or executor path. Helpful patterns:
  - Planner: log when you fetch existing resources, how many desired items you saw, and each change you enqueue.
  - Executor/adapters: log before and after every SDK call (create/update/delete) and log input identifiers.
- Logging is especially important for child resources because they often lack labels and rely on parent metadata.
- The e2e harness captures `kongctl.log` per command when debug logging is enabled (i.e., `--log-level debug` or
  `--log-level trace`), so these logs should be readable in CI artifacts.

### PARENT RESOURCE

#### 1. RESOURCE DEFINITION
**Location**: `internal/declarative/resources/`
**File**: `<resource_name>.go`

```go
package resources

import (
    kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

// FooResource represents a Foo in declarative configuration
type FooResource struct {
    kkComps.CreateFooRequest `yaml:",inline" json:",inline"`  // Embed SDK type
    Ref     string       `yaml:"ref" json:"ref"`
    Kongctl *KongctlMeta `yaml:"kongctl,omitempty" json:"kongctl,omitempty"`

    // Child resources (optional, can also be root-level)
    Children []FooChildResource `yaml:"children,omitempty" json:"children,omitempty"`

    // Resolved Konnect ID (not serialized)
    konnectID string `yaml:"-" json:"-"`
}

// REQUIRED: Implement Resource interface
func (f FooResource) GetType() ResourceType {
    return ResourceTypeFoo  // Add to types.go
}

func (f FooResource) GetRef() string {
    return f.Ref
}

// GetMoniker returns identifier for matching (name, slug, etc.)
func (f FooResource) GetMoniker() string {
    return f.Name  // or f.Slug, depending on API
}

func (f FooResource) GetDependencies() []ResourceRef {
    deps := []ResourceRef{}
    // Add any cross-resource dependencies
    if f.ParentRef != "" {
        deps = append(deps, ResourceRef{Kind: "parent_type", Ref: f.ParentRef})
    }
    return deps
}

func (f FooResource) Validate() error {
    if err := ValidateRef(f.Ref); err != nil {
        return fmt.Errorf("invalid foo ref: %w", err)
    }
    // Validate required fields
    if f.Name == "" {
        return fmt.Errorf("name is required")
    }
    // Validate nested children
    for i, child := range f.Children {
        if err := child.Validate(); err != nil {
            return fmt.Errorf("child[%d] validation failed: %w", i, err)
        }
    }
    return nil
}

func (f *FooResource) SetDefaults() {
    // Set default values
    if f.Name == "" {
        f.Name = f.Ref
    }
    // Set defaults for children
    for i := range f.Children {
        f.Children[i].SetDefaults()
    }
}

func (f FooResource) GetKonnectID() string {
    return f.konnectID
}

// GetKonnectMonikerFilter returns filter for API lookup
func (f FooResource) GetKonnectMonikerFilter() string {
    return fmt.Sprintf("name[eq]=%s", f.Name)  // Adjust based on API
}

// TryMatchKonnectResource matches against Konnect API response
func (f *FooResource) TryMatchKonnectResource(konnectResource any) bool {
    v := reflect.ValueOf(konnectResource)
    if v.Kind() == reflect.Ptr {
        v = v.Elem()
    }
    if v.Kind() != reflect.Struct {
        return false
    }

    nameField := v.FieldByName("Name")
    idField := v.FieldByName("ID")

    if nameField.IsValid() && idField.IsValid() &&
        nameField.Kind() == reflect.String && idField.Kind() == reflect.String {
        if nameField.String() == f.Name {
            f.konnectID = idField.String()
            return true
        }
    }
    return false
}

// REQUIRED FOR LABEL SUPPORT: Implement ResourceWithLabels
func (f FooResource) GetLabels() map[string]string {
    // Convert SDK labels to map[string]string
    labels := make(map[string]string)
    for k, v := range f.Labels {
        if v != nil {
            labels[k] = *v  // If SDK uses *string
        }
        // OR: labels[k] = v  // If SDK uses string
    }
    return labels
}

func (f *FooResource) SetLabels(labels map[string]string) {
    // Convert map to SDK format
    f.Labels = make(map[string]*string)  // Or map[string]string
    for k, v := range labels {
        val := v
        f.Labels[k] = &val  // If SDK uses *string
        // OR: f.Labels[k] = v  // If SDK uses string
    }
}
```

**ADD TO**: `internal/declarative/resources/types.go`
```go
const (
    ResourceTypeFoo ResourceType = "foo"
)
```

**ADD TO**: `internal/declarative/resources/types.go` ResourceSet struct:
```go
type ResourceSet struct {
    Foos []FooResource `yaml:"foos,omitempty" json:"foos,omitempty"`
    // ... existing resources
}
```

#### 2. STATE CLIENT METHODS
**Location**: `internal/declarative/state/client.go`

Add API field to Client struct:
```go
type Client struct {
    fooAPI helpers.FooAPI
    // ... existing APIs
}
```

Add normalized type:
```go
type Foo struct {
    kkComps.FooResponseSchema  // Or ListFoosResponseFoo
    NormalizedLabels map[string]string
}
```

Implement CRUD methods:
```go
func (c *Client) ListManagedFoos(ctx context.Context, namespaces []string) ([]Foo, error) {
    lister := func(ctx context.Context, pageSize, pageNumber int64) ([]Foo, *PageMeta, error) {
        req := kkOps.ListFoosRequest{
            PageSize:   &pageSize,
            PageNumber: &pageNumber,
        }

        resp, err := c.fooAPI.ListFoos(ctx, req)
        if err != nil {
            return nil, nil, WrapAPIError(err, "list foos", nil)
        }

        var filteredFoos []Foo
        for _, f := range resp.ListFoosResponse.Data {
            // Normalize labels
            normalized := normalizeLabels(f.Labels)  // Handle SDK label format

            // Filter by managed status and namespace
            if labels.IsManagedResource(normalized) {
                if shouldIncludeNamespace(normalized[labels.NamespaceKey], namespaces) {
                    foo := Foo{
                        FooResponseSchema: f,
                        NormalizedLabels:  normalized,
                    }
                    filteredFoos = append(filteredFoos, foo)
                }
            }
        }

        meta := &PageMeta{Total: resp.ListFoosResponse.Meta.Page.Total}
        return filteredFoos, meta, nil
    }

    return PaginateAll(ctx, lister)
}

func (c *Client) CreateFoo(ctx context.Context, foo kkComps.CreateFoo, namespace string) (*kkComps.FooResponse, error) {
    resp, err := c.fooAPI.CreateFoo(ctx, foo)
    if err != nil {
        return nil, WrapAPIError(err, "create foo", &ErrorWrapperOptions{
            ResourceType: "foo",
            ResourceName: foo.Name,
            Namespace:    namespace,
            UseEnhanced:  true,
        })
    }

    if err := ValidateResponse(resp.FooResponse, "create foo"); err != nil {
        return nil, err
    }

    return resp.FooResponse, nil
}

func (c *Client) UpdateFoo(ctx context.Context, id string, foo kkComps.UpdateFoo, namespace string) (*kkComps.FooResponse, error) {
    resp, err := c.fooAPI.UpdateFoo(ctx, id, foo)
    if err != nil {
        return nil, WrapAPIError(err, "update foo", &ErrorWrapperOptions{
            ResourceType: "foo",
            ResourceName: *foo.Name,  // Adjust based on SDK
            Namespace:    namespace,
            UseEnhanced:  true,
        })
    }

    return resp.FooResponse, nil
}

func (c *Client) DeleteFoo(ctx context.Context, id string) error {
    err := c.fooAPI.DeleteFoo(ctx, id)
    if err != nil {
        return WrapAPIError(err, "delete foo", nil)
    }
    return nil
}
```

#### 3. PLANNER IMPLEMENTATION
**Location**: `internal/declarative/planner/foo_planner.go`

```go
package planner

import (
    "context"
    "github.com/kong/kongctl/internal/declarative/resources"
    "github.com/kong/kongctl/internal/declarative/state"
)

type fooPlannerImpl struct {
    planner   *Planner
    resources *resources.ResourceSet
}

func newFooPlanner(planner *Planner, resourceSet *resources.ResourceSet) *fooPlannerImpl {
    return &fooPlannerImpl{
        planner:   planner,
        resources: resourceSet,
    }
}

func (f *fooPlannerImpl) GetDesiredFoos(namespace string) []resources.FooResource {
    var result []resources.FooResource
    for _, foo := range f.resources.Foos {
        if foo.Kongctl == nil || foo.Kongctl.Namespace == nil {
            continue
        }
        if *foo.Kongctl.Namespace == namespace {
            result = append(result, foo)
        }
    }
    return result
}

func (f *fooPlannerImpl) PlanChanges(ctx context.Context, plannerCtx *Config, plan *Plan) error {
    namespace := plannerCtx.Namespace

    // Plan parent foos
    if err := f.planner.planFooChanges(ctx, plannerCtx, f.GetDesiredFoos(namespace), plan); err != nil {
        return err
    }

    // Plan child resources if any
    // if err := f.planner.planFooChildrenChanges(ctx, plannerCtx, ...); err != nil {
    //     return err
    // }

    return nil
}

// In planner.go, add method:
func (p *Planner) planFooChanges(
    ctx context.Context, plannerCtx *Config, desired []resources.FooResource, plan *Plan,
) error {
    namespace := plannerCtx.Namespace

    // 1. Fetch current foos
    currentFoos, err := p.client.ListManagedFoos(ctx, []string{namespace})
    if err != nil {
        return err
    }

    // 2. Index by name (or other identifier)
    currentByName := make(map[string]state.Foo)
    for _, foo := range currentFoos {
        currentByName[foo.Name] = foo
    }

    var protectionErrors []error
    desiredNames := make(map[string]bool)

    // 3. Process desired foos
    for _, desiredFoo := range desired {
        desiredNames[desiredFoo.Name] = true

        current, exists := currentByName[desiredFoo.Name]

        if !exists {
            // CREATE
            fooChangeID := p.planFooCreate(desiredFoo, plan)
            // Plan children if nested
            // p.planFooChildrenCreate(namespace, desiredFoo, fooChangeID, plan)
        } else {
            // UPDATE or protection change
            isProtected := labels.IsProtectedResource(current.NormalizedLabels)
            shouldProtect := (desiredFoo.Kongctl != nil &&
                             desiredFoo.Kongctl.Protected != nil &&
                             *desiredFoo.Kongctl.Protected)

            if isProtected != shouldProtect {
                // Protection change
                needsUpdate, updateFields := p.shouldUpdateFoo(current, desiredFoo)
                protectionChange := &ProtectionChange{Old: isProtected, New: shouldProtect}

                if err := p.validateProtectionWithChange("foo", desiredFoo.Name,
                                                          protectionChange, ActionUpdate); err != nil {
                    protectionErrors = append(protectionErrors, err)
                } else {
                    p.planFooProtectionChangeWithFields(current, desiredFoo,
                                                        isProtected, shouldProtect, updateFields, plan)
                }
            } else {
                // Regular update
                needsUpdate, updateFields := p.shouldUpdateFoo(current, desiredFoo)
                if needsUpdate {
                    if err := p.validateProtection("foo", desiredFoo.Name, isProtected, ActionUpdate); err != nil {
                        protectionErrors = append(protectionErrors, err)
                    } else {
                        p.planFooUpdateWithFields(current, desiredFoo, updateFields, plan)
                    }
                }
            }

            // Plan child resource changes
            // p.planFooChildResourceChanges(ctx, plannerCtx, current, desiredFoo, plan)
        }
    }

    // 4. SYNC MODE: Delete unmanaged
    if plan.Metadata.Mode == PlanModeSync {
        for name, current := range currentByName {
            if !desiredNames[name] {
                isProtected := labels.IsProtectedResource(current.NormalizedLabels)
                if err := p.validateProtection("foo", name, isProtected, ActionDelete); err != nil {
                    protectionErrors = append(protectionErrors, err)
                } else {
                    p.planFooDelete(current, plan)
                }
            }
        }
    }

    // 5. Fail fast if protected resources conflict
    if len(protectionErrors) > 0 {
        return fmt.Errorf("cannot generate plan due to protected resources: %v", protectionErrors)
    }

    return nil
}

func (p *Planner) shouldUpdateFoo(current state.Foo, desired resources.FooResource) (bool, map[string]any) {
    updates := make(map[string]any)

    // Compare fields that can be updated
    if desired.Description != nil {
        currentDesc := getString(current.Description)
        if currentDesc != *desired.Description {
            updates["description"] = *desired.Description
        }
    }

    // Compare labels (only user labels)
    // NOTE: CompareUserLabels returns TRUE when labels DIFFER (not when equal)
    if desired.Labels != nil {
        if labels.CompareUserLabels(current.NormalizedLabels, desired.GetLabels()) {
            updates["labels"] = desired.GetLabels()
        }
    }

    // Add other field comparisons

    return len(updates) > 0, updates
}

func (p *Planner) planFooCreate(foo resources.FooResource, plan *Plan) string {
    protection := extractProtection(foo.Kongctl)
    namespace := extractNamespace(foo.Kongctl)

    config := CreateConfig{
        ResourceType:   "foo",
        ResourceName:   foo.Name,
        ResourceRef:    foo.GetRef(),
        RequiredFields: []string{"name"},
        FieldExtractor: func(_ any) map[string]any {
            return extractFooFields(foo)
        },
        Namespace: namespace,
        DependsOn: []string{},
    }

    change, err := p.genericPlanner.PlanCreate(context.Background(), config)
    if err != nil {
        // Handle error appropriately - this is example code
        // In real implementation, return or log the error
        return ""
    }
    change.Protection = protection

    plan.AddChange(change)
    return change.ID
}

func extractFooFields(resource any) map[string]any {
    fields := make(map[string]any)
    foo := resource.(resources.FooResource)

    fields["name"] = foo.Name
    if foo.Description != nil {
        fields["description"] = *foo.Description
    }

    // Copy user labels (namespace/protection added during execution)
    if len(foo.Labels) > 0 {
        fields["labels"] = foo.GetLabels()
    }

    return fields
}

func (p *Planner) planFooUpdateWithFields(
    current state.Foo, desired resources.FooResource,
    updateFields map[string]any, plan *Plan,
) {
    namespace := extractNamespace(desired.Kongctl)
    protection := extractProtection(desired.Kongctl)

    // Include current labels for removal support
    updateFields[FieldCurrentLabels] = current.NormalizedLabels

    config := UpdateConfig{
        ResourceType:   "foo",
        ResourceName:   desired.Name,
        ResourceRef:    desired.GetRef(),
        ResourceID:     current.ID,
        FieldExtractor: func(_ any) map[string]any {
            return updateFields
        },
        Namespace: namespace,
    }

    change, err := p.genericPlanner.PlanUpdate(context.Background(), config)
    if err != nil {
        // Handle error appropriately - this is example code
        // In real implementation, return the error
        return
    }
    change.Protection = protection

    plan.AddChange(change)
}

func (p *Planner) planFooDelete(foo state.Foo, plan *Plan) {
    namespace := DefaultNamespace
    if ns, ok := foo.NormalizedLabels[labels.NamespaceKey]; ok {
        namespace = ns
    }

    config := DeleteConfig{
        ResourceType: "foo",
        ResourceName: foo.Name,
        ResourceRef:  foo.Name,
        ResourceID:   foo.ID,
        Namespace:    namespace,
    }

    change := p.genericPlanner.PlanDelete(context.Background(), config)
    plan.AddChange(change)
}
```

**ADD TO**: `internal/declarative/planner/planner.go`
```go
type Planner struct {
    fooPlannerImpl *fooPlannerImpl
    // ... existing planners
}

func NewPlanner(client *state.Client, resourceSet *resources.ResourceSet) *Planner {
    p := &Planner{
        client:    client,
        resources: resourceSet,
    }
    p.fooPlannerImpl = newFooPlanner(p, resourceSet)
    // ... initialize other planners
    return p
}

func (p *Planner) GeneratePlan(...) {
    // In namespace loop, add:
    if err := p.fooPlannerImpl.PlanChanges(ctx, plannerCtx, plan); err != nil {
        return nil, err
    }
}
```

#### 4. EXECUTOR ADAPTER
**Location**: `internal/declarative/executor/foo_adapter.go`

```go
package executor

import (
    "context"
    "github.com/Kong/sdk-konnect-go/models/components"
    "github.com/kong/kongctl/internal/declarative/labels"
    "github.com/kong/kongctl/internal/declarative/state"
)

type FooAdapter struct {
    client *state.Client
}

func NewFooAdapter(client *state.Client) *FooAdapter {
    return &FooAdapter{client: client}
}

func (a *FooAdapter) MapCreateFields(
    _ context.Context, execCtx *ExecutionContext,
    fields map[string]any, create *components.CreateFooRequest,
) error {
    namespace := execCtx.Namespace
    protection := execCtx.Protection

    // Map required fields
    create.Name = common.ExtractResourceName(fields)

    // Map optional fields
    common.MapOptionalStringFieldToPtr(&create.Description, fields, "description")

    // Handle labels
    userLabels := labels.ExtractLabelsFromField(fields["labels"])
    labelsMap := labels.BuildCreateLabels(userLabels, namespace, protection)

    // Convert to SDK format
    if len(labelsMap) > 0 {
        // If SDK uses map[string]*string:
        create.Labels = labels.ConvertStringMapToPointerMap(labelsMap)
        // If SDK uses map[string]string:
        // create.Labels = labelsMap
    }

    return nil
}

func (a *FooAdapter) MapUpdateFields(
    _ context.Context, execCtx *ExecutionContext,
    fields map[string]any, update *components.UpdateFooRequest,
    currentLabels map[string]string,
) error {
    namespace := execCtx.Namespace
    protection := execCtx.Protection

    // Only include changed fields
    for field, value := range fields {
        switch field {
        case "name":
            if name, ok := value.(string); ok {
                update.Name = &name
            }
        case "description":
            if desc, ok := value.(string); ok {
                update.Description = &desc
            }
        }
    }

    // Handle labels
    desiredLabels := labels.ExtractLabelsFromField(fields["labels"])
    if desiredLabels != nil {
        plannerCurrentLabels := labels.ExtractLabelsFromField(fields[planner.FieldCurrentLabels])
        if plannerCurrentLabels != nil {
            currentLabels = plannerCurrentLabels
        }

        labelsMap := labels.BuildUpdateLabels(desiredLabels, currentLabels, namespace, protection)

        // Convert to SDK format
        update.Labels = labels.ConvertStringMapToPointerMap(labelsMap)
        // OR: update.Labels = labelsMap
    } else if currentLabels != nil {
        // No label changes, preserve with updated protection
        labelsMap := labels.BuildUpdateLabels(currentLabels, currentLabels, namespace, protection)
        update.Labels = labels.ConvertStringMapToPointerMap(labelsMap)
    }

    return nil
}

func (a *FooAdapter) Create(
    ctx context.Context, req components.CreateFooRequest,
    namespace string, _ *ExecutionContext,
) (string, error) {
    resp, err := a.client.CreateFoo(ctx, req, namespace)
    if err != nil {
        return "", err
    }
    return resp.ID, nil
}

func (a *FooAdapter) Update(
    ctx context.Context, id string, req components.UpdateFooRequest,
    namespace string, _ *ExecutionContext,
) (string, error) {
    resp, err := a.client.UpdateFoo(ctx, id, req, namespace)
    if err != nil {
        return "", err
    }
    return resp.ID, nil
}

func (a *FooAdapter) Delete(ctx context.Context, id string, _ *ExecutionContext) error {
    return a.client.DeleteFoo(ctx, id)
}

func (a *FooAdapter) ResourceType() string {
    return "foo"
}

func (a *FooAdapter) RequiredFields() []string {
    return []string{"name"}
}

func (a *FooAdapter) SupportsUpdate() bool {
    return true
}
```

**ADD TO**: `internal/declarative/executor/executor.go`
```go
type Executor struct {
    fooAdapter *FooAdapter
    // ... existing adapters
}

func New(client *state.Client, reporter ProgressReporter, dryRun bool) *Executor {
    return &Executor{
        fooAdapter: NewFooAdapter(client),
        // ... other adapters
    }
}

func (e *Executor) executeChange(ctx context.Context, change planner.PlannedChange) error {
    switch change.ResourceType {
    case "foo":
        return e.executeFooChange(ctx, change)
    // ... other cases
    }
}

func (e *Executor) executeFooChange(ctx context.Context, change planner.PlannedChange) error {
    execCtx := &ExecutionContext{
        PlannedChange: &change,
        Namespace:     change.Namespace,
        Protection:    change.Protection,
    }

    switch change.Action {
    case planner.ActionCreate:
        var req components.CreateFooRequest
        if err := e.fooAdapter.MapCreateFields(ctx, execCtx, change.Fields, &req); err != nil {
            return err
        }
        id, err := e.fooAdapter.Create(ctx, req, change.Namespace, execCtx)
        if err != nil {
            return err
        }
        e.trackCreatedResource(change.ResourceRef, id)
        return nil

    case planner.ActionUpdate:
        var req components.UpdateFooRequest
        if err := e.fooAdapter.MapUpdateFields(ctx, execCtx, change.Fields, &req, nil); err != nil {
            return err
        }
        _, err := e.fooAdapter.Update(ctx, change.ResourceID, req, change.Namespace, execCtx)
        return err

    case planner.ActionDelete:
        return e.fooAdapter.Delete(ctx, change.ResourceID, execCtx)
    }

    return fmt.Errorf("unknown action: %s", change.Action)
}
```

---

### CHILD RESOURCE

#### 1. RESOURCE DEFINITION
**Location**: `internal/declarative/resources/foo_child.go`

```go
type FooChildResource struct {
    kkComps.CreateFooChildRequest `yaml:",inline" json:",inline"`
    Ref    string `yaml:"ref" json:"ref"`
    Foo    string `yaml:"foo,omitempty" json:"foo,omitempty"`  // Parent ref

    // Nested children if hierarchical
    Children []FooChildResource `yaml:"children,omitempty" json:"children,omitempty"`

    konnectID string `yaml:"-" json:"-"`
}

func (f FooChildResource) GetType() ResourceType {
    return ResourceTypeFooChild
}

func (f FooChildResource) GetRef() string {
    return f.Ref
}

func (f FooChildResource) GetMoniker() string {
    return f.Slug  // or f.Name, f.Version, etc.
}

func (f FooChildResource) GetDependencies() []ResourceRef {
    deps := []ResourceRef{}
    if f.Foo != "" {
        deps = append(deps, ResourceRef{Kind: "foo", Ref: f.Foo})
    }
    return deps
}

func (f FooChildResource) Validate() error {
    if err := ValidateRef(f.Ref); err != nil {
        return fmt.Errorf("invalid child ref: %w", err)
    }

    // Validate required fields
    if f.Slug == "" {
        return fmt.Errorf("slug is required")
    }

    // Validate nested children
    // Children nested under parent automatically inherit the parent reference
    // and should not redefine it (to avoid conflicts)
    for i, child := range f.Children {
        if child.Foo != "" {
            return fmt.Errorf("child[%d] should not define foo (inherited from parent)", i)
        }
        if err := child.Validate(); err != nil {
            return fmt.Errorf("child[%d] validation failed: %w", i, err)
        }
    }

    return nil
}

func (f *FooChildResource) SetDefaults() {
    // Set defaults
    for i := range f.Children {
        f.Children[i].SetDefaults()
    }
}

func (f FooChildResource) GetKonnectID() string {
    return f.konnectID
}

func (f FooChildResource) GetKonnectMonikerFilter() string {
    return fmt.Sprintf("slug[eq]=%s", f.Slug)
}

func (f *FooChildResource) TryMatchKonnectResource(konnectResource any) bool {
    v := reflect.ValueOf(konnectResource)
    if v.Kind() == reflect.Ptr {
        v = v.Elem()
    }
    if v.Kind() != reflect.Struct {
        return false
    }

    slugField := v.FieldByName("Slug")
    idField := v.FieldByName("ID")

    if slugField.IsValid() && idField.IsValid() &&
        slugField.Kind() == reflect.String && idField.Kind() == reflect.String {
        if slugField.String() == f.Slug {
            f.konnectID = idField.String()
            return true
        }
    }
    return false
}

// REQUIRED: Implement ResourceWithParent
func (f FooChildResource) GetParentRef() *ResourceRef {
    if f.Foo != "" {
        return &ResourceRef{Kind: "foo", Ref: f.Foo}
    }
    return nil
}

// Custom JSON unmarshaling to reject kongctl metadata
func (f *FooChildResource) UnmarshalJSON(data []byte) error {
    var temp struct {
        Ref      string                `json:"ref"`
        Foo      string                `json:"foo,omitempty"`
        Slug     string                `json:"slug"`
        Content  string                `json:"content"`
        Children []FooChildResource    `json:"children,omitempty"`
        Kongctl  any                   `json:"kongctl,omitempty"`
    }

    if err := json.Unmarshal(data, &temp); err != nil {
        return err
    }

    if temp.Kongctl != nil {
        return fmt.Errorf("kongctl metadata not supported on child resources")
    }

    f.Ref = temp.Ref
    f.Foo = temp.Foo
    f.Slug = temp.Slug
    f.Content = temp.Content
    f.Children = temp.Children

    return nil
}
```

**ADD TO**: `internal/declarative/resources/types.go`
```go
const (
    ResourceTypeFooChild ResourceType = "foo_child"
)

type ResourceSet struct {
    FooChildren []FooChildResource `yaml:"foo_children,omitempty" json:"foo_children,omitempty"`
    // ... existing resources
}
```

**ADD TO PARENT**: `internal/declarative/resources/foo.go`
```go
type FooResource struct {
    // ... existing fields
    Children []FooChildResource `yaml:"children,omitempty" json:"children,omitempty"`
}
```

#### 2. STATE CLIENT METHODS
**ADD TO**: `internal/declarative/state/client.go`

```go
type FooChild struct {
    ID      string
    Slug    string
    Content string
    // ... other fields
}

func (c *Client) ListFooChildren(ctx context.Context, fooID string) ([]FooChild, error) {
    resp, err := c.fooChildAPI.ListFooChildren(ctx, fooID)
    if err != nil {
        return nil, WrapAPIError(err, "list foo children", nil)
    }

    var children []FooChild
    for _, child := range resp.Data {
        children = append(children, FooChild{
            ID:      child.ID,
            Slug:    child.Slug,
            Content: child.Content,
        })
    }

    return children, nil
}

func (c *Client) GetFooChild(ctx context.Context, fooID, childID string) (*FooChild, error) {
    resp, err := c.fooChildAPI.GetFooChild(ctx, fooID, childID)
    if err != nil {
        return nil, err
    }

    return &FooChild{
        ID:      resp.ID,
        Slug:    resp.Slug,
        Content: resp.Content,
    }, nil
}

func (c *Client) CreateFooChild(ctx context.Context, fooID string, child components.CreateFooChildRequest) (string, error) {
    resp, err := c.fooChildAPI.CreateFooChild(ctx, fooID, child)
    if err != nil {
        return "", err
    }
    return resp.ID, nil
}

func (c *Client) UpdateFooChild(ctx context.Context, fooID, childID string, child components.UpdateFooChildRequest) error {
    _, err := c.fooChildAPI.UpdateFooChild(ctx, fooID, childID, child)
    return err
}

func (c *Client) DeleteFooChild(ctx context.Context, fooID, childID string) error {
    return c.fooChildAPI.DeleteFooChild(ctx, fooID, childID)
}
```

#### 3. PLANNER IMPLEMENTATION
**ADD TO**: `internal/declarative/planner/foo_planner.go`

```go
func (f *fooPlannerImpl) GetDesiredFooChildren(namespace string) []resources.FooChildResource {
    var result []resources.FooChildResource
    for _, child := range f.resources.FooChildren {
        // Child resources inherit namespace from parent
        // Filter by parent's namespace
        result = append(result, child)
    }
    return result
}

func (f *fooPlannerImpl) PlanChanges(ctx context.Context, plannerCtx *Config, plan *Plan) error {
    namespace := plannerCtx.Namespace

    // Plan parent foos
    if err := f.planner.planFooChanges(ctx, plannerCtx, f.GetDesiredFoos(namespace), plan); err != nil {
        return err
    }

    // Plan root-level child resources
    if err := f.planner.planFooChildrenChanges(ctx, plannerCtx, namespace, f.GetDesiredFooChildren(namespace), plan); err != nil {
        return err
    }

    return nil
}

// In planner.go:
func (p *Planner) planFooChildrenChanges(
    ctx context.Context, plannerCtx *Config, namespace string,
    desired []resources.FooChildResource, plan *Plan,
) error {
    // Group by parent foo
    childrenByFoo := make(map[string][]resources.FooChildResource)
    for _, child := range desired {
        childrenByFoo[child.Foo] = append(childrenByFoo[child.Foo], child)
    }

    // Get foo name to ID mapping
    fooNameToID := p.buildFooNameToIDMap(namespace)

    for fooName, children := range childrenByFoo {
        fooID := fooNameToID[fooName]

        if fooID != "" {
            // Foo exists: full diff
            if err := p.planFooChildChangesForExistingFoo(ctx, namespace, fooID, fooName, children, plan); err != nil {
                return err
            }
        } else {
            // Foo doesn't exist: plan creates only
            p.planFooChildrenCreateForNewFoo(namespace, fooName, children, plan)
        }
    }

    return nil
}

func (p *Planner) planFooChildChangesForExistingFoo(
    ctx context.Context, namespace string, fooID string, fooRef string,
    desired []resources.FooChildResource, plan *Plan,
) error {
    // 1. List current children
    currentChildren, err := p.client.ListFooChildren(ctx, fooID)
    if err != nil {
        return err
    }

    // 2. Index by slug (or other identifier)
    currentBySlug := make(map[string]state.FooChild)
    for _, child := range currentChildren {
        currentBySlug[child.Slug] = child
    }

    desiredSlugs := make(map[string]bool)

    // 3. Compare desired vs current
    for _, desiredChild := range desired {
        desiredSlugs[desiredChild.Slug] = true

        if current, exists := currentBySlug[desiredChild.Slug]; !exists {
            // CREATE
            p.planFooChildCreate(namespace, fooRef, fooID, desiredChild, []string{}, plan)
        } else {
            // CHECK UPDATE
            fullChild, err := p.client.GetFooChild(ctx, fooID, current.ID)
            if err != nil {
                return err
            }

            if p.shouldUpdateFooChild(*fullChild, desiredChild) {
                p.planFooChildUpdate(namespace, fooRef, fooID, current.ID, desiredChild, plan)
            }
        }
    }

    // 4. SYNC MODE: Delete unmanaged
    if plan.Metadata.Mode == PlanModeSync {
        for slug, current := range currentBySlug {
            if !desiredSlugs[slug] {
                p.planFooChildDelete(fooRef, fooID, current.ID, slug, plan)
            }
        }
    }

    return nil
}

func (p *Planner) planFooChildrenCreateForNewFoo(
    namespace string, fooRef string, children []resources.FooChildResource, plan *Plan,
) {
    for _, child := range children {
        p.planFooChildCreate(namespace, fooRef, "", child, []string{}, plan)
    }
}

func (p *Planner) planFooChildCreate(
    namespace string, fooRef string, fooID string,
    child resources.FooChildResource, dependsOn []string, plan *Plan,
) {
    fields := make(map[string]any)
    fields["slug"] = child.Slug
    fields["content"] = child.Content

    change := &planner.PlannedChange{
        ID:           fmt.Sprintf("change-%d", len(plan.Changes)+1),
        ResourceType: "foo_child",
        ResourceRef:  child.GetRef(),
        Action:       planner.ActionCreate,
        Fields:       fields,
        Namespace:    namespace,
        DependsOn:    dependsOn,
    }

    // Set parent reference
    if fooID != "" {
        change.Parent = &planner.ParentInfo{
            Type: "foo",
            Ref:  fooRef,
            ID:   fooID,
        }
    } else {
        // Parent doesn't exist yet, add reference for runtime resolution
        change.References = map[string]planner.ReferenceInfo{
            "foo_id": {
                Ref: fooRef,
                ID:  "",  // Will be resolved at execution
                LookupFields: map[string]string{
                    "name": fooRef,
                },
            },
        }
    }

    plan.AddChange(*change)
}

func (p *Planner) shouldUpdateFooChild(current state.FooChild, desired resources.FooChildResource) bool {
    if current.Content != desired.Content {
        return true
    }
    // Compare other fields
    return false
}
```

**CHILD RESOURCES CREATED WITH PARENT**: Add to `planFooCreate`:
```go
func (p *Planner) planFooCreate(foo resources.FooResource, plan *Plan) string {
    // Create the parent foo change (see full implementation earlier in guide)
    protection := extractProtection(foo.Kongctl)
    namespace := extractNamespace(foo.Kongctl)

    config := CreateConfig{
        ResourceType:   "foo",
        ResourceName:   foo.Name,
        ResourceRef:    foo.GetRef(),
        RequiredFields: []string{"name"},
        FieldExtractor: func(_ any) map[string]any {
            return extractFooFields(foo)
        },
        Namespace: namespace,
        DependsOn: []string{},
    }

    change, err := p.genericPlanner.PlanCreate(context.Background(), config)
    if err != nil {
        return ""
    }
    change.Protection = protection
    plan.AddChange(change)

    // Get the change ID of the just-added parent
    fooChangeID := change.ID

    // Plan nested children with dependency on parent
    p.planFooChildrenCreateWithParent(namespace, foo.GetRef(), foo.Children, fooChangeID, plan)

    return fooChangeID
}

func (p *Planner) planFooChildrenCreateWithParent(
    namespace string, fooRef string, children []resources.FooChildResource,
    parentChangeID string, plan *Plan,
) {
    for _, child := range children {
        p.planFooChildCreate(namespace, fooRef, "", child, []string{parentChangeID}, plan)
    }
}
```

#### 4. EXECUTOR ADAPTER
**Location**: `internal/declarative/executor/foo_child_adapter.go`

```go
type FooChildAdapter struct {
    client *state.Client
}

func NewFooChildAdapter(client *state.Client) *FooChildAdapter {
    return &FooChildAdapter{client: client}
}

func (a *FooChildAdapter) MapCreateFields(
    _ context.Context, execCtx *ExecutionContext,
    fields map[string]any, create *components.CreateFooChildRequest,
) error {
    slug, ok := fields["slug"].(string)
    if !ok {
        return fmt.Errorf("slug is required")
    }
    create.Slug = slug

    content, ok := fields["content"].(string)
    if !ok {
        return fmt.Errorf("content is required")
    }
    create.Content = content

    // Handle parent reference if hierarchical
    if execCtx != nil && execCtx.PlannedChange != nil {
        if parentRef, ok := execCtx.PlannedChange.References["parent_child_id"]; ok {
            if parentRef.ID != "" {
                create.ParentChildID = &parentRef.ID
            }
        }
    }

    return nil
}

func (a *FooChildAdapter) MapUpdateFields(
    _ context.Context, execCtx *ExecutionContext,
    fields map[string]any, update *components.UpdateFooChildRequest,
    _ map[string]string,
) error {
    if slug, ok := fields["slug"].(string); ok {
        update.Slug = &slug
    }

    if content, ok := fields["content"].(string); ok {
        update.Content = &content
    }

    return nil
}

func (a *FooChildAdapter) Create(
    ctx context.Context, req components.CreateFooChildRequest,
    _ string, execCtx *ExecutionContext,
) (string, error) {
    fooID, err := a.getFooIDFromExecutionContext(execCtx)
    if err != nil {
        return "", err
    }

    return a.client.CreateFooChild(ctx, fooID, req)
}

func (a *FooChildAdapter) Update(
    ctx context.Context, id string, req components.UpdateFooChildRequest,
    _ string, execCtx *ExecutionContext,
) (string, error) {
    fooID, err := a.getFooIDFromExecutionContext(execCtx)
    if err != nil {
        return "", err
    }

    if err := a.client.UpdateFooChild(ctx, fooID, id, req); err != nil {
        return "", err
    }
    return id, nil
}

func (a *FooChildAdapter) Delete(ctx context.Context, id string, execCtx *ExecutionContext) error {
    fooID, err := a.getFooIDFromExecutionContext(execCtx)
    if err != nil {
        return err
    }

    return a.client.DeleteFooChild(ctx, fooID, id)
}

func (a *FooChildAdapter) ResourceType() string {
    return "foo_child"
}

func (a *FooChildAdapter) RequiredFields() []string {
    return []string{"slug", "content"}
}

func (a *FooChildAdapter) SupportsUpdate() bool {
    return true
}

func (a *FooChildAdapter) getFooIDFromExecutionContext(execCtx *ExecutionContext) (string, error) {
    if execCtx == nil || execCtx.PlannedChange == nil {
        return "", fmt.Errorf("execution context required")
    }

    change := *execCtx.PlannedChange

    // Priority 1: Check References (for new parent)
    if fooRef, ok := change.References["foo_id"]; ok && fooRef.ID != "" {
        return fooRef.ID, nil
    }

    // Priority 2: Check Parent field (for existing parent)
    if change.Parent != nil && change.Parent.ID != "" {
        return change.Parent.ID, nil
    }

    return "", fmt.Errorf("foo ID required for child operations")
}
```

**ADD TO EXECUTOR**: `internal/declarative/executor/executor.go`
```go
type Executor struct {
    fooChildAdapter *FooChildAdapter
    // ... existing
}

func New(...) *Executor {
    return &Executor{
        fooChildAdapter: NewFooChildAdapter(client),
        // ...
    }
}

func (e *Executor) executeChange(ctx context.Context, change planner.PlannedChange) error {
    switch change.ResourceType {
    case "foo_child":
        return e.executeFooChildChange(ctx, change)
    // ...
    }
}

func (e *Executor) executeFooChildChange(ctx context.Context, change planner.PlannedChange) error {
    execCtx := &ExecutionContext{
        PlannedChange: &change,
        Namespace:     change.Namespace,
    }

    switch change.Action {
    case planner.ActionCreate:
        var req components.CreateFooChildRequest
        if err := e.fooChildAdapter.MapCreateFields(ctx, execCtx, change.Fields, &req); err != nil {
            return err
        }
        id, err := e.fooChildAdapter.Create(ctx, req, change.Namespace, execCtx)
        if err != nil {
            return err
        }
        e.trackCreatedResource(change.ResourceRef, id)
        return nil

    case planner.ActionUpdate:
        var req components.UpdateFooChildRequest
        if err := e.fooChildAdapter.MapUpdateFields(ctx, execCtx, change.Fields, &req, nil); err != nil {
            return err
        }
        _, err := e.fooChildAdapter.Update(ctx, change.ResourceID, req, change.Namespace, execCtx)
        return err

    case planner.ActionDelete:
        return e.fooChildAdapter.Delete(ctx, change.ResourceID, execCtx)
    }

    return fmt.Errorf("unknown action: %s", change.Action)
}
```

---

### SINGLETON CHILD RESOURCE

**Pattern**: Same as child resource, but:
1. **NO CREATE/DELETE**: Only UPDATE operations
2. **Always exists**: For every parent instance
3. **Planner always generates UPDATE**: Never checks if exists

**Example**: PortalCustomization

**Key Differences**:

```go
// In planner:
func (p *Planner) planFooCustomizationChanges(...) error {
    // NO LIST/COMPARE - always plan UPDATE

    for _, desired := range desiredCustomizations {
        fooID := fooNameToID[desired.Foo]

        if fooID != "" {
            // Foo exists: fetch current and compare
            current, err := p.client.GetFooCustomization(ctx, fooID)
            needsUpdate := p.shouldUpdateFooCustomization(current, desired)
            if needsUpdate {
                p.planFooCustomizationUpdate(namespace, desired, fooID, plan)
            }
        } else {
            // Foo doesn't exist yet: plan update for later
            p.planFooCustomizationUpdate(namespace, desired, "", plan)
        }
    }

    // NO DELETE LOGIC - customization always exists

    return nil
}

// State client: NO Create, NO Delete
func (c *Client) GetFooCustomization(ctx context.Context, fooID string) (*FooCustomization, error) {
    // Always returns result (never 404)
}

func (c *Client) UpdateFooCustomization(ctx context.Context, fooID string, customization components.UpdateFooCustomization) error {
    // Only update method
}

// Adapter: NO Create, NO Delete
func (a *FooCustomizationAdapter) Create(...) { panic("not supported") }
func (a *FooCustomizationAdapter) Delete(...) { panic("not supported") }
func (a *FooCustomizationAdapter) SupportsUpdate() bool { return true }
```

---

## IMPERATIVE GET COMMAND IMPLEMENTATION

### PARENT RESOURCE GET COMMAND

#### 1. GET COMMAND ENTRY POINT
**Location**: `internal/cmd/root/verbs/get/foo.go`

```go
package get

import (
    "context"
    "fmt"
    "github.com/kong/kongctl/internal/cmd"
    "github.com/kong/kongctl/internal/cmd/root/products"
    "github.com/kong/kongctl/internal/cmd/root/products/konnect"
    "github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
    "github.com/kong/kongctl/internal/cmd/root/products/konnect/foo"
    "github.com/kong/kongctl/internal/cmd/root/verbs"
    "github.com/kong/kongctl/internal/konnect/helpers"
    "github.com/spf13/cobra"
)

func NewDirectFooCmd() (*cobra.Command, error) {
    addFlags := func(verb verbs.VerbValue, cmd *cobra.Command) {
        cmd.Flags().String(common.BaseURLFlagName, common.BaseURLDefault,
            fmt.Sprintf(`Base URL for Konnect API requests.
- Config path: [ %s ]`, common.BaseURLConfigPath))

        cmd.Flags().String(common.PATFlagName, "",
            fmt.Sprintf(`Konnect Personal Access Token (PAT).
- Config path: [ %s ]`, common.PATConfigPath))

        if verb == verbs.Get || verb == verbs.List {
            cmd.Flags().Int(common.RequestPageSizeFlagName, common.DefaultRequestPageSize,
                fmt.Sprintf(`Max number of results per page.
- Config path: [ %s ]`, common.RequestPageSizeConfigPath))
        }
    }

    preRunE := func(c *cobra.Command, args []string) error {
        ctx := c.Context()
        if ctx == nil {
            ctx = context.Background()
        }
        ctx = context.WithValue(ctx, products.Product, konnect.Product)
        ctx = context.WithValue(ctx, helpers.SDKAPIFactoryKey, helpers.SDKAPIFactory(common.KonnectSDKFactory))
        c.SetContext(ctx)

        return bindFooFlags(c, args)
    }

    fooCmd, err := foo.NewFooCmd(Verb, addFlags, preRunE)
    if err != nil {
        return nil, err
    }

    fooCmd.Example = `  # List all foos
  kongctl get foos
  # Get a specific foo
  kongctl get foo <id|name>
  # List child resources
  kongctl get foo children --foo-id <foo-id>`

    return fooCmd, nil
}

func bindFooFlags(c *cobra.Command, args []string) error {
    helper := cmd.BuildHelper(c, args)
    cfg, err := helper.GetConfig()
    if err != nil {
        return err
    }

    if f := c.Flags().Lookup(common.BaseURLFlagName); f != nil {
        if err := cfg.BindFlag(common.BaseURLConfigPath, f); err != nil {
            return err
        }
    }

    if f := c.Flags().Lookup(common.PATFlagName); f != nil {
        if err := cfg.BindFlag(common.PATConfigPath, f); err != nil {
            return err
        }
    }

    if f := c.Flags().Lookup(common.RequestPageSizeFlagName); f != nil {
        if err := cfg.BindFlag(common.RequestPageSizeConfigPath, f); err != nil {
            return err
        }
    }

    return nil
}
```

**ADD TO**: `internal/cmd/root/verbs/get/get.go`
```go
func NewGetCmd() (*cobra.Command, error) {
    // ... existing code

    // Add foo command for Konnect-first pattern
    fooCmd, err := NewDirectFooCmd()
    if err != nil {
        return nil, err
    }
    cmd.AddCommand(fooCmd)

    return cmd, nil
}
```

#### 2. RESOURCE COMMAND IMPLEMENTATION
**Location**: `internal/cmd/root/products/konnect/foo/foo.go`

```go
package foo

import (
    "fmt"
    "github.com/kong/kongctl/internal/cmd/root/verbs"
    "github.com/kong/kongctl/internal/meta"
    "github.com/kong/kongctl/internal/util/i18n"
    "github.com/kong/kongctl/internal/util/normalizers"
    "github.com/spf13/cobra"
)

const (
    CommandName = "foo"
)

var (
    fooUse   = CommandName
    fooShort = i18n.T("root.products.konnect.foo.fooShort", "Manage Konnect Foo resources")
    fooLong  = normalizers.LongDesc(i18n.T("root.products.konnect.foo.fooLong",
        `The foo command allows you to work with Konnect Foo resources.`))
    fooExample = normalizers.Examples(i18n.T("root.products.konnect.foo.fooExamples",
        fmt.Sprintf(`
# List all foos
%[1]s get foos
# Get a specific foo
%[1]s get foo <id|name>
# List child resources
%[1]s get foo children --foo-id <foo-id>
`, meta.CLIName)))
)

func NewFooCmd(
    verb verbs.VerbValue,
    addParentFlags func(verbs.VerbValue, *cobra.Command),
    parentPreRun func(*cobra.Command, []string) error,
) (*cobra.Command, error) {
    baseCmd := cobra.Command{
        Use:     fooUse,
        Short:   fooShort,
        Long:    fooLong,
        Example: fooExample,
        Aliases: []string{"foos", "f", "F"},
    }

    switch verb {
    case verbs.Get:
        return newGetFooCmd(verb, &baseCmd, addParentFlags, parentPreRun).Command, nil
    case verbs.List:
        return newGetFooCmd(verb, &baseCmd, addParentFlags, parentPreRun).Command, nil
    default:
        return &baseCmd, nil
    }
}
```

#### 3. GET COMMAND HANDLER
**Location**: `internal/cmd/root/products/konnect/foo/getFoo.go`

```go
package foo

import (
    "context"
    "fmt"
    "sort"
    "strings"
    "time"

    kk "github.com/Kong/sdk-konnect-go"
    kkComps "github.com/Kong/sdk-konnect-go/models/components"
    kkOps "github.com/Kong/sdk-konnect-go/models/operations"
    "github.com/charmbracelet/bubbles/table"
    "github.com/kong/kongctl/internal/cmd"
    "github.com/kong/kongctl/internal/cmd/output/tableview"
    "github.com/kong/kongctl/internal/konnect/helpers"
    "github.com/kong/kongctl/internal/util"
    "github.com/spf13/cobra"
)

type getFooCmd struct {
    *cobra.Command
}

func newGetFooCmd(
    verb verbs.VerbValue,
    base *cobra.Command,
    addParentFlags func(verbs.VerbValue, *cobra.Command),
    parentPreRun func(*cobra.Command, []string) error,
) *getFooCmd {
    cmd := &getFooCmd{
        Command: &cobra.Command{
            Use:     base.Use,
            Short:   "List or get Konnect Foos",
            Long:    `Use the get verb with the foo command to query Konnect Foos.`,
            Aliases: base.Aliases,
            PreRunE: parentPreRun,
            RunE: func(c *cobra.Command, args []string) error {
                return runGetFoo(c, args)
            },
        },
    }

    if addParentFlags != nil {
        addParentFlags(verb, cmd.Command)
    }

    // Add child resource subcommands
    cmd.AddCommand(newGetFooChildrenCmd(verb, addParentFlags, parentPreRun))

    return cmd
}

type textDisplayRecord struct {
    ID               string
    Name             string
    Description      string
    LocalCreatedTime string
    LocalUpdatedTime string
}

func fooToDisplayRecord(f *kkComps.FooResponseSchema) textDisplayRecord {
    const missing = "n/a"

    id := missing
    if f.ID != "" {
        id = util.AbbreviateUUID(f.ID)
    }

    name := missing
    if f.Name != "" {
        name = f.Name
    }

    description := missing
    if f.Description != nil && *f.Description != "" {
        description = *f.Description
    }

    createdAt := f.CreatedAt.In(time.Local).Format("2006-01-02 15:04:05")
    updatedAt := f.UpdatedAt.In(time.Local).Format("2006-01-02 15:04:05")

    return textDisplayRecord{
        ID:               id,
        Name:             name,
        Description:      description,
        LocalCreatedTime: createdAt,
        LocalUpdatedTime: updatedAt,
    }
}

func runGetFoo(c *cobra.Command, args []string) error {
    helper := cmd.BuildHelper(c, args)

    ctx := c.Context()
    client, err := helpers.GetSDKAPIFactory[*kk.SDK](ctx, helper)
    if err != nil {
        return err
    }

    // No args: list all
    if len(args) == 0 {
        return listFoos(ctx, helper, client)
    }

    // One arg: get specific foo by ID or name
    if len(args) == 1 {
        return getFoo(ctx, helper, client, args[0])
    }

    return fmt.Errorf("too many arguments")
}

func listFoos(ctx context.Context, helper *cmd.Helper, client *kk.SDK) error {
    pageSize := int64(100)
    pageNumber := int64(1)

    req := kkOps.ListFoosRequest{
        PageSize:   &pageSize,
        PageNumber: &pageNumber,
    }

    resp, err := client.Foos.ListFoos(ctx, req)
    if err != nil {
        return fmt.Errorf("failed to list foos: %w", err)
    }

    // Handle different output formats
    format, err := helper.GetOutputFormat()
    if err != nil {
        return err
    }

    switch format {
    case cmd.OutputFormatJSON:
        return helper.OutputJSON(resp.ListFoosResponse.Data)
    case cmd.OutputFormatYAML:
        return helper.OutputYAML(resp.ListFoosResponse.Data)
    default:
        // Text table output
        return outputFooTable(helper, resp.ListFoosResponse.Data)
    }
}

// Helper function to check if string is a valid UUID
func isUUID(s string) bool {
    // Simple UUID format check (8-4-4-4-12 hex digits)
    return len(s) == 36 && s[8] == '-' && s[13] == '-' && s[18] == '-' && s[23] == '-'
}

func getFoo(ctx context.Context, helper *cmd.Helper, client *kk.SDK, idOrName string) error {
    // Try as UUID first
    if isUUID(idOrName) {
        resp, err := client.Foos.GetFoo(ctx, idOrName)
        if err == nil {
            return outputSingleFoo(helper, resp.FooResponse)
        }
    }

    // Try as name
    pageSize := int64(100)
    filter := fmt.Sprintf("name[eq]=%s", idOrName)

    resp, err := client.Foos.ListFoos(ctx, kkOps.ListFoosRequest{
        PageSize: &pageSize,
        Filter:   &filter,
    })
    if err != nil {
        return fmt.Errorf("failed to find foo: %w", err)
    }

    if len(resp.ListFoosResponse.Data) == 0 {
        return fmt.Errorf("foo not found: %s", idOrName)
    }

    if len(resp.ListFoosResponse.Data) > 1 {
        return fmt.Errorf("multiple foos found with name: %s", idOrName)
    }

    return outputSingleFoo(helper, &resp.ListFoosResponse.Data[0])
}

func outputFooTable(helper *cmd.Helper, foos []kkComps.FooResponseSchema) error {
    rows := make([]table.Row, 0, len(foos))
    for _, f := range foos {
        rec := fooToDisplayRecord(&f)
        rows = append(rows, table.Row{
            rec.ID,
            rec.Name,
            rec.Description,
            rec.LocalCreatedTime,
            rec.LocalUpdatedTime,
        })
    }

    columns := []table.Column{
        {Title: "ID", Width: 10},
        {Title: "Name", Width: 30},
        {Title: "Description", Width: 40},
        {Title: "Created", Width: 20},
        {Title: "Updated", Width: 20},
    }

    t := tableview.NewTableView(columns, rows)
    helper.Output(t.View())

    return nil
}

func outputSingleFoo(helper *cmd.Helper, foo *kkComps.FooResponseSchema) error {
    format, err := helper.GetOutputFormat()
    if err != nil {
        return err
    }

    switch format {
    case cmd.OutputFormatJSON:
        return helper.OutputJSON(foo)
    case cmd.OutputFormatYAML:
        return helper.OutputYAML(foo)
    default:
        return outputFooDetail(helper, foo)
    }
}

func outputFooDetail(helper *cmd.Helper, foo *kkComps.FooResponseSchema) error {
    var output strings.Builder

    output.WriteString(fmt.Sprintf("ID: %s\n", foo.ID))
    output.WriteString(fmt.Sprintf("Name: %s\n", foo.Name))

    if foo.Description != nil {
        output.WriteString(fmt.Sprintf("Description: %s\n", *foo.Description))
    }

    output.WriteString(fmt.Sprintf("Created: %s\n",
        foo.CreatedAt.In(time.Local).Format("2006-01-02 15:04:05")))
    output.WriteString(fmt.Sprintf("Updated: %s\n",
        foo.UpdatedAt.In(time.Local).Format("2006-01-02 15:04:05")))

    helper.Output(output.String())
    return nil
}
```

### CHILD RESOURCE GET COMMAND

**Location**: `internal/cmd/root/products/konnect/foo/children.go`

```go
package foo

import (
    "context"
    "fmt"

    kk "github.com/Kong/sdk-konnect-go"
    kkOps "github.com/Kong/sdk-konnect-go/models/operations"
    "github.com/kong/kongctl/internal/cmd"
    "github.com/spf13/cobra"
)

const (
    fooIDFlagName   = "foo-id"
    fooNameFlagName = "foo-name"
)

func newGetFooChildrenCmd(
    verb verbs.VerbValue,
    addParentFlags func(verbs.VerbValue, *cobra.Command),
    parentPreRun func(*cobra.Command, []string) error,
) *cobra.Command {
    cmd := &cobra.Command{
        Use:     "children",
        Short:   "Manage foo children",
        Aliases: []string{"child", "ch"},
        PreRunE: parentPreRun,
        RunE: func(c *cobra.Command, args []string) error {
            return runGetFooChildren(c, args)
        },
    }

    cmd.Flags().String(fooIDFlagName, "", "Foo ID")
    cmd.Flags().String(fooNameFlagName, "", "Foo name")

    if addParentFlags != nil {
        addParentFlags(verb, cmd)
    }

    return cmd
}

func runGetFooChildren(c *cobra.Command, args []string) error {
    helper := cmd.BuildHelper(c, args)

    // Get foo ID (from flag or name lookup)
    fooID, err := getFooIDFromFlags(c, helper)
    if err != nil {
        return err
    }

    ctx := c.Context()
    client, err := helpers.GetSDKAPIFactory[*kk.SDK](ctx, helper)
    if err != nil {
        return err
    }

    // No args: list all children
    if len(args) == 0 {
        return listFooChildren(ctx, helper, client, fooID)
    }

    // One arg: get specific child
    if len(args) == 1 {
        return getFooChild(ctx, helper, client, fooID, args[0])
    }

    return fmt.Errorf("too many arguments")
}

func getFooIDFromFlags(c *cobra.Command, helper *cmd.Helper) (string, error) {
    fooID, _ := c.Flags().GetString(fooIDFlagName)
    if fooID != "" {
        return fooID, nil
    }

    fooName, _ := c.Flags().GetString(fooNameFlagName)
    if fooName == "" {
        return "", fmt.Errorf("either --foo-id or --foo-name is required")
    }

    // Lookup foo by name
    ctx := c.Context()
    client, err := helpers.GetSDKAPIFactory[*kk.SDK](ctx, helper)
    if err != nil {
        return "", err
    }

    filter := fmt.Sprintf("name[eq]=%s", fooName)
    resp, err := client.Foos.ListFoos(ctx, kkOps.ListFoosRequest{Filter: &filter})
    if err != nil {
        return "", fmt.Errorf("failed to find foo: %w", err)
    }

    if len(resp.ListFoosResponse.Data) == 0 {
        return "", fmt.Errorf("foo not found: %s", fooName)
    }

    return resp.ListFoosResponse.Data[0].ID, nil
}

func listFooChildren(ctx context.Context, helper *cmd.Helper, client *kk.SDK, fooID string) error {
    resp, err := client.FooChildren.ListFooChildren(ctx, fooID)
    if err != nil {
        return fmt.Errorf("failed to list children: %w", err)
    }

    format, err := helper.GetOutputFormat()
    if err != nil {
        return err
    }

    switch format {
    case cmd.OutputFormatJSON:
        return helper.OutputJSON(resp.Data)
    case cmd.OutputFormatYAML:
        return helper.OutputYAML(resp.Data)
    default:
        return outputFooChildrenTable(helper, resp.Data)
    }
}

func getFooChild(ctx context.Context, helper *cmd.Helper, client *kk.SDK, fooID, childIDOrSlug string) error {
    // Try as ID first
    if isUUID(childIDOrSlug) {
        resp, err := client.FooChildren.GetFooChild(ctx, fooID, childIDOrSlug)
        if err == nil {
            return outputSingleFooChild(helper, resp)
        }
    }

    // List and filter by slug
    resp, err := client.FooChildren.ListFooChildren(ctx, fooID)
    if err != nil {
        return err
    }

    for _, child := range resp.Data {
        if child.Slug == childIDOrSlug {
            return outputSingleFooChild(helper, &child)
        }
    }

    return fmt.Errorf("child not found: %s", childIDOrSlug)
}

func outputFooChildrenTable(helper *cmd.Helper, children []kkComps.FooChild) error {
    // Similar to parent resource table output
    // ...
}

func outputSingleFooChild(helper *cmd.Helper, child *kkComps.FooChild) error {
    // Similar to parent resource detail output
    // ...
}
```

---

## KEY PATTERNS & CONVENTIONS

### RESOURCE IDENTIFICATION
- **Parent resources**: Identified by NAME (e.g., `api.Name`, `portal.Name`)
- **Child resources**: Identified by PARENT + MONIKER (e.g., `slug`, `version`)
- **Moniker types**: slug, name, version, username (varies by resource)

### LABEL MANAGEMENT
- **KONGCTL-NAMESPACE**: Applied to all parent resources
- **KONGCTL-PROTECTED**: Applied when `kongctl.protected: true`
- **User labels**: Preserved and compared separately from KONGCTL labels
- **Label removal**: Empty string signals API to remove label

### NAMESPACE HANDLING
- **Parent resources**: Must have namespace (explicit, file default, or implicit "default")
- **Child resources**: Inherit namespace from parent
- **Planner**: Processes each namespace independently
- **State client**: Filters by namespace when listing managed resources

### PROTECTION VALIDATION
- **Planner**: Collects all protection errors, fails fast before execution
- **Executor**: Double-checks protection before delete/update
- **Protection changes**: Allowed (can unprotect a resource)

### REFERENCE RESOLUTION

**Reference Syntax**: `!ref <resource-ref>#<field>`
- Uses YAML custom tag `!ref` to create cross-resource references
- Format: `!ref my-resource#id` or `!ref my-resource#name`
- The `#field` suffix is optional; defaults to `#id` if omitted
- Processed by loader before parsing into resource structures

**Resolution Phases**:
- **Planning time**: Resolve refs to existing resources (already in Konnect)
- **Execution time**: Resolve refs created in same execution (new resources)
- **Parent ID resolution**: Check References first, then Parent field

**Example**:
```yaml
portals:
  - ref: my-portal
    name: My Portal
    default_application_auth_strategy_id: !ref my-auth-strategy#id

auth_strategies:
  - ref: my-auth-strategy
    name: My Auth Strategy
```

**How it works**:
1. Loader processes `!ref` tags and replaces with placeholder: `__KONGCTL_REF__my-auth-strategy#id__`
2. Planner detects placeholders and creates Reference entries in PlannedChange
3. Executor resolves references to actual IDs before making SDK calls

### FIELD COMPARISON
- **Sparse updates**: Only include changed fields in update request
- **Nil handling**: Distinguish between "not set" and "empty string"
- **Normalization**: Normalize before comparison (e.g., JSON spec normalization)

### DEPENDENCY MANAGEMENT
- **Child → Parent**: Children depend on parents
- **Reference → Referenced**: Resources with !ref depend on referenced resources
- **Topological sort**: Ensures correct execution order

### ERROR HANDLING
- **State client**: Wrap errors with context (resource type, name, operation)
- **Enhanced errors**: Use ErrorWrapperOptions for rich error messages
- **Validation**: Validate early (resource definition, planning, execution)

### SDK TYPE MAPPING
- **Embed SDK types**: Use `yaml:",inline"` and `json:",inline"`
- **Pointer fields**: SDK uses pointers for optional fields
- **Label conversion**: Handle map[string]string ↔ map[string]*string
- **Enum types**: Use SDK enum constants (e.g., `kkComps.PublishedStatusPublished`)

---

## TESTING REQUIREMENTS

### UNIT TESTS
- Resource validation logic
- Field comparison logic
- Label building/conversion
- Reference resolution

### INTEGRATION TESTS
**When required**:
- New CLI commands/subcommands
- Authentication flow changes
- Configuration management changes
- API client modifications

**When unit tests sufficient**:
- Pure functions and utilities
- Configuration parsing
- Input validation
- String manipulation

---

## COMMON MISTAKES TO AVOID

1. **Forgetting to add resource to ResourceSet in types.go**
2. **Not implementing all Resource interface methods**
3. **Missing label normalization in state client**
4. **Incorrect parent ID resolution in child adapters**
5. **Not handling protection validation in planner**
6. **Forgetting to add planner call in GeneratePlan loop**
7. **Not converting SDK label types (map[string]*string)**
8. **Missing field in extractFields function**
9. **Not tracking created resources in executor**
10. **Forgetting to add get command to get.go**

---

## VERIFICATION CHECKLIST

After implementing new resource:

### Declarative Configuration
- [ ] Resource definition in `resources/` with all interface methods
- [ ] Resource type constant added to `types.go`
- [ ] ResourceSet includes new resource array
- [ ] State client has CRUD methods
- [ ] State client has ListManaged method with namespace filtering
- [ ] Planner implementation with CREATE/UPDATE/DELETE logic
- [ ] Planner added to GeneratePlan loop
- [ ] Executor adapter with MapCreateFields/MapUpdateFields
- [ ] Executor adapter handles parent ID resolution (if child)
- [ ] Executor change handler added to executeChange switch
- [ ] Labels properly converted between SDK and internal formats

### Imperative Get Command
- [ ] Get command entry point in `verbs/get/`
- [ ] Resource command in `products/konnect/<resource>/`
- [ ] GET handler with list/get by ID/name
- [ ] Child resource subcommands (if applicable)
- [ ] Output formatting (JSON, YAML, table, detail)
- [ ] Command added to get.go

### Testing
- [ ] Build succeeds: `make build`
- [ ] Linter passes: `make lint`
- [ ] Tests pass: `make test`
- [ ] Integration tests (if applicable): `make test-integration`
- [ ] Manual testing of declarative apply/plan/diff
- [ ] Manual testing of get commands

---

## EXAMPLE YAML CONFIGURATION

### Parent Resource
```yaml
foos:
  - ref: my-foo
    kongctl:
      namespace: production
      protected: true
    name: my-foo-name
    description: This is my foo
    labels:
      environment: prod
      team: platform
    children:
      - ref: child-1
        slug: getting-started
        content: "Welcome content"
```

### Child Resource (Root-Level)
```yaml
foo_children:
  - ref: child-2
    foo: my-foo  # Parent reference
    slug: advanced-guide
    content: "Advanced content"
```

### References
```yaml
foos:
  - ref: foo-a
    name: Foo A

foo_children:
  - ref: child-a
    foo: foo-a
    slug: page-1
    parent_child_ref: !ref another-child#id  # Reference to sibling
```

---

END OF IMPLEMENTATION GUIDE
