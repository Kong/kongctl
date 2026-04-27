# Script Extension Example

This extension contributes `kongctl get hello-script`.

```sh
chmod +x docs/examples/extensions/script/kongctl-ext-hello-script
kongctl link extension docs/examples/extensions/script
kongctl get hello-script -- --example
```

The runtime reads `KONGCTL_EXTENSION_CONTEXT` to find the generated
`context.json` file.
