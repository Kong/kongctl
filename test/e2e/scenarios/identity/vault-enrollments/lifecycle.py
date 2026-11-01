"""Verify pending shared enrollment, without exposing or following capabilities."""

import base64
import json
import os
from pathlib import Path
import secrets
import shutil
import stat
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
    scratch = tempfile.TemporaryDirectory(prefix="kongctl-vault-enrollment-", dir=os.environ.get("TMPDIR"))
    work = Path(scratch.name)
    private = work / "private"
    private.mkdir(mode=0o700)
    target = os.environ.get("KONGCTL_E2E_KONNECT_ENV", "com").strip() or "com"
    assert target in ("com", "tech")
    base = os.environ.get("KONGCTL_E2E_KONNECT_BASE_URL", "").strip() or "https://us.api.konghq." + target
    host = urllib.parse.urlsplit(base)
    assert host.scheme == "https" and host.hostname.endswith((".api.konghq.com", ".api.konghq.tech"))
    token = os.environ["KONGCTL_E2E_KONNECT_PAT"]
    hidden = [token, secrets.token_urlsafe(32), secrets.token_urlsafe(32)]
    opener = urllib.request.build_opener(NoRedirect())
    headers = {"Authorization": "Bearer " + token, "Content-Type": "application/json"}
    config = ["--config-file", str(Path(os.environ["XDG_CONFIG_HOME"]) / "kongctl/config.yaml")]
    root = "/v2/directories/" + directory
    phase = "setup"
    providers, key = [], None

    def check_public(data):
        for value in hidden:
            assert value.encode() not in data and base64.b64encode(value.encode()) not in data

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
            return json.loads(data) if data else None

    def collection(suffix):
        value = request("GET", root + "/" + suffix)
        assert isinstance(value["data"], list) and value["meta"]["page"]["next"] in (None, "")
        return value["data"]

    def credentials(provider):
        return collection("vault/providers/" + provider + "/credentials")

    def invoke(label, args, output="json", failure=None):
        nonlocal phase
        phase = label
        log = work / (label + ".log")
        result = subprocess.run([binary, *args, "-o", output, "--log-level", "trace", "--log-file", str(log), *config],
            cwd=work, stdin=subprocess.DEVNULL, capture_output=True, timeout=120, check=False)
        for data in (result.stdout, result.stderr, log.read_bytes() if log.exists() else b""):
            check_public(data)
        (work / (label + ".stdout")).write_bytes(result.stdout)
        (work / (label + ".stderr")).write_bytes(result.stderr)
        if failure:
            assert result.returncode != 0 and failure.lower() in (result.stdout + result.stderr).decode().lower(), label
            return None
        assert result.returncode == 0, label
        return json.loads(result.stdout) if output == "json" else result.stdout.decode()

    def scope(provider, by_name=False):
        return ["--directory-name", name, "--provider-name", provider["name"]] if by_name else [
            "--directory-id", directory, "--provider-id", provider["id"]]

    def capture(path, credential):
        assert stat.S_IMODE(path.stat().st_mode) == 0o600
        value = json.loads(path.read_text())
        assert value["status"] == "captured" and value["directory_id"] == directory
        assert value["credential_id"] == credential and "secret" not in value
        url = value["enrollment_url"]
        parsed = urllib.parse.urlsplit(url)
        assert parsed.scheme == "https" and parsed.hostname and not parsed.username and not parsed.fragment
        hidden.append(url)
        for key, values in urllib.parse.parse_qs(parsed.query).items():
            if key in ("token", "state"):
                hidden.extend(values)
        return value

    def create_provider(label, template, mode):
        body = {"name": "enrollment-" + label, "template_name": template, "credential_type": mode}
        if template != "static_secret":
            body.update(client_id="public-enrollment-client", client_secret=hidden[1], scopes=["repo"])
        value = request("POST", root + "/vault/providers", body, 201)
        providers.append(value)
        return value

    try:
        before_directory = request("GET", root)
        assert before_directory["name"] == name and before_directory["managed_by"] == {}
        for suffix in ("principals", "groups", "trusted-idps", "vault/keys", "vault/providers"):
            assert collection(suffix) == []
        shared = create_provider("shared", "github", "shared")
        other_shared = create_provider("other-shared", "github", "shared")
        user = create_provider("user", "github", "user")
        static_provider = create_provider("static", "static_secret", "shared")
        first_path = private / "created.json"
        first = invoke("create-default", ["create", "identity", "vault-provider-credential", *scope(shared),
            "--secret-output-file", str(first_path)])
        assert first["credential_type"] == "shared" and first["status"] == "pending"
        assert first["priority"] is None and first["selector"] is None
        assert "enrollment_url" not in first and "managed_by" not in first and "labels" not in first
        assert capture(first_path, first["id"])["provider_id"] == shared["id"]
        original_file = first_path.read_bytes()
        second_path = private / "rule.json"
        second = invoke("create-rule-text", ["create", "identity", "vault-provider-credential", *scope(shared, True),
            "--selector", "true", "--priority", "0", "--secret-output-file", str(second_path)], output="text")
        assert "pending" in second and "{" not in second
        values = credentials(shared["id"])
        assert len(values) == 2
        rule = next(c for c in values if c["priority"] == 0)
        capture(second_path, rule["id"])
        before = credentials(shared["id"])
        for label, selected, extra, failure in (
            ("static-create-blocked", static_provider, [], "declarations"),
            ("user-create-blocked", user, [], "shared"),
            ("duplicate-default-blocked", shared, [], "occupied"),
            ("duplicate-priority-blocked", shared, ["--selector", "false", "--priority", "0"], "occupied"),
        ):
            path = private / (label + ".json")
            invoke(label, ["create", "identity", "vault-provider-credential", *scope(selected), *extra,
                "--secret-output-file", str(path)], failure=failure)
            assert not path.exists()
        invoke("existing-capture-blocked", ["create", "identity", "vault-provider-enrollment", first["id"],
            *scope(shared), "--secret-output-file", str(first_path)], failure="reserve secret output")
        assert first_path.read_bytes() == original_file
        invoke("wrong-provider-issuance-blocked", ["create", "identity", "vault-provider-enrollment", first["id"],
            *scope(other_shared), "--secret-output-file", str(private / "wrong.json")], failure="absent")
        assert credentials(shared["id"]) == before
        renewed_path = private / "renewed.json"
        renewed = invoke("explicit-reissue-yaml", ["create", "identity", "vault-provider-enrollment", first["id"],
            *scope(shared, True), "--secret-output-file", str(renewed_path)], output="yaml")
        assert "pending" in renewed and "enrollment_url:" not in renewed
        capture(renewed_path, first["id"])
        assert first_path.read_bytes() == original_file and credentials(shared["id"]) == before
        one = invoke("public-get", ["get", "identity", "vault-provider-credential", first["id"], *scope(shared)])
        assert one["status"] == "pending" and "enrollment_url" not in one
        listed = invoke("public-list", ["list", "identity", "vault-provider-credentials", *scope(shared), "--page-size", "1"])
        assert {c["id"] for c in listed} == {first["id"], rule["id"]}
        invoke("wrong-provider-delete-blocked", ["delete", "identity", "vault-provider-credential", first["id"],
            *scope(other_shared), "--auto-approve"], failure="absent")
        invoke("delete-default", ["delete", "identity", "vault-provider-credential", first["id"],
            *scope(shared, True), "--auto-approve"])
        invoke("deleted-issuance-blocked", ["create", "identity", "vault-provider-enrollment", first["id"],
            *scope(shared), "--secret-output-file", str(private / "deleted.json")], failure="absent")
        invoke("delete-rule", ["delete", "identity", "vault-provider-credential", rule["id"],
            *scope(shared), "--auto-approve"], output="text")
        assert credentials(shared["id"]) == []
        public = json.loads((Path(__file__).parent.parent / "vault-keys/public-keys.json").read_text())[0]["public_key"]
        key = request("POST", root + "/vault/keys", {"name": "enrollment-key", "public_key": public}, 201)
        static = request("POST", root + "/vault/providers/" + static_provider["id"] + "/credentials",
            {"secret": hidden[2]}, 201)
        invoke("delete-static", ["delete", "identity", "vault-provider-credential", static["id"],
            *scope(static_provider), "--auto-approve"])
        assert credentials(static_provider["id"]) == []
        assert request("GET", root)["managed_by"] == before_directory["managed_by"]
    except Exception:
        raise RuntimeError("Shared enrollment lifecycle failed during " + phase + "; private responses withheld") from None
    finally:
        try:
            for provider in providers:
                path = root + "/vault/providers/" + provider["id"]
                for credential in credentials(provider["id"]):
                    assert credential["credential_type"] == "shared"
                    request("DELETE", path + "/credentials/" + credential["id"], expected=204)
                request("DELETE", path, expected=204)
            if key:
                request("DELETE", root + "/vault/keys/" + key["id"], expected=204)
            assert collection("vault/providers") == [] and collection("vault/keys") == []
            shutil.rmtree(private)
            for path in work.rglob("*"):
                if path.is_file():
                    check_public(path.read_bytes())
            for path in work.iterdir():
                assert path.is_file()
                shutil.copyfile(path, artifacts / path.name)
        finally:
            scratch.cleanup()
    assert not work.exists()
    print(json.dumps({"private_enrollment_capture": True, "explicit_url_reissuance": True,
        "pending_without_consent_claim": True, "default_and_zero_priority": True,
        "metadata_reads_exclude_capabilities": True, "scope_type_and_file_guards": True,
        "explicit_shared_retirement": True, "management_metadata_unchanged": True,
        "private_workspace_removed": True, "fixtures_removed": True}))


if __name__ == "__main__":
    main()
