"""Exercise credential reads; password material never enters scenario output."""

import base64
import datetime
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
    artifact_root = work.parents[2]
    assert not private.is_relative_to(artifact_root)
    target = os.environ.get("KONGCTL_E2E_KONNECT_ENV", "com").strip() or "com"
    assert target in ("com", "tech")
    base = os.environ.get("KONGCTL_E2E_KONNECT_BASE_URL", "").strip() or "https://us.api.konghq." + target
    parsed = urllib.parse.urlparse(base)
    assert parsed.scheme == "https" and parsed.hostname.endswith((".api.konghq.com", ".api.konghq.tech"))
    headers = {"Authorization": "Bearer " + os.environ["KONGCTL_E2E_KONNECT_PAT"], "Content-Type": "application/json"}
    config = []
    if os.environ.get("KONGCTL_PROFILE") == "e2e":
        config = ["--config-file", str(Path(os.environ["XDG_CONFIG_HOME"]) / "kongctl/config.yaml")]
    root = "/v2/directories/" + directory
    fixture_label = "identity-basic-auth-e2e"
    metadata = {"tool": "kongctl", "version": "e2e-fixture", "kongctl-namespace": fixture_label,
                "kongctl-protected": "false", "kongctl-ref": directory}
    principals, captures, needles = [], [], []

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

    def clean_text(data):
        assert not any(needle in data for needle in needles)

    def run(label, noun, args, output="json", fail=False, verb="get", explicit_product=False):
        log = work / (label + ".log")
        product = ["konnect"] if explicit_product else []
        result = subprocess.run([binary, verb, *product, "identity", noun, *args, "-o", output,
                                 "--log-level", "trace", "--log-file", str(log), *config],
                                capture_output=True, timeout=60, check=False)
        clean_text(result.stdout)
        clean_text(result.stderr)
        if log.exists():
            clean_text(log.read_bytes())
        # Keep only verified public output in artifacts.
        (work / (label + ".stdout")).write_bytes(result.stdout)
        (work / (label + ".stderr")).write_bytes(result.stderr)
        if fail:
            assert result.returncode != 0 and b"HTTP 404" in result.stderr
            return None
        assert result.returncode == 0
        if output == "json":
            value = json.loads(result.stdout)
            items = value if isinstance(value, list) else [value]
            assert all("secret" not in v and "password" not in v for v in items)
            return value
        return result.stdout.decode()

    def scope(principal, by_name=False):
        return ["--directory-name", directory_name, "--principal-id", principal] if by_name else [
            "--directory-id", directory, "--principal-id", principal]

    def traversed(label, suffix):
        lines = (work / (label + ".log")).read_text().splitlines()
        requests = [line for line in lines if "log_type=http_request " in line and suffix + " " in line]
        return (len(requests) >= 2 and all("page[size]:1" in line for line in requests)
                and any("page[after]:" in line for line in requests))

    try:
        for kind in ("agent", "user", None, "agent"):
            # Deliberately omit the harness's generic e2e-fixture label. If this
            # verified teardown cannot finish, generic cleanup must refuse a
            # principal cascade instead of erasing unexpected credentials.
            value = request("POST", root + "/principals", {
                "type": kind, "display_name": "Basic auth metadata inspection",
                "labels": {fixture_label: directory},
            })
            principals.append(value["id"])
        records = []
        for index, principal in enumerate(principals[:3]):
            usernames = [directory, "worker-" + directory] if index == 0 else ["worker-" + str(index) + "-" + directory]
            for username in usernames:
                path = root + "/principals/" + principal + "/basic-auths"
                basic = request("POST", path, {"username": username, "managed_by": metadata})
                assert basic["managed_by"] == metadata
                records.append((principal, basic["id"], username))
        expiry = (datetime.datetime.now(datetime.timezone.utc) + datetime.timedelta(days=3)).isoformat()
        passwords = []
        for index, (principal, basic, _) in enumerate(records):
            for slot in range(2 if index == 0 else 1):
                body = {"managed_by": metadata}
                if slot == 0:
                    body["expires_at"] = expiry
                if slot == 1:
                    body["secret"] = secrets.token_urlsafe(32)
                capture = private / ("password-" + str(index) + "-" + str(slot) + ".json")
                with os.fdopen(os.open(capture, os.O_CREAT | os.O_EXCL | os.O_WRONLY, 0o600), "w+") as output:
                    json.dump({"status": "creation_not_confirmed", "directory_id": directory,
                               "principal_id": principal, "basic_auth_id": basic}, output)
                    output.flush()
                    os.fsync(output.fileno())
                    value = request("POST", root + "/principals/" + principal + "/basic-auths/" + basic + "/passwords", body)
                    assert value.get("secret") and value["managed_by"] == metadata
                    if "secret" in body:
                        assert value["secret"] == body["secret"]
                    material = value["secret"].encode()
                    needles.extend((material, base64.b64encode(material)))
                    output.seek(0)
                    output.truncate()
                    json.dump({"status": "captured", "id": value["id"], "secret": value["secret"]}, output)
                    output.flush()
                    os.fsync(output.fileno())
                captures.append(capture)
                passwords.append((principal, basic, value["id"]))
        agent, basic, uuid_username = records[0]
        values = run("usernames-page-one", "principal-basic-auths", [*scope(agent), "--page-size", "1"])
        assert {v["id"] for v in values} == {r[1] for r in records if r[0] == agent}
        assert all(v["managed_by"] == metadata for v in values)
        assert len(run("list-explicit-product", "principal-basic-auths", scope(agent, True),
                       verb="list", explicit_product=True)) == 2
        assert run("uuid-name", "principal-basic-auth", [*scope(agent, True), "--username", uuid_username])["id"] == basic
        assert run("uuid-id", "principal-basic-auth", [basic, *scope(agent)])["username"] == uuid_username
        values = run("passwords-page-one", "principal-basic-auth-passwords", [*scope(agent), "--basic-auth-id", basic,
                                                                               "--page-size", "1"])
        assert len(values) == 2 and sum(v["expires_at"] is None for v in values) == 1
        assert all(v["managed_by"] == metadata for v in values)
        assert len(run("list-passwords-explicit-product", "principal-basic-auth-passwords",
                       [*scope(agent, True), "--username", uuid_username],
                       verb="list", explicit_product=True)) == 2
        password_id = passwords[0][2]
        assert run("password-detail", "principal-basic-auth-password", [password_id, *scope(agent), "--username",
                                                                        uuid_username])["id"] == password_id
        for label, noun, args in (
            ("username-text", "principal-basic-auth", [basic, *scope(agent)]),
            ("username-list-text", "principal-basic-auths", scope(agent)),
            ("password-text", "principal-basic-auth-password", [password_id, *scope(agent), "--basic-auth-id", basic]),
            ("password-list-text", "principal-basic-auth-passwords", [*scope(agent), "--username", uuid_username]),
        ):
            output = run(label, noun, args, "text")
            assert "ID" in output and ("EXPIRES_AT" if "password" in noun else "USERNAME") in output
            assert "{" not in output and "secret" not in output
        for label, noun, args in (
            ("username-yaml", "principal-basic-auth", [basic, *scope(agent)]),
            ("password-yaml", "principal-basic-auth-password", [password_id, *scope(agent), "--basic-auth-id", basic]),
        ):
            output = run(label, noun, args, "yaml")
            assert "managed_by:" in output and "secret:" not in output and "password:" not in output
        for index, (principal, credential, username) in enumerate(records[1:], start=1):
            assert run("lookup-" + str(index), "principal-basic-auth", [*scope(principal), "--username", username])["id"] == credential
            assert len(run("passwords-" + str(index), "principal-basic-auth-passwords",
                           [*scope(principal), "--basic-auth-id", credential])) == 1
        assert run("empty-usernames", "principal-basic-auths", scope(principals[-1])) == []
        empty = request("POST", root + "/principals/" + principals[-1] + "/basic-auths",
                        {"username": "empty-" + directory, "managed_by": metadata})
        assert run("empty-passwords", "principal-basic-auth-passwords", [*scope(principals[-1], True),
                                                                        "--basic-auth-id", empty["id"]]) == []
        run("wrong-principal", "principal-basic-auth", [basic, *scope(principals[-1])], fail=True)
        run("wrong-password-principal", "principal-basic-auth-password", [password_id, *scope(principals[-1]),
                                                                          "--basic-auth-id", basic], fail=True)
        run("wrong-password-credential", "principal-basic-auth-password", [password_id, *scope(agent),
                                                                           "--basic-auth-id", records[1][1]], fail=True)
        assert traversed("usernames-page-one", "/basic-auths")
        assert traversed("passwords-page-one", "/passwords")
        assert len(captures) == 5 and all(p.stat().st_mode & 0o077 == 0 for p in captures)
        for path in artifact_root.rglob("*"):
            if path.is_file():
                clean_text(path.read_bytes())
        result = {"principal_types": 3, "username_records": 5, "password_records": 5,
                  "real_username_pagination": True, "real_password_pagination": True, "output_formats": 3,
                  "wrong_parent_rejected": True, "captures_private": True, "artifacts_clean": True}
    finally:
        # Observe and verify every descendant before any cleanup writes. An
        # uncertain create is recovered by the selected principal's collection.
        observed = []
        for principal in principals:
            path = root + "/principals/" + principal
            fresh = request("GET", path)
            assert fresh["labels"] == {fixture_label: directory}
            assert collection(path + "/identities") == [] and collection(path + "/api-keys") == []
            for basic in collection(path + "/basic-auths"):
                assert basic["managed_by"] == metadata
                password_path = path + "/basic-auths/" + basic["id"] + "/passwords"
                children = collection(password_path)
                assert all(v["managed_by"] == metadata for v in children)
                observed.append((path, basic["id"], children))
        for path, basic, children in observed:
            password_path = path + "/basic-auths/" + basic + "/passwords"
            for value in children:
                fresh = request("GET", password_path + "/" + value["id"])
                assert fresh["managed_by"] == metadata
                request("DELETE", password_path + "/" + value["id"])
            assert collection(password_path) == []
            fresh = request("GET", path + "/basic-auths/" + basic)
            assert fresh["managed_by"] == metadata
            request("DELETE", path + "/basic-auths/" + basic)
        for principal in principals:
            path = root + "/principals/" + principal
            assert collection(path + "/basic-auths") == []
            assert request("GET", path)["labels"] == {fixture_label: directory}
            request("DELETE", path)
        assert collection(root + "/principals") == []
    result["fixtures_removed"] = True
    return result


if __name__ == "__main__":
    try:
        print(json.dumps(main()))
    except Exception:
        print("basic-auth metadata inspection or verified fixture cleanup failed", file=sys.stderr)
        sys.exit(1)
