# Event Gateway Examples — FAQ

## How do I load a static key value from an environment variable?

Use the `!env` YAML tag to inject a secret from the environment at
apply time rather than hardcoding it in the manifest.

```yaml
static_keys:
  - ref: default-static-key
    name: default-static-key
    description: "Default Static Key"
    value: !env "ENV_STATIC_KEY_VALUE"
```

`ENV_STATIC_KEY_VALUE` must be set in the shell when you run
`kongctl apply`. The value should be a 256-bit (32-byte)
base64-encoded string.
```
export ENV_STATIC_KEY_VALUE=$(printf '%s' 'your-secret-value' | openssl dgst -sha256 -binary | base64 -w0)
```

See [static-key.yaml](static-key.yaml) for a minimal example that
contains the static key resource.

---

## How do I set destination using reference for backend cluster's ID in a virtual cluster?

Use the `!ref` YAML tag with the syntax `<ref-name>#<field>` to pull
a field from another resource in the same manifest at plan/apply
time.

```yaml
virtual_clusters:
  - ref: default-virtual-cluster
    name: default-virtual-cluster
    destination:
      id: !ref default-backend-cluster#id
```

`default-backend-cluster` is the `ref` value assigned to the backend
cluster resource elsewhere in the manifest. The `#id` fragment tells
kongctl to substitute the resolved Konnect ID of that resource once
it has been created or looked up.

See [event-gateway.yaml](event-gateway.yaml) for the full example.
