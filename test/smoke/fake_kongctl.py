#!/usr/bin/env python3
import json
import os
import pathlib
import re
import sys

args = sys.argv[1:]
if args[:1] == ["--profile"]:
    args = args[2:]
log_path = pathlib.Path(os.environ["FAKE_KONGCTL_LOG"])
with log_path.open("a", encoding="utf-8") as handle:
    handle.write(" ".join(args) + "\n")

joined = " ".join(args)
fail_on = os.environ.get("FAKE_FAIL_ON", "")
if fail_on and fail_on in joined:
    print(f"injected failure for {fail_on}", file=sys.stderr)
    raise SystemExit(1)
if os.environ.get("FAKE_FAIL_DELETE") == "1" and args[:1] == ["delete"]:
    print("injected delete failure", file=sys.stderr)
    raise SystemExit(1)

state_path = pathlib.Path(os.environ["FAKE_KONGCTL_STATE"])
state = json.loads(state_path.read_text(encoding="utf-8")) if state_path.exists() else {}

def save_state():
    state_path.write_text(json.dumps(state, sort_keys=True), encoding="utf-8")

def option(name, default=""):
    if name not in args:
        return default
    return args[args.index(name) + 1]

def resource_from_path(path):
    value = str(path)
    for key in ("control_plane", "ai_gateway", "event_gateway", "portal", "api"):
        if f"/{key}/" in value or value.endswith(f"/{key}.yaml"):
            return key
    raise RuntimeError(f"unknown fixture path: {value}")

def parse_fixture(path):
    text = pathlib.Path(path).read_text(encoding="utf-8")
    key = resource_from_path(path)
    root = {"api": "apis", "portal": "portals", "control_plane": "control_planes", "ai_gateway": "ai_gateways", "event_gateway": "event_gateways"}[key]
    def scalar(field):
        match = re.search(rf"^    {field}: (.+)$", text, re.MULTILINE)
        if not match:
            return ""
        raw = match.group(1)
        try:
            return json.loads(raw)
        except json.JSONDecodeError:
            return raw.strip('"')
    ref_match = re.search(r"^  - ref: (.+)$", text, re.MULTILINE)
    ref = json.loads(ref_match.group(1))
    item = {
        "ref": ref, "name": scalar("name") or ref,
        "display_name": scalar("display_name"), "description": scalar("description"),
        "root": root, "labels": {"smoke-phase": re.search(r'smoke-phase: "([^"]+)"', text).group(1)},
        "_fixture": text,
        "_namespace": re.search(r'namespace: "([^"]+)"', text).group(1),
    }
    for field in ("snippets", "backend_clusters", "virtual_clusters",
                  "model_providers", "models", "policies", "mcp_servers"):
        raw = scalar(field)
        if raw != "":
            if isinstance(raw, str):
                raw = re.sub(r"!ref ([\w#-]+)", r'"\1"', raw)
                raw = json.loads(raw)
            item[field] = raw
    return key, item

CHILD_TYPES = {
    "snippets": "portal_snippet", "backend_clusters": "event_gateway_backend_cluster",
    "virtual_clusters": "event_gateway_virtual_cluster", "model_providers": "ai_gateway_model_provider",
    "models": "ai_gateway_model", "policies": "ai_gateway_policy", "mcp_servers": "ai_gateway_mcp_server",
}

def flatten(key, item):
    if item is None:
        return {}
    root = {k: v for k, v in item.items() if k not in CHILD_TYPES and k != "_fixture"}
    result = {(key, root["name"]): root}
    policy_names = {p["ref"]: p["name"] for p in item.get("policies", [])}
    for field, kind in CHILD_TYPES.items():
        for child in item.get(field, []):
            child = json.loads(json.dumps(child))
            if field == "models":
                child["policies"] = [policy_names.get(p.split("#")[0], p) for p in child.get("policies", [])]
            result[(kind, child["name"])] = child
    if key == "api":
        for field, kind in (("versions", "api_version"), ("documents", "api_document")):
            if f"    {field}:" in item["_fixture"]:
                result[(kind, field)] = {"ref": field, "name": field}
    return result

