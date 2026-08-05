---
title: Local Development
summary: Link an extension working tree and test contributed commands.
order: 4
related:
  - label: Script extension example
    url: >-
      https://github.com/Kong/kongctl/tree/main/docs/examples/extensions/script
  - label: Go extension example
    url: https://github.com/Kong/kongctl/tree/main/docs/examples/extensions/go
---

## Goal

You will link a local extension and exercise its contributed command paths.

## Prepare the executable

`kongctl` does not compile extension source. The manifest's `runtime.command`
must already exist, stay inside the extension root, and be runnable.

For a script extension:

```shell
chmod +x ./my-extension/bin/kongctl-ext-foo
```

Build Go extension binaries before linking them.

## Link the working tree

Linking keeps the extension connected to its source directory, so edits are
available immediately:

```shell
kongctl link extension ./my-extension
```

Inspect the linked package, then test its commands:

```shell
kongctl get extension kong/foo
kongctl get foo --help
kongctl get foo
```

Replace `kong/foo` and `get foo` with the identity and command path in your
manifest.

## Test a managed install

Use a local install to test copying the package into the extension home:

```shell
kongctl install extension ./my-extension
kongctl list extensions
kongctl uninstall extension kong/foo
```

Linked and local-path extensions are not upgraded. Re-link or reinstall them
from their source directory.
