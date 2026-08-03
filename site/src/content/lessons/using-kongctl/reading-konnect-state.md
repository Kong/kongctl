---
title: Browsing Konnect
summary: Inspect live Konnect resources before managing them.
order: 2
---

## Goal

You will list live Konnect resources and recognize the difference between
imperative commands and declarative configuration.

## List a resource collection

Use `get` without a resource name or ID to list the collection:

```shell
kongctl get apis
```

An empty result is valid when the organization does not contain any APIs.

## Get one resource

If the list contains an API, pass its name or ID to the resource command.
Quote names that contain spaces:

> _Note:_ These commands won't show anything until you have resources in
> your Konnect organization. Later lessons help you set up resources via
> declarative configuration. Or you can use the UI to create resources and
> view them in `kongctl`.

```shell
kongctl get apis "My Simple API" -o text
```

```text
NAME           DESCRIPTION               ID
My Simple API  The simplest API example  45d7…
```

Use the full ID and JSON output to inspect the complete structured response:

```shell label="example command"
kongctl get apis 45d79870-eb41-4c23-b51b-99123de692ea -o json
```

```json label="example output"
{
  "api_spec_ids": [],
  "attributes": {},
  "created_at": "2026-07-29T19:53:39.413Z",
  "current_version_summary": null,
  "description": "The simplest API example",
  "id": "45d79870-eb41-4c23-b51b-99123de692ea",
  "labels": {
    "KONGCTL-namespace": "default"
  },
  "name": "My Simple API",
  "portals": [],
  "slug": "my-simple-api",
  "updated_at": "2026-07-29T19:53:39.413Z",
  "version": null
}
```

## Explore interactively

Open the interactive resource viewer from a terminal:

```shell label="launch the interactive TUI"
kongctl view
```

Use the on-screen keys to navigate resources and press `q` to exit.

## Imperative vs declarative

The commands in this lesson are imperative read-only commands. `kongctl`
does not support (at this time) creating or updating Konnect resources with
imperative commands. Instead, declarative configuration is the method of
managing Konnect resources by defining the desired state in input configuration
and then applying it to your organization. `kongctl` compares that desired input
with live Konnect state and plans the operations needed to reconcile them.

The declarative configuration chapter will explain resource creation in more
detail.
