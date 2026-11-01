"""Use the native importer to verify the ordinary declarative ownership boundary."""

import csv
import io
import json
import os
from pathlib import Path
import secrets
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
    scratch = tempfile.TemporaryDirectory(prefix="kongctl-org-chart-", dir=os.environ.get("TMPDIR"))
    work = Path(scratch.name)
    target = os.environ.get("KONGCTL_E2E_KONNECT_ENV", "com").strip() or "com"
    assert target in ("com", "tech")
    base = os.environ.get("KONGCTL_E2E_KONNECT_BASE_URL", "").strip() or "https://us.api.konghq." + target
    host = urllib.parse.urlsplit(base)
    assert host.scheme == "https" and host.hostname.endswith((".api.konghq.com", ".api.konghq.tech"))
    token = os.environ["KONGCTL_E2E_KONNECT_PAT"]
    opener = urllib.request.build_opener(NoRedirect())
    headers = {"Authorization": "Bearer " + token, "Content-Type": "application/json"}
    config = ["--config-file", str(Path(os.environ["XDG_CONFIG_HOME"]) / "kongctl/config.yaml")]
    root = "/v2/directories/" + directory
    namespace, phase = "identity-import-boundary", "setup"
    prefix = secrets.token_hex(12)
    columns = ["external_id", "email", "given_name", "family_name", "manager_external_id"]
    ceo = [prefix + "-root", prefix + "-root@example.test", "Synthetic", "Executive", ""]
    employee = [prefix + "-employee", prefix + "-employee@example.test", "Synthetic", "Employee", ceo[0]]
    known, imported, principal_id = set(), False, None

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

    def native_import(rows, dry_run=False, costs=None):
        text = io.StringIO()
        writer = csv.writer(text)
        writer.writerow(columns + ([] if costs is None else ["cost_center"]))
        writer.writerows(row + ([] if costs is None else [costs[i]]) for i, row in enumerate(rows))
        boundary = "kongctl-org-chart-" + secrets.token_hex(8)
        body = ("--" + boundary + '\r\nContent-Disposition: form-data; name="dry_run"\r\n\r\n' +
            str(dry_run).lower() + '\r\n'
            "--" + boundary + '\r\nContent-Disposition: form-data; name="file"; filename="fixture.csv"\r\n'
            "Content-Type: text/csv\r\n\r\n" + text.getvalue() + "\r\n--" + boundary + "--\r\n").encode()
        value = request("POST", root + "/org-chart/import", body,
            content_type="multipart/form-data; boundary=" + boundary)
        assert value["errors"] == []
        return value

    def invoke(label, args, failure=False):
        nonlocal phase
        phase = label
        formatting = [] if args[0] in ("plan", "dump") else ["-o", "json"]
        result = subprocess.run([binary, *args, *formatting, "--log-level", "trace", "--log-file",
            str(work / (label + ".log")), *config], cwd=work, stdin=subprocess.DEVNULL,
            capture_output=True, timeout=120, check=False)
        # Command output and trace bodies can contain synthetic HR values. Keep
        # them only in the private temporary workspace, never harness artifacts.
        (work / (label + ".stdout")).write_bytes(result.stdout)
        (work / (label + ".stderr")).write_bytes(result.stderr)
        if failure:
            assert result.returncode != 0 and b"organization-chart importer" in result.stdout + result.stderr, label
        else:
            assert result.returncode == 0, label
        return result.stdout

    def manifest(path, display_name="Pre-import user", empty=False):
        value = {"_defaults": {"kongctl": {"namespace": namespace}}, "identity": {"directories": [{
            "ref": "directory", "_external": {"id": directory}, "principals": [] if empty else [{
                "ref": "employee", "display_name": display_name, "type": "user",
                "metadata": {"source": "before-import"}, "labels": {"team": "test"}}]}]}}
        path.write_text(json.dumps(value))
        return str(path)

    def snapshot():
        values = collection("principals")
        return {"principals": values, "groups": collection("groups"), "identities": {
            value["id"]: collection("principals/" + value["id"] + "/identities") for value in values}}

    try:
        before_directory = request("GET", root)
        assert before_directory["name"] == name and before_directory["managed_by"] == {}
        assert collection("principals") == [] and collection("groups") == []
        example = Path(__file__).resolve().parents[5] / "docs/examples/declarative/identity/org-chart.csv"
        with example.open(newline="") as source:
            rows = list(csv.DictReader(source))
        preview = native_import([[row[field] for field in columns] for row in rows],
            dry_run=True, costs=[row["cost_center"] for row in rows])
        assert preview["counts"]["users"]["created"] == 3 and preview["counts"]["teams"]["created"] == 2
        assert collection("principals") == [] and collection("groups") == []
        initial = manifest(work / "initial.yaml")
        invoke("create-managed-user", ["apply", "-f", initial, "--auto-approve"])
        values = collection("principals")
        assert len(values) == 1 and values[0]["managed_by"]["kongctl-ref"] == "employee"
        before = values[0]
        principal_id = before["id"]
        known.add(principal_id)
        update = manifest(work / "update.yaml", "Changed by saved plan")
        saved_update, saved_delete = work / "update-plan.json", work / "delete-plan.json"
        invoke("save-update", ["plan", "--mode", "apply", "-f", update, "--output-file", str(saved_update)])
        invoke("save-delete", ["plan", "--mode", "delete", "-f", initial, "--output-file", str(saved_delete)])
        assert len(json.loads(saved_update.read_text())["changes"]) == 1
        assert len(json.loads(saved_delete.read_text())["changes"]) == 1
        request("POST", root + "/principals/" + principal_id + "/identities",
            {"type": "custom", "key": "kong:email", "value": employee[1]}, expected=201)
        report = native_import([ceo, employee])
        imported = True
        assert report["counts"]["users"]["adopted"] == 1
        values = collection("principals")
        assert len(values) == 2
        for value in values:
            if value["id"] != principal_id:
                identities = collection("principals/" + value["id"] + "/identities")
                assert any(v.get("key") == "kong:external_id" and v.get("value") == ceo[0] for v in identities)
                known.add(value["id"])
        current = request("GET", root + "/principals/" + principal_id)
        assert current["managed_by"] == before["managed_by"] and current["provisioned_by"] == "imported"
        state_before = snapshot()
        for label, args in (
            ("fresh-apply", ["plan", "--mode", "apply", "-f", initial]),
            ("fresh-delete", ["plan", "--mode", "delete", "-f", initial]),
            ("scoped-prune", ["plan", "--mode", "sync", "-f", manifest(work / "empty.yaml", empty=True)]),
            ("saved-update", ["apply", "--plan", str(saved_update), "--auto-approve"]),
            ("saved-delete", ["delete", "--plan", str(saved_delete), "--auto-approve"]),
            ("adoption", ["adopt", "identity", "principal", principal_id, "--directory-id", directory,
                "--ref", "employee", "--namespace", "another-namespace", "--overwrite-namespace"]),
        ):
            invoke(label, args, failure=True)
        no_op = manifest(work / "same.yaml", current["display_name"])
        exact = json.loads(Path(no_op).read_text())
        exact["identity"]["directories"][0]["principals"][0].update({
            field: current[field] for field in ("description", "metadata", "labels", "active")})
        Path(no_op).write_text(json.dumps(exact))
        invoke("same-fields-blocked", ["plan", "--mode", "apply", "-f", no_op], failure=True)
        inspected = json.loads(invoke("inspect-users", ["get", "identity", "principals", "--directory-id", directory]))
        assert len(inspected) == 2 and all(v["provisioned_by"] == "imported" for v in inspected)
        groups = json.loads(invoke("inspect-teams", ["get", "identity", "groups", "--directory-id", directory]))
        assert len(groups) == 1 and groups[0]["type"] == "team"
        dump, replan = work / "dump.yaml", work / "replan.json"
        invoke("dump", ["dump", "declarative", "--resources", "identity.directories",
            "--include-child-resources", "--output-file", str(dump)])
        invoke("replan", ["plan", "--mode", "apply", "-f", str(dump), "--output-file", str(replan)])
        assert json.loads(replan.read_text())["changes"] == []
        assert snapshot() == state_before
        assert request("GET", root) == before_directory
    except Exception:
        raise RuntimeError("Importer boundary verification failed during " + phase + "; private HR data withheld") from None
    finally:
        try:
            if imported:
                native_import([ceo])
            assert collection("groups") == []
            values = collection("principals")
            assert all(value["id"] in known for value in values)
            for value in values:
                path = root + "/principals/" + value["id"]
                if value["provisioned_by"] == "imported":
                    request("PATCH", path, {"active": False})
                    fresh = request("GET", path)
                    assert fresh["active"] is False and fresh["provisioned_by"] == "konnect_user"
                request("DELETE", path, expected=204)
            assert collection("principals") == [] and collection("groups") == []
        finally:
            scratch.cleanup()
    assert not work.exists()
    (Path(location) / "org-chart-privacy.json").write_text(json.dumps({
        "temporary_workspace": str(work), "removed": True, "private_hr_files_copied": False,
        "published_example_dry_run_verified": True}) + "\n")
    print(json.dumps({"native_adoption_retains_metadata": True, "imported_provisioning_observed": True,
        "fresh_management_blocked": True, "saved_writes_blocked": True, "ordinary_adoption_blocked": True,
        "metadata_reads_work": True, "dump_replans_external": True, "blocked_operations_preserve_state": True,
        "private_workspace_removed": True, "fixtures_removed": True}))


if __name__ == "__main__":
    main()
