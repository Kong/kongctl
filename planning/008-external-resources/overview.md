# External Resources Feature Overview

## Purpose

The External Resources feature enables kongctl to reference and integrate with
resources managed by other Kong declarative configuration tools (decK, Kong
Operator, Terraform provider). This allows users to maintain existing workflows
and investments while gradually adopting kongctl for comprehensive Konnect
resource management.

## Problem Statement

Kong customers currently use multiple declarative configuration tools:
- **decK**: Manages Kong Gateway "core entities" (services, routes, plugins,
  consumers) via YAML configuration
- **Kong Operator**: Kubernetes-native operator for reconciling K8s resources
  to Konnect
- **Terraform Provider**: Infrastructure-as-code management for Konnect

As kongctl expands its capabilities, users need a migration path that doesn't
require abandoning existing tools immediately. They need to operate multiple
tools in parallel during transition periods.

## Solution Overview

External Resources allows kongctl to:
1. Reference resources managed by external tools without taking ownership
2. Query and retrieve identity information (IDs, names, composite keys) for
   these external resources
3. Use external resource data in relationships and dependencies within kongctl
   configurations
4. Enable gradual migration from other tools to kongctl

## Key Capabilities

### External Resource Declaration
Users define `external_resource` blocks in kongctl configuration that:
- Specify the resource type and identifying attributes
- Indicate which external tool manages the resource
- Provide sufficient information for kongctl to query the resource via SDK

### Resource Information Retrieval
kongctl will:
- Use the Konnect SDK to query external resources based on provided identifiers
- Cache retrieved information for use within the configuration lifecycle
- Validate that external resources exist and are accessible

### Integration Points
External resources can be referenced in:
- Relationship definitions (e.g., linking an API to an externally-managed
  Control Plane)
- Dependency declarations
- Resource associations

## Benefits

1. **Gradual Migration**: Users can adopt kongctl incrementally without
   disrupting existing workflows
2. **Tool Interoperability**: Enables mixed-tool environments during transition
   periods
3. **Investment Protection**: Preserves existing configuration investments in
   decK, Terraform, or Kong Operator
4. **Flexibility**: Supports various migration strategies based on customer needs

## Success Criteria

- Users can reference any Konnect resource managed by external tools
- External resource data is accurately retrieved and usable within kongctl
- Clear error messages when external resources are unavailable or misconfigured
- Minimal performance impact from external resource queries
- Documentation clearly explains usage patterns and migration strategies