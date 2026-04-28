# Extension Examples

This directory contains small kongctl CLI extension examples.

- `script`: a shell-based debug extension
- `go`: a Go-based extension

Read the [extension builder guide](../../extensions.md) for manifest rules,
local development, release archive layout, GitHub installs, and upgrade
behavior.

Quick local link workflow:

```sh
kongctl link extension docs/examples/extensions/script
kongctl get extension kong/debug
kongctl get debug-info
```

The Go example must be built before linking:

```sh
cd docs/examples/extensions/go
go build -o bin/kongctl-ext-hello-go .
cd ../../../..
kongctl link extension docs/examples/extensions/go
kongctl get hello-go
```
