---
title: Konnect Authentication
summary: Choose an authentication method and verify access to your account.
order: 2
---

## Goal

You will authenticate `kongctl`, verify access to your organization and any
available user identity, and compare command output formats.

## Prerequisites

Complete [Install kongctl](../install-kongctl/) first. You also need a
[Kong Konnect account](https://konghq.com/products/kong-konnect/register).

## Choose an authentication method

Use either browser-based login or an access token.

### Browser-based login flow

Start the browser-based login flow:

```shell
kongctl login
```

Follow the terminal instructions to finish signing in.

### Access token

Use either a personal access token (PAT) or a system account access token
(SPAT). Set the token as an environment variable, replacing `<access-token>`
with your token:

```shell
export KONGCTL_DEFAULT_KONNECT_PAT="<access-token>"
```

## Verify

Check whether the authenticated identity has an associated Konnect user:

```shell
kongctl get me
```

Browser login and PAT authentication return user information. This command is
expected to fail with a SPAT because a system account does not have a user
identity.

Confirm `kongctl` can read your organization:

```shell
kongctl get organization
```

This command works with browser login, a PAT, or a SPAT.

## Compare output formats

The `-o` flag presents the same resource in text, JSON, or YAML. Run each
command and compare the results:

```shell
kongctl get organization -o text
kongctl get organization -o json
kongctl get organization -o yaml
```

Text is optimized for reading in a terminal. JSON and YAML preserve structured
data for scripts or other tools.
