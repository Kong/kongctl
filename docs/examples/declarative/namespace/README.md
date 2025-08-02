# Namespace Example

This example demonstrates how to use namespaces to isolate resources between different teams.

## What are Namespaces?

Namespaces allow multiple teams to manage their own resources within the same Kong Konnect organization. Resources are tagged with a `KONGCTL-namespace` label, and kongctl operations only affect resources within the specified namespaces.

## Files

- `team-alpha.yaml` - APIs owned by Team Alpha (namespace: team-alpha)
- `team-beta.yaml` - APIs owned by Team Beta (namespace: team-beta)

## Usage

```bash
# Apply Team Alpha's resources
kongctl apply -f team-alpha.yaml

# Apply Team Beta's resources  
kongctl apply -f team-beta.yaml

# Sync Team Alpha's namespace (removes unmanaged resources in team-alpha namespace only)
kongctl sync -f team-alpha.yaml

# Preview changes with dry-run
kongctl sync -f team-beta.yaml --dry-run
```

## Key Points

- Only parent resources (APIs, Portals, Auth Strategies) can have namespaces
- Child resources inherit their parent's namespace
- Operations are isolated to the namespaces defined in your configuration files
- Resources without a namespace use the "default" namespace