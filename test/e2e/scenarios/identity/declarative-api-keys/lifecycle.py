"""Verify generated-key declarations with private captures outside artifacts."""

import base64
import copy
import json
import os
from pathlib import Path
import secrets
import shutil
import subprocess
import sys
import tempfile
import urllib.parse
import urllib.request


def main():
    binary, directory, directory_name, artifact_dir = sys.argv[1:5]
    artifacts = Path(artifact_dir).resolve()
    scratch = tempfile.TemporaryDirectory(prefix="kongctl-declarative-keys-", dir=os.environ.get("TMPDIR"))
    work, private = Path(scratch.name), Path(scratch.name) / "private"
    private.mkdir(mode=0o700)
    assert not work.is_relative_to(artifacts.parents[2])
    target = os.environ.get("KONGCTL_E2E_KONNECT_ENV", "com").strip() or "com"
    assert target in ("com", "tech")
    base = os.environ.get("KONGCTL_E2E_KONNECT_BASE_URL", "").strip() or "https://us.api.konghq." + target
    host = urllib.parse.urlparse(base)
    assert host.scheme == "https" and host.hostname.endswith((".api.konghq.com", ".api.konghq.tech"))
    headers = {"Authorization": "Bearer " + os.environ["KONGCTL_E2E_KONNECT_PAT"], "Content-Type": "application/json"}
    config = ["--config-file", str(Path(os.environ["XDG_CONFIG_HOME"]) / "kongctl/config.yaml")]
    root, namespace = "/v2/directories/" + directory, "identity-declarative-keys-e2e"
    fixture_map = {"fixture": directory, "owner": "declarative-keys-e2e"}
    parents, known, needles = [], set(), []
    phase, success = "setup", False

    def remember(value):
        needles.extend((value.encode(), base64.b64encode(value.encode())))

    remember(os.environ["KONGCTL_E2E_KONNECT_PAT"])

    class NoRedirect(urllib.request.HTTPRedirectHandler):
        def redirect_request(self, req, fp, code, msg, hdrs, newurl):
            raise RuntimeError("unexpected API redirect")

    opener = urllib.request.build_opener(NoRedirect())

    def request(method, path, body=None):
        req = urllib.request.Request(base.rstrip("/") + path, method=method, headers=headers,
                                     data=None if body is None else json.dumps(body).encode())
        with opener.open(req, timeout=30) as response:
            data = response.read()
            return json.loads(data) if data else None

    def collection(path):
        result, cursors, route = [], set(), path.split("?")[0]
        while True:
            value = request("GET", path)
            assert isinstance(value.get("data"), list)
            result.extend(value["data"])
            after = value["meta"]["page"]["next"]
            if after in (None, ""):
                assert len({v["id"] for v in result}) == len(result)
                return result
            cursor = urllib.parse.parse_qs(urllib.parse.urlparse(after).query).get("page[after]", [None])[0]
            if cursor is None:
                assert "://" not in after and "?" not in after
                cursor = after
            assert cursor and cursor not in cursors
            cursors.add(cursor)
            path = route + "?" + urllib.parse.urlencode({"page[size]": 100, "page[after]": cursor})

    def invoke(label, args, failure=None, raw=False):
        nonlocal phase
        phase = label
        output = [] if raw or args[0] == "plan" else ["-o", "json"]
        result = subprocess.run([binary, *args, *output, "--log-level", "trace", "--log-file",
                                 str(work / (label + ".log")), *config],
                                capture_output=True, timeout=90, check=False)
        (work / (label + ".stdout")).write_bytes(result.stdout)
        (work / (label + ".stderr")).write_bytes(result.stderr)
        if failure:
            assert result.returncode != 0
            assert failure.lower() in (result.stdout + result.stderr).decode().lower()
            return None
        assert result.returncode == 0
        return result.stdout.decode() if raw else json.loads(result.stdout)

    def plan(label, document, mode="apply", failure=None):
        path = work / (label + ".yaml")
        path.write_text(json.dumps(document, indent=2))
        value = invoke(label, ["plan", "--mode", mode, "-f", str(path)], failure)
        if value is not None:
            (work / (label + ".json")).write_text(json.dumps(value, indent=2))
        return value

    def capture_files(result):
        paths = result.get("generated_files", [])
        # Public apply results keep capture locations in execution metadata.
        if not paths:
            paths = result.get("execution", {}).get("generated_files", [])
        assert paths
        for path in paths:
            path = Path(path)
            assert path.is_relative_to(private) and path.stat().st_mode & 0o077 == 0
            value = json.loads(path.read_text())
            assert value["status"] == "captured" and value["directory_id"] == directory
            assert value["resource_type"] in ("identity_principal_api_key", "identity_group_api_key")
            assert value["resource_ref"] and value["namespace"] == namespace and value["parent_ref"]
            remember(value["secret"])
            known.add(value["api_key_id"])
        return paths

    def saved(label, verb, source, capture=None, failure=None):
        args = [verb, "--plan", str(work / (source + ".json")), "--auto-approve"]
        if capture:
            args += ["--secret-output-dir", str(private / capture)]
        value = invoke(label, args, failure)
        if value is not None:
            assert value["summary"]["failed"] == 0
            if capture:
                assert len(capture_files(value)) == 2
        return value

    def route(kind, parent):
        return root + "/" + ("principals" if kind == "principal" else "groups") + "/" + parent

    def update_parent(kind, parent, metadata):
        path = route(kind, parent)
        current = request("GET", path)
        body = {k: current[k] for k in ("display_name", "description", "labels", "metadata") if k in current}
        if kind == "group":
            body["name"] = current["name"]
        body["managed_by"] = metadata
        request("PUT", path, body)

    def native_key(kind, parent, body):
        path = private / ("native-" + str(len(known)) + ".json")
        with os.fdopen(os.open(path, os.O_CREAT | os.O_EXCL | os.O_WRONLY, 0o600), "w+") as output:
            json.dump({"status": "creation_not_confirmed", "parent": parent}, output)
            output.flush()
            os.fsync(output.fileno())
            value = request("POST", route(kind, parent) + "/api-keys", body)
            known.add(value["id"])
            remember(value["secret"])
            output.seek(0)
            output.truncate()
            json.dump({"status": "captured", "id": value["id"], "secret": value.pop("secret")}, output)
            output.flush()
            os.fsync(output.fileno())
        return value

    try:
        original_directory = request("GET", root)
        assert original_directory["managed_by"] == {} and original_directory["labels"] == {}
        manifest = {"_defaults": {"kongctl": {"namespace": namespace}}, "identity": {
            "directories": [{"ref": "default-directory", "_external": {
                "selector": {"matchFields": {"name": directory_name}}}}],
            "principals": [{"ref": "worker", "directory": "default-directory", "display_name": "Generated-key worker",
                            "type": "agent", "api_keys": [{"ref": "worker-primary", "type": "v1", "name": "Worker primary",
                                                            "expires_at": "2030-01-01T00:00:00Z"}]}],
            "groups": [{"ref": "workers", "directory": "default-directory", "name": "GeneratedKeyWorkers", "type": "group",
                        "api_keys": [{"ref": "workers-primary", "type": "v1", "name": "Workers primary",
                                      "expires_at": "2030-01-01T00:00:00Z"}]}]}}
        initial = plan("create", manifest)
        assert initial["summary"]["total_changes"] == 4
        saved("missing-capture", "apply", "create", failure="secret-output-dir")
        assert collection(root + "/principals") == [] and collection(root + "/groups") == []
        saved("create-apply", "apply", "create", "initial")
        principal = collection(root + "/principals")[0]["id"]
        group = collection(root + "/groups")[0]["id"]
        parents.extend((("principal", principal), ("group", group)))
        old_ids, original_maps = {}, {}
        for kind, parent in parents:
            value = collection(route(kind, parent) + "/api-keys")[0]
            old_ids[kind] = value["id"]
            original_maps[kind] = value["managed_by"]
            assert set(value["managed_by"]) == {"tool", "version", "kongctl-ref", "kongctl-create-type"}
            assert value["managed_by"]["tool"] == "kongctl" and value["managed_by"]["kongctl-create-type"] == "v1"
            assert value["managed_by"]["version"] and value["expires_at"] == "2030-01-01T00:00:00Z"
        assert plan("create-no-op", manifest)["summary"]["total_changes"] == 0
        assert "No changes" in invoke("diff-no-op", ["diff", "-f", str(work / "create.yaml")], raw=True)
        saved("stale-create", "apply", "create", "stale", failure="already exists")
        assert not list((private / "stale").glob("*.json"))
        for resource in ("identity.principal_api_keys", "identity.group_api_keys"):
            label = resource.rsplit(".", 1)[1]
            explanation = invoke(label + "-explain", ["explain", resource])
            assert explanation["properties"]["expires_at"]["type"] == ["string", "null"]
            assert "api_keys:" in invoke(label + "-scaffold", ["scaffold", resource], raw=True)
        update = copy.deepcopy(manifest)
        update["identity"]["principals"][0]["api_keys"][0]["name"] = "Worker renamed"
        for field in ("principals", "groups"):
            update["identity"][field][0]["api_keys"][0].pop("expires_at")
        assert plan("rename", update)["summary"]["total_changes"] == 1
        saved("rename-apply", "apply", "rename")
        for kind, parent in parents:
            value = request("GET", route(kind, parent) + "/api-keys/" + old_ids[kind])
            assert value["expires_at"] == "2030-01-01T00:00:00Z" and value["managed_by"] == original_maps[kind]
        # Production currently ignores group-key renames and null names for
        # both key types. Retain these assertions in development until the API
        # fixes reach production, then remove this environment condition.
        if target == "tech":
            renamed = copy.deepcopy(update)
            renamed["identity"]["groups"][0]["api_keys"][0]["name"] = "Group renamed"
            assert plan("group-name-rename", renamed)["summary"]["total_changes"] == 1
            saved("group-rename-apply", "apply", "group-name-rename")
            cleared = copy.deepcopy(renamed)
            for field in ("principals", "groups"):
                cleared["identity"][field][0]["api_keys"][0]["name"] = None
            assert plan("clear-key-names", cleared)["summary"]["total_changes"] == 2
            saved("clear-key-names-apply", "apply", "clear-key-names")
            assert plan("clear-key-names-noop", cleared)["summary"]["total_changes"] == 0
            for kind, parent in parents:
                observed = request("GET", route(kind, parent) + "/api-keys/" + old_ids[kind])
                assert observed["name"] is None and observed["managed_by"] == original_maps[kind]
                assert observed["expires_at"] == "2030-01-01T00:00:00Z"
            assert plan("restore-key-names", update)["summary"]["total_changes"] == 2
            saved("restore-key-names-apply", "apply", "restore-key-names")
        clear = copy.deepcopy(update)
        for field in ("principals", "groups"):
            clear["identity"][field][0]["api_keys"][0]["expires_at"] = None
        assert plan("clear-expiration", clear)["summary"]["total_changes"] == 2
        for kind, parent in parents:
            duplicate = native_key(kind, parent, {"type": "v1", "managed_by": original_maps[kind]})
            plan(kind + "-duplicate-ref", clear, failure="multiple API keys")
            saved(kind + "-saved-duplicate", "apply", "clear-expiration", failure="multiple API keys")
            request("DELETE", route(kind, parent) + "/api-keys/" + duplicate["id"])
            legacy = native_key(kind, parent, {"type": "v1", "managed_by": {
                "tool": "kongctl", "kongctl-ref": original_maps[kind]["kongctl-ref"]}})
            plan(kind + "-legacy-collision", clear, failure="legacy")
            saved(kind + "-saved-legacy", "apply", "clear-expiration", failure="legacy")
            request("DELETE", route(kind, parent) + "/api-keys/" + legacy["id"])
        for kind, parent in parents:
            metadata = request("GET", route(kind, parent))["managed_by"]
            changed = dict(metadata, **{"kongctl-namespace": "foreign"})
            update_parent(kind, parent, changed)
            saved(kind + "-stale-owner", "apply", "clear-expiration", failure="namespace")
            update_parent(kind, parent, metadata)
        saved("clear-apply", "apply", "clear-expiration")
        assert plan("clear-no-op", clear)["summary"]["total_changes"] == 0
        protected = copy.deepcopy(clear)
        for field in ("principals", "groups"):
            protected["identity"][field][0]["kongctl"] = {"protected": True}
        assert plan("protect-parents", protected)["summary"]["total_changes"] == 2
        saved("protect-apply", "apply", "protect-parents")
        plan("protected-delete", protected, "delete", failure="protected")
        assert plan("unprotect-parents", clear)["summary"]["total_changes"] == 2
        saved("unprotect-apply", "apply", "unprotect-parents")
        dumped = invoke("dump", ["dump", "declarative", "--resources", "identity.directories", "--filter-id", directory,
                                 "--include-child-resources", "--default-namespace", namespace], raw=True)
        assert "api_keys:" in dumped and "expires_at: null" in dumped
        assert "managed_by:" not in dumped and "secret" not in dumped
        dump_file = work / "dump.yaml"
        dump_file.write_text(dumped)
        assert invoke("dump-no-op", ["plan", "--mode", "apply", "-f", str(dump_file)])["summary"]["total_changes"] == 0
        for kind, parent in parents:
            native_key(kind, parent, {"type": "v1", "managed_by": fixture_map})
            material = secrets.token_urlsafe(24)
            remember(material)
            native_key(kind, parent, {"type": "imported", "secret": material})
            native_key(kind, parent, {"type": "v1", "managed_by": {"tool": "kongctl", "kongctl-ref": "legacy-" + kind}})
            public = invoke(kind + "-pagination", ["get", "identity", kind + "-api-keys", "--directory-id", directory,
                                                   "--" + kind + "-id", parent, "--page-size", "1"])
            assert len(public) == 4
        plan("foreign-parent-delete-blocked", clear, "delete", failure="authorize each key retirement")
        omitted = copy.deepcopy(clear)
        for field in ("principals", "groups"):
            omitted["identity"][field][0].pop("api_keys")
        assert plan("omitted-collections", omitted, "sync")["summary"]["total_changes"] == 0
        replacement = copy.deepcopy(clear)
        for field in ("principals", "groups"):
            replacement["identity"][field][0]["api_keys"][0]["ref"] += "-next"
        rotation = plan("replacement", replacement, "sync")
        assert rotation["summary"]["total_changes"] == 4
        for deletion in (v for v in rotation["changes"] if v["action"] == "DELETE"):
            creation = next(v for v in rotation["changes"] if v["action"] == "CREATE" and v["resource_type"] == deletion["resource_type"])
            assert creation["id"] in deletion["depends_on"]
        saved("replacement-apply", "sync", "replacement", "replacement")
        for kind, parent in parents:
            values = collection(route(kind, parent) + "/api-keys")
            assert len(values) == 4 and old_ids[kind] not in {v["id"] for v in values}
        empty = copy.deepcopy(omitted)
        for field in ("principals", "groups"):
            empty["identity"][field][0]["api_keys"] = []
        assert plan("empty-collections", empty, "sync")["summary"]["total_changes"] == 2
        saved("empty-apply", "sync", "empty-collections")
        for kind, parent in parents:
            values = collection(route(kind, parent) + "/api-keys")
            assert len(values) == 3
            for value in values:
                assert value["id"] in known
                request("DELETE", route(kind, parent) + "/api-keys/" + value["id"])
        assert plan("recreate-for-parent-delete", clear)["summary"]["total_changes"] == 2
        saved("recreate-apply", "apply", "recreate-for-parent-delete", "final")
        deletion = plan("delete", clear, "delete")
        assert deletion["summary"]["total_changes"] == 4
        for parent_change in (v for v in deletion["changes"] if v["resource_type"] in ("identity_principal", "identity_group")):
            child = next(v for v in deletion["changes"] if v.get("parent", {}).get("id") == parent_change["resource_id"])
            assert child["id"] in parent_change["depends_on"]
        saved("delete-apply", "delete", "delete")
        parents.clear()
        assert collection(root + "/principals") == [] and collection(root + "/groups") == []
        assert request("GET", root) == original_directory
        success = True
    except Exception:
        raise RuntimeError("Generated-key declarative lifecycle failed during " + phase + "; private values withheld") from None
    finally:
        try:
            # Discover parents even when a create response/capture check failed
            # before normal ID bookkeeping, and tolerate already retired IDs.
            present = {}
            for kind, field in (("principal", "principals"), ("group", "groups")):
                for value in collection(root + "/" + field):
                    present[(kind, value["id"])] = value
                    metadata = value["managed_by"]
                    ours = metadata.get("tool") == "kongctl" and metadata.get("kongctl-namespace") == namespace
                    ours = ours and (metadata.get("kongctl-ref") == "worker" if kind == "principal"
                                     else value["name"] == "GeneratedKeyWorkers")
                    if ours and (kind, value["id"]) not in parents:
                        parents.append((kind, value["id"]))
            for capture_path in private.rglob("*.json"):
                captured = json.loads(capture_path.read_text())
                if captured.get("status") == "captured":
                    known.add(captured.get("api_key_id", captured.get("id")))
            for kind, parent in parents:
                if (kind, parent) not in present:
                    continue
                path = route(kind, parent)
                current = request("GET", path)
                assert current["managed_by"].get("tool") == "kongctl"
                assert current["managed_by"].get("kongctl-namespace") == namespace
                for value in collection(path + "/api-keys"):
                    metadata = value["managed_by"]
                    issued_here = set(metadata) == {"tool", "version", "kongctl-ref", "kongctl-create-type"}
                    issued_here = issued_here and metadata.get("tool") == "kongctl" and metadata.get("kongctl-create-type") == "v1"
                    issued_here = issued_here and metadata.get("kongctl-ref") in {
                        "worker-primary", "workers-primary", "worker-primary-next", "workers-primary-next"}
                    assert value["id"] in known or issued_here
                    assert metadata in ({}, fixture_map) or metadata.get("tool") == "kongctl"
                    assert request("GET", path + "/api-keys/" + value["id"])["managed_by"] == metadata
                    request("DELETE", path + "/api-keys/" + value["id"])
                assert collection(path + "/api-keys") == []
                if kind == "principal":
                    assert collection(path + "/identities") == [] and collection(path + "/basic-auths") == []
                else:
                    assert collection(path + "/membership") == []
                request("DELETE", path)
            if success:
                for path in work.iterdir():
                    if path.is_file():
                        assert not any(needle in path.read_bytes() for needle in needles)
                        shutil.copyfile(path, artifacts / path.name)
        finally:
            scratch.cleanup()
    assert not work.exists()
    (artifacts / "private-workspace.json").write_text(json.dumps({"path": str(work), "removed": True,
        "private_sources_and_captures_copied": False}) + "\n")
    return {"generated_declarations": True, "saved_plan_capture_preflight": True, "inherited_parent_authority": True,
            "principal_rename_and_nullable_expiration_updates": True,
            "deferred_checks": [] if target == "tech" else ["group_key_rename", "null_key_names"],
            "dump_reload": True, "foreign_imported_and_legacy_preserved": True,
            "explicit_collection_sync": True, "replacement_before_retirement": True, "child_before_parent_deletion": True,
            "duplicate_and_legacy_ref_guards": True,
            "private_workspace_removed": True, "fixtures_removed": True}


if __name__ == "__main__":
    try:
        print(json.dumps(main()))
    except Exception as error:
        if isinstance(error, RuntimeError):
            print(str(error), file=sys.stderr)
        print("Generated-key lifecycle or verified fixture cleanup failed", file=sys.stderr)
        sys.exit(1)