def make_plan(key, desired, mode):
    old = flatten(key, state.get(key))
    new = {} if mode == "delete" else flatten(key, desired)
    changes = []
    for identity, value in new.items():
        if old.get(identity) != value:
            changes.append({"action": "UPDATE" if identity in old else "CREATE",
                            "resource_type": identity[0], "resource_ref": value["ref"]})
    if mode in ("sync", "delete"):
        for identity, value in old.items():
            if identity not in new:
                changes.append({"action": "DELETE", "resource_type": identity[0], "resource_ref": value["ref"]})
    # The fake orders the dependency graph independently from the script's checker.
    ranks = {"ai_gateway_model_provider": 1, "ai_gateway_policy": 1, "ai_gateway_model": 2,
             "event_gateway_backend_cluster": 1, "event_gateway_virtual_cluster": 2}
    changes.sort(key=lambda c: (c["action"] == "DELETE",
                               -ranks.get(c["resource_type"], 0) if c["action"] == "DELETE"
                               else ranks.get(c["resource_type"], 0)))
    for i, change in enumerate(changes):
        change["id"] = str(i)
        change["namespace"] = desired["_namespace"]
    plan = {"metadata": {"mode": mode}, "summary": {"total_changes": len(changes)}, "changes": changes,
            "execution_order": [c["id"] for c in changes], "_key": key, "_desired": desired}
    fault = os.environ.get("FAKE_FAULT", "")
    if fault == "missing-child" and key == "api" and changes:
        changes.pop()
        plan["summary"]["total_changes"] = len(changes)
    if fault == "bad-namespace" and changes:
        changes[0]["namespace"] = "another-project"
    if fault == "unsafe-order" and any(c["action"] == "DELETE" for c in changes) and mode == "sync":
        plan["execution_order"].reverse()
    if fault == "false-zero" and not changes:
        plan["changes"] = [{"action": "UPDATE", "resource_type": key, "resource_ref": desired["ref"]}]
    return plan

def resource_from_command():
    if "control-plane" in args or "control-planes" in args:
        return "control_plane"
    if "event-gateway" in args or "event-gateways" in args:
        return "event_gateway"
    if "ai-gateway" in args or "ai-gateways" in args:
        return "ai_gateway"
    if "portal" in args or "portals" in args:
        return "portal"
    return "api"

def print_execution(changes=1):
    print(json.dumps({"summary": {"status": "success", "failed": 0, "applied": changes, "total_changes": changes}}))

if args == ["--help"]:
    print("kongctl fake help")
elif args[:2] == ["get", "profiles"]:
    path = pathlib.Path(option("--config-file", os.environ.get("KONGCTL_CONFIG_FILE", "")))
    if not path.is_file():
        print("provided config file path does not exist", file=sys.stderr)
        raise SystemExit(1)
    print(json.dumps(re.findall(r"^(\S+):", path.read_text(), re.MULTILINE)))
elif args[:2] == ["version", "--full"]:
    print(json.dumps({"version": "1.2.3", "commit": "abcdef123456", "date": "2026-08-21T00:00:00Z"}))
elif args[:1] == ["explain"]:
    subject = args[1]
    if "--extended" in args:
        if subject.endswith("vaults"):
            print("auth_method=approle auth_method=kubernetes")
        else:
            print("type=openai type=azure allowed: basic")
        raise SystemExit(0)
    roots = {"api": "apis", "portal": "portals", "control_plane": "control_planes", "ai_gateway": "ai_gateways", "event_gateway": "event_gateways",
             "api.versions": "api_versions", "api.documents": "api_documents"}
    properties = {
        "api": ["ref", "name", "description", "version", "slug"],
        "portal": ["ref", "name", "display_name", "description"],
        "control_plane": ["ref", "name", "description", "cluster_type"],
        "ai_gateway": ["ref", "name", "display_name", "description"],
        "event_gateway": ["ref", "name", "description"],
        "api.versions": ["ref", "version", "spec"],
        "api.documents": ["ref", "content", "title", "slug"],
    }[subject]
    print(json.dumps({
        "type": "object",
        "title": f"kongctl declarative schema: {subject}",
        "x-kongctl-root-key": roots[subject],
        "x-kongctl-maturity": {"level": "ga"},
        "required": ["ref"],
        "properties": {field: {"type": "string"} for field in properties},
    }))
