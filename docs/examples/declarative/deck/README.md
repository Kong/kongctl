# Deck Integration Example

This example demonstrates the control-plane scoped `_deck` integration. kongctl runs deck once
per control plane that declares `_deck`, then resolves external gateway services by selector name.

## Files

- `control-plane.yaml` – creates a control plane, declares `_deck`, and defines an external
  gateway service selector.
- `gateway-services.yaml` – decK state file for the gateway service. Uses `_info.select_tags`
  to scope sync operations.
- `api.yaml` – defines an API implementation that references the gateway service created by decK.

## Usage

Prerequisites:
- `deck` is installed and on your PATH.
- You have Konnect credentials configured (for example `KONGCTL_DEFAULT_KONNECT_PAT` or `kongctl login`).

Preview the plan:

```bash
kongctl plan --mode apply -f control-plane.yaml -f api.yaml
```

Apply the configuration:

```bash
kongctl apply -f control-plane.yaml -f api.yaml
```

Run in sync mode to delete managed resources in the namespace (use with caution):

```bash
kongctl sync -f control-plane.yaml -f api.yaml
```

## Notes

- `gateway-services.yaml` must remain in the same directory as `control-plane.yaml`, or update the
  `_deck.files` path accordingly. Paths are resolved relative to the config file.
- The `_info.select_tags` section is required to prevent deck `sync` from deleting resources that
  are managed by other decK files.
- If your control plane already exists, you can mark it as external by adding an `_external` block
  and selector to `control-plane.yaml`.
