# Kongctl Project Status Report - January 23, 2025

## Executive Summary

`kongctl` is a tech preview CLI tool for Kong and Kong Konnect with a limited but growing feature 
set including both imperative commands and declarative configuration management. The project 
has completed recent development implement declarative configuration for limited resource types 
with ongoing work focusing on hardening, testing, extending supported resources, and adding
support for adding external references to declarative configurations (external references are resources
not declared in the configuration).

## Technical Summary

The CLI is built with Go using the Cobra framework, providing dual-mode operation through 
traditional imperative commands (`get`, `create`, `delete`) and GitOps-ready declarative 
configuration. The declarative system uses YAML with advanced features including Yaml Tag functions
(`!file` tags), value extraction from OpenAPI specs, and label-based resource lifecycle 
management. Operations follow a strict safety model: non-destructive `apply` commands handle 
CREATE/UPDATE only, while `sync` provides full state reconciliation including DELETE operations. 
The architecture emphasizes type safety through SDK type embedding, compile-time validation, and 
idempotent configuration-based change detection.

## Konnect-First Design Approach

The CLI follows a "Konnect-first" approach where Konnect is the implied default product:
- **Declarative**: `kongctl apply` defers to `kongctl apply konnect`
- **Imperative** (future): `kongctl get gateway control-planes` will defer to 
  `kongctl get konnect gateway control-planes`
- **Authentication**: `kongctl login` will defer to `kongctl login konnect` in next iteration

## Current Capabilities

### Imperative Operations
- **Authentication**: `kongctl login` with OAuth device flow, PAT support
- **Resource Retrieval**: `get`, `list` for control planes, services, routes
- **Resource Management**: `create`, `delete` operations
- **Data Operations**: `dump` objects to Terraform import format
- **Utilities**: `version`, `completion` for shell autocomplete

### Declarative Operations

**Supported Resource Types:**
- **Portals**: Developer portals with themes, authentication, RBAC
- **APIs**: Full lifecycle with versions, publications, implementations, documents
- **Auth Strategies**: OAuth2/OIDC, API Key authentication
- **Control Planes**: Defined in YAML but not yet operational (no planner implementation)

**Core Commands:**
```bash
# Generate execution plan (preview changes)
kongctl plan -f portal.yaml -f api.yaml --output-file plan.json

# Apply configuration (CREATE/UPDATE only)
kongctl apply -f ./config -R --auto-approve

# Full synchronization (includes DELETE)
kongctl sync -f ./config --dry-run

# Show differences
kongctl diff -f ./config
```

**YAML Configuration Example:**
```yaml
apis:
  - ref: banking-api
    name: !file ./specs/openapi.yaml#info.title
    versions:
      - ref: v1
        spec: !file ./specs/openapi.yaml
    publications:
      - ref: prod-pub
        portal_id: developer-portal
        auth_strategy_ids: [oauth-strategy]
    documents:
      - ref: quickstart
        content: !file ./docs/quickstart.md
```

## Technical Architecture

### Core Design Decisions

1. **Type-Specific ResourceSet**: Explicit fields for each resource type providing compile-time 
   safety and IDE support over generic collections

2. **SDK Type Embedding**: Direct embedding of Kong SDK types with YAML inline tags, avoiding 
   duplication and ensuring API compatibility

3. **Three-Tier Identity System**:
   - **SDK ID**: UUID from Konnect (e.g., `a1b2c3d4-e5f6-7890-abcd-ef1234567890`)
   - **SDK Name**: Human-friendly identifier (e.g., `developer-portal-prod`)
   - **Ref field**: Config-only identifier for cross-references (e.g., `main-portal`)

4. **Configuration Change Detection**: Direct field-by-field comparison for detecting changes
   (Note: Config hashing was planned but not implemented)

5. **Reference Resolution**: Validation and resolution at plan time with `<unknown>` placeholders 
   for resources being created

### Safety Features

