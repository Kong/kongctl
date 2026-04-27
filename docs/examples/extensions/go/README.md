# Go Extension Example

This extension contributes `kongctl get hello-go`.

Build the runtime before installing or linking. `kongctl` does not compile
extension source during install.

```sh
cd docs/examples/extensions/go
go build -o bin/kongctl-ext-hello-go .
cd ../../../..
kongctl link extension docs/examples/extensions/go
kongctl get hello-go -- --example
```
