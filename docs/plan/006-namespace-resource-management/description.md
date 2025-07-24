# Stage 6: Namespace-Based Resource Management

## Overview

Implement namespace-based resource isolation to enable multiple teams to 
safely manage their own Kong Konnect resources within a shared organization. 
This feature addresses the limitation where all resources with 
`KONGCTL-managed: true` are treated as a single set, which causes conflicts 
when multiple teams use kongctl within the same Konnect organization.

## Problem Statement

Currently, Kong Konnect advises organizations to use a single organization 
for all resources to provide data visibility across their implementation. 
However, this creates challenges:

- Multiple teams cannot independently manage their resources
- Running `sync` or `apply` affects ALL managed resources
- Risk of one team accidentally modifying or deleting another team's resources
- No way to express ownership over a subset of resources

## Solution

Introduce a `namespace` field in the `kongctl` section of resources that:
- Groups resources by ownership
- Provides isolation during sync/apply operations
- Can be set via file-level defaults
- Defaults to "default" when not specified

## Important Limitations

**Konnect API only supports labels on top-level resources**. This means:
- `kongctl.namespace` and `kongctl.protected` can only be set on parent 
  resources (Portals, APIs, Application Auth Strategies)
- Child resources (API versions, publications, implementations, documents, 
  portal pages, customizations, snippets) inherit the namespace of their parent
- This is a limitation of the Konnect platform, not kongctl

## User Stories

### As a Platform Team Lead
I want to manage core platform APIs in a "platform" namespace so that 
application teams cannot accidentally modify or delete critical 
infrastructure resources.

### As an Application Team Developer
I want to manage my team's APIs in our own namespace so that we can use 
declarative configuration without affecting other teams' resources.

### As a DevOps Engineer
I want to see which namespace resources belong to during operations so that 
I can understand the scope of changes being made.

## Requirements

### Functional Requirements

1. **Namespace Field**
   - Add `namespace` field to `kongctl` section of top-level resources only
   - Field defaults to "default" when not specified
   - Follows same pattern as `protected` field
   - Child resources inherit parent's namespace

2. **File-Level Defaults**
   - Support `_defaults.kongctl.namespace` in YAML files
   - Applied to all top-level resources in the file unless overridden
   - Enables consistent namespace assignment

3. **Namespace Isolation**
   - Commands only operate on resources in declared namespaces
   - `sync` only deletes resources within operated namespaces
   - No cross-namespace interference
   - Child resources are managed based on parent's namespace

4. **Multi-Namespace Operations**
   - Single command can process multiple namespaces from different files
   - Each namespace is planned and executed independently
   - Clear visibility of namespace operations

### Non-Functional Requirements

1. **Performance**
   - Namespace filtering should not significantly impact performance
   - Efficient grouping and processing by namespace

2. **User Experience**
   - Clear output showing namespace operations
   - Intuitive configuration syntax
   - Minimal configuration works out of the box
   - Clear documentation about label limitations

## Example Usage

### Minimal Configuration (Uses Default Namespace)
```yaml
# simple-api.yaml - no namespace specified
apis:
  - ref: basic-api
    name: "Basic API"
    description: "Simple API with default namespace"
    # No kongctl section needed - defaults to namespace: "default"
    
portals:
  - ref: dev-portal
    name: "Developer Portal"
    kongctl:
      protected: true
      # namespace defaults to "default" when not specified
```

### Multi-Team Configuration
```yaml
# team-alpha/apis.yaml
_defaults:
  kongctl:
    namespace: team-alpha

apis:
  - ref: user-api
    name: "User Management API"
    description: "API for user operations"
    kongctl:
      protected: false
      namespace: team-alpha  # Inherits from _defaults if not specified
    
    # Child resources - no kongctl section, inherit parent namespace
    versions:
      - ref: user-api-v1
        name: "v1.0.0"
        version: "1.0.0"
        spec:
          openapi: 3.0.0
          info:
            title: User API
            version: 1.0.0
          paths:
            /users:
              get:
                summary: List users
    
    implementations:
      - ref: user-api-impl-prod
        implementation_url: "https://api.example.com/users/v1"
        service:
          id: "d125e0a1-b305-4ae2-9fa8-3a57f9df85e1"
          control_plane_id: prod-cp

# team-alpha/portals.yaml
_defaults:
  kongctl:
    namespace: team-alpha

portals:
  - ref: developer-portal
    name: "Developer Portal"
    description: "Public developer portal"
    kongctl:
      protected: true
      namespace: team-alpha
    
    # Child resources inherit namespace
    pages:
      - ref: home
        slug: "/"
        title: "Welcome to Our APIs"
        content: "# Welcome\nGet started with our APIs"
    
    customization:
      ref: developer-portal-custom
      theme:
        mode: "light"
        colors:
          primary: "#8250FF"

# shared/auth-strategies.yaml
_defaults:
  kongctl:
    namespace: shared

application_auth_strategies:
  - ref: key-auth-strategy
    name: "API Key Strategy"
    display_name: "API Key Authentication"
    kongctl:
      protected: true
      namespace: shared  # Shared across teams
    strategy_type: key_auth
    configs:
      key_auth:
        key_names: ["x-api-key"]
```

### Command Output
```bash
$ kongctl sync -f team-alpha/ -f shared/

Loading configurations...
Found 2 namespace(s): team-alpha, shared

Planning changes for namespace: team-alpha
- CREATE api "user-api"
- CREATE api_version "user-api/v1.0.0" (child of user-api)
- CREATE api_implementation "user-api/prod" (child of user-api)
- UPDATE portal "developer-portal"
- DELETE api "old-api" (not in configuration)

Planning changes for namespace: shared
- CREATE application_auth_strategy "key-auth-strategy"

Proceed with sync? (y/N)
```

## Success Criteria

1. Multiple teams can manage resources independently in same Konnect org
2. No accidental cross-namespace modifications or deletions
3. Clear visibility of namespace operations in command output
4. Child resources correctly managed based on parent namespace
5. Documentation clearly explains label limitations
6. Default namespace ("default") works for simple use cases

## Out of Scope

- Namespace-level access control or permissions
- Server-side namespace enforcement
- Labels on child resources (Konnect limitation)
- Cross-namespace resource references
- Migration from non-namespaced resources