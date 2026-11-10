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

| Command                    | Job                                            |
| -------------------------- | ---------------------------------------------- |
| `kongctl adopt`            | Sets namespace ownership metadata in Konnect. |
| `kongctl dump declarative` | Exports resource state as declarative YAML.    |

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
