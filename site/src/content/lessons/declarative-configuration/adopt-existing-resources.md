---
title: Adopt Existing Resources
summary: Bring existing Konnect resources under declarative management.
order: 8
related:
  - label: Declarative configuration documentation
    url: https://developer.konghq.com/kongctl/declarative/
---

## Goal

You will understand how `adopt` and `dump declarative` cooperate to bring an
existing Konnect resource under declarative management.

## Two commands, two jobs

Resources created outside declarative configuration do not have a `kongctl`
namespace. The two commands handle separate parts of bringing them under
management:

| Command                    | Job                                           |
| -------------------------- | --------------------------------------------- |
| `kongctl adopt`            | Sets namespace ownership metadata in Konnect. |
| `kongctl dump declarative` | Exports resource state as declarative YAML.   |

Adoption establishes the resource's management scope. Dumping gives you the
desired-state configuration to maintain in version control.

## Adopt the resource

Identify the existing resource by ID or a supported name, then assign it to a
namespace. For an AI Gateway:

```shell label="Command syntax"
kongctl adopt ai-gateway <id-or-display-name> --namespace <namespace>
```

`adopt` preserves the resource's configuration and adds its
ownership metadata. It refuses to replace an existing namespace unless
you explicitly use `--overwrite-namespace`.

## Adopt independently owned children

AI Gateway child ownership is independent of its parent. Adopting a gateway
does not adopt its models, providers, policies, or other independently owned
children. Select each family explicitly:

```shell label="Run this..."
kongctl adopt ai-gateway models --name-pattern 'team-*' \
  --gateway-name shared-gateway --namespace team-a --dry-run -o json
```

`--dry-run` validates and reports ownership changes without writing. It is
available on every adoption command and supports text, JSON, and YAML. Its
report is separate from a declarative plan. Remove `--dry-run` to adopt the
selection. Use `--all` instead of `--name-pattern` to select every model within
that gateway, or provide an exact child name or ID.

Selection stays within one family and parent. All selected children are
validated before writes begin; API failures can leave a partially completed
batch. Adoption preserves configuration, protection, and credentials. It does
not adopt independently owned descendants. Unsupported credential-preserving
updates, including affected AppRole vaults and runtime TLS certificates, are
rejected or unavailable.

Kong Identity groups, OAuth clients, and basic-auth usernames also support
bulk selection within their parents. Principals and identities retain
individual UUID and explicit `--ref` requirements.

## Export exactly the adopted resources

Every adoption command supports `--declarative-output`, which writes only the
selected resources as declarative YAML using the same conversion capabilities
as `dump`. Parent adoption exports the parent itself; child adoption exports
the selection. This avoids exporting unrelated siblings and manually
reconstructing the adoption selection from a whole-gateway dump.

```shell label="Run this..."
kongctl adopt ai-gateway models --name-pattern 'legacy-*' \
  --gateway-name shared-gateway --namespace team-a \
  --parent-ref shared-gateway --declarative-output adopted-models.yaml

kongctl plan --mode apply \
  -f existing-gateway.yaml -f adopted-models.yaml
```

`--parent-ref` is the ref of the gateway declaration in
`existing-gateway.yaml`. The exported root-level models use
`ai_gateway: !ref shared-gateway#id`. Without this flag, the output includes an
external gateway declaration using its exact ID, so the file stands alone.
Referenced providers and other independently owned children are not adopted
or exported automatically.

Each adopted resource has explicit namespace/protection settings; no file-level
default is added. Write-only credentials and generated `managed_by` entries
are omitted. Your selected stdout format still applies to adoption results;
the output file is YAML and must not already exist.

With `--dry-run`, the file is clearly marked as proposed configuration and no
remote writes occur. Use a different file path when performing the adoption.
Batch API failures retain successfully adopted children in the output. File
failures identify adopted resources that could not be exported; review the
receipts before retrying.

## Dump its configuration

Use the resource ID to export only the adopted AI Gateway:

```shell label="Command syntax"
kongctl dump declarative \
  --resources ai_gateways \
  --filter-id <ai-gateway-id> \
  --default-namespace <namespace> \
  --output-file ai-gateway.yaml
```

Review the generated YAML before committing it. Dumped configuration can need
editing when an API does not return a field, including some secret values.

## Confirm the handoff

Plan from the dumped file and review the result:

```shell label="Run this..."
kongctl plan \
  --mode apply \
  -f ai-gateway.yaml \
  --output-file adoption-plan.json
kongctl diff --plan adoption-plan.json
```

An unchanged resource should produce a plan with no changes. Future edits to
`ai-gateway.yaml` can now be reviewed and applied through the normal
declarative workflow.
