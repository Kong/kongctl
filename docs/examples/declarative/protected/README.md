# Protected Resources Example

This example demonstrates how to protect critical resources from accidental modification or deletion.

## What is Resource Protection?

The `protected: true` flag prevents resources from being modified or deleted through kongctl operations. This is useful for production resources that should not be changed without explicit intention.

## Files

- `production.yaml` - Production resources with protection enabled

## How Protection Works

Protected resources:
- Cannot be updated or deleted by kongctl
- Must have protection explicitly removed before making changes
- Provide safety against accidental modifications

## Usage

```bash
# Apply protected resources
kongctl apply -f production.yaml

# Attempting to delete protected resources will fail
kongctl sync -f empty-config.yaml  # Will not delete protected resources

# To modify protected resources:
# 1. First remove protection by setting protected: false
# 2. Apply the change
# 3. Re-enable protection
```

## Best Practices

- Always protect production resources
- Use protection for critical infrastructure APIs
- Consider combining protection with namespaces for additional isolation
- Document why resources are protected in comments