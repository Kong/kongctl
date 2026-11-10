# Adopt existing resources

`kongctl adopt` assigns ownership to an existing resource without recreating
it or reconciling its public configuration. Every command requires an explicit
`--namespace`. Existing ownership requires `--overwrite-namespace`, even when
it already names the requested namespace. Adoption preserves protection.

## Inspect an adoption without writing

Every adoption command accepts `--dry-run`. It defaults to `false`.
Dry runs perform the same selection and preflight validation, including
ownership conflicts and credential-preservation restrictions, without writes:

```sh
kongctl adopt ai-gateway models existing-model \
  --gateway-name shared-gateway --namespace team-a --dry-run

kongctl adopt ai-gateway models --all \
  --gateway-name shared-gateway --namespace team-a --dry-run -o json
```

Text output identifies each resource and parent and shows ownership metadata
changes. JSON and YAML output use an adoption-specific report:

```json
{
  "dry_run": true,
  "changes": [
    {
      "resource_type": "ai_gateway_model",
      "id": "5078f559-063b-4f10-9098-9e6553b21d9b",
      "name": "existing-model",
      "namespace": "team-a",
      "parent": {"gateway_id": "4078f559-063b-4f10-9098-9e6553b21d9b"},
      "metadata_location": "managed_by",
      "before": {},
      "after": {
        "tool": "kongctl",
        "version": "dev",
        "kongctl-namespace": "team-a",
        "kongctl-protected": "false"
      }
    }
  ]
}
```

`version` reflects the executing binary. The report contains ownership
metadata only; public configuration and credentials are excluded. For label
ownership, only reserved `KONGCTL-*` entries appear. `before` and `after` show
complete ownership maps, including unchanged protection. Text shows changed
entries. This report is separate from a declarative plan and cannot be passed
to `apply` or `sync`. Remove `--dry-run` to perform the adoption, which observes
and validates the resources again.

## Export adopted resources as configuration

Every adoption command accepts `--declarative-output PATH` to export exactly
the selected resources as declarative configuration. This includes portals,
APIs, control planes, AI Gateways, Event Gateway control planes, dashboards,
application auth strategies, DCR providers, organization teams, Identity
directories and auth servers, and all supported child adoption commands.
It uses the same API-to-declaration conversion capabilities as `dump`.

The file contains explicit namespace/protection metadata on each adopted
resource, with no file-level defaults. It omits generated `managed_by` values,
write-only credentials, siblings, and independently owned descendants.
Other referenced resources are not implicitly adopted or declared.

For example, adopt an existing gateway and generate its declaration:

```sh
kongctl adopt ai-gateway shared-gateway --namespace team-a \
  --declarative-output adopted-gateway.yaml
```

This exports the gateway itself. Its models, providers, and other children
require separate adoption commands. Control-plane group exports preserve the
current member IDs without adopting those member control planes.

To export a filtered child selection:

```sh
kongctl adopt ai-gateway models --name-pattern 'legacy-*' \
  --gateway-name shared-gateway --namespace team-a \
  --declarative-output adopted-models.yaml
```

For child adoption without `--parent-ref`, the file includes an external
parent declaration using the selected parent's exact remote ID. The parent
is referenced, not adopted
or reconciled. For Identity children under principals, the output also includes
the external directory needed to resolve that principal.

If your existing inputs already declare the parent, supply its local ref:

```sh
kongctl adopt ai-gateway models --name-pattern 'legacy-*' \
  --gateway-name shared-gateway --namespace team-a \
  --parent-ref shared-gateway --declarative-output adopted-models.yaml

kongctl plan --mode apply \
  -f existing-gateway.yaml -f adopted-models.yaml
```

This emits `ai_gateway: !ref shared-gateway#id` on the exported models and
omits the parent declaration. `--parent-ref` names the existing declaration;
it does not select the remote parent or load existing files. The normal
`--gateway-id`/`--gateway-name` selectors still identify the remote gateway.
Include a declaration for that same gateway when loading the files together,
or incorporate the generated child entries into your existing inputs.
A missing ref fails normal reference validation during loading/planning.

For Identity adoption, `--parent-ref` names the nearest parent: a directory
for groups/principals, a principal for identities/basic-auth usernames, or an
Auth Server for OAuth clients. Persisted principal and identity refs retain
the explicitly supplied `--ref`. Other child refs use the existing remote IDs.
`--parent-ref` is available only on child adoption commands. Do not duplicate
declarations across your inputs.

The destination must be a new file in an existing directory. Existing files
and symlinks are rejected before resource mutation, including during dry runs.
There is no automatic editing or overwriting of existing configuration.
Stdout retains the usual adoption receipts or dry-run report in the selected
`-o` format; the configuration file is always YAML.

To preview both ownership changes and the generated configuration:

```sh
kongctl adopt ai-gateway models --name-pattern 'legacy-*' \
  --gateway-name shared-gateway --namespace team-a --dry-run \
  --parent-ref shared-gateway --declarative-output proposed-models.yaml
```

The file begins with a `DRY RUN` comment identifying proposed configuration.
No remote resources are changed. Use a different destination, such as
`adopted-models.yaml`, when performing the actual adoption. A matching plan
should recognize the adopted objects without recreation; existing credentials
remain in place because the exported inputs do not select secret writes.

