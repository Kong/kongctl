"""Verify declarative provider configuration without claiming runtime enrollment."""

import base64
import json
import os
from pathlib import Path
import secrets
import subprocess
import sys
import urllib.error
import urllib.parse
import urllib.request


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None


def main():
    binary, location, directory, name = sys.argv[1:5]
    work = Path(location)
    target = os.environ.get("KONGCTL_E2E_KONNECT_ENV", "com").strip() or "com"
    assert target in ("com", "tech")
    base = os.environ.get("KONGCTL_E2E_KONNECT_BASE_URL", "").strip() or "https://us.api.konghq." + target
    host = urllib.parse.urlparse(base)
    assert host.scheme == "https" and host.hostname.endswith((".api.konghq.com", ".api.konghq.tech"))
    token = os.environ["KONGCTL_E2E_KONNECT_PAT"]
    hidden = [token, secrets.token_urlsafe(32), secrets.token_urlsafe(32), secrets.token_urlsafe(32)]
    needles = [value for text in hidden for value in (text.encode(), base64.b64encode(text.encode()))]
    headers = {"Authorization": "Bearer " + token, "Content-Type": "application/json"}
    opener = urllib.request.build_opener(NoRedirect())
    config = ["--config-file", str(Path(os.environ["XDG_CONFIG_HOME"]) / "kongctl/config.yaml")]
    namespace = "identity-vault-providers-e2e"
    root = "/v2/directories/" + directory
    phase, deleted, key_id = "setup", False, None
    known = {"billing-provider", "billing-replacement", "github-provider", "custom-provider", "unlisted-provider", "late-provider"}
    credential_ids = set()

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

    def collection(suffix="vault/providers"):
        value = request("GET", root + "/" + suffix)
        assert isinstance(value.get("data"), list) and value["meta"]["page"]["next"] in (None, "")
        if suffix == "vault/providers":
            assert all(p["directory_id"] == directory and p["name"] in known for p in value["data"])
        return value["data"]

    def invoke(label, args, output="json", failure=None):
        nonlocal phase
        phase = label
        log = work / (label + ".log")
        formatting = [] if args[0] in ("plan", "diff", "scaffold", "dump") else ["-o", output]
        result = subprocess.run([binary, *args, *formatting, "--log-level", "trace", "--log-file", str(log), *config],
            capture_output=True, timeout=120, check=False)
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

    def manifest(label, providers=None, grouped=False, external=False):
        parent = {"ref": "people", "name": name, "allow_all_control_planes": True, "vault": {"enabled": True}}
        if external:
            parent = {"ref": "people", "_external": {"id": directory}}
        identity = {"directories": [parent]}
        if providers is not None:
            providers = [dict(p) for p in providers]
            if grouped:
                for provider in providers:
                    provider["directory"] = "people"
                identity["vault"] = {"providers": providers}
            else:
                parent.setdefault("vault", {})["providers"] = providers
        data = json.dumps({"_defaults": {"kongctl": {"namespace": namespace}}, "identity": identity})
        data = data.replace('"DEFER_PROVIDER_SECRET"', '!secret {source: !env KONGCTL_E2E_VAULT_PROVIDER_SECRET}')
        path = work / (label + ".yaml")
        path.write_text(data)
        return str(path)

    def plan(label, source, mode="apply", failure=None, extra=()):
        return invoke(label, ["plan", "--mode", mode, "-f", source, *extra], failure=failure)

    def apply(label, source, verb="apply"):
        value = invoke(label, [verb, "-f", source, "--auto-approve"])
        assert value["summary"]["failed"] == 0
        return value

    def save(label, value):
        path = work / (label + ".json")
        path.write_text(json.dumps(value))
        return str(path)

    def management(selected_namespace=namespace, protected=False):
        current = request("GET", root)
        fields = ("name", "description", "allowed_control_planes", "allow_all_control_planes", "ttl_secs",
                  "negative_ttl_secs", "vault_enabled", "labels", "managed_by")
        body = {key: current[key] for key in fields}
        body["managed_by"] = {**current["managed_by"], "kongctl-namespace": selected_namespace,
                              "kongctl-protected": str(protected).lower()}
        assert request("PUT", root, body)["managed_by"] == body["managed_by"]

    static = {"ref": "billing", "name": "billing-provider", "template_name": "static_secret",
              "credential_type": "shared", "enabled": False, "refresh_buffer_seconds": 30}
    oauth = {"ref": "github", "name": "github-provider", "template_name": "github", "client_id": "upstream-fixture-client",
             "client_secret": "DEFER_PROVIDER_SECRET", "credential_type": "shared", "scopes": ["repo"],
             "enabled": True, "pkce_enabled": True, "refresh_buffer_seconds": 60,
             "token_endpoint": "https://fixture.example.test/token", "authorization_endpoint": "https://fixture.example.test/authorize"}
    dcr = {"ref": "custom", "name": "custom-provider", "template_name": "custom-oauth", "dcr_enabled": True,
           "credential_type": "user", "scopes": ["read"], "enabled": False, "pkce_enabled": True,
           "token_endpoint": "https://fixture.example.test/token", "authorization_endpoint": "https://fixture.example.test/authorize",
           "registration_endpoint": "https://fixture.example.test/register", "refresh_buffer_seconds": 0}

    try:
        current = request("GET", root)
        assert current["name"] == name and current["managed_by"] == {} and current["vault_enabled"] is True
        assert collection() == []
        schema = invoke("explain", ["explain", "identity.vault.providers"])
        assert schema["type"] == "object"
        scaffold = invoke("scaffold", ["scaffold", "identity.vault.providers"], output="text")
        assert "providers:" in scaffold and "template_name:" in scaffold
        initial = manifest("initial", [static, oauth, dcr])
        plan("unowned-parent-blocked", initial, failure="explicitly adopt")
        plan("external-parent-blocked", manifest("external", [static], external=True), failure="external parents are read-only")
        invoke("adopt-directory", ["adopt", "identity", "directory", name, "--namespace", namespace])
        os.environ.pop("KONGCTL_E2E_VAULT_PROVIDER_SECRET", None)
        creation = plan("create-plan", initial)
        assert creation["summary"]["by_action"] == {"CREATE": 3}
        assert sum(len(p.get("secret_writes", [])) for p in creation["changes"]) == 1
        os.environ["KONGCTL_E2E_VAULT_PROVIDER_SECRET"] = hidden[1]
        saved = save("create-plan", creation)
        assert invoke("apply-saved-create", ["apply", "--plan", saved, "--auto-approve"])["summary"]["failed"] == 0
        values = {p["name"]: p for p in collection()}
        assert len(values) == 3
        for configured in (static, oauth, dcr):
            observed = values[configured["name"]]
            for field, value in configured.items():
                if field in ("ref", "client_secret"):
                    continue
                actual = observed[field + "_effective"] if field.endswith("_endpoint") else observed[field]
                assert actual == value, field
            assert "managed_by" not in observed and "labels" not in observed and "client_secret" not in observed
        owner = request("GET", root)["managed_by"]
        assert owner["tool"] == "kongctl" and owner["kongctl-namespace"] == namespace
        assert plan("repeat-noop", initial)["summary"]["total_changes"] == 0
        updated = {**oauth, "enabled": False, "pkce_enabled": False, "refresh_buffer_seconds": 0,
                   "scopes": ["repo", "read:org"], "token_endpoint": "https://other.example.test/token",
                   "authorization_endpoint": "https://other.example.test/authorize"}
        changed = manifest("updated", [static, updated, {**dcr, "enabled": True}], grouped=True)
        update = plan("update-plan", changed)
        assert update["summary"]["by_action"] == {"UPDATE": 2}
        assert not any(p.get("secret_writes") for p in update["changes"])
        diff = invoke("diff-update", ["diff", "-f", changed], output="text")
        assert "refresh_buffer_seconds" in diff and "scopes" in diff
        apply("apply-update", changed)
        assert plan("update-noop", changed)["summary"]["total_changes"] == 0
        rotation = plan("explicit-secret-plan", changed, extra=("--write-secret", "github#client_secret"))
        assert len(rotation["changes"]) == 1
        change = rotation["changes"][0]
        assert change["namespace"] == namespace and change["parent"]["id"] == directory
        assert change["protecting_parent"]["resource_type"] == "identity_directory"
        saved = save("rotate-plan", rotation)
        before = collection()
        for reason in ("namespace", "protected"):
            try:
                management("foreign" if reason == "namespace" else namespace, reason == "protected")
                invoke("secret-" + reason + "-blocked", ["apply", "--plan", saved, "--auto-approve"], failure=reason)
                assert collection() == before
            finally:
                management()
        os.environ["KONGCTL_E2E_VAULT_PROVIDER_SECRET"] = hidden[2]
        assert invoke("apply-secret-only", ["apply", "--plan", saved, "--auto-approve"])["summary"]["failed"] == 0
        assert plan("secret-omission-noop", changed)["summary"]["total_changes"] == 0
        plan("unsupported-replacement", manifest("replacement", [static, {**updated, "client_id": "different-client"}, dcr]), failure="verified PATCH")
        plan("empty-scopes-rejected", manifest("empty-scopes", [{**updated, "scopes": []}]), failure="empty provider scopes")
        plan("empty-endpoint-rejected", manifest("empty-endpoint", [{**updated, "token_endpoint": ""}]), failure="empty provider endpoints")
        dump = work / "providers-dump.yaml"
        invoke("dump", ["dump", "declarative", "--resources", "identity.directories", "--include-child-resources",
            "--output-file", str(dump)], output="yaml")
        assert "providers:" in dump.read_text() and "client_secret:" not in dump.read_text()
        assert "registration_endpoint:" in dump.read_text()
        assert plan("dump-noop", str(dump))["summary"]["total_changes"] == 0
        apply("apply-dump", str(dump))
        assert plan("omitted-preserves", manifest("omitted"), "sync")["summary"]["total_changes"] == 0
        renamed = {**static, "name": "billing-replacement"}
        replacement = manifest("renamed", [renamed, updated, {**dcr, "enabled": True}])
        changes = plan("name-replacement-plan", replacement, "sync")["changes"]
        assert len(changes) == 2
        created = next(p for p in changes if p["action"] == "CREATE")
        retired = next(p for p in changes if p["action"] == "DELETE")
        assert created["id"] in retired["depends_on"]
        apply("name-replacement-sync", replacement, "sync")
        static, changed = renamed, replacement
        values = {p["name"]: p for p in collection()}
        assert "billing-provider" not in values and static["name"] in values
        request("POST", root + "/vault/providers", {"name": "unlisted-provider", "template_name": "static_secret", "credential_type": "shared"}, 201)
        assert plan("apply-preserves-unlisted", changed)["summary"]["total_changes"] == 0
        prune = plan("sync-plan", changed, "sync")
        assert prune["summary"]["by_action"] == {"DELETE": 1}
        apply("scoped-sync", changed, "sync")
        assert len(collection()) == 3
        public = json.loads((Path(__file__).parent.parent / "vault-keys/public-keys.json").read_text())[0]
        key = request("POST", root + "/vault/keys", {"name": "credential-fixture", "public_key": public["public_key"]}, 201)
        key_id = key["id"]
        provider = values[static["name"]]
        credential_path = root + "/vault/providers/" + provider["id"] + "/credentials"
        empty = manifest("empty", [])
        removal = plan("empty-sync-plan", empty, "sync")
        credential = request("POST", credential_path, {"secret": hidden[3]}, 201)
        credential_ids.add(credential["id"])
        plan("attached-credential-blocks-plan", empty, "sync", failure="attached")
        saved = save("remove-plan", removal)
        invoke("attached-credential-blocks-saved", ["sync", "--plan", saved, "--auto-approve"], failure="attached")
        assert request("GET", root + "/vault/providers/" + provider["id"])["id"] == provider["id"]
        request("DELETE", credential_path + "/" + credential["id"], expected=204)
        credential_ids.remove(credential["id"])
        apply("empty-sync", empty, "sync")
        assert collection() == []
        request("DELETE", root + "/vault/keys/" + key_id, expected=204)
        key_id = None
        single = manifest("single", [static])
        apply("recreate-selected", single)
        deletion = plan("delete-plan", single, "delete")
        child = next(p for p in deletion["changes"] if p["resource_type"] == "identity_vault_provider")
        parent = next(p for p in deletion["changes"] if p["resource_type"] == "identity_directory")
        assert child["id"] in parent["depends_on"]
        saved = save("delete-plan", deletion)
        late = request("POST", root + "/vault/providers", {"name": "late-provider", "template_name": "static_secret", "credential_type": "shared"}, 201)
        invoke("late-provider-blocks-parent", ["delete", "--plan", saved, "--auto-approve"], failure="attached Vault providers")
        assert request("GET", root)["id"] == directory and len(collection()) == 1
        request("DELETE", root + "/vault/providers/" + late["id"], expected=204)
        if name != "default":
            assert invoke("delete-saved", ["delete", "--plan", saved, "--auto-approve"])["summary"]["failed"] == 0
            deleted = True
            request("GET", root, expected=404)
            assert invoke("delete-repeat", ["delete", "--plan", saved, "--auto-approve"])["summary"]["failed"] == 0
            bootstrap = plan("bootstrap-plan", single)
            assert bootstrap["summary"]["by_action"] == {"CREATE": 2}
            apply("bootstrap-directory-and-provider", single)
            fresh = invoke("bootstrap-directory", ["get", "identity", "directory", name])
            directory, root, deleted = fresh["id"], "/v2/directories/" + fresh["id"], False
            assert len(collection()) == 1 and collection()[0]["enabled"] is False
            assert plan("bootstrap-noop", single)["summary"]["total_changes"] == 0
            apply("direct-delete", single, "delete")
            deleted = True
            request("GET", root, expected=404)
        for path in work.rglob("*"):
            if path.is_file():
                assert not any(needle in path.read_bytes() for needle in needles)
        if name == "default":
            assert request("GET", root)["id"] == directory
        return {"provider_modes": 3, "all_public_fields": True, "explicit_parent_adoption": True,
            "initial_disabled_state": True, "deferred_and_selected_secrets": True,
            "saved_authority_and_credential_guards": True, "scoped_sync_and_delete": True,
            "name_replacement_ordered": True,
            "dump_reload": True, "bootstrap_parent_and_provider": name != "default", "fixtures_removed": True}
    except Exception:
        raise RuntimeError("Vault provider phase failed: " + phase) from None
    finally:
        if not deleted:
            current = request("GET", root)
            assert current["id"] == directory and current["name"] == name and current["vault_enabled"] is True
            for provider in collection():
                path = root + "/vault/providers/" + provider["id"]
                for credential in collection("vault/providers/" + provider["id"] + "/credentials"):
                    assert credential["id"] in credential_ids
                    request("DELETE", path + "/credentials/" + credential["id"], expected=204)
                assert collection("vault/providers/" + provider["id"] + "/credentials") == []
                request("DELETE", path, expected=204)
            assert collection() == []
            if key_id is not None:
                request("DELETE", root + "/vault/keys/" + key_id, expected=204)


if __name__ == "__main__":
    try:
        print(json.dumps(main()))
    except Exception as error:
        print(str(error) if isinstance(error, RuntimeError) else "Vault provider scenario or guarded cleanup failed", file=sys.stderr)
        sys.exit(1)
