# Inspect OAuth clients and secrets

This branch supports `get` and `list` for authorization-server OAuth clients
and client-secret metadata. Client declarative lifecycle and secret creation
are still being implemented in #1401 and #1404. The existing
`auth-servers.yaml` example manages servers, scopes and claims; it does not
create OAuth clients yet.

Select the parent server explicitly. Client IDs are opaque strings, so a
positional argument always means an ID. Use `--client-name` for an exact,
case-sensitive name lookup; the lookup rejects duplicate matches.

```sh
kongctl get identity auth-server-clients --auth-server-name my-issuer
kongctl get identity auth-server-client CLIENT_ID \
  --auth-server-name my-issuer -o yaml
kongctl get identity auth-server-client --client-name my-service \
  --auth-server-name my-issuer
```

Read clients' grants, response types, redirect/login URIs, token durations,
scope policies, labels and endpoint authentication methods. Reads expose
the API's configuration; they do not establish that every grant is enabled
in the current environment. US `.com` rejected authorization-code grants
in the current probes (engineering question Q17).

Select a client by ID or exact name to inspect its secrets:

```sh
kongctl get identity auth-server-client-secrets \
  --auth-server-name my-issuer --client-name my-service
kongctl get identity auth-server-client-secret SECRET_UUID \
  --auth-server-id SERVER_UUID --client-id CLIENT_ID -o yaml
```

These commands show IDs, enabled state and timestamps. Secret values are
excluded in JSON, YAML and text output. The API returns a generated value
only at creation; an existing secret cannot be recovered with a read.
No credential is created, rotated, enabled, disabled or deleted by these
commands.

Clients have labels available for future namespace/protection storage.
Secret records have no labels, `managed_by`, or name, so there is no
independent declarative secret-slot identity. Explicit rotation operations
will require secure capture of newly generated material. The current API
permits two secret records total, including disabled records (Q16).

Both `get` and `list` traverse every page. Client traversal uses terminal
pages because the deployed API inflates totals for clients with multiple
allowed scopes (Q18). Failed pages never yield a partial inventory. Secret
lists currently report the expected total.
