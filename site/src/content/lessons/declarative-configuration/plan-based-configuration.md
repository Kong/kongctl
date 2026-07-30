---
title: Plan-Based Configuration
summary: Understand desired state, plans, and state-free reconciliation.
order: 1
related:
  - label: Declarative configuration documentation
    url: https://developer.konghq.com/kongctl/declarative/
---

## Goal

You will understand how `kongctl` turns desired state into a reviewable plan
without maintaining a local state file.

## Describe the desired state

Imperative commands describe _how_. For example...

```shell
kongctl get ai-gateways
```

describes a step, _get_ my AI Gateway resources.

Declarative configuration...

```yaml
ai_gateways:
  - ref: my-aigw
    name: my-aigw
    display_name: My AI Gateway
```

describes _what_ you want. I want an AI Gateway with a
name `my-aigw` and a display name of `My AI Gateway`.

This YAML is a configuration manifest, it's the _desired state_.
It describes the desired Konnect resource state, but it is
not a script that defines the API calls required to get it.
You can have one or many configuration file inputs for declarative
configuration commands.

## Every change starts with a plan

All `kongctl` declarative commands revolve around a _plan_. A plan contains
the ordered operations needed to move resources from their current state to
the desired state.

Every plan has one of three modes:

| Mode     | What the plan can do                                     |
| -------- | -------------------------------------------------------- |
| `apply`  | Create and update resources, but never delete them.      |
| `sync`   | Create, update, and delete resources to match the input. |
| `delete` | Delete the resources represented by the input.           |

Generate a plan by choosing its mode and providing one or more configuration
files:

```shell
kongctl plan --mode <mode> -f <input-configuration-files>
```

`kongctl` loads the configuration, queries live Konnect state, and writes a
JSON plan artifact to standard output. No Konnect resources are changed.
Add `--output-file plan.json` to save the artifact for review or later
execution.

## Explicit and implicit plans

You can generate and execute a plan as separate steps:

```shell
kongctl plan --mode apply -f ai-gateway.yaml --output-file plan.json
kongctl apply --plan plan.json
```

You can also provide the configuration directly to an execution command:

```shell
kongctl apply -f ai-gateway.yaml
```

The second form still creates an `apply`-mode plan. The plan is generated
internally and then executed instead of being saved as a separate artifact.

The same choice is available for `sync` and `delete`: provide configuration
files and let the command generate its plan, or provide a previously generated
plan artifact.

## Match the command to the mode

A saved plan must be executed by the command that matches its mode:

| Plan mode | Execution command              |
| --------- | ------------------------------ |
| `apply`   | `kongctl apply --plan <plan>`  |
| `sync`    | `kongctl sync --plan <plan>`   |
| `delete`  | `kongctl delete --plan <plan>` |

`kongctl` rejects a plan when the execution command does not match its mode.
`kongctl diff --plan <plan>` can inspect a saved plan in any mode without
executing it.

## State-free reconciliation

`kongctl` does not keep a state database. Each new plan compares the input
configuration with current Konnect state.
