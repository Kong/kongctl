# Go Extension Example

This extension contributes `kongctl get hello-go` and `kongctl get hello-go me`.

For the full extension builder guide, see
[docs/extensions.md](../../../extensions.md).

Build the runtime before installing or linking. `kongctl` does not compile
extension source during install.

```sh
cd docs/examples/extensions/go
CGO_ENABLED=0 go build -o bin/kongctl-ext-hello-go .
cd ../../../..
kongctl link extension docs/examples/extensions/go
kongctl get hello-go
kongctl get hello-go --output json --jq '{id, email}'
kongctl get hello-go --label example me
kongctl get hello-go me --label=example
```

The runtime uses `github.com/kong/kongctl/pkg/sdk` to load the extension
runtime context, create an authenticated `sdk-konnect-go` client, and render
output using the parent command's output settings.

The parent declares a persistent string flag, `label`, which the `me`
subcommand inherits. Both placements above produce the same display label.
The Go runtime parses `invocation.remaining_args` with `flag.FlagSet`.
Repeated labels retain their original order, so Go's parser uses the last
value. The label appears in the text display record; JSON and YAML output
continue to show the raw user response. Run `kongctl get hello-go me --help`
to see the inherited declaration.
