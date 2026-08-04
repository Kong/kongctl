---
title: Plan-Based Configuration
summary: Understand desired state, plans, and state-free reconciliation.
order: 1
related:
  - label: Declarative configuration documentation
    url: https://developer.konghq.com/kongctl/declarative/
---

## Goal

The following lessons are about declarative configuration: how `kongctl`
manages the state of resources. In this lesson you will understand how
`kongctl` turns desired state into a plan document and the different execution
_modes_.

> _Note:_ The next few sections are purely informative. The information is key
> to understanding the `kongctl` declarative configuration system.
> Once you reach _The Getting Started_ lesson, you will begin to run
> commands with functioning configurations.

## Describe the desired state

Imperative commands describe _how_. For example...

```shell label="imperative command..."
kongctl get ai-gateways
```

describes a step; _get_ my AI Gateway resources.

On the other hand, declarative configuration...

```yaml label="Example declarative configuration"
ai_gateways:
  - ref: my-aigw
    name: my-aigw
    display_name: My AI Gateway
```

describes _what_ you want; I want an AI Gateway with a
name `my-aigw` and a display name of `My AI Gateway`.

This YAML is a configuration manifest. It's the _desired state_, describing
how you want the Konnect resource state to be, but it is not a script that
defines the API calls required to get there.

You can have one or many configuration files describing your desired state,
and there are different ways to organize the declarations within them
depending on your needs.

## Every change starts with a plan

All `kongctl` declarative commands revolve around a _plan_. A plan contains
the set of operations needed to move resources from their current state to
the desired state.

Every plan has one of three modes:

| Mode     | What the plan can do                                        |
| -------- | ----------------------------------------------------------- |
| `apply`  | Create and update resources only (no delete).               |
| `sync`   | Create, update, and delete resources.                       |
| `delete` | Only delete the resources represented by the configuration. |

The mode is stored in the plan artifact. It defines which operations the plan
may contain and which command can execute it.

## The plan command

The `kongctl plan` command accepts one or more configuration files along with
the mode:

```shell label="Command syntax"
kongctl plan --mode <mode> -f <input-configuration-files>
```

`kongctl` loads the configuration, queries the live Konnect state, and writes
a JSON plan artifact. No Konnect resources are changed at this stage; the
output is simply a plan of changes to be made. Add `--output-file <file>` to
save the artifact for review or later execution.

For example, this saves an apply-mode plan to the file `plan.json`:

```shell label="Example plan command"
kongctl plan \
  --mode apply \
  -f ai-gateway.yaml \
  --output-file plan.json
```

The following is an example basic plan for an AI Gateway so you can see
the general structure:

```json rows=8 label="example plan"
{
  "metadata": {
    "version": "1.0",
    "generated_at": "2026-08-02T00:38:03.151546468Z",
    "generator": "kongctl/1.8.0",
    "mode": "apply"
  },
  "changes": [
    {
      "id": "1:c:ai_gateway:basic-ai-gateway",
      "resource_type": "ai_gateway",
      "resource_ref": "basic-ai-gateway",
      "action": "CREATE",
      "fields": {
        "description": "AI Gateway created from kongctl",
        "display_name": "Basic AI Gateway",
        "name": "basic-ai-gateway"
      },
      "protection": false,
      "namespace": "default"
    }
  ],
  "execution_order": ["1:c:ai_gateway:basic-ai-gateway"],
  "execution_groups": [["1:c:ai_gateway:basic-ai-gateway"]],
  "summary": {
    "total_changes": 1,
    "by_action": {
      "CREATE": 1
    },
    "by_resource": {
      "ai_gateway": 1
    }
  }
}
```

## Execution Commands

Once you have a plan, you will want to execute it such that the changes
are applied to Konnect. Each _plan mode_ has a corresponding execution command:

| Plan mode | Execution command |
| --------- | ----------------- |
| `apply`   | `kongctl apply`   |
| `sync`    | `kongctl sync`    |
| `delete`  | `kongctl delete`  |

An execution command accepts either a plan file with the `--plan` flag or a
list of configuration files with `-f`.

A saved plan must be executed by the command that matches its mode. `kongctl`
rejects a plan when the execution command does not match its mode.

Thus, there are two ways to execute a change.

### Plan, then execute

Generate a plan artifact first when you want a review or approval step before
execution:

```shell label="generate the plan"
kongctl plan --mode apply -f ai-gateway.yaml --output-file plan.json
```

```shell label="execute the saved plan"
kongctl apply --plan plan.json
```

The first command only creates the plan. The second command executes that
saved apply-mode plan.

### Plan and execute together

Provide the input configuration files directly to the execution command
when you do not need a separate plan artifact:

```shell label="plan and execute in one command"
kongctl apply -f ai-gateway.yaml
```

This _still creates an apply-mode plan_, but `kongctl` generates it internally
and then executes it immediately.

The same choice is available for `sync` and `delete`: provide configuration
files and let the command generate its plan, or provide a previously generated
plan artifact.

## State-free reconciliation

Unlike other declarative tools, `kongctl` does not keep a state database.
Each new plan compares the input configuration with current Konnect state.
As a result of this design, `kongctl` must know which resources are under its
control. In the next lessons we will cover how resources are identified and
recognized as managed by `kongctl`.
