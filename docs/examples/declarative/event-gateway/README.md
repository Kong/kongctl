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
export ENV_STATIC_KEY_VALUE=$(
  printf '%s' 'your-secret-value' | openssl dgst -sha256 -binary | base64 -w0
)
```

See [static-key.yaml](static-key.yaml) for a minimal example that
contains the static key resource.

---

## How do I set a virtual cluster destination by reference?

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

---

## How do I expose a backend topic under an alias?

Use `topic_aliases` on an Event Gateway virtual cluster. Each alias
maps a client-visible topic name to the namespace-visible backend topic
name. Optional `condition` values are CEL expressions evaluated against
the connection auth context, and `conflict` controls how alias/topic name
collisions are handled.

```yaml
event_gateways:
  - ref: my-event-gateway
    name: my-event-gateway
    min_runtime_version: "1.2"
    virtual_clusters:
      - ref: public-orders-virtual-cluster
        name: public-orders-virtual-cluster
        destination:
          id: !ref orders-backend-cluster#id
        authentication:
          - type: anonymous
        namespace:
          mode: hide_prefix
          prefix: tenant-a.
        topic_aliases:
          - alias: public-orders
            topic: orders
            condition: "context.auth.type == 'anonymous'"
            conflict: warn
```

When a namespace is configured, `topic` uses the namespace-visible topic
name. In this example, clients use `public-orders` while the backend
topic remains under the `tenant-a.` namespace prefix. The Event Gateway
control plane is pinned to minimum runtime version `1.2` because topic
aliases require Event Gateway runtime 1.2 or later.

See [topic-aliases.yaml](topic-aliases.yaml) for a focused example.
