"""Inspect native trust configuration and declaratively managed associations."""

import base64
import json
import os
from pathlib import Path
import subprocess
import sys
import urllib.parse
import urllib.request


def main():
    binary, work_dir, directory, directory_name, server, server_name = sys.argv[1:7]
    work = Path(work_dir).resolve()
    target = os.environ.get("KONGCTL_E2E_KONNECT_ENV", "com").strip() or "com"
    assert target in ("com", "tech")
    base = os.environ.get("KONGCTL_E2E_KONNECT_BASE_URL", "").strip() or "https://us.api.konghq." + target
    parsed = urllib.parse.urlparse(base)
    assert parsed.scheme == "https" and parsed.hostname.endswith((".api.konghq.com", ".api.konghq.tech"))
    token = os.environ["KONGCTL_E2E_KONNECT_PAT"]
    needles = (token.encode(), base64.b64encode(token.encode()))
    headers = {"Authorization": "Bearer " + token, "Content-Type": "application/json"}
    config = []
    if os.environ.get("KONGCTL_PROFILE") == "e2e":
        config = ["--config-file", str(Path(os.environ["XDG_CONFIG_HOME"]) / "kongctl/config.yaml")]
    root = "/v2/directories/" + directory
    namespace = "identity-trusted-idp-e2e"
    manifest = work / "principals.yaml"
    issuers = set()
    original_directory = None

    def request(method, path, body=None):
        req = urllib.request.Request(base.rstrip("/") + path, method=method, headers=headers,
            data=None if body is None else json.dumps(body).encode())
        with urllib.request.urlopen(req, timeout=30) as response:
            data = response.read()
            return json.loads(data) if data else None

    def collection(path):
        result, cursors = [], set()
        while True:
            value = request("GET", path)
            assert isinstance(value.get("data"), list)
            result.extend(value["data"])
            next_page = value["meta"]["page"]["next"]
            if next_page in (None, ""):
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

    def invoke(label, args, output="json", failure=None):
        log = work / (label + ".log")
        formatting = [] if args[0] == "plan" else ["-o", output]
        result = subprocess.run([binary, *args, *formatting, "--log-level", "trace", "--log-file", str(log), *config],
            capture_output=True, timeout=90, check=False)
        clean(result.stdout)
        clean(result.stderr)
        if log.exists():
            clean(log.read_bytes())
        (work / (label + ".stdout")).write_bytes(result.stdout)
        (work / (label + ".stderr")).write_bytes(result.stderr)
        if failure:
            assert result.returncode != 0 and failure.encode() in result.stderr and result.stdout == b""
            return None
        assert result.returncode == 0, label
        return json.loads(result.stdout) if output == "json" else result.stdout.decode()

    def get(label, noun, args, **options):
        path = [noun] if isinstance(noun, str) else noun
        return invoke(label, ["get", "identity", *path, *args], **options)

    def traversed(label, suffix):
        lines = (work / (label + ".log")).read_text().splitlines()
        requests = [line for line in lines if "log_type=http_request " in line and suffix + " " in line]
        return (len(requests) >= 2 and all("page[size]:1" in line for line in requests)
                and any("page[after]:" in line for line in requests))

    def owned_principal(value):
        metadata = value["managed_by"]
        ref = metadata.get("kongctl-ref")
        return (ref in ("staff-one", "staff-two") and metadata.get("tool") == "kongctl"
                and metadata.get("kongctl-namespace") == namespace and metadata.get("kongctl-protected") == "false"
                and bool(metadata.get("version")) and len(metadata) == 5
                and value["labels"] == {"identity-trusted-idp-e2e": directory})

    try:
        original_directory = request("GET", root)
        assert original_directory["name"] == directory_name and original_directory["vault_enabled"] is True
        assert collection(root + "/trusted-idps") == []
        assert collection(root + "/principals") == [] and collection(root + "/groups") == []
        source = request("GET", "/v1/auth-servers/" + server)
        assert source["id"] == server and source["name"] == server_name
        issuer, jwks = source["issuer"], source["jwks_uri"]
        assert isinstance(issuer, str) and isinstance(jwks, str)
        issuers.update((issuer, issuer + "/second", issuer + "/empty"))
        mapping = {"metadata": {"team": "team", "department": "department"},
                   "email": "email", "groups": "groups", "display_name": "name"}
        first = request("POST", root + "/trusted-idps", {"issuer_url": issuer,
            "display_name": "Shared display name", "jit_provisioning_enabled": False,
            "vault_access_enabled": False, "jit": {"mapping": mapping}})
        second = request("POST", root + "/trusted-idps", {"issuer_url": issuer + "/second", "jwks_uri": jwks,
            "display_name": "Shared display name", "jit_provisioning_enabled": True, "vault_access_enabled": True})
        empty = request("POST", root + "/trusted-idps", {"issuer_url": issuer + "/empty", "jwks_uri": jwks,
            "jit_provisioning_enabled": False, "vault_access_enabled": False})
        before = {value["id"]: value for value in collection(root + "/trusted-idps")}
        assert len(before) == 3
        manifest.write_text(json.dumps({"_defaults": {"kongctl": {"namespace": namespace}}, "identity": {
            "directories": [{"ref": "people", "_external": {"id": directory}}],
            "principals": [{"ref": ref, "directory": "people", "display_name": ref, "type": "user", "active": True,
                "labels": {"identity-trusted-idp-e2e": directory}, "identities": [{"ref": ref + "-login",
                    "type": "oidc", "issuer": issuer, "claim": {"name": "sub", "value": ref}}]}
                for ref in ("staff-one", "staff-two")]}}))
        created = invoke("apply-principals", ["apply", "-f", str(manifest), "--auto-approve"])
        assert created["summary"]["applied"] == 4 and created["summary"]["failed"] == 0
        plan = invoke("replan-principals", ["plan", "--mode", "apply", "-f", str(manifest)])
        assert plan["summary"]["total_changes"] == 0
        scope = ["--directory-id", directory]
        namescope = ["--directory-name", directory_name]
        values = get("idps-page-one", "trusted-idps", [*scope, "--page-size", "1"])
        assert {v["id"] for v in values} == set(before)
        detail = get("idp-detail", "trusted-idp", [first["id"], *scope])
        assert detail["issuer_url"] == issuer and detail["display_name"] == "Shared display name"
        assert detail["jit"] == {"mapping": mapping} and detail["jwks_uri"] is None
        assert detail["jit_provisioning_enabled"] is False and detail["vault_access_enabled"] is False
        selected = get("idp-issuer", "trusted-idp", [*namescope, "--issuer-url", issuer + "/second"])
        assert selected["id"] == second["id"] and selected["jwks_uri"] == jwks
        assert selected["jit_provisioning_enabled"] is True and selected["vault_access_enabled"] is True
        explicit = invoke("list-explicit-product", ["list", "konnect", "identity", "trusted-idps", *namescope])
        assert {v["id"] for v in explicit} == set(before)
        principal_args = [*scope, "--trusted-idp-id", first["id"]]
        principals = get("principals-page-one", ["trusted-idp", "principals"], [*principal_args, "--page-size", "1"])
        assert len(principals) == 2 and all(owned_principal(value) for value in principals)
        selected = get("principals-issuer", ["trusted-idps", "principal"], [*namescope, "--issuer-url", issuer])
        assert {v["id"] for v in selected} == {v["id"] for v in principals}
        flat = get("principals-flat", "trusted-idp-principals", principal_args)
        assert {v["id"] for v in flat} == {v["id"] for v in principals}
        listed = invoke("principals-list", ["list", "identity", "trusted-idp", "principals", *principal_args])
        assert {v["id"] for v in listed} == {v["id"] for v in principals}
        assert get("empty-principals", ["trusted-idp", "principals"], [*scope, "--trusted-idp-id", empty["id"]]) == []
        for label, noun, args in (
            ("idp-text", "trusted-idp", [first["id"], *scope]),
            ("idp-list-text", "trusted-idps", namescope),
            ("principals-text", ["trusted-idp", "principals"], principal_args),
        ):
            output = get(label, noun, args, output="text")
            assert "ID" in output and "{" not in output
            assert (issuer if "principals" not in noun else "staff-one") in output
        for label, noun, args in (
            ("idp-yaml", "trusted-idp", [first["id"], *scope]),
            ("principals-yaml", ["trusted-idp", "principals"], principal_args),
        ):
            output = get(label, noun, args, output="yaml")
            assert ("issuer_url:" if "principals" not in noun else "managed_by:") in output
        get("unknown-directory", "trusted-idps", ["--directory-id", "731559d3-39bd-48c7-b33c-eb192847eb20"], failure="HTTP 404")
        get("unknown-idp", "trusted-idp", ["731559d3-39bd-48c7-b33c-eb192847eb20", *scope], failure="HTTP 404")
        assert traversed("idps-page-one", "/trusted-idps")
        assert traversed("principals-page-one", "/principals")
        after = {value["id"]: value for value in collection(root + "/trusted-idps")}
        assert after == before
        for path in work.parents[2].rglob("*"):
            if path.is_file():
                clean(path.read_bytes())
        result = {"trusted_idps": 3, "associated_principals": 2, "output_formats": 3,
            "exact_issuer_selection": True, "real_idp_pagination": True, "real_principal_pagination": True,
            "external_directory_declarative_principals": True, "inspection_preserves_configuration": True}
    finally:
        if original_directory is not None:
            fresh = request("GET", root)
            assert fresh["id"] == directory and fresh["name"] == directory_name and fresh["vault_enabled"] is True
            remaining = collection(root + "/trusted-idps")
            assert all(value["issuer_url"] in issuers for value in remaining)
            observed = collection(root + "/principals")
            assert all(owned_principal(value) for value in observed)
            if manifest.exists():
                invoke("delete-principals", ["delete", "-f", str(manifest), "--auto-approve"])
            assert collection(root + "/principals") == [] and collection(root + "/groups") == []
            for value in remaining:
                current = request("GET", root + "/trusted-idps/" + value["id"])
                assert current["id"] == value["id"] and current["issuer_url"] == value["issuer_url"]
                assert collection(root + "/trusted-idps/" + value["id"] + "/principals") == []
                request("DELETE", root + "/trusted-idps/" + value["id"])
            assert collection(root + "/trusted-idps") == []
    # Check disabled capability only after every IdP has been explicitly removed.
    replacement = {key: original_directory[key] for key in ("name", "description", "allowed_control_planes",
        "allow_all_control_planes", "ttl_secs", "negative_ttl_secs", "vault_enabled", "labels", "managed_by")}
    try:
        request("PUT", root, {**replacement, "vault_enabled": False})
        get("disabled-vault", "trusted-idps", ["--directory-id", directory], failure="HTTP 403")
    finally:
        restored = request("PUT", root, replacement)
        assert restored["vault_enabled"] is True
    result.update(vault_disabled_reported=True, fixtures_removed=True)
    return result


if __name__ == "__main__":
    try:
        print(json.dumps(main()))
    except Exception:
        print("Trusted IdP inspection or verified fixture cleanup failed", file=sys.stderr)
        sys.exit(1)
