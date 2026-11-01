"""Exercise credential administration without publishing private key material."""

import base64
import json
import os
from pathlib import Path
import secrets
import shutil
import subprocess
import sys
import tempfile
import urllib.parse
import urllib.request


def main():
    binary, directory, directory_name, artifact_dir = sys.argv[1:5]
    artifacts = Path(artifact_dir).resolve()
    scratch = tempfile.TemporaryDirectory(prefix="kongctl-api-key-writes-", dir=os.environ.get("TMPDIR"))
    work, private = Path(scratch.name), Path(scratch.name) / "private"
    private.mkdir(mode=0o700)
    assert not work.is_relative_to(artifacts.parents[2])
    target = os.environ.get("KONGCTL_E2E_KONNECT_ENV", "com").strip() or "com"
    assert target in ("com", "tech")
    base = os.environ.get("KONGCTL_E2E_KONNECT_BASE_URL", "").strip() or "https://us.api.konghq." + target
    host = urllib.parse.urlparse(base)
    assert host.scheme == "https" and host.hostname.endswith((".api.konghq.com", ".api.konghq.tech"))
    headers = {"Authorization": "Bearer " + os.environ["KONGCTL_E2E_KONNECT_PAT"], "Content-Type": "application/json"}
    config = ["--config-file", str(Path(os.environ["XDG_CONFIG_HOME"]) / "kongctl/config.yaml")]
    root = "/v2/directories/" + directory
    namespace = "identity-key-writes-e2e"
    fixture_map = {"fixture": directory, "owner": "key-write-e2e"}
    needles = []
    parents, known = [], set()
    phase, success = "setup", False

    def request(method, path, body=None):
        req = urllib.request.Request(base.rstrip("/") + path, method=method, headers=headers,
                                     data=None if body is None else json.dumps(body).encode())
        with urllib.request.urlopen(req, timeout=30) as response:
            data = response.read()
            return json.loads(data) if data else None

    def collection(path):
        result, cursors = [], set()
        route = path.split("?")[0]
        while True:
            value = request("GET", path)
            assert isinstance(value.get("data"), list)
            result.extend(value["data"])
            next_page = value["meta"]["page"]["next"]
            if next_page in (None, ""):
                assert len({v["id"] for v in result}) == len(result)
                return result
            cursor = urllib.parse.parse_qs(urllib.parse.urlparse(next_page).query).get("page[after]", [None])[0]
            if cursor is None:
                assert "://" not in next_page and "?" not in next_page
                cursor = next_page
            assert cursor and cursor not in cursors
            cursors.add(cursor)
            path = route + "?" + urllib.parse.urlencode({"page[size]": 100, "page[after]": cursor})

    def remember(value):
        needles.extend((value.encode(), base64.b64encode(value.encode())))

    def clean(data):
        assert not any(needle in data for needle in needles)

    remember(os.environ["KONGCTL_E2E_KONNECT_PAT"])

    def invoke(label, args, failure=None, output="json", env=None):
        nonlocal phase
        phase = label
        log = work / (label + ".log")
        result = subprocess.run([binary, *args, *([] if args[0] == "plan" else ["-o", output]),
                                 "--log-level", "trace", "--log-file", str(log), *config],
                                capture_output=True, timeout=90, check=False, env=env)
        (work / (label + ".stdout")).write_bytes(result.stdout)
        (work / (label + ".stderr")).write_bytes(result.stderr)
        if failure:
            assert result.returncode != 0
            assert failure.lower() in (result.stdout + result.stderr).decode().lower()
            return None
        assert result.returncode == 0
        return json.loads(result.stdout) if output == "json" else result.stdout.decode()

    def capture(path, kind, parent, expected=None):
        value = json.loads(path.read_text())
        assert value["directory_id"] == directory and value[kind + "_id"] == parent
        assert value["api_key_id"] and value["status"] == "captured" and value["secret"]
        known.add(value["api_key_id"])
        assert path.stat().st_mode & 0o077 == 0
        if expected is not None:
            assert value["secret"] == expected
        remember(value["secret"])
        return value["api_key_id"]

    def scope(kind, parent, by_name=False):
        return [*( ["--directory-name", directory_name] if by_name else ["--directory-id", directory]),
                "--" + kind + "-id", parent]

    def key(label, kind, verb, args=None, **kwargs):
        return invoke(label, [verb, "identity", kind + "-api-key", *(args or [])], **kwargs)

    def source(name, value):
        path = private / name
        with os.fdopen(os.open(path, os.O_CREAT | os.O_EXCL | os.O_WRONLY, 0o600), "w") as out:
            out.write(value)
        return path

    try:
        manifest = {"_defaults": {"kongctl": {"namespace": namespace}}, "identity": {
            "directories": [{"ref": "people", "_external": {"selector": {"matchFields": {"name": directory_name}}}}],
            "principals": [{"ref": "worker", "directory": "people", "display_name": "Key lifecycle worker", "type": "agent"},
                           {"ref": "other-worker", "directory": "people", "display_name": "Other key worker", "type": "user"}],
            "groups": [{"ref": "workers", "directory": "people", "name": "KeyLifecycleWorkers", "type": "group"},
                       {"ref": "other-workers", "directory": "people", "name": "OtherKeyWorkers", "type": "group"}]}}
        manifest_path = work / "parents.yaml"
        manifest_path.write_text(json.dumps(manifest))
        invoke("apply-parents", ["apply", "-f", str(manifest_path), "--auto-approve"])
        principal_values, group_values = collection(root + "/principals"), collection(root + "/groups")
        assert len(principal_values) == 2 and len(group_values) == 2
        for kind, values in (("principal", principal_values), ("group", group_values)):
            for value in values:
                assert value["managed_by"]["tool"] == "kongctl"
                assert value["managed_by"]["kongctl-namespace"] == namespace
                parents.append((kind, value["id"]))
                assert collection(root + "/" + ("principals" if kind == "principal" else "groups") + "/" + value["id"] + "/api-keys") == []
        for kind, values in (("principal", principal_values), ("group", group_values)):
            parent, other = values[0]["id"], values[1]["id"]
            selected = scope(kind, parent)
            expiry = "2031-01-01T00:00:00Z"
            imported = secrets.token_urlsafe(32)
            env_material = secrets.token_urlsafe(32)
            remember(imported)
            remember(env_material)
            source_path = source(kind + "-source.txt", imported)
            env = os.environ.copy()
            env["KONGCTL_E2E_KEY_WRITE_SOURCE"] = env_material
            key(kind + "-missing-capture", kind, "create", selected, failure="secret-output-file")
            key(kind + "-missing-source", kind, "create", [*selected, "--secret-env", "KONGCTL_E2E_UNSET_KEY_WRITE",
                "--secret-output-file", str(private / (kind + "-missing.json"))], failure="could not resolve")
            key(kind + "-bad-expiration", kind, "create", [*selected, "--expires-at", "tomorrow",
                "--secret-output-file", str(private / (kind + "-invalid.json"))], failure="RFC3339")
            assert not (private / (kind + "-missing.json")).exists() and not (private / (kind + "-invalid.json")).exists()
            generated_path = private / (kind + "-generated.json")
            generated = key(kind + "-generated", kind, "create", [*scope(kind, parent, True), "--name", "primary",
                "--expires-at", expiry, "--secret-output-file", str(generated_path)])
            first = capture(generated_path, kind, parent)
            assert generated["id"] == first and generated["name"] == "primary" and generated["expires_at"] == expiry
            assert generated["managed_by"] == {}
            key(kind + "-existing-capture", kind, "create", [*selected, "--secret-output-file", str(generated_path)], failure="file exists")
            link = private / (kind + "-link.json")
            link.symlink_to(generated_path)
            key(kind + "-symlink-capture", kind, "create", [*selected, "--secret-output-file", str(link)], failure="file exists")
            if kind == "group":
                key("group-import-name-rejected", kind, "create", [*selected, "--secret-file", str(source_path),
                    "--name", "unsupported", "--secret-output-file", str(private / "group-unsupported.json")], failure="imported group")
                key("group-rename-rejected", kind, "patch", [first, *selected, "--name", "unsupported"], failure="unknown flag")
                assert not (private / "group-unsupported.json").exists()
            else:
                changed = key("principal-rename-preserves-expiration", kind, "patch", [first, *selected, "--name", "renamed"])
                assert changed["expires_at"] == expiry and changed["name"] == "renamed"
            imported_path = private / (kind + "-imported.json")
            second = key(kind + "-imported", kind, "create", [*selected, "--secret-file", str(source_path), "--no-expiration",
                "--secret-output-file", str(imported_path), *(["--name", "supplied"] if kind == "principal" else [])])
            assert capture(imported_path, kind, parent, imported) == second["id"] and second["expires_at"] is None
            third_path = private / (kind + "-env.json")
            third = key(kind + "-imported-env", kind, "create", [*selected, "--secret-env", "KONGCTL_E2E_KEY_WRITE_SOURCE",
                "--secret-output-file", str(third_path)], env=env)
            assert capture(third_path, kind, parent, env_material) == third["id"]
            listed = invoke(kind + "-pagination", ["list", "konnect", "identity", kind + "-api-keys", *selected, "--page-size", "1"])
            assert {v["id"] for v in listed} == {first, second["id"], third["id"]}
            trace = (work / (kind + "-pagination.log")).read_text()
            requests = [line for line in trace.splitlines() if "log_type=http_request " in line and "/api-keys " in line]
            assert len(requests) == 3 and all("page[size]:1" in line for line in requests)
            assert sum("page[after]:" in line for line in requests) == 2
            for verb, extra in (("get", []), ("patch", ["--no-expiration"]), ("delete", ["--auto-approve"])):
                key(kind + "-wrong-parent-" + verb, kind, verb, [first, *scope(kind, other), *extra], failure="HTTP 404")
            changed = key(kind + "-expiration", kind, "patch", [first, *selected, "--expires-at", "2032-01-01T00:00:00Z"])
            assert changed["expires_at"] == "2032-01-01T00:00:00Z" and changed["managed_by"] == {}
            changed = key(kind + "-clear", kind, "patch", [first, *selected, "--no-expiration"])
            assert changed["expires_at"] is None
            text = key(kind + "-detail-text", kind, "get", [first, *selected], output="text")
            assert "id: " + first in text and "expires_at: -" in text and "{" not in text
            yaml = key(kind + "-detail-yaml", kind, "get", [first, *selected], output="yaml")
            assert "managed_by:" in yaml and "secret:" not in yaml
            text = key(kind + "-list-text", kind, "list", selected, output="text")
            assert "EXPIRES_AT" in text and first in text
            assert invoke(kind + "-parent-no-op", ["plan", "--mode", "apply", "-f", str(manifest_path)])["summary"]["total_changes"] == 0
            one = json.loads(json.dumps(manifest))
            one["identity"].pop("groups" if kind == "principal" else "principals")
            guard = work / (kind + "-guard.yaml")
            guard.write_text(json.dumps(one))
            invoke(kind + "-delete-guard", ["plan", "--mode", "delete", "-f", str(guard)], failure="API key")
            # Verify that explicit metadata mutations preserve a foreign map.
            route = root + "/" + ("principals" if kind == "principal" else "groups") + "/" + parent + "/api-keys"
            seeded = request("POST", route, {"type": "v1", "managed_by": fixture_map, "expires_at": expiry})
            known.add(seeded["id"])
            remember(seeded.pop("secret"))
            changed = key(kind + "-foreign-map-preserved", kind, "patch", [seeded["id"], *selected, "--no-expiration"])
            assert changed["managed_by"] == fixture_map and changed["expires_at"] is None
            for index, key_id in enumerate((first, second["id"], third["id"], seeded["id"])):
                key(kind + "-retire-" + str(index), kind, "delete", [key_id, *selected, "--auto-approve"])
            assert key(kind + "-empty", kind, "get", selected) == []
            key(kind + "-retired-absent", kind, "get", [first, *selected], failure="HTTP 404")
        invoke("delete-parents", ["delete", "-f", str(manifest_path), "--auto-approve"])
        parents.clear()
        assert collection(root + "/principals") == [] and collection(root + "/groups") == []
        assert request("GET", root)["name"] == directory_name
        success = True
    except Exception:
        raise RuntimeError("API-key lifecycle failed during " + phase + "; private values withheld") from None
    finally:
        try:
            # Teardown requires known fixture IDs, unchanged parent ownership,
            # complete collections and fresh detail before every deletion.
            observations = []
            for kind, parent in parents:
                path = root + "/" + ("principals" if kind == "principal" else "groups") + "/" + parent
                current = request("GET", path)
                assert current["managed_by"].get("kongctl-namespace") == namespace
                for value in collection(path + "/api-keys"):
                    assert value["id"] in known and value["managed_by"] in ({}, fixture_map)
                    observations.append((path, value))
            for path, value in observations:
                assert request("GET", path + "/api-keys/" + value["id"])["managed_by"] == value["managed_by"]
                request("DELETE", path + "/api-keys/" + value["id"])
            for kind, parent in parents:
                path = root + "/" + ("principals" if kind == "principal" else "groups") + "/" + parent
                assert collection(path + "/api-keys") == []
                if kind == "principal":
                    assert collection(path + "/identities") == [] and collection(path + "/basic-auths") == []
                else:
                    assert collection(path + "/membership") == []
                request("DELETE", path)
            if success:
                for path in work.iterdir():
                    if path.is_file():
                        clean(path.read_bytes())
                        shutil.copyfile(path, artifacts / path.name)
        finally:
            scratch.cleanup()
    assert not work.exists()
    (artifacts / "private-workspace.json").write_text(json.dumps({"path": str(work), "removed": True,
        "private_sources_and_captures_copied": False}) + "\n")
    return {"generated_and_imported": True, "private_capture_and_sources": True, "expiration_and_name_updates": True,
            "complete_public_pagination": True, "ownership_preserved": True, "wrong_parent_rejected": True,
            "explicit_rotation_and_retirement": True, "declarative_parent_guards": True, "unsupported_changes_rejected": True,
            "output_formats": 3, "private_workspace_removed": True, "fixtures_removed": True}


if __name__ == "__main__":
    try:
        print(json.dumps(main()))
    except Exception as error:
        # HTTP/SDK diagnostics may contain key material. Never emit a traceback.
        if isinstance(error, RuntimeError):
            print(str(error), file=sys.stderr)
        print("API-key lifecycle or verified fixture cleanup failed", file=sys.stderr)
        sys.exit(1)
