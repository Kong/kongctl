# Getting Started with kongctl

This guide walks you through using kongctl to manage Kong Konnect resources with declarative configuration. You'll learn how to define APIs, portals, and other resources as code.

## Prerequisites

1. **Kong Konnect Account**: [Sign up for free](https://konghq.com/products/kong-konnect/register) if you don't have one
2. **kongctl installed**: See [installation instructions](../README.md#installation)
3. **Authenticated with Konnect**: Run `kongctl login` to authenticate

## Introduction to Declarative Configuration

kongctl uses YAML files to define your Konnect resources. Instead of clicking through the UI or making individual API calls, you define your desired state in configuration files and kongctl handles the rest.

### Key Concepts

- **Resources**: APIs, Portals, Auth Strategies, and their related configurations
- **References (ref)**: Unique identifiers you define for each resource
- **Plan**: A preview of changes before they're applied
- **Apply**: Execute the changes to update Konnect

## Step 1: Create Your First Portal

Let's start by creating a developer portal. Create a file named `portal.yaml`:

```yaml
# portal.yaml
portals:
  - ref: my-first-portal
    name: "my-developer-portal"
    display_name: "My Developer Portal"
    description: "A portal for our API documentation"
    authentication_enabled: false
```

### Plan the Changes

Before applying, always preview what will happen:

```shell
kongctl plan -f portal.yaml
```

You'll see output like:
```
Planning changes...
Changes to apply:
  + CREATE portal "my-first-portal"
    
Total changes: 1 create, 0 update, 0 delete
```

### Apply the Configuration

Apply the changes to create the portal:

```shell
kongctl apply -f portal.yaml
```

### Verify

Check that your portal was created:

```shell
kongctl get portals
```

## Step 2: Add Your First API

Now let's add an API. Create `api.yaml`:

```yaml
# api.yaml
apis:
  - ref: users-api
    name: "Users API"
    description: "API for user management"
    version: "v1.0.0"
    labels:
      team: backend
      environment: production
```

Plan and apply:

```shell
kongctl plan -f api.yaml
kongctl apply -f api.yaml
```

## Step 3: Publish the API to Your Portal

To make your API visible in the portal, create a publication. Create `publication.yaml`:

```yaml
# publication.yaml
api_publications:
  - ref: users-api-publication
    api: users-api              # References the API we created
    portal: my-first-portal     # References the portal we created
    visibility: public
    auto_approve_registrations: true
```

Plan and apply:

```shell
kongctl plan -f publication.yaml
kongctl apply -f publication.yaml
```

## Step 4: Combine Resources in One File

For better organization, you can define multiple resources in a single file. Create `complete-setup.yaml`:

```yaml
# complete-setup.yaml
portals:
  - ref: developer-portal
    name: "developer-portal"
    display_name: "Developer Portal"
    description: "Central hub for all our APIs"
    authentication_enabled: true

apis:
  - ref: users-api
    name: "Users API"
    description: "Manage user accounts and profiles"
    version: "v1.0.0"
    labels:
      team: identity
      
  - ref: products-api  
    name: "Products API"
    description: "Product catalog and inventory"
    version: "v2.0.0"
    labels:
      team: ecommerce

api_publications:
  - ref: users-publication
    api: users-api
    portal: developer-portal
    visibility: public
    
  - ref: products-publication
    api: products-api
    portal: developer-portal
    visibility: public
```

Plan and apply all resources at once:

```shell
kongctl plan -f complete-setup.yaml
kongctl apply -f complete-setup.yaml
```

## Step 5: Add API Versions with OpenAPI Specs

Let's enhance our API with an OpenAPI specification. First, create the spec file:

```yaml
# specs/users-openapi.yaml
openapi: 3.0.0
info:
  title: Users API
  version: 1.0.0
  description: API for managing users
paths:
  /users:
    get:
      summary: List all users
      responses:
        '200':
          description: Success
  /users/{id}:
    get:
      summary: Get user by ID
      parameters:
        - name: id
          in: path
          required: true
          schema:
            type: string
      responses:
        '200':
          description: Success
```

Now update your configuration to include the API version:

```yaml
# api-with-version.yaml
apis:
  - ref: users-api
    name: "Users API"
    description: "API for user management"
    version: "v1.0.0"
    
    versions:
      - ref: users-api-v1
        name: "v1.0.0"
        spec: !file ./specs/users-openapi.yaml
```

## Step 6: Implementing the Sync Workflow

The sync command ensures your Konnect organization matches your configuration exactly, removing any resources not defined in your files.

### Understanding Sync vs Apply

- **apply**: Only creates or updates resources defined in your configuration
- **sync**: Creates, updates, AND deletes resources to match your configuration exactly

### Using Namespaces for Safety

To prevent accidental deletion of resources managed by other teams, use namespaces:

```yaml
# team-backend.yaml
_defaults:
  kongctl:
    namespace: team-backend

apis:
  - ref: users-api
    name: "Users API"
    # Inherits namespace: team-backend
    
  - ref: orders-api
    name: "Orders API"
    # Also inherits namespace: team-backend
```

When you sync with a namespace, only resources in that namespace are affected:

```shell
# Preview what will be synced
kongctl sync -f team-backend.yaml --dry-run

# Sync only team-backend namespace
kongctl sync -f team-backend.yaml
```

## Step 7: Managing Multiple Environments

Use profiles to manage different Konnect environments:

```shell
# Development environment
kongctl apply -f config.yaml --profile dev

# Production environment  
kongctl apply -f config.yaml --profile prod
```

Each profile can have different authentication tokens and Konnect organizations.

## Next Steps

### Explore Advanced Features

1. **[YAML Tags](declarative/YAML-Tags-Reference.md)**: Load content from external files
2. **[Multi-team workflows](declarative/Configuration-Guide.md#namespace-management)**: Use namespaces for team isolation
3. **[CI/CD Integration](declarative/ci-cd-integration.md)**: Automate with GitHub Actions or GitLab

### Example Configurations

Browse the [examples directory](examples/declarative/) for:
- Basic API configurations
- Multi-resource setups
- Team collaboration patterns
- Portal customization

### Best Practices

1. **Always plan before applying**: Review changes with `kongctl plan`
2. **Use version control**: Store your YAML files in git
3. **Organize by team or service**: Use separate files or directories
4. **Test in non-production first**: Use profiles for different environments
5. **Use namespaces**: Prevent accidental cross-team impacts

## Troubleshooting

If you encounter issues:

1. Check authentication: `kongctl get apis`
2. Enable debug logging: `kongctl plan -f config.yaml --log-level debug`
3. Consult the [Troubleshooting Guide](troubleshooting.md)
4. Report issues on [GitHub](https://github.com/kong/kongctl/issues)

## Summary

You've learned how to:
- Create portals and APIs using declarative configuration
- Use `plan` to preview changes
- Apply configurations to update Konnect
- Organize resources in YAML files
- Use sync for complete state management

Continue exploring kongctl's features to automate your API management workflow!