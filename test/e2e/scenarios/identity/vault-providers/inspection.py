"""Inspect public provider configuration without minting enrollment capabilities."""

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


def main():
    binary, location, directory, name = sys.argv[1:5]
    work = Path(location)
    target = os.environ.get("KONGCTL_E2E_KONNECT_ENV", "com").strip() or "com"
    assert target in ("com", "tech")
    base = os.environ.get("KONGCTL_E2E_KONNECT_BASE_URL", "").strip() or "https://us.api.konghq." + target
    host = urllib.parse.urlparse(base)
    assert host.scheme == "https" and host.hostname.endswith((".api.konghq.com", ".api.konghq.tech"))
    pat = os.environ["KONGCTL_E2E_KONNECT_PAT"]
    client_secret = secrets.token_urlsafe(32)
    needles = [value for text in (pat, client_secret) for value in (text.encode(), base64.b64encode(text.encode()))]
    headers = {"Authorization": "Bearer " + pat, "Content-Type": "application/json"}
    config = ["--config-file", str(Path(os.environ["XDG_CONFIG_HOME"]) / "kongctl/config.yaml")]
    root = "/v2/directories/" + directory
    known = {}
    phase = "setup"
    provider_fields = ("id", "directory_id", "name", "template_name", "bearer_methods_supported", "enabled", "scopes",
        "token_endpoint_effective", "authorization_endpoint_effective", "registration_endpoint_effective",
        "dcr_enabled", "credential_type", "pkce_enabled", "refresh_buffer_seconds", "created_at", "updated_at")
    template_fields = ("name", "display_name", "secret_type", "bearer_methods_supported", "default_scopes",
        "pkce_required", "token_endpoint", "authorization_endpoint", "registration_endpoint")

    class NoRedirect(urllib.request.HTTPRedirectHandler):
        def redirect_request(self, req, fp, code, msg, headers, newurl):
            return None

    opener = urllib.request.build_opener(NoRedirect())

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

    def collection(path):
        value = request("GET", path)
        assert isinstance(value["data"], list) and value["meta"]["page"]["next"] in (None, "")
        return value["data"]

    def projection(value, templates=False):
        result = {field: value[field] for field in (template_fields if templates else provider_fields)}
        if not templates and value.get("client_id") is not None:
            result["client_id"] = value["client_id"]
        return result

    def invoke(label, args, output="json", failure=None):
        nonlocal phase
        phase = label
        log = work / (label + ".log")
        result = subprocess.run([binary, *args, "-o", output, "--log-level", "trace", "--log-file", str(log), *config],
            capture_output=True, timeout=90, check=False)
        for data in (result.stdout, result.stderr, log.read_bytes() if log.exists() else b""):
            assert not any(needle in data for needle in needles)
            assert b"enrollment_url" not in data and b"state_token" not in data
        (work / (label + ".stdout")).write_bytes(result.stdout)
        (work / (label + ".stderr")).write_bytes(result.stderr)
        if failure:
            assert result.returncode != 0 and failure.lower() in (result.stdout + result.stderr).decode().lower()
            assert result.stdout == b""
            return None
        assert result.returncode == 0, label
        return json.loads(result.stdout) if output == "json" else result.stdout.decode()

    def assert_paged(label, route, count):
        lines = (work / (label + ".log")).read_text().splitlines()
        requests = [line for line in lines if "log_type=http_request " in line and "route=" + route + " " in line]
        assert len(requests) == count and all("page[size]:1" in line for line in requests)
        assert sum("page[after]:" in line for line in requests) == count - 1

    def create(label, template, **fields):
        value = request("POST", root + "/vault/providers",
            {"name": label, "template_name": template, **fields}, expected=201)
        assert value["directory_id"] == directory and value["name"] == label
        known[value["id"]] = value["name"]
        return value

    original = request("GET", root)
    assert original["name"] == name and original["vault_enabled"] is True and original["managed_by"] == {}
    assert collection(root + "/vault/providers") == []
    try:
        templates = collection("/v2/credential-provider-templates")
        expected_templates = {value["name"]: projection(value, True) for value in templates}
        assert len(expected_templates) == 8
        catalog = invoke("template-pages", ["get", "identity", "credential-provider-templates", "--page-size", "1"])
        assert {value["name"]: value for value in catalog} == expected_templates
        assert_paged("template-pages", "/v2/credential-provider-templates", 8)
        for template in templates:
            detail = invoke("template-" + template["name"],
                ["get", "konnect", "identity", "credential-provider-template", template["name"]])
            assert detail == expected_templates[template["name"]]
        template_text = invoke("template-text", ["get", "identity", "provider-template", "static_secret"], "text")
        assert "SECRET_TYPE" in template_text and "static_secret" in template_text and "{" not in template_text
        template_yaml = invoke("template-yaml", ["list", "identity", "provider-templates"], "yaml")
        assert "default_scopes:" in template_yaml and "pkce_required: false" in template_yaml
        assert invoke("empty-providers", ["get", "identity", "vault-providers", "--directory-name", name]) == []
        static = create("static-" + directory, "static_secret", credential_type="shared")
        request("PATCH", root + "/vault/providers/" + static["id"], {"enabled": False, "refresh_buffer_seconds": 45})
        create("github-" + directory, "github", credential_type="shared", client_id="public-fixture-client",
            client_secret=client_secret, scopes=["repo"], pkce_enabled=True,
            token_endpoint="https://provider-fixture.example.test/token",
            authorization_endpoint="https://provider-fixture.example.test/authorize")
        create("custom-" + directory, "custom-oauth", dcr_enabled=True, scopes=["read"], pkce_enabled=True,
            token_endpoint="https://provider-fixture.example.test/token",
            authorization_endpoint="https://provider-fixture.example.test/authorize",
            registration_endpoint="https://provider-fixture.example.test/register")
        create("datadog-" + directory, "datadog")
        uuid_name = "731559d3-39bd-48c7-b33c-eb192847eb20"
        uuid_named = create(uuid_name, "static_secret", credential_type="shared")
        before = {value["id"]: value for value in collection(root + "/vault/providers")}
        assert len(before) == 5
        values = invoke("provider-pages", ["list", "identity", "credential-providers",
            "--directory-name", name, "--page-size", "1"])
        assert {value["id"]: value for value in values} == {key: projection(value) for key, value in before.items()}
        assert_paged("provider-pages", root + "/vault/providers", 5)
        for value in before.values():
            detail = invoke("provider-" + value["id"],
                ["get", "identity", "vault-provider", value["id"], "--directory-id", directory])
            assert detail == projection(value)
            named = invoke("name-" + value["id"],
                ["get", "identity", "vault-provider", "--provider-name", value["name"], "--directory-id", directory,
                 "--page-size", "1"])
            assert named == detail
        by_argument = invoke("name-argument", ["get", "konnect", "identity", "credential-provider",
            static["name"], "--directory-name", name])
        assert by_argument == projection(before[static["id"]])
        by_uuid_name = invoke("uuid-name", ["get", "identity", "vault-provider",
            "--provider-name", uuid_name, "--directory-name", name])
        assert by_uuid_name["id"] == uuid_named["id"]
        text = invoke("provider-text", ["get", "identity", "vault-provider", static["id"],
            "--directory-id", directory], "text")
        assert "DCR_ENABLED" in text and "false" in text and "{" not in text
        yaml = invoke("provider-yaml", ["list", "identity", "vault-providers", "--directory-name", name], "yaml")
        assert "token_endpoint_effective:" in yaml and "enabled: false" in yaml
        invoke("missing-provider", ["get", "identity", "vault-provider", "--provider-name", "missing-" + directory,
            "--directory-id", directory], failure="not found")
        invoke("missing-template", ["get", "identity", "provider-template", "missing-template"],
            failure="failed to get credential provider template")
        assert {value["id"]: value for value in collection(root + "/vault/providers")} == before
        assert request("GET", root)["managed_by"] == {}
        # Catalog discovery is independent of Vault enablement. Provider reads
        # must surface the disabled collection rather than presenting it empty.
        try:
            assert request("PATCH", root, {"vault_enabled": False})["vault_enabled"] is False
            disabled_catalog = invoke("templates-while-disabled", ["get", "identity", "provider-templates"])
            assert {value["name"]: value for value in disabled_catalog} == expected_templates
            invoke("providers-while-disabled", ["get", "identity", "vault-providers", "--directory-id", directory],
                failure="HTTP 403")
        finally:
            assert request("PATCH", root, {"vault_enabled": True})["vault_enabled"] is True
        assert {value["id"]: value for value in collection(root + "/vault/providers")} == before
        for log in work.glob("*.log"):
            for line in log.read_text().splitlines():
                if "log_type=http_request " in line:
                    assert "method=GET " in line and "/credentials" not in line
                    assert "host=" + host.hostname + " " in line
        return {"templates": 8, "providers": 5, "output_formats": 3,
            "complete_public_configuration": True, "real_template_pagination": True, "real_provider_pagination": True,
            "exact_name_and_uuid_selection": True, "templates_without_enabled_vault": True,
            "inspection_preserves_configuration": True, "secret_and_enrollment_values_excluded": True,
            "fixtures_removed": True}
    except Exception:
        raise RuntimeError("Provider inspection phase failed: " + phase) from None
    finally:
        fresh = request("GET", root)
        assert fresh["id"] == directory and fresh["name"] == name and fresh["vault_enabled"] is True
        for value in collection(root + "/vault/providers"):
            assert known.get(value["id"]) == value["name"]
            path = root + "/vault/providers/" + value["id"]
            assert collection(path + "/credentials") == []
            current = request("GET", path)
            assert current["id"] == value["id"] and current["directory_id"] == directory
            request("DELETE", path, expected=204)
        assert collection(root + "/vault/providers") == []


if __name__ == "__main__":
    try:
        print(json.dumps(main()))
    except Exception as error:
        print(str(error) if isinstance(error, RuntimeError) else "Provider inspection or verified cleanup failed", file=sys.stderr)
        sys.exit(1)
