# Platform Team Configuration

This configuration demonstrates how a platform team manages shared infrastructure, specifically a developer portal that will be used by multiple teams across the organization.

## Overview

The platform team is responsible for:
- Managing the shared developer portal
- Providing stable portal infrastructure for other teams
- Publishing their own platform APIs
- Maintaining portal branding, navigation, and content

## Files Structure

```
platform/
├── portal.yaml         # Complete portal definition
├── api.yaml           # Platform Core API
├── pages/             # Portal static content
│   ├── home.md
│   ├── apis.md  
│   ├── getting-started.md
│   ├── guides.md
│   └── guides/
│       ├── authentication.md
│       └── rate-limits.md
├── snippets/          # Reusable portal content
│   ├── welcome-banner.md
│   └── api-quickstart.md
├── specs/             # OpenAPI specifications
│   ├── platform-core-v2.yaml
│   └── platform-core-v1.yaml (legacy)
├── docs/              # API documentation
│   ├── overview.md
│   ├── authentication.md
│   ├── webhooks.md
│   └── examples.md
└── README.md          # This file
```

## Portal Configuration

The portal (`portal.yaml`) includes:

### Basic Settings
- **Name**: "Shared Developer Portal"
- **Authentication**: Disabled for public access
- **Visibility**: Public APIs and pages
- **Branding**: Company theme with custom colors

### Navigation Structure
- **Main Menu**: Getting Started, APIs, Developer Guides
- **Footer**: Product links, company links, legal links

### Content Pages
- **Home**: Welcome page with API overview
- **APIs**: Catalog of all published APIs
- **Getting Started**: Quick start guide
- **Guides**: Comprehensive developer documentation

### Reusable Snippets
- **Welcome Banner**: Hero section for home page
- **API Quickstart**: Quick start code examples

## Platform Core API

The Platform Core API (`api.yaml`) provides:

### Core Services (v2.1.0)
- **Authentication**: OAuth 2.0 and API key support
- **User Management**: Profile and permission management
- **Configuration**: Feature flags and settings
- **Health Checks**: Service status monitoring

### Legacy Support (v1.0.0)
- Maintained for backward compatibility
- Basic authentication and user endpoints
- Will be deprecated in future releases

## Deployment

```bash
# Deploy the platform configuration
cd platform/
kongctl apply portal.yaml api.yaml
```

This creates:
1. The "Shared Developer Portal" with all pages and branding
2. Platform Core API published to the portal
3. Portal infrastructure that other teams can reference

## External Reference

Other teams reference this portal using:

```yaml
portals:
  - ref: shared-developer-portal
    _external:
      selector:
        matchFields:
          name: "Shared Developer Portal"
```

## Best Practices

### Portal Management
- Keep portal names stable (other teams depend on them)
- Use semantic versioning for portal updates
- Document portal structure and available snippets
- Coordinate navigation changes with API teams

### API Integration  
- Ensure Platform Core API is always published to the portal
- Maintain backward compatibility for legacy versions
- Provide comprehensive documentation and examples
- Monitor API usage and performance

### Content Management
- Regular content reviews and updates
- Keep getting started guides current
- Ensure code examples work with latest API versions
- Maintain consistent writing style and branding

## Monitoring

The platform team should monitor:
- Portal availability and performance
- API usage metrics and error rates
- Developer engagement with portal content
- External team dependencies on shared resources

## Support

- **Team**: Platform Team
- **Email**: platform-team@company.com
- **Portal**: Direct portal management questions
- **API**: Platform Core API technical issues
- **Infrastructure**: Shared resource availability