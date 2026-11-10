"""Generated key lifecycle beneath an importer-controlled external principal."""
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
import urllib.request


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None


def main():
    binary, directory, directory_name = sys.argv[1:4]
    target = os.environ.get("KONGCTL_E2E_KONNECT_ENV", "com").strip() or "com"
    assert target in ("com", "tech")
    base = os.environ.get("KONGCTL_E2E_KONNECT_BASE_URL", "").strip() or "https://us.api.konghq." + target
    token = os.environ["KONGCTL_E2E_KONNECT_PAT"]
    opener = urllib.request.build_opener(NoRedirect())
    root = "/v2/directories/" + directory
    namespace, prefix = "external-principal-keys", secrets.token_hex(8)
    principal, phase = None, "setup"

    def request(method, path, body=None, content_type="application/json", expected=200):
        req = urllib.request.Request(base.rstrip("/") + path, method=method,
            headers={"Authorization": "Bearer " + token, "Content-Type": content_type},
            data=body if isinstance(body, bytes) else None if body is None else json.dumps(body).encode())
        try:
            response = opener.open(req, timeout=30)
        except urllib.error.HTTPError as error:
            response = error
        with response:
            raw = response.read()
            assert response.status == expected, (method, response.status)
            return json.loads(raw) if raw else None

    def collection(path):
        value = request("GET", path)
        assert not value.get("meta", {}).get("page", {}).get("next")
        return value["data"]

    text = io.StringIO()
    writer = csv.writer(text)
    writer.writerow(["external_id", "email", "given_name", "family_name", "manager_external_id", "cost_center"])
    writer.writerow([prefix, prefix + "@example.test", "Synthetic", "Employee", "", "SUPPORT"])

    with tempfile.TemporaryDirectory(prefix="kongctl-external-keys-", dir=os.environ.get("TMPDIR")) as temporary:
        work = Path(temporary)
        roster = work / "roster.csv"
        roster.write_text(text.getvalue())

        def invoke(label, args, failure=None, raw=False):
            nonlocal phase
            phase = label
            result = subprocess.run([binary, *args, "--log-level", "error"], cwd=work,
                capture_output=True, stdin=subprocess.DEVNULL, timeout=120, check=False)
            (work / (label + ".stdout")).write_bytes(result.stdout)
            (work / (label + ".stderr")).write_bytes(result.stderr)
            if failure:
                assert result.returncode != 0 and failure.encode() in result.stdout + result.stderr, label
                return None
            assert result.returncode == 0, label
            return result.stdout if raw else json.loads(result.stdout)

        def import_roster(label, dry_run=False):
            report = invoke(label, ["import", "id", "org-chart", "--directory-id", directory,
                "--file", str(roster), "--report-output-dir", str(work / "reports"),
                "--dry-run" if dry_run else "--execute", "-o", "json"])
            assert report["errors"] == [] and report["dry_run"] is dry_run
            saved = Path(report["report_output_file"])
            assert saved.is_relative_to(work / "reports") and saved.stat().st_mode & 0o777 == 0o600
            return report

        def declaration(keys):
            parent = {"ref": "employee", "_external": {"id": principal}}
            if keys is not None:
                parent["api_keys"] = keys
            return {"_defaults": {"kongctl": {"namespace": namespace}}, "identity": {"directories": [{
                "ref": "directory", "_external": {"selector": {"matchFields": {"name": directory_name}}},
                "principals": [parent]}]}}

        def plan(label, keys, mode="apply"):
            source = work / (label + ".yaml")
            source.write_text(json.dumps(declaration(keys)))
            value = invoke(label, ["plan", "--mode", mode, "-f", str(source)])
            (work / (label + ".json")).write_text(json.dumps(value))
            return value

        def apply(label, saved, verb="apply", capture=False):
            args = [verb, "--plan", str(work / (saved + ".json")), "--auto-approve", "-o", "json"]
            if capture:
                args.extend(["--secret-output-dir", str(work / (label + "-secrets"))])
            return invoke(label, args)

        try:
            assert collection(root + "/principals") == [] and collection(root + "/groups") == []
            assert import_roster("preview-import", dry_run=True)["counts"]["users"]["created"] == 1
            assert collection(root + "/principals") == []
            assert import_roster("execute-import")["counts"]["users"]["created"] == 1
            principals = invoke("read-principals", ["get", "id", "principals",
                "--directory-id", directory, "-o", "json"])
            assert len(principals) == 1
            principal = principals[0]["id"]
            parent_path = root + "/principals/" + principal
            key_path = parent_path + "/api-keys"
            before = request("GET", parent_path)
            assert before["provisioned_by"] == "imported" and before["managed_by"] == {}
            assert principals[0]["type"] == "user" and principals[0]["active"] is True
            assert principals[0]["display_name"] == "Synthetic Employee"
            assert principals[0]["metadata"]["cost_center"] == "SUPPORT"
            assert principals[0]["managed_by"] == {} and principals[0]["provisioned_by"] == "imported"
            detail = invoke("read-principal-by-id", ["get", "id", "principal", principal,
                "--directory-id", directory, "-o", "json"])
            assert detail["id"] == principal and detail["provisioned_by"] == "imported"
            assert detail["managed_by"] == {} and detail["display_name"] == "Synthetic Employee"
            links = invoke("read-identities", ["get", "id", "principal", "identities",
                "--directory-id", directory, "--principal-id", principal, "-o", "json"])
            custom = {value["key"]: value for value in links if value["type"] == "custom"}
            assert custom["kong:email"]["value"] == prefix + "@example.test"
            assert custom["kong:external_id"]["value"] == prefix
            markers = [custom[key] for key in ("kong:org_chart_root", "kong:org_root") if key in custom]
            assert len(markers) == 1 and markers[0]["value"] == principal
            assert all(value["managed_by"] == {} for value in custom.values())
            key = {"ref": "primary", "type": "v1", "name": "Application key", "expires_at": "2030-01-01T00:00:00Z"}
            planned = plan("create", [key])
            assert planned["summary"]["total_changes"] == 1
            assert planned["changes"][0]["resource_type"] == "identity_principal_api_key"
            assert planned["changes"][0]["parent"]["id"] == principal
            assert planned["changes"][0].get("protecting_parent") is None
            created = apply("create-key", "create", capture=True)
            assert created["summary"]["applied"] == 1
            captures = list((work / "create-key-secrets").glob("*.json"))
            assert len(captures) == 1 and captures[0].stat().st_mode & 0o777 == 0o600
            captured = json.loads(captures[0].read_text())
            assert captured["secret"] and captured["status"] == "captured"
            current = collection(key_path)[0]
            assert len(current["managed_by"]) == 5 and current["managed_by"]["kongctl-namespace"] == namespace
            public_keys = invoke("read-generated-key", ["get", "id", "principal", "api-keys",
                "--directory-id", directory, "--principal-id", principal, "-o", "json"])
            assert len(public_keys) == 1 and public_keys[0]["id"] == current["id"]
            assert public_keys[0]["managed_by"] == current["managed_by"]
            assert captured["secret"] not in (work / "read-generated-key.stdout").read_text()
            assert plan("no-op", [key])["summary"]["total_changes"] == 0
            assert import_roster("repeat-import")["counts"]["users"]["unchanged"] == 1
            assert collection(key_path)[0] == current
            # The importer currently recreates its org-root marker identity.
            # Subsequent key-only operations must preserve this fresh snapshot.
            identities = request("GET", parent_path + "/identities")
            changed = {**key, "name": "Renamed application key", "expires_at": None}
            assert plan("update", [changed])["summary"]["total_changes"] == 1
            assert b"Renamed application key" in invoke("diff", ["diff", "--plan", str(work / "update.json")], raw=True)
            apply("update-key", "update")
            assert plan("updated-no-op", [changed])["summary"]["total_changes"] == 0
            duplicate = request("POST", key_path, {"type": "v1", "managed_by": current["managed_by"]}, expected=201)
            source = work / "duplicate.yaml"
            source.write_text(json.dumps(declaration([changed])))
            invoke("duplicate", ["plan", "-f", str(source)], failure="multiple API keys")
            request("DELETE", key_path + "/" + duplicate["id"], expected=204)
            dumped = work / "dump.yaml"
            invoke("dump", ["dump", "declarative", "--resources", "identity.directories", "--filter-id", directory,
                "--include-child-resources", "--output-file", str(dumped)], raw=True)
            assert "managed_by:" not in dumped.read_text() and captured["secret"] not in dumped.read_text()
            assert invoke("dump-plan", ["plan", "--mode", "apply", "-f", str(dumped)])["summary"]["total_changes"] == 0
            foreign = {**current["managed_by"], "kongctl-namespace": "another-team"}
            request("POST", key_path, {"type": "v1", "managed_by": foreign}, expected=201)
            request("POST", key_path, {"type": "v1"}, expected=201)
            assert plan("omitted", None, "sync")["summary"]["total_changes"] == 0
            assert plan("empty", [], "sync")["summary"]["total_changes"] == 1
            apply("sync-key", "empty", verb="sync")
            assert len(collection(key_path)) == 2
            assert plan("recreate", [key])["summary"]["total_changes"] == 1
            apply("recreate-key", "recreate", capture=True)
            assert plan("delete", [key], "delete")["summary"]["total_changes"] == 1
            apply("delete-key", "delete", verb="delete")
            assert len(collection(key_path)) == 2
            assert request("GET", parent_path) == before
            assert request("GET", parent_path + "/identities") == identities
        except Exception:
            raise RuntimeError("External principal key verification failed during " + phase + "; private data withheld") from None
        finally:
            if principal:
                parent_path = root + "/principals/" + principal
                for value in collection(parent_path + "/api-keys"):
                    request("DELETE", parent_path + "/api-keys/" + value["id"], expected=204)
                request("PATCH", parent_path, {"active": False})
                for value in collection(parent_path + "/identities"):
                    request("DELETE", parent_path + "/identities/" + value["id"], expected=204)
                request("DELETE", parent_path, expected=204)
        assert collection(root + "/principals") == [] and collection(root + "/groups") == []
    print(json.dumps({key: True for key in ("cli_import_preview_execute", "cli_principal_and_identity_reads",
        "cli_api_key_reads", "independent_generated_keys", "repeat_import_preserves_keys",
        "updates_and_no_drift", "duplicate_conflicts", "scoped_sync_and_explicit_delete", "dump_reload",
        "principal_unchanged", "fixtures_removed")}))


if __name__ == "__main__":
    main()
