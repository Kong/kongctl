# Declarative Parallel Execution Design

## Purpose

This document describes how declarative execution in `kongctl` is parallelized,
how execution groups are planned, and how failures are propagated without
unnecessarily stopping unrelated work.

The design goal is to improve throughput while preserving correctness,
determinism, and predictable failure behavior.

## Scope

This design applies to declarative plan execution for commands such as:

- `kongctl apply`
- `kongctl sync`
- `kongctl delete`

It covers planner and executor behavior for both regular resource changes and
external-tool steps.

## High-Level Model

Declarative execution is split into two phases:

1. Planning:
   Build dependency graph, produce a topological order, and derive ordered
   execution groups.
2. Execution:
   Run groups sequentially, and run members within each group concurrently,
   bounded by `--max-concurrency`.

Key property:

- Group order is strict.
- Intra-group parallelism is allowed.
- A failed change only blocks changes that depend on it.

## Planner Design

### Dependency Graph Construction

The planner dependency resolver builds graph edges from:

- Explicit dependencies (`DependsOn`)
- Implicit dependencies (reference placeholders resolved to create changes)
- Parent dependencies when parent IDs are unresolved (`[unknown]`)

The resolver emits:

- `ExecutionOrder` (flat topological order)
- `ExecutionGroups` (Kahn levels for safe parallelism)
- `FullDepsMap` (all dependency edges per node)

The planner persists implicit edges back into each change's `DependsOn`, making
the plan artifact the single source of truth for execution ordering.

### Why Groups

Execution groups represent Kahn levels from topological sorting:

- Nodes in the same level have no remaining dependencies on each other.
- Nodes in later levels depend only on earlier levels.

This gives deterministic and safe concurrency without recomputing runtime
ordering rules.

## Executor Design

### Grouped Execution Strategy

When `ExecutionGroups` exist, executor behavior is:

1. Process group `N`.
2. Classify each change in group `N` as:
   - runnable
   - blocked (because a dependency has failed or was blocked)
3. Run runnable changes with `errgroup` limit `e.concurrency`.
4. Wait for all runnable changes in the group to complete.
5. Promote failures to a shared `blockedOrFailed` set.
6. Proceed to group `N+1`.

When `ExecutionGroups` are absent (legacy plans), executor falls back to
sequential `ExecutionOrder`.

### Concurrency Contract

`--max-concurrency` controls executor worker slots only. It does not limit
internal parallelism inside external tools or downstream SDKs.

If `max-concurrency` is set to `1`, executor processes one runnable change at a
time, but any called subprocess can still use its own threads/goroutines.

## Concurrency Policy and Rate Limits

### Configuration

Current executor bounds are:

- Default concurrency: `5`
- Minimum concurrency: `1`
- Maximum concurrency: `200`

### Rate-Limit Rationale (2000 requests/min)

Assume average request latency of 200 ms.

Approximate sustained request rate:

`requests_per_min ~= concurrency * (60_000 / avg_latency_ms)`

At 200 ms:

- `concurrency = 1` -> ~300 req/min
- `concurrency = 5` -> ~1500 req/min
- `concurrency = 7` -> ~2100 req/min

This is why default `5` is chosen: it stays below 2000 req/min with practical
headroom for burstiness, retries, and mixed operation latency.

### Why Min and Max

- Min `1`:
  Ensures a deterministic serial mode for troubleshooting, low-rate budgets, or
  fragile environments.
- Max `200`:
  Hard safety cap to prevent accidental extreme values. This cap is not a
  recommended operational target for a 2000 req/min budget.

Operational guidance:

- Use default `5` for general usage under a 2000 req/min budget.
- Tune down to `1-3` for strict throttling.
- Tune up cautiously only when observed end-to-end request rate remains within
  allowed limits.

## Error Propagation Strategy

### Principle

Do not fail fast globally. Fail locally and block only dependent changes.

### Runtime Mechanics

Executor tracks two related concepts:

- Per-change errors are recorded in `ExecutionResult.Errors` and
  `FailureCount`.
- `blockedOrFailed` tracks change IDs that should block dependents.

For each group:

- Runnable changes execute.
- If a change fails, its ID is marked failed.
- In later groups, any change whose `DependsOn` includes a failed/blocked ID is
  skipped as blocked.

This design allows unrelated changes to continue, even if one branch fails.

### Example

Given dependencies:

- `A` (fails)
- `B` depends on `A`
- `C` independent of `A`

Behavior:

- `A` runs and fails.
- `B` is skipped as blocked.
- `C` still runs.

This is the intended behavior for high-throughput, partially independent plans.

## External Tool Steps and Parallel Execution

External tool steps (for example `_deck` with `EXTERNAL_TOOL`) are regular plan
changes and therefore participate in the same dependency and group scheduling
model.

Implications:

- Planner can place external-tool changes into groups with dependencies.
- Executor schedules them using the same worker limit.
- A worker running an external tool blocks until that tool exits.
- The tool's internal concurrency is outside executor's `--max-concurrency`
  control.

## Determinism and Safety

Design choices that preserve deterministic behavior:

- Planner computes explicit group ordering.
- Executor processes groups strictly in order.
- Dependency edges are persisted into the plan artifact.
- Blockers are computed from dependency IDs, not timing.

This avoids race-prone mutation of future changes and keeps execution behavior
traceable from the plan artifact.

## Observability and Result Model

Execution results are aggregate-first:

- success count
- failure count
- skipped count
- structured error list

Command-level failure is based on aggregate result errors rather than first
runtime failure. This enables complete runs with meaningful partial-success
reporting.

### Benefits

- Higher throughput on independent changes
- Controlled and tunable concurrency
- Scoped failure impact
- Plan artifact remains authoritative for ordering

