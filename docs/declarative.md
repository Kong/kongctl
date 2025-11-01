# Declarative Configuration

This page covers what you need to know for managing Kong Konnect
resources using the `kongctl` declarative configuration approach.

## Table of Contents

- [Overview](#overview)
- [Quick Start](#quick-start)
- [Core Concepts](#core-concepts)
- [Resource Types](#supported-resource-types)
- [Configuration Structure](#configuration-structure)
- [Kongctl Metadata](#kongctl-metadata)
- [YAML Tags](#yaml-tags)
- [Commands Reference](#commands-reference)
- [CI/CD Integration](#cicd-integration)
- [Best Practices](#best-practices)
- [Troubleshooting](#troubleshooting)

## Overview

`kongctl`'s declarative management feature enables you to 
manage your [Kong Konnect](https://konghq.com/products/kong-konnect) resources with 
simple YAML declaration files and a simple state free CLI tool.

### Key Principles

1. **Configuration manifests**: Configuration is expressed as simple YAML files that describe the desired state of your Konnect resources. 
   Configuration files can be split into multiple files and directories for modularity and reuse.
1. **Plan-Based**: Plans are objects that represent required changes to move a set of resources from one state to another, desired, state. 
   In `kongctl`, plan artifacts are first class concepts that can be created, stored, reviewed, and applied. Plans are represented as JSON objects
   and can be generated and stored as files for later application. When running declarative commands, if plans are not provided
   they are generated implicitly and executed immediately.
1. **State-Free**: `kongctl` does not use a state file or database to store the current state. The system relies on querying of the 
   online Konnect state in order to calculate plans and apply changes.
1. **Namespace Support**: Namespaces provide a way to isolate resources between teams and environments. 
   Each resource can be assigned to a specific namespace, and resources in different namespaces are not considered when calculating plans or 
   applying changes. Namespaces can be set at the resource level or inherited from file-level defaults.

## Quick Start

### Prerequisites

1. **Kong Konnect Account**: [Sign up for free](https://konghq.com/products/kong-konnect/register)
2. **`kongctl` installed**: See [installation instructions](../README.md#installation)
3. **Authenticated with Konnect**: Run `kongctl login`

### Create Your First Configuration

Create a working directory:

```shell
mkdir kong-portal && cd kong-portal
```

Create a file named `portal.yaml`:

```yaml
portals:
  - ref: my-portal
    name: "my-developer-portal"
    display_name: "My Developer Portal"
    description: "API documentation for developers"
    authentication_enabled: false
    default_api_visibility: "public"
    default_page_visibility: "public"

apis:
  - ref: users-api
    name: "Users API"
    description: "API for user management"

    publications:
      - ref: users-api-publication
        portal_id: my-portal
```

Preview changes:

```shell
kongctl diff -f portal.yaml
```

Apply configuration:

```shell
kongctl apply -f portal.yaml
```

Verify resources with `kongctl get` commands:

```shell
kongctl get portals
```

```shell
kongctl get apis
```

Your developer portal and API are now live! Visit the [Konnect Console](https://cloud.konghq.com/us/portals/) 
to see your developer portal with the published API.

## Core Concepts

### Resource Identity

Resources have multiple identifiers:

- **ref**: Required field for each reference in declarative configuration. 
    `ref` must be unique across any loaded set of configuration files.
- **id**: Most Konnect resources have an `id` field which is a Konnect
    assigned UUID. This field is not represented in declarative configuration files.
- **name**: Many Konnect resources have a `name` field which may or may not be 
    unique within an organization for that resource type. 

```yaml
application_auth_strategies:
  - ref: oauth-strategy              # ref identifies a resource within a configuration
    name: "OAuth 2.0 Strategy"       # Identifies an auth strategy within Konnect

portals:
  - ref: developer-portal
    name: "Developer Portal"
    default_application_auth_strategy: oauth-strategy  # References the auth strategy by it's ref value
```

### Plan Artifacts

Plans are central to how `kongctl` manages resources. Plans are objects which
define the required steps to move a set of resources from their current state to a
desired state. Plans can be created, stored, reviewed, and applied at a later time
and are stored as JSON files. Plans are not required to be used, 
but can enable advanced workflows.

#### How Planning Works

Both `apply` and `sync` commands use the planning engine internally:

**Implicit Planning** (direct execution):

```shell
# Internally generates plan and executes it
kongctl apply -f config.yaml
```

**Explicit Planning** (two-phase execution):

```shell
# Phase 1: Generate plan artifact
kongctl plan -f config.yaml --output-file plan.json

# Phase 2: Execute plan artifact (can be done later)
kongctl apply --plan plan.json
```

#### Why Use Plan Artifacts?

Plan artifacts enable more advanced workflows:

- **Audit Trail**: Store plans in version control alongside configurations
- **Review Process**: Share plans with team members before execution
- **Deferred Execution**: Generate plans in CI, apply them after approval
- **Rollback Safety**: Keep previously applied plans for rollback
- **Compliance**: Document exactly what changes were planned

## Supported Resource Types

`kongctl` aims to support all of the Kong Konnect resource types but each
resource requires specific handling and coding changes. The following 
lists the currently supported resources and their relationships.

### Parent vs Child Resources

**Parent Resources** (support kongctl metadata):

- APIs
- Portals
- Application Auth Strategies
- Control Planes (including Control Plane Groups)

**Child Resources** (do NOT support kongctl metadata):

- API Versions
- API Publications
- API Implementations
- API Documents
- Portal Pages
- Portal Snippets
- Portal Customizations
- Portal Custom Domains
- Gateway Services 

## Configuration Structure

### Basic Structure

```yaml
# Optional defaults section
_defaults:
  kongctl: # kongctl metadata defaults
    namespace: production
    protected: false

portals: # List of portal resources
  - ref: developer-portal # ref is required on all resources
    name: "developer-portal"
    display_name: "Developer Portal"
    description: "API documentation hub"
    kongctl: # kongctl metadata defined explicitly on resource
      namespace: platform-prod
      protected: true
```

### Root vs hierarchical configuration

Parents are defined at the root of a configuration while
children can be expressed both nested under their parent
and at the root with a parent reference field.

**Hierarchical Configuration**:

```yaml
apis:
  - ref: users-api
    name: "Users API"
    versions:
      - ref: v1
        name: "v1.0.0"
        spec: !file ./specs/users-v1.yaml
    publications:
      - ref: public
        portal: main-portal
        visibility: public
```

**Separate Configuration**:

```yaml
apis:
  - ref: users-api
    name: "Users API"

api_versions:
  - ref: v1
    api: users-api
    name: "v1.0.0"
    spec: !file ./specs/users-v1.yaml

api_publications:
  - ref: public
    api: users-api
    portal: main-portal
```

### Control Plane Groups

Control planes can represent Konnect control plane groups by setting their cluster type to `"CLUSTER_TYPE_CONTROL_PLANE_GROUP"`. Group entries manage membership through the `members` array. Each member must resolve to the Konnect ID of a non-group control plane, so you can provide literal UUIDs or reference other declarative control planes with `!ref`.

```yaml
control_planes:
  - ref: shared-group
    name: "shared-group"
    cluster_type: "CLUSTER_TYPE_CONTROL_PLANE_GROUP"
    members:
      - id: !ref prod-us-runtime#id
      - id: !ref prod-eu-runtime#id
```

When you apply or sync this configuration, `kongctl` replaces the entire membership list in Konnect to match the declarative `members` block.

## Kongctl Metadata

The `kongctl` section provides metadata for resource management.
This metadata is stored in Kong Konnect labels and labels are only 
provided on parent resources. Thus, `kongctl` metadata is 
**only supported on parent resources**.

### Protected Resources

The `protected` field prevents accidental deletion of critical resources:

```yaml
portals:
  - ref: production-portal
    name: "Production Portal"
    kongctl:
      protected: true  # Cannot be deleted until protection is removed
```

### Namespace Management

The `namespace` field enables multi-team resource isolation:

```yaml
apis:
  - ref: billing-api
    name: "Billing API"
    kongctl:
      namespace: finance-team  # Owned by finance team
      protected: false
```

### File-Level Defaults

Use `_defaults` to set default values for all resources in a file:

```yaml
_defaults:
  kongctl:
    namespace: platform-team
    protected: true

portals:
  - ref: api-portal
    name: "API Portal"
    # Inherits namespace: platform-team and protected: true

  - ref: test-portal
    name: "Test Portal"
    kongctl:
      namespace: qa-team
      protected: false
    # Overrides both defaults
```

### Namespace and Protected Field Behavior

`kongctl` provides some default behavior depending on how metadata fields
are specified or omitted. The following tables summarize the behavior.

#### `namespace` Field Behavior

| File Default | Resource Value | Final Result | Notes                        |
|--------------|----------------|--------------|------------------------------|
| Not set      | Not set        | "default"    | System default               |
| Not set      | "team-a"       | "team-a"     | Resource explicit            |
| Not set      | "" (empty)     | ERROR        | Empty namespace not allowed  |
| "team-b"     | Not set        | "team-b"     | Inherits default             |
| "team-b"     | "team-a"       | "team-a"     | Resource overrides           |
| "team-b"     | "" (empty)     | ERROR        | Empty namespace not allowed  |
| "" (empty)   | Any value      | ERROR        | Empty default not allowed    |

#### `protected` Field Behavior

| File Default | Resource Value | Final Result | Notes              |
|--------------|----------------|--------------|-------------------|
| Not set      | Not set        | false        | System default     |
| Not set      | true           | true         | Resource explicit  |
| Not set      | false          | false        | Explicit false     |
| true         | Not set        | true         | Inherits default   |
| true         | false          | false        | Resource overrides |
| false        | true           | true         | Resource overrides |

Child resources automatically inherit the namespace of their parent resource:

## YAML Tags

YAML tags are like preprocessors for YAML file data. They allow you to 
load content from external files, reference across resources and extract specific
values from structured data. Over time more tags may be added to support various
functions and use cases.

### Loading File Content to YAML Fields

Load the entire content of a file as a string:

```yaml
apis:
  - ref: users-api
    name: "Users API"
    description: !file ./docs/api-description.md
```

Supported file types: Any text file (`.txt`, `.md`, `.yaml`, `.json`, etc.)

### Value Extraction

You can extract specific values from structured data loaded from the `file` tag
with this hash (`#`) notation:

```yaml
apis:
  - ref: users-api
    name: !file ./specs/openapi.yaml#info.title # loads info.title field from the openapi.yaml file
    description: !file ./specs/openapi.yaml#info.description
    version: !file ./specs/openapi.yaml#info.version

    versions:
      - ref: v1
        spec: !file ./specs/openapi.yaml
```

Alternatively values can be extracted using this map format:

```yaml
apis:
  - ref: products-api
    name: !file
      path: ./specs/products.yaml
      extract: info.title
    labels:
      contact: !file
        path: ./specs/products.yaml
        extract: info.contact.email
```

### Path Resolution

All file paths are resolved relative to the directory containing the
configuration file:

```
project/
├── config.yaml          # Main config file
├── specs/
│   ├── users-api.yaml
│   └── products-api.yaml
└── docs/
    └── descriptions.txt
```

In `config.yaml`:

```yaml
apis:
  - ref: users-api
    name: !file ./specs/users-api.yaml#info.title
    description: !file ./docs/descriptions.txt
```

### Security Features

**Path Traversal Prevention**: Absolute paths and path traversal attempts are
blocked:

```yaml
# ❌ These will fail with security errors
description: !file /etc/passwd
config: !file ../../../sensitive/file.yaml

# ✅ These are allowed
description: !file ./docs/description.txt
config: !file ./config/settings.yaml
```

**File Size Limits**: Files are limited to 10MB.

### Performance Features

**File Caching**: Files are cached during a single execution to improve
performance:

```yaml
apis:
  - ref: api-1
    name: !file ./common.yaml#api.name        # File loaded and cached
    description: !file ./common.yaml#api.desc # Uses cached version
  - ref: api-2
    team: !file ./common.yaml#team.name       # Uses cached version
```

## Commands Reference

The following are high level descriptions of commands for declarative
configuration management. See the command usage text for details on 
command usage, flags and options.

### plan

Create a plan - a JSON file containing the set of planned changes to a set of resources.
Plans are generated with either `--mode apply` or `--mode sync` which determines
whether resources missing from the input configuration are planned for deletion or not.

Generate an apply plan and output to STDOUT:

```shell
kongctl plan -f config.yaml --mode apply
```

Generate a sync plan and output to STDOUT:

```shell
kongctl plan -f config.yaml --mode sync
```

### apply

Applying a configuration will create or update resources to match the desired state
and will **not delete** resources. Because `apply` does not delete resources, it can
be used for incremental application of resource configurations. For example, you could
apply a `portal` in one command and then later apply `apis` in a separate command. With
the `sync` command, this process is not possible as missing resources will be deleted.

Apply directly from config:

```shell
kongctl apply -f config.yaml
```

Apply from saved plan:

```shell
kongctl apply --plan plan.json
```

Preview changes without applying:

```shell
kongctl apply -f config.yaml --dry-run
```

### sync

`sync` applies a set of configurations including deleting resources
missing from the input configuration data.

Preview sync changes:

```shell
kongctl sync -f config.yaml --dry-run
```

Sync configuration with a prompt confirmation:

```shell
kongctl sync -f team-config.yaml
```

Skip confirmation prompt (caution!): 

```shell
kongctl sync -f config.yaml --auto-approve
```

Sync from a plan artifact:

```shell
kongctl sync --plan plan.json
```

### diff

Display human-readable preview of changes between current and desired state:

Preview changes from configuration file:

```shell
kongctl diff -f config.yaml
```

Preview changes from a plan artifact:

```shell
kongctl diff --plan plan.json
```

### adopt

`kongctl` declarative configuration engine will only consider resources that
are part of the list of `kongctl.namespace` values given to it during planning
and execution of changes. There may be cases where you want to bring an
existing Konnect resource into configuration that was created outside of the
configuration management process. The `adopt` command enables you to
add the proper namespace label to an existing Konnect resources without modifying any other
fields. Once you adopt a resource, you need to add the configuration for it
to your configuration set to ensure it is managed going forward.

Adopt a portal by name:

```shell
kongctl adopt portal my-portal --namespace team-alpha
```

Adopt a control plane by ID:

```shell
kongctl adopt control-plane 22cd8a0b-72e7-4212-9099-0764f8e9c5ac \
  --namespace platform
```

If the resource already has a `KONGCTL-namespace` label, the command fails
without making changes. 

### dump

Export current Konnect resource state to various formats.

```shell
# Export all APIs with their child resources and include debug logging
# to tf-import format
kongctl dump tf-import --resources=api --include-child-resources
```

```shell
# Export all portal and api resources to 
# kongctl declarative configuration with format and the team-alpha namespace
kongctl dump declarative --resources=portal,api --default-namespace=team-alpha
```

## CI/CD Integration

Key principles for CI/CD integration:

1. **Plan on PR**: Generate and review plans in pull requests
2. **Apply on Merge**: Apply reviewed plans when merged to target branch
3. **Environment Separation**: Different configs for dev/staging/prod
4. **Approval Gates**: Require human approval for production

## Best Practices

### Multi-Team Setup

Each team manages their own namespace:

```yaml
# team-alpha/config.yaml
_defaults:
  kongctl:
    namespace: team-alpha

apis:
  - ref: frontend-api
    name: "Frontend API"
    # Automatically in team-alpha namespace
```

### Environment Management

Use configuration profiles for different environments:

```shell
# Development environment
kongctl apply -f config.yaml --profile dev

# Production environment
kongctl apply -f config.yaml --profile prod
```

### Security Best Practices

1. **Protect production resources**:
   ```yaml
   apis:
     - ref: payment-api
       kongctl:
         namespace: production
         protected: true
   ```

2. **Use namespaces for isolation**:
   - One namespace per team
   - Separate namespaces for environments
   - Clear namespace ownership documentation

3. **Version control everything**:
   - Configuration files
   - OpenAPI specifications
   - Documentation

4. **Review plans before applying**:
   - Use `plan` in production
   - Save plans for audit trail
   - Implement approval workflows

### Plan Artifact Workflows

#### Basic Plan Review Workflow

Developer creates plan:

```shell
kongctl plan -f config.yaml --output-file proposed-changes.json
```

Review changes visually:

```shell
kongctl diff --plan proposed-changes.json
```

Share plan for review (commit to git, attach to PR, etc.):

```shell
git add proposed-changes.json
git commit -m "Plan for adding new API endpoints"
```

After approval, apply the plan:

```shell
kongctl apply --plan proposed-changes.json
```

#### Production Deployment with Approval

```shell
# CI/CD Pipeline Stage 1: Plan Generation
kongctl plan -f production-config.yaml \
  --output-file plan-$(date +%Y%m%d-%H%M%S).json

# Stage 2: Manual approval gate
# - Plan artifact is stored as build artifact
# - Team reviews plan details
# - Approval triggers next stage

# Stage 3: Plan Execution
kongctl apply --plan plan-20240115-142530.json --auto-approve
```

#### Emergency Rollback Using Previous Plan

List recent plans (assuming you store them):

```shell
ls -la plans/
```

Review what the previous state included:

```shell
kongctl diff --plan plans/last-known-good.json
```

Revert to previous state:

```shell
kongctl sync --plan plans/last-known-good.json --auto-approve
```

### Common Mistakes to Avoid

❌ **Setting kongctl on child resources**:

```yaml
# WRONG
apis:
  - ref: my-api
    kongctl:
      namespace: team-a
    versions:
      - ref: v1
        kongctl:  # ERROR - not supported on child resources
          protected: true
```

✅ **Correct approach**:

```yaml
# RIGHT
apis:
  - ref: my-api
    kongctl:
      namespace: team-a
      protected: true
    versions:
      - ref: v1
```

❌ **Using name as identifier**:

```yaml
# WRONG - using display name
api_publications:
  - ref: pub1
    api: "Users API"
```

✅ **Use ref for references**:

```yaml
# RIGHT - using ref
api_publications:
  - ref: pub1
    api: users-api
```

### Field Validation

Kongctl uses strict YAML validation to catch configuration errors early:

```yaml
# This will cause an error
portals:
  - ref: my-portal
    name: "My Portal"
    lables:  # ❌ ERROR: Unknown field 'lables'. Did you mean 'labels'?
      team: platform
```

Common field name errors:

- `lables` → `labels`
- `descriptin` → `description`
- `displayname` → `display_name`
- `strategytype` → `strategy_type`

## Troubleshooting

### Common Issues

**Authentication Failures**:

- Verify PAT is not expired
- Check authentication: `kongctl get me`
- Ensure proper credential storage

**Plan Generation Failures**:

- Validate YAML syntax
- Check file paths are correct
- Verify network connectivity

**Apply Failures**:

- Review plan for conflicts
- Check for protected resources
- Verify dependencies exist

**File Loading Errors**:

```
Error: failed to process file tag: file not found: ./specs/missing.yaml
```

- Verify the file path is correct
- Check that the file exists
- Ensure proper relative path from config file location

### Debug Mode

Enable verbose logging:

```bash
kongctl apply -f config.yaml --log-level debug
```

Enable trace logging for HTTP requests:

```bash
kongctl apply -f config.yaml --log-level trace
```

For more troubleshooting help, see the [Troubleshooting
Guide](troubleshooting.md).

## Examples

Browse the [examples directory](examples/declarative/)

## Related Documentation

- [Troubleshooting Guide](troubleshooting.md) - Common issues and solutions
- [E2E Test Harness](e2e.md) - How to run end-to-end tests locally and in CI
