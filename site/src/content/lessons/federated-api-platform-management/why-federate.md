---
title: Why Federate?
summary: Balance a shared platform with independent team ownership.
order: 1
related:
  - label: Federated AI Gateway example
    url: https://github.com/Kong/kongctl/tree/main/docs/examples/declarative/ai-gateway/federated
---

## Goal

You will understand how federated management separates platform ownership
from the resources that product teams manage.

## One platform, many owners

A central platform team can provide shared capabilities, standards, and
guardrails without becoming the operator for every team-owned resource.
Product teams can then manage their resources through self-service workflows.

In the example used throughout this chapter:

- A platform team owns a shared AI Gateway and model providers.
- Peer teams own models, policies, consumers, agents, and other resources.
- Team configurations refer to the shared gateway instead of redefining it.

This creates a deliberate ownership boundary: the platform team manages the
shared foundation, while peer teams control the resources needed for their
work.

## What `kongctl` contributes

Declarative configuration gives each team a reviewable description of its
resources. Namespaces, references, external resources, and lookups let those
descriptions connect without requiring one team to own every configuration
file.

The rest of this chapter introduces each of those building blocks.
