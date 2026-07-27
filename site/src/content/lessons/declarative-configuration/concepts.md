---
title: Declarative configuration concepts
summary: Preview YAML changes without maintaining a local state file.
order: 1
related:
  - label: Declarative configuration documentation
    url: https://developer.konghq.com/kongctl/declarative/
  - label: Declarative resource reference
    url: https://developer.konghq.com/kongctl/supported-resources/
---

## Goal

You will describe a Konnect API in YAML and preview the changes kongctl would
make. The preview does not modify Konnect.

## Imperative or declarative?

An imperative command asks for an operation now:

```shell
kongctl get apis
```

Declarative configuration describes the result you want. kongctl reads the
YAML, queries live Konnect resources, and calculates a plan to reconcile the
two. It does not maintain a separate state file.

## Create a small configuration

Create a working directory:

```shell
mkdir -p kongctl-learning
cd kongctl-learning
```

Create `api.yaml` in that directory with this content:

```yaml
apis:
  - ref: learn-kongctl-api
    name: learn-kongctl-api
    description: API used while learning kongctl
```

The `ref` is kongctl's stable identifier for this resource inside declarative
configuration. It is not written to Konnect.

## Preview the plan

Make sure you are authenticated, then ask kongctl to calculate an apply-mode
diff:

```shell
kongctl login
kongctl diff -f api.yaml --mode apply
```

`--mode apply` previews creates and updates, but not deletes. The command reads
the current Konnect state and prints the proposed operations without executing
them.

## Check your understanding

Run the preview again:

```shell
kongctl diff -f api.yaml --mode apply
```

Because no operation was applied, the preview should describe the same
proposed change. This illustrates the state-free model: each run compares the
configuration with live Konnect rather than a local state database.
