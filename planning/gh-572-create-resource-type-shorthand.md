# GH-572 Follow-up Plan: `create <resource-type> -f` Shorthand

## Context

This branch already adds declarative `kongctl create -f` support and best-effort
create execution for existing resources.

The remaining UX gap is one-off resource creation. Today a user who wants to
create a single API via stdin must still provide full declarative shape:

```sh
echo '
apis:
  - ref: simple-api
    name: My Simple API
    description: The simplest API example
' | kongctl create -f - --auto-approve
```

That is valid, but the `apis:` wrapper and `ref:` field do not add much value
for a simple standalone create flow.

## Recommendation

Add resource-scoped create subcommands that can infer the surrounding
`ResourceSet` wrapper from the command path.

Example target UX:

```sh
echo '
name: My Simple API
description: The simplest API example
' | kongctl create api -f - --auto-approve
```

The command should normalize this input into the existing declarative model
before planning and execution. This keeps one planner and one executor path.

## Why This Approach

- The command path already tells us the resource type.
- It avoids introducing a second global declarative syntax.
- It preserves strict `ResourceSet` loading for generic `kongctl create -f`.
- It is shorter and easier for humans and agents to generate.
- It leaves room for future imperative `create api --name ...` flows.

## Non-Goals

- Do not infer resource type from arbitrary YAML fields.
- Do not replace or relax full declarative `ResourceSet` input.
- Do not add a separate execution path outside the declarative planner.
- Do not require a new global `kind:` shorthand as part of this change.

## Proposed UX Rules

### Full declarative create

This remains unchanged:

```sh
kongctl create -f config.yaml
```

Input must still be a full declarative `ResourceSet`.

### Resource-scoped create

When a resource-type subcommand is used with `-f` or `--plan`, the command may
accept either:

- a full declarative `ResourceSet`
- a bare single-resource document for that resource type

Examples:

```sh
kongctl create api -f api.yaml
kongctl create portal -f portal.yaml
kongctl create auth-strategy -f auth.yaml
```

### Bare document normalization

For a bare API document, the command should synthesize:

```yaml
apis:
  - ref: <generated-ref>
    ...
```

The generated `ref` should be deterministic when omitted. A simple slug based
on resource type plus the primary name is enough.

Explicit `ref` should still be allowed in the bare document. It should only be
required when the user needs cross-resource references.

## Command Scope

Start with top-level parent resources that already map cleanly to a single
resource entry in declarative config:

- `create api`
- `create portal`
- `create auth-strategy`
- `create gateway control-plane` or `create control-plane`, depending on the
  existing command tree that we choose to extend

Child-resource shorthand should be deferred. Those flows often require parent
identity flags and do not fit the same single-document shape as cleanly.

## Implementation Plan

### 1. Add create subcommands for top-level resources

Mirror the existing `get` resource command structure where practical, but keep
the initial behavior narrow:

- if no `-f` or `--plan` is provided, keep current help behavior
- if `-f` or `--plan` is provided, route into declarative create execution

This should be additive to the current root-level `kongctl create -f` support.

### 2. Add command-level input normalization

Implement shorthand expansion in the create command layer, not in the generic
declarative loader.

Preferred flow:

1. Read the file/stdin bytes in the resource-specific command.
2. Try strict decode as full `ResourceSet`.
3. If that fails, decode as the bare resource struct for that command.
4. Wrap the decoded resource into a synthetic `ResourceSet`.
5. Pass the normalized `ResourceSet` into the existing declarative planner.

This keeps `internal/declarative/loader` strict for generic declarative use.

### 3. Add deterministic ref generation

If the bare resource omits `ref`, synthesize one from:

- resource type
- best primary identity field, usually `name`

The generated ref only needs local stability within the input. It does not need
to be globally unique outside the document.

### 4. Reuse existing create-mode semantics

Once normalized, the flow should use the same create-mode planner and executor
already added on this branch:

- duplicate create conflicts become `existing`
- missing resources are created
- mixed duplicate and new resources exit successfully when there are no real
  failures

### 5. Extend docs and examples

Document the new shortcut as a CLI convenience layered on top of declarative
create, not as a new declarative file format.

## Testing Plan

### E2E

Add scenario coverage for one resource type first, preferably `api`:

1. `kongctl create api -f -` on an empty org with a bare API document
2. run the same command again and assert `existing: 1`
3. optional follow-up with a second bare resource to confirm new resources are
   still created correctly

### Unit

Add focused tests for:

- bare document detection and normalization
- generated ref behavior
- full `ResourceSet` input still passing through unchanged
- command argument validation for resource-scoped create commands

## Alternatives Considered

### Global `kind:` shorthand

Example:

```yaml
kind: api
name: My Simple API
```

This is workable, but it broadens the declarative file format itself. The
resource-scoped command is a better first step because it is explicit, smaller
in scope, and easier to add without weakening strict generic loading.

### Field-based type inference

This should be avoided. Too many resource types share fields like `name` and
`description`, so inference would be brittle and hard to explain.

## Suggested Order

1. `create api -f` shorthand
2. `create portal -f` shorthand
3. `create auth-strategy -f` shorthand
4. control plane shorthand once the preferred command path is settled

This gives the branch a clear follow-up that builds on the current declarative
create work without changing its core execution model.
