# Breaking bug fix: preserve AI Gateway OIDC upstream token header settings

Fixes [kongctl issue #2765](https://github.com/Kong/kongctl/issues/2765).

For AI Gateway OpenID Connect auth strategies, kongctl no longer injects
`upstream_access_token_header: authorization:bearer` when the manifest omits
the field or explicitly sets it to null. Other OIDC fields retain their
existing SDK types, serialization, and defaults.

This changes behavior when creating or recreating a strategy from an existing
manifest that omits the field. With `hide_credentials: true`, an unconfigured
header allows the KOKO-4858 credential-hiding behavior instead of explicitly
requesting token forwarding. Upstreams that rely on forwarding must opt in:

```yaml
config:
  upstream_access_token_header: authorization:bearer
```

Updating an existing strategy still preserves its stored header when the
manifest omits the field. To clear it, explicitly declare:

```yaml
config:
  upstream_access_token_header: null
```

Review manifests produced by older `kongctl dump` or saved `get` output before
applying them after upgrading. Those clients could report
`authorization:bearer` even when Konnect stored no header or null. The updated
client reports the actual state, so applying an old manifest containing that
explicit string can now plan a change and enable forwarding. Re-dump with the
updated client, or remove an unintentionally injected field to preserve the
observed setting. Use null if the intent is to clear any stored header.

Because undeclared config fields are excluded from comparison, environments
with different stored header values can both show zero planned changes when
the manifest omits the field. Declare a string or null to manage it explicitly.
