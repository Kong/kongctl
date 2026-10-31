# OAuth clients and secret rotation

`auth-server-clients.yaml` declares an authorization server, scope and OAuth
client. This branch supports the declarative client lifecycle, get/list and
adoption, plus explicit secret creation, enable/disable and deletion.

## Declare a client and capture its initial secret

```sh
kongctl plan -f auth-server-clients.yaml --mode apply > client-plan.json
kongctl diff --plan client-plan.json
kongctl apply --plan client-plan.json --auto-approve \
  --secret-output-dir ./private-client-secrets
kongctl plan -f auth-server-clients.yaml --mode apply
```

Creation requires `--secret-output-dir` on `apply` or `sync`. Every initial
credential gets a new `0600` JSON file reserved before any API mutations.
Text output reports paths; JSON/YAML includes `execution.generated_files`.
Only those private files contain secret values. They also identify the parent,
client ID, local ref and namespace. Store the value in the consuming
application's secret store. Updates and no-op runs need no output directory.
Output paths and credential values are absent from plans and manifests.

Clients require explicit `ref`, API `name`, `grant_types`, and
`response_types`; use `[]` for an empty required array. Client names are
unique within a server. Allowed scopes accept UUIDs or `!ref scope-ref#id`,
including scopes created in the same plan, and must belong to that server.
The example uses the verified `client_credentials` grant. Authorization-code
and refresh-token grants remain outside this public preview's enum; the
current deployment also rejects authorization-code creation (Q17).

Optional settings preserve their current value when omitted. Explicit empty
grant, response, redirect and allowed-scope arrays clear their lists.
`labels: {}` clears public labels while retaining ownership. Token durations
are seconds, between 60 and 2592000. Redirect and nonempty login URIs require
HTTPS; an empty login URI clears it. Ordinary PATCH updates do not write
`client_secret`. When the authentication method is omitted, every update
explicitly preserves its fresh existing value to work around API question Q15.

Clients store independent namespace/protection in labels. They can use a
namespace different from their server and can be nested under a declared
external server. Grouped `identity.clients` declarations require
`auth_server: server-ref`. An explicit `clients: []` scopes sync to the source
namespace; omission retains the collection. Sync never prunes other client
namespaces merely because they share a server. Protected clients require a
separate metadata-only unprotection before public changes or deletion.

Adopt an existing client before declaring it, selecting an opaque ID or exact
name under an explicit server:

```sh
kongctl adopt identity auth-server-client --client-name my-service \
  --auth-server-name my-issuer --namespace identity-client-example
```

Existing ownership requires `--overwrite-namespace`. Adoption preserves
configuration, protection, authentication method and credential material.
Dump includes safe client configuration and scope references:

```sh
kongctl dump declarative --resources identity.auth_servers \
  --filter-name my-issuer --include-child-resources > identity.yaml
```

## Inspect clients and secret metadata

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

Clients use labels for namespace/protection storage.
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
