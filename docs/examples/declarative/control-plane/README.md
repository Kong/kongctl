# Control Plane Example

This example demonstrates how to manage Konnect control planes with `kongctl`
declarative configuration. It shows how to provision them with different `kongctl`
metadata and configuration details.

## Files

- `control-plane.yaml` – defines a production control plane and a staging control plane. The
  example highlights:
  - `cluster_type` and `auth_type` fields for runtime configuration
  - optional `proxy_urls` entries that describe data plane endpoints
  - use of `kongctl` metadata for namespaces and protection settings
- `control-plane-group.yaml` – defines a control plane group and links member runtimes using the `members` list.

## Usage

Preview the change set:

```bash
kongctl diff -f control-plane.yaml
```

Apply the configuration:

```bash
kongctl apply -f control-plane.yaml
```

Run in sync mode to delete unmanaged control planes in the namespace (use with caution):

```bash
kongctl sync -f control-plane.yaml
```

Apply the control plane group example:

```bash
kongctl apply -f control-plane-group.yaml
```
