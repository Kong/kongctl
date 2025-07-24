# Stage 6: Namespace-Based Resource Management - Technical Overview

## Architecture Overview

The namespace feature builds on the existing label system to provide resource 
isolation. Each top-level resource is tagged with a `KONGCTL-namespace` label 
that determines which namespace it belongs to. Commands operate only on 
resources within the namespaces found in the configuration files.

## Key Design Decisions

### 1. Namespace as a Label

Following the pattern established by `kongctl.protected`, namespaces are 
implemented as labels:
- Configuration: `kongctl.namespace: team-alpha`
- Label: `KONGCTL-namespace: team-alpha`
- Consistent with existing label management

### 2. Default Namespace with File Defaults

- Implicit "default" namespace when not specified
- File-level defaults via `_defaults.kongctl.namespace`
- Minimal configurations work without explicit namespace

### 3. Parent-Only Labeling

Due to Konnect API limitations:
- Only top-level resources (APIs, Portals, Auth Strategies) have labels
- Child resources are managed based on their parent's namespace
- Planning and execution handle parent-child relationships

### 4. Multi-Namespace Operations

- Single command can process multiple namespaces
- Each namespace is planned independently
- No cross-namespace interference during sync

## Implementation Approach

### Phase 1: Core Infrastructure

1. **Extend KongctlMeta struct** to include namespace field
2. **Add _defaults parsing** to configuration loader
3. **Implement namespace defaulting** to "default" during loading

### Phase 2: Planning Integration

1. **Update planners** to handle namespace field
2. **Group resources by namespace** before planning
3. **Enhance state client** with namespace filtering

### Phase 3: Execution Updates

1. **Convert namespace to label** during resource operations
2. **Filter managed resources** by namespace in state client
3. **Update command output** to show namespace operations

### Phase 4: Testing and Documentation

1. **Comprehensive tests** for multi-namespace scenarios
2. **Update examples** with namespace usage
3. **Document limitations** and best practices

## Technical Components

### Configuration Types

```go
// Add to KongctlMeta in types.go
type KongctlMeta struct {
    Protected bool   `yaml:"protected,omitempty"`
    Namespace string `yaml:"namespace,omitempty"`
}

// Add to ResourceSet for file defaults
type ResourceSet struct {
    Defaults *DefaultsConfig `yaml:"_defaults,omitempty"`
    // ... existing fields
}

type DefaultsConfig struct {
    Kongctl *KongctlDefaults `yaml:"kongctl,omitempty"`
}

type KongctlDefaults struct {
    Namespace string `yaml:"namespace,omitempty"`
}
```

### Label Management

**Important**: To stay within Konnect's 5-label limit, we're removing the 
`KONGCTL-managed` and `KONGCTL-last-updated` labels. The namespace label will 
serve as both ownership and management indicator.

- Add `NamespaceKey = "KONGCTL-namespace"` constant
- Remove deprecated `ManagedKey` and `LastUpdatedKey` constants
- Update `BuildCreateLabels` to only add namespace and protected labels
- Replace managed resource checks with namespace presence

### Planning Changes

- Add namespace field to PlannedChange struct
- Group resources by namespace before planning
- Process each namespace independently

### State Client Updates

```go
// Resources are managed if they have a namespace label
func (c *StateClient) ListManagedAPIs(namespaces []string) ([]*API, error) {
    // Filter by KONGCTL-namespace IN namespaces
    // Any resource with namespace label is considered managed
}
```

## Error Handling

### Validation Errors
- Conflicting namespace in parent-child relationships
- Invalid namespace values
- Namespace change attempts on existing resources

### Runtime Errors
- Resources found without expected namespace labels
- Namespace mismatch during updates
- Cross-namespace reference attempts

## Performance Considerations

1. **Efficient Filtering**: Use server-side filtering where possible
2. **Batch Operations**: Group API calls by namespace
3. **Caching**: Cache namespace lookups within planning cycle

## Security Considerations

1. **No Cross-Namespace Access**: Strict isolation enforcement
2. **Clear Audit Trail**: Namespace visible in all operations
3. **Validation**: Prevent namespace changes on existing resources

## Future Extensibility

The `_defaults` structure allows for future enhancements:
- Additional default values
- Namespace-specific settings
- Policy configurations