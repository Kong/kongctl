---
title: Ownership Boundaries
summary: Organize team-owned resources without losing shared guardrails.
order: 2
related:
  - label: Declarative configuration documentation
    url: https://developer.konghq.com/kongctl/declarative/
---

## Goal

You will identify where team ownership appears in a federated declarative
configuration.

## Separate configurations by owner

Keep resources near the team that changes and reviews them. A simple
repository layout makes the boundary visible:

```text
federated/
├── central-team/
│   └── ai-gateway.yaml
└── peer-team/
    ├── support-model.yaml
    └── support-policy.yaml
```

The directory is an organizational boundary. The declarative namespace tells
`kongctl` which managed resources belong to a reconciliation scope:

```yaml
_defaults:
  kongctl:
    namespace: federated-ai-gateway
```

Labels can record additional ownership information in Konnect:

```yaml
labels:
  team: support-experience
  ownership: peer
```

Namespaces control declarative planning scope; they do not grant permissions.
Use Konnect authorization to control what each team is allowed to change.
