"""Exercise public encryption keys under explicitly managed directory authority."""

import base64
import json
import os
from pathlib import Path
import subprocess
import sys
import urllib.error
import urllib.parse
import urllib.request


def main():
    binary, location, directory, name = sys.argv[1:5]
    work = Path(location)
    public = json.loads((Path(__file__).parent / "public-keys.json").read_text())
    target = os.environ.get("KONGCTL_E2E_KONNECT_ENV", "com").strip() or "com"
    assert target in ("com", "tech")
    base = os.environ.get("KONGCTL_E2E_KONNECT_BASE_URL", "").strip() or "https://us.api.konghq." + target
    host = urllib.parse.urlparse(base)
    assert host.scheme == "https" and host.hostname.endswith((".api.konghq.com", ".api.konghq.tech"))
    token = os.environ["KONGCTL_E2E_KONNECT_PAT"]
    needles = (token.encode(), base64.b64encode(token.encode()))
    headers = {"Authorization": "Bearer " + token, "Content-Type": "application/json"}
    config = ["--config-file", str(Path(os.environ["XDG_CONFIG_HOME"]) / "kongctl/config.yaml")]
    root = "/v2/directories/" + directory
    namespace = "identity-vault-keys-e2e"
    known = {value["kid"]: value for value in public}
    deleted = False
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
            return json.loads(data) if data and expected != 404 else None

    def collection():
        value = request("GET", root + "/vault/keys")
        assert isinstance(value.get("data"), list) and value["meta"]["page"]["next"] in (None, "")
        for key in value["data"]:
            assert key["directory_id"] == directory and key["kid"] in known
        return value["data"]

    def invoke(label, args, output="json", failure=None):
        nonlocal phase
        phase = label
        log = work / (label + ".log")
        formatting = [] if args[0] in ("plan", "diff", "scaffold", "dump") else ["-o", output]
        result = subprocess.run([binary, *args, *formatting, "--log-level", "trace", "--log-file", str(log), *config],
            capture_output=True, timeout=90, check=False)
        for data in (result.stdout, result.stderr, log.read_bytes() if log.exists() else b""):
            assert not any(needle in data for needle in needles)
            assert b"BEGIN PRIVATE KEY" not in data
        (work / (label + ".stdout")).write_bytes(result.stdout)
        (work / (label + ".stderr")).write_bytes(result.stderr)
        if failure:
            assert result.returncode != 0 and failure.lower() in (result.stdout + result.stderr).decode().lower(), label
            return None
        assert result.returncode == 0, label
        return json.loads(result.stdout) if output == "json" else result.stdout.decode()

    def file(label, value):
        path = work / (label + ".yaml")
        path.write_text(json.dumps(value))
        return str(path)

    def manifest(label, indices=None, renamed=False, grouped=False, external=False, crlf=False):
        parent = {"ref": "people", "name": name, "allow_all_control_planes": True, "vault": {"enabled": True}}
        if external:
            parent = {"ref": "people", "_external": {"id": directory}}
        identity = {"directories": [parent]}
        if indices is not None:
            keys = [{"ref": "encryption-" + str(i), "name": "primary" if renamed and i == 0 else "shared-name",
                "public_key": public[i]["public_key"].replace("\n", "\r\n") if crlf else public[i]["public_key"]}
                for i in indices]
            if grouped:
                for key in keys:
                    key["directory"] = "people"
                identity["vault"] = {"keys": keys}
            else:
                parent.setdefault("vault", {})["keys"] = keys
        return file(label, {"_defaults": {"kongctl": {"namespace": namespace}}, "identity": identity})

    def plan(label, source, mode="apply", failure=None):
        return invoke(label, ["plan", "--mode", mode, "-f", source], failure=failure)

    def apply(label, source, verb="apply"):
        value = invoke(label, [verb, "-f", source, "--auto-approve"])
        assert value["summary"]["failed"] == 0
        return value

    def management(selected_namespace=namespace, protected=False):
        current = request("GET", root)
        fields = ("name", "description", "allowed_control_planes", "allow_all_control_planes", "ttl_secs",
                  "negative_ttl_secs", "vault_enabled", "labels", "managed_by")
        body = {key: current[key] for key in fields}
        body["managed_by"] = {**current["managed_by"], "kongctl-namespace": selected_namespace,
                              "kongctl-protected": str(protected).lower()}
        result = request("PUT", root, body)
        assert result["managed_by"] == body["managed_by"]

    def native_create(index):
        value = request("POST", root + "/vault/keys", {"name": "shared-name", "public_key": public[index]["public_key"]}, 201)
        assert value["kid"] == public[index]["kid"]
        return value

    try:
        original = request("GET", root)
        assert original["name"] == name and original["managed_by"] == {} and original["vault_enabled"] is True
        assert collection() == []
        # Schema commands need no credential or remote ownership.
        schema = invoke("explain", ["explain", "identity.vault.keys"])
        assert schema["type"] == "object"
        scaffold = invoke("scaffold", ["scaffold", "identity.vault.keys"], output="text")
        assert "identity:" in scaffold and "keys:" in scaffold and "public_key:" in scaffold
        assert invoke("empty-read", ["get", "identity", "vault-keys", "--directory-id", directory]) == []
        native_create(0)
        initial = manifest("initial", [0, 1, 2])
        plan("unowned-parent-blocked", initial, failure="explicitly adopt")
        plan("external-parent-read-only", manifest("external", [0], external=True), failure="external parents are read-only")
        adopted = invoke("adopt-directory", ["adopt", "identity", "directory", name, "--namespace", namespace])
        assert adopted["namespace"] == namespace
        owner = request("GET", root)["managed_by"]
        assert owner["tool"] == "kongctl" and owner["kongctl-namespace"] == namespace and len(owner) == 4
        creation = plan("plan-create", initial)
        assert creation["summary"]["total_changes"] == 2
        saved = work / "create-plan.json"
        saved.write_text(json.dumps(creation))
        assert invoke("apply-saved-create", ["apply", "--plan", str(saved), "--auto-approve"])["summary"]["failed"] == 0
        values = invoke("paged-list", ["get", "identity", "vault-keys", "--directory-name", name, "--page-size", "1"])
        assert len(values) == 3 and len({value["name"] for value in values}) == 1
        before = {value["kid"]: value for value in values}
        for value in values:
            expected = known[value["kid"]]
            assert set(value) == {"id", "directory_id", "name", "public_key", "kid", "algorithm", "key_size", "created_at", "updated_at"}
            assert value["public_key"] == expected["public_key"] and value["algorithm"] == expected["algorithm"]
            assert value["key_size"] == expected["bits"] and value["created_at"] and value["updated_at"]
            detail = invoke("detail-" + value["id"], ["get", "identity", "vault-key", value["id"], "--directory-id", directory])
            assert detail == value
        selected = invoke("kid-lookup", ["get", "konnect", "identity", "vault-key", "--kid", public[2]["kid"],
            "--directory-id", directory, "--page-size", "1"])
        assert selected == before[public[2]["kid"]]
        for label in ("paged-list", "kid-lookup"):
            requests = [line for line in (work / (label + ".log")).read_text().splitlines()
                        if "log_type=http_request " in line and root + "/vault/keys " in line]
            assert len(requests) == 3 and all("page[size]:1" in line for line in requests)
            assert sum("page[after]:" in line for line in requests) == 2
        text = invoke("text-detail", ["get", "identity", "vault-key", "--directory-name", name, "--kid", public[0]["kid"]], output="text")
        assert "KID" in text and "KEY_SIZE" in text and public[0]["kid"] in text and "{" not in text
        yaml = invoke("yaml-list", ["list", "identity", "vault-keys", "--directory-id", directory], output="yaml")
        assert "public_key:" in yaml and "BEGIN PUBLIC KEY" in yaml
        assert plan("repeat-apply", initial)["summary"]["total_changes"] == 0
        assert plan("equivalent-pem", manifest("equivalent", [0, 1, 2], crlf=True))["summary"]["total_changes"] == 0
        renamed = manifest("rename", [0, 1, 2], renamed=True, grouped=True)
        rename = plan("rename-plan", renamed)
        assert len(rename["changes"]) == 1 and rename["changes"][0]["action"] == "UPDATE"
        assert set(rename["changes"][0]["changed_fields"]) == {"name"}
        saved = work / "rename-plan.json"
        saved.write_text(json.dumps(rename))
        for reason in ("namespace", "protected"):
            try:
                management("foreign" if reason == "namespace" else namespace, reason == "protected")
                invoke("saved-" + reason + "-blocked", ["apply", "--plan", str(saved), "--auto-approve"], failure=reason)
                assert {value["kid"]: value for value in collection()} == before
            finally:
                management()
        assert invoke("saved-rename", ["apply", "--plan", str(saved), "--auto-approve"])["summary"]["failed"] == 0
        first = request("GET", root + "/vault/keys/" + before[public[0]["kid"]]["id"])
        assert first["kid"] == public[0]["kid"] and first["name"] == "primary"
        assert plan("rename-noop", renamed)["summary"]["total_changes"] == 0
        before_disabled = {value["kid"]: value for value in collection()}
        try:
            assert request("PATCH", root, {"vault_enabled": False})["vault_enabled"] is False
            invoke("disabled-read", ["get", "identity", "vault-keys", "--directory-id", directory], failure="HTTP 403")
            plan("disabled-plan", renamed, failure="explicitly enable Vault")
            assert request("GET", root)["vault_enabled"] is False
        finally:
            assert request("PATCH", root, {"vault_enabled": True})["vault_enabled"] is True
        assert {value["kid"]: value for value in collection()} == before_disabled
        invoke("diff-noop", ["diff", "-f", renamed], output="text")
        dump = work / "dump.yaml"
        invoke("dump", ["dump", "declarative", "--resources", "identity.directories", "--include-child-resources",
            "--output-file", str(dump)], output="yaml")
        assert "keys:" in dump.read_text() and "BEGIN PUBLIC KEY" in dump.read_text()
        assert plan("dump-noop", str(dump))["summary"]["total_changes"] == 0
        apply("reload-dump", str(dump))
        assert plan("omitted-preserves", manifest("omitted"), "sync")["summary"]["total_changes"] == 0
        # A plain grouping object cannot acquire its own lifecycle or select
        # omitted child collections. Native API payloads still use vault_enabled.
        for label, group in (("group-omitted", None), ("empty-group", {})):
            source = json.loads(Path(manifest(label)).read_text())
            parent = source["identity"]["directories"][0]
            if group is None:
                del parent["vault"]
            else:
                parent["vault"] = group
            assert plan(label + "-sync", file(label, source), "sync")["summary"]["total_changes"] == 0
        for label, fields in (
            ("null-group", {"vault": None}),
            ("null-keys", {"vault": {"keys": None}}),
            ("legacy-key", {"vault_keys": []}),
            ("mixed-format", {"vault": {}, "vault_enabled": True}),
        ):
            source = {"identity": {"directories": [{"ref": "people", "name": name, **fields}]}}
            plan(label + "-rejected", file(label, source), failure="failed to load configuration")
        native_create(3)
        sync = plan("explicit-sync-plan", renamed, "sync")
        assert sync["summary"]["by_action"] == {"DELETE": 1}
        apply("explicit-sync", renamed, "sync")
        assert len(collection()) == 3
        rotated = manifest("rotation", [3])
        rotation = plan("rotation-plan", rotated, "sync")
        creates = [value for value in rotation["changes"] if value["action"] == "CREATE"]
        retires = [value for value in rotation["changes"] if value["action"] == "DELETE"]
        assert len(creates) == 1 and len(retires) == 3
        assert all(creates[0]["id"] in value["depends_on"] for value in retires)
        apply("rotate", rotated, "sync")
        assert len(collection()) == 1 and collection()[0]["kid"] == public[3]["kid"]
        capacity = manifest("capacity", [0, 1, 2, 3, 4])
        apply("fill-capacity", capacity)
        assert len(collection()) == 5
        plan("quota-blocked", manifest("six", list(range(6))), "sync", failure="five Vault keys")
        assert len(collection()) == 5
        apply("retire-explicitly", manifest("four", [0, 1, 2, 3]), "sync")
        apply("add-after-retirement", manifest("five-new", [0, 1, 2, 3, 5]))
        assert len(collection()) == 5 and any(value["kid"] == public[5]["kid"] for value in collection())
        apply("empty-collection", manifest("empty", []), "sync")
        assert collection() == [] and request("GET", root)["id"] == directory
        apply("recreate-for-delete", rotated)
        deletion = plan("delete-plan", rotated, "delete")
        assert len(deletion["changes"]) == 2
        child = next(value for value in deletion["changes"] if value["resource_type"] == "identity_vault_key")
        parent = next(value for value in deletion["changes"] if value["resource_type"] == "identity_directory")
        assert child["id"] in parent["depends_on"]
        saved = work / "delete-plan.json"
        saved.write_text(json.dumps(deletion))
        late = native_create(1)
        invoke("late-attachment-blocks-parent", ["delete", "--plan", str(saved), "--auto-approve"], failure="attached Vault keys")
        assert request("GET", root)["id"] == directory
        assert [value["id"] for value in collection()] == [late["id"]]
        request("DELETE", root + "/vault/keys/" + late["id"], expected=204)
        assert invoke("delete-saved", ["delete", "--plan", str(saved), "--auto-approve"])["summary"]["failed"] == 0
        deleted = True
        request("GET", root, expected=404)
        assert invoke("delete-repeat", ["delete", "--plan", str(saved), "--auto-approve"])["summary"]["failed"] == 0
        # Exercise a missing directory and its key in one plan, including
        # runtime parent-ID resolution, then direct declarative teardown.
        bootstrap = plan("bootstrap-plan", rotated)
        assert bootstrap["summary"]["total_changes"] == 2
        apply("bootstrap-parent-and-key", rotated)
        fresh_parent = invoke("bootstrap-directory", ["get", "identity", "directory", name])
        directory = fresh_parent["id"]
        root = "/v2/directories/" + directory
        deleted = False
        assert fresh_parent["vault_enabled"] is True and len(collection()) == 1
        assert plan("bootstrap-noop", rotated)["summary"]["total_changes"] == 0
        apply("direct-delete", rotated, "delete")
        deleted = True
        request("GET", root, expected=404)
        return {"public_key_algorithms": 2, "output_formats": 3, "real_pagination": True,
            "explicit_parent_adoption": True, "fingerprint_identity_and_rename": True,
            "scoped_sync_and_rotation": True, "saved_authority_checks": True, "five_key_limit": True,
            "dump_reload": True, "guarded_ordered_deletion": True, "fixtures_removed": True}
    except Exception:
        raise RuntimeError("Vault public key phase failed: " + phase) from None
    finally:
        if not deleted:
            current = request("GET", root)
            assert current["id"] == directory and current["name"] == name and current["vault_enabled"] is True
            for key in collection():
                assert key["kid"] in known
                fresh = request("GET", root + "/vault/keys/" + key["id"])
                assert fresh["id"] == key["id"] and fresh["kid"] == key["kid"]
                request("DELETE", root + "/vault/keys/" + key["id"], expected=204)
            assert collection() == []


if __name__ == "__main__":
    try:
        print(json.dumps(main()))
    except Exception as error:
        print(str(error) if isinstance(error, RuntimeError) else "Vault public key scenario or verified cleanup failed", file=sys.stderr)
        sys.exit(1)
