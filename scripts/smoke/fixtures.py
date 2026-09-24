#!/usr/bin/env python3
"""Enrich scaffold-derived smoke fixtures using only the Python standard library."""

import copy
import json
import pathlib
import sys


def children(key, runtime21):
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
            "config": {"route": {"model": {"values": ["smoke-model"]}}},
            "formats": [{"type": "openai"}], "capabilities": ["generate"],
            "targets": [{"name": "gpt-4o", "provider": "smoke-provider",
                         "config": {"type": "openai"}}],
            "policies": ["SMOKE_POLICY_REFERENCE"],
        }],
    }
    if runtime21:
        result.update(min_runtime_version="2.1", runtime_auto_upgrade=False)
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
                       "method": "GET", "path": "/records"}],
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
    )


def main():
    key, directory, runtime21 = sys.argv[1:]
    directory = pathlib.Path(directory)
    initial = children(key, runtime21 == "true")
    updated = copy.deepcopy(initial)
    if key == "portal":
        updated["snippets"][0]["content"] = "Smoke snippet updated content"
    elif key == "event_gateway":
        updated["backend_clusters"][0]["description"] += " updated"
        updated["virtual_clusters"][0]["description"] += " updated"
    elif key == "ai_gateway":
        updated["policies"][0]["config"]["minute"] = 20
        updated["models"][0]["display_name"] = "Smoke model updated"
        if runtime21 == "true":
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
                           "model_providers", "models", "policies", "mcp_servers")
            pruned = {field: ([] if field in collections else value) for field, value in values.items()}
            (directory / "pruned.yaml").write_text(base + render(pruned))
            if key == "ai_gateway":
                replacement = copy.deepcopy(values)
                replacement["model_providers"][0]["name"] = "smoke-provider-replacement"
                replacement["models"][0]["targets"][0]["provider"] = "smoke-provider-replacement"
                replacement["policies"][0]["name"] = "smoke-policy-replacement"
                (directory / "replacement.yaml").write_text(base + render(replacement))


if __name__ == "__main__":
    main()
