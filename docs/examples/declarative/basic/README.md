# Basic Examples

This directory contains minimal examples with only required fields for each resource type.

## Files

- `api.yaml` - Minimal API configuration
- `portal.yaml` - Minimal Portal configuration  
- `app_auth_strategy.yaml` - Minimal Application Auth Strategy configuration

## Usage

Each file demonstrates the minimum required fields to deploy that resource type to Kong Konnect.

Deploy a single resource

```bash
kongctl apply -f api.yaml
```

Deploy all resources in current directory:

```bash
kongctl apply -f .
```
