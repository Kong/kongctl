---
title: AI Gateway 2.0 Beta Steps
summary: Install the required prerelease and authenticate to Konnect .tech.
order: 2
related:
  - label: Konnect authentication lesson
    url: https://kong.github.io/kongctl/installation/authenticate/
---

> **IMPORTANT:** The examples in this chapter currently require the AI Gateway
> 2.0 prerelease build and the Kong Konnect `.tech` environment. Complete this
> setup in the shell you will use for the remaining lessons.

## Install the prerelease

Install the latest AI Gateway 2.0 prerelease build:

```shell label="Run this..."
curl -fsSL https://get.konghq.com/kongctl | \
  sh -s -- --version prerelease-aigw-2
```

## Target Konnect dev environment

We need to test against the development environment while
AI Gateway 2.0 is still in beta.

Set `konnect.environment` for the `default` profile with this
env var:

```shell label="Run this..."
export KONGCTL_DEFAULT_KONNECT_ENVIRONMENT=tech
```

## Authenticate to .tech

Production credentials cannot authenticate to `.tech`. Choose one of the
following methods after setting the environment variable.

### Browser-based login

> _NOTE:_ If `KONGCTL_DEFAULT_KONNECT_PAT` is already set for production, unset it so it
> does not override the browser login credentials.

Run the login flow again:

```shell label="Run this..."
kongctl login
```

### Personal access token

Alternatively, provide a PAT created in `.tech`:

```shell label="Set dev token"
export KONGCTL_DEFAULT_KONNECT_PAT="<tech-personal-access-token>"
```

## Verify

Confirm that the you are now setup to reach the `.tech` organization:

```shell label="Run this..."
kongctl get organization
```
