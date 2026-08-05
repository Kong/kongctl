---
title: A Self-Service Workflow
summary: Let teams plan owned resources against a shared platform.
order: 6
related:
  - label: Federated AI Gateway example
    url: https://github.com/Kong/kongctl/tree/main/docs/examples/declarative/ai-gateway/federated
---

## Goal

You will recognize a practical workflow for centralized platform ownership
and team self-service.

## Platform team

The platform team defines and maintains shared resources, such as an AI
Gateway, model providers, and organization-wide defaults. It applies those
resources before dependent teams run their configurations.

## Peer team

Each peer team keeps its owned resources in a separate directory. It can
preview changes against the existing shared platform:

```shell
kongctl plan \
  -f docs/examples/declarative/ai-gateway/federated/external-peer-team
```

After reviewing the plan, the team can apply only that configuration:

```shell
kongctl apply \
  -f docs/examples/declarative/ai-gateway/federated/external-peer-team
```

This workflow gives the peer team a direct path to delivery while the platform
team continues to own shared infrastructure and organizational guardrails.
Repository review, Konnect permissions, and automated plan checks can enforce
the boundary around that self-service path.
