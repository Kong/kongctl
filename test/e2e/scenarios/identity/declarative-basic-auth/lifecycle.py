"""Run username-record lifecycle; native credential material stays private."""

import base64
import json
import os
from pathlib import Path
import subprocess
import sys
import traceback
import urllib.parse
import urllib.request


def main():
    binary, private_dir, work_dir, name = sys.argv[1:5]
    private, work = Path(private_dir).resolve(), Path(work_dir).resolve()
    artifacts = work.parents[2]
    assert not private.is_relative_to(artifacts)
    target = os.environ.get("KONGCTL_E2E_KONNECT_ENV", "com").strip() or "com"
    assert target in ("com", "tech")
    base = os.environ.get("KONGCTL_E2E_KONNECT_BASE_URL", "").strip() or "https://us.api.konghq." + target
    parsed = urllib.parse.urlparse(base)
    assert parsed.scheme == "https" and parsed.hostname.endswith((".api.konghq.com", ".api.konghq.tech"))
    headers = {"Authorization": "Bearer " + os.environ["KONGCTL_E2E_KONNECT_PAT"], "Content-Type": "application/json"}
    config = ["--config-file", str(Path(os.environ["XDG_CONFIG_HOME"]) / "kongctl/config.yaml")]
    namespace, foreign_namespace = "identity-basic-e2e", "identity-basic-foreign-e2e"
    parent_namespace, directory_namespace = "identity-basic-principals-e2e", "identity-basic-directories-e2e"
    marker = {"e2e-basic-auth": name[-36:]}
    fixture_map = {"e2e-fixture": name[-36:]}
    needles, captures = [], []
    username = "worker-" + name

    def request(method, path, body=None):
        payload = None if body is None else json.dumps(body).encode()
        req = urllib.request.Request(base.rstrip("/") + path, method=method, headers=headers, data=payload)
        with urllib.request.urlopen(req, timeout=30) as response:
            value = response.read()
            return json.loads(value) if value else None

    def collection(path):
        result, cursors = [], set()
        while True:
            value = request("GET", path)
            assert isinstance(value.get("data"), list)
            result.extend(value["data"])
            next_page = value["meta"]["page"]["next"]
            if next_page is None or next_page == "":
                return result
            cursor = urllib.parse.parse_qs(urllib.parse.urlparse(next_page).query).get("page[after]", [None])[0]
            if cursor is None:
                assert "://" not in next_page and "?" not in next_page
                cursor = next_page
            assert cursor and cursor not in cursors
            cursors.add(cursor)
            path = path.split("?")[0] + "?" + urllib.parse.urlencode({"page[size]": 100, "page[after]": cursor})

    def clean(data):
        assert not any(needle in data for needle in needles)

    def invoke(label, args, failure=None, raw=False):
        output = [] if raw or args[0] == "plan" else ["-o", "json"]
        result = subprocess.run([binary, *args, *output, "--log-level", "trace", "--log-file",
                                 str(work / (label + ".log")), *config],
                                capture_output=True, timeout=90, check=False)
        clean(result.stdout)
        clean(result.stderr)
        (work / (label + ".stdout")).write_bytes(result.stdout)
        (work / (label + ".stderr")).write_bytes(result.stderr)
        if failure:
            assert result.returncode != 0
            assert failure.lower() in (result.stdout + result.stderr).decode().lower()
            return None
        assert result.returncode == 0
        return result.stdout.decode() if raw else json.loads(result.stdout)

    def file(label, document):
        path = work / (label + ".yaml")
        path.write_text(json.dumps(document, indent=2))
        return str(path)

    def plan(label, document, mode="apply", failure=None):
        value = invoke(label, ["plan", "--mode", mode, "-f", file(label, document)], failure)
        if value is not None:
            (work / (label + ".json")).write_text(json.dumps(value, indent=2))
        return value

    def saved(label, verb, plan_label, failure=None):
        value = invoke(label, [verb, "--plan", str(work / (plan_label + ".json")), "--auto-approve"], failure)
        if value is not None:
            assert value["summary"]["failed"] == 0
        return value

    def metadata(ns=namespace, protected=False):
        return {"tool": "kongctl", "version": "fixture", "kongctl-namespace": ns,
                "kongctl-protected": str(protected).lower()}

    def basic(ref, value, ns=namespace, protected=False):
        return {"ref": ref, "username": value, "kongctl": {"namespace": ns, "protected": protected}}

    def document(children=None, managed_parent=False):
        parent = {"ref": "worker", "directory": "people"}
        if managed_parent:
            parent.update(display_name="Basic-auth worker", type="agent", labels=marker,
                          kongctl={"namespace": parent_namespace, "protected": False})
        else:
            parent["_external"] = {"id": principal}
        if children is not None:
            parent["basic_auths"] = children
        return {"_defaults": {"kongctl": {"namespace": namespace}}, "identity": {
            "directories": [{"ref": "people", "_external": {"selector": {"matchFields": {"name": name}}}}],
            "principals": [parent]}}

    def capture(path, body):
        target_file = private / ("credential-" + str(len(captures)) + ".json")
        with os.fdopen(os.open(target_file, os.O_CREAT | os.O_EXCL | os.O_WRONLY, 0o600), "w+") as output:
            json.dump({"status": "creation_not_confirmed", "path": path}, output)
            output.flush()
            os.fsync(output.fileno())
            value = request("POST", path, body)
            assert value.get("secret")
            material = value.pop("secret")
            needles.extend((material.encode(), base64.b64encode(material.encode())))
            output.seek(0)
            output.truncate()
            json.dump({"status": "captured", "id": value["id"], "secret": material}, output)
            output.flush()
            os.fsync(output.fileno())
        captures.append(target_file)
        return value

    try:
        initial = {"_defaults": {"kongctl": {"namespace": namespace}}, "identity": {"directories": [{
            "ref": "people", "name": name, "labels": marker,
            "allow_all_control_planes": True, "allowed_control_planes": [], "vault": {"enabled": False},
            "kongctl": {"namespace": directory_namespace, "protected": True},
            "principals": [{"ref": "worker", "display_name": "Basic-auth worker", "type": "agent", "labels": marker,
                            "kongctl": {"namespace": parent_namespace, "protected": True},
                            "basic_auths": [basic("login", username)]}]}]}}
        initial_plan = plan("create", initial)
        assert initial_plan["summary"]["total_changes"] == 3
        changes = {v["resource_type"]: v for v in initial_plan["changes"]}
        child = changes["identity_basic_auth"]
        assert len(child["depends_on"]) == 2 and len(child["fields"]["managed_by"]) == 4
        assert child["namespace"] == namespace and "protecting_parent" not in child
        assert saved("create-apply", "apply", "create")["summary"]["applied"] == 3
        directory = invoke("directory", ["get", "identity", "directory", name])["id"]
        root = "/v2/directories/" + directory
        principal = collection(root + "/principals")[0]["id"]
        parent_path = root + "/principals/" + principal
        basic_path = parent_path + "/basic-auths"
        current = collection(basic_path)[0]
        assert current["username"] == username and len(current["managed_by"]) == 4
        assert "kongctl-ref" not in current["managed_by"]
        login_id = current["id"]
        assert plan("initial-no-op", initial)["summary"]["total_changes"] == 0
        explain = invoke("explain", ["explain", "identity.basic_auths"])
        assert explain["type"] == "object"
        assert "basic_auths:" in invoke("scaffold", ["scaffold", "identity.basic_auths"], raw=True)
        # Source refs are local: changing one does not rename or recreate a username.
        assert plan("ref-only-no-op", document([basic("different-local-ref", username)]))["summary"]["total_changes"] == 0
        protected = document([basic("login", username, protected=True)])
        assert plan("protect", protected)["summary"]["total_changes"] == 1
        assert "managed_by" in invoke("diff", ["diff", "--plan", str(work / "protect.json")], raw=True)
        saved("protect-apply", "apply", "protect")
        plan("protected-delete", protected, "delete", failure="protected")
        assert plan("unprotect", document([basic("login", username)]))["summary"]["total_changes"] == 1
        saved("unprotect-apply", "apply", "unprotect")
        # Saved updates must reject a later ownership change.
        plan("stale-protection", protected)
        request("PUT", basic_path + "/" + login_id, {"username": username, "managed_by": metadata(foreign_namespace)})
        saved("reject-stale-owner", "apply", "stale-protection", failure="namespace")
        request("PUT", basic_path + "/" + login_id, {"username": username, "managed_by": metadata()})
        adopted_username = "adopt-" + name
        adopted = request("POST", basic_path, {"username": adopted_username, "managed_by": fixture_map})
        adopted_password_path = basic_path + "/" + adopted["id"] + "/passwords"
        adopted_password = capture(adopted_password_path, {"managed_by": fixture_map, "expires_at": "2031-01-01T00:00:00Z"})
        adopt_doc = document([basic("adopted", adopted_username)])
        plan("reject-unowned", adopt_doc, failure="adopt")
        invoke("adopt", ["adopt", "identity", "principal-basic-auth", "--directory-id", directory,
                         "--principal-id", principal, "--username", adopted_username, "--namespace", namespace])
        assert plan("adopted-no-op", adopt_doc)["summary"]["total_changes"] == 0
        retained_password = request("GET", adopted_password_path + "/" + adopted_password["id"])
        assert retained_password["managed_by"] == fixture_map
        assert retained_password["expires_at"].startswith("2031-01-01T00:00:00")
        invoke("reject-adopt-owned", ["adopt", "identity", "principal-basic-auth", adopted["id"],
                                     "--directory-id", directory, "--principal-id", principal,
                                     "--namespace", foreign_namespace], failure="overwrite-namespace")
        foreign = request("POST", basic_path, {"username": "foreign-" + name, "managed_by": metadata(foreign_namespace)})
        unowned = request("POST", basic_path, {"username": "unowned-" + name, "managed_by": fixture_map})
        external_doc = document([{"ref": "external-login", "_external": {"selector": {"matchFields": {"username": username}}}}])
        assert plan("external-lookup", external_doc)["summary"]["total_changes"] == 0
        assert plan("external-only-sync", external_doc, "sync")["summary"]["total_changes"] == 0
        dumped = invoke("dump", ["dump", "declarative", "--resources", "identity.directories", "--filter-id", directory,
                                 "--include-child-resources", "--default-namespace", namespace], raw=True)
        assert "basic_auths:" in dumped and "managed_by:" not in dumped and "passwords:" not in dumped
        dump_file = work / "dump.yaml"
        dump_file.write_text(dumped)
        assert invoke("dump-no-op", ["plan", "--mode", "apply", "-f", str(dump_file)])["summary"]["total_changes"] == 0
        # The common platform-provided directory has no management metadata.
        # Its dump must retain external lookup without demanding adoption.
        fresh_directory = request("GET", root)
        directory_body = {k: fresh_directory[k] for k in (
            "name", "description", "allow_all_control_planes", "allowed_control_planes",
            "ttl_secs", "negative_ttl_secs", "vault_enabled", "labels", "managed_by")}
        original_management = directory_body["managed_by"]
        directory_body["managed_by"] = {}
        request("PUT", root, directory_body)
        external_dump = invoke("external-directory-dump", ["dump", "declarative", "--resources", "identity.directories",
                                                            "--filter-id", directory, "--include-child-resources",
                                                            "--default-namespace", namespace], raw=True)
        assert "_external:" in external_dump and "matchFields:" in external_dump
        external_dump_file = work / "external-directory-dump.yaml"
        external_dump_file.write_text(external_dump)
        assert invoke("external-directory-dump-no-op", ["plan", "--mode", "apply", "-f", str(external_dump_file)])["summary"]["total_changes"] == 0
        directory_body["managed_by"] = original_management
        request("PUT", root, directory_body)
        request("DELETE", adopted_password_path + "/" + adopted_password["id"])
        # A password attached after a saved delete must block the write.
        child_doc = document([basic("login", username)])
        plan("saved-delete", child_doc, "delete")
        password = capture(basic_path + "/" + login_id + "/passwords", {"managed_by": fixture_map})
        saved("reject-late-password", "delete", "saved-delete", failure="password")
        plan("reject-password-plan", child_doc, "delete", failure="password")
        assert request("GET", basic_path + "/" + login_id)["id"] == login_id
        request("DELETE", basic_path + "/" + login_id + "/passwords/" + password["id"])
        saved("saved-delete-after-retirement", "delete", "saved-delete")
        # Omitted collections preserve credentials; explicit empty scopes prune only this namespace.
        assert plan("omitted-sync", document(), "sync")["summary"]["total_changes"] == 0
        pruned = plan("empty-sync", document([]), "sync")
        assert pruned["summary"]["total_changes"] == 1
        saved("empty-sync-apply", "sync", "empty-sync")
        assert {v["id"] for v in collection(basic_path)} == {foreign["id"], unowned["id"]}
        # Parent deletion observes all credentials, regardless of ownership.
        parent_doc = document(managed_parent=True)
        plan("unprotect-parent", parent_doc)
        saved("unprotect-parent-apply", "apply", "unprotect-parent")
        key = capture(parent_path + "/api-keys", {"type": "v1", "managed_by": fixture_map})
        plan("reject-parent-key", parent_doc, "delete", failure="API keys")
        request("DELETE", parent_path + "/api-keys/" + key["id"])
        plan("reject-parent-basic", parent_doc, "delete", failure="basic-auth")
        for value in (foreign, unowned):
            assert collection(basic_path + "/" + value["id"] + "/passwords") == []
            request("DELETE", basic_path + "/" + value["id"])
        # Cross-namespace final plans must order authorized children before their principal.
        joint = document([basic("one", "one-" + name), basic("two", "two-" + name, foreign_namespace)], True)
        grouped = joint["identity"]["principals"][0]["basic_auths"].pop()
        grouped["principal"] = "worker"
        joint["identity"]["basic_auths"] = [grouped]
        plan("joint-create", joint)
        saved("joint-create-apply", "apply", "joint-create")
        deletion = plan("joint-delete", joint, "delete")
        assert deletion["summary"]["total_changes"] == 3
        parent_change = next(v for v in deletion["changes"] if v["resource_type"] == "identity_principal")
        assert len(parent_change["depends_on"]) == 2
        saved("joint-delete-apply", "delete", "joint-delete")
        saved("repeat-saved-delete", "delete", "joint-delete")
        assert collection(root + "/principals") == []
        assert captures and all(p.stat().st_mode & 0o077 == 0 for p in captures)
        for path in artifacts.rglob("*"):
            if path.is_file():
                clean(path.read_bytes())
        result = {"verified": True, "independent_ownership": True, "saved_plan_guards": True, "dump_reload": True,
                  "scoped_sync": True, "credential_deletion_guards": True, "artifacts_clean": True}
    finally:
        directories = [v for v in collection("/v2/directories") if v["name"] == name]
        assert len(directories) <= 1
        if directories:
            root = "/v2/directories/" + directories[0]["id"]
            assert directories[0]["labels"] == marker
            assert collection(root + "/groups") == []
            descendants, key_descendants = [], []
            parents = collection(root + "/principals")
            for parent in parents:
                assert parent["labels"] == marker
                path = root + "/principals/" + parent["id"]
                assert collection(path + "/identities") == []
                keys = collection(path + "/api-keys")
                assert all(v["managed_by"] == fixture_map for v in keys)
                key_descendants.extend((path, key) for key in keys)
                credentials = collection(path + "/basic-auths")
                for value in credentials:
                    m = value["managed_by"]
                    assert name in value["username"]
                    assert m == fixture_map or (m.get("tool") == "kongctl" and m.get("kongctl-namespace") in (namespace, foreign_namespace))
                    children = collection(path + "/basic-auths/" + value["id"] + "/passwords")
                    assert all(v["managed_by"] == fixture_map for v in children)
                    descendants.append((path, value, children))
            for path, key in key_descendants:
                assert request("GET", path + "/api-keys/" + key["id"])["managed_by"] == fixture_map
                request("DELETE", path + "/api-keys/" + key["id"])
            for path, value, children in descendants:
                child_path = path + "/basic-auths/" + value["id"]
                for child in children:
                    assert request("GET", child_path + "/passwords/" + child["id"])["managed_by"] == fixture_map
                    request("DELETE", child_path + "/passwords/" + child["id"])
                assert collection(child_path + "/passwords") == []
                assert request("GET", child_path)["managed_by"] == value["managed_by"]
                request("DELETE", child_path)
            for parent in parents:
                path = root + "/principals/" + parent["id"]
                assert collection(path + "/basic-auths") == [] and collection(path + "/api-keys") == []
                assert request("GET", path)["labels"] == marker
                request("DELETE", path)
            assert collection(root + "/principals") == []
    result["fixtures_removed"] = True
    return result


if __name__ == "__main__":
    try:
        print(json.dumps(main()))
    except Exception:
        lines = [frame.lineno for frame in traceback.extract_tb(sys.exc_info()[2]) if frame.filename == __file__]
        print("basic-auth lifecycle or verified cleanup failed at lines " + str(lines), file=sys.stderr)
        sys.exit(1)
