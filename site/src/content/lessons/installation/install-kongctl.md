---
title: Install kongctl
summary: Install the CLI, verify it, and connect it to your Konnect account.
order: 1
related:
  - label: Complete kongctl installation documentation
    url: https://developer.konghq.com/kongctl/#install-kongctl
  - label: kongctl releases
    url: https://github.com/Kong/kongctl/releases
---

## Goal

You will install `kongctl`, confirm that it runs, and authenticate with
Konnect.

## Prerequisites

You need a Kong Konnect account to authenticate after the installation.

## Install kongctl

Choose the installation command for your operating system.

### macOS with Homebrew

```shell
brew install --cask kong/kongctl/kongctl
```

### Linux or macOS with the shell installer

The installer detects your platform, verifies the release checksum, and
installs without `sudo`.

```shell
curl -fsSL https://get.konghq.com/kongctl | sh
```

> On Windows, download the matching archive from the kongctl releases page
> linked below.

## Check it worked

Ask kongctl for its detailed version information:

```shell
kongctl version --full
```

The output should identify a released kongctl version, commit, and build date.
Its first line will resemble:

```text
1.x.x (<commit> : <build-date>)
```

## Connect to Konnect

Start the browser-based login flow:

```shell
kongctl login
```

Follow the terminal instructions, then verify that kongctl can read your user
record:

```shell
kongctl get me
```

You now have an authenticated kongctl installation.
