# Managing Gateway configuration with decK

[Declarative configuration](../declarative.md)

Read this page when orchestrating decK state through a control plane's `_deck`
block.

[Deck](https://developer.konghq.com/deck/) integration is configured on control planes via the `_deck` pseudo-resource. kongctl runs deck once per
control plane that declares `_deck`, then resolves external gateway services by
selector name. `_external.requires.deck` is not supported.

```yaml
control_planes:
  - ref: prod-cp
    name: "prod-cp"
    _deck:
      files:
        - "kong.yaml"
      flags:
        - "--select-tag=kongctl"

    gateway_services:
      - ref: billing-gw
        _external:
          selector:
            matchFields:
              name: "billing-service"
```

Important notes for deck integration:
- `_deck` is allowed only on control planes and only one `_deck` config is
  allowed per control plane.
- `_deck.files` must include at least one state file.
- `_deck.flags` can include additional deck flags (but not Konnect auth or
  output flags).
- `_external.selector.matchFields.name` is required for external gateway
  services and must be the only selector field.
- kongctl runs exactly one `deck gateway apply` or `deck gateway sync` per
  control plane that declares `_deck`.
- Deck state files should include `_info.select_tags` and matching `tags` on
  entities so `sync` does not delete resources owned by other deck files.
  kongctl does not inject select tags for you.
- Relative deck file paths are resolved relative to the declarative config file
  and must remain within the `--base-dir` boundary (default: the config file
  directory).
- When `_deck.files` is inherited from a template, its paths remain relative to
  the file containing the consuming control plane. Unlike a `!file` tag, a deck
  file path is not resolved in the template definition's context.
- Plan files store deck base directories relative to the plan file location.
  When emitting a plan to stdout, the base directory is made relative to the
  current working directory (use `--output-file` for portable plans). Applying a
  plan resolves them from the plan file directory (or the current working
  directory when using `--plan -`).
- `kongctl plan`/`diff` runs `deck gateway diff` to decide whether an external
  tool change is needed. `kongctl apply` runs `deck gateway apply` and
  `kongctl sync` runs `deck gateway sync`. For apply mode, deletes reported by
  deck diff are ignored.
- If the control plane is being created in the same plan (or the ID is not
  available), kongctl skips deck diff and includes the external tool step.
- For gateway steps, kongctl injects Konnect auth flags and output flags
  (`--json-output --no-color`); do not supply `--konnect-token`,
  `--konnect-control-plane-name`, `--konnect-addr`, or output flags yourself.
- Plans represent deck resolution targets explicitly via
  `post_resolution_targets` on the `_deck` change entry, including control plane
  identifiers and the gateway service selector.
