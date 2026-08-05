---
title: Configuration Paths
summary: Map config paths from command help into profile-scoped YAML.
order: 3
---

## Goal

You will find a configuration path in command help and translate it into the
correct location under a configuration profile.

## Find a configuration path

Many command flags can be specified by a configuration value. The
**Config path** specifies where that value is read from in the configuration
file. Take, for example, the `--output` flag, which specifies the command's
output format.

```shell
kongctl --help | grep -A 2 -- '--output'
```

The result includes:

```text
-o, --output string        Configures the format of data written to STDOUT.
                           - Config path: [ output ]
                           - Allowed    : [ json|yaml|text ] (default "text")
```

The **Config path** is `output`. You can set this in the configuration file
directly under the profile you want:

```yaml
default:
  output: yaml
```

## Nested Config path

Consider the command that lists Konnect APIs:

```shell
kongctl get konnect apis
```

Inspect its `--page-size` flag:

```shell
kongctl get konnect apis --help | grep -A 1 -- '--page-size'
```

The help text identifies `konnect.page-size` as the _Config Path_.
`page-size` is nested under `konnect`.

```text
--page-size int    Max number of results to include per response page.
                   - Config path: [ konnect.page-size ] (default 10)
```

Each dot creates another YAML level. The path `konnect.page-size` becomes:

```yaml
default:
  konnect:
    page-size: 10
```

A complete example with both configurations specified:

```yaml
default:
  output: yaml
  konnect:
    page-size: 10
```

Run the command to use those defaults:

```shell
kongctl get konnect apis
```
