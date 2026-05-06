# Debug Script Extension Example

This extension contributes `kongctl get debug-info` and
`kongctl print-debug-info`.

For the full extension builder guide, see
[docs/extensions.md](../../../extensions.md).

```sh
chmod +x docs/examples/extensions/script/kongctl-ext-debug
kongctl link extension docs/examples/extensions/script
kongctl get debug-info --example
kongctl print-debug-info --example
```

The runtime reads `KONGCTL_EXTENSION_CONTEXT` to find the generated
`context.json` file, prints the context path and context contents, and invokes
`kongctl get me` as a child process. The child command inherits the parent
command's output format.
