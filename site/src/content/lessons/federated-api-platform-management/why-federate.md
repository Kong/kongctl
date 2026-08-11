---
title: Federated Management
summary: Share a platform while teams manage their own AI Gateway resources.
order: 1
related:
  - label: Declarative configuration documentation
    url: https://developer.konghq.com/kongctl/declarative/
---

## Goal

You will understand the federated example that you will build throughout this
chapter.

## Targeted Governance

Federated management breaks up ownership of a platform enabling self-service
while maintaining central control over critical components.
A central team provides shared core components, capabilities and guardrails.
Satellite teams use those capabilities to manage the resources needed for
their work.

This chapter builds one shared AI Gateway with three owners:

| Owner       | Owns                                  |
| ----------- | ------------------------------------- |
| Platform    | The AI Gateway and an OpenAI Provider |
| Engineering | An Engineering Assistant Model        |
| Product     | A Product Assistant Model             |

The Platform team controls the shared foundation. Engineering and Product can
plan and apply their models without redefining or taking ownership of the AI
Gateway.

## Simple deployment to federated

You will first create all three configurations in a single unit and
then break them apart into satellite configurations

This is a small AI Gateway specific example, but the same ownership pattern
can extend across the entire Konnect API Platform of resources.
