# Identity Examples

These examples demonstrate how to manage Kong Identity resources with
`kongctl` declarative configuration.

The configuration uses the top-level `identity` key. Directory resources live
under `identity.directories` so future Kong Identity resources and child
resources can be added in the same folder and configuration group.

## Files

- `directories.yaml` - declares a Kong Identity directory available to all
  control planes with `allow_all_control_planes: true`.
- `directory-with-control-plane-ref.yaml` - declares a control plane and a
  Kong Identity directory whose `allowed_control_planes` list uses `!ref`.
  This requires an organization entitlement that supports restricted
  directories.

## Usage

Preview the change set:

```bash
kongctl diff -f directories.yaml
```

Apply the configuration:

```bash
kongctl apply -f directories.yaml --auto-approve
```

Check the created directories:

```bash
kongctl get identity directories -o yaml
```

Dump directories back to declarative configuration:

```bash
kongctl dump declarative --resources=identity.directories
```

Delete the example resources when you are done:

```bash
kongctl delete -f directories.yaml --auto-approve
```

## Notes

Use `allow_all_control_planes: true` for unrestricted gateway access to a
directory. Organizations with a one-directory quota must set
`allow_all_control_planes: true`. If your organization supports restricted
directories, set `allowed_control_planes` to a list of control plane IDs or
`!ref` references to declarative `control_planes`.

If set explicitly, `ttl_secs` and `negative_ttl_secs` must be between 300 and
86400 seconds.

Realm configuration is read-only in this release. It appears in imperative
detail output from `kongctl get identity directory <id|name>` but is not part
of declarative desired state.
