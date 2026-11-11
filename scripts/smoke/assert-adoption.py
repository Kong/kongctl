#!/usr/bin/env python3
"""Check ownership previews and adoption receipts for the smoke fixtures."""

import json
import pathlib
import sys

path, stage, key, root_name, original_namespace, target_namespace = sys.argv[1:]
value = json.loads(pathlib.Path(path).read_text())
resource_type = "portal" if key == "portal" else "ai_gateway_model"
name = root_name if key == "portal" else "smoke-model"
if stage == "preview":
    assert value["dry_run"] is True, value
    changes = value["changes"]
else:
    changes = value if isinstance(value, list) else [value]
assert len(changes) == 1, changes
change = changes[0]
assert change["resource_type"] == resource_type, change
assert change["name"] == name and change["id"], change
assert change["namespace"] == target_namespace, change
if stage == "preview":
    namespace_key = "KONGCTL-namespace" if key == "portal" else "kongctl-namespace"
    assert change["before"][namespace_key] == original_namespace, change
    assert change["after"][namespace_key] == target_namespace, change
    if key == "ai_gateway":
        assert change["parent"]["gateway_id"], change
