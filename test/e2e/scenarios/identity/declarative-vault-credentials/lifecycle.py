"""Verify static shared declarations without persisting credential material."""

import base64
import json
import os
from pathlib import Path
import secrets
import shutil
import subprocess
import sys
import tempfile
import urllib.error
import urllib.parse
import urllib.request


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None


def main():
    binary, location, directory, name = sys.argv[1:5]
    artifacts = Path(location)
    scratch = tempfile.TemporaryDirectory(prefix="kongctl-static-credentials-", dir=os.environ.get("TMPDIR"))
    work = Path(scratch.name)
    target = os.environ.get("KONGCTL_E2E_KONNECT_ENV", "com").strip() or "com"
    assert target in ("com", "tech")
    base = os.environ.get("KONGCTL_E2E_KONNECT_BASE_URL", "").strip() or "https://us.api.konghq." + target
    host = urllib.parse.urlparse(base)
    assert host.scheme == "https" and host.hostname.endswith((".api.konghq.com", ".api.konghq.tech"))
    token = os.environ["KONGCTL_E2E_KONNECT_PAT"]
    hidden = [token, secrets.token_urlsafe(32), secrets.token_urlsafe(32)]
    needles = [value for text in hidden for value in (text.encode(), base64.b64encode(text.encode()))]
    headers = {"Authorization": "Bearer " + token, "Content-Type": "application/json"}
    opener = urllib.request.build_opener(NoRedirect())
    config = ["--config-file", str(Path(os.environ["XDG_CONFIG_HOME"]) / "kongctl/config.yaml")]
    namespace = "identity-static-credentials-e2e"
    root = "/v2/directories/" + directory
    phase, deleted = "setup", False
    private = work / "private-static-credential-source"
    private.mkdir(mode=0o700)
    secret_file = private / "value"
    secret_file.write_text(hidden[2])
    secret_file.chmod(0o600)
    public_key = json.loads((Path(__file__).parent.parent / "vault-keys/public-keys.json").read_text())[0]["public_key"]

    def request(method, path, body=None, expected=200):
        req = urllib.request.Request(base.rstrip("/") + path, method=method, headers=headers,
            data=None if body is None else json.dumps(body).encode())
        try:
            response = opener.open(req, timeout=30)
        except urllib.error.HTTPError as error:
            response = error
        with response:
            data = response.read()
            assert response.status == expected
            return json.loads(data) if data and expected != 404 else None

    def collection(suffix):
        value = request("GET", root + "/" + suffix)
        assert isinstance(value.get("data"), list) and value["meta"]["page"]["next"] in (None, "")
        return value["data"]

    def credentials(provider):
        return collection("vault/providers/" + provider + "/credentials")

    def invoke(label, args, output="json", failure=None):
        nonlocal phase
        phase = label
        log = work / (label + ".log")
        formatting = [] if args[0] in ("plan", "diff", "scaffold", "dump") else ["-o", output]
        result = subprocess.run([binary, *args, *formatting, "--log-level", "trace", "--log-file", str(log), *config],
            cwd=work, capture_output=True, timeout=120, check=False)
        for data in (result.stdout, result.stderr, log.read_bytes() if log.exists() else b""):
            assert not any(needle in data for needle in needles)
            assert b"enrollment_url" not in data
        (work / (label + ".stdout")).write_bytes(result.stdout)
        (work / (label + ".stderr")).write_bytes(result.stderr)
        if failure:
            assert result.returncode != 0 and failure.lower() in (result.stdout + result.stderr).decode().lower(), label
            return None
        assert result.returncode == 0, label
        return json.loads(result.stdout) if output == "json" else result.stdout.decode()

    def manifest(label, values=None, grouped=False, keys=True, external=False):
        parent = {"ref": "people", "name": name, "allow_all_control_planes": True, "vault": {"enabled": True}}
        if external:
            parent = {"ref": "people", "_external": {"id": directory}}
        if keys:
            parent.setdefault("vault", {})["keys"] = [{"ref": "encryption", "name": "encryption", "public_key": public_key}]
        provider = {"ref": "billing", "name": "billing-static", "template_name": "static_secret", "credential_type": "shared"}
        identity = {"directories": [parent]}
        if grouped:
            provider["directory"] = "people"
            identity["vault"] = {"providers": [provider]}
            if values is not None:
                identity["vault"]["provider_credentials"] = [{**v, "provider": "billing"} for v in values]
        else:
            if values is not None:
                provider["shared_credentials"] = values
            parent.setdefault("vault", {})["providers"] = [provider]
        text = json.dumps({"_defaults": {"kongctl": {"namespace": namespace}}, "identity": identity})
        text = text.replace('"ENV_SOURCE"', '!secret {source: !env KONGCTL_E2E_STATIC_VAULT_SECRET}')
        text = text.replace('"FILE_SOURCE"', '!secret {source: !file ./private-static-credential-source/value}')
        path = work / (label + ".yaml")
        path.write_text(text)
        return str(path)

    def plan(label, source, mode="apply", extra=(), failure=None):
        return invoke(label, ["plan", "--mode", mode, "-f", source, *extra], failure=failure)

    def save(label, value):
        path = work / (label + ".json")
        path.write_text(json.dumps(value))
        return str(path)

    def execute(label, value, failure=None):
        verb = value["metadata"]["mode"]
        assert verb in ("apply", "sync", "delete")
        result = invoke(label, [verb, "--plan", save(label, value), "--auto-approve"], failure=failure)
        if not failure:
            assert result["summary"]["failed"] == 0

    def management(selected_namespace=namespace, protected=False):
        current = request("GET", root)
        fields = ("name", "description", "allowed_control_planes", "allow_all_control_planes", "ttl_secs",
                  "negative_ttl_secs", "vault_enabled", "labels", "managed_by")
        body = {key: current[key] for key in fields}
        body["managed_by"] = {**current["managed_by"], "kongctl-namespace": selected_namespace,
                              "kongctl-protected": str(protected).lower()}
        request("PUT", root, body)

    fallback = {"ref": "fallback", "secret": "ENV_SOURCE"}
    standard = {"ref": "standard", "selector": "true", "priority": 0, "secret": "FILE_SOURCE"}
    priority = {"ref": "priority-rule", "selector": "true", "priority": 3, "secret": "ENV_SOURCE"}
    values = [fallback, standard, priority]
    try:
        assert request("GET", root)["name"] == name and collection("vault/providers") == []
        schema = invoke("explain", ["explain", "identity.vault.provider_credentials"])
        assert schema["type"] == "object"
        assert "provider_credentials:" in invoke("scaffold", ["scaffold", "identity.vault.provider_credentials"], output="text")
        source = manifest("initial", values)
        plan("unowned-parent-blocked", source, failure="explicitly adopt")
        plan("external-parent-blocked", manifest("external", values, external=True, keys=False), failure="external")
        invoke("adopt-directory", ["adopt", "identity", "directory", name, "--namespace", namespace])
        plan("missing-material-blocked", manifest("missing-material", [{"ref": "no-source"}]), failure="deferred secret")
        plan("missing-key-blocked", manifest("missing-key", values, keys=False), failure="encryption key")
        os.environ.pop("KONGCTL_E2E_STATIC_VAULT_SECRET", None)
        creation = plan("create-plan", source)
        assert creation["summary"]["total_changes"] == 5
        execute("missing-environment-before-write", creation, failure="secret source")
        assert collection("vault/providers") == [] and collection("vault/keys") == []
        os.environ["KONGCTL_E2E_STATIC_VAULT_SECRET"] = hidden[1]
        execute("apply-create", creation)
        providers = collection("vault/providers")
        assert len(providers) == 1
        provider = providers[0]["id"]
        observed = credentials(provider)
        assert len(observed) == 3 and {c["priority"] for c in observed} == {None, 0, 3}
        assert all(c["credential_type"] == "shared" and c["status"] == "active" for c in observed)
        assert all("managed_by" not in c and "labels" not in c for c in observed)
        assert request("GET", root)["managed_by"]["kongctl-namespace"] == namespace
        assert plan("repeat-noop", source)["summary"]["total_changes"] == 0
        grouped = manifest("grouped", values, grouped=True)
        assert plan("grouped-noop", grouped)["summary"]["total_changes"] == 0
        renamed_ref = manifest("ref-renamed", [{**fallback, "ref": "renamed-fallback"}, standard, priority])
        assert plan("ref-rename-noop", renamed_ref)["summary"]["total_changes"] == 0
        rotation = plan("explicit-rotation", grouped, extra=("--write-secret", "standard#secret"))
        assert len(rotation["changes"]) == 1
        change = rotation["changes"][0]
        assert change["parent"]["id"] == provider and change["references"]["directory_id"]["id"] == directory
        assert change["protecting_parent"]["resource_type"] == "identity_directory"
        before = credentials(provider)
        for reason in ("namespace", "protected"):
            try:
                management("foreign" if reason == "namespace" else namespace, reason == "protected")
                execute("saved-" + reason, rotation, failure=reason)
                assert credentials(provider) == before
            finally:
                management()
        secret_file.unlink()
        execute("missing-file-before-write", rotation, failure="secret source")
        secret_file.write_text(hidden[2])
        secret_file.chmod(0o600)
        execute("apply-rotation", rotation)
        assert {(c["id"], c["selector"], c["priority"]) for c in credentials(provider)} == {(c["id"], c["selector"], c["priority"]) for c in before}
        by_priority = {c["priority"]: c for c in credentials(provider)}
        rule_path = root + "/vault/providers/" + provider + "/credentials/" + by_priority[0]["id"]
        request("PUT", rule_path, {"secret": hidden[2], "selector": "false", "priority": 0})
        execute("saved-routing-blocked", rotation, failure="routing changed")
        request("PUT", rule_path, {"secret": hidden[2], "selector": "true", "priority": 0})
        key = collection("vault/keys")[0]
        request("DELETE", root + "/vault/keys/" + key["id"], expected=204)
        execute("saved-key-blocked", rotation, failure="encryption key")
        request("POST", root + "/vault/keys", {"name": "encryption", "public_key": public_key}, expected=201)
        updated = [fallback, {**standard, "selector": "false"}, priority]
        source = manifest("updated", updated)
        plan("routing-needs-explicit-material", source, failure="explicit secret-write")
        diff = invoke("diff-routing", ["diff", "-f", source, "--write-secret", "standard#secret"], output="text")
        assert "selector" in diff
        execute("apply-routing", plan("routing-plan", source, extra=("--write-secret", "standard#secret")))
        assert plan("routing-noop", source)["summary"]["total_changes"] == 0
        replacement_values = [fallback, updated[1], {**priority, "priority": 7}]
        replacement_source = manifest("replacement", replacement_values)
        replacement = plan("replacement-plan", replacement_source, mode="sync")
        writes = [c for c in replacement["changes"] if c["resource_type"] == "identity_vault_provider_credential"]
        assert {c["action"] for c in writes} == {"CREATE", "DELETE"}
        create_id = next(c["id"] for c in writes if c["action"] == "CREATE")
        assert create_id in next(c for c in writes if c["action"] == "DELETE")["depends_on"]
        execute("apply-replacement", replacement)
        assert {c["priority"] for c in credentials(provider)} == {None, 0, 7}
        dump = work / "dump.yaml"
        invoke("dump", ["dump", "declarative", "--resources", "identity.directories", "--include-child-resources", "--output-file", str(dump)], output="text")
        text = dump.read_text()
        assert "shared_credentials:" in text and "secret:" not in text
        assert not any(needle in dump.read_bytes() for needle in needles)
        assert plan("dump-noop", str(dump))["summary"]["total_changes"] == 0
        assert plan("omitted-preserved", manifest("omitted"), mode="sync")["summary"]["total_changes"] == 0
        execute("scoped-prune", plan("scoped-prune-plan", manifest("scoped-prune", [fallback]), mode="sync"))
        assert len(credentials(provider)) == 1
        plan("parent-without-retirement-blocked", manifest("parent-only"), mode="delete", failure="attached")
        deletion = plan("delete-plan", manifest("delete", [fallback]), mode="delete")
        late = request("POST", root + "/vault/providers/" + provider + "/credentials",
            {"secret": hidden[1], "selector": "true", "priority": 11}, expected=201)
        execute("late-credential-blocks-parent", deletion, failure="attached")
        assert any(c["id"] == late["id"] for c in credentials(provider))
        request("DELETE", root + "/vault/providers/" + provider + "/credentials/" + late["id"], expected=204)
        if name != "default":
            execute("delete-with-retirement", deletion)
            request("GET", root, expected=404)
            deleted = True
            bootstrap_source = manifest("bootstrap", values)
            bootstrap = plan("bootstrap-plan", bootstrap_source)
            assert bootstrap["summary"]["total_changes"] == 6
            execute("bootstrap", bootstrap)
            directories = request("GET", "/v2/directories")["data"]
            matches = [value for value in directories if value["name"] == name]
            assert len(matches) == 1
            directory = matches[0]["id"]
            root = "/v2/directories/" + directory
            deleted = False
            assert len(credentials(collection("vault/providers")[0]["id"])) == 3
            execute("bootstrap-delete", plan("bootstrap-delete-plan", bootstrap_source, mode="delete"))
            request("GET", root, expected=404)
            deleted = True
    except Exception:
        raise RuntimeError("Static credential lifecycle failed during " + phase + "; private responses withheld") from None
    finally:
        os.environ.pop("KONGCTL_E2E_STATIC_VAULT_SECRET", None)
        shutil.rmtree(private)
        try:
            if not deleted:
                for p in collection("vault/providers"):
                    assert p["name"] == "billing-static" and p["template_name"] == "static_secret"
                    for c in credentials(p["id"]):
                        assert c["credential_type"] == "shared"
                        request("DELETE", root + "/vault/providers/" + p["id"] + "/credentials/" + c["id"], expected=204)
                    request("DELETE", root + "/vault/providers/" + p["id"], expected=204)
                for key in collection("vault/keys"):
                    assert key["name"] == "encryption"
                    request("DELETE", root + "/vault/keys/" + key["id"], expected=204)
            for path in work.rglob("*"):
                if path.is_file():
                    assert not any(needle in path.read_bytes() for needle in needles)
            for path in work.iterdir():
                assert path.is_file()
                shutil.copyfile(path, artifacts / path.name)
        finally:
            scratch.cleanup()
    assert not work.exists()
    print(json.dumps({"static_shared_slots": 3, "deferred_sources_and_explicit_replacement": True,
        "inherited_parent_authority": True, "saved_routing_and_key_guards": True,
        "scoped_sync_and_retirement": True, "dump_without_material": True,
        "bootstrap_dependencies": name != "default", "fixtures_removed": True, "private_workspace_removed": True}))


if __name__ == "__main__":
    main()
