# Stage 1 Execution Plan: Detailed Steps

## Progress Summary

| Step | Description | Status | Dependencies |
|------|-------------|--------|--------------|
| 1 | Add Verb Constants | Completed | None |
| 2 | Create Command Stubs | Completed | Step 1 |
| 3 | Define Core Types | Completed | None |
| 4 | Define Portal Resource | Completed | Step 3 |
| 5 | Implement YAML Loader | Completed | Step 4 |
| 6 | Add Multi-file Support | Completed | Step 5 |
| 7 | Integrate with Plan Command | Not Started | Step 6 |

*See [process.md](process.md) for status definitions and development workflow.*

---

## Step 1: Add Verb Constants

### Status
Completed

### Dependencies
None

### Changes
- **File**: `internal/cmd/root/verbs/verbs.go`
- Add constants: `Plan`, `Sync`, `Diff`, `Export`

### Implementation
```go
const (
    // Existing verbs...
    Plan   = VerbValue("plan")
    Sync   = VerbValue("sync")
    Diff   = VerbValue("diff")
    Export = VerbValue("export")
)
```

### Tests
- None required (simple constants)

### Commit Message
```
feat(verbs): add declarative config verb constants

Actual commit: 11a2aa9 - Add Plan, Sync, Diff, and Export verb constants to support
declarative configuration commands
```

---

## Step 2: Create Command Stubs

### Status
Completed

### Dependencies
Step 1

### Changes
Create new files:
- `internal/cmd/root/verbs/plan/plan.go`
- `internal/cmd/root/verbs/sync/sync.go`
- `internal/cmd/root/verbs/diff/diff.go`
- `internal/cmd/root/verbs/export/export.go`
- Update `internal/cmd/root/cmd.go` to register new commands

### Implementation Example (plan.go)
```go
package plan

import (
    "context"
    "fmt"

    "github.com/kong/kongctl/internal/cmd/root/verbs"
    "github.com/kong/kongctl/internal/meta"
    "github.com/kong/kongctl/internal/util/i18n"
    "github.com/kong/kongctl/internal/util/normalizers"
    "github.com/spf13/cobra"
)

const (
    Verb = verbs.Plan
)

var (
    planUse = Verb.String()
    
    planShort = i18n.T("root.verbs.plan.planShort", 
        "Generate execution plan for declarative configuration")
    
    planLong = normalizers.LongDesc(i18n.T("root.verbs.plan.planLong",
        `Generate an execution plan showing what changes would be made to
        Konnect resources based on the provided declarative configuration files.`))
    
    planExamples = normalizers.Examples(i18n.T("root.verbs.plan.planExamples",
        fmt.Sprintf(`
        # Generate a plan from configuration directory
        %[1]s plan --dir ./konnect-config
        
        # Save plan to file
        %[1]s plan --dir ./config --output-file plan.json
        `, meta.CLIName)))
)

func NewPlanCmd() (*cobra.Command, error) {
    cmd := &cobra.Command{
        Use:     planUse,
        Short:   planShort,
        Long:    planLong,
        Example: planExamples,
        RunE: func(cmd *cobra.Command, args []string) error {
            return fmt.Errorf("plan command not yet implemented")
        },
        PersistentPreRun: func(cmd *cobra.Command, _ []string) {
            cmd.SetContext(context.WithValue(cmd.Context(), verbs.Verb, Verb))
        },
    }
    
    // Add flags
    cmd.Flags().StringP("dir", "d", ".", "Directory containing configuration files")
    cmd.Flags().String("output-file", "", "Save plan to file")
    
    return cmd, nil
}
```

### Tests
- Command registration test
- Help text validation

### Commit Message
```
feat(commands): add declarative config command stubs

Actual commit: 814dbd3 - Add plan, sync, diff, and export command stubs with
Konnect-first aliasing pattern where verb commands redirect to konnect subcommands
```

---

## Step 3: Define Core Types

### Status
Completed

### Dependencies
None

### Changes
- Create directory: `internal/declarative/resources/`
- Create file: `internal/declarative/resources/types.go`

