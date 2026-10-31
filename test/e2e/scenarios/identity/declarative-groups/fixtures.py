"""Prepare explicit manifests and verify deployment-specific deletion boundaries."""

import json
import os
from pathlib import Path
import re
import subprocess
import sys


def write(work, name, value):
    data = json.dumps(value, indent=2)
    data = re.sub(r'"REF:([^"]+)"', lambda match: "!ref " + match[1], data)
    (work / (name + ".yaml")).write_text(data + "\n")


def main():
    action, location = sys.argv[1:3]
    work = Path(location)
    if action == "prepare":
        name = sys.argv[3]
        prefix = "Kongctl" + name.replace("-", "")
        alpha, adopted, foreign = [prefix + suffix for suffix in ("Alpha", "Adopted", "Foreign")]
        labels = {"e2e-fixture": "REF:people#id", "team": "operations"}
        principal = {"ref": "operator", "type": "user", "display_name": "Group operator",
                     "labels": labels, "kongctl": {"namespace": "identity-group-member-e2e"}}

        def manifest(group=None, principal_value=None, managed=False, empty=False):
            directory = {"ref": "people"}
            if managed:
                directory.update(name=name, allow_all_control_planes=True, vault_enabled=False,
                                 kongctl={"namespace": "identity-group-directory-e2e", "protected": True})
            else:
                directory["_external"] = {"selector": {"matchFields": {"name": name}}}
            if group is not None:
                directory["groups"] = [group]
            if empty:
                directory["groups"] = []
            if principal_value is not None:
                directory["principals"] = [principal_value]
            return {"_defaults": {"kongctl": {"namespace": "identity-group-e2e"}},
                    "identity": {"directories": [directory]}}

        group = {"ref": "alpha", "name": alpha, "type": "group", "description": "Initial group",
                 "metadata": {"count": 3, "enabled": False, "department": "operations",
                              "roles": ["reader", "writer"], "levels": [1, 2], "large": 9007199254740994},
                 "labels": labels}
        write(work, "initial", manifest(group, principal, managed=True))
        write(work, "update", manifest(dict(group, description="Updated group",
              metadata={"count": 5, "enabled": True}, labels=dict(labels, team="updated"))))
        write(work, "clear", manifest({"ref": "alpha", "name": alpha, "description": "", "metadata": {}, "labels": {}}))
        write(work, "partial", manifest({"ref": "alpha", "name": alpha, "description": "Preserve omitted maps"}))
        write(work, "restore-labels", manifest({"ref": "alpha", "name": alpha, "labels": labels}))
        for filename, protected in (("protected", True), ("unprotect", False)):
            write(work, filename, manifest({"ref": "alpha", "name": alpha, "kongctl": {"protected": protected}}))
        write(work, "blocked", manifest({"ref": "alpha", "name": alpha, "description": "Forbidden"}))
        write(work, "group-only", manifest({"ref": "alpha", "name": alpha}))
        write(work, "principal-only", manifest(principal_value=principal))
        write(work, "empty", manifest(empty=True))
        write(work, "omitted", manifest())
        external = {"ref": "existing-group", "_external": {"selector": {"matchFields": {"name": alpha}}}}
        write(work, "external", manifest(external))
        write(work, "adopted", manifest({"ref": "adopted", "name": adopted, "description": "Adopted group"}))
        write(work, "foreign", manifest({"ref": "foreign", "name": foreign, "type": "group", "labels": labels,
              "kongctl": {"namespace": "identity-group-foreign-e2e"}}))
        return {"prepared": True, "alpha_name": alpha, "adopted_name": adopted, "foreign_name": foreign}

    if action == "remove":
        binary, operation, manifest, expected, tag = sys.argv[3:8]
        mode = "delete" if operation == "delete" else "sync"
        config = []
        if os.environ.get("KONGCTL_PROFILE") == "e2e":
            config = ["--config-file", str(Path(os.environ["XDG_CONFIG_HOME"]) / "kongctl/config.yaml")]
        plan = subprocess.run([binary, "plan", "--mode", mode, "-f", manifest, *config],
                              text=True, capture_output=True, timeout=90, check=False)
        (work / (tag + "-plan.stdout")).write_text(plan.stdout)
        (work / (tag + "-plan.stderr")).write_text(plan.stderr)
        if plan.returncode:
            # A legacy deployment may not implement group-key inspection (Q33).
            assert "verify group API keys before deletion: HTTP 501" in plan.stderr
            return {"verified": True, "deletion_supported": False}
        value = json.loads(plan.stdout)
        assert value["summary"]["total_changes"] == int(expected)
        saved = work / (tag + "-plan.json")
        saved.write_text(json.dumps(value))
        result = subprocess.run([binary, operation, "--plan", str(saved), "--auto-approve", "-o", "json", *config],
                                text=True, capture_output=True, timeout=90, check=False)
        (work / (tag + "-apply.stdout")).write_text(result.stdout)
        (work / (tag + "-apply.stderr")).write_text(result.stderr)
        assert result.returncode == 0, result.stderr
        summary = json.loads(result.stdout)["summary"]
        assert summary["applied"] == int(expected) and summary["failed"] == 0
        return {"verified": True, "deletion_supported": True}

    if action == "scope":
        observed = json.loads(Path(sys.argv[3]).read_text())
        deleted = sys.argv[4].lower() == "true"
        primary = [g for g in observed if g["managed_by"].get("kongctl-namespace") == "identity-group-e2e"]
        foreign = [g for g in observed if g["managed_by"].get("kongctl-namespace") == "identity-group-foreign-e2e"]
        assert len(primary) == (0 if deleted else 2) and len(foreign) == 1
        return {"verified": True, "foreign_preserved": True}

    raise ValueError("unknown action")


if __name__ == "__main__":
    print(json.dumps(main()))
