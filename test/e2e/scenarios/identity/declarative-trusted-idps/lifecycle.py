"""Verify trust declarations without inferring runtime JIT or principal ownership."""

import base64
import json
import os
from pathlib import Path
import subprocess
import sys
import urllib.error
import urllib.parse
import urllib.request


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None


def main():
    binary, location, directory, name, server, server_name = sys.argv[1:7]
    work = Path(location)
    target = os.environ.get("KONGCTL_E2E_KONNECT_ENV", "com").strip() or "com"
    assert target in ("com", "tech")
    base = os.environ.get("KONGCTL_E2E_KONNECT_BASE_URL", "").strip() or "https://us.api.konghq." + target
    host = urllib.parse.urlparse(base)
    assert host.scheme == "https" and host.hostname.endswith((".api.konghq.com", ".api.konghq.tech"))
    token = os.environ["KONGCTL_E2E_KONNECT_PAT"]
    needles = (token.encode(), base64.b64encode(token.encode()))
    headers = {"Authorization": "Bearer " + token, "Content-Type": "application/json"}
    opener = urllib.request.build_opener(NoRedirect())
    config = ["--config-file", str(Path(os.environ["XDG_CONFIG_HOME"]) / "kongctl/config.yaml")]
    namespace, principal_namespace = "identity-trust-declarative-e2e", "identity-trust-subjects-e2e"
    root = "/v2/directories/" + directory
    phase, deleted = "setup", False
    known_issuers = set()

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

    def collection(suffix="trusted-idps"):
        value = request("GET", root + "/" + suffix)
        assert isinstance(value.get("data"), list) and value["meta"]["page"]["next"] in (None, "")
        if suffix == "trusted-idps":
            assert all(v["issuer_url"] in known_issuers for v in value["data"])
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
        (work / (label + ".stdout")).write_bytes(result.stdout)
        (work / (label + ".stderr")).write_bytes(result.stderr)
        if failure:
            assert result.returncode != 0 and failure.lower() in (result.stdout + result.stderr).decode().lower(), label
            return None
        assert result.returncode == 0, label
        return json.loads(result.stdout) if output == "json" else result.stdout.decode()

    def manifest(label, trusts=None, grouped=False, external=False, principals=None):
        parent = {"ref": "people", "name": name, "allow_all_control_planes": True, "vault_enabled": True,
            "kongctl": {"namespace": namespace}}
        if external:
            parent = {"ref": "people", "_external": {"id": directory}}
        identity = {"directories": [parent]}
        if trusts is not None:
            trusts = [dict(v) for v in trusts]
            if grouped:
                for value in trusts:
                    value["directory"] = "people"
                identity["trusted_idps"] = trusts
            else:
                parent["trusted_idps"] = trusts
        if principals is not None:
            parent["principals"] = principals
        path = work / (label + ".yaml")
        source_namespace = principal_namespace if principals is not None else namespace
        path.write_text(json.dumps({"_defaults": {"kongctl": {"namespace": source_namespace}}, "identity": identity}))
        return str(path)

    def plan(label, source, mode="apply", failure=None):
        return invoke(label, ["plan", "--mode", mode, "-f", source], failure=failure)

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

    def subject(issuer):
        return {"ref": "staff", "display_name": "Trust lifecycle subject", "type": "user", "active": False,
            "kongctl": {"namespace": principal_namespace}, "labels": {"trust-declarative-fixture": directory},
            "identities": [{"ref": "staff-oidc", "type": "oidc", "issuer": issuer,
                "kongctl": {"namespace": principal_namespace},
                "claim": {"name": "sub", "value": "trust-lifecycle-subject"}}]}

    try:
        current = request("GET", root)
        assert current["name"] == name and current["managed_by"] == {} and current["vault_enabled"] is True
        assert collection() == [] and collection("principals") == [] and collection("groups") == []
        source = request("GET", "/v1/auth-servers/" + server)
        assert source["id"] == server and source["name"] == server_name
        issuer, jwks = source["issuer"], source["jwks_uri"]
        known_issuers.update((issuer, issuer + "/partners", issuer + "/empty", issuer + "/replacement", issuer + "/late"))
        full = {"ref": "workforce", "issuer_url": issuer, "display_name": "Shared display",
            "jwks_uri": jwks, "jit_provisioning_enabled": False, "vault_access_enabled": True,
            "jit": {"mapping": {"email": "email", "groups": "groups", "display_name": "name",
                "metadata": {"team": "team", "department": "department"}}}}
        other = {"ref": "partners", "issuer_url": issuer + "/partners", "display_name": "Shared display",
            "jwks_uri": jwks, "jit_provisioning_enabled": True, "vault_access_enabled": False}
        empty = {"ref": "empty", "issuer_url": issuer + "/empty", "jwks_uri": jwks,
            "jit_provisioning_enabled": False, "vault_access_enabled": False, "jit": {}}
        assert invoke("explain", ["explain", "identity.trusted_idps"])["type"] == "object"
        scaffold = invoke("scaffold", ["scaffold", "identity.trusted_idps"], output="text")
        assert "trusted_idps:" in scaffold and "issuer_url:" in scaffold
        initial = manifest("initial", [full, other, empty])
        plan("unowned-blocked", initial, failure="explicitly adopt")
        plan("external-blocked", manifest("external", [full], external=True), failure="external parents are read-only")
        invoke("adopt-default", ["adopt", "identity", "directory", name, "--namespace", namespace])
        creation = plan("create-plan", initial)
        assert creation["summary"]["by_action"] == {"CREATE": 3}
        saved = save("create-plan", creation)
        assert invoke("apply-saved-create", ["apply", "--plan", saved, "--auto-approve"])["summary"]["failed"] == 0
        values = {v["issuer_url"]: v for v in collection()}
        assert len(values) == 3
        for configured in (full, other, empty):
            observed = values[configured["issuer_url"]]
            for field, value in configured.items():
                if field != "ref" and not (field == "jit" and value == {}):
                    assert observed[field] == value, field
            assert "managed_by" not in observed and "labels" not in observed
        assert plan("repeat-noop", initial)["summary"]["total_changes"] == 0
        partial = {"ref": "workforce", "issuer_url": issuer, "display_name": "Updated display"}
        changed = manifest("partial", [partial, other, empty], grouped=True)
        update = plan("update-plan", changed)
        assert update["summary"]["by_action"] == {"UPDATE": 1}
        assert "display_name" in invoke("diff", ["diff", "-f", changed], output="text")
        saved = save("update-plan", update)
        before = collection()
        for reason in ("namespace", "protected"):
            try:
                management("foreign" if reason == "namespace" else namespace, reason == "protected")
                invoke("saved-" + reason, ["apply", "--plan", saved, "--auto-approve"], failure=reason)
                assert collection() == before
            finally:
                management()
        path = root + "/trusted-idps/" + values[issuer]["id"]
        hydrated = {key: value for key, value in values[issuer].items() if key in full and key != "ref"}
        hydrated["jwks_uri"] = None
        hydrated["jit"]["mapping"]["email"] = "preferred_email"
        request("PUT", path, hydrated)
        invoke("apply-saved-update", ["apply", "--plan", saved, "--auto-approve"])
        observed = request("GET", path)
        assert observed["jwks_uri"] is None and observed["jit"]["mapping"]["email"] == "preferred_email"
        assert observed["jit_provisioning_enabled"] is False and observed["vault_access_enabled"] is True
        cleared = {**partial, "display_name": None, "jwks_uri": None, "jit": {},
            "jit_provisioning_enabled": True, "vault_access_enabled": False}
        changed = manifest("cleared", [cleared, other, empty])
        apply("clear-public-fields", changed)
        observed = request("GET", path)
        assert observed["display_name"] is None and observed["jwks_uri"] is None and not observed.get("jit")
        assert plan("clear-noop", changed)["summary"]["total_changes"] == 0
        dump = work / "trust-dump.yaml"
        invoke("dump", ["dump", "declarative", "--resources", "identity.directories", "--include-child-resources",
            "--output-file", str(dump)], output="yaml")
        assert "trusted_idps:" in dump.read_text() and "jwks_uri: null" in dump.read_text()
        assert plan("dump-noop", str(dump))["summary"]["total_changes"] == 0
        apply("apply-dump", str(dump))
        assert plan("omitted-preserves", manifest("omitted"), "sync")["summary"]["total_changes"] == 0
        replacement = {**empty, "ref": "workforce", "issuer_url": issuer + "/replacement"}
        changed = manifest("replacement", [replacement, other, empty])
        transition = plan("issuer-replacement", changed, "sync")["changes"]
        created = next(v for v in transition if v["action"] == "CREATE")
        removed = next(v for v in transition if v["action"] == "DELETE")
        assert len(transition) == 2 and created["id"] in removed["depends_on"]
        apply("sync-replacement", changed, "sync")
        values = {v["issuer_url"]: v for v in collection()}
        assert issuer not in values and replacement["issuer_url"] in values
        prune = plan("empty-sync-plan", manifest("empty-collection", []), "sync")
        associations = manifest("independent-subject", external=True, principals=[subject(replacement["issuer_url"])])
        apply("create-inactive-subject", associations)
        plan("associated-plan-blocked", manifest("blocked-empty", []), "sync", failure="associated principals")
        saved = save("prune-plan", prune)
        invoke("late-association-blocks-saved", ["sync", "--plan", saved, "--auto-approve"], failure="associated principals")
        assert request("GET", root + "/trusted-idps/" + values[replacement["issuer_url"]]["id"])["id"]
        apply("restore-other-trusts", changed)
        detached = manifest("retarget", [other, empty], principals=[subject(other["issuer_url"])])
        detachment = plan("retarget-sync-plan", detached, "sync")
        trust_delete = next(v for v in detachment["changes"] if v["resource_type"] == "identity_trusted_idp")
        identity_update = next(v for v in detachment["changes"] if v["resource_type"] == "identity_principal_identity")
        assert identity_update["action"] == "UPDATE" and identity_update["id"] in trust_delete["depends_on"]
        assert identity_update["namespace"] == principal_namespace and trust_delete["namespace"] == namespace
        apply("retarget-and-retire", detached, "sync")
        person = collection("principals")[0]
        assert person["active"] is False and person["managed_by"]["kongctl-namespace"] == principal_namespace
        identity_values = collection("principals/" + person["id"] + "/identities")
        assert len(identity_values) == 1 and identity_values[0]["issuer"] == other["issuer_url"]
        remaining_subject = subject(other["issuer_url"])
        remaining_subject["identities"] = []
        detached = manifest("delete-identity", [empty], principals=[remaining_subject])
        detachment = plan("delete-identity-sync-plan", detached, "sync")
        trust_delete = next(v for v in detachment["changes"] if v["resource_type"] == "identity_trusted_idp")
        identity_delete = next(v for v in detachment["changes"] if v["resource_type"] == "identity_principal_identity")
        assert identity_delete["action"] == "DELETE" and identity_delete["id"] in trust_delete["depends_on"]
        apply("delete-identity-and-retire", detached, "sync")
        assert len(collection("principals")) == 1 and collection("principals/" + person["id"] + "/identities") == []
        apply("delete-independent-subject", associations, "delete")
        assert collection("principals") == []
        single = manifest("single", [empty])
        deletion = plan("delete-plan", single, "delete")
        child = next(v for v in deletion["changes"] if v["resource_type"] == "identity_trusted_idp")
        parent = next(v for v in deletion["changes"] if v["resource_type"] == "identity_directory")
        assert child["id"] in parent["depends_on"]
        saved = save("delete-plan", deletion)
        late = request("POST", root + "/trusted-idps", {"issuer_url": issuer + "/late", "jwks_uri": jwks}, 201)
        invoke("late-trust-blocks-parent", ["delete", "--plan", saved, "--auto-approve"], failure="attached Trusted IdPs")
        assert request("GET", root)["id"] == directory and len(collection()) == 1
        request("DELETE", root + "/trusted-idps/" + late["id"], expected=204)
        invoke("delete-saved", ["delete", "--plan", saved, "--auto-approve"])
        deleted = True
        request("GET", root, expected=404)
        invoke("delete-repeat", ["delete", "--plan", saved, "--auto-approve"])
        # Exercise the common missing-default bootstrap with its actual name.
        name = "default"
        single = manifest("default-bootstrap", [empty])
        assert plan("bootstrap-plan", single)["summary"]["by_action"] == {"CREATE": 2}
        apply("bootstrap-default-and-trust", single)
        fresh = invoke("bootstrap-directory", ["get", "identity", "directory", name])
        directory, root, deleted = fresh["id"], "/v2/directories/" + fresh["id"], False
        assert len(collection()) == 1 and plan("bootstrap-noop", single)["summary"]["total_changes"] == 0
        apply("direct-delete", single, "delete")
        deleted = True
        request("GET", root, expected=404)
        for path in work.rglob("*"):
            if path.is_file():
                assert not any(needle in path.read_bytes() for needle in needles)
        return {"trust_records": 3, "all_public_fields": True, "parent_adoption_and_protection": True,
            "fresh_put_hydration_and_clears": True, "issuer_replacement_ordered": True,
            "independent_detachments_ordered": True, "saved_association_and_parent_guards": True,
            "scoped_sync_and_delete": True, "dump_reload": True, "bootstrap_default": True, "fixtures_removed": True}
    except Exception:
        raise RuntimeError("Trusted IdP phase failed: " + phase) from None
    finally:
        if not deleted:
            current = request("GET", root)
            assert current["id"] == directory and current["name"] == name and current["vault_enabled"] is True
            for person in collection("principals"):
                assert person["managed_by"].get("tool") == "kongctl"
                assert person["managed_by"].get("kongctl-namespace") == principal_namespace
                assert person["managed_by"].get("kongctl-ref") == "staff"
                path = root + "/principals/" + person["id"]
                for identity in collection("principals/" + person["id"] + "/identities"):
                    assert identity["managed_by"].get("kongctl-ref") == "staff-oidc"
                    assert identity["managed_by"].get("kongctl-namespace") == principal_namespace
                    request("DELETE", path + "/identities/" + identity["id"], expected=204)
                assert collection("principals/" + person["id"] + "/identities") == []
                request("DELETE", path, expected=204)
            for value in collection():
                assert collection("trusted-idps/" + value["id"] + "/principals") == []
                request("DELETE", root + "/trusted-idps/" + value["id"], expected=204)
            assert collection() == [] and collection("principals") == [] and collection("groups") == []
            assert collection("vault/keys") == [] and collection("vault/providers") == []
            request("DELETE", root + "?force=false", expected=204)


if __name__ == "__main__":
    try:
        print(json.dumps(main()))
    except Exception as error:
        print(str(error) if isinstance(error, RuntimeError) else "Trusted IdP scenario or guarded cleanup failed", file=sys.stderr)
        sys.exit(1)
