# kongctl

The Kong Konnect CLI

## ⚠️ Tech Preview ⚠️

**`kongctl` is a _Tech Preview_ project. This software is provided by Kong, Inc. without warranty and is not recommended for 
production use. The CLI is under active development - interfaces, commands, and behaviors are subject to change without notice. 
Use at your own risk for evaluation and testing purposes only.**

By using this software, you acknowledge that:
- It may contain bugs and incomplete features
- It should not be used for critical systems or production workloads
- Data loss or service disruption may occur
- No support commitments or SLAs apply

## Table of Contents

- [What is `kongctl`?](#what-is-kongctl)
- [Installation](#installation)
  - [macOS](#macos)
  - [Linux](#linux)
  - [Verify](#verify)
- [Getting Started](#getting-started)
  - [1. Create a Kong Konnect Account](#1-create-a-kong-konnect-account)
  - [2. Authenticate with Konnect](#2-authenticate-with-konnect)
  - [3. Test the Authentication](#3-test-the-authentication)
  - [4. Next Steps](#4-next-steps)
- [Documentation Listing](#documentation-listing)
- [Configuration and Profiles](#configuration-and-profiles)
  - [Color Themes](#color-themes)
  - [Authentication Options](#authentication-options)
- [Command Structure](#command-structure)
- [Support](#support)

## What is `kongctl`?

`kongctl` is a command-line tool for Kong Konnect that enables you to:
- Manage Konnect resources programmatically
- Define your Konnect API infrastructure as code using declarative configuration
- Integrate Konnect into your CI/CD pipelines
- Automate API lifecycle management

*Note: Future releases may include support for Kong Gateway on-premise deployments.*

## Installation

### macOS

Install using Homebrew (distributed as a cask):

```shell
brew install --cask kong/kongctl/kongctl
```

If you previously installed the old formula, remove it first with `brew uninstall kongctl`.

### Linux

Download from the [release page](https://github.com/kong/kongctl/releases):

```shell
# Example: Install v0.0.12 for x86-64
curl -sL https://github.com/Kong/kongctl/releases/download/v0.0.12/kongctl_linux_amd64.zip -o kongctl_linux_amd64.zip
unzip kongctl_linux_amd64.zip -d /tmp
sudo cp /tmp/kongctl /usr/local/bin/
```

### Verify

```shell
kongctl version --full
```

## Getting Started

### 1. Create a Kong Konnect Account

If you don't have a Kong Konnect account, [sign up for free](https://konghq.com/products/kong-konnect/register).

### 2. Authenticate with Konnect

Use the `kongctl login` command to authenticate with you Kong Konnect account:

```shell
kongctl login
```

Follow the instructions given in the terminal to complete the login process.

### 3. Test the Authentication

You can verify that `kongctl` is authenticated and can access information on your Konnect account by running:

```shell
kongctl get me 
```

### 4. Next Steps

**→ [Read the Declarative Configuration Guide](docs/declarative.md)** - Learn how to use declarative configuration to manage your APIs in Konnect

## Documentation Listing

- **[Declarative Configuration Guide](docs/declarative.md)** - Complete guide covering quick start, concepts, YAML tags, CI/CD integration, and best practices
- **[Troubleshooting Guide](docs/troubleshooting.md)** - Common issues and solutions
- **[Examples](docs/examples/)** - Sample configurations and use cases
- **[E2E Test Harness](docs/e2e.md)** - How to run end-to-end tests locally and in CI

## Configuration and Profiles

`kongctl` configuration data is read from `$XDG_CONFIG_HOME/kongctl/config.yaml`. The format of the file is YAML,
and at the root level you can specify profiles. A profile is a named collection of configuration values. By default
there is a `default` profile, but you can create additional profiles for different environments or configurations.

The basic example of a configuration file follows:

```yaml
default:
  output: text
```

Some flags and options can be defaulted by providing a value in the configuration file, effectively 
allowing you to override the default behavior of commands. Flags that support this will be documented
in the command help text with a "Config path" note that looks like this:

```text
-o, --output string        Configures the format of data written to STDOUT.
                             - Config path: [ output ]
                             - Allowed    : [ json|yaml|text ] (default "text")
```

The above help text shows a YAML key path for the `--output` flag which controls the format of output text
from the CLI. The config path is the location in the configuration file where a flag value can be defaulted. 
In this case it specifies that output formats can be set in the configuration file under an `output` key. 

### Color Themes

Interactive experiences—such as `kongctl kai` or `kongctl get --interactive` views—share a configurable Bubble Tea
color theme. Use the `--color-theme` flag (or set the `color-theme` key in your configuration file) to select a
palette. The default `kong` theme mirrors the existing brand styling, and you can switch to any
[`bubbletint`](https://github.com/lrstanley/bubbletint) theme by ID, for example:

```yaml
default:
  color-theme: tokyo_night
```

```shell
kongctl get apis --interactive --color-theme tokyo_night
```

You can also use the shorthand `-i` flag.

When you request the interactive view—using commands like `kongctl get --interactive`
or `kongctl get konnect --interactive`—the CLI launches a Konnect resource navigator
home screen. The navigator lists the
currently supported resources (APIs, application-auth-strategies, gateway control-planes,
and portals) in alphabetical order. Select a resource to open its existing interactive
table view, and use `esc`/`backspace` to return to the navigator and switch between
resources.

Themes adapt automatically to light and dark terminals and honor the global `--color` mode.

It's called a config _path_ because the key may be nested. For example this `--control-plane-flag` has this 
nested path:

```text
--control-plane-id string     The ID of the control plane to use for a gateway service command.
                                    - Config path: [ konnect.gateway.control-plane.id ]
```

To set the default for this, a configuration file might looks like this:

```yaml
default:
  konnect:
    gateway:
      control-plane:
        id: <control-plane-id>
```

You can specify different values under different profiles, like so:

```yaml
default:
  output: text
cicd:
  output: json
```

This configuration file defines two profiles: `default` and `cicd`. 
The `default` profile will output text, while the `cicd` profile will output JSON.

You can use the `--profile` flag to specify which profile to use when running commands:

```shell
kongctl get apis --profile cicd
```

Or you can set the `KONGCTL_PROFILE` environment variable:

```shell
KONGCTL_PROFILE=cicd kongctl get apis
```

Configuration values can also be specified using environment variables. `kongctl` looks for environment variables
which follow the pattern `KONGCTL_<PROFILE>_<PATH>`, where `<PROFILE>` is the profile name in uppercase and `<PATH>` 
is the configuration path in uppercase. For example, to set the output format for the `default` profile, you can use:

```shell
KONGCTL_DEFAULT_OUTPUT=yaml kongctl get apis 
```

### Authentication Options

`kongctl` makes requests to the Konnect API using API tokens. There are two primary methods for authentication.

1. **Device Flow** (Recommended):

   Execute the following command to authorize `kongctl` with your Kong Konnect account:

   ```shell
   kongctl login
   ```

   This command will generate a web link you can use to open a browser window and authenticate with your Kong Konnect account. 
   After logging in and authorizing the CLI using the provided code, `kongctl` will store token and refresh token data in a file at 
   `$XDG_CONFIG_HOME/kongctl/.<profile>-konnect-token.json`

2. **Personal Access Token flag**:

   You can also pass an API token directly using the `--pat` flag. This is useful for automation pipelines 
   where you want to avoid interactive login or provide various tokens for different operations.

   ```shell
   kongctl get apis --pat <token>
   ```

   You can also set an environment variable for the token following the same pattern as configuration values:

   ```
   KONGCTL_DEFAULT_KONNECT_PAT=<token> kongctl get apis
   ```

## Command Structure

Commands generally follow a verb->product->resource->args pattern with `konnect` as the default product.

```shell
kongctl <verb> <product> <resource-type> [resource-name] [flags]
```

Examples:
- `kongctl get apis` - List all APIs in Konnect (`konnect` product is implicit)
- `kongctl get konnect apis` - List all APIs in Konnect (using full product name)
- `kongctl get api users-api` - Get specific API details
- `kongctl delete api my-api` - Delete an API from Konnect

## Support

- **Issues**: [GitHub Issues](https://github.com/kong/kongctl/issues)
- **Documentation**: [Kong Docs](https://developer.konghq.com)
- **Community**: [Kong Nation](https://discuss.konghq.com)

Remember: This is tech preview software. Please report bugs and provide feedback through GitHub 
Issues or the [Kong Nation](https://discuss.konghq.com/) community.
