"""Exercise explicit native import while keeping HR sources/reports private."""

import base64
import csv
import hashlib
import io
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
    binary, directory, name, location = sys.argv[1:5]
    artifacts = Path(location)
    scratch = tempfile.TemporaryDirectory(prefix="kongctl-org-chart-import-", dir=os.environ.get("TMPDIR"))
    work, private = Path(scratch.name), Path(scratch.name) / "private"
    private.mkdir(mode=0o700)
    target = os.environ.get("KONGCTL_E2E_KONNECT_ENV", "com").strip() or "com"
    assert target in ("com", "tech")
    base = os.environ.get("KONGCTL_E2E_KONNECT_BASE_URL", "").strip() or "https://us.api.konghq." + target
    host = urllib.parse.urlsplit(base)
    assert host.scheme == "https" and host.hostname.endswith((".api.konghq.com", ".api.konghq.tech"))
    token = os.environ["KONGCTL_E2E_KONNECT_PAT"]
    opener = urllib.request.build_opener(NoRedirect())
    headers = {"Authorization": "Bearer " + token, "Content-Type": "application/json"}
    root = "/v2/directories/" + directory
    config = ["--config-file", str(Path(os.environ["XDG_CONFIG_HOME"]) / "kongctl/config.yaml")]
    prefix = secrets.token_hex(12)
    columns = ["external_id", "email", "given_name", "family_name", "manager_external_id"]
    ceo = [prefix + "-ceo", prefix + "-ceo@example.test", prefix + "-root", prefix + "-executive", ""]
    manager = [prefix + "-manager", prefix + "-manager@example.test", prefix + "-manager-name", prefix + "-lead", ceo[0]]
    worker = [prefix + "-worker", prefix + "-worker@example.test", prefix + "-worker-name", prefix + "-person", manager[0]]
    costs = [prefix + "-EXEC", prefix + "-OPS", prefix + "-FULFILL"]
    hidden = list({token, *ceo, *manager, *worker, *costs, prefix} - {""})
    phase, attempted_execution = "setup", False
    known = set()

    def public(data):
        assert all(value.encode() not in data and base64.b64encode(value.encode()) not in data for value in hidden)

    def request(method, path, body=None, expected=200, content_type="application/json"):
        req = urllib.request.Request(base.rstrip("/") + path, method=method,
            headers={**headers, "Content-Type": content_type},
            data=body if isinstance(body, bytes) else None if body is None else json.dumps(body).encode())
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

    def snapshot():
        principals = collection("principals")
        return {"principals": principals, "groups": collection("groups"), "identities": {
            value["id"]: collection("principals/" + value["id"] + "/identities") for value in principals}}

    def indexed():
        result = {}
        for value in collection("principals"):
            if value["id"] in known:
                continue
            for identity in collection("principals/" + value["id"] + "/identities"):
                if identity.get("key") == "kong:external_id":
                    assert identity["value"].startswith(prefix + "-")
                    result[identity["value"]] = value
        return result

    def write_csv(label, rows, cost_values=None):
        path = private / (label + ".csv")
        with path.open("w", newline="") as source:
            writer = csv.writer(source)
            writer.writerow(columns + ([] if cost_values is None else ["cost_center"]))
            writer.writerows(row + ([] if cost_values is None else [cost_values[i]]) for i, row in enumerate(rows))
        path.chmod(0o600)
        return path

    def invoke(label, source, mode="--dry-run", output="json", failure=None, report_dir=None, explicit=False):
        nonlocal phase, attempted_execution
        phase = label
        report_dir = report_dir or private / (label + "-reports")
        before_reports = set(report_dir.glob("*.json"))
        args = [binary, "import", *(["konnect"] if explicit else []), "identity", "org-chart",
            "--directory-name", name, "--file", str(source), "--report-output-dir", str(report_dir),
            "-o", output, "--log-level", "trace", "--log-file", str(work / (label + ".log")), *config]
        if mode:
            args.append(mode)
        if mode == "--execute":
            attempted_execution = True
        result = subprocess.run(args, cwd=work, stdin=subprocess.DEVNULL, capture_output=True, timeout=120, check=False)
        for data in (result.stdout, result.stderr, (work / (label + ".log")).read_bytes() if
            (work / (label + ".log")).exists() else b""):
            public(data)
        (work / (label + ".stdout")).write_bytes(result.stdout)
        (work / (label + ".stderr")).write_bytes(result.stderr)
        if failure:
            assert result.returncode != 0 and failure.lower() in (result.stdout + result.stderr).decode().lower(), label
        else:
            assert result.returncode == 0, label
        value = json.loads(result.stdout) if result.stdout.strip() and output == "json" else result.stdout.decode()
        new_reports = set(report_dir.glob("*.json")) - before_reports
        assert len(new_reports) <= 1, label
        capture_path = next(iter(new_reports)) if new_reports else report_dir / "unused.json"
        if isinstance(value, dict):
            assert value["report_output_file"] == str(capture_path), label
        return value, capture_path

    def report(path, source, dry_run):
        assert stat.S_IMODE(path.stat().st_mode) == 0o600
        value = json.loads(path.read_text())
        assert value["status"] == "captured" and value["directory_id"] == directory
        assert value["dry_run"] is dry_run
        assert value["request_sha256"] == hashlib.sha256(source.read_bytes()).hexdigest()
        return value["report"]

    def cleanup_import():
        text = io.StringIO()
        writer = csv.writer(text)
        writer.writerow(columns)
        writer.writerow(ceo)
        boundary = "kongctl-cleanup-" + secrets.token_hex(8)
        body = ("--" + boundary + '\r\nContent-Disposition: form-data; name="dry_run"\r\n\r\nfalse\r\n'
            "--" + boundary + '\r\nContent-Disposition: form-data; name="file"; filename="cleanup.csv"\r\n'
            "Content-Type: text/csv\r\n\r\n" + text.getvalue() + "\r\n--" + boundary + "--\r\n").encode()
        value = request("POST", root + "/org-chart/import", body, content_type="multipart/form-data; boundary=" + boundary)
        assert value["errors"] == []

    try:
        before_directory = request("GET", root)
        assert before_directory["name"] == name and before_directory["managed_by"] == {}
        assert collection("principals") == [] and collection("groups") == []
        outside = request("POST", root + "/principals", {"type": "user", "display_name": prefix + "-outside",
            "metadata": {"outside": prefix}}, expected=201)
        known.add(outside["id"])
        ownership = {"tool": "external-tool", "owner": "org-import-e2e"}
        manual = request("POST", root + "/principals", {"type": "user", "display_name": prefix + "-before",
            "managed_by": ownership, "metadata": {"retained": prefix}}, expected=201)
        known.add(manual["id"])
        request("POST", root + "/principals/" + manual["id"] + "/identities",
            {"type": "custom", "key": "kong:email", "value": worker[1]}, expected=201)
        original = write_csv("original", [ceo, manager, worker], costs)
        before = snapshot()
        preview, saved = invoke("preview", original)
        assert preview["status"] == "validated" and preview["dry_run"] is True and preview["rows"] == 3
        assert preview["counts"]["users"]["created"] == 2 and preview["counts"]["users"]["adopted"] == 1
        assert preview["counts"]["teams"]["created"] == 2 and preview["warning_count"] == 1
        assert preview["warnings"][0]["type"] == "principal_adopted"
        assert worker[1] in report(saved, original, True)["warnings"][0]["detail"]
        assert snapshot() == before
        old_bytes = saved.read_bytes()
        explicit, explicit_report = invoke("explicit-preview", original, explicit=True)
        assert {key: value for key, value in explicit.items() if key != "report_output_file"} == {
            key: value for key, value in preview.items() if key != "report_output_file"}
        assert report(explicit_report, original, True)["counts"] == report(saved, original, True)["counts"]
        assert snapshot() == before
        repeated_preview, new_report = invoke("repeated-preview", original, report_dir=saved.parent)
        assert repeated_preview["status"] == "validated" and new_report != saved
        assert report(new_report, original, True)["counts"] == report(saved, original, True)["counts"]
        assert saved.read_bytes() == old_bytes
        _, unused = invoke("missing-intent", original, mode=None, failure="choose exactly one")
        assert not unused.exists()
        bad = private / "invalid.csv"
        bad.write_bytes(b"header\n\xff\n")
        _, unused = invoke("invalid-encoding", bad, failure="UTF-8")
        assert not unused.exists()
        bad_schema = private / "schema.csv"
        bad_schema.write_text("email\n" + worker[1] + "\n")
        _, captured = invoke("bad-schema", bad_schema, failure="HTTP 400")
        assert report(captured, bad_schema, True)["status"] == 400
        assert snapshot() == before
        created, captured = invoke("execute-text", original, mode="--execute", output="text")
        assert "completed" in created and "principal_adopted" in created and "{" not in created
        assert report(captured, original, False)["counts"]["users"]["adopted"] == 1
        values = indexed()
        assert set(values) == {ceo[0], manager[0]}
        ids = {key: value["id"] for key, value in values.items()}
        adopted = request("GET", root + "/principals/" + manual["id"])
        assert adopted["managed_by"] == ownership and adopted["metadata"]["retained"] == prefix
        assert adopted["display_name"] == worker[2] + " " + worker[3] and adopted["provisioned_by"] == "imported"
        repeated, captured = invoke("repeat", original, mode="--execute", explicit=True)
        assert repeated["counts"]["users"]["created"] == 0 and repeated["counts"]["users"]["unchanged"] == 3
        assert {key: value["id"] for key, value in indexed().items()} == ids
        worker[4] = ceo[0]
        moved = write_csv("moved", [ceo, manager, worker])
        value, captured = invoke("manager-move-yaml", moved, mode="--execute", output="yaml")
        assert "completed" in value and "dry_run: false" in value
        assert report(captured, moved, False)["counts"]["teams"]["deleted"] == 1
        assert request("GET", root + "/principals/" + manual["id"])["metadata"]["cost_center"] == costs[2]
        cleared = write_csv("cleared", [ceo, manager, worker], [costs[0], costs[1], ""])
        value, captured = invoke("clear-cost", cleared, mode="--execute")
        assert value["counts"]["users"]["updated"] == 1
        assert "cost_center" not in request("GET", root + "/principals/" + manual["id"])["metadata"]
        departed = write_csv("departed", [ceo, worker])
        value, captured = invoke("departure", departed, mode="--execute")
        assert value["counts"]["users"]["inactive"] == 1
        assert request("GET", root + "/principals/" + ids[manager[0]])["active"] is False
        returned = write_csv("returned", [ceo, manager, worker])
        value, captured = invoke("return-inactive", returned, mode="--execute")
        assert any(w["type"] == "inactive_not_reactivated" for w in value["warnings"])
        assert request("GET", root + "/principals/" + ids[manager[0]])["active"] is False
        assert {key: value["id"] for key, value in indexed().items()} == ids
        duplicate = write_csv("duplicate", [ceo, worker, worker])
        before_error = snapshot()
        value, captured = invoke("row-errors", duplicate, mode="--execute", failure="row errors")
        assert value["status"] == "reported_errors" and value["error_count"] == 2
        assert {e["field"] for e in value["errors"]} == {"email", "external_id"}
        assert len(report(captured, duplicate, False)["errors"]) == 2 and snapshot() == before_error
        assert request("GET", root + "/principals/" + outside["id"]) == outside
        assert request("GET", root) == before_directory
    except Exception:
        raise RuntimeError("Org-chart import lifecycle failed during " + phase + "; private HR values withheld") from None
    finally:
        try:
            observed = indexed()
            known.update(value["id"] for value in observed.values())
            assert all(value["id"] in known for value in collection("principals"))
            if attempted_execution and collection("groups"):
                cleanup_import()
            assert collection("groups") == []
            for value in collection("principals"):
                assert value["id"] in known
                path = root + "/principals/" + value["id"]
                if value["provisioned_by"] == "imported":
                    request("PATCH", path, {"active": False})
                    assert request("GET", path)["provisioned_by"] == "konnect_user"
                request("DELETE", path, expected=204)
            assert collection("principals") == [] and collection("groups") == []
            shutil.rmtree(private)
            for path in work.iterdir():
                assert path.is_file()
                public(path.read_bytes())
                shutil.copyfile(path, artifacts / path.name)
        finally:
            scratch.cleanup()
    assert not work.exists()
    (artifacts / "org-import-privacy.json").write_text(json.dumps({"temporary_workspace": str(work),
        "removed": True, "private_sources_and_reports_copied": False}) + "\n")
    print(json.dumps({"dry_run_preserves_state": True, "complete_private_reports": True,
        "safe_counts_and_notices": True, "explicit_execution_and_adoption": True,
        "repeat_ids_and_manager_changes": True, "cost_omission_and_clearing": True,
        "departures_and_inactive_returns": True, "errors_and_local_guards": True,
        "private_workspace_removed": True, "fixtures_removed": True}))


if __name__ == "__main__":
    main()
