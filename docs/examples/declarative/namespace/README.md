# Namespace Example

This example demonstrates how to use namespaces to isolate groups of resources for declarative configuration.

## What are Namespaces?

Namespaces allow arbitrary grouping of resources within the same Kong Konnect organization. 
Resources are tagged with a `KONGCTL-namespace` label, and `kongctl` operations only affect resources within 
the specified namespaces.

## Files

- `team-alpha.yaml` - APIs owned by Team Alpha (namespace: team-alpha)
- `team-beta.yaml` - APIs owned by Team Beta (namespace: team-beta)

## Usage

Sync both team's resources:

```bash
kongctl sync -f team-alpha.yaml -f team-beta.yaml
```

Investigate the resources created and observe the namespace labels applied:

```bash
kongctl get apis -o json
```

Because we are using `sync`, resources can be deleted. To remove all managed
APIs in a namespace, pass an explicit empty API list with the namespace default:

⚠️ Warning: This removes all resources in the namespace, so use with caution! ⚠️

```bash
echo "_defaults: {kongctl: {namespace: team-beta}}\napis: []" | kongctl sync -f -
```

Notice that only APIs in the `team-beta` namespace will be removed. Check the
list of APIs again to verify that `team-alpha` APIs remain intact:

```bash
kongctl get apis -o json
```

## Key Points

- Only parent resources (APIs, Portals, Auth Strategies) can have namespaces
- Child resources inherit their parent's namespace
- Operations are isolated to the namespaces defined in your configuration files
- Resources without a namespace use the "default" namespace
