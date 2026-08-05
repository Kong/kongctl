---
title: Install and Upgrade
summary: Manage trusted extensions from GitHub releases.
order: 5
related:
  - label: Complete extension developer guide
    url: https://github.com/Kong/kongctl/blob/main/docs/extensions.md
---

## Goal

You will install a trusted GitHub extension and understand how it is upgraded
or removed.

## Install from GitHub

Replace `<owner>/<repository>` with the extension repository:

```shell
kongctl install extension <owner>/<repository>
```

Pin a release when you need a repeatable version:

```shell
kongctl install extension <owner>/<repository>@v0.1.0
```

`kongctl` prefers a compatible GitHub release archive. If none is available,
it can fall back to a repository whose root contains a manifest and runnable
executable.

Remote installs display a trust warning with the selected source, executable,
command paths, and package hashes. Use `--yes` only in automation after the
source has been reviewed.

## Upgrade

Upgrade one installed extension:

```shell
kongctl upgrade extension <publisher>/<name>
```

Upgrade all eligible GitHub release installations:

```shell
kongctl upgrade extensions
```

Linked extensions and local-path installs are skipped.

## Remove an extension

Uninstall the extension while preserving its extension-owned data:

```shell
kongctl uninstall extension <publisher>/<name>
```

Add `--remove-data` only when the extension-owned data directory should also
be deleted.
