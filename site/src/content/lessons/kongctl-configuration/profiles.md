---
title: Profiles and configuration
summary: Understand how profiles group settings and shape environment variables.
order: 1
---

## Goal

You will understand the default kongctl profile and recognize how a profile
name appears in environment variables.

## What is a profile?

Every kongctl command runs with a profile: a named collection of settings and
credentials. If you do not select one, kongctl uses the `default` profile.

Profiles let you keep settings for different teams or environments separate.
Browser login credentials are also stored for the active profile.

## Inspect your profiles

List the profiles kongctl can use:

```shell
kongctl get profiles
```

The output includes the profile used so far:

```text
PROFILE
default
```

## Decode the PAT variable

The PAT variable from installation follows kongctl's configuration pattern:

```text
KONGCTL_<PROFILE>_<CONFIGURATION_PATH>
```

In `KONGCTL_DEFAULT_KONNECT_PAT`, `DEFAULT` selects the `default` profile and
`KONNECT_PAT` identifies the setting.

## Select a profile explicitly

Run a command with the default profile named explicitly:

```shell
kongctl get me --profile default
```

You can also select it for the current shell:

```shell
export KONGCTL_PROFILE=default
kongctl get me
```

Both commands use the same profile and authentication established during
installation.
