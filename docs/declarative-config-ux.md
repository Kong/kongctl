# kongctl Declarative Configuration - Technical Specification

## Overview

kongctl will implement declarative configuration management for Kong Konnect resources using a plan-based workflow. Unlike Terraform, 
it requires no state storage. Users define desired configuration in YAML files, generate execution plans, and apply changes.

## Core Concepts

### Plans

A plan is a JSON artifact containing instructions to transform resources from current state to desired state. Plans can be:
- Generated from YAML configuration files
- Saved, transported, and reviewed before execution
- Applied to make actual changes to Konnect resources

### Operation Modes

1. **Sync**: Full reconciliation including resource deletion
   - Manages all resources in configuration set
   - Deletes resources not present in desired configuration
   - Reverts unspecified values to defaults
   - Used for CI/CD workflows

2. **Apply**: Partial reconciliation without deletion
   - Only creates or updates specified resources
   - Ignores resources not in configuration
   - Only updates specified fields
   - Used for incremental changes, onboarding, quickstarts

## Command Interface

### Plan Generation
```bash
kongctl plan                                    # Generate plan from current directory
kongctl plan --dir /path/to/configs            # Generate plan from specific directory
kongctl plan --output-file my-plan.json        # Save plan to file
kongctl plan --apply-only                      # Generate apply-only plan (no deletes)
```

### Plan Execution
```bash
kongctl apply                                   # Generate and apply plan (apply-only mode)
kongctl apply --plan my-plan.json              # Apply existing plan (must be apply-only)
kongctl sync                                    # Generate and sync plan (with deletes)
kongctl sync --plan my-plan.json               # Sync existing plan
```

### Inspection Commands
```bash
kongctl diff                                    # Show pending changes
kongctl diff --plan my-plan.json               # Show changes in existing plan
kongctl diff --output yaml                     # Output in YAML format
kongctl export                                  # Export existing resources to YAML
kongctl export --filter <criteria>             # Export filtered resources
```

## Configuration Format

### Resource Declaration

Resources are declared as top-level YAML collections with string-based name references for dependencies:

```yaml
teams:
  - name: flight-operations
    description: Kong Airlines Flight Operations Team
    labels:
      department: operations
      cost-center: fl-ops-001
    kongctl:
      protected: true

auth_strategies:
  - name: api-key-auth
    display_name: Kong Airlines API Key Auth
    strategy_type: key_auth
    configs:
      key-auth:
        key_names: [apikey, api-key, x-api-key]
    labels:
      security-level: basic

apis:
  - name: flights-api
    version: v1
    slug: flights-api-v1
    publications:
      portal: kong-airlines-portal          # String reference to portal
      visibility: public
      auth_strategy_ids: [api-key-auth]     # String reference to auth strategy
    labels:
      team: flight-operations               # String reference to team
```

### Metadata Management

The `kongctl` section in resources is converted to special labels in Konnect:

```yaml
# In configuration:
kongctl:
  protected: true

# Becomes labels in Konnect:
labels:
  KONGCTL/managed: "true"
  KONGCTL/protected: "true"
  KONGCTL/config-hash: "sha256:abc123"
  KONGCTL/last-updated: "2025-01-24T10:30:00Z"
```

## Plan Structure

Plans are JSON documents with the following structure:

```json
{
  "metadata": {
    "generated_at": "2025-01-24T10:30:00Z",
    "config_hash": "sha256:7a8f3b2c",
    "plan_version": "1.0",
    "generated_by": "kongctl v0.1.0",
    "reference_mappings": {
      "kong-airlines-portal": "portal-2c5e8a7f-9b3d-4f6e-a1c8-7d5b2a8f3c9e",
      "api-key-auth": "auth-123e4567-e89b-12d3-a456-426614174000"
    }
  },
  "summary": {
    "total_changes": 3,
    "by_action": {"CREATE": 2, "UPDATE": 1},
    "by_resource": {"developer_portal": 1, "api": 1}
  },
  "changes": [
    {
      "id": "change-001",
      "resource_type": "developer_portal",
      "resource_id": "portal-2c5e8a7f-9b3d-4f6e-a1c8-7d5b2a8f3c9e",
      "resource_name": "kong-airlines-portal",
      "action": "UPDATE",
      "current_state": { /* full resource */ },
      "field_changes": [
        {
          "field": "auto_approve_applications",
          "current_value": true,
          "desired_value": false
        }
      ],
      "depends_on": [],
      "execution_context": {
        "api_endpoint": "/v3/developer-portals/{id}",
        "http_method": "PATCH"
      }
    }
  ],
  "execution_order": ["change-001", "change-002", "change-003"]
}
```

## Technical Implementation Details

### Resource Identity

- Resources are identified by name, not UUID
- Names must be unique within resource type
- Server-assigned IDs are tracked in plan metadata for reference resolution

### Dependency Resolution

- Dependencies expressed via string-based names
- Automatic ordering based on dependency graph
- Creation happens in dependency order, deletion in reverse

### Drift Detection

- Configuration hash stored in `KONGCTL/config-hash` label
- Plan validation checks if resources have changed since plan generation
- Resources without kongctl labels are considered unmanaged

### Resource Protection

Protected resources require two-step deletion:
1. Remove `protected: true` from configuration and apply
2. Delete the resource in subsequent operation

### State Tracking Without State Files

State information stored as Konnect resource labels:
- `KONGCTL/managed`: Resource is managed by kongctl
- `KONGCTL/config-hash`: Hash of declarative config for drift detection
- `KONGCTL/last-updated`: Last modification timestamp
- `KONGCTL/protected`: Requires explicit unprotection

## Differences from decK

1. **First-class resources only**: No nested resource declarations
2. **Name-based references**: No UUID references between resources
3. **Label-based metadata**: Instead of tags with special behaviors
4. **Plan-based workflow**: Explicit plan generation before changes
5. **Server-assigned IDs**: Cannot specify custom resource IDs
6. **Protection mechanism**: Built-in two-step deletion for critical resources

## Authentication

Uses standard kongctl authentication:
- Access token via flags, environment variables, or config files
- `kongctl login` with device auth grant flow
- Organization determined by access token

## Error Handling

- API errors during plan generation result in partial plans with clear error messages
- Plan execution validates state hasn't drifted since generation
- Dependency failures halt execution at safe points
- Clear error messages indicate which resources failed and why

## File Organization

- Arbitrary directory structure supported
- All `.yaml` and `.yml` files in directory tree are processed
- Resources can be split across multiple files
- No special file naming requirements

## Future Considerations

- Resource filtering for partial syncs
- Plan comparison and merging
- Rollback plan generation
- Support for non-Konnect targets (`kongctl sync gateway`)
