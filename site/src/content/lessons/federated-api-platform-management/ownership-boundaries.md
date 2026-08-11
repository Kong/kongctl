---
title: Ownership Boundaries
summary: Give the Platform, Engineering, and Product teams separate files.
order: 3
related:
  - label: Metadata lesson
    url: https://kong.github.io/kongctl/declarative-configuration/metadata/
---

## Goal

You will create a `kongctl` project that separates team resource ownership.

## Create the workspace

Start outside the `aigw` directory from the previous chapter. Create and enter
a new workspace:

```shell label="Run this..."
mkdir -p federated-aigw/platform \
  federated-aigw/engineering \
  federated-aigw/product && cd federated-aigw
```

The chapter will build this structure one file at a time:

```text label="Workspace"
federated-aigw/
├── platform/
│   └── ai-gateway.yaml
├── engineering/
│   └── model.yaml
└── product/
    └── model.yaml
```

Each directory represents a configuration owned and reviewed by a different
team. In practice, these directories could be separate repositories.

## Define boundaries

`kongctl` metadata can help you set boundaries between resources.
`namespace` and `labels` provide metadata and behavior support for
segementing resources, while `ref` and `lookup` features allow
you to create resource relationships across those boundaries.

We will start with a fully local configuration and move to a distributed
one later in the lesson.
