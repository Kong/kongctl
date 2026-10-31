"""Verify observable directory dependencies before planned or saved deletion."""

import json
import os
from pathlib import Path
import subprocess
import sys
import urllib.error
import urllib.parse
import urllib.request


def main():
    binary, location, directory, name, server = sys.argv[1:6]
    work = Path(location)
    target = os.environ.get("KONGCTL_E2E_KONNECT_ENV", "com").strip() or "com"
    assert target in ("com", "tech")
    base = os.environ.get("KONGCTL_E2E_KONNECT_BASE_URL", "").strip() or "https://us.api.konghq." + target
    host = urllib.parse.urlparse(base)
    assert host.scheme == "https" and host.hostname.endswith((".api.konghq.com", ".api.konghq.tech"))
    headers = {"Authorization": "Bearer " + os.environ["KONGCTL_E2E_KONNECT_PAT"], "Content-Type": "application/json"}
    config = ["--config-file", str(Path(os.environ["XDG_CONFIG_HOME"]) / "kongctl/config.yaml")]
    root = "/v2/directories/" + directory
    namespace = "identity-directory-guard-e2e"
    marker = "guard-" + directory
    known = {}
    phase = "setup"

    def request(method, path, body=None, expected=200):
        req = urllib.request.Request(base.rstrip("/") + path, method=method, headers=headers,
                                     data=None if body is None else json.dumps(body).encode())
        try:
            response = urllib.request.urlopen(req, timeout=30)
        except urllib.error.HTTPError as error:
            response = error
        with response:
            data = response.read()
            assert response.status == expected
            return json.loads(data) if data else None

    def collection(path):
        value = request("GET", path)
        assert isinstance(value["data"], list) and value["meta"]["page"]["next"] in (None, "")
        return value["data"]

    def invoke(label, args, failure=None):
        nonlocal phase
        phase = label
        result = subprocess.run([binary, *args, *([] if args[0] in ("plan", "diff") else ["-o", "json"]),
                                 "--log-level", "trace", "--log-file", str(work / (label + ".log")), *config],
                                capture_output=True, timeout=90, check=False)
        (work / (label + ".stdout")).write_bytes(result.stdout)
        (work / (label + ".stderr")).write_bytes(result.stderr)
        if failure:
            assert result.returncode != 0
            assert failure.lower() in (result.stdout + result.stderr).decode().lower()
            return None
        assert result.returncode == 0
        return result.stdout.decode() if args[0] == "diff" else json.loads(result.stdout)

    def file(label, value):
        path = work / (label + ".yaml")
        path.write_text(json.dumps(value))
        return str(path)

    def document(vault=None, empty=False):
        parent = {"ref": "people", "name": name}
        if vault is not None:
            parent["vault_enabled"] = vault
        return {"_defaults": {"kongctl": {"namespace": namespace}},
                "identity": {"directories": [] if empty else [parent]}}

    source = file("directory", document())
    prune = file("empty", document(empty=True))
    on = file("enabled", document(True))
    off = file("disabled", document(False))
    saved = work / "delete-plan.json"

    def toggle(label, value):
        result = invoke(label, ["apply", "-f", on if value else off, "--auto-approve"])
        assert result["summary"]["failed"] == 0
        assert request("GET", root)["vault_enabled"] is value

    def blocked(label, reason):
        invoke(label + "-plan", ["plan", "--mode", "delete", "-f", source], failure=reason)
        invoke(label + "-sync-plan", ["plan", "--mode", "sync", "-f", prune], failure=reason)
        invoke(label + "-sync", ["sync", "-f", prune, "--auto-approve"], failure=reason)
        invoke(label + "-delete", ["delete", "-f", source, "--auto-approve"], failure=reason)
        invoke(label + "-saved", ["delete", "--plan", str(saved), "--auto-approve"], failure=reason)
        assert request("GET", root)["id"] == directory

    def retire(path, original):
        current = request("GET", path)
        assert current["id"] == original["id"]
        for field in ("kid", "name", "issuer_url"):
            if field in original:
                assert current[field] == original[field]
        if "/vault/providers/" in path:
            assert collection(path + "/credentials") == []
        request("DELETE", path, expected=204)
        request("GET", path, expected=404)
        del known[path]

    try:
        assert request("GET", root)["vault_enabled"] is True
        for suffix in ("principals", "groups", "vault/keys", "vault/providers", "trusted-idps"):
            assert collection(root + "/" + suffix) == []
        issuer = request("GET", "/v1/auth-servers/" + server)["issuer"]
        invoke("adopt", ["adopt", "identity", "directory", name, "--namespace", namespace])
        original = request("GET", root)
        assert original["managed_by"]["kongctl-namespace"] == namespace
        assert original["managed_by"]["kongctl-protected"] == "false"
        plan = invoke("initial-delete-plan", ["plan", "--mode", "delete", "-f", source])
        assert plan["summary"]["total_changes"] == 1
        saved.write_text(json.dumps(plan))
        assert "identity_directory" in invoke("saved-diff", ["diff", "--plan", str(saved)])
        public_key = (Path(__file__).parent / "public-key.pem").read_text()
        for kind, body, reason in (
            ("vault/keys", {"name": marker, "public_key": public_key}, "Vault keys"),
            ("vault/providers", {"name": marker, "template_name": "static_secret", "credential_type": "shared"}, "Vault providers"),
            ("trusted-idps", {"issuer_url": issuer}, "Trusted IdPs"),
        ):
            value = request("POST", root + "/" + kind, body, expected=201)
            path = root + "/" + kind + "/" + value["id"]
            known[path] = value
            label = kind.replace("/", "-")
            blocked(label + "-attached", reason)
            assert invoke(label + "-no-op", ["plan", "--mode", "apply", "-f", source])["summary"]["total_changes"] == 0
            toggle(label + "-disable", False)
            blocked(label + "-hidden", "explicitly enable Vault")
            assert request("GET", root)["vault_enabled"] is False
            request("GET", path, expected=403)
            toggle(label + "-reenable", True)
            blocked(label + "-visible-again", reason)
            retire(path, value)
        assert not known
        # A plan does not grant authority to enable Vault during execution.
        toggle("late-disable", False)
        blocked("late-disabled-empty", "explicitly enable Vault")
        toggle("explicit-final-enable", True)
        result = invoke("delete-after-retirement", ["delete", "--plan", str(saved), "--auto-approve"])
        assert result["summary"]["failed"] == 0 and result["summary"]["applied"] == 1
        request("GET", root, expected=404)
        result = invoke("repeat-saved-delete", ["delete", "--plan", str(saved), "--auto-approve"])
        assert result["summary"]["failed"] == 0
        return {"attached_families": 3, "planning_and_scoped_sync": True, "saved_plan_rechecks": True,
                "disabled_vault_blocks": True, "observations_do_not_enable": True,
                "explicit_retirement_then_delete": True, "repeat_saved_deletion": True, "fixtures_removed": True}
    except Exception:
        print("directory deletion verification failed at " + phase, file=sys.stderr)
        raise
    finally:
        # Leave unknown records for the opt-in empty-inventory cleanup guard.
        directories = collection("/v2/directories")
        if any(v["id"] == directory for v in directories):
            fresh = request("GET", root)
            if not fresh["vault_enabled"]:
                invoke("cleanup-explicit-enable", ["api", "patch", root, "vault_enabled:=true"])
            for path, value in list(known.items()):
                retire(path, value)
            for suffix in ("vault/keys", "vault/providers", "trusted-idps"):
                assert collection(root + "/" + suffix) == []


if __name__ == "__main__":
    try:
        print(json.dumps(main()))
    except Exception:
        print("directory deletion guards or verified fixture cleanup failed", file=sys.stderr)
        sys.exit(1)
