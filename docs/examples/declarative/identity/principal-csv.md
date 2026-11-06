# Principal CSV import through declarative configuration

[principal-csv.yaml](principal-csv.yaml) reads [principals.csv](principals.csv)
under the existing directory named `default`. The directory stays external;
the CSV declares managed principals and supported nested resources. If default
is absent, use the separate [directory bootstrap](README.md) first.

Validate the CSV locally, without API calls or printing row values:

```sh
kongctl inspect identity principal csv \
  --file docs/examples/declarative/identity/principals.csv \
  --metadata-mode merge -o json
```

Inspection checks syntax, headers, duplicates, public value types and supported
nested shapes. Its counts have `remote_state_checked: false`. Planning also
checks directory references, current ownership, duplicate remote matches,
protection and optional UUID assertions.

```sh
kongctl plan --mode apply \
  -f docs/examples/declarative/identity/principal-csv.yaml \
  --output-file principal-csv-plan.json
kongctl diff --plan principal-csv-plan.json
kongctl apply --plan principal-csv-plan.json --auto-approve \
  --secret-output-dir ~/kongctl-private/principal-csv
kongctl plan --mode apply \
  -f docs/examples/declarative/identity/principal-csv.yaml
```

The example creates two principals, one custom identity, one generated key and
one basic-auth username record. Initial key material goes only to a new private
`0600` capture. Normal plan, diff, dump, inspection and errors contain no key
material. The basic-auth record has no password; use the existing explicit
password commands when needed. Replanning produces no changes.

Each CSV record represents one principal. A header named `ref` is required,
and every row must supply a valid unique local ref. Supported columns are:

| Column | Value |
| --- | --- |
| `ref` | Required local identity; matching uses directory, namespace and stored ref. |
| `id` | Optional existing-principal UUID assertion; it never authorizes creation or adoption. |
| `directory` | Declared directory ref, required when the CSV is used at `identity.principals` instead of beneath a directory. |
| `display_name`, `description` | Public strings. Quoted CSV cells can contain commas or newlines. |
| `type` | `user`, `agent`, or literal `null` for an explicitly unclassified principal. Type is create-only under the current API restriction. |
| `active` | Literal `true` or `false`; omission preserves current state. |
| `labels` | JSON object of public string labels. |
| `metadata` | JSON object using the supported public scalar/array value types. |
| `kongctl` | Optional JSON object for namespace/protection; otherwise YAML defaults apply. |
| `identities` | JSON array of ordinary declarative principal identities. |
| `api_keys` | JSON array of generated key declarations with explicit `type: v1`. |
| `basic_auths` | JSON array of basic-auth username declarations. |

Use normal CSV quoting: JSON quotes inside a quoted cell are doubled. Blank
cells omit a field and preserve its current value. An explicit `{}` is an
empty map and `[]` an empty child collection. JSON null is supported only where
the underlying declaration allows it, such as a generated key's expiration.
Duplicate headers, refs, asserted UUIDs and JSON object keys are rejected.
CSV errors report row/column context and withhold cell values.

The `!csv` mapping requires an explicit `metadata_mode`:

- `merge` adds/replaces supplied metadata keys and preserves unmentioned keys.
  Arrays replace the value at their key; they do not concatenate. A saved plan
  retains merge intent and combines the patch with fresh execution-time state.
  An empty object is a no-op for metadata.
- `sync` replaces the complete metadata map. An empty object removes all keys.
  Omitting a metadata cell preserves the map in either mode.

These are public metadata policies, separate from the `kongctl sync` resource
deletion mode. The same policies are available on YAML principals through
`metadata_mode`; existing YAML without it retains replacement behavior.
New plans use format `1.2` so older binaries reject merge instructions instead
of accidentally applying their patches as complete replacements. This binary
still reads ordinary `1.0` and `1.1` plans with their original behavior.

Names and display names are not lookup identities. If `id` is supplied, the
existing principal must already match the declared namespace/ref and that UUID.
A foreign or unowned principal requires the established explicit adoption
workflow first. Imported organization-chart principals remain outside ordinary
management, even if they have old ownership markers.

Child collections use the same ownership rules as YAML. Identities and
basic-auth username records have independent ownership; generated keys inherit
the managed principal's namespace/protection. A blank child cell preserves the
collection during sync, while `[]` explicitly scopes pruning of supported
managed children. Removing an entire principal row can retire that principal
only when existing descendant guards permit it; it does not silently revoke
unmentioned credentials. Review a sync plan before execution.

CSV values are literal data. YAML tags, template definitions and internal
reference encodings in cells do not become executable references or secret
sources. Reserved template-control keys and internal environment/secret
encodings are rejected. Use ordinary YAML declarations alongside the CSV for
cross-resource `!ref` bindings or deferred secret workflows. Imported API-key
material, passwords and ownership-map injection are rejected by the supported
schema.
The native organization-chart import has a separate CSV contract and lifecycle.

Paths are relative to the YAML source and use the same `--base-dir` boundary
and symlink checks as `!file`. Inputs must be regular local UTF-8 files within
the size limit. CSV is not fetched over HTTP. Dump emits ordinary declarative
YAML and public metadata, without CSV source bindings or private capture files.

```sh
kongctl dump declarative --resources identity.directories \
  --include-child-resources > principal-csv-dump.yaml
kongctl plan --mode apply -f principal-csv-dump.yaml
kongctl delete -f docs/examples/declarative/identity/principal-csv.yaml \
  --auto-approve
```

Retain the declared children in the CSV when deleting the complete example so
their explicit retirements can precede the principals. Cleanup preserves the
external default directory.
