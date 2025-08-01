# kongctl

A command-line interface (CLI) for Kong Konnect.

## ⚠️ Tech Preview Disclaimer

**This is a TECH PREVIEW release of kongctl. This software is provided by Kong, Inc. without warranty and is not recommended for production use. The CLI is under active development - interfaces, commands, and behaviors are subject to change without notice. Use at your own risk for evaluation and testing purposes only.**

By using this software, you acknowledge that:
- It may contain bugs and incomplete features
- It should not be used for critical systems or production workloads
- Data loss or service disruption may occur
- No support commitments or SLAs apply

## What is kongctl?

kongctl is a command-line tool for Kong Konnect that enables you to:
- Manage Konnect resources programmatically
- Define your API infrastructure as code using declarative configuration
- Integrate Konnect into your CI/CD pipelines
- Automate API lifecycle management

*Note: Future releases may include support for Kong Gateway on-premise deployments.*

## Installation

### macOS

Install using Homebrew:

```shell
brew tap kong/kongctl
brew install kongctl
```

Verify installation:

```shell
kongctl version --full
```

### Linux

Download from the [release page](https://github.com/kong/kongctl/releases):

```shell
# Example: Install v0.0.12 for x86-64
curl -sL https://github.com/Kong/kongctl/releases/download/v0.0.12/kongctl_linux_amd64.zip -o kongctl_linux_amd64.zip
unzip kongctl_linux_amd64.zip -d /tmp
sudo cp /tmp/kongctl /usr/local/bin/
```

## Getting Started

### 1. Create a Kong Konnect Account

If you don't have a Kong Konnect account, [sign up for free](https://konghq.com/products/kong-konnect/register).

### 2. Authenticate with Konnect

Login using device authorization:

```shell
kongctl login
```

This will open your browser for authentication. Once complete, your credentials are securely stored.

### 3. Test Your Connection

List your control planes:

```shell
kongctl get control-planes
```

### 4. Next Steps

**→ [Read the Getting Started Guide](docs/getting-started.md)** - Learn how to use declarative configuration to manage your APIs in Konnect

## Quick Examples

### Define an API with Declarative Configuration

```yaml
# my-api.yaml
apis:
  - ref: users-api
    name: "Users API"
    description: "User management API"
    version: "v1.0.0"

portals:
  - ref: developer-portal
    name: "developer-portal"
    display_name: "Developer Portal"
```

Plan and apply your configuration:

```shell
# Preview changes
kongctl plan -f my-api.yaml

# Apply configuration
kongctl apply -f my-api.yaml
```

## Documentation

- **[Getting Started Guide](docs/getting-started.md)** - Step-by-step tutorial for declarative configuration
- **[Declarative Configuration](docs/declarative-configuration.md)** - Complete reference guide
- **[Troubleshooting Guide](docs/troubleshooting.md)** - Common issues and solutions
- **[Examples](docs/examples/)** - Sample configurations and use cases

### Additional Resources

- **[Configuration Guide](docs/declarative/Configuration-Guide.md)** - Detailed configuration reference
- **[YAML Tags Reference](docs/declarative/YAML-Tags-Reference.md)** - External file loading
- **[CI/CD Integration](docs/declarative/ci-cd-integration.md)** - Automation examples

## Configuration & Profiles

kongctl uses profiles to manage different Konnect environments (dev, staging, production). Configuration is stored at `~/.config/kongctl/config.yaml`.

### Using Profiles

```shell
# Use dev profile
kongctl get apis --profile dev

# Or via environment variable
KONGCTL_PROFILE=prod kongctl get apis
```

### Authentication Options

1. **Device Flow** (Recommended):
   ```shell
   kongctl login
   ```

2. **Personal Access Token (PAT)**:
   ```shell
   kongctl get apis --pat your-token-here
   # Or via environment variable
   KONGCTL_PROFILE=dev KONGCTL_DEV_KONNECT_PAT=your-token kongctl get apis
   ```

## Command Structure

Commands follow a verb-resource pattern for Konnect resources:

```shell
kongctl <verb> <resource-type> [resource-name] [flags]
```

Examples:
- `kongctl get apis` - List all APIs in Konnect
- `kongctl get api users-api` - Get specific API details
- `kongctl create portal` - Create a new developer portal
- `kongctl delete api my-api` - Delete an API from Konnect

## Support

- **Issues**: [GitHub Issues](https://github.com/kong/kongctl/issues)
- **Documentation**: [Kong Docs](https://docs.konghq.com)
- **Community**: [Kong Nation](https://discuss.konghq.com)

Remember: This is tech preview software. Please report bugs and provide feedback through GitHub Issues.