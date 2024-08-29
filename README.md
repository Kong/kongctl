# kongctl

A CLI for operating Kong Gateway and Kong Konnect

> :warning: **WARNING: This is a work in progress CLI. Do not use in production. The CLI is under
heavy development and the API and behavior are subject to change.**

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

First, the CLI profile is determined by the `--profile` flag or the `KONG_PROFILE` environment variable.
Once the profile is known, the CLI looks for the Konnect Personal Access Token (PAT) in the following order:

1. The `--pat` flag is used to specify the Konnect Personal Access Token. For example:
    
    ```shell
    kongctl get konnect gateway control-planes --pat kpat_Pfjifj...
    ```

2. The `KONG_<PROFILE>_KONNECT_PAT` where `<PROFILE>` is the name of the profile you are specifying for this command. For example:

    ```shell
    KONG_DEV_KONNECT_PAT=kpat_Pfjifj... kongctl get konnect gateway control-planes
    ```

3. If a PAT is not found, the CLI moves to using a Device Grant Authorization flow. This authorization technique uses a browser based flow allowing you
to authorize your CLI using the web based organization authorization provided by Kong Konnect. You can initialize this authorization flow by invoking
the `kongctl login konnect` command. This command will prompt you with a URL to navigate to in your browser to authorize the CLI. Once you have authorized
the CLI, the CLI will store the access and refresh tokens in a file located at `$XDG_CONFIG_HOME/kongctl/.<profile>-konnect-token.json`.

4. If the CLI locates an authorization file located at `$XDG_CONFIG_HOME/kongctl/.<profile>-konnect-token.json`, it will 
read the expriration date stored in the file. If the token is not expired, it will use the token for all requests made during that
command execution. If the token is expired, the CLI will attempt to refresh the token using the refresh token stored in the file. If the refresh
operation fails (because the refresh token itself is expired), the user will need to re-invoke the `kongctl login konnect` command to re-authorize the CLI.

### Command Structure

The CLI is designed to follow a natural language style command structure.  Commands are generally strucutred around verbs followed by resources.  For example:

```shell
kongctl get konnect gateway control-planes
```

The verb is `get` and you are asking the CLI to retrieve a list of control planes from the Kong Konnect Gateway Manager.
The CLI will attempt to provide helpful usage text for each command to help you understand the expected input.

