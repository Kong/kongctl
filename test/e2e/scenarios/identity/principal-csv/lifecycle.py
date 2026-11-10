"""Exercise CSV desired state; keep CSV sources and key material private."""

import base64
import copy
import csv
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import urllib.parse
import urllib.request


def main():
    binary, directory, directory_name, artifact_dir = sys.argv[1:5]
    artifacts = Path(artifact_dir).resolve()
    scratch = tempfile.TemporaryDirectory(prefix="kongctl-principal-csv-", dir=os.environ.get("TMPDIR"))
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
    root, namespace = "/v2/directories/" + directory, "identity-principal-csv-e2e"
    marker = {"e2e-csv": directory}
    needles, foreign_id = [], None
    phase, success = "setup", False

    def remember(value):
        needles.extend((value.encode(), base64.b64encode(value.encode())))

    remember(os.environ["KONGCTL_E2E_KONNECT_PAT"])

    class NoRedirect(urllib.request.HTTPRedirectHandler):
        def redirect_request(self, req, fp, code, msg, hdrs, newurl):
            raise RuntimeError("unexpected API redirect")

    opener = urllib.request.build_opener(NoRedirect())

    def request(method, path, body=None):
        req = urllib.request.Request(base.rstrip("/") + path, method=method, headers=headers,
                                     data=None if body is None else json.dumps(body).encode())
        with opener.open(req, timeout=30) as response:
            data = response.read()
            return json.loads(data) if data else None

    def collection(path):
        result, cursors, route = [], set(), path.split("?")[0]
        while True:
            value = request("GET", path)
            assert isinstance(value.get("data"), list)
            result.extend(value["data"])
            after = value["meta"]["page"]["next"]
            if after in (None, ""):
                assert len({v["id"] for v in result}) == len(result)
                return result
            cursor = urllib.parse.parse_qs(urllib.parse.urlparse(after).query).get("page[after]", [None])[0]
            if cursor is None:
                assert "://" not in after and "?" not in after
                cursor = after
            assert cursor and cursor not in cursors
            cursors.add(cursor)
            path = route + "?" + urllib.parse.urlencode({"page[size]": 100, "page[after]": cursor})

    def invoke(label, args, failure=None, raw=False):
        nonlocal phase
        phase = label
        result = subprocess.run([binary, *args, *([] if raw or args[0] == "plan" else ["-o", "json"]),
                                 "--log-level", "trace", "--log-file", str(work / (label + ".log")), *config],
                                capture_output=True, timeout=90, check=False)
        (work / (label + ".stdout")).write_bytes(result.stdout)
        (work / (label + ".stderr")).write_bytes(result.stderr)
        if failure:
            assert result.returncode != 0
            assert failure.lower() in (result.stdout + result.stderr).decode().lower()
            return None
        assert result.returncode == 0
        return result.stdout.decode() if raw else json.loads(result.stdout)

    fields = ["ref", "id", "display_name", "description", "type", "active", "labels", "metadata", "identities", "api_keys", "basic_auths"]

    def document(label, rows, mode="merge"):
        with (work / (label + ".csv")).open("w", newline="") as output:
            writer = csv.DictWriter(output, fieldnames=fields)
            writer.writeheader()
            for row in rows:
                writer.writerow({key: json.dumps(value, separators=(",", ":")) if isinstance(value, (dict, list)) else value
                                 for key, value in row.items()})
        path = work / (label + ".yaml")
        path.write_text("_defaults:\n  kongctl: {namespace: " + namespace + "}\nidentity:\n  directories:\n" +
                        "    - ref: people\n      _external: {id: " + directory + "}\n" +
                        "      principals: !csv {path: " + label + ".csv, metadata_mode: " + mode + "}\n")
        return path

    def plan(label, rows, mode="apply", metadata_mode="merge", failure=None):
        value = invoke(label, ["plan", "--mode", mode, "-f", str(document(label, rows, metadata_mode))], failure)
        if value is not None:
            (work / (label + ".json")).write_text(json.dumps(value, indent=2))
        return value

    def saved(label, source, verb="apply", capture=False, dry_run=False, failure=None):
        args = [verb, "--plan", str(work / (source + ".json")), "--auto-approve"]
        if capture:
            args += ["--secret-output-dir", str(private / label)]
        if dry_run:
            args += ["--dry-run"]
        value = invoke(label, args, failure)
        if value is not None:
            assert value["summary"]["failed"] == 0
            if capture:
                paths = value["execution"]["generated_files"]
                assert len(paths) == 1
                path = Path(paths[0])
                assert path.is_relative_to(private) and path.stat().st_mode & 0o077 == 0
                record = json.loads(path.read_text())
                assert record["status"] == "captured" and record["directory_id"] == directory
                assert record["namespace"] == namespace and record["resource_ref"] == "worker-key"
                remember(record["secret"])
        return value

    try:
        original_directory = request("GET", root)
        assert original_directory["managed_by"] == {} and original_directory["labels"] == {}
        rows = [{"ref": "worker", "display_name": "CSV worker", "description": "Initial CSV worker", "type": "agent",
                 "active": "true", "labels": marker,
                 "metadata": {"team": "fulfillment", "read_only": False, "large": 9007199254740994,
                              "roles": ["worker"], "levels": [1, 2]},
                 "identities": [{"ref": "worker-external", "type": "custom", "key": "external_id", "value": "csv-worker"}],
                 "api_keys": [{"ref": "worker-key", "type": "v1", "name": "CSV worker primary", "expires_at": None}],
                 "basic_auths": [{"ref": "worker-login", "username": "csv-" + directory}]},
                {"ref": "reader", "display_name": "CSV reader", "description": "Initial reader", "type": "user",
                 "active": "false", "labels": marker, "metadata": {"team": "audit", "read_only": True},
                 "identities": [], "api_keys": [], "basic_auths": []}]
        initial_path = document("initial-source", rows)
        inspected = invoke("offline-inspection", ["inspect", "identity", "principal-csv", "--file", str(work / "initial-source.csv"),
                                                "--metadata-mode", "merge", "--base-url", "http://127.0.0.1:9"])
        assert inspected == {"rows": 2, "identities": 1, "api_keys": 1, "basic_auths": 1,
                             "metadata_mode": "merge", "remote_state_checked": False}
        for output in ("text", "yaml"):
            printed = invoke("offline-" + output, ["inspect", "konnect", "identity", "principal-csv", "--file",
                             str(work / "initial-source.csv"), "--metadata-mode", "merge", "-o", output], raw=True)
            assert "remote_state_checked" in printed and "CSV worker" not in printed
        initial = plan("create", rows)
        assert initial["metadata"]["version"] == "1.2" and initial["summary"]["total_changes"] == 5
        saved("initial-dry-run", "create", dry_run=True)
        assert collection(root + "/principals") == []
        saved("missing-capture", "create", failure="secret-output-dir")
        assert collection(root + "/principals") == []
        saved("create-apply", "create", capture=True)
        observed = {value["managed_by"]["kongctl-ref"]: value for value in collection(root + "/principals")}
        worker, reader = observed["worker"]["id"], observed["reader"]["id"]
        worker_path = root + "/principals/" + worker
        assert observed["worker"]["active"] is True and observed["reader"]["active"] is False
        assert observed["worker"]["metadata"]["large"] == 9007199254740994
        assert len(collection(worker_path + "/identities")) == 1
        assert len(collection(worker_path + "/api-keys")) == 1
        assert len(collection(worker_path + "/basic-auths")) == 1
        assert plan("initial-no-op", rows)["summary"]["total_changes"] == 0
        assert "No changes" in invoke("initial-diff", ["diff", "-f", str(initial_path)], raw=True)
        for row in rows:
            row["id"] = observed[row["ref"]]["id"]
        assert plan("asserted-no-op", rows)["summary"]["total_changes"] == 0
        wrong = copy.deepcopy(rows)
        wrong[0]["id"] = reader
        plan("duplicate-uuid", wrong, failure="UUID must be valid and unique")
        wrong[0]["id"] = "5078f559-063b-4f10-9098-9e6553b21d9b"
        plan("wrong-uuid", wrong, failure="id assertion")
        foreign = request("POST", root + "/principals", {"display_name": "Unmanaged CSV neighbor", "type": "agent", "labels": marker})
        foreign_id = foreign["id"]
        wrong[0]["id"] = foreign_id
        plan("unowned-uuid", wrong, failure="id assertion")
        assert len(collection(root + "/principals")) == 3
        rows[0]["description"], rows[0]["labels"] = "", ""
        rows[0]["metadata"] = {"team": "platform"}
        merged = plan("merge", rows)
        assert merged["summary"]["total_changes"] == 1
        change = merged["changes"][0]
        assert change["metadata_merge"] is True and change["fields"]["metadata"] == {"team": "platform"}
        current = request("GET", worker_path)
        body = {key: current[key] for key in ("display_name", "description", "type", "active", "labels", "metadata", "managed_by")}
        body["description"] = "Fresh server description"
        body["labels"]["late"] = "preserved"
        body["metadata"]["late"] = "preserved"
        request("PUT", worker_path, body)
        saved("merge-apply", "merge")
        current = request("GET", worker_path)
        assert current["metadata"]["team"] == "platform" and current["metadata"]["late"] == "preserved"
        assert current["metadata"]["large"] == 9007199254740994 and current["metadata"]["roles"] == ["worker"]
        assert current["description"] == "Fresh server description" and current["labels"]["late"] == "preserved"
        assert plan("merge-no-op", rows)["summary"]["total_changes"] == 0
        legacy = copy.deepcopy(merged)
        legacy["metadata"]["version"] = "1.1"
        (work / "unsafe-legacy.json").write_text(json.dumps(legacy))
        saved("reject-legacy-merge", "unsafe-legacy", failure="merge requires plan version")
        replacement = plan("replace", rows, metadata_mode="sync")
        assert replacement["summary"]["total_changes"] == 1
        assert not replacement["changes"][0].get("metadata_merge", False)
        replacement["metadata"]["version"] = "1.1"
        (work / "legacy-replace.json").write_text(json.dumps(replacement))
        saved("legacy-replace-apply", "legacy-replace")
        assert request("GET", worker_path)["metadata"] == {"team": "platform"}
        rows[0]["metadata"] = {}
        assert plan("clear", rows, metadata_mode="sync")["summary"]["total_changes"] == 1
        saved("clear-apply", "clear")
        assert request("GET", worker_path)["metadata"] == {}
        dumped = invoke("dump", ["dump", "declarative", "--resources", "identity.directories", "--filter-id", directory,
                                 "--include-child-resources", "--default-namespace", namespace], raw=True)
        assert "!csv" not in dumped and "metadata_mode:" not in dumped and "managed_by:" not in dumped
        dump_path = work / "dump.yaml"
        dump_path.write_text(dumped)
        assert invoke("dump-no-op", ["plan", "--mode", "apply", "-f", str(dump_path)])["summary"]["total_changes"] == 0
        for child in ("identities", "api_keys", "basic_auths"):
            rows[0][child] = []
        assert plan("retire-children", rows, mode="sync")["summary"]["total_changes"] == 3
        saved("retire-children-apply", "retire-children", verb="sync")
        assert plan("remove-reader", rows[:1], mode="sync")["summary"]["total_changes"] == 1
        saved("remove-reader-apply", "remove-reader", verb="sync")
        assert plan("empty-csv", [], mode="sync")["summary"]["total_changes"] == 1
        saved("empty-csv-apply", "empty-csv", verb="sync")
        values = collection(root + "/principals")
        assert len(values) == 1 and values[0]["id"] == foreign_id and values[0]["managed_by"] == {}
        success = True
    except Exception as error:
        trace, line = error.__traceback__, 0
        while trace:
            if trace.tb_frame.f_code.co_filename == __file__:
                line = trace.tb_lineno
            trace = trace.tb_next
        raise RuntimeError("Principal CSV lifecycle failed during " + phase + " at script line " + str(line) +
                           "; input and private values withheld") from None
    finally:
        try:
            for principal in collection(root + "/principals"):
                metadata = principal["managed_by"]
                ours = metadata.get("tool") == "kongctl" and metadata.get("kongctl-namespace") == namespace
                assert ours or principal["id"] == foreign_id and principal["labels"] == marker
                path = root + "/principals/" + principal["id"]
                for child_route in ("identities", "api-keys", "basic-auths"):
                    for child in collection(path + "/" + child_route):
                        management = child["managed_by"]
                        assert management.get("tool") == "kongctl"
                        if child_route == "api-keys":
                            assert management.get("kongctl-create-type") == "v1" and management.get("kongctl-ref") == "worker-key"
                        else:
                            assert management.get("kongctl-namespace") == namespace
                        if child_route == "basic-auths":
                            assert collection(path + "/basic-auths/" + child["id"] + "/passwords") == []
                        request("DELETE", path + "/" + child_route + "/" + child["id"])
                    assert collection(path + "/" + child_route) == []
                request("DELETE", path)
            assert collection(root + "/principals") == [] and collection(root + "/groups") == []
            assert request("GET", root) == original_directory
            if success:
                for path in work.iterdir():
                    if path.is_file() and path.suffix != ".csv":
                        assert not any(needle in path.read_bytes() for needle in needles)
                        shutil.copyfile(path, artifacts / path.name)
        finally:
            scratch.cleanup()
    assert not work.exists()
    (artifacts / "private-workspace.json").write_text(json.dumps({"path": str(work), "removed": True,
        "private_sources_and_captures_copied": False, "csv_inputs_copied": False}) + "\n")
    return {"offline_inspection": True, "typed_principal_and_nested_import": True, "generated_key_private_capture": True,
            "uuid_assertion_guards": True, "fresh_metadata_merge": True, "metadata_replacement_and_clear": True,
            "plan_version_guards": True, "dump_reload": True, "scoped_sync_and_foreign_preservation": True,
            "csv_inputs_not_copied": True, "private_workspace_removed": True, "fixtures_removed": True}


if __name__ == "__main__":
    try:
        print(json.dumps(main()))
    except Exception as error:
        if isinstance(error, RuntimeError):
            print(str(error), file=sys.stderr)
        print("Principal CSV lifecycle or verified fixture cleanup failed", file=sys.stderr)
        sys.exit(1)
