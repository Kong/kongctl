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
    for key in ("identity_principal", "identity_auth_server", "control_plane", "ai_gateway", "event_gateway", "portal", "api"):
        if f"/{key}/" in value or value.endswith(f"/{key}.yaml"):
            return key
    raise RuntimeError(f"unknown fixture path: {value}")

def namespace_field(text):
    # Only the generated root metadata and file-default blocks carry ownership.
    for indent in (6, 4, 2):
        match = re.search(
            rf"^{' ' * indent}kongctl:\n"
            rf"(?:{' ' * (indent + 2)}(?!namespace:)\S[^\n]*\n)*"
            rf"{' ' * (indent + 2)}namespace: ([^\n]+)$", text, re.MULTILINE,
        )
        if match:
            return match
    raise SystemExit("fixture is missing kongctl namespace metadata")

def parse_fixture(path):
    text = pathlib.Path(path).read_text(encoding="utf-8")
    key = resource_from_path(path)
    original_text = text
    if key.startswith("identity_"):
        prefix, envelope = text.split("identity:\n", 1)
        text = prefix + "\n".join(line[2:] for line in envelope.splitlines()) + "\n"
    root = {"api": "apis", "portal": "portals", "control_plane": "control_planes", "ai_gateway": "ai_gateways", "event_gateway": "event_gateways", "identity_principal": "identity.principals", "identity_auth_server": "identity.auth_servers"}[key]
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
    namespace_match = namespace_field(text)
    namespace = namespace_match.group(1)
    if namespace.startswith("!env "):
        source = namespace.removeprefix("!env ")
        if source.startswith("{var: "):
            source = source.removeprefix("{var: ").removesuffix("}")
        namespace = os.environ.get(source)
        if not namespace:
            raise SystemExit(f"namespace environment variable {source} is unset or empty")
    else:
        namespace = json.loads(namespace)
    if option("--require-namespace", namespace) != namespace:
        raise SystemExit("fixture namespace does not match --require-namespace")
    item = {
        "ref": ref, "name": scalar("name") or scalar("display_name") or ref,
        "display_name": scalar("display_name"), "description": scalar("description"),
        "root": root, "labels": {"smoke-phase": re.search(r'smoke-phase: "([^"]+)"', text).group(1)},
        "_fixture": original_text,
        "_namespace": namespace,
    }
    item["labels"]["KONGCTL-namespace"] = namespace
    item["audience"] = scalar("audience")
    if key == "identity_principal":
        item["metadata"] = scalar("metadata")
        item["type"] = scalar("type")
    for field in ("ttl_secs", "allow_all_control_planes", "vault"):
        value = scalar(field)
        if value != "":
            item[field] = value
    item["managed_by"] = {"kongctl-namespace": namespace}
    if key == "identity_principal":
        if '"_external"' not in text or not any(json.dumps(d["name"]) in text for d in directories or [{"name": "default"}]):
            raise SystemExit("smoke must reference the existing default directory")
        item["id"] = "33333333-3333-4333-8333-333333333333"
    for field in ("identities", "scopes", "claims", "snippets", "backend_clusters", "virtual_clusters",
                  "model_providers", "models", "policies", "mcp_servers", "datastores"):
        raw = scalar(field)
        if raw != "":
            if isinstance(raw, str):
                raw = re.sub(r"!ref ([\w#-]+)", r'"\1"', raw)
                raw = re.sub(r"!env \{var: (\w+)\}",
                             lambda match: json.dumps(os.environ[match.group(1)]), raw)
                raw = json.loads(raw)
            item[field] = raw
    return key, item

CHILD_TYPES = {
    "identities": "identity_principal_identity", "scopes": "identity_auth_server_scope",
    "claims": "identity_auth_server_claim",
    "snippets": "portal_snippet", "backend_clusters": "event_gateway_backend_cluster",
    "virtual_clusters": "event_gateway_virtual_cluster", "model_providers": "ai_gateway_model_provider",
    "datastores": "ai_gateway_datastore",
    "models": "ai_gateway_model", "policies": "ai_gateway_policy", "mcp_servers": "ai_gateway_mcp_server",
}

