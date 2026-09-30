"""Check replacement and dependency ordering in a real saved sync plan."""

import json
import sys
from pathlib import Path

plan = json.loads(Path(sys.argv[1]).read_text())
positions = {change_id: index for index, change_id in enumerate(plan["execution_order"])}
changes = plan["changes"]
creates = [c for c in changes if c["action"] == "CREATE"]
deletes = [c for c in changes if c["action"] == "DELETE"]
servers = [c for c in deletes if c["resource_type"] == "ai_gateway_mcp_server"]
assert len(servers) == 6, servers

for old in servers:
    replacements = [
        c for c in creates
        if c["resource_type"] == old["resource_type"]
        and c["parent"]["id"] == old["parent"]["id"]
        and c["resource_ref"] == old["resource_ref"].removesuffix("-renamed")
    ]
    assert len(replacements) == 1, (old, replacements)
    new = replacements[0]
    assert new["id"] in old["depends_on"], (old, new)
    assert positions[new["id"]] < positions[old["id"]]
    for prerequisite in creates:
        if prerequisite["parent"]["id"] != old["parent"]["id"]:
            continue
        if prerequisite["resource_type"] in {"ai_gateway_auth_strategy", "ai_gateway_policy"}:
            assert positions[prerequisite["id"]] < positions[new["id"]]
    for dependency in deletes:
        if dependency["parent"]["id"] != old["parent"]["id"]:
            continue
        if dependency["resource_type"] in {"ai_gateway_auth_strategy", "ai_gateway_policy"}:
            assert positions[old["id"]] < positions[dependency["id"]]

print("Verified six same-gateway replacement edges and prerequisite/deletion ordering")
