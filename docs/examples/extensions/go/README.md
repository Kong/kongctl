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
kongctl get hello-go --example
```

The runtime invokes `kongctl get me` as a child process; the child command
inherits the parent command's output format.
