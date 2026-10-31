# OAuth clients and secret rotation

This branch supports `get` and `list` for authorization-server OAuth clients
and client-secret metadata, plus explicit secret creation, enable/disable and
deletion. Client declarative lifecycle and initial client-secret capture are
still being implemented in #1401. The existing
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
capture newly issued material in a new private file. The current API
permits two secret records total, including disabled records (Q16).

Both `get` and `list` traverse every page. Client traversal uses terminal
pages because the deployed API inflates totals for clients with multiple
allowed scopes (Q18). Failed pages never yield a partial inventory. Secret
lists currently report the expected total.

## Rotate a secret explicitly

Keep the existing working secret while issuing a replacement. Creation needs
an unused file path; the command reserves it with permissions `0600` before
POST. Normal JSON/YAML/text output contains metadata only.

```sh
kongctl create identity auth-server-client-secret \
  --auth-server-name my-issuer --client-name my-service \
  --enabled=false --secret-output-file ./issued-client-secret.json
```

The private JSON file contains `auth_server_id`, `client_id`, `secret_id`,
`status: captured` and `secret`. Store that value in the consuming
application's secret store. Enable the replacement using its metadata ID:

```sh
kongctl patch identity auth-server-client-secret NEW_SECRET_UUID \
  --auth-server-name my-issuer --client-name my-service --enabled=true
```

After testing authentication with the replacement and switching consumers,
disable or delete the old record explicitly:

```sh
kongctl patch identity auth-server-client-secret OLD_SECRET_UUID \
  --auth-server-name my-issuer --client-name my-service --enabled=false
kongctl delete identity auth-server-client-secret OLD_SECRET_UUID \
  --auth-server-name my-issuer --client-name my-service
```

Deletion uses the standard confirmation flow; `--auto-approve` skips it.
`patch` requires an explicit `--enabled` value. The `konnect` product form is
also supported, such as `kongctl create konnect identity ...`.

To import caller-supplied material, add `--secret-env VARIABLE_NAME` or
`--secret-file ./client-secret.txt` to the create command. These reuse the
deferred source resolver and require a non-empty string. A text file's
contents are used as supplied. The command still needs a new capture path.
No secret-value flag or normal stdout output is provided.

Two existing records occupy both slots even when disabled. The command does
not retire either record to make room. Inspect the inventory and explicitly
retire an unwanted record before creating another one.

## Failure recovery

Creation makes one attempt through both SDK and transport. A failed request
leaves a reserved JSON file with `status: creation_not_confirmed`. Inspect
remote secret metadata before retrying: the response may have been lost after
creation. The API cannot return an existing value on a later read.

If capture fails after creation, the error identifies the returned secret ID
when available. The file is retained for inspection; the command neither
repeats creation nor deletes the remote credential automatically. A missing
value produces `status: secret_missing`. Explicitly retire an unusable record
before issuing a replacement into a fresh output path. Q19 tracks the API's
recovery contract. Existing output paths, including symlinks, are refused.
