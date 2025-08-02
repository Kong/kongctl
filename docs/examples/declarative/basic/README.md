# Basic Examples

This directory contains minimal examples with only required fields for each resource type.

## Files

- `api.yaml` - Minimal API configuration
- `portal.yaml` - Minimal Portal configuration  
- `app_auth_strategy.yaml` - Minimal Application Auth Strategy configuration

## Usage

Each file demonstrates the absolute minimum required fields to successfully deploy that resource type to Kong Konnect.

```bash
# Deploy a single resource
kongctl apply -f api.yaml

# Deploy all basic resources
kongctl apply -f .
```

These examples are intended as starting points. For more complex configurations, see the comprehensive example.