def flatten(key, item):
    if item is None:
        return {}
    root = {k: v for k, v in item.items() if k not in CHILD_TYPES and k != "_fixture"}
    result = {(key, root["name"]): root}
    policy_names = {p["ref"]: p["name"] for p in item.get("policies", [])}
    datastore_names = {d["ref"]: d["name"] for d in item.get("datastores", [])}
    for field, kind in CHILD_TYPES.items():
        for child in item.get(field, []):
            child = json.loads(json.dumps(child))
            if field == "models":
                child["policies"] = [policy_names.get(p.split("#")[0], p) for p in child.get("policies", [])]
                for store in child.get("datastores", []):
                    store["name"] = datastore_names.get(store["name"].split("#")[0], store["name"])
            result[(kind, child.get("name", child["ref"]))] = child
            for identity in child.get("identities", []):
                result[("identity_principal_identity", identity["ref"])] = identity
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
    ranks = {"ai_gateway_datastore": 1, "ai_gateway_model_provider": 1,
             "ai_gateway_policy": 1, "ai_gateway_model": 2,
             "identity_principal": 1, "identity_principal_identity": 2,
             "identity_auth_server_scope": 1, "identity_auth_server_claim": 2,
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
    if "identity" in args:
        return "identity_auth_server" if any(a.startswith("auth-server") for a in args) else "identity_principal"
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

def saved_plan():
    plan = json.loads(pathlib.Path(option("--plan")).read_text(encoding="utf-8"))
    key, desired = plan["_key"], plan["_desired"]
    environment = os.environ.get(f"KONGCTL_SMOKE_NAMESPACE_{key.upper()}")
    with log_path.with_suffix(".namespaces.jsonl").open("a", encoding="utf-8") as handle:
        handle.write(json.dumps({"verb": args[0], "key": key, "planned": desired["_namespace"],
                                 "environment": environment}) + "\n")
    if os.environ.get("FAKE_FAULT") == f"rebind-{args[0]}-namespace":
        desired["_namespace"] = environment
        desired["labels"]["KONGCTL-namespace"] = environment
    return plan, key, desired

directory_path = state_path.with_suffix(".directories.json")
directories = json.loads(directory_path.read_text()) if directory_path.exists() else [
    {"id": "22222222-2222-4222-8222-222222222222", "name": "default"},
]
if "identity" in args and any(a in args for a in ("directory", "directories")):
    if os.environ.get("FAKE_FAULT") == "directory-read-error":
        print("directory inventory: authentication failed", file=sys.stderr)
        raise SystemExit(1)
    if args[0] == "list":
        print(json.dumps(directories))
    else:
        matches = [d for d in directories if d["name"] in args or d["id"] in args]
        if len(matches) != 1:
            print("identity directory not found", file=sys.stderr)
            raise SystemExit(1)
        print(json.dumps(matches[0]))
    raise SystemExit(0)

if args[0] == "plan" and option("-f").endswith("identity-directory-bootstrap.yaml"):
    declaration = json.loads(pathlib.Path(option("-f")).read_text())["identity"]["directories"][0]
    assert declaration["allow_all_control_planes"] is True
    assert declaration["kongctl"]["protected"] is True
    assert not directories, "directory quota exceeded"
    change = {"id": "bootstrap", "action": "CREATE", "resource_type": "identity_directory",
              "resource_ref": "default", "namespace": "kctl-smoke-identity-bootstrap", "protection": True,
              "fields": {"name": "default", "allow_all_control_planes": True}}
    plan = {"metadata": {"mode": "apply"}, "summary": {"total_changes": 1},
            "changes": [change], "_bootstrap": True}
    pathlib.Path(option("--output-file")).write_text(json.dumps(plan))
    print(json.dumps(plan))
    raise SystemExit(0)
if args[0] == "apply" and option("--plan"):
    plan = json.loads(pathlib.Path(option("--plan")).read_text())
    if plan.get("_bootstrap"):
        if os.environ.get("FAKE_FAULT") == "bootstrap-failure":
            print("directory create failed", file=sys.stderr)
            raise SystemExit(1)
        assert not directories, "directory quota exceeded"
        directory_path.write_text(json.dumps([
            {"id": "22222222-2222-4222-8222-222222222222", "name": "default"},
        ]))
        print_execution(1)
        raise SystemExit(0)

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
             "api.versions": "api_versions", "api.documents": "api_documents", "identity_principal": "identity.principals", "identity_auth_server": "identity.auth_servers"}
    properties = {
        "identity_principal": ["ref", "directory", "description", "display_name"],
        "identity_auth_server": ["ref", "name", "description", "audience"],
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
    outputs.update({
        "identity_principal": "identity:\n  principals:\n    - ref: my-resource\n      display_name: My Resource\n      directory: value\n",
        "identity_auth_server": "identity:\n  auth_servers:\n    - ref: my-resource\n      name: my-resource\n      audience: https://api.example.com\n",
    })
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
    if pathlib.Path(fixture).name == "adopted.yaml" and "/ai_gateway/" in fixture:
        exported = json.loads(pathlib.Path(fixture).read_text())
        assert exported["ai_gateways"][0]["_external"]["id"] == "ai_gateway-id"
        models = exported["ai_gateway_models"]
        assert len(models) == len(state["ai_gateway"]["models"])
        for model, original in zip(models, state["ai_gateway"]["models"]):
            assert model["ai_gateway"] == "gateway"
            assert model["kongctl"]["namespace"] == state["ai_gateway"]["_namespace"]
            assert {k: v for k, v in model.items() if k not in ("ai_gateway", "kongctl")} == {
                k: v for k, v in original.items() if k != "kongctl"}
        key, desired = "ai_gateway", state["ai_gateway"]
    else:
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
        plan, key, desired = saved_plan()
    else:
        key, desired = parse_fixture(option("-f"))
        plan = make_plan(key, desired, "sync")
    state[key] = desired
    save_state()
    print_execution(len(plan["changes"]))
elif args[:1] == ["adopt"]:
    key = "portal" if args[1] == "portal" else "ai_gateway"
    item = state[key]
    child = key == "ai_gateway"
    namespace = option("--namespace")
    namespace_key = "kongctl-namespace" if child else "KONGCTL-namespace"
    change = {"resource_type": "ai_gateway_model" if child else "portal",
              "id": "model-id" if child else "portal-id",
              "name": "smoke-model" if child else item["name"], "namespace": namespace}
    dry_run = "--dry-run" in args
    if dry_run:
        change.update(before={namespace_key: item["_namespace"]}, after={namespace_key: namespace})
        if child:
            change["parent"] = {"gateway_id": "ai_gateway-id"}
    output = pathlib.Path(option("--declarative-output"))
    if child:
        models = json.loads(json.dumps(item["models"]))
        for model in models:
            model["ai_gateway"] = "gateway"
            model["kongctl"] = {"namespace": namespace, "protected": False}
        text = json.dumps({"ai_gateways": [{"ref": "gateway", "_external": {"id": "ai_gateway-id"}}],
                           "ai_gateway_models": models})
    else:
        text = item["_fixture"]
        match = namespace_field(text)
        text = text[:match.start(1)] + json.dumps(namespace) + text[match.end(1):]
    output.write_text(("# DRY RUN: proposed configuration\n" if dry_run else "") + text)
    if not dry_run:
        assert namespace == item["_namespace"]
    if os.environ.get("FAKE_FAULT") == "bad-adoption-preview" and dry_run:
        change["after"][namespace_key] = "wrong-namespace"
    print(json.dumps({"dry_run": True, "changes": [change]} if dry_run else [change] if child else change))
elif args[:2] == ["dump", "declarative"]:
    keys = {"apis": "api", "portals": "portal", "control_planes": "control_plane",
            "ai_gateways": "ai_gateway", "event_gateways": "event_gateway", "identity.directories": "identity_principal", "identity.auth_servers": "identity_auth_server"}
    key = keys[option("--resources")]
    text = state[key]["_fixture"]
    namespace = option("--default-namespace", state[key]["_namespace"])
    if os.environ.get("FAKE_FAULT") == "wrong-dump-namespace":
        namespace = "another-project"
    match = namespace_field(text)
    text = text[:match.start(1)] + json.dumps(namespace) + text[match.end(1):]
    text = re.sub(r"!env \{var: KONGCTL_SMOKE_NAMESPACE_\w+\}", json.dumps(namespace), text)
    if os.environ.get("FAKE_FAULT") == "bad-dump" and key == "portal":
        text = re.sub(r"^    snippets:.*$", "    snippets: []", text, flags=re.MULTILINE)
    if "--include-child-resources" not in args:
        text = "\n".join(line for line in text.splitlines()
                         if not any(line.startswith(f"    {field}:") for field in CHILD_TYPES)) + "\n"
    if os.environ.get("FAKE_FAULT") == "missing-dump-datastore" and key == "ai_gateway":
        text = re.sub(r"^    datastores:.*\n", "", text, flags=re.MULTILINE)
    if option("--output-file"):
        pathlib.Path(option("--output-file")).write_text(text, encoding="utf-8")
    else:
        print(text)
elif args[:1] == ["delete"]:
    dry_run = "--dry-run" in args
    if "--plan" in args:
        plan, key, desired = saved_plan()
        if key in state and desired["_namespace"] != state[key]["_namespace"]:
            print_execution(0)
            raise SystemExit(0)
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
        if key == "identity_principal":
            person = dict(item)
            if os.environ.get("FAKE_FAULT") == "stale-principal":
                person["metadata"]["department"] = "engineering"
            if "principal-identities" in args:
                output = person["identities"]
            else:
                output = person if person["id"] in args else [person]
            print(json.dumps(output))
            raise SystemExit(0)
        for command, field in (("auth-server-scope", "scopes"), ("auth-server-claim", "claims")):
            if key == "identity_auth_server" and command in args:
                output = dict(item[field][0])
                if field == "scopes":
                    output["id"] = "44444444-4444-4444-8444-444444444444"
                else:
                    output["include_in_scopes"] = ["44444444-4444-4444-8444-444444444444"]
                if os.environ.get("FAKE_FAULT") == "stale-claim" and field == "claims":
                    output["value"] = '\"engineering\"'
                print(json.dumps(output))
                raise SystemExit(0)
        if "datastores" in args:
            stores = item.get("datastores", [])
            names = []
            index = args.index("datastores") + 1
            while index < len(args):
                if args[index].startswith("-"):
                    index += 2
                else:
                    names.append(args[index])
                    index += 1
            name = option("--datastore-name") or (names[0] if names else "")
            if name:
                matches = [store for store in stores if store["name"] == name]
                if not matches:
                    print("datastore not found", file=sys.stderr)
                    raise SystemExit(1)
                print(json.dumps(matches[0]))
            else:
                print(json.dumps(stores))
            raise SystemExit(0)
        for command, field in (("models", "models"), ("policies", "policies"),
                               ("mcp-servers", "mcp_servers"), ("snippets", "snippets")):
            if command in args:
                child = item[field][0]
                kind = CHILD_TYPES[field]
                output = flatten(key, item)[(kind, child["name"])]
                fault = os.environ.get("FAKE_FAULT", "")
                if field == "models" and fault in ("numeric-format", "wrong-threshold"):
                    vector = output["config"]["balancer"]["vectordb"]
                    vector["dimensions"] = 1536.0
                    vector["threshold"] = 0.7000000000000001 if fault == "numeric-format" else 0.71
                if fault == "stale-mcp-limit" and field == "mcp_servers":
                    output["rate_limiting"]["limits"][0]["limit"] = 10
                if fault == "reordered-mcp" and field == "mcp_servers":
                    output["rate_limiting"]["limits"].reverse()
                    output["tools"].insert(0, {"name": "another-tool"})
                print(json.dumps(output))
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
