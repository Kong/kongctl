# Debug Script Extension Example

This extension contributes `kongctl get debug-info` and
`kongctl print-debug-info`, including its `status` subcommand.

For the full extension builder guide, see
[docs/extensions.md](../../../extensions.md).

```sh
chmod +x docs/examples/extensions/script/kongctl-ext-debug
kongctl link extension docs/examples/extensions/script
kongctl get debug-info --example
kongctl print-debug-info --example
kongctl print-debug-info --context example status
kongctl print-debug-info status --context=example --verbose=false
kongctl print-debug-info --context=first status --context second
```

The runtime reads `KONGCTL_EXTENSION_CONTEXT` to find the generated
`context.json` file, prints the context path and context contents, and invokes
`kongctl get me` as a child process. The child command inherits the parent
command's output format.

`context` and `verbose` are declared persistent flags. The script prints
their raw tokens so you can compare `invocation.original_args`,
`invocation.remaining_args`, and process arguments. The repeated-context
example preserves both occurrences in order. These example flags are for
inspection; they do not change the reentrant call's configuration.

Run `kongctl print-debug-info status --help` to see inherited declarations.
Run `kongctl print-debug-info -- status --help` to forward literal tokens
to the parent contribution without selecting the `status` subcommand.
