#!/usr/bin/env python3
"""Check the smoke test's child transitions, including dependency order."""

import collections
import json
import sys

path, resource, stage, namespace = sys.argv[1:]
plan = json.load(open(path, encoding="utf-8"))
changes = plan.get("changes", [])
if plan.get("metadata", {}).get("mode") != "sync":
    raise SystemExit("expected a sync plan")
if not changes or plan.get("summary", {}).get("total_changes") != len(changes):
    raise SystemExit("empty or inconsistent transition plan")
if any(change["resource_type"] == resource for change in changes):
    raise SystemExit("child transition unexpectedly changes the root")
if any(change.get("namespace") != namespace for change in changes):
    raise SystemExit("child transition escapes the smoke namespace")

actual = collections.Counter((c["action"], c["resource_type"]) for c in changes)
provider = "ai_gateway_model_provider"
model = "ai_gateway_model"
policy = "ai_gateway_policy"
if stage == "replacement":
    expected = collections.Counter({
        ("CREATE", provider): 1, ("DELETE", provider): 1,
        ("CREATE", policy): 1, ("DELETE", policy): 1, ("UPDATE", model): 1,
    })
else:
    types = {
        "portal": ["portal_snippet"],
        "event_gateway": ["event_gateway_backend_cluster", "event_gateway_virtual_cluster"],
        "ai_gateway": [provider, model, policy],
    }[resource]
    if resource == "ai_gateway":
        types.append("ai_gateway_mcp_server")
    expected = collections.Counter(("DELETE", kind) for kind in types)
if actual != expected:
    raise SystemExit(f"unexpected actions: {actual}; wanted {expected}")

order = plan.get("execution_order", [])
if len(order) != len(changes) or set(order) != {c["id"] for c in changes}:
    raise SystemExit("execution order does not cover every change exactly once")
groups = plan.get("execution_groups")
if groups:
    flattened = [change_id for group in groups for change_id in group]
    if len(flattened) != len(order) or set(flattened) != set(order):
        raise SystemExit("execution groups do not cover every change exactly once")


def before(first, second):
    first_id = next(c["id"] for c in changes if (c["action"], c["resource_type"]) == first)
    second_id = next(c["id"] for c in changes if (c["action"], c["resource_type"]) == second)
    if order.index(first_id) >= order.index(second_id):
        raise SystemExit(f"unsafe dependency order: {first} must precede {second}")
    if groups:
        group_for = {change_id: index for index, group in enumerate(groups) for change_id in group}
        if group_for[first_id] >= group_for[second_id]:
            raise SystemExit(f"unsafe concurrent execution: {first} must finish before {second}")


if stage == "replacement":
    for kind in (provider, policy):
        before(("CREATE", kind), ("UPDATE", model))
        before(("UPDATE", model), ("DELETE", kind))
elif resource == "ai_gateway":
    for kind in (provider, policy):
        before(("DELETE", model), ("DELETE", kind))
elif resource == "event_gateway":
    # Backend deletion cascades to its virtual clusters; avoid a second delete
    # of an already removed virtual cluster, including concurrent execution.
    before(("DELETE", "event_gateway_virtual_cluster"), ("DELETE", "event_gateway_backend_cluster"))
