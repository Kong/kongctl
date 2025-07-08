# Declarative Configuration Examples

This directory contains example declarative configuration files for kongctl.

## Files

- `portal.yaml` - Example portal configuration with child resources including customization, custom domain, pages, and snippets

## Portal Custom Domains

When configuring custom domains for portals, Konnect manages SSL certificates automatically. You only need to:
1. Set up your DNS to point to the Konnect-provided CNAME
2. Specify the domain verification method (currently only "http" is supported)
3. Konnect will handle SSL certificate provisioning and renewal

## Usage

To use these examples with kongctl:

```bash
kongctl plan -f portal.yaml
kongctl apply -f portal.yaml
```

The examples demonstrate best practices for declarative portal configuration in Konnect.