### Implementation
```go
package resources

// ResourceSet contains all declarative resources from configuration files
type ResourceSet struct {
    Portals                   []PortalResource                    `yaml:"portals,omitempty"`
    ApplicationAuthStrategies []ApplicationAuthStrategyResource   `yaml:"application_auth_strategies,omitempty"`
    APIPublications          []APIPublicationResource            `yaml:"api_publications,omitempty"`
    APIImplementations       []APIImplementationResource         `yaml:"api_implementations,omitempty"`
    ControlPlanes            []ControlPlaneResource              `yaml:"control_planes,omitempty"`
    Services                 []ServiceResource                   `yaml:"services,omitempty"`
    // Additional resource types will be added as support is implemented
}

// KongctlMeta contains tool-specific metadata for resources
type KongctlMeta struct {
    // Protected prevents accidental deletion of critical resources
    Protected bool `yaml:"protected,omitempty"`
}

// ResourceValidator interface for common validation behavior
type ResourceValidator interface {
    Validate() error
}

// ReferencedResource interface for resources that can be referenced
type ReferencedResource interface {
    GetRef() string
}

// ReferenceMapping interface for resources that have reference fields
type ReferenceMapping interface {
    GetReferenceFieldMappings() map[string]string
}
```

### Tests
- None for initial types (interfaces and simple structs)

### Commit Message
```
feat(declarative): add core resource types

Actual commit: 73216dc - Add ResourceSet container, KongctlMeta for tool-specific metadata,
and common interfaces for resource validation and cross-references
```

---

## Step 4: Define Portal Resource

### Status
Completed

### Dependencies
Step 3

### Implementation Notes
- Completed portal, auth strategy, control plane, and API resource definitions
- Implemented API resource restructuring from cross-reference pattern to parent-child nesting
- API child resources (versions, publications, implementations) are now nested under API resources
- All resource types use proper SDK embedding (internal SDK for portals/APIs, public SDK for auth strategies/control planes)

### Changes
- Create file: `internal/declarative/resources/portal.go`

### Implementation
```go
package resources

import (
    "fmt"
    "github.com/Kong/sdk-konnect-go-internal/models/components"
)

// PortalResource represents a portal in declarative configuration
type PortalResource struct {
    // Embed SDK type for API fields
    components.CreatePortal `yaml:",inline"`
    
    // Reference identifier for cross-resource references
    Ref string `yaml:"ref"`
    
    // Tool-specific metadata
    Kongctl *KongctlMeta `yaml:"kongctl,omitempty"`
}

// GetRef returns the reference identifier used for cross-resource references
func (p PortalResource) GetRef() string {
    return p.Ref
}

// GetReferenceFieldMappings returns the field mappings for reference validation
func (p PortalResource) GetReferenceFieldMappings() map[string]string {
    return map[string]string{
        "default_application_auth_strategy_id": "application_auth_strategy",
    }
}

// Validate ensures the portal resource is valid
func (p PortalResource) Validate() error {
    if p.Ref == "" {
        return fmt.Errorf("portal ref is required")
    }
    
    // If API Name is not set, use ref as default
    if p.Name == "" {
        p.Name = p.Ref
    }
    
    return nil
}

// SetDefaults applies default values to portal resource
func (p *PortalResource) SetDefaults() {
    // If API Name is not set, use ref as default
    if p.Name == "" {
        p.Name = p.Ref
    }
}

// Example of additional resource types for reference handling

// ApplicationAuthStrategyResource represents an auth strategy in declarative configuration
type ApplicationAuthStrategyResource struct {
    // Embed SDK type
    components.CreateApplicationAuthStrategy `yaml:",inline"`
    
    // Reference identifier
    Ref string `yaml:"ref"`
    
    // Tool-specific metadata
    Kongctl *KongctlMeta `yaml:"kongctl,omitempty"`
}

func (a ApplicationAuthStrategyResource) GetRef() string {
    return a.Ref
}

// GetReferenceFieldMappings returns empty map as auth strategies don't reference other resources
func (a ApplicationAuthStrategyResource) GetReferenceFieldMappings() map[string]string {
    return map[string]string{} // No outbound references
}

// APIPublicationResource demonstrates complex reference patterns
type APIPublicationResource struct {
    // Embed SDK type
    components.APIPublication `yaml:",inline"`
    
    // Reference identifier
    Ref string `yaml:"ref"`
    
    // Tool-specific metadata
    Kongctl *KongctlMeta `yaml:"kongctl,omitempty"`
}

func (a APIPublicationResource) GetRef() string {
    return a.Ref
}

// GetReferenceFieldMappings defines which fields reference other resources
func (a APIPublicationResource) GetReferenceFieldMappings() map[string]string {
    return map[string]string{
        "portal_id":         "portal",
        "api_id":           "api",
        "auth_strategy_ids": "application_auth_strategy",
    }
}

// Add API Implementation example showing qualified field names
type APIImplementationResource struct {
    components.APIImplementation `yaml:",inline"`
    Ref string `yaml:"ref"`
    Kongctl *KongctlMeta `yaml:"kongctl,omitempty"`
}

func (a APIImplementationResource) GetRef() string {
    return a.Ref
}

// GetReferenceFieldMappings uses qualified field names for nested references
func (a APIImplementationResource) GetReferenceFieldMappings() map[string]string {
    return map[string]string{
        "service.control_plane_id": "control_plane",  // Qualified field name
        "service.id":               "service",         // Context is clear
    }
}
```

