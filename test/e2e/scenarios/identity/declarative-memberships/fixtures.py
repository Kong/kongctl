"""Prepare direct membership manifests and inspect deployment-specific teardown."""

import copy
import json
import os
from pathlib import Path
import re
import subprocess
import sys


def main():
    action, location = sys.argv[1:3]
    work = Path(location)
    if action == "prepare":
        name = sys.argv[3]
        group_name = "Kongctl" + name.replace("-", "") + "Members"
        labels = {"e2e-fixture": "REF:people#id"}
        principals = [{"ref": ref, "type": kind, "display_name": ref.title(), "labels": labels,
                       "kongctl": {"namespace": "identity-membership-principal-e2e"}}
                      for ref, kind in [("alice", "user"), ("worker", "agent"), ("carol", "user")]]

        def membership(ref):
            return {"ref": ref + "-member", "member": {"type": "principal", "id": "REF:" + ref + "#id"}}

        def manifest(members=None, include_principals=False, managed_directory=False, protected=False):
            group = {"ref": "operators", "name": group_name, "labels": labels,
                     "kongctl": {"namespace": "identity-group-e2e", "protected": protected}}
            if members is not None:
                group["memberships"] = [membership(ref) for ref in members]
            directory = {"ref": "people", "groups": [group]}
            if managed_directory:
                directory.update(name=name, allow_all_control_planes=True, vault={"enabled": False},
                                 kongctl={"namespace": "identity-membership-directory-e2e", "protected": True})
            else:
                directory["_external"] = {"selector": {"matchFields": {"name": name}}}
            if include_principals:
                directory["principals"] = copy.deepcopy(principals)
            return {"_defaults": {"kongctl": {"namespace": "identity-group-e2e"}},
                    "identity": {"directories": [directory]}}

        def write(filename, value):
            data = json.dumps(value, indent=2)
            data = re.sub(r'"REF:([^"]+)"', lambda match: "!ref " + match[1], data)
            (work / (filename + ".yaml")).write_text(data + "\n")

        initial = manifest(["alice", "worker"], True, True)
        initial["identity"]["directories"][0]["principals"] = principals[:2]
        write("initial", initial)
        write("add", manifest(["alice", "carol"], True))
        write("omitted", manifest())
        write("clear", manifest([]))
        write("protected", manifest(protected=True))
        write("protected-clear", manifest([], protected=True))
        write("unprotected", manifest())
        delete = manifest(["alice", "worker"], True, True)
        delete["identity"]["directories"][0]["kongctl"]["protected"] = False
        if name == "default":
            parent = delete["identity"]["directories"][0]
            parent.clear()
            parent.update(manifest(["alice", "worker"], True)["identity"]["directories"][0])
        write("delete", delete)
        unprotect_directory = manifest(managed_directory=True)
        unprotect_directory["identity"]["directories"][0]["kongctl"]["protected"] = False
        del unprotect_directory["identity"]["directories"][0]["groups"]
        write("unprotect-directory", unprotect_directory)
        group_only = manifest()
        write("group-only", group_only)
        principal_only = manifest(include_principals=True)
        del principal_only["identity"]["directories"][0]["groups"]
        write("principal-only", principal_only)
        return {"prepared": True, "group_name": group_name, "delete_changes": 6 if name == "default" else 7}

    if action == "verify-plan":
        plan = json.loads(Path(sys.argv[3]).read_text())
        memberships = [c for c in plan["changes"] if c["resource_type"] == "identity_group_membership"]
        assert len(memberships) == 2 and all(c["action"] == "CREATE" for c in memberships)
        for change in memberships:
            assert change["namespace"] == "identity-group-e2e"
            assert change["protecting_parent"]["resource_type"] == "identity_group"
            assert "managed_by" not in change["fields"] and "labels" not in change["fields"]
            assert len(change["depends_on"]) >= 2
        return {"verified": True}

    if action == "repeat-delete":
        saved = work / "delete-all-plan.json"
        if not saved.exists():
            assert "verify group API keys before deletion: HTTP 501" in (work / "delete-all-plan.stderr").read_text()
            return {"verified": True, "deletion_supported": False}
        binary = sys.argv[3]
        config = []
        if os.environ.get("KONGCTL_PROFILE") == "e2e":
            config = ["--config-file", str(Path(os.environ["XDG_CONFIG_HOME"]) / "kongctl/config.yaml")]
        result = subprocess.run([binary, "delete", "--plan", str(saved), "--auto-approve", "-o", "json", *config],
                                text=True, capture_output=True, timeout=90, check=False)
        (work / "repeat-delete.stdout").write_text(result.stdout)
        (work / "repeat-delete.stderr").write_text(result.stderr)
        assert result.returncode == 0, result.stderr
        assert json.loads(result.stdout)["summary"]["failed"] == 0
        return {"verified": True, "deletion_supported": True}

    if action == "verify-members":
        values = json.loads(Path(sys.argv[3]).read_text())
        expected = int(sys.argv[4])
        assert len(values) == expected
        assert all(v["member"]["type"] == "principal" and v["id"] != v["member"]["id"] for v in values)
        assert all("managed_by" not in v and "labels" not in v for v in values)
        return {"verified": True}

    raise ValueError("unknown action")


if __name__ == "__main__":
    print(json.dumps(main()))
