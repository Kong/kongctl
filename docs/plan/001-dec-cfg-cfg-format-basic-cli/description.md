# KongCtl Stage 1 - Configuration Format & Basic CLI

## Goal
Establish the YAML configuration format and integrate basic commands into kongctl.

## Deliverables
- Define YAML structure using SDK types directly (no duplicate structs)
- Add command stubs: `kongctl plan`, `kongctl apply`, `kongctl sync`, `kongctl diff`, `kongctl export`
- Implement configuration loading from single/multiple YAML files
- Basic validation for portal resources

## Implementation Details

### Command Integration
Add new verb commands following the existing pattern:
```
internal/cmd/root/verbs/
  ├── plan/
  │   └── plan.go
  ├── apply/
  │   └── apply.go
  ├── sync/
  │   └── sync.go
  ├── diff/
  │   └── diff.go
  └── export/
      └── export.go
```

Each command should register with the root command and display appropriate help text. Commands should return "not yet implemented" for now.

### Suggested Configuration Format
```yaml
# portals.yaml
portals:
  - name: developer-portal
    display_name: "Kong Developer Portal"
    description: "Main developer portal for Kong APIs"
    auto_approve_developers: false
    auto_approve_applications: true
    labels:
      team: platform
      environment: production
    kongctl:
      protected: true
```

### Suggested Resource Type Definition
```go
// internal/declarative/resources/portal.go
type PortalResource struct {
    components.CreatePortal `yaml:",inline"`
    Name string `yaml:"name"` // Required for identity
    Kongctl *KongctlMeta `yaml:"kongctl,omitempty"`
}

type KongctlMeta struct {
    Protected bool `yaml:"protected,omitempty"`
}

// Suggested root structure (avoid "Config" to prevent confusion with CLI config)
type ResourceSet struct {
    Portals []PortalResource `yaml:"portals,omitempty"`
}
```

### Suggested Configuration Loader
```go
// internal/declarative/config/loader.go
type Loader struct {
    rootPath string
}

func (l *Loader) Load() (*ResourceSet, error) {
    // 1. Find all .yaml and .yml files
    // 2. Parse each file
    // 3. Merge configurations
    // 4. Validate required fields
    return resourceSet, nil
}
```

## Tests Required
- Configuration parsing with valid/invalid YAML
- Multi-file configuration merging
- Name uniqueness validation
- Command registration and help text

## Proof of Success
```bash
# Load and validate a portal configuration
$ kongctl plan --dir ./portals
Configuration loaded successfully:
- 1 portal found: "developer-portal"

# Show command stubs are working
$ kongctl apply
Error: apply command not yet implemented

$ kongctl export
Error: export command not yet implemented
```

## Dependencies
- Kong SDK for Go (already in go.mod)
- Existing kongctl command framework
- YAML parsing library (gopkg.in/yaml.v3)

## Notes
- Use SDK types directly via struct embedding to avoid duplication
- Follow existing kongctl patterns for command structure
- Configuration format should be extensible for future resource types
- The `kongctl` section in YAML is optional and tool-specific
- Avoid using "Config" for declarative structures to prevent confusion with existing CLI config system that handles runtime values from files/env/flags