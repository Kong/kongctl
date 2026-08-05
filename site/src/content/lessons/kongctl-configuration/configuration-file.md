---
title: The Configuration File
summary: Locate and structure the user-owned CLI configuration file.
order: 2
---

## Goal

You will locate the `kongctl` configuration file and understand how it groups
settings by profile.

## Locate the file

`kongctl` reads `config.yaml` from the standard configuration directory:

- `$XDG_CONFIG_HOME/kongctl/config.yaml` when `XDG_CONFIG_HOME` is set.
- `$HOME/.config/kongctl/config.yaml` otherwise, usually written as
  `~/.config/kongctl/config.yaml`.

The installation routines do not create this file. The first `kongctl` command
creates it when it is missing and writes an initial `default` profile. Otherwise
`kongctl` does not write to this file.

Print the path for your current environment:

```shell
printf '%s\n' "${XDG_CONFIG_HOME:-$HOME/.config}/kongctl/config.yaml"
```

## You own the file

`kongctl` writes the file only when initializing a missing default
configuration. After that initial creation, `kongctl` reads the file but does
not rewrite it. You maintain the configuration values it consumes.

This prevents later commands from rewriting or reformatting your configuration
choices.

## Profiles are root keys

The file is YAML. Each top-level key is a profile name, and the values nested
under it belong to that profile:

```yaml
<profile>:
  <config-key>: <value>
<profile-2>:
  <config-key>: <value>
```

For example:

```yaml
default:
  output: text
team-a:
  output: yaml
```

## Check the profiles

After saving the file, list the profiles `kongctl` found:

```shell
kongctl get profiles
```

The example command should show `default` and `team-a` profiles.
