---
title: Runtime Context
summary: Receive host settings, arguments, and authenticated access safely.
order: 3
related:
  - label: Go extension example
    url: https://github.com/Kong/kongctl/tree/main/docs/examples/extensions/go
  - label: Complete extension developer guide
    url: https://github.com/Kong/kongctl/blob/main/docs/extensions.md
---

## Goal

You will understand what `kongctl` passes to an extension and how an extension
can reuse the active invocation context.

## Receive arguments

Extension-specific arguments and flags normally pass through directly:

```shell
kongctl get foo --limit 10
```

Use `--` when an extension must receive a flag name owned by the host:

```shell
kongctl get foo -- --output raw
```

## Load the context

Before starting an extension, `kongctl` writes a context file and sets:

> `KONGCTL_EXTENSION_CONTEXT=/path/to/context.json`

The file describes the matched command path, remaining arguments, selected
profile, resolved Konnect URL, output and log settings, extension data
directory, and host version. It does not contain secrets.

`invocation.original_args` preserves the invocation excluding the kongctl
executable, including aliases and original flag placement.
`matched_command_path.path` identifies the canonical contribution selected.
`invocation.remaining_args` is exactly the executable's argument list: host
flags, command-path tokens, and the first `--` separator have been removed.

Declared persistent extension flags retain their order across subcommands.
For example, `ai --context=first status --context second` delivers
`["--context=first", "--context", "second"]`. Tokens following `--` are
literal. In a subtree using persistent declarations, `ai -- status --help`
selects `ai` and delivers `["status", "--help"]`.

## Reuse authenticated access

A script extension can invoke `kongctl` as a child process:

```shell
kongctl get me
```

The child inherits the parent extension context. Go extensions can instead use
`github.com/kong/kongctl/pkg/sdk` to create an authenticated Konnect client and
render output using the parent's format settings.

Keep structured results on stdout and send diagnostics to stderr.