### Tests
- Validation tests (missing ref, etc.)
- Default value tests
- Ref field handling tests

### Commit Message
```
feat(declarative): add portal resource definitions

Actual commits: 
- dc4301a: Add portal, auth strategy, control plane, and API resource definitions
- 0f5629b: Restructure API resources with parent-child relationships
```

---

## Step 5: Implement YAML Loader

### Status
Completed

### Dependencies
Step 4

### Changes
- Create directory: `internal/declarative/loader/`
- Create file: `internal/declarative/loader/loader.go`

### Implementation
```go
package loader

import (
    "fmt"
    "io"
    "os"
    "path/filepath"
    
    "github.com/kong/kongctl/internal/declarative/resources"
    "gopkg.in/yaml.v3"
)

// Loader handles loading declarative configuration from files
type Loader struct {
    rootPath string
}

// New creates a new configuration loader
func New(rootPath string) *Loader {
    return &Loader{
        rootPath: rootPath,
    }
}

// LoadFile loads configuration from a single YAML file
func (l *Loader) LoadFile(path string) (*resources.ResourceSet, error) {
    file, err := os.Open(path)
    if err != nil {
        return nil, fmt.Errorf("failed to open file %s: %w", path, err)
    }
    defer file.Close()
    
    return l.parseYAML(file)
}

// parseYAML parses YAML content into ResourceSet
func (l *Loader) parseYAML(r io.Reader) (*resources.ResourceSet, error) {
    var rs resources.ResourceSet
    
    decoder := yaml.NewDecoder(r)
    if err := decoder.Decode(&rs); err != nil {
        return nil, fmt.Errorf("failed to parse YAML: %w", err)
    }
    
    // Validate all resources
    if err := l.validateResourceSet(&rs); err != nil {
        return nil, err
    }
    
    return &rs, nil
}

// validateResourceSet validates all resources and checks for ref uniqueness
func (l *Loader) validateResourceSet(rs *resources.ResourceSet) error {
    // Build registry of all resources by type for reference validation
    resourceRegistry := make(map[string]map[string]bool)
    
    // Check portal ref uniqueness and build registry
    portalRefs := make(map[string]bool)
    resourceRegistry["portal"] = portalRefs
    for i := range rs.Portals {
        portal := &rs.Portals[i]
        
        // Apply defaults
        portal.SetDefaults()
        
        // Validate
        if err := portal.Validate(); err != nil {
            return fmt.Errorf("invalid portal %q: %w", portal.GetRef(), err)
        }
        
        // Check uniqueness
        if portalRefs[portal.GetRef()] {
            return fmt.Errorf("duplicate portal ref: %s", portal.GetRef())
        }
        portalRefs[portal.GetRef()] = true
    }
    
    // Check auth strategy ref uniqueness and build registry
    authStrategyRefs := make(map[string]bool)
    resourceRegistry["application_auth_strategy"] = authStrategyRefs
    for i := range rs.ApplicationAuthStrategies {
        strategy := &rs.ApplicationAuthStrategies[i]
        
        // Apply defaults
        strategy.SetDefaults()
        
        // Validate
        if err := strategy.Validate(); err != nil {
            return fmt.Errorf("invalid application_auth_strategy %q: %w", strategy.GetRef(), err)
        }
        
        // Check uniqueness
        if authStrategyRefs[strategy.GetRef()] {
            return fmt.Errorf("duplicate application_auth_strategy ref: %s", strategy.GetRef())
        }
        authStrategyRefs[strategy.GetRef()] = true
    }
    
    // Validate cross-resource references
    if err := l.validateReferences(rs, resourceRegistry); err != nil {
        return err
    }
    
    return nil
}

// validateReferences validates that all cross-resource references are valid using per-resource mappings
func (l *Loader) validateReferences(rs *resources.ResourceSet, registry map[string]map[string]bool) error {
    // Validate all resource types that implement ReferenceMapping
    resourceSlices := []interface{}{
        rs.Portals,
        rs.ApplicationAuthStrategies, 
        rs.APIPublications,
        rs.APIImplementations,
        rs.ControlPlanes,
        rs.Services,
    }
    
    for _, slice := range resourceSlices {
        if err := l.validateResourceSlice(slice, registry); err != nil {
            return err
        }
    }
    
    return nil
}

// validateResourceSlice validates references for a slice of resources
func (l *Loader) validateResourceSlice(slice interface{}, registry map[string]map[string]bool) error {
    switch resources := slice.(type) {
    case []resources.PortalResource:
        for _, resource := range resources {
            if err := l.validateResourceReferences(resource, registry); err != nil {
                return err
            }
        }
    case []resources.APIPublicationResource:
        for _, resource := range resources {
            if err := l.validateResourceReferences(resource, registry); err != nil {
                return err
            }
        }
    case []resources.APIImplementationResource:
        for _, resource := range resources {
            if err := l.validateResourceReferences(resource, registry); err != nil {
                return err
            }
        }
    // Add cases for other resource types as needed
    }
    return nil
}

// validateResourceReferences validates references for a single resource using its mapping
func (l *Loader) validateResourceReferences(resource resources.ReferenceMapping, registry map[string]map[string]bool) error {
    mappings := resource.GetReferenceFieldMappings()
    
    for fieldName, expectedType := range mappings {
        fieldValue := l.getFieldValue(resource, fieldName)
        if fieldValue != "" && !registry[expectedType][fieldValue] {
            return fmt.Errorf("resource %q references unknown %s: %s", 
                resource.(resources.ReferencedResource).GetRef(), expectedType, fieldValue)
        }
    }
    return nil
}

// getFieldValue extracts field value using reflection, supporting qualified field names like "service.id"
func (l *Loader) getFieldValue(resource interface{}, fieldName string) string {
    // Implementation would use reflection to extract field values
    // Supporting both simple fields ("portal_id") and qualified fields ("service.id")
    // This is a placeholder - actual implementation would handle reflection
    return ""
}
```

