---
title: Installation
summary: Install the CLI and verify that it runs.
order: 1
related:
  - label: Complete kongctl installation documentation
    url: https://developer.konghq.com/kongctl/#install-kongctl
  - label: kongctl releases
    url: https://github.com/Kong/kongctl/releases
---

## Goal

You will install `kongctl` and confirm that the CLI runs.

## Prerequisites

You need a supported operating system and permission to install local
software.

## Install kongctl

Choose the installation command for your operating system.

### macOS or Linux with Homebrew

```shell
brew install --cask kong/kongctl/kongctl
```

Alternatively, install the formula. Homebrew uses a prebuilt bottle when one
is available for your platform, or installs the upstream release binary from
its ZIP archive. Neither path requires Go:

```shell
brew install --formula kong/kongctl/kongctl
```

If switching from the cask to the formula, first run
`brew uninstall --cask kong/kongctl/kongctl`. Both install the same `kongctl`
command and cannot be installed together.

### Linux or macOS with the shell installer

The installer detects your platform, verifies the release checksum, and
installs without `sudo`.

```shell
curl -fsSL https://get.konghq.com/kongctl | sh
```

> On Windows, download the matching archive from the `kongctl` releases page
> linked below.

## Verify

Ask `kongctl` for its detailed version information:

```shell
kongctl version --full
```

The output should identify a released `kongctl` version, commit, and build date.
Its first line will resemble:

```text
1.x.x (<commit> : <build-date>)
```
