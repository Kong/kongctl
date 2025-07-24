# KongCtl Declarative Configuration - Design Brief

## Overview

This brief outlines the design for declarative configuration management in `kongctl` for Kong Konnect resources. The feature provides a plan-based workflow for managing Konnect resources through YAML configuration files, similar to infrastructure-as-code tools but simplified for Konnect's specific needs.

## Core Design Principles

### State-Free Management
Unlike traditional IaC tools, kongctl operates without maintaining local state files. Instead:
- Current state is queried directly from Konnect APIs
- Resource ownership tracked via Konnect labels
- Configuration drift detected through hash comparison

### Plan-Based Operations
All changes follow a plan/review/execute workflow:
1. Analyze differences between desired (YAML) and current (API) state
2. Generate an execution plan showing exact changes
3. Review plan in human or machine-readable format
4. Execute changes with safety controls

### Resource Identity
- Resources identified by user-defined names, not UUIDs
- Names must be unique within resource type
- Enables portable configurations across environments
- Server-assigned IDs tracked internally but not exposed

## Command Interface

### Core Commands

```bash
kongctl plan                    # Generate execution plan
kongctl diff                    # Display pending changes
kongctl apply                   # Execute changes (create/update only)
kongctl sync                    # Full reconciliation (includes deletions)
kongctl export                  # Export existing resources to YAML
```

### Command Options

- `--dir <path>` - Specify configuration directory
- `--output-file <file>` - Save plan to file
- `--plan <file>` - Use existing plan file
- `--output <format>` - Output format (human/json/yaml)
- `--dry-run` - Preview without making changes
- `--auto-approve` - Skip confirmation prompts

## Configuration Format

### YAML Structure

```yaml
# Top-level resources that can be referenced
application_auth_strategies:
  - name: key-auth-strategy
    display_name: "API Key Authentication"
    auth_type: key_auth
    configs:
      key_names: ["api_key", "x-api-key"]
    labels:
      team: platform

  - name: oidc-strategy
    display_name: "OpenID Connect"
    auth_type: openid_connect
    configs:
      issuer: "https://auth.example.com"
      scopes: ["openid", "profile"]

# Resources with nested children
portals:
  - name: developer-portal
    display_name: "Kong Developer Portal"
    description: "Main API portal"
    auto_approve_developers: false
    auto_approve_applications: true
    # Reference by name - resolved to ID at execution time
    default_application_auth_strategy: key-auth-strategy
    labels:
      department: engineering
      cost-center: eng-001
    kongctl:
      protected: true       # Prevents accidental deletion
      namespace: platform   # Resource ownership for multi-team environments
    
    # Nested resources for parent-child relationships
    pages:
      - name: getting-started
        slug: /getting-started
        title: "Getting Started Guide"
        content: |
          # Welcome
          Documentation content here...
        visibility: public
        status: published
      
      - name: api-reference
        slug: /api-reference
        title: "API Reference"
        content: "Full API documentation"
    
    specs:
      - name: users-api-v1
        spec_file: ./openapi/users-v1.yaml
        title: "Users API v1"
        description: "User management endpoints"

# Other top-level resources
teams:
  - name: platform-team
    description: "Platform engineering team"
    labels:
      department: engineering
```

### Key Characteristics
- Simple YAML format (no DSL or HCL)
- Parent-child resources are nested (following API structure)
- Independent resources reference each other by name (not UUID)
- Names are resolved to IDs during plan generation
- Optional `kongctl` section for tool-specific settings (parent resources only)
- Labels support user metadata and tool tracking
- Child resources inherit namespace from their parent

### Name Resolution
References between resources use human-readable names:
- Configuration: `default_application_auth_strategy: key-auth-strategy`
- Resolves to: `default_application_auth_strategy_id: "uuid-123-456"`

Future versions may support namespaced references for multi-team scenarios:
- `default_application_auth_strategy: platform-team/key-auth-strategy`

## Label Management

Resources managed by kongctl are tracked using labels:

```yaml
labels:
  # User-defined labels
  team: platform
  environment: production
  
  # kongctl-managed labels (added automatically)
  KONGCTL/managed: "true"
  KONGCTL/config-hash: "sha256:abc123..."
  KONGCTL/last-updated: "2025-01-20T10:30:00Z"
  KONGCTL/protected: "true"  # If protection enabled
```

