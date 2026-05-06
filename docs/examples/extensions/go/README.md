# Go Extension Example

This extension contributes `kongctl get hello-go`.

For the full extension builder guide, see
[docs/extensions.md](../../../extensions.md).

Build the runtime before installing or linking. `kongctl` does not compile
extension source during install.

```sh
cd docs/examples/extensions/go
go build -o bin/kongctl-ext-hello-go .
cd ../../../..
kongctl link extension docs/examples/extensions/go
kongctl get hello-go
kongctl get hello-go --output json --jq '{id, email}'
```

The runtime uses `github.com/kong/kongctl/pkg/sdk` to load the extension
runtime context, create an authenticated `sdk-konnect-go` client, and render
output using the parent command's output settings.
