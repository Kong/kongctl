# Organization personal access tokens

Organization administrators can list personal access tokens (PATs) across an
organization for audits and offboarding:

```sh
kongctl get org pats
kongctl get org pats -o json
```

kongctl resolves the current organization through `/v3/organizations/me`.
Authentication must support that route; service-account authentication that
cannot call `/me` is not supported by this command.

Organization selection is unavailable while the API returns the same token
inventory for different organization IDs in the request path. For direct
requests, use `kongctl api` as shown below; the path ID should not be relied on
to select another organization's inventory.

The command fetches every page. Use `--page-size` to control the number of
records per request. Text output includes token ID, name, user ID, and state;
JSON and YAML output include the full token metadata, including expiration
and last-used timestamps. Token secrets are not returned by this API.

`kongctl list org pats` and the full `organization` spelling also work.
For an individual user's tokens, use `kongctl get pat --user-id <user-id>`.

## Direct API requests

`kongctl api` automatically routes known global API paths, including
organization PATs and PAT settings, users, teams, system accounts, roles,
identity providers, audit logs, and available regions, to
`https://global.api.konghq.com`:

```sh
kongctl api get /v3/organizations/<organization-id>/personal-access-tokens
```

Other paths retain the configured regional host. Global routing uses
`global.api.konghq.tech` when the resolved regional base URL uses
`konghq.tech`. Explicit `--base-url`, profile base URL settings, environment
base URL overrides, custom hosts, and absolute request URLs are preserved.
These base URL overrides also apply to `get org pats`.
