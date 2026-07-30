---
title: Command Structure
summary: Navigate verbs, products, resources, and command help.
order: 1
---

## Goal

You will understand how a `kongctl` command is organized and use command help
to discover what is available.

## Read a command

Most commands follow this pattern:

> **Command pattern**
>
> `kongctl <verb> [product] <resource> [name-or-id] [flags]`

The verb describes the action, such as `get`. The product identifies the
platform, such as `konnect`. The resource identifies what the command acts on,
such as `apis`.

## "Konnect first" CLI

`kongctl` implies the `konnect` product when it is not provided, making these
commands equivalent:

```shell
kongctl get konnect apis
kongctl get apis
```

The long-term goal for `kongctl` is to provide functionality across Kong
products and projects, including on-premises runtimes. The `konnect` product
leaves room for future non-Konnect commands, while Konnect remains the CLI's
initial primary use case.

## Discover commands with help

Start at a verb to see its resources and examples:

```shell
kongctl get --help
```

Move deeper to see the arguments, child resources, and flags for APIs:

```shell
kongctl get konnect apis --help
```

Add `--help` at any level when you are unsure what comes next.
