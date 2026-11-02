# Token Vault public encryption keys

A directory has one Token Vault, configured with `vault.enabled` in YAML
(the API field is `vault_enabled`). Its public keys encrypt Vault data.
They are separate from API keys used to authenticate a
principal or group. This example configures public keys; it does not perform
runtime encryption or store the corresponding private keys.

The preview SDK and an environment exposing the Vault API are required.
US `.tech` supports the operations exercised here. Verify availability before
using a production environment.

## Use the default directory

Other Identity examples resolve the platform-provided `default` directory
externally. Vault keys have no `labels` or `managed_by`, so their writes instead
require explicit directory ownership. The directory's namespace and protection
govern its keys. A declared key collection supplies sync authority over that
collection, including keys registered outside KongCTL. Review existing keys
before claiming or synchronizing that collection.
Keys cannot carry independent team namespaces or protection settings.

Inspect the existing directory:

```sh
kongctl get identity directory default -o json
```

For an unmanaged directory, explicitly adopt it:

```sh
kongctl adopt identity directory default --namespace identity-directory
```

If the directory is already managed, use its existing namespace. A protected
directory requires a separate protection-only update before key updates or
deletion; do not combine unprotection with other changes. See the
[directory examples](README.md).

[vault-directory.yaml](vault-directory.yaml) explicitly enables Vault. It also
creates `default` if it is absent. Review its directory settings before use:

```sh
kongctl plan -f docs/examples/declarative/identity/vault-directory.yaml
kongctl apply -f docs/examples/declarative/identity/vault-directory.yaml
```

Existing disabled Vaults must be enabled in this separate step. The API hides
their children, so a plan cannot safely observe keys while proposing to enable
Vault in the same operation. A new directory can be created with explicit
`vault.enabled: true` and its keys in one plan.

## Register and inspect public keys

Replace [vault-public-key.pem](vault-public-key.pem) with your application's
public key. The checked-in fixture has no private counterpart in this repo.
Keep the actual private key with the application that needs it.

```sh
kongctl plan -f docs/examples/declarative/identity/vault-keys.yaml
kongctl apply -f docs/examples/declarative/identity/vault-keys.yaml
kongctl get identity vault keys --directory-name default -o json
```

Required fields are `ref`, `name`, and `public_key`. Supported material is one
PEM PKIX `PUBLIC KEY`: RSA of at least 2048 bits, or EC P-256/P-384/P-521.
Private keys, certificates, and PKCS#1 `RSA PUBLIC KEY` blocks are rejected.
Equivalent supported PEM whitespace, including CRLF, matches the same key.
Names contain 1 to 255 ASCII letters, digits, underscores or hyphens.

The API returns a generated route `id` and an RFC 7638 SHA-256 `kid`
fingerprint. The fingerprint identifies the key material. Names can repeat and
can be changed. The local `ref` is not stored remotely. Select a key by UUID or
exact fingerprint, including with an externally managed directory:

```sh
kongctl get identity vault key KEY_UUID --directory-name default -o text
kongctl get identity vault key --kid FINGERPRINT \
  --directory-name default -o yaml
```

Text summarizes identity, algorithm and size. JSON/YAML include the public
material and timestamps. Reads paginate the complete collection and verify
directory identity and the fingerprint against returned material.

## Rename, rotate, and synchronize

Changing `name` plans a PATCH against the same key. Changing `public_key` means
a different fingerprint and creates another key; the old key is preserved by
`apply`. `diff` shows the intended changes. Repeating an unchanged plan yields
zero changes.

`sync` prunes only an explicitly declared key collection. Omitting `vault.keys`
preserves existing keys; `vault.keys: []` explicitly retires all keys under that
managed directory. Grouped `identity.vault.keys` entries use a `directory`
selector referencing the declared managed directory. External directory
references never authorize key writes or sync.
A grouped `identity.vault.keys: []` has no directory selector; use a nested
empty collection to retire a directory's keys.

At most five keys can be registered. A rotation with space registers new
material before sync deletes old keys. At capacity, planning fails even if the
same sync would remove another key: explicitly select and retire a key first,
then add the new one. KongCTL does not pick an encryption key to delete for you.

The API specification states that deleting a key stops its use for new
encryptions and does not rewrite previously encrypted credentials. Existing
ciphertext recovery and private-key retention are application concerns; the
administrative E2E scenario does not prove runtime encryption behavior.

## Export and deletion

```sh
kongctl dump declarative --resources identity.directories \
  --include-child-resources --output-file directory-dump.yaml
kongctl plan -f directory-dump.yaml
kongctl scaffold identity.vault.keys
kongctl explain identity.directories.vault.keys
```

Public keys are exported only beneath an explicitly managed, enabled directory.
An unmanaged or disabled directory's key collection is omitted, not asserted
empty. Enable Vault before exporting its keys. Dumped public configuration can
be reloaded without changes; local refs may differ because keys have no ref
metadata field. Adoption is through the directory, not an individual key.

`delete -f vault-keys.yaml` also selects the declared directory for deletion.
Use an explicit collection sync when retiring keys while keeping the directory.
Directory deletion waits for its explicitly selected key deletions and then
requires fresh empty key/provider/Trusted IdP inventories. Additional
attachments block the directory deletion. Saved writes recheck current parent
identity, namespace, protection, enablement and key fingerprint. Observation
and deletion are not atomic; Q37 tracks the required backend contract.

See [directory deletion](directory-deletion.md) and the
[metadata ledger](../../../contributor/kong-identity-metadata-storage.md).
