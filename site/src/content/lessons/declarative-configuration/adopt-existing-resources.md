---
title: Adopt Existing Resources
summary: Bring existing Konnect resources under declarative management.
order: 7
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
| `kongctl adopt`            | Adds the `KONGCTL-namespace` label in Konnect. |
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
`KONGCTL-namespace` label. It refuses to replace an existing namespace unless
you explicitly use `--overwrite-namespace`.

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
