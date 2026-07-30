---
title: Configuration Profiles
summary: Understand how profiles group settings and credentials.
order: 1
---

## Goal

You will understand `kongctl` profiles, the default profile, and how to select
profiles.

## What is a profile?

Every `kongctl` command runs with a profile: a named collection of settings
and credentials. If you do not select one, `kongctl` uses the `default`
profile.

Profiles let you organize configurations and credentials to match your desired
workflow. You may have different profiles for different environments or
machines, or maybe for different product areas or jobs to be done.

## Inspect your profiles

List the profiles `kongctl` can use:

```shell
kongctl get profiles
```

The output lists available profiles. By default there will be one, the
`default` profile. You will learn to create a profile in a subsequent lesson.

```text
PROFILE
default
```

## Select a profile explicitly

Every command can accept a `--profile` flag to set the profile for that
command invocation.

```shell
kongctl get me --profile default
```

The profile can also be specified by an environment variable,
`KONGCTL_PROFILE`. In `kongctl`, flags take precedence over environment
variables.

```shell
export KONGCTL_PROFILE=default
kongctl get me
```

Both commands use the same profile and authentication established during
installation.
