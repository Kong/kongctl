---
title: Authenticate with Konnect
summary: Choose an authentication method and verify access to your account.
order: 2
---

## Goal

You will authenticate kongctl, verify access to your user and organization,
and compare its output formats.

## Prerequisites

You need an installed kongctl CLI and a Kong Konnect account.

## Choose an authentication method

Use either browser-based login or a personal access token.

### Browser-based login flow

Start the browser-based login flow:

```shell
kongctl login
```

Follow the terminal instructions to finish signing in.

### Personal access token (PAT)

Set your Konnect personal access token as an environment variable. Replace
`<personal-access-token>` with your token:

```shell
export KONGCTL_DEFAULT_KONNECT_PAT="<personal-access-token>"
```

## Verify your access

Confirm kongctl can read your Konnect user:

```shell
kongctl get me
```

Then confirm it can read your organization:

```shell
kongctl get organization
```

Both commands should return information from the Konnect account you used to
authenticate.

## Compare output formats

The `-o` flag presents the same resource in text, JSON, or YAML. Run each
command and compare the results:

```shell
kongctl get me -o text
kongctl get me -o json
kongctl get me -o yaml
```

Text is optimized for reading in a terminal. JSON and YAML preserve structured
data for scripts or other tools.