### Tests
- Valid YAML parsing
- Invalid YAML error handling
- Ref uniqueness validation
- Missing required fields
- File not found errors

### Test Example
```go
func TestLoaderLoadFile(t *testing.T) {
    tests := []struct {
        name    string
        yaml    string
        wantErr bool
        errMsg  string
    }{
        {
            name: "valid portal",
            yaml: `
portals:
  - ref: test-portal
    name: "Test Portal"
    description: "A test portal"
`,
            wantErr: false,
        },
        {
            name: "missing ref",
            yaml: `
portals:
  - name: "Test Portal"
`,
            wantErr: true,
            errMsg:  "portal ref is required",
        },
        {
            name: "duplicate refs",
            yaml: `
portals:
  - ref: portal1
    name: "Portal 1"
  - ref: portal1
    name: "Portal 2"
`,
            wantErr: true,
            errMsg:  "duplicate portal ref: portal1",
        },
        {
            name: "valid portal with auth strategy reference",
            yaml: `
application_auth_strategies:
  - ref: oauth-strategy
    name: "OAuth Strategy"
    auth_type: openid_connect

portals:
  - ref: test-portal
    name: "Test Portal"
    default_application_auth_strategy_id: oauth-strategy
`,
            wantErr: false,
        },
        {
            name: "invalid auth strategy reference",
            yaml: `
portals:
  - ref: test-portal
    name: "Test Portal"
    default_application_auth_strategy_id: nonexistent-strategy
`,
            wantErr: true,
            errMsg:  "references unknown application_auth_strategy: nonexistent-strategy",
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Create temp file
            tmpfile, err := os.CreateTemp("", "test-*.yaml")
            require.NoError(t, err)
            defer os.Remove(tmpfile.Name())
            
            _, err = tmpfile.Write([]byte(tt.yaml))
            require.NoError(t, err)
            tmpfile.Close()
            
            // Test loading
            loader := New(".")
            _, err = loader.LoadFile(tmpfile.Name())
            
            if tt.wantErr {
                assert.Error(t, err)
                assert.Contains(t, err.Error(), tt.errMsg)
            } else {
                assert.NoError(t, err)
            }
        })
    }
}
```