### Label Functions
- **KONGCTL/managed** - Identifies kongctl-managed resources
- **KONGCTL/config-hash** - Enables fast drift detection
- **KONGCTL/last-updated** - Tracks last modification
- **KONGCTL/protected** - Prevents deletion (requires two-step removal)

## Plan Document Format

Plans are JSON documents containing:

```json
{
  "metadata": {
    "generated_at": "2025-01-20T10:30:00Z",
    "version": "1.0",
    "config_hash": "sha256:def456..."
  },
  "summary": {
    "total_changes": 4,
    "by_action": {"CREATE": 3, "UPDATE": 1},
    "by_resource": {"application_auth_strategy": 1, "portal": 1, "portal_page": 2}
  },
  "changes": [
    {
      "id": "change-001",
      "resource_type": "application_auth_strategy",
      "resource_name": "key-auth-strategy",
      "action": "CREATE",
      "desired_state": { /* resource config */ },
      "depends_on": []
    },
    {
      "id": "change-002",
      "resource_type": "portal",
      "resource_name": "developer-portal",
      "action": "CREATE",
      "desired_state": { 
        /* includes resolved ID for auth strategy */
        "default_application_auth_strategy_id": "uuid-123-456"
      },
      "depends_on": ["change-001"],
      "references": {
        "default_application_auth_strategy": {
          "name": "key-auth-strategy",
          "resolved_id": "uuid-123-456"
        }
      }
    }
  ],
  "execution_order": ["change-001", "change-002", "change-003", "change-004"]
}
```

## Operational Modes

### Apply Mode
- Creates new resources
- Updates existing managed resources  
- Ignores unmanaged resources
- Never deletes resources
- Use case: Incremental changes, onboarding, guides

### Sync Mode
- Full reconciliation with desired state
- Deletes managed resources not in configuration
- Requires additional safety confirmations
- Use case: CI/CD, full environment management

## Safety Features

### Protected Resources
Resources marked with `kongctl.protected: true`:
- Cannot be deleted in single operation
- Require removal of protection first
- Designed for critical production resources

### Drift Detection
- Configuration hash stored in labels
- Plan validation checks for external changes
- Warning if resources modified outside kongctl

### Dependency Management
- Explicit dependency resolution
- Resources created in dependency order
- Deletion in reverse order (future)
- Circular dependency detection
- Name-to-ID resolution for cross-resource references
- Field naming follows intuitive patterns:
  - API field: `default_application_auth_strategy_id`
  - YAML field: `default_application_auth_strategy` (name reference)

## File Organization

```
konnect-config/
├── portals.yaml        # Portals with nested pages/specs
├── teams.yaml          # Top-level teams
└── openapi/           # Referenced spec files
    ├── users-v1.yaml
    └── orders-v1.yaml
```

- Arbitrary directory structure supported
- All `.yaml` and `.yml` files processed
- Resources can be split across files
- Nested resources stay with their parent
- Referenced files (like OpenAPI specs) can be organized separately

## Resource Types

Initial implementation focuses on Developer Portal resources:
- `portals` - Developer portal instances
  - `pages` - Nested documentation pages (`/portals/{id}/pages`)
  - `specs` - Nested API specifications (`/portals/{id}/specs`)
- `application_auth_strategies` - Authentication strategies (top-level)

Future releases will add:
- `teams` - Konnect teams (top-level)
- `api_products` - API product definitions (top-level)
- `applications` - Developer applications (context-dependent)
- Additional Konnect resources

Note: Resource structure (nested vs top-level) follows the Konnect API paths - resources with paths like `/parent/{id}/child/{id}` are nested, while resources with paths like `/resource/{id}` are top-level.

## Comparison with Existing Tools

### vs Terraform
- No state file management required
- Simple YAML instead of HCL
- Konnect-specific with native understanding of resources
- Lighter weight for Konnect-only workflows

### vs deck
- Designed for Konnect platform resources (not Gateway config)
- Name-based references (not UUID-based)
- Label-based metadata (not tags)
- Plan-based workflow (explicit change preview)
- Server-assigned IDs (not client-specified)