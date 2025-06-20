# Stage 1 Execution Plan: Detailed Steps

## Step 1: Add Verb Constants

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

Add Plan, Sync, Diff, and Export verb constants to support
declarative configuration commands
```

---

## Step 2: Create Command Stubs

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

Add plan, apply, sync, diff, and export command stubs that
return "not yet implemented" errors. Commands are properly
registered with appropriate help text and flags.
```

---

## Step 3: Define Core Types

### Changes
- Create directory: `internal/declarative/resources/`
- Create file: `internal/declarative/resources/types.go`

### Implementation
```go
package resources

// ResourceSet contains all declarative resources from configuration files
type ResourceSet struct {
    Portals []PortalResource `yaml:"portals,omitempty"`
    // Future resource types will be added here
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

// NamedResource interface for resources that have a declarative name
type NamedResource interface {
    GetName() string
}
```

### Tests
- None for initial types (interfaces and simple structs)

### Commit Message
```
feat(declarative): add core resource types

Add ResourceSet container, KongctlMeta for tool-specific metadata,
and common interfaces for resource validation and naming
```

---

## Step 4: Define Portal Resource

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
    
    // Declarative name for references (distinct from API Name field)
    DeclarativeName string `yaml:"name"`
    
    // Tool-specific metadata
    Kongctl *KongctlMeta `yaml:"kongctl,omitempty"`
}

// GetName returns the declarative name used for references
func (p PortalResource) GetName() string {
    return p.DeclarativeName
}

// Validate ensures the portal resource is valid
func (p PortalResource) Validate() error {
    if p.DeclarativeName == "" {
        return fmt.Errorf("portal name is required")
    }
    
    // If API Name is not set, use declarative name
    if p.Name == "" {
        p.Name = p.DeclarativeName
    }
    
    return nil
}

// SetDefaults applies default values to portal resource
func (p *PortalResource) SetDefaults() {
    // If API Name is not set, use declarative name
    if p.Name == "" {
        p.Name = p.DeclarativeName
    }
}
```

### Tests
- Validation tests (missing name, etc.)
- Default value tests
- Name field handling tests

### Commit Message
```
feat(declarative): add portal resource definition

Add PortalResource wrapper type that embeds SDK's CreatePortal
and adds declarative-specific fields for name references and
kongctl metadata
```

---

## Step 5: Implement YAML Loader

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

// validateResourceSet validates all resources and checks for name uniqueness
func (l *Loader) validateResourceSet(rs *resources.ResourceSet) error {
    // Check portal name uniqueness
    portalNames := make(map[string]bool)
    for i := range rs.Portals {
        portal := &rs.Portals[i]
        
        // Apply defaults
        portal.SetDefaults()
        
        // Validate
        if err := portal.Validate(); err != nil {
            return fmt.Errorf("invalid portal %q: %w", portal.GetName(), err)
        }
        
        // Check uniqueness
        if portalNames[portal.GetName()] {
            return fmt.Errorf("duplicate portal name: %s", portal.GetName())
        }
        portalNames[portal.GetName()] = true
    }
    
    return nil
}
```

### Tests
- Valid YAML parsing
- Invalid YAML error handling
- Name uniqueness validation
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
  - name: test-portal
    display_name: "Test Portal"
    description: "A test portal"
`,
            wantErr: false,
        },
        {
            name: "missing name",
            yaml: `
portals:
  - display_name: "Test Portal"
`,
            wantErr: true,
            errMsg:  "portal name is required",
        },
        {
            name: "duplicate names",
            yaml: `
portals:
  - name: portal1
    display_name: "Portal 1"
  - name: portal1
    display_name: "Portal 2"
`,
            wantErr: true,
            errMsg:  "duplicate portal name: portal1",
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
```

---

## Step 6: Add Multi-file Support

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
```

---

## Step 7: Integrate with Plan Command

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
            fmt.Fprintf(cmd.OutOrStdout(), " %q", portal.GetName())
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