### Commit Message
```
feat(loader): implement YAML configuration loader

Add loader package that can parse YAML files into ResourceSet,
validate resources, and ensure name uniqueness

Actual commits:
- 4623a0f: feat(loader): implement YAML loader with validation (WIP)
- 1a8150d: fix(loader): fix loop variable issue in cross-reference validation
- 980985f: fix(lint): fix variable naming for AuthStrategyIDs field
```

---

## Step 6: Add Multi-file Support

### Status
Completed

### Dependencies
Step 5

### Changes
- Extend `internal/declarative/loader/loader.go`

### Implementation
```go
// Load loads all YAML files from the root path
func (l *Loader) Load() (*resources.ResourceSet, error) {
    info, err := os.Stat(l.rootPath)
    if err != nil {
        return nil, fmt.Errorf("failed to stat path %s: %w", l.rootPath, err)
    }
    
    if info.IsDir() {
        return l.loadDirectory()
    }
    
    return l.LoadFile(l.rootPath)
}

// loadDirectory loads all YAML files from a directory
func (l *Loader) loadDirectory() (*resources.ResourceSet, error) {
    var allResources resources.ResourceSet
    
    err := filepath.Walk(l.rootPath, func(path string, info os.FileInfo, err error) error {
        if err != nil {
            return err
        }
        
        // Skip directories
        if info.IsDir() {
            return nil
        }
        
        // Only process .yaml and .yml files
        ext := filepath.Ext(path)
        if ext != ".yaml" && ext != ".yml" {
            return nil
        }
        
        // Load file
        rs, err := l.LoadFile(path)
        if err != nil {
            return fmt.Errorf("failed to load %s: %w", path, err)
        }
        
        // Merge resources
        allResources.Portals = append(allResources.Portals, rs.Portals...)
        
        return nil
    })
    
    if err != nil {
        return nil, err
    }
    
    // Validate merged resources
    if err := l.validateResourceSet(&allResources); err != nil {
        return nil, err
    }
    
    return &allResources, nil
}
```

### Tests
- Directory with multiple files
- Mixed .yaml and .yml extensions
- Non-YAML files ignored
- Subdirectory traversal
- Name conflicts across files

### Commit Message
```
feat(loader): add multi-file configuration support

Extend loader to handle directories, merge resources from multiple
YAML files, and validate the combined result

Actual commit: d6472a4
```

---

## Step 7: Integrate with Plan Command

### Status
Not Started

### Dependencies
Step 6

### Changes
- Update `internal/cmd/root/verbs/plan/plan.go`

### Implementation
```go
func NewPlanCmd() (*cobra.Command, error) {
    cmd := &cobra.Command{
        Use:     planUse,
        Short:   planShort,
        Long:    planLong,
        Example: planExamples,
        RunE:    runPlan,
        PersistentPreRun: func(cmd *cobra.Command, _ []string) {
            cmd.SetContext(context.WithValue(cmd.Context(), verbs.Verb, Verb))
        },
    }
    
    // Add flags
    cmd.Flags().StringP("dir", "d", ".", "Directory containing configuration files")
    cmd.Flags().String("output-file", "", "Save plan to file")
    
    return cmd, nil
}

func runPlan(cmd *cobra.Command, args []string) error {
    dir, _ := cmd.Flags().GetString("dir")
    
    // Load configuration
    loader := loader.New(dir)
    resourceSet, err := loader.Load()
    if err != nil {
        return fmt.Errorf("failed to load configuration: %w", err)
    }
    
    // Display summary
    fmt.Fprintln(cmd.OutOrStdout(), "Configuration loaded successfully:")
    
    if len(resourceSet.Portals) > 0 {
        fmt.Fprintf(cmd.OutOrStdout(), "- %d portal(s) found:", len(resourceSet.Portals))
        for _, portal := range resourceSet.Portals {
            fmt.Fprintf(cmd.OutOrStdout(), " %q", portal.GetRef())
        }
        fmt.Fprintln(cmd.OutOrStdout())
    }
    
    // TODO: Generate actual plan in Stage 2
    fmt.Fprintln(cmd.OutOrStdout(), "\nPlan generation not yet implemented")
    
    return nil
}
```

### Tests
- Integration test with valid configuration
- Error handling for invalid configurations
- Output formatting

### Commit Message
```
feat(plan): integrate configuration loader with plan command

Connect plan command to loader, display summary of loaded resources,
and prepare for plan generation in Stage 2
```