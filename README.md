# <picture><source media="(prefers-color-scheme: dark)" srcset="./brand/logo/dark/Kong-Logomark.svg"><source media="(prefers-color-scheme: light)" srcset="./brand/logo/light/Kong-Logomark.svg"><img src="./brand/logo/light/Kong-Logomark.svg" alt="Kong logo" width="32"></picture> kongctl

The Kong Konnect CLI

## Table of Contents

- [What is `kongctl`?](#what-is-kongctl)
- [Documentation](#documentation)
- [Installation](#installation)
  - [macOS](#macos)
  - [Linux](#linux)
  - [Verify](#verify)
- [Getting Started](#getting-started)
  - [1. Create a Kong Konnect Account](#1-create-a-kong-konnect-account)
  - [2. Authenticate with Konnect](#2-authenticate-with-konnect)
  - [3. Test the Authentication](#3-test-the-authentication)
  - [4. Switch Region](#4-switch-konnect-regions)
  - [5. Next Steps](#5-next-steps)
- [Telemetry](#telemetry)
- [Documentation Listing](#documentation-listing)
- [Configuration and Profiles](#configuration-and-profiles)
  - [Authentication Options](#authentication-options)
  - [Color Themes](#color-themes)
- [Command Structure](#command-structure)
- [Support](#support)
- [Security](#security)

## What is `kongctl`?

`kongctl` is a command-line tool for Kong Konnect that enables you to:
- Manage Konnect resources programmatically
- Define your Konnect API infrastructure as code using declarative configuration
- Integrate Konnect into your CI/CD pipelines
- Automate API lifecycle management

*Note: Future releases may include support for Kong Gateway on-premise deployments.*

## Documentation

For complete documentation and guides, see the documentation on the Kong Developer site:

[https://developer.konghq.com/kongctl/](https://developer.konghq.com/kongctl/)

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
# Example: Install the latest release for x86-64
curl -sL https://github.com/Kong/kongctl/releases/latest/download/kongctl_linux_amd64.zip -o kongctl_linux_amd64.zip
unzip kongctl_linux_amd64.zip -d kongctl-linux-amd64
sudo install kongctl-linux-amd64/kongctl /usr/local/bin/kongctl
```

### Verify

```shell
kongctl version --full
```

## Getting Started

### 1. Create a Kong Konnect Account

If you don't have a Kong Konnect account, [sign up for free](https://konghq.com/products/kong-konnect/register).

### 2. Authenticate with Konnect

Use the `kongctl login` command to authenticate with your Kong Konnect account:

```shell
kongctl login
```

Follow the instructions given in the terminal to complete the login process.

*Note: For token-based authentication with Konnect PATs, see
[Authentication Options](#authentication-options).*

### 3. Test the Authentication

You can verify that `kongctl` is authenticated and can access information on your Konnect account by running:

```shell
kongctl get me
```

or the associated organization with:
```shell
kongctl get organization
```

### 4. Switch Konnect Regions

By default `kongctl` uses the `us` region for Konnect API requests. You can switch regions be passing the
`--region` flag to your command with the short region code, such as `eu`, `us`, or `au`.

Run `kongctl get regions` to retrieve the list of currently supported regions directly from Konnect.
The [Konnect geos documentation](https://developer.konghq.com/konnect-platform/geos/) also tracks new regions as they launch.

*Note: Region can also be configured with the configuration key `konnect.region`. See [Configuration and Profiles](#configuration-and-profiles)
for details on managing `kongctl` configuration.*

### 5. Next Steps

**→ [Read the Declarative Configuration Guide](docs/declarative.md)** - Learn how to use declarative configuration to manage your APIs in Konnect

## Telemetry

`kongctl` collects limited usage data to help Kong understand CLI usage.

Collected:
  - kongctl version
  - operating system and architecture
  - command path, such as `login`, `apply`, or `get apis`

Not collected:
  - command arguments or flag values
  - resource names or IDs
  - auth tokens, request bodies, or response bodies
  - config file contents, file paths, hostnames, usernames, or email addresses

Telemetry can be disabled at any time with:
  `kongctl --no-telemetry <command>`
  `KONGCTL_NO_TELEMETRY=true kongctl <command>`
  `DO_NOT_TRACK=1 kongctl <command>`

The first interactive `kongctl login` also asks whether kongctl may collect
usage data on this device. The answer is saved to
`$XDG_CONFIG_HOME/kongctl/.telemetry-enabled` (or
`$HOME/.config/kongctl/.telemetry-enabled`) and applies to all local configuration profiles.
This local preference file overrides profile config.
`DO_NOT_TRACK=1`, `KONGCTL_NO_TELEMETRY=true`, and `--no-telemetry` still
disable telemetry even when the local preference file says telemetry is
enabled.

Disable telemetry persistently for a specific profile:

```yaml
profile-name:
  telemetry:
    enabled: false
```

*Note: See [Configuration and Profiles](#configuration-and-profiles) for configuration instructions.*

Telemetry values can be inspected without sending it by enabling debug
mode:

```yaml
profile-name:
  telemetry:
    enabled: true
    debug: true
```

When debug mode is enabled, events are written to
`$XDG_CONFIG_HOME/kongctl/logs/telemetry.log` (or
`$HOME/.config/kongctl/logs/telemetry.log`) and are not sent to the telemetry
backend.

## Documentation Listing

- **[Declarative Configuration Guide](docs/declarative.md)** - Complete guide covering quick start, concepts, YAML tags, CI/CD integration, and best practices
- **[Troubleshooting Guide](docs/troubleshooting.md)** - Common issues and solutions
- **[Examples](docs/examples/)** - Sample configurations and use cases

## Configuration and Profiles

`kongctl` configuration data is read from `$XDG_CONFIG_HOME/kongctl` and falls back to 
`$HOME/.config/kongctl`. The standard configuration file (YAML format) is located in this location and is named 
`config.yaml`. At the root level you specify profiles where a profile is a named collection of 
configuration values. By default there is a `default` profile, but you can 
create additional profiles for different environments or configurations.

*Note: By design `kongctl` does not write to your configuration file. The philosphy for this is that
there should be one owner and writer of configuration data, and that is the user.*

Some flags and options can be changed by providing a value in the configuration file, effectively 
allowing you to override the default behavior of command options. Flags that support this will 
be documented in the command help text with a "Config path" note that looks like this:

```text
-o, --output string        Configures the format of data written to STDOUT.
                             - Config path: [ output ]
                             - Allowed    : [ json|yaml|text ] (default "text")
```

The above help text shows a YAML key path for the `--output` flag which controls the format of output text
from the CLI. The config path is the location in the configuration file where a flag value can be defaulted. 
In this case it specifies that output formats can be set in the configuration file under an `output` key. 
Nested config paths use dots in command help. For example, `konnect.region`
is configured as `konnect: { region: ... }` in the profile.

The basic example of a configuration file follows:

```yaml
default:
  output: text
  konnect:
    region: us
second-profile:
  output: json
  konnect:
    region: eu
```

Specifying a profile can be done using the `--profile` flag or by setting or exporting
the `KONGCTL_PROFILE` environment variable.

Configuration values can also be specified using environment variables. `kongctl` looks for environment variables
which follow the pattern `KONGCTL_<PROFILE>_<PATH>`, where `<PROFILE>` is the profile name in uppercase and `<PATH>` 
is the configuration path in uppercase with `_` substituting for `.`.
For example, to set the `output` format for the `default` profile, you can use:

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

  *Note: To clear saved Konnect credentials for a profile, run `kongctl logout [--profile <name>]`. This removes the local device
  flow token file so that subsequent commands prompt you to authenticate again with `kongctl login`.*

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

### Color Themes

Interactive experiences, such as `kongctl view`, share a configurable color
theme. Use the `--color-theme` flag (or set the `color-theme` key in your
configuration file) to select a palette. The default `auto` setting detects
whether the terminal uses a dark background and selects `kong-dark` or
`kong-light` accordingly. You can still choose either Kong theme explicitly.
The legacy `kong` name remains supported as an alias for `kong-light`, and you
can also switch to any
[`bubbletint`](https://github.com/lrstanley/bubbletint) theme by ID, for
example:

```yaml
default:
  color-theme: tokyo_night
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
- `kongctl delete -f api.yaml` - Delete resources declared in a file

## Support

- **Issues**: [GitHub Issues](https://github.com/kong/kongctl/issues)
- **Documentation**: [Kong Docs](https://developer.konghq.com/kongctl)
- **Community**: [Kong Nation](https://discuss.konghq.com)

Please report bugs and provide feedback through GitHub Issues or the
[Kong Nation](https://discuss.konghq.com/) community.

## Security

For security-related issues, see the [Security Policy](SECURITY.md). Please do
not publicly disclose vulnerabilities in GitHub issues or community forums.
Report potential vulnerabilities to
[vulnerability@konghq.com](mailto:vulnerability@konghq.com). Enterprise
customers can also use their customer support channels.
