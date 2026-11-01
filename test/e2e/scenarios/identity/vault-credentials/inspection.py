"""Observe public Vault credential metadata without initiating enrollment reads."""
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
    material = [secrets.token_urlsafe(40) for _ in range(4)]
    hidden = [token, *material]
    headers = {"Authorization": "Bearer " + token, "Content-Type": "application/json"}
    opener = urllib.request.build_opener(NoRedirect())
    config = ["--config-file", str(Path(os.environ["XDG_CONFIG_HOME"]) / "kongctl/config.yaml")]
    root = "/v2/directories/" + directory
    phase, principal, key = "setup", None, None
    known_providers, known_credentials = set(), set()

    def clean(data):
        for value in hidden:
            assert value.encode() not in data and base64.b64encode(value.encode()) not in data
        assert b"enrollment_url" not in data

    def request(method, path, body=None, expected=200):
        req = urllib.request.Request(base.rstrip("/") + path, method=method, headers=headers,
            data=None if body is None else json.dumps(body).encode())
        try:
            response = opener.open(req, timeout=30)
        except urllib.error.HTTPError as error:
            response = error
        with response:
            raw = response.read()
            assert response.status == expected
            return json.loads(raw) if raw else None

    def collection(suffix):
        value = request("GET", root + "/" + suffix)
        assert isinstance(value["data"], list) and value["meta"]["page"]["next"] in (None, "")
        return value["data"]

    def invoke(label, args, output="json", failure=None):
        nonlocal phase
        phase = label
        log = work / (label + ".log")
        result = subprocess.run([binary, *args, "-o", output, "--log-level", "trace", "--log-file", str(log), *config],
            capture_output=True, timeout=90, check=False)
        for data in (result.stdout, result.stderr, log.read_bytes() if log.exists() else b""):
            clean(data)
        (work / (label + ".stdout")).write_bytes(result.stdout)
        (work / (label + ".stderr")).write_bytes(result.stderr)
        if failure:
            assert result.returncode != 0 and failure.lower() in (result.stdout + result.stderr).decode().lower()
            assert result.stdout == b""
            return None
        assert result.returncode == 0
        return json.loads(result.stdout) if output == "json" else result.stdout.decode()

    def credential(provider, body):
        value = request("POST", root + "/vault/providers/" + provider["id"] + "/credentials", body, 201)
        known_credentials.add(value["id"])
        if value.get("enrollment_url"):
            url = value["enrollment_url"]
            hidden.append(url)
            hidden.extend(v for key, values in urllib.parse.parse_qs(urllib.parse.urlparse(url).query).items()
                          if key in ("token", "state") for v in values)
        return value

    def provider(name, template, **fields):
        value = request("POST", root + "/vault/providers", {"name": name, "template_name": template, **fields}, 201)
        known_providers.add(value["id"])
        return value

    try:
        current = request("GET", root)
        assert current["name"] == name and current["vault_enabled"] is True
        for suffix in ("principals", "groups", "vault/providers", "vault/keys", "trusted-idps"):
            assert collection(suffix) == []
        public_key = json.loads((Path(__file__).parent.parent / "vault-keys/public-keys.json").read_text())[0]["public_key"]
        key = request("POST", root + "/vault/keys", {"name": "credential-inspection", "public_key": public_key}, 201)
        static = provider("static-inspection", "static_secret", credential_type="shared")
        oauth = provider("oauth-inspection", "github", credential_type="shared", client_id="public-inspection-client",
                         client_secret=material[3], scopes=["repo"])
        empty = provider("user-inspection", "github", credential_type="user", client_id="public-inspection-client",
                         client_secret=material[3], scopes=["repo"])
        default = credential(static, {"secret": material[0]})
        for number in range(2):
            credential(static, {"secret": material[number + 1], "selector": str(number == 0).lower(), "priority": number})
        pending = credential(oauth, {})
        credential(oauth, {"selector": "true", "priority": 0})
        principal = request("POST", root + "/principals", {"display_name": "Credential inspection subject",
            "type": "user", "active": False}, 201)
        snapshots = {v["id"]: collection("vault/providers/" + v["id"] + "/credentials") for v in (static, oauth, empty)}
        assert len(snapshots[static["id"]]) == 3 and len(snapshots[oauth["id"]]) == 2
        assert all(v["status"] == "active" for v in snapshots[static["id"]])
        assert all(v["status"] == "pending" for v in snapshots[oauth["id"]])
        assert all("enrollment_url" not in v for values in snapshots.values() for v in values)
        scope = ["--directory-id", directory]
        namescope = ["--directory-name", name]
        static_scope = [*scope, "--provider-id", static["id"]]
        values = invoke("static-pages", ["get", "identity", "vault-provider-credentials", *static_scope, "--page-size", "1"])
        assert values == snapshots[static["id"]]
        selected = invoke("static-detail", ["get", "identity", "vault-provider-credential", default["id"], *static_scope])
        assert selected == next(v for v in values if v["id"] == default["id"])
        assert selected["selector"] is None and selected["priority"] is None
        values = invoke("oauth-pages", ["list", "konnect", "identity", "provider-credentials", *namescope,
            "--provider-name", oauth["name"], "--page-size", "1"])
        assert values == snapshots[oauth["id"]]
        selected = invoke("oauth-detail", ["get", "identity", "vault-provider-credential", pending["id"],
            *namescope, "--provider-name", oauth["name"]])
        assert selected["status"] == "pending" and "enrollment_url" not in selected
        for noun, parent in (("vault-provider-credentials", [*scope, "--provider-id", empty["id"]]),
                             ("principal-vault-credentials", [*scope, "--principal-id", principal["id"]])):
            assert invoke(noun + "-empty", ["get", "identity", noun, *parent]) == []
        for label, noun, parent in (
            ("provider", "vault-provider-credential", [pending["id"], *namescope, "--provider-name", oauth["name"]]),
            ("principal", "principal-vault-credentials", [*scope, "--principal-id", principal["id"]]),
        ):
            text = invoke(label + "-text", ["get", "identity", noun, *parent], output="text")
            assert "ID" in text and "{" not in text
            yaml = invoke(label + "-yaml", ["get", "identity", noun, *parent], output="yaml")
            assert ("pending" in yaml) if label == "provider" else yaml.strip() == "[]"
        invoke("wrong-provider", ["get", "identity", "vault-provider-credential", default["id"], *scope,
            "--provider-id", empty["id"]], failure="not found")
        invoke("missing-principal-credential", ["get", "identity", "principal-vault-credential", default["id"], *scope,
            "--principal-id", principal["id"]], failure="not found")
        invoke("missing-parent", ["get", "identity", "vault-provider-credentials", *scope,
            "--provider-id", "731559d3-39bd-48c7-b33c-eb192847eb20"], failure="HTTP 404")
        for provider_id, expected in snapshots.items():
            assert collection("vault/providers/" + provider_id + "/credentials") == expected
        for label, count in (("static-pages", 3), ("oauth-pages", 2)):
            requests = [v for v in (work / (label + ".log")).read_text().splitlines()
                        if "log_type=http_request " in v and "/credentials " in v]
            assert len(requests) == count and any("page[after]:" in v for v in requests)
        for path in work.rglob("*"):
            if path.is_file():
                data = path.read_bytes()
                clean(data)
                if path.suffix == ".log":
                    for line in data.decode().splitlines():
                        if "log_type=http_request " in line:
                            assert "/credentials/" not in line and "/vault-credentials/" not in line
        return {"static_credentials": 3, "pending_oauth_credentials": 2, "real_credential_pagination": True,
            "metadata_reads_do_not_mint_enrollment": True, "parent_scope_and_output_formats": True,
            "principal_empty_inspection": True, "inspection_preserves_records": True,
            "secrets_and_capabilities_excluded": True, "fixtures_removed": True}
    except Exception:
        raise RuntimeError("Vault credential inspection phase failed: " + phase) from None
    finally:
        current = request("GET", root)
        assert current["id"] == directory and current["name"] == name and current["vault_enabled"] is True
        if principal:
            path = root + "/principals/" + principal["id"]
            assert collection("principals/" + principal["id"] + "/vault-credentials") == []
            assert collection("principals/" + principal["id"] + "/identities") == []
            request("DELETE", path, expected=204)
        assert collection("principals") == [] and collection("groups") == []
        for provider in collection("vault/providers"):
            assert provider["id"] in known_providers
            path = root + "/vault/providers/" + provider["id"]
            for value in collection("vault/providers/" + provider["id"] + "/credentials"):
                assert value["id"] in known_credentials and value["credential_type"] == "shared"
                request("DELETE", path + "/credentials/" + value["id"], expected=204)
            assert collection("vault/providers/" + provider["id"] + "/credentials") == []
            request("DELETE", path, expected=204)
        assert collection("vault/providers") == []
        if key:
            request("DELETE", root + "/vault/keys/" + key["id"], expected=204)
        assert collection("vault/keys") == [] and collection("trusted-idps") == []


if __name__ == "__main__":
    try:
        print(json.dumps(main()))
    except Exception as error:
        print(str(error) if isinstance(error, RuntimeError) else "Vault credential inspection or guarded cleanup failed", file=sys.stderr)
        sys.exit(1)
