---
title: Getting Started
summary: Create, plan, apply, and verify a basic AI Gateway.
order: 5
related:
  - label: AI Gateway declarative examples
    url: https://github.com/Kong/kongctl/tree/main/docs/examples/declarative/ai-gateway
  - label: Declarative configuration documentation
    url: https://developer.konghq.com/kongctl/declarative/
---

## Goal

You will create a basic AI Gateway configuration, review its plan, apply it,
and verify the result.

## Prerequisites

Complete [Konnect Authentication](../../installation/authenticate/) and make
sure the active profile can create AI Gateway resources.

## Create a working directory

Create and enter a directory for the configuration:

```shell
mkdir -p aigw
cd aigw
```

## Create the configuration

Copy this complete command into your terminal. The quoted heredoc writes the
YAML exactly as shown to `ai-gateway.yaml`:

```shell
cat > ai-gateway.yaml <<'YAML'
_defaults:
  kongctl:
    namespace: aigw-learning

ai_gateways:
  - ref: my-aigw
    name: my-aigw
    display_name: My AI Gateway
    proxy_urls:
      - host: aigw.example.com
        port: 443
        protocol: https
YAML
```

This command intentionally replaces `ai-gateway.yaml` inside the new tutorial
directory if the file already exists.

## Create and review a plan

Compare the configuration with live Konnect state and save an apply-mode plan:

```shell
kongctl plan \
  --mode apply \
  -f ai-gateway.yaml \
  --output-file plan.json
```

Review the planned operations without executing them:

```shell
kongctl diff --plan plan.json
```

## Apply the plan

The next command creates the AI Gateway after you confirm the plan:

```shell
kongctl apply --plan plan.json
```

## Verify

Read the new AI Gateway by its display name:

```shell
kongctl get ai-gateway "My AI Gateway" -o text
```
