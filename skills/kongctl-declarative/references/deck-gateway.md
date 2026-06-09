# decK Gateway Integration

Use this file when the user wants Kong Gateway runtime configuration:
services, routes, plugins, consumers, upstreams, or Gateway config generated
from OpenAPI. kongctl manages Konnect SaaS resources; decK manages Kong
Gateway state. Combine them with `control_planes[]._deck`.

## Decision Rule

- Use kongctl `apis`, `portals`, and `api.versions` for API catalog,
  portal publication, and spec storage in Konnect.
- Use decK state files for Gateway services, routes, plugins, consumers,
  consumer groups, upstreams, targets, and other runtime entities.
- Use both when an OpenAPI spec should appear in the Konnect API catalog
  and configure an actual Gateway route/service/plugin implementation.

## Preconditions

- Confirm decK is installed: `deck version`.
- For CI, install both tools:
  - `kong/setup-kongctl@v1`
  - `kong/setup-deck@v1`
- Use `deck file ... --help` for APIOps helper syntax when uncertain.

## `_deck` Integration Pattern

Declare `_deck` on a control plane. kongctl runs decK once per control plane
that declares `_deck`, then resolves external gateway services by selector.

```yaml
control_planes:
  - ref: example-cp
    name: "example-cp"
    cluster_type: "CLUSTER_TYPE_SERVERLESS_V1"
    _deck:
      files:
        - "gateway.yaml"
      flags:
        - "--analytics=false"

    gateway_services:
      - ref: example-gw-svc
        _external:
          selector:
            matchFields:
              name: "example-service"

apis:
  - ref: example-api
    name: "Example API"
    implementations:
      - ref: example-api-impl
        service:
          control_plane_id: !ref example-cp#id
          id: !ref example-gw-svc#id
```

Rules:

- `_deck` is allowed only on `control_planes`.
- `_deck.files` must include at least one decK state file.
- `_deck.flags` may contain extra decK flags, but not Konnect auth or output
  flags. Do not set `--konnect-token`, `--konnect-control-plane-name`,
  `--konnect-addr`, `--json-output`, or `--output`; kongctl injects those.
- Relative `_deck.files` paths resolve from the kongctl config file that
  declares `_deck`, and must stay inside the `--base-dir` boundary.
- `gateway_services` that point at decK-created services must use
  `_external.selector.matchFields.name`; match the service `name` in the
  decK state file.
- In `plan`/`diff`, kongctl runs `deck gateway diff`. In `apply`, kongctl
  runs `deck gateway apply`. In `sync`, kongctl runs `deck gateway sync`.

## decK State File Requirements

Always scope decK state before using sync. Include `_info.select_tags` and
matching `tags` on generated entities so `deck gateway sync` does not delete
resources owned by other decK files.

```yaml
_format_version: "3.0"

_info:
  select_tags:
    - "payments"

services:
  - name: payments-service
    url: https://payments.example.com
    tags:
      - payments
    routes:
      - name: payments-route
        paths:
          - /payments
        tags:
          - payments
```

## OpenAPI to Gateway APIOps

For prompts like "set up an API Gateway from this OpenAPI spec":

1. Keep the OpenAPI file in its existing repository location.
2. Generate decK state from the spec:
   ```bash
   deck file openapi2kong \
     --spec <openapi.yaml> \
     --select-tag <tag> \
     --output-file <gateway.yaml>
   ```
3. Inspect the generated service and route names.
4. Ensure `_info.select_tags` and entity `tags` match the chosen tag.
5. Patch operational defaults when needed:
   ```bash
   deck file patch \
     --state <gateway.yaml> \
     --selector '$..services[*]' \
     --value 'read_timeout:30000' \
     --output-file <gateway-patched.yaml>
   ```
6. Add plugins when needed:
   ```bash
   deck file add-plugins \
     --state <gateway.yaml> \
     --selector '$..services[*]' \
     --config '{"name":"key-auth"}' \
     --output-file <gateway-plugins.yaml>
   ```
7. Add or normalize tags when needed:
   ```bash
   deck file add-tags \
     --state <gateway.yaml> \
     --selector 'services[*]' \
     --output-file <gateway-tagged.yaml> \
     <tag>
   ```
8. Add `_deck.files` to the target control plane.
9. Add `control_planes[].gateway_services` external selectors for any
   generated Gateway services that Konnect API implementations must reference.
10. Add `apis[].implementations[].service` with `control_plane_id` pointing
    to the control plane and `id` pointing to the external gateway service.
11. Validate with `kongctl diff -f <resources> --mode apply -o text`.

Prefer writing each APIOps command to a new output file, then promote the
final state file after inspection. When a repository already has scripts for
OpenAPI-to-decK generation, update those scripts instead of replacing them.

## Repository Example

When working inside the `kongctl` repository, use this concrete example:

- `docs/examples/declarative/deck/control-plane.yaml`
- `docs/examples/declarative/deck/gateway-services.yaml`
- `docs/examples/declarative/deck/api.yaml`

The same pattern applies in other repositories even when the paths differ.
