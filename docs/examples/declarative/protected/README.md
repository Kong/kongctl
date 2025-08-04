# Protected Resources Example

This example demonstrates how to protect critical resources from accidental modification or deletion.

## What is Resource Protection?

The `protected: true` flag prevents resources from being modified or deleted _through `kongctl` operations_. 
It's important to understand that this does not prevent changes made directly through the Konnect UI, API, or 
other means. Protection is not a native feature to Kong Konnect, but rather a mechanism provided by `kongctl` 
to help reduce the likelyhood of accidental changes or deletes.

## How Protection Works

When a resource is marked as protected, a label `KONGCTL-protected: true` is added to the resource. If no
label is present, the resource is considered unprotected.

- `protected: true` flag is added to the resource definition and applied when it is created
- Subsequent attempts to update or delete protected resources will be flagged and disallowed by `kongctl`
- A user must remove _only_ the protected flag and syncronize the change before future modifications are allowed

## Files

- `protected.yaml` - Production resources with protection enabled


## Usage

Sync a protected resources:

```bash
kongctl sync -f protected.yaml
```

View the protected label applied to the resources with the protected flag:

```bash
kongctl get apis -o json 
```

To test the protection, try commenting out the `order-api` resource in the configuration:

```yaml
  #- ref: order-api
  #  name: "Order Management API"
  #  description: "Production API for order processing"
```

Note: This resource inherits `protected: true` from the defaults section.

And run the sync command again:

```bash
kongctl sync -f protected.yaml
```

Typically the `sync` command will look for resources that exist (in the current namespace) and plan to delete them. 
With protection on, you should receive an error indicating that the resource is protected and cannot be deleted:

```text
Error: failed to generate plan: failed to plan API changes for namespace default: Cannot generate plan due to protected resources:
- api "Order Management API" is protected and cannot be deleted

To proceed, first update these resources to set protected: false
```

Try uncommenting the `order-api` resource, setting protected to false:

```yaml
  - ref: order-api
    name: "Order Management API"
    description: "Production API for order processing"
    kongctl:
      protected: false
```

Run the sync command again:

```bash
kongctl sync -f protected.yaml
```

You should see various indications that the resource will no longer be protected. 
Confirm the changes and try commenting out and sync'ing the resource again.

## Best Practices

- Use protection for critical infrastructure APIs
- Consider combining protection with namespaces for additional isolation
- Document why resources are protected in comments
