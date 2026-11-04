#!/usr/bin/env python3
"""Enrich scaffold-derived smoke fixtures using only the Python standard library."""

import copy
import json
import pathlib
import sys


def children(key):
    if key == "portal":
        return {"snippets": [{
            "ref": "smoke-snippet-ref", "name": "smoke-snippet",
            "title": "Smoke snippet", "description": "Smoke content",
            "visibility": "private", "status": "unpublished",
            "content": "Smoke snippet initial content",
        }]}
    if key == "event_gateway":
        return {
            "backend_clusters": [{
                "ref": "smoke-backend-ref", "name": "smoke-backend",
                "description": "Smoke backend",
                "bootstrap_servers": ["smoke.example.com:9092"],
                "authentication": {"type": "anonymous"},
                "tls": {"enabled": False},
            }],
            "virtual_clusters": [{
                "ref": "smoke-virtual-ref", "name": "smoke-virtual",
                "description": "Smoke virtual cluster",
                "destination": {"id": "SMOKE_BACKEND_REFERENCE"},
                "authentication": [{"type": "anonymous"}],
                "acl_mode": "passthrough", "dns_label": "smoke",
            }],
        }
    if key != "ai_gateway":
        return {}
    result = {
        "datastores": [{
            "ref": "smoke-store-ref", "name": "smoke-store",
            "display_name": "Smoke store", "type": "vectordb",
            "config": {"host": "database.example.com", "database": "smoke"},
        }],
        "proxy_urls": [{"host": "smoke.example.com", "port": 443, "protocol": "https"}],
        "model_providers": [{
            "ref": "smoke-provider-ref", "name": "smoke-provider",
            "display_name": "Smoke provider", "type": "openai",
            "config": {"auth": {"type": "basic", "headers": []}},
        }],
        "policies": [{
            "ref": "smoke-policy-ref", "name": "smoke-policy",
            "display_name": "Smoke policy", "type": "rate-limiting",
            "enabled": True, "global": False, "config": {"minute": 10},
        }],
        "models": [{
            # UUID-shaped refs must still match existing children by name.
            "ref": "00000000-0000-4000-8000-000000000001", "name": "smoke-model",
            "display_name": "Smoke model", "type": "model", "enabled": True,
            "config": {
                "route": {"model": {"values": ["smoke-model"]}},
                "balancer": {
                    "algorithm": "semantic",
                    "embeddings": {"provider": "smoke-provider", "name": "text-embedding-3-small",
                                   "config": {"type": "openai"}},
                    "vectordb": {"type": "pgvector", "dimensions": 1536,
                                 "distance_metric": "cosine", "threshold": 0.7},
                },
            },
            "formats": [{"type": "openai"}], "capabilities": ["generate"],
            "targets": [{"name": "gpt-4o", "provider": "smoke-provider",
                         "semantic_description": "General business questions and detailed explanations.",
                         "config": {"type": "openai"}},
                        {"name": "gpt-4o-mini", "provider": "smoke-provider",
                         "semantic_description": "CATCHALL", "config": {"type": "openai"}}],
            "policies": ["SMOKE_POLICY_REFERENCE"],
            "datastores": [{"name": "SMOKE_DATASTORE_REFERENCE"}],
        }],
    }
    result.update(min_runtime_version="2.3", runtime_auto_upgrade=False)
    model = result["models"][0]
    model["config"]["route"]["model"]["values"].append("smoke-alias")
    model["targets"][0]["config"].update(
        input_cost_list=[{"modal": "text", "cost": 2.5}],
        output_cost_list=[{"modal": "audio", "cost": 10}],
        cache_read_cost_list=[{"modal": "image", "cost": 0.5}],
    )
    result["policies"][0]["condition"] = "http.method == 'POST'"
    result["mcp_servers"] = [{
        "ref": "smoke-mcp-ref", "name": "smoke-mcp",
        "display_name": "Smoke MCP", "type": "conversion-listener",
        "tools": [{"name": "lookup", "description": "Look up a record",
                   "method": "GET", "path": "/records",
                   "access": {"acls": {"allow_when": ["http.headers.x_smoke_tool != null"]}}}],
        "access": {
            "acl_attribute_type": "consumer",
            "acls": {"allow_when": ["http.headers.x_smoke_user != null"]},
            "default_tool_acls": {"deny_when": ["http.headers.x_smoke_blocked != null"]},
        },
        "rate_limiting": {"limits": [
            {"method": "tools/list", "limit": 10, "window_size": 60},
            {"tool": "lookup", "limit": 5, "window_size": 1},
        ]},
        "config": {
            "url": "https://example.com", "route": {"paths": ["/smoke"]},
            "server": {"forward_client_headers": True, "timeout": 10000},
            "allowed_versions": ["2025-11-25"],
            "cache": {"tools_list": {"ttl_ms": 0, "cache_scope": "private"},
                      "discover": {"ttl_ms": 60000, "cache_scope": "public"}},
        },
    }]
    return result