- **Protected Resources**: `kongctl.protected: true` label prevents accidental deletion
- **Managed Resource Tracking**: `KONGCTL-managed` label identifies kongctl-managed resources
- **Mode-Aware Operations**: Separate `apply` (safe) vs `sync` (with deletions) commands
- **Fail-Fast Validation**: Early detection of invalid references and protected resource 
  modifications
- **Confirmation Requirements**: Explicit "yes" confirmation for destructive operations

### File Handling

- **kubectl-style Loading**: `-f` flag supporting files, directories, stdin, CSV lists
- **Recursive Processing**: `-R` flag for directory traversal
- **Security Constraints**: No parent directory traversal, relative paths only
- **Multiple Sources**: Combine multiple files/directories in single operation

## Current Limitations

1. **No Server-Side Label Filtering**: SDK limitation requires client-side filtering of resources
2. **Konnect-Only**: Gateway (on-premise) support planned but not implemented
3. **Basic YAML Tag Features**: Dot notation for value extraction (e.g., `!file spec.yaml#info.title`), 
   no JSONPath support
4. **Manual Discovery**: Users must know which fields to configure (no helpful / automatic discovery)
5. **Environment-Bound Plans**: Resource IDs are resolved at plan time, making plans specific to 
   the environment where they were generated
6. **Limited Resource Support**: Only portals, APIs, and auth strategies; plugins, consumers, 
   certificates pending
7. **Export Not Implemented**: Cannot export current state to declarative format (`export` command 
   returns error)

## Technical Debt

### High Priority
- **Dual ID Support**: Both refs and UUIDs accepted for control_plane_id (ADR-001-010)
- **Type-Specific Adapters**: Boilerplate code for each resource type (ADR-004-010)
- **Configuration Discovery**: Implementation details deferred (ADR-003-012)
- **Control Plane Support**: Resource defined but planner not implemented

### Medium Priority
- **Generic Resource Interface**: Current type-specific approach limits extensibility
- **Plan Portability**: Environment-specific IDs prevent plan reuse
- **Error Message Consistency**: Varying quality across operations
- **Config Hash Implementation**: Planned but not implemented (ADR-002-002)

### Low Priority
- **Performance Optimization**: Client-side filtering inefficiencies
- **Test Coverage**: Integration tests need expansion
- **Documentation**: API documentation generation incomplete

## Next Steps

### 1. **Production Hardening**
- Comprehensive error handling improvements
- Performance optimization for large configurations
- Retry logic and timeout handling
- Stability testing with real-world scenarios

### 2. **Testing Enhancement**
- Expand integration test coverage for all resource types
- Add stress testing for large deployments
- Implement end-to-end workflow testing
- Manual testing of control plane support

### 3. **Resource Support Extension**
- **Declarative**: Complete control plane planner implementation
- **Declarative**: Add plugins, consumers, certificates
- **Imperative**: Extend command coverage for more resource types
- **Both**: Gateway (on-premise) support

### 4. **Feature Implementation**
- Export command for declarative configuration
- Configuration discovery mode
- Advanced YAML tag sources (environment variables, vaults)
- Konnect-first command restructuring

### 5. **Developer Experience**
- Code generation for type-specific adapters
- Improved error messages with remediation hints
- Progress indicators for long operations
- Better documentation and examples

## Command Reference

```bash
# Imperative Commands
kongctl get konnect gateway control-planes
kongctl create konnect portal --name "Developer Portal"
kongctl delete konnect api <api-id>

# Declarative Workflow
kongctl plan -f ./config --mode sync        # Preview all changes
kongctl apply -f ./config --dry-run         # Safe preview
kongctl sync -f ./config --auto-approve     # Full reconciliation

# Authentication
kongctl login                               # Interactive OAuth flow
kongctl login --pat $KONG_PAT              # Token authentication

# Output Formats
kongctl get konnect apis -o json           # JSON output
kongctl plan -f ./config -o yaml           # YAML output
```
