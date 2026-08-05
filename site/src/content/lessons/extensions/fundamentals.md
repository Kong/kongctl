---
title: Extension Fundamentals
summary: Understand how extensions add commands and where trust begins.
order: 1
related:
  - label: Complete extension developer guide
    url: https://github.com/Kong/kongctl/blob/main/docs/extensions.md
---

## Goal

You will understand how an extension participates in the `kongctl` command
tree and what permissions it receives.

## What is an extension?

An extension is an external executable that contributes one or more command
paths to `kongctl`. For example, an extension could add:

> `kongctl get debug-info`

`kongctl` installs, links, upgrades, and removes extensions. When an extension
command runs, its executable starts as a separate child process.

## Inspect installed extensions

List the extensions currently available:

```shell
kongctl list extensions
```

An empty list is valid when no extensions are installed.

Inspect one extension by its manifest identity:

```shell
kongctl get extension <publisher>/<name>
```

Replace `<publisher>` and `<name>` with values from the installed extension
list.

## Understand the trust boundary

> **Extensions are executable code**
>
> Install and run only extensions whose source and release artifacts you
> trust.

Extensions are not isolated sandboxes. They run with your operating-system
permissions and can make Konnect requests with the profile and credentials
available to the parent `kongctl` command.
