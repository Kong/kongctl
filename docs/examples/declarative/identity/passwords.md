# Password creation and lifecycle

An orders worker has a declaratively managed principal and username, with
passwords enrolled separately. Apply [basic-auth.yaml](basic-auth.yaml) as
described in [the username tutorial](basic-auth.md). It references the existing
`default` directory without managing its settings. If the directory is absent,
follow the [bootstrap instructions](README.md) first.

Read the worker's principal UUID and use it below:

```sh
kongctl get identity principals --directory-name default
export PRINCIPAL_ID=REPLACE_WITH_WORKER_UUID
kongctl get identity principal-basic-auth \
  --directory-name default --principal-id "$PRINCIPAL_ID" \
  --username kongctl-example-order-worker -o json
```

The username carries KongCTL ownership in `managed_by`. These explicit
password commands do not add ownership metadata. They preserve any existing
password management map, including metadata belonging to another tool.

## Create and capture

Choose a private directory outside your repository and build artifacts.
Every creation requires a new output filename; existing files and symlinks
are refused before issuing a password.

```sh
mkdir -m 700 "$HOME/identity-orders-secrets"
kongctl create identity principal-basic-auth-password \
  --directory-name default --principal-id "$PRINCIPAL_ID" \
  --username kongctl-example-order-worker \
  --expires-at 2031-01-01T00:00:00Z \
  --secret-output-file "$HOME/identity-orders-secrets/worker-first.json"
```

Without a secret source, the API generates material. The new 0600 JSON file
contains `status: captured`, `directory_id`, `principal_id`, `basic_auth_id`,
`password_id` and `secret`. Normal command output and get/list contain only
public metadata. Transfer the value through your application's secret store;
do not commit the capture file.

Import a value from a private file, or name an environment variable using
`--secret-env VARIABLE_NAME`. The value is never a positional argument.

```sh
kongctl create identity principal-basic-auth-password \
  --directory-name default --principal-id "$PRINCIPAL_ID" \
  --username kongctl-example-order-worker \
  --secret-file "$HOME/identity-orders-secrets/import-source.txt" \
  --no-expiration \
  --secret-output-file "$HOME/identity-orders-secrets/worker-second.json"
kongctl get identity principal-basic-auth-passwords \
  --directory-name default --principal-id "$PRINCIPAL_ID" \
  --username kongctl-example-order-worker -o json
```

Prepare `import-source.txt` through your secret-management tooling before
running the second create. Both generated and imported passwords are captured.
Read/list cannot recover material later. The API permits two total password
records per username, including expired records; a third create fails until
an existing record is explicitly deleted. No command automatically frees a slot.

If a create fails after the request was attempted, retain the capture file
and inspect the selected password collection before retrying. A pending file
contains `status: creation_not_confirmed` and the parent IDs. The outcome
may be uncertain; no automatic second POST or rollback is attempted. Material
already captured is retained even if later metadata verification fails.

## Expiration, replacement and rotation

Use a password UUID from get/list or the capture's public `password_id`:

```sh
export PASSWORD_ID=REPLACE_WITH_PASSWORD_UUID
kongctl patch identity principal-basic-auth-password "$PASSWORD_ID" \
  --directory-name default --principal-id "$PRINCIPAL_ID" \
  --username kongctl-example-order-worker --no-expiration
kongctl patch identity principal-basic-auth-password "$PASSWORD_ID" \
  --directory-name default --principal-id "$PRINCIPAL_ID" \
  --username kongctl-example-order-worker --expires-at 2031-06-01T00:00:00Z
```

An expiration-only command sends an explicit timestamp or null through PATCH.
This avoids the deployed API's behavior of clearing an omitted expiration.
There is no enable/disable flag on this resource.

Supplying a secret source replaces material in the same record through PUT:

```sh
kongctl patch identity principal-basic-auth-password "$PASSWORD_ID" \
  --directory-name default --principal-id "$PRINCIPAL_ID" \
  --username kongctl-example-order-worker \
  --secret-file "$HOME/identity-orders-secrets/replacement.txt"
```

Prepare the replacement source first. Omitted expiration and the existing
management map are hydrated from a fresh GET and preserved. You can combine
replacement with an explicit expiration change. No generated replacement is
implicit, and PUT is attempted once. Public read-back verifies expiration and
metadata; it cannot prove which value authenticates. There is no new capture
file for replacement because the caller supplies the material.

For rotation using overlapping records, create a second password, deploy it,
verify authentication through your integration, then explicitly delete the
old record. Administrative E2E coverage does not establish runtime password
authentication: the tested runtime API requires an mTLS integration (Q30).

## Retire passwords before declarative teardown

```sh
kongctl delete identity principal-basic-auth-password "$PASSWORD_ID" \
  --directory-name default --principal-id "$PRINCIPAL_ID" \
  --username kongctl-example-order-worker
kongctl delete identity principal-basic-auth-password \
  --directory-name default --principal-id "$PRINCIPAL_ID" \
  --username kongctl-example-order-worker --all
kongctl delete -f basic-auth.yaml
```

Deletion uses the usual confirmation; add `--auto-approve` for unattended use.
`--all` explicitly deletes every password under the selected username,
including records without ownership metadata. It verifies an empty collection
afterward. Omit `--all` to retire only the selected UUID. A parent may also be
selected using `--basic-auth-id UUID`; `--username` always means an exact name.

Passwords are excluded from declarative dump, plan, apply, sync and adoption.
Adding or changing passwords leaves the username's declarative plan unchanged.
Deleting a username or principal refuses attached passwords. Remove them
explicitly before declarative teardown; the external directory is retained.
Metadata-only password ownership updates have no verified preserving API
request (Q35), so no declarative password lifecycle is claimed.
