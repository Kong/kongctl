---
title: AI Gateway 2.0 Setup
summary: Install kongctl and authenticate to Konnect.
order: 2
related:
  - label: Konnect authentication lesson
    url: https://kong.github.io/kongctl/installation/authenticate/
---

Complete this setup in the shell you will use for the remaining lessons.

## Install kongctl

Install the latest kongctl release:

```shell label="Run this..."
curl -fsSL https://get.konghq.com/kongctl | sh
```

## Authenticate to Konnect

Choose one of the following authentication methods.

### Browser-based login

> _NOTE:_ If `KONGCTL_DEFAULT_KONNECT_PAT` is already set, unset it so it does
> not override the browser login credentials.

Run the login flow:

```shell label="Run this..."
kongctl login
```

### Personal access token

Alternatively, provide a Konnect personal access token:

```shell label="Set token"
export KONGCTL_DEFAULT_KONNECT_PAT="<personal-access-token>"
```

## Verify

Confirm that you can reach your Konnect organization:

```shell label="Run this..."
kongctl get organization
```