The whole selection and its declarative projections are validated before the
first resource write. A failed preflight removes the empty reserved file.
After each confirmed adoption, kongctl updates the output with confirmed
successes only. If a later API write fails, the file retains those successes.
If a file write fails after adoption, the error identifies the adopted object
and destination, and stdout still reports that successful adoption. The last
complete output snapshot is retained. Inspect receipts and remote state before
retrying; there is no rollback of already completed adoptions.

## AI Gateway children

Adopting an AI Gateway updates its own namespace label. It does not adopt any
children. These child subcommands establish independent ownership:

| Subcommand | Resource |
| --- | --- |
| `models` | Models |
| `model-providers` | Model providers |
| `auth-strategies` | Authentication strategies |
| `policies` | Policy instances |
| `custom-policies` | Custom policy definitions |
| `agents` | Agents |
| `mcp-servers` | MCP servers |
| `vaults` | Vaults |
| `consumers` | Consumers |
| `consumer-groups` | Consumer groups |
| `ca-certificates` | CA certificates |
| `snis` | SNIs |

Select an explicit gateway with `--gateway-id` or `--gateway-name`. Names are
matched exactly within that gateway and resource family. Singular command
aliases match the corresponding `get ai-gateway` aliases.

```sh
# Adopt one model by exact name.
kongctl adopt ai-gateway models existing-model \
  --gateway-name shared-gateway --namespace team-a

# Adopt one model by UUID, avoiding ambiguity with a UUID-shaped name.
kongctl adopt ai-gateway models \
  --model-id 5078f559-063b-4f10-9098-9e6553b21d9b \
  --gateway-name shared-gateway --namespace team-a

# Adopt every model within that gateway.
kongctl adopt ai-gateway models --all \
  --gateway-name shared-gateway --namespace team-a

# Adopt models matching a name glob; quote it to prevent shell expansion.
kongctl adopt ai-gateway models --name-pattern 'team-*' \
  --gateway-name shared-gateway --namespace team-a
```

Choose exactly one identifier, exact child ID/name selector flag, `--all`, or
`--name-pattern`. Globs support `*`, `?`, and character classes such as `[ab]`;
they match the full resource name, not the display name. A missing match or
ambiguous exact identifier fails. `--all` and patterns select only one family
under one parent; run separate commands for other families.

Each adopted child receives generated `managed_by` entries: `tool`,
`version`, `kongctl-namespace`, and `kongctl-protected`. Kongctl replaces the
complete tool-owned map, retaining existing protection and public labels.
The parent, siblings outside the selection, independently owned descendants,
consumer credentials, and group membership are unchanged.

Runtime TLS certificates cannot be adopted because their API cannot safely
preserve private keys during metadata updates. Config stores, their secrets,
and data plane certificates are excluded. Credentials inherit consumer
ownership and have no independent adoption. HashiCorp AppRole vault adoption
fails before writing when GET redacts `secret_id` and no `secret_id_file` is
configured. Adoption does not resubmit or rotate credentials.

## Kong Identity children

Existing Identity adoption paths and aliases remain available. Ordinary
groups, OAuth clients, and basic-auth usernames also support `--all` and
`--name-pattern` under their existing explicit parent selectors. For
basic-auth records, name patterns match usernames.

```sh
kongctl adopt identity groups --name-pattern 'team-*' \
  --directory-name default --namespace team-a

kongctl adopt identity auth-server clients --all \
  --auth-server-name shared-issuer --namespace team-a

kongctl adopt identity principal basic-auths --all \
  --directory-name default \
  --principal-id 5078f559-063b-4f10-9098-9e6553b21d9b --namespace team-a

kongctl adopt identity principals \
  5078f559-063b-4f10-9098-9e6553b21d9b \
  --directory-name default --ref employee --namespace team-a

kongctl adopt identity principal identities \
  6078f559-063b-4f10-9098-9e6553b21d9b \
  --directory-name default \
  --principal-id 5078f559-063b-4f10-9098-9e6553b21d9b \
  --ref employee-login --namespace team-a
```

Principals and identities require individual UUID selection and an explicit
persisted `--ref`. They do not support bulk selection. Duplicate namespace/ref
matches are rejected. Native org-chart imported principals remain controlled
by their importer. Existing unsupported identity association and team-group
boundaries remain enforced.

OAuth clients use reserved labels; the other supported Identity children use
`managed_by`. Adoption preserves the OAuth authentication method and existing
client secrets. Basic-auth adoption selects the username record and preserves
its password children. API keys, passwords, client secrets, and children that
inherit ownership do not gain independent adoption.

## Batch behavior and declarative handoff

Every selected child is read and validated before the first write. A known
ownership conflict or unsupported operation aborts the whole selection before
mutation. Writes then execute individually; the API does not provide an
atomic batch transaction. If a write fails, kongctl stops, prints completed
adoptions, and reports the failing resource. A failing request may have
reached the API. Inspect current state before retrying; successful adoptions
will require `--overwrite-namespace` on a repeated selection.

Adoption does not create, delete, prune, or reconcile payloads. After adopting,
dump the configuration, review its namespace and protection, and generate a
fresh declarative plan. A matching declaration recognizes the existing child
by its original identity within the selected parent. Declare existing shared
parents as external resources when only their children should be managed.
