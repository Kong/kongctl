---
title: Working Example
summary: Work through the complete declarative lifecycle of an AI Gateway.
order: 5
related:
  - label: AI Gateway declarative examples
    url: https://github.com/Kong/kongctl/tree/main/docs/examples/declarative/ai-gateway
  - label: Declarative configuration documentation
    url: https://developer.konghq.com/kongctl/declarative/
---

## Goal

You will work through the complete declarative lifecycle of an AI Gateway:
create, inspect, update, reconcile, and delete.

## Prerequisites

Complete [Konnect Authentication](../../installation/authenticate/) and make
sure the active profile can create _AI Gateway 2.0_ resources.

## Create the configuration

Create and enter a working directory:

```shell label="run this..."
mkdir -p aigw && cd aigw
```

Write the desired state to `ai-gateway.yaml`:

```shell label="run this..."
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

The quoted heredoc writes the YAML exactly as shown. It replaces an existing
`ai-gateway.yaml` in this tutorial directory.

## Apply and inspect

Preview what an `apply` will do:

```shell label="run this..."
kongctl diff --mode apply -f ai-gateway.yaml
```

You should see the planned AI Gateway create:

```text label="diff output example"
Plan: 1 to add, 0 to change

=== Namespace: aigw-learning ===
+ [1:c:ai_gateway:my-aigw] ai_gateway "my-aigw" will be created
  name: "my-aigw"
  display_name: "My AI Gateway"
  proxy_urls: [{aigw.example.com 443 https}]
  protection: disabled
```

Apply the changes. Internally, this recreates the plan and then asks for
confirmation before executing it:

```shell label="apply the configuration"
kongctl apply -f ai-gateway.yaml
```

You should see similar output on successful execution:

```text label="applied changes"
...
Executing changes:
[1/1] [namespace: aigw-learning] Creating ai_gateway: my-aigw... ✓
...
```

Use the `get` verb to inspect the new AI Gateway:

```shell label="get ai-gateway"
kongctl get ai-gateway "My AI Gateway" -o yaml
```

## Update a value

Replace the configuration with a description added:

```shell
cat > ai-gateway.yaml <<'YAML'
_defaults:
  kongctl:
    namespace: aigw-learning

ai_gateways:
  - ref: my-aigw
    name: my-aigw
    display_name: My AI Gateway
    description: Managed with kongctl
    proxy_urls:
      - host: aigw.example.com
        port: 443
        protocol: https
YAML
```

Preview the change:

```shell
kongctl diff --mode apply -f ai-gateway.yaml
```

Confirm that the plan contains an `UPDATE`, then apply it:

```shell
kongctl apply -f ai-gateway.yaml
```

Inspect the updated live values as JSON and use the built-in `jq` filter to
show only the `description` field:

```shell
kongctl get ai-gateway "My AI Gateway" -o json --jq '.description'
```

The diff contains an `UPDATE`. The final command shows the new description in
the live resource.

## Reconcile drift

Drift means that live state no longer matches the desired file. Use a
temporary configuration to simulate a change made outside that file:

```shell
sed 's/Managed with kongctl/Temporary drift/' ai-gateway.yaml > drift.yaml
```

Apply the temporary configuration:

```shell
kongctl apply -f drift.yaml
```

The original file still contains the desired value. Preview the difference
and restore it:

```shell
kongctl diff --mode apply -f ai-gateway.yaml
```

The diff contains an `UPDATE` that restores the desired description. Apply it:

```shell
kongctl apply -f ai-gateway.yaml
```

## Compare apply and sync

Create a second AI Gateway configuration in the same namespace:

```shell
cat > extra-ai-gateway.yaml <<'YAML'
_defaults:
  kongctl:
    namespace: aigw-learning

ai_gateways:
  - ref: extra-aigw
    name: extra-aigw
    display_name: Extra AI Gateway
    proxy_urls:
      - host: extra-aigw.example.com
        port: 443
        protocol: https
YAML
```

Apply the extra resource:

```shell
kongctl apply -f extra-ai-gateway.yaml
```

Preview the original desired state in apply mode:

```shell
kongctl diff --mode apply -f ai-gateway.yaml
```

Apply mode leaves the extra resource alone. Now preview the same input in sync
mode:

```shell
kongctl diff --mode sync -f ai-gateway.yaml
```

Sync mode plans to delete the extra resource because it is managed in the same
namespace but absent from the desired AI Gateway collection. Execute the sync
to remove it:

```shell
kongctl sync -f ai-gateway.yaml
```

## Delete the resource

Preview deleting the resource represented by the file:

```shell
kongctl diff --mode delete -f ai-gateway.yaml
```

Review the planned `DELETE`, then execute it:

```shell
kongctl delete -f ai-gateway.yaml
```

`kongctl delete` creates a delete-mode plan internally and asks for
confirmation before executing it.

Verify that the tutorial AI Gateways no longer appear:

```shell
kongctl list ai-gateways
```

> _Note:_ The list may contain unrelated AI Gateways. Delete mode targets the
> resources represented by the input; it does not delete every resource in the
> namespace.