def render(values):
    text = "".join(f"    {key}: {json.dumps(value)}\n" for key, value in values.items())
    return text.replace('"SMOKE_BACKEND_REFERENCE"', "!ref smoke-backend-ref#id").replace(
        '"SMOKE_POLICY_REFERENCE"', "!ref smoke-policy-ref#name"
    ).replace('"SMOKE_DATASTORE_REFERENCE"', "!ref smoke-store-ref#name").replace(
        '"SMOKE_NAMESPACE_REFERENCE"', "!env {var: KONGCTL_SMOKE_NAMESPACE_AI_GATEWAY}"
    )


def main():
    key, directory = sys.argv[1:]
    directory = pathlib.Path(directory)
    initial = children(key)
    updated = copy.deepcopy(initial)
    if key == "portal":
        updated["snippets"][0]["content"] = "Smoke snippet updated content"
    elif key == "event_gateway":
        updated["backend_clusters"][0]["description"] += " updated"
        updated["virtual_clusters"][0]["description"] += " updated"
    elif key == "ai_gateway":
        # These children own namespaces independently of their gateway.
        for field in ("model_providers", "policies", "models", "mcp_servers"):
            for child in updated[field]:
                child["kongctl"] = {"namespace": "SMOKE_NAMESPACE_REFERENCE"}
        updated["datastores"][0]["display_name"] = "Smoke store updated"
        updated["policies"][0]["config"]["minute"] = 20
        mcp = updated["mcp_servers"][0]
        mcp["rate_limiting"]["limits"][0]["limit"] = 20
        mcp["rate_limiting"]["limits"][1]["limit"] = 10
        mcp["access"]["acls"]["allow_when"] = ["http.headers.x_smoke_user == 'allowed'"]
        mcp["access"]["default_tool_acls"]["deny_when"] = ["http.headers.x_smoke_blocked == 'true'"]
        mcp["tools"][0]["access"]["acls"]["allow_when"] = ["http.headers.x_smoke_tool == 'allowed'"]
        updated["models"][0]["display_name"] = "Smoke model updated"
        updated["models"][0]["targets"][0]["config"]["input_cost_list"][0]["cost"] = 3.5
        updated["mcp_servers"][0]["config"]["cache"]["discover"]["ttl_ms"] = 120000
    for phase, values in (("initial", initial), ("updated", updated)):
        path = directory / f"{phase}.yaml"
        base = path.read_text()
        # Scaffolds may already supply commented or active optional root fields.
        base = "\n".join(line for line in base.splitlines()
                         if not any(line.startswith(f"    {field}:") for field in values)) + "\n"
        path.write_text(base + render(values))
        if phase == "updated" and key in ("portal", "event_gateway", "ai_gateway"):
            collections = ("snippets", "backend_clusters", "virtual_clusters",
                           "model_providers", "models", "policies", "mcp_servers", "datastores")
            pruned = {field: ([] if field in collections else value) for field, value in values.items()}
            (directory / "pruned.yaml").write_text(base + render(pruned))
            if key == "ai_gateway":
                replacement = copy.deepcopy(values)
                replacement["model_providers"][0]["name"] = "smoke-provider-replacement"
                for target in replacement["models"][0]["targets"]:
                    target["provider"] = "smoke-provider-replacement"
                replacement["models"][0]["config"]["balancer"]["embeddings"]["provider"] = \
                    "smoke-provider-replacement"
                replacement["policies"][0]["name"] = "smoke-policy-replacement"
                (directory / "replacement.yaml").write_text(base + render(replacement))


if __name__ == "__main__":
    main()