elif args[:1] == ["scaffold"]:
    subject = args[1]
    bad = os.environ.get("FAKE_BAD_SCAFFOLD", "")
    outputs = {
        "api": """apis:
  - ref: my-resource
    name: my-resource
    description: Example description
    version: v1.0.0
    slug: my-resource
    # labels: {}
""",
        "portal": """portals:
  - ref: my-resource
    name: my-resource
    display_name: My Resource
    description: Example description
""",
        "control_plane": """control_planes:
  - ref: my-resource
    name: my-resource
    description: Example description
    # cluster_type: value
""",
        "event_gateway": """event_gateways:\n  - ref: my-resource\n    name: my-resource\n    description: Example description\n""",
        "ai_gateway": """ai_gateways:
  - ref: my-resource
    name: my-ai-gateway
    display_name: My AI Gateway
    # description:
""",
        "api.versions": """apis:
  - ref: my-resource
    versions:
      - version: v1.0.0
        spec: !file ./specs/api.yaml
        ref: my-resource
""",
        "api.documents": """apis:
  - ref: my-resource
    documents:
      - content: !file ./content.txt
        # title: value
        slug: my-resource
        ref: my-resource
""",
    }
    output = outputs[subject]
    if bad == subject:
        output = output.replace(output.splitlines()[0], "wrong_root:", 1)
    print(output, end="")
elif args[:2] == ["patch", "file"]:
    text = pathlib.Path(args[2]).read_text()
    if os.environ.get("FAKE_FAULT") == "lost-tags":
        text = text.replace("!ref", "")
    print(text.replace("name: before", "name: after"))
elif args[:1] == ["plan"]:
    fixture = option("-f")
    key, desired = parse_fixture(fixture)
    mode = option("--mode", "sync")
    plan = make_plan(key, desired, mode)
    pathlib.Path(option("--output-file")).write_text(json.dumps(plan), encoding="utf-8")
    print(json.dumps(plan))
elif args[:1] == ["diff"]:
    plan = json.loads(pathlib.Path(option("--plan")).read_text(encoding="utf-8"))
    print("\n".join(change["resource_ref"] for change in plan["changes"]) or "No changes")
elif args[:1] in (["apply"], ["sync"]):
    if "--plan" in args:
        plan = json.loads(pathlib.Path(option("--plan")).read_text(encoding="utf-8"))
        key, desired = plan["_key"], plan["_desired"]
    else:
        key, desired = parse_fixture(option("-f"))
        plan = make_plan(key, desired, "sync")
    state[key] = desired
    save_state()
    print_execution(len(plan["changes"]))
elif args[:2] == ["dump", "declarative"]:
    keys = {"apis": "api", "portals": "portal", "control_planes": "control_plane",
            "ai_gateways": "ai_gateway", "event_gateways": "event_gateway"}
    key = keys[option("--resources")]
    text = state[key]["_fixture"]
    if os.environ.get("FAKE_FAULT") == "bad-dump" and key == "portal":
        text = re.sub(r"^    snippets:.*$", "    snippets: []", text, flags=re.MULTILINE)
    if "--include-child-resources" not in args:
        text = "\n".join(line for line in text.splitlines()
                         if not any(line.startswith(f"    {field}:") for field in CHILD_TYPES)) + "\n"
    if option("--output-file"):
        pathlib.Path(option("--output-file")).write_text(text, encoding="utf-8")
    else:
        print(text)
elif args[:1] == ["delete"]:
    dry_run = "--dry-run" in args
    if "--plan" in args:
        plan = json.loads(pathlib.Path(option("--plan")).read_text(encoding="utf-8"))
        key = plan["_key"]
    else:
        key = resource_from_path(option("-f"))
    changes = 1 if key in state else 0
    if not dry_run:
        state.pop(key, None)
        save_state()
    print_execution(changes)
elif args[:1] in (["get"], ["list"]):
    key = resource_from_command()
    item = state.get(key)
    if item and args[0] == "get":
        for command, field in (("models", "models"), ("policies", "policies"),
                               ("mcp-servers", "mcp_servers"), ("snippets", "snippets")):
            if command in args:
                child = item[field][0]
                kind = CHILD_TYPES[field]
                print(json.dumps(flatten(key, item)[(kind, child["name"])]))
                raise SystemExit(0)
    if args[0] == "list":
        value = {} if os.environ.get("FAKE_FAULT") == "bad-list" else ([item] if item else [])
        if "text" in args and item:
            print(item["display_name"] if key == "ai_gateway" else item["name"])
        else:
            print(json.dumps(value))
    elif not item:
        message = "authentication failed" if os.environ.get("FAKE_FAULT") == "wrong-not-found" else "not found"
        print(message, file=sys.stderr)
        raise SystemExit(1)
    else:
        print(json.dumps(item))
else:
    print(f"unsupported fake command: {joined}", file=sys.stderr)
    raise SystemExit(2)
