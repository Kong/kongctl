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

```shell
kongctl get apis "My Simple API" -o text
```

```text
NAME           DESCRIPTION               ID
My Simple API  The simplest API example  45d7…
```

Use the full ID and JSON output to inspect the complete structured response:

```shell
kongctl get apis 45d79870-eb41-4c23-b51b-99123de692ea -o json
```

```json
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

```shell
kongctl view
```

Use the on-screen keys to navigate resources and press `q` to exit.

## Imperative vs declarative

The commands in this lesson are imperative read-only commands. `kongctl`
does not support (at this time) creating or updating Konnect resources with
imperative commands. Insetad, declarative configuration is the method of
managing Konnect resources by defining the desired state in input configuration
and then applying it to your organization. `kongctl` compares that desired input
with live Konnect state and plans the operations needed to reconcile them.

The next chapter introduces that workflow.
