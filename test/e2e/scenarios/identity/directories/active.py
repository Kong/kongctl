"""Exercise active lifecycle without changing the selected directory's settings."""

import json
import os
from pathlib import Path
import subprocess
import sys
import uuid


def main():
    binary, location, directory_id = sys.argv[1:4]
    work = Path(location)
    work.mkdir(parents=True, exist_ok=True)
    ref = "active-" + uuid.uuid4().hex[:12]
    namespace = "identity-active-e2e"
    sequence = 0

    def run(*args, failure=None, raw=False):
        nonlocal sequence
        sequence += 1
        output = [] if args[0] in ("plan", "diff", "dump") else ["-o", "json"]
        config = []
        if os.environ.get("KONGCTL_PROFILE") == "e2e":
            config = ["--config-file", str(Path(os.environ["XDG_CONFIG_HOME"]) / "kongctl/config.yaml")]
        result = subprocess.run(
            [binary, *args, *output, *config, "--no-telemetry"],
            text=True, capture_output=True, timeout=90, check=False,
        )
        (work / f"active-{sequence:02d}.stdout").write_text(result.stdout)
        (work / f"active-{sequence:02d}.stderr").write_text(result.stderr)
        if failure:
            assert result.returncode != 0 and failure in result.stderr + result.stdout, args
            return None
        assert result.returncode == 0, (args, result.stderr)
        return result.stdout if raw else json.loads(result.stdout)

    def manifest(name, **fields):
        principal = {"ref": ref, "labels": {"e2e-fixture": directory_id}, **fields}
        value = {
            "_defaults": {"kongctl": {"namespace": namespace}},
            "identity": {"directories": [{
                "ref": "people", "_external": {"id": directory_id}, "principals": [principal],
            }]},
        }
        path = work / (name + ".yaml")
        path.write_text(json.dumps(value, indent=2) + "\n")
        return str(path)

    base = manifest("active-omitted", display_name="Active lifecycle probe", type="agent")
    disabled = manifest("active-false", type="agent", active=False)
    enabled = manifest("active-true", active=True)
    partial = manifest("active-partial", display_name="Active omission preserved")

    def observed():
        values = run("get", "identity", "principals", "--directory-id", directory_id)
        matches = [p for p in values if p.get("managed_by", {}).get("kongctl-ref") == ref
                   and p["managed_by"].get("kongctl-namespace") == namespace]
        assert len(matches) == 1
        return matches[0]

    def apply(path, count):
        value = run("apply", "-f", path, "--auto-approve")
        assert value["summary"]["failed"] == 0 and value["summary"]["applied"] == count

    def no_op(path):
        assert run("plan", "--mode", "apply", "-f", path)["summary"]["total_changes"] == 0

    try:
        apply(base, 1)
        initial = observed()
        if "active" not in initial:
            run("plan", "--mode", "apply", "-f", disabled, failure="Q10")
            no_op(base)
            return {"verified": True, "active_supported": False}

        assert initial["active"] is True
        run("delete", "-f", base, "--auto-approve")
        plan = run("plan", "--mode", "apply", "-f", disabled)
        assert len(plan["changes"]) == 1 and plan["changes"][0]["fields"]["active"] is False
        saved = work / "active-create-plan.json"
        saved.write_text(json.dumps(plan))
        assert run("apply", "--plan", str(saved), "--auto-approve")["summary"]["failed"] == 0
        principal = observed()
        assert principal["active"] is False and len(principal["managed_by"]) == 5
        principal_id = principal["id"]
        no_op(disabled)
        apply(partial, 1)
        assert observed()["active"] is False
        diff = run("diff", "-f", enabled, raw=True)
        assert "active" in diff and "false" in diff and "true" in diff
        plan = run("plan", "--mode", "apply", "-f", enabled)
        assert len(plan["changes"]) == 1 and plan["changes"][0]["fields"]["active"] is True
        saved = work / "active-update-plan.json"
        saved.write_text(json.dumps(plan))
        assert run("apply", "--plan", str(saved), "--auto-approve")["summary"]["failed"] == 0
        assert observed()["active"] is True and observed()["id"] == principal_id
        no_op(enabled)
        apply(disabled, 1)
        protected = manifest("active-protected", kongctl={"protected": True})
        apply(protected, 1)
        run("apply", "-f", enabled, "--auto-approve", failure="protected")
        unprotected = manifest("active-unprotected", kongctl={"protected": False})
        apply(unprotected, 1)
        assert observed()["active"] is False
        run("adopt", "identity", "principal", principal_id, "--directory-id", directory_id,
            "--ref", ref, "--namespace", namespace, "--overwrite-namespace")
        assert observed()["active"] is False
        no_op(disabled)
        dumped = run("dump", "declarative", "--resources", "identity.directories",
                     "--filter-id", directory_id, "--include-child-resources", raw=True)
        assert ref in dumped and "active: false" in dumped
        (work / "active-dump.yaml").write_text(dumped)
        # Export stays a read operation: do not adopt the external directory to reapply it.
        run("sync", "-f", enabled, "--auto-approve")
        assert observed()["active"] is True
        no_op(enabled)
        return {"verified": True, "active_supported": True}
    finally:
        # Use the existing ownership-aware deletion path; never remove other principals.
        values = run("get", "identity", "principals", "--directory-id", directory_id)
        owned = [p for p in values if p.get("managed_by", {}).get("kongctl-ref") == ref
                 and p["managed_by"].get("kongctl-namespace") == namespace]
        if len(owned) == 1 and owned[0]["managed_by"].get("kongctl-protected") == "true":
            apply(manifest("active-cleanup-unprotect", kongctl={"protected": False}), 1)
        run("delete", "-f", base, "--auto-approve")
        remaining = run("get", "identity", "principals", "--directory-id", directory_id)
        assert not any(p.get("managed_by", {}).get("kongctl-ref") == ref for p in remaining)


if __name__ == "__main__":
    print(json.dumps(main()))
