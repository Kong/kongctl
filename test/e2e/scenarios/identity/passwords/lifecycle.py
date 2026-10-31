"""Exercise explicit password operations without publishing credential material."""

import base64
import json
import os
from pathlib import Path
import secrets
import subprocess
import sys
import urllib.parse
import urllib.request


def main():
    binary, private_dir, work_dir, directory, directory_name = sys.argv[1:6]
    private, work = Path(private_dir).resolve(), Path(work_dir).resolve()
    artifacts = work.parents[2]
    assert not private.is_relative_to(artifacts)
    target = os.environ.get("KONGCTL_E2E_KONNECT_ENV", "com").strip() or "com"
    assert target in ("com", "tech")
    base = os.environ.get("KONGCTL_E2E_KONNECT_BASE_URL", "").strip() or "https://us.api.konghq." + target
    host = urllib.parse.urlparse(base)
    assert host.scheme == "https" and host.hostname.endswith((".api.konghq.com", ".api.konghq.tech"))
    headers = {"Authorization": "Bearer " + os.environ["KONGCTL_E2E_KONNECT_PAT"], "Content-Type": "application/json"}
    config = ["--config-file", str(Path(os.environ["XDG_CONFIG_HOME"]) / "kongctl/config.yaml")]
    root = "/v2/directories/" + directory
    namespace = "identity-passwords-e2e"
    marker = {"password-fixture": directory}
    fixture_map = {"fixture": directory, "owner": "password-e2e"}
    principals, needles, captures = [], [], []
    phase = "setup"

    def request(method, path, body=None):
        payload = None if body is None else json.dumps(body).encode()
        req = urllib.request.Request(base.rstrip("/") + path, method=method, headers=headers, data=payload)
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
            if next_page is None or next_page == "":
                return result
            cursor = urllib.parse.parse_qs(urllib.parse.urlparse(next_page).query).get("page[after]", [None])[0]
            if cursor is None:
                assert "://" not in next_page and "?" not in next_page
                cursor = next_page
            assert cursor and cursor not in cursors
            cursors.add(cursor)
            path = path.split("?")[0] + "?" + urllib.parse.urlencode({"page[size]": 100, "page[after]": cursor})

    def remember(material):
        needles.extend((material.encode(), base64.b64encode(material.encode())))

    def clean(data):
        assert not any(needle in data for needle in needles)

    def invoke(label, args, failure=None, output="json", env=None):
        nonlocal phase
        phase = label
        log = work / (label + ".log")
        result = subprocess.run([binary, *args, *([] if args[0] == "plan" else ["-o", output]),
                                 "--log-level", "trace", "--log-file", str(log), *config],
                                capture_output=True, timeout=90, check=False, env=env)
        clean(result.stdout)
        clean(result.stderr)
        if log.exists():
            clean(log.read_bytes())
        (work / (label + ".stdout")).write_bytes(result.stdout)
        (work / (label + ".stderr")).write_bytes(result.stderr)
        if failure:
            assert result.returncode != 0
            assert failure.lower() in (result.stdout + result.stderr).decode().lower()
            return None
        assert result.returncode == 0
        return json.loads(result.stdout) if output == "json" else result.stdout.decode()

    def capture(path, expected=None):
        value = json.loads(path.read_text())
        assert value["status"] == "captured" and value["secret"]
        assert value["directory_id"] == directory and value["principal_id"] == principals[0]
        assert value["basic_auth_id"] == basic and value["password_id"]
        assert "auth_server_id" not in value
        assert path.stat().st_mode & 0o077 == 0
        if expected is not None:
            assert value["secret"] == expected
        remember(value["secret"])
        captures.append(path)
        return value["password_id"]

    def scope(principal=None, credential=None, by_name=False):
        parent = ["--directory-name", directory_name] if by_name else ["--directory-id", directory]
        return [*parent, "--principal-id", principal or principals[0],
                *( ["--username", username] if by_name else ["--basic-auth-id", credential or basic])]

    def password(label, verb, args=None, **kwargs):
        return invoke(label, [verb, "identity", "principal-basic-auth-password", *(args or [])], **kwargs)

    def plan(label, path, mode="apply", failure=None):
        return invoke(label, ["plan", "--mode", mode, "-f", str(path)], failure=failure)

    try:
        # Separate fixture labels prevent generic cleanup from cascading through
        # a principal with credentials if this explicitly verified teardown fails.
        for _ in range(2):
            principal = request("POST", root + "/principals", {
                "display_name": "Password lifecycle worker", "type": "agent", "labels": marker})
            principals.append(principal["id"])
        username = directory  # UUID-shaped names must remain exact usernames.
        manifest = {"_defaults": {"kongctl": {"namespace": namespace}}, "identity": {
            "directories": [{"ref": "people", "_external": {"selector": {"matchFields": {"name": directory_name}}}}],
            "principals": [{"ref": "worker", "directory": "people", "_external": {"id": principals[0]},
                            "basic_auths": [{"ref": "login", "username": username}]}]}}
        source = work / "username.yaml"
        source.write_text(json.dumps(manifest))
        assert plan("create-plan", source)["summary"]["total_changes"] == 1
        invoke("create-username", ["apply", "-f", str(source), "--auto-approve"])
        basic = invoke("read-username", ["get", "identity", "principal-basic-auth", "--directory-name", directory_name,
                                        "--principal-id", principals[0], "--username", username])["id"]
        other = request("POST", root + "/principals/" + principals[0] + "/basic-auths", {
            "username": "other-" + directory, "managed_by": fixture_map})["id"]
        expiry = "2031-01-01T00:00:00Z"
        imported, replacement = secrets.token_urlsafe(32), secrets.token_urlsafe(32)
        remember(imported)
        remember(replacement)
        source_file = private / "source.txt"
        with os.fdopen(os.open(source_file, os.O_CREAT | os.O_EXCL | os.O_WRONLY, 0o600), "w") as out:
            out.write(imported)
        env = os.environ.copy()
        env["KONGCTL_E2E_PASSWORD_REPLACEMENT"] = replacement

        password("reject-missing-capture", "create", scope(), failure="secret-output-file")
        password("reject-missing-change", "patch", [directory, *scope()], failure="provide --expires-at")
        password("reject-bad-expiration", "create", [*scope(), "--expires-at", "tomorrow",
                 "--secret-output-file", str(private / "invalid.json")], failure="RFC3339")
        password("reject-missing-source", "create", [*scope(), "--secret-env", "KONGCTL_E2E_UNSET_PASSWORD",
                 "--secret-output-file", str(private / "missing.json")], failure="could not resolve")
        assert not (private / "invalid.json").exists() and not (private / "missing.json").exists()
        generated_path = private / "generated.json"
        generated = password("create-generated", "create", [*scope(by_name=True), "--expires-at", expiry,
                             "--secret-output-file", str(generated_path)])
        first = capture(generated_path)
        assert generated["id"] == first and generated["expires_at"] == expiry and generated["managed_by"] == {}
        password("reject-existing-file", "create", [*scope(), "--secret-output-file", str(generated_path)],
                 failure="file exists")
        link = private / "capture-link.json"
        link.symlink_to(generated_path)
        password("reject-symlink", "create", [*scope(), "--secret-output-file", str(link)], failure="file exists")
        imported_path = private / "imported.json"
        second = password("create-imported-file", "create", [*scope(), "--secret-file", str(source_file),
                          "--no-expiration", "--secret-output-file", str(imported_path)])
        assert capture(imported_path, imported) == second["id"] and second["expires_at"] is None
        password("reject-third-slot", "create", [*scope(), "--secret-output-file", str(private / "full.json")],
                 failure="HTTP 403")
        pending = json.loads((private / "full.json").read_text())
        assert pending["status"] == "creation_not_confirmed" and "secret" not in pending
        assert len(invoke("read-two-pages", ["list", "konnect", "identity", "principal-basic-auth-passwords",
                                            *scope(), "--page-size", "1"])) == 2

        for label, verb, args in (
            ("wrong-principal-patch", "patch", [first, *scope(principals[1]), "--no-expiration"]),
            ("wrong-username-replace", "patch", [first, *scope(credential=other), "--secret-file", str(source_file)]),
            ("wrong-username-delete", "delete", [first, *scope(credential=other), "--auto-approve"]),
            ("wrong-principal-delete-all", "delete", [*scope(principals[1]), "--all", "--auto-approve"]),
        ):
            password(label, verb, args, failure="HTTP 404")
        changed = password("replace-env-preserving-expiration", "patch", [first, *scope(), "--secret-env",
                           "KONGCTL_E2E_PASSWORD_REPLACEMENT"], env=env)
        assert changed["expires_at"] == expiry and changed["managed_by"] == {}
        changed = password("clear-expiration", "patch", [first, *scope(), "--no-expiration"])
        assert changed["expires_at"] is None
        changed = password("replace-file-and-expiration", "patch", [first, *scope(by_name=True), "--secret-file",
                           str(source_file), "--expires-at", expiry])
        assert changed["expires_at"] == expiry
        text = password("patch-text", "patch", [first, *scope(), "--expires-at", "2020-01-01T00:00:00Z"], output="text")
        assert "EXPIRES_AT" in text and first in text and "{" not in text
        password("expired-still-occupies-slot", "create", [*scope(), "--secret-output-file", str(private / "expired-full.json")],
                 failure="HTTP 403")
        assert plan("username-no-op-with-passwords", source)["summary"]["total_changes"] == 0
        plan("block-declarative-username-delete", source, "delete", failure="password")
        password("delete-one", "delete", [first, *scope(), "--auto-approve"])
        password("confirm-single-absent", "get", [first, *scope()], failure="HTTP 404")
        assert len(invoke("one-remains", ["get", "identity", "principal-basic-auth-passwords", *scope()])) == 1
        password("delete-all", "delete", [*scope(by_name=True), "--all", "--auto-approve"])
        password("delete-all-empty", "delete", [*scope(), "--all", "--auto-approve"])
        assert invoke("empty-passwords", ["get", "identity", "principal-basic-auth-passwords", *scope()]) == []

        # Imperative writes preserve arbitrary existing management metadata;
        # they do not silently enroll credentials into declarative ownership.
        path = root + "/principals/" + principals[0] + "/basic-auths/" + basic + "/passwords"
        seeded = request("POST", path, {"secret": imported, "expires_at": expiry, "managed_by": fixture_map})
        seeded.pop("secret")
        assert seeded["managed_by"] == fixture_map
        value = password("preserve-map-expiration", "patch", [seeded["id"], *scope(), "--no-expiration"])
        assert value["managed_by"] == fixture_map and value["expires_at"] is None
        value = password("preserve-map-replacement", "patch", [seeded["id"], *scope(), "--secret-env",
                         "KONGCTL_E2E_PASSWORD_REPLACEMENT"], env=env)
        assert value["managed_by"] == fixture_map and value["expires_at"] is None
        yaml = password("patch-yaml", "patch", [seeded["id"], *scope(), "--expires-at", expiry], output="yaml")
        assert "managed_by:" in yaml and "secret:" not in yaml
        password("delete-seeded", "delete", [seeded["id"], *scope(), "--auto-approve"])
        env_path = private / "imported-env.json"
        value = password("create-imported-env", "create", [*scope(), "--secret-env", "KONGCTL_E2E_PASSWORD_REPLACEMENT",
                         "--secret-output-file", str(env_path)], env=env)
        assert capture(env_path, replacement) == value["id"]
        assert value["expires_at"] is None
        password("retire-imported-env", "delete", [value["id"], *scope(), "--auto-approve"])
        assert plan("username-no-op-after-passwords", source)["summary"]["total_changes"] == 0
        invoke("delete-declared-username", ["delete", "-f", str(source), "--auto-approve"])
        assert request("GET", root + "/principals/" + principals[0])["labels"] == marker
        assert request("GET", root)["name"] == directory_name
        phase = "verify-artifacts"
        for file in artifacts.rglob("*"):
            if file.is_file():
                clean(file.read_bytes())
        assert len(captures) == 3
        result = {"generated_and_imported": True, "expiration_and_replacement": True, "metadata_preserved": True,
                  "occupied_slots_checked": True, "wrong_parent_rejected": True, "declarative_deletion_guards": True,
                  "declarative_plan_unchanged": True, "single_and_all_deletion": True, "output_formats": 3,
                  "captures_private": True, "artifacts_clean": True}
    except Exception:
        print("password lifecycle verification failed at " + phase, file=sys.stderr)
        raise
    finally:
        # Inspect every descendant before any teardown write; reject unexpected
        # ownership and never rely on the API's principal/username cascades.
        observed = []
        for principal in principals:
            path = root + "/principals/" + principal
            assert request("GET", path)["labels"] == marker
            assert collection(path + "/identities") == [] and collection(path + "/api-keys") == []
            for basic_value in collection(path + "/basic-auths"):
                managed = basic_value["managed_by"]
                assert managed == fixture_map or (managed.get("tool") == "kongctl" and
                    managed.get("kongctl-namespace") == namespace and managed.get("kongctl-protected") == "false")
                children = collection(path + "/basic-auths/" + basic_value["id"] + "/passwords")
                assert all(v["managed_by"] in ({}, fixture_map) for v in children)
                observed.append((path, basic_value, children))
        for path, basic_value, children in observed:
            password_path = path + "/basic-auths/" + basic_value["id"] + "/passwords"
            for value in children:
                assert request("GET", password_path + "/" + value["id"])["managed_by"] == value["managed_by"]
                request("DELETE", password_path + "/" + value["id"])
            assert collection(password_path) == []
            assert request("GET", path + "/basic-auths/" + basic_value["id"])["managed_by"] == basic_value["managed_by"]
            request("DELETE", path + "/basic-auths/" + basic_value["id"])
        for principal in principals:
            path = root + "/principals/" + principal
            assert collection(path + "/basic-auths") == [] and request("GET", path)["labels"] == marker
            request("DELETE", path)
        assert collection(root + "/principals") == []
    result["fixtures_removed"] = True
    return result


if __name__ == "__main__":
    try:
        print(json.dumps(main()))
    except Exception:
        # Parser/HTTP exceptions can contain secret material; no tracebacks.
        print("password lifecycle or verified fixture cleanup failed", file=sys.stderr)
        sys.exit(1)
