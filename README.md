# kongctl

A CLI for operating Kong Gateway and Kong Konnect

> :warning: **WARNING: This is a work in progress CLI. Do not use in production. The CLI is under
heavy development and the API and behavior are subject to change.**

## Installation

### macOS

If you are on macOS, install `kongctl` using Homebrew.

Add the `kongctl` tap to your local Homebrew installation:

```shell
brew tap kong/kongctl
```

Install `kongctl`:

```shell
brew install kongctl
```

Verify the installation:

```shell
kongctl version --full
```

Which should report the installed version:

```text
0.0.12 (100a56d2e877b3004d3753446a98001c5010b478 : 2024-08-29T21:59:04Z)
```

Upgrades can be applied using:

```shell
brew upgrade kongctl
```

### Linux

To install on Linux download the proper release from the the GitHub 
[release page](https://github.com/kong/kongctl/releases) and extract the binary to a location in your PATH.

For example to install the `0.0.12` version of the x86-64 compatible binary:

```shell
curl -sL https://github.com/Kong/kongctl/releases/download/v0.0.12/kongctl_linux_amd64.zip -o kongctl_linux_amd64.zip
unzip kongctl_linux_amd64.zip -d /tmp
sudo cp /tmp/kongctl /usr/local/bin/
```

Verify the installation:

```shell
kongctl version --full
```

Which should report the installed version:

```text
0.0.12 (100a56d2e877b3004d3753446a98001c5010b478 : 2024-08-29T21:59:04Z)
```

## Usage

### Configuration File

The CLI aims to allow all values that affect the behavior to be configurable. The CLI uses the `viper` library
and conforms to it's own [precedence rules](https://github.com/spf13/viper?tab=readme-ov-file#why-viper). 
The default configuration file is created and stored at
`$XDG_CONFIG_HOME/kongctl/config.yaml`, or if `XDG_CONFIG_HOME` is not set, `$HOME/.config/kongctl/config.yaml`.

A configuration file can be specified with the `--config-file` flag, and the default file is ignored and the configuration
is read from the path specified.

A default configuration file is created on initial execution if it doesn't exist. A basic configuration will look like 
the following:

```yaml
default:
  output: text
dev:
  output: text
prod:
  output: json
```

Each top level key is a profile name and configuration values are specified in the object underneath it. 
More on profiles next...

### Profiles

The CLI supports profiles, which are used to isolate configurations. The profile is determined in the following precedence: 

1. The `--profile` flag
2. The `KONGCTL_PROFILE` environment variable

Once the profile is determined, the CLI will read the configuration from the configuration file, using the sub-configuration 
under the profile name.

> :warning: **Note: Do not use `-` characters in profile names if you intend to use environment
variables. The `-` character is not allowed in environment variable names.**

### Configuration Values

With the exception of the `--config-file` and `--profile` flags, every flag for every command can be set via the configuration system.
The command usage text will aid you in determining the configuration path for all flags.  For example:

```shell
--log-level string     Configures the logging level. Execution logs are written to STDERR.
                             - Config path: [ log-level ]
```

The usage help text for the `--log-level` flag indicates that the configuration path is `log-level`. That means for the `default` profile,
the configuration would look like:

```yaml
default:
  log-level: debug
```

Another example for the `--page-size` flag which is used to specify how many records are returned for a request to a Kong Konnect API, looks like the following:

```shell
--page-size int        Max number of results to include per response page for get and list operations.
                              (config path = 'konnect.page-size') (default 10)
```

Here, the config path is `konnect.page-size`, which means for a profile named "dev", the configuration would look like:

```yaml
dev:
  konnect:
    page-size: 20
```

### Konnect Authorization

When invoking commands that interact with the Kong Konnect API, 
the following logic is used to determine which access token to use for requests.

First, the CLI profile is determined by the `--profile` flag or the `KONGCTL_PROFILE` environment variable.
Once the profile is known, the CLI looks for a Konnect Access Token in the following order:

1. The `--pat` flag is used to specify a Konnect Personal Access Token (PAT). For example:
    
    ```shell
    kongctl get konnect gateway control-planes --pat kpat_Pfjifj...
    ```

2. The `KONGCTL_<PROFILE>_KONNECT_PAT` environment variable (where `<PROFILE>` is the name of the profile you are specifying for this command) is read
next. For example:
    ```shell
    KONGCTL_PROFILE=dev KONGCTL_DEV_KONNECT_PAT=kpat_Pfjifj... kongctl get konnect gateway control-planes
    ```

    Or:
    ```shell
    KONGCTL_FOO_KONNECT_PAT=kpat_Pfjifj... kongctl get konnect gateway control-planes --profile foo
    ```

3. If a PAT is not found, the CLI moves to using a 
[Device Authorization Flow](https://auth0.com/docs/get-started/authentication-and-authorization-flow/device-authorization-flow). 
This authorization technique uses a browser based flow, allowing you to authorize your CLI using the organization authorization provided by Kong Konnect. 
The credentials provided by this flow are preferred over the PAT, as they contain a shorter expriration time and are more secure.

   You can initialize this authorization flow by invoking the `kongctl login` command. This command will display a URL you navigate to in your 
   browser to authorize the CLI. Included in the URL is a device code that is a one-use code generated by the auth flow for your specific CLI.
   Once you have authorized the CLI using the browser, the CLI will store the access and refresh tokens in a file located 
   in a file named `.<profile>-konnect-token.json` in the same path as the loaded configuration file.

5. If the CLI locates an access token file located in the same path as the loaded configuration file, with a file name 
following the pattern `.<profile>-konnect-token.json`, the CLI will read the expriration date stored in the file and determine if the token is expired.

   If the token is unexpired, it will use the token for all requests made for that command execution. If the token is expired, the CLI will attempt to 
   refresh the token using the refresh token stored in the file. A new token is obtained, stored in the file, and used for the command execution. 

   If the refresh operation fails (maybe because the refresh token itself is expired), 
   the user will need to re-invoke the `kongctl login` command to re-authorize the CLI.

### Command Structure

The CLI is designed to follow a natural language style command structure.  Commands are generally strucutred around verbs followed by resources.  For example:

```shell
kongctl get konnect gateway control-planes
```

The verb is `get` and you are asking the CLI to retrieve a list of control planes from the Kong Konnect Gateway Manager.
The CLI will attempt to provide helpful usage text for each command to help you understand the expected input.

See the usage text for any command:

```shell
kongctl get konnect gateway control-planes --help
```

## Declarative Configuration

Kongctl supports declarative configuration management for Kong Konnect resources using YAML files. This allows you to define your API infrastructure as code and manage it through standard DevOps workflows.

### Supported Resources

- **APIs**: Define APIs with versions, publications, and implementations
- **Portals**: Create developer portals for API documentation
- **API Versions**: Manage different versions of your APIs
- **API Publications**: Control which APIs are published to which portals
- **API Implementations**: Link APIs to Kong Gateway services
- **API Documents**: Additional documentation for your APIs

### Quick Start

1. **Create a basic API configuration**:

```yaml
# my-api.yaml
apis:
  - ref: users-api
    name: "Users API"
    description: "User management and authentication API"
    version: "v1.0.0"
    labels:
      team: platform
      environment: production

portals:
  - ref: developer-portal
    name: "developer-portal"
    display_name: "Developer Portal"
    description: "API documentation for developers"
    authentication_enabled: true

api_publications:
  - ref: users-api-publication
    api: users-api
    portal: developer-portal
    visibility: public
    auto_approve_registrations: false
```

2. **Generate and review a plan**:

```shell
kongctl plan --config my-api.yaml
```

3. **Apply the configuration**:

```shell
kongctl apply --config my-api.yaml
```

### YAML Tags for External Content

Kongctl supports YAML tags for loading content from external files, enabling better organization and reusability:

#### Simple File Loading

```yaml
apis:
  - ref: my-api
    name: "My API"
    # Load description from external text file
    description: !file ./docs/api-description.txt
```

#### Value Extraction from OpenAPI Specs

```yaml
apis:
  - ref: users-api
    # Extract API metadata from OpenAPI specification
    name: !file ./specs/users-api.yaml#info.title
    description: !file ./specs/users-api.yaml#info.description
    version: !file ./specs/users-api.yaml#info.version
    
    versions:
      - ref: users-api-v1
        name: !file ./specs/users-api.yaml#info.version
        gateway_service:
          control_plane_id: "your-control-plane-id"
          id: "your-service-id"
        # Load entire OpenAPI spec
        spec: !file ./specs/users-api.yaml
```

#### Map Format for Complex Extraction

```yaml
apis:
  - ref: products-api
    name: !file
      path: ./specs/products-api.yaml
      extract: info.title
    description: !file
      path: ./specs/products-api.yaml
      extract: info.description
    labels:
      contact_email: !file
        path: ./specs/products-api.yaml
        extract: info.contact.email
```

### Multi-Resource Configuration

Define complex API platforms with multiple resources:

```yaml
# Complete API platform configuration
portals:
  - ref: public-portal
    name: "public-portal"
    display_name: "Public Developer Portal"
    description: "APIs for external developers"
    authentication_enabled: true

  - ref: partner-portal
    name: "partner-portal"
    display_name: "Partner Portal"
    description: "Private APIs for trusted partners"
    authentication_enabled: true
    rbac_enabled: true

apis:
  - ref: users-api
    name: "Users API"
    description: "User management API"
    version: "v2.0.0"
    labels:
      team: identity
      criticality: high
    kongctl:
      protected: true  # Prevent accidental deletion
    
    versions:
      - ref: users-api-v2
        name: "v2.0.0"
        gateway_service:
          control_plane_id: "your-control-plane-id"
          id: "your-service-id"
        spec: !file ./specs/users-api.yaml
    
    # Nested publications
    publications:
      - ref: users-public
        portal: public-portal
        visibility: public
        auto_approve_registrations: false
      
      - ref: users-partner
        portal: partner-portal
        visibility: private
        auto_approve_registrations: false

# Separate API publications (alternative to nested)
api_publications:
  - ref: products-api-publication
    api: products-api
    portal: public-portal
    visibility: public
    auto_approve_registrations: true
```

### Configuration Patterns

#### Team Ownership Pattern

Organize configurations across multiple files for team ownership:

```yaml
# main-config.yaml (Platform team)
portals:
  - ref: main-portal
    name: "main-portal"
    display_name: "Developer Portal"
    # Portal configuration...

apis:
  # Load team-specific API configurations
  - !file ./teams/identity/users-api.yaml
  - !file ./teams/ecommerce/products-api.yaml
  - !file ./teams/payments/billing-api.yaml

api_publications:
  # Platform team manages publication policies
  - ref: users-api-publication
    api: users-api
    portal: main-portal
    visibility: public
```

```yaml
# teams/identity/users-api.yaml (Identity team)
ref: users-api
name: "Users API"
description: "User management and authentication"
version: "v3.0.0"
labels:
  team: identity
  owner: identity-team

versions:
  - ref: users-api-v3
    name: "v3.0.0"
    gateway_service:
      control_plane_id: "control-plane-id"
      id: "service-id"
    spec: !file ./specs/users-v3.yaml
```

### Commands for Declarative Configuration

- **`kongctl plan`**: Generate execution plan showing changes
- **`kongctl apply`**: Apply configuration changes
- **`kongctl diff`**: Show differences between current and desired state

### Examples

Complete examples are available in the [`docs/examples/apis/`](docs/examples/apis/) directory:

- [`basic-api.yaml`](docs/examples/apis/basic-api.yaml) - Simple API definition
- [`api-with-versions.yaml`](docs/examples/apis/api-with-versions.yaml) - API with multiple versions
- [`api-with-external-spec.yaml`](docs/examples/apis/api-with-external-spec.yaml) - Using external OpenAPI specs
- [`api-with-yaml-tags.yaml`](docs/examples/apis/api-with-yaml-tags.yaml) - YAML tag functionality
- [`multi-resource.yaml`](docs/examples/apis/multi-resource.yaml) - Complete multi-resource platform
- [`separate-files-example/`](docs/examples/apis/separate-files-example/) - Team ownership pattern

### Troubleshooting

#### Common Issues

**File loading errors**:
```
Error: failed to process file tag: file not found: ./specs/api.yaml
```
- Verify file paths are relative to the configuration file
- Ensure the referenced file exists
- Check file permissions

**Cross-resource reference errors**:
```
Error: resource "my-api" references unknown portal: unknown-portal
```
- Verify all referenced resources are defined in the configuration
- Check resource `ref` values match exactly
- Ensure proper resource ordering (dependencies before dependents)

**Large file handling**:
- File loading has a 10MB size limit for security
- For large OpenAPI specs, consider splitting into smaller files
- Use value extraction to load only needed portions

For more detailed troubleshooting, see the [examples](docs/examples/apis/) and inline comments in the configuration files.

### Namespace Management

Kongctl supports namespace-based resource management, allowing multiple teams to safely manage their own resources within a shared Kong Konnect organization. Namespaces provide isolation between different teams or environments.

#### Key Concepts

- **Namespace Label**: Resources are tagged with a `KONGCTL-namespace` label
- **Default Namespace**: Resources without explicit namespace use "default"
- **Parent Resources Only**: Only parent resources (APIs, Portals, Auth Strategies) can have namespaces
- **Namespace Isolation**: Operations only affect resources in specified namespaces

#### Setting Namespaces

Add a namespace to any parent resource using the `kongctl` section:

```yaml
apis:
  - ref: payment-api
    name: "Payment Processing API"
    description: "Handles payment transactions"
    kongctl:
      namespace: payments-team  # This API belongs to payments team
      protected: false
```

#### File-Level Defaults

Use `_defaults` to set a namespace for all resources in a file:

```yaml
_defaults:
  kongctl:
    namespace: platform-team    # Default for all resources in this file
    protected: false

apis:
  - ref: users-api
    name: "User API"
    # Inherits namespace: platform-team from defaults
  
  - ref: admin-api
    name: "Admin API"
    kongctl:
      namespace: admin-team     # Override the default namespace
      protected: true           # Also override protection
```

#### Multi-Team Example

Different teams can manage their own resources independently:

```yaml
# team-alpha.yaml
apis:
  - ref: frontend-api
    name: "Frontend API"
    kongctl:
      namespace: team-alpha

# team-beta.yaml  
apis:
  - ref: backend-api
    name: "Backend API"
    kongctl:
      namespace: team-beta
```

When you run `kongctl sync -f team-alpha.yaml`, only resources in the `team-alpha` namespace are affected. Resources in other namespaces remain untouched.

#### Namespace Visibility

Commands show namespace operations clearly:

```bash
$ kongctl plan -f team-configs/
Loading configurations...
Found 2 namespace(s): team-alpha, team-beta

Planning changes for namespace: team-alpha
- CREATE api "frontend-api"

Planning changes for namespace: team-beta
- CREATE api "backend-api"
```

#### Best Practices

1. **One namespace per team**: Each team should use their own namespace
2. **Use descriptive names**: `payments-team`, `platform-team`, `prod`, `staging`
3. **Document ownership**: Include comments about namespace ownership
4. **Protect production**: Combine namespaces with `protected: true` for critical resources

#### Examples

Complete namespace examples are available in [`docs/examples/declarative/namespace/`](docs/examples/declarative/namespace/):

- **Single team** - Basic namespace usage
- **Multi-team** - Multiple teams in one organization  
- **With defaults** - Using `_defaults` section
- **Protected resources** - Production best practices

For detailed namespace documentation, see the [Configuration Guide](docs/declarative/Configuration-Guide.md).
