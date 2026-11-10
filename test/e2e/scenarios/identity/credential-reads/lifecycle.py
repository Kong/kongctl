"""Use direct API fixtures to verify read-only credential commands."""

import base64
import json
import os
from pathlib import Path
import secrets
import subprocess
import sys
import tempfile
import urllib.error
import urllib.request


def main():
    binary, secret_root, artifact_root, directory = sys.argv[1:5]
    artifacts = Path(artifact_root)
    artifacts.mkdir(parents=True, exist_ok=True)
    temporary = tempfile.TemporaryDirectory(prefix="credential-reads-", dir=secret_root)
    private = Path(temporary.name)
    os.chmod(private, 0o700)
    environment = os.environ.get("KONGCTL_E2E_KONNECT_ENV", "com") or "com"
    base = os.environ.get("KONGCTL_E2E_KONNECT_BASE_URL") or "https://us.api.konghq." + environment
    assert base in ("https://us.api.konghq.com", "https://us.api.konghq.tech")
    token = os.environ["KONGCTL_E2E_KONNECT_PAT"]
    headers = {"Authorization": "Bearer " + token, "Content-Type": "application/json"}
    prefix = "read-" + secrets.token_hex(5)
    root, cleanup, needles = "/v2/directories/" + directory, [], [token]
    serial, phase, cleanup_failed = 0, "setup", False
    before = None

    class NoRedirect(urllib.request.HTTPRedirectHandler):
        def redirect_request(self, req, fp, code, msg, hdrs, newurl):
            return None

    opener = urllib.request.build_opener(NoRedirect())

    def remember(value):
        if isinstance(value, dict):
            for key, item in value.items():
                if key in ("secret", "client_secret", "enrollment_url") and isinstance(item, str):
                    needles.extend((item, base64.b64encode(item.encode()).decode()))
                else:
                    remember(item)
        elif isinstance(value, list):
            for item in value:
                remember(item)

    def request(method, path, body=None, expected=(200,)):
        nonlocal serial
        serial += 1
        remember(body)
        req = urllib.request.Request(base + path, method=method, headers=headers,
                                     data=None if body is None else json.dumps(body).encode())
        try:
            with opener.open(req, timeout=30) as response:
                status, raw = response.status, response.read()
        except urllib.error.HTTPError as error:
            status, raw = error.code, error.read()
        value = json.loads(raw) if raw else None
        remember(value)
        with os.fdopen(os.open(private / (str(serial) + ".json"),
                               os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600), "w") as stream:
            json.dump({"method": method, "path": path, "status": status, "response": value}, stream)
        assert status in expected, "direct fixture request failed"
        return value

    def create(path, body):
        value = request("POST", path, body, (201,))
        target = path + "/" + value["id"]
        cleanup.append(target)
        return target, value

    def read(label, args, output="json", verb="get"):
        nonlocal phase
        phase = label
        result = subprocess.run([binary, verb, "identity", *args, "-o", output,
                                 "--log-level", "error", "--konnect-env", environment,
                                 "--base-url", base], capture_output=True, text=True, timeout=45)
        public = result.stdout + result.stderr
        assert not any(needle and needle in public for needle in needles), "credential leaked in CLI output"
        assert "enrollment_url" not in public and '"secret"' not in public and '"client_secret"' not in public
        (artifacts / (label + ".stdout")).write_text(result.stdout)
        (artifacts / (label + ".stderr")).write_text(result.stderr)
        assert result.returncode == 0, "credential read failed"
        return json.loads(result.stdout) if output == "json" else result.stdout

    try:
        before = request("GET", root)
        assert before["vault_enabled"], "fixture requires an enabled Vault"
        principal, _ = create(root + "/principals", {"display_name": prefix, "type": "agent"})
        group, _ = create(root + "/groups", {"name": prefix})
        principal_id, group_id = principal.rsplit("/", 1)[1], group.rsplit("/", 1)[1]
        directory_flags = ["--directory-id", directory]
        for kind, parent, parent_id in (("principal", principal, principal_id), ("group", group, group_id)):
            issued = []
            for body in ({"type": "v1"}, {"type": "imported", "secret": secrets.token_urlsafe(32)}):
                remember(body)
                _, value = create(parent + "/api-keys", body)
                issued.append(value["id"])
            scope = [kind, "api-key", *directory_flags, "--" + kind + "-id", parent_id]
            items = read(kind + "-keys", [*scope, "--page-size", "1"])
            assert {item["id"] for item in items} == set(issued)
            assert read(kind + "-detail", [*scope, issued[0]])["id"] == issued[0]
            assert len(read(kind + "-list", scope, verb="list")) == 2
            for output in ("yaml", "text"):
                assert read(kind + "-keys-" + output, scope, output)
        basic, value = create(principal + "/basic-auths", {"username": prefix})
        password_ids = []
        for _ in range(2):
            _, password = create(basic + "/passwords", {"secret": secrets.token_urlsafe(32)})
            password_ids.append(password["id"])
        basic_scope = ["principal", "basic-auth", *directory_flags, "--principal-id", principal_id]
        assert value["id"] in {item["id"] for item in read("basic-auths", basic_scope)}
        assert read("basic-auth-detail", [*basic_scope, value["id"]])["username"] == prefix
        password_scope = ["principal", "basic-auth", "password", *directory_flags,
                          "--principal-id", principal_id, "--basic-auth-id", value["id"]]
        assert {item["id"] for item in read("passwords", [*password_scope, "--page-size", "1"])} == set(password_ids)
        assert read("password-detail", [*password_scope, password_ids[0]])["id"] == password_ids[0]

        server, _ = create("/v1/auth-servers", {"name": prefix, "audience": "https://fixture.example.test"})
        client, value = create(server + "/clients", {"name": prefix, "grant_types": ["client_credentials"],
                                "response_types": [], "token_endpoint_auth_method": "client_secret_basic"})
        initial = request("GET", client + "/secrets")["data"]
        cleanup.extend(client + "/secrets/" + item["id"] for item in initial)
        _, secret = create(client + "/secrets", {"secret": secrets.token_urlsafe(32), "enabled": False})
        secret_scope = ["auth-server", "client", "secret", "--auth-server-id", server.rsplit("/", 1)[1],
                        "--client-id", value["id"]]
        items = read("client-secrets", [*secret_scope, "--page-size", "1"])
        assert secret["id"] in {item["id"] for item in items}
        assert read("client-secret-detail", [*secret_scope, secret["id"]])["enabled"] is False

        if not request("GET", root + "/vault/keys")["data"]:
            public = json.loads((Path(__file__).parent.parent / "vault-keys/public-keys.json").read_text())[0]
            create(root + "/vault/keys", {"name": prefix, "public_key": public["public_key"]})
        provider, value = create(root + "/vault/providers", {"name": prefix, "template_name": "github",
                                "credential_type": "shared", "client_id": prefix,
                                "client_secret": secrets.token_urlsafe(32)})
        _, credential = create(provider + "/credentials", {})
        vault_scope = ["vault", "provider", "credential", *directory_flags, "--provider-id", value["id"]]
        assert credential["id"] in {item["id"] for item in read("vault-credentials", vault_scope)}
        assert read("vault-credential-detail", [*vault_scope, credential["id"]])["id"] == credential["id"]
        for label, scope in (("password", password_scope), ("client-secret", secret_scope), ("vault", vault_scope)):
            for output in ("yaml", "text"):
                assert read(label + "-" + output, scope, output)
        assert request("GET", root) == before
    except Exception:
        raise RuntimeError("Credential metadata read coverage failed during " + phase) from None
    finally:
        try:
            for path in reversed(cleanup):
                try:
                    request("DELETE", path, expected=(200, 204, 404))
                    request("GET", path, expected=(404,))
                except Exception:
                    cleanup_failed = True
            if before is not None:
                try:
                    assert request("GET", root) == before
                except Exception:
                    cleanup_failed = True
        finally:
            temporary.cleanup()
        if cleanup_failed:
            raise RuntimeError("Credential read fixture cleanup failed")
    return {"principal_and_group_keys": True, "basic_auth_passwords": True, "oauth_secret_metadata": True,
            "shared_vault_metadata": True, "pagination_and_details": True, "public_output_formats": True,
            "secrets_and_capabilities_excluded": True, "fixtures_removed": True, "directory_preserved": True}


if __name__ == "__main__":
    try:
        print(json.dumps(main()))
    except RuntimeError as error:
        print(str(error), file=sys.stderr)
        sys.exit(1)
