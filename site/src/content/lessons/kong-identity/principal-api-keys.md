---
title: Issue a Principal API Key
summary: >-
  Generate an agent credential, capture it privately, and distinguish it
  from identities.
order: 3
---

## Goal

Give `support-worker` a generated API key. An API key is a **credential** owned
by the principal, not another identity association. The directory can still
be externally managed. Complete [Principals and Identities](../principals-and-identities/)
first and stay in the same terminal and `identity-guide` directory.

## Add the declaration

Edit `principals.yaml`. Add this block directly under the agent principal
(the item with `ref: support-worker`), at the same indentation as `metadata`.
Do not add it to the human principal or the custom identity:

```yaml
api_keys:
  - ref: support-worker-primary
    type: v1
    name: Guide support automation key
    expires_at: null
```

`type: v1` is the generated key format, independent of the principal's
`type: agent`. Explicit `expires_at: null` means no expiration for this local
demonstration. Choose an expiration suitable for your real environment.

If you prefer a complete file, [download the updated principal manifest](../../examples/identity-guide/principals-with-key.yaml)
and save it as `principals.yaml` in this workspace. It contains both principals,
the custom identity and this key declaration. Preserve any description edits
you intentionally made in the previous lesson.

## Capture the generated material

```shell
mkdir -m 700 private
```

```shell
kongctl apply -f principals.yaml \
  --secret-output-dir "$PWD/private"
```

Expect one API-key creation. The result prints a private capture filename.
The plaintext key is returned only when issued; normal reads and dumps do
not recover it. Do not share the capture or commit `private/`. Record the
exact path printed by the command; do not select an arbitrary file with a glob.
If you repeat an unchanged apply, it should not issue another key.

Load it into your Bash environment without displaying it, replacing the path
with the file printed by apply:

```shell
export KEY_CAPTURE_FILE="$PWD/private/<capture-filename>.json"
```

```shell
export SUPPORT_API_KEY="$(python3 -c \
  'import json, os
with open(os.environ["KEY_CAPTURE_FILE"]) as capture:
    print(json.load(capture)["secret"])')"
```

```shell
test -n "$SUPPORT_API_KEY" && echo 'Agent key loaded'
```

Python 3 is required for this extraction. The capture envelope's `secret`
field holds the key. The command prints only the confirmation, not the key.
Keep this terminal open for the AI Gateway quickstarts; a new terminal will
need this variable set again.

## Inspect public metadata

```shell
kongctl get id principal api-keys \
  --principal-id "$SUPPORT_PRINCIPAL_ID" \
  -o json
```

```shell
kongctl diff --mode apply -f principals.yaml
```

Expect one key, a null expiration and no configuration changes. Key ownership
uses its generated management metadata and parent authority. A nonunique
human-facing key name is not the declarative lookup identity.

## Supply an existing secret instead

For `identity_principal_api_key`, use `type: imported` and a deferred source:

```yaml
api_keys:
  - ref: support-worker-imported
    type: imported
    name: Imported support automation key
    secret: !secret { source: !env SUPPORT_API_KEY }
    expires_at: null
```

Set the environment variable before execution. Existing deferred file sources
and parts expressions also work. Planning stores the expression without reading
material; apply resolves it before remote writes. Imported creation requires
neither `--write-secrets` nor `--secret-output-dir`.

The secret is create-only. Reapply preserves it; changing the source value does
not rotate it. Existing managed imported keys may omit `secret` for metadata
updates. Bulk `--write-secrets` skips existing material with a warning; explicit
`--write-secret support-worker-imported#secret` fails with rotation guidance.
To rotate, add a new key/ref, apply, switch applications, verify access, and
explicitly retire the old key through a reviewed scoped sync deletion.

Dumps include only creation type and public metadata. They can manage an
existing key, but recreation requires supplying `!secret` again. Imported keys
can belong to ordinary principals or external imported principals; those are
separate concepts. Imported group keys remain unsupported because the API
loses their initial name.

## Continue

The key is not useful against arbitrary services by itself. Continue to
[Authenticate AI Gateway with Kong Identity](../../quickstarts/identity-authentication/)
to configure a service that accepts it, or continue through the remaining
Identity lessons to prepare Token Vault first.
