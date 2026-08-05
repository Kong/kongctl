---
title: Publish an Extension
summary: Package release archives for predictable remote installation.
order: 6
related:
  - label: Complete release guide and checklist
    url: >-
      https://github.com/Kong/kongctl/blob/main/docs/extensions.md#release-archives
---

## Goal

You will understand how to package and validate an extension release.

## Build the archive

Prefer GitHub release archives for public extensions. The archive root must
contain the manifest instead of wrapping the package in another directory:

- `kongctl-extension.yaml`
- `bin/kongctl-ext-foo`
- `README.md`

Script extensions can publish one universal archive. Compiled extensions
should publish a separate archive for each supported operating system and
architecture, for example:

- `kongctl-ext-foo-linux-amd64.tar.gz`
- `kongctl-ext-foo-darwin-arm64.tar.gz`
- `kongctl-ext-foo-windows-amd64.zip`

## Verify the release

Before publishing:

1. Run the executable directly.
2. Link it and exercise every contributed command.
3. Install it from a local path.
4. Publish a tagged GitHub release archive.
5. Install that exact tag.
6. Upgrade from an older tag.
7. Verify structured output remains clean with `--output json`.

## Prefer safe examples

Public examples should begin with read-only behavior, such as displaying the
runtime context, calling `kongctl get me`, or summarizing APIs, portals, and
control planes.
