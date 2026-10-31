# Kong Identity directories

These examples require the Kong Identity SDK preview used by this branch and
access to the regional directory administration API.

Most users can reference an existing directory, commonly named `default`.
Start with `external-default.yaml`: it resolves the directory by its exact
name, without creating or modifying it. An explicit ID is also supported.
KongCTL does not create an implicit default directory or select one silently.
Other Kong Identity resource types will be added separately.

`directories.yaml` manages a directory's configuration. Run this example
separately from `external-default.yaml`. Creating another directory requires
available organization quota. The API requires `allow_all_control_planes:
true` for organizations with a one-directory quota.

```sh
kongctl get identity directories
kongctl get identity directory default -o json
kongctl plan -f external-default.yaml
kongctl plan -f directories.yaml --mode apply > plan.json
kongctl apply --plan plan.json --auto-approve
kongctl dump declarative --resources identity.directories
```

To manage an existing directory, explicitly adopt it into your namespace
before applying a managed declaration with the same name:

```sh
kongctl adopt identity directory default --namespace identity-team
```

Adoption preserves its settings, public labels, and protection status. It
replaces `managed_by` with KongCTL's ownership and version metadata. Use
`--overwrite-namespace` only when intentionally taking over existing namespace
ownership. Adoption alone does not alter directory configuration.

Each managed directory requires an explicit API `name` in addition to its
local `ref`. The API name is unique in its organization and region. KongCTL
matches that name and checks ownership locally after reading every list page.
It stores `tool`, `version`, `kongctl-namespace`, and `kongctl-protected` in
`managed_by`. Do not supply `managed_by` in a manifest. Public `labels` remain
independent of ownership metadata.

All mutable API settings are supported: `description`,
`allow_all_control_planes`, `allowed_control_planes`, `ttl_secs`,
`negative_ttl_secs`, `vault_enabled`, and `labels`. Cache TTLs range from 300
through 86400 seconds. Omitted settings are preserved during updates; an
explicit empty description, labels map, or control-plane list clears that
setting. Omitted settings on creation use the documented API defaults.
Updates use a fresh read and full replacement because PATCH cannot update
`description` or `managed_by`.

The preview specification exposes `vault_enabled`, but the US `.com` API
currently accepts `true` and returns `false`. KongCTL reports that mismatch
as an error. Keep it false until Token Vault is available in your environment;
the directory lifecycle scenario verifies false, while unit coverage verifies
that a true request is sent and an ignored value is reported.

`scoped-control-plane.yaml` shows restricted access with a control plane
reference. Use UUIDs or `!ref gateway#id`, including references to external
control planes. New control planes are created before directories that
reference them. Apply each example separately.

Protection follows the usual KongCTL lifecycle. Set `kongctl.protected` to
false and apply that change separately before changing protected settings or
deleting the directory. A protected resource cannot be changed and
unprotected in the same operation.

`sync` only prunes directories when `identity.directories` is explicitly in
scope. To remove every managed, unprotected directory in this namespace:

```yaml
_defaults:
  kongctl:
    namespace: identity-team
identity:
  directories: []
```

Omitting the collection leaves directories alone during sync. External,
foreign-namespace, and unowned directories are not pruned. `delete -f` can
delete explicitly declared managed directories. KongCTL never requests
forced cascading deletion; the API must permit deletion without losing
descendant resources. Principals, credentials, groups, Token Vault children,
and other directory children are outside this first implementation.
