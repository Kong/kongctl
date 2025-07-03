# Separate Files Example - Team Ownership Pattern

This example demonstrates how to organize API resources across multiple files to enable team ownership and separation of concerns.

## Structure

```
separate-files-example/
├── main-config.yaml          # Platform team manages portals and publications
├── teams/
│   ├── identity/
│   │   ├── user-api.yaml     # Identity team manages user API
│   │   ├── specs/            # Team-specific OpenAPI specs
│   │   └── docs/             # Team-specific documentation
│   ├── ecommerce/
│   │   ├── products-api.yaml # E-commerce team manages products API
│   │   └── specs/
│   └── payments/
│       └── billing-api.yaml  # Payments team manages billing API
└── README.md
```

## Usage

Deploy the configuration using the main config file:

```bash
kongctl plan --config main-config.yaml
kongctl apply --config main-config.yaml
```

## Benefits

1. **Team Ownership**: Each team owns their API definitions
2. **Separation of Concerns**: Platform team manages cross-cutting concerns
3. **Independent Development**: Teams can modify their APIs independently
4. **Centralized Policy**: Platform team controls portal and publication policies
5. **File Loading**: Uses YAML tags to load from team-specific files

## Team Responsibilities

### Platform Team
- Portal configuration and management
- API publication policies
- Cross-team resource coordination
- Main configuration maintenance

### API Teams (Identity, E-commerce, Payments)
- API specifications and versions
- Team-specific labels and metadata
- Service implementations
- Team-specific documentation

## File Loading Features Demonstrated

1. **Simple file loading**: `!file ./teams/identity/user-api.yaml`
2. **Value extraction**: `!file ./specs/products.yaml#info.title`
3. **Mixed approaches**: Teams can choose their preferred configuration style
4. **Documentation loading**: `!file ./docs/team-guide.md`

This pattern scales well for organizations with multiple teams managing different APIs while maintaining centralized governance.