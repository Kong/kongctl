"""Read principal group projections; keep employee data out of artifacts."""
import base64
import csv
import io
import json
import os
from pathlib import Path
import re
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
    binary, directory, name, output = sys.argv[1:5]
    artifacts = Path(output)
    scratch = tempfile.TemporaryDirectory(prefix="kongctl-principal-groups-", dir=os.environ.get("TMPDIR"))
    private = Path(scratch.name)
    target = os.environ.get("KONGCTL_E2E_KONNECT_ENV", "com").strip() or "com"
    assert target in ("com", "tech")
    base = os.environ.get("KONGCTL_E2E_KONNECT_BASE_URL", "").strip() or "https://us.api.konghq." + target
    host = urllib.parse.urlparse(base)
    assert host.scheme == "https" and host.hostname.endswith((".api.konghq.com", ".api.konghq.tech"))
    token = os.environ["KONGCTL_E2E_KONNECT_PAT"]
    needles = (token.encode(), base64.b64encode(token.encode()))
    opener = urllib.request.build_opener(NoRedirect())
    root = "/v2/directories/" + directory
    config = ["--config-file", str(Path(os.environ["XDG_CONFIG_HOME"]) / "kongctl/config.yaml")]
    prefix = secrets.token_hex(12)
    columns = ["external_id", "email", "given_name", "family_name", "manager_external_id"]
    rows = [[prefix + "-root", prefix + "-root@example.test", prefix + "-root", "leader", ""],
            [prefix + "-manager", prefix + "-manager@example.test", prefix + "-manager", "lead", prefix + "-root"],
            [prefix + "-worker", prefix + "-worker@example.test", prefix + "-worker", "worker", prefix + "-manager"]]
    hidden = {value.encode() for row in rows for value in row if value}
    known, ordinary = set(), set()
    phase, import_started = "setup", False
    traces = {}

    def request(method, path, body=None, expected=(200, 201, 204), content_type="application/json"):
        req = urllib.request.Request(base.rstrip("/") + path, method=method,
            headers={"Authorization": "Bearer " + token, "Content-Type": content_type},
            data=body if isinstance(body, bytes) else None if body is None else json.dumps(body).encode())
        try:
            response = opener.open(req, timeout=30)
        except urllib.error.HTTPError as error:
            response = error
        with response:
            raw = response.read()
            assert response.status in expected
            return json.loads(raw) if raw and response.status != 404 else None

    def collection(suffix):
        value = request("GET", root + "/" + suffix)
        assert isinstance(value["data"], list) and value["meta"]["page"]["next"] in (None, "")
        return value["data"]

    def upload(values):
        stream = io.StringIO()
        writer = csv.writer(stream)
        writer.writerow(columns)
        writer.writerows(values)
        boundary = "group-inspection-" + secrets.token_hex(8)
        body = ("--" + boundary + '\r\nContent-Disposition: form-data; name="dry_run"\r\n\r\nfalse\r\n'
            "--" + boundary + '\r\nContent-Disposition: form-data; name="file"; filename="fixture.csv"\r\n'
            "Content-Type: text/csv\r\n\r\n" + stream.getvalue() + "\r\n--" + boundary + "--\r\n").encode()
        report = request("POST", root + "/org-chart/import", body, content_type="multipart/form-data; boundary=" + boundary)
        assert report["errors"] == []

    def imported():
        result = {}
        for person in collection("principals"):
            if person["id"] in known:
                continue
            for identity in collection("principals/" + person["id"] + "/identities"):
                if identity.get("key") == "kong:external_id":
                    assert identity["value"].startswith(prefix + "-")
                    result[identity["value"]] = person
        return result

    def invoke(label, args, output="json", failure=None):
        nonlocal phase
        phase = label
        log = private / (label + ".log")
        result = subprocess.run([binary, *args, "-o", output, "--log-level", "trace", "--log-file", str(log), *config],
            capture_output=True, timeout=120, check=False, stdin=subprocess.DEVNULL)
        text = log.read_text() if log.exists() else ""
        for data in (result.stdout, result.stderr, text.encode()):
            assert not any(needle in data for needle in needles)
        requests = [line for line in text.splitlines() if "log_type=http_request " in line]
        assert all("method=GET " in line and "host=" + host.hostname in line for line in requests)
        projection = [line for line in requests if re.search(r"route=" + re.escape(root) + r"/principals/[^/ ]+/groups ", line)]
        traces[label] = {"projection_pages": len(projection),
            "bracketed_size_one": all("page[size]:1" in line for line in projection),
            "cursor_pages": sum("page[after]:" in line for line in projection), "get_only": True,
            "expected_host_only": True}
        if failure:
            assert result.returncode != 0 and failure.lower() in (result.stdout + result.stderr).decode().lower()
            return None
        assert result.returncode == 0
        return json.loads(result.stdout) if output == "json" else result.stdout.decode()

    def args(principal, selected=None, explicit=False, directory_name=False, verb="get"):
        return [verb, *(["konnect"] if explicit else []), "identity", "principal-groups",
            *([selected] if selected else []), "--directory-name" if directory_name else "--directory-id",
            name if directory_name else directory, "--principal-id", principal, "--page-size", "1"]

    original = request("GET", root)
    assert original["name"] == name and collection("principals") == [] and collection("groups") == []
    try:
        person = request("POST", root + "/principals", {"display_name": "Group inspection subject", "type": "user"})
        known.add(person["id"])
        outsider = request("POST", root + "/principals", {"display_name": "Unassociated subject", "type": "agent"})
        known.add(outsider["id"])
        membership_ids = set()
        for group_name in ("inspection-operators", "inspection-auditors"):
            group = request("POST", root + "/groups", {"name": group_name, "type": "group"})
            ordinary.add(group["id"])
            entry = request("POST", root + "/groups/" + group["id"] + "/membership",
                {"member": {"type": "principal", "id": person["id"]}})
            membership_ids.add(entry["id"])
        before = collection("groups"), request("GET", root + "/principals/" + person["id"])
        collection("principals/" + person["id"] + "/groups")
        observed = invoke("ordinary-json", args(person["id"], directory_name=True))
        assert len(observed) == 2 and {v["id"] for v in observed} == ordinary
        assert all(v["direct"] and v["membership_id"] in membership_ids for v in observed)
        assert not (ordinary & membership_ids)
        selected = observed[0]
        assert invoke("ordinary-id", args(person["id"], selected["id"])) == selected
        assert invoke("ordinary-name", args(person["id"], selected["name"], explicit=True)) == selected
        yaml = invoke("ordinary-yaml", args(person["id"], explicit=True, verb="list"), output="yaml")
        assert yaml.count("direct: true") == 2 and "membership_id:" in yaml
        text = invoke("ordinary-text", args(person["id"]), output="text")
        assert "GROUP_ID" in text and "DIRECT" in text and "MEMBERSHIP_ID" in text and "{" not in text
        assert invoke("empty", args(outsider["id"])) == []
        invoke("wrong-principal", args(outsider["id"], selected["id"]), failure="not found under the selected principal")
        invoke("entry-id-is-not-group-id", args(person["id"], selected["membership_id"]), failure="not found")
        invoke("missing", args("5078f559-063b-4f10-9098-9e6553b21d9b"), failure="404")
        assert (collection("groups"), request("GET", root + "/principals/" + person["id"])) == before
        assert all(value["managed_by"] == {} for value in collection("groups"))
        for group_id in list(ordinary):
            for entry in collection("groups/" + group_id + "/membership"):
                assert entry["id"] in membership_ids
                request("DELETE", root + "/groups/" + group_id + "/membership/" + entry["id"])
            request("DELETE", root + "/groups/" + group_id)
            ordinary.remove(group_id)
        request("DELETE", root + "/principals/" + person["id"])
        known.remove(person["id"])
        phase, import_started = "import hierarchy", True
        upload(rows)
        people = imported()
        assert len(people) == 3
        known.update(v["id"] for v in people.values())
        worker = people[rows[2][0]]
        native = collection("principals/" + worker["id"] + "/groups")
        before = collection("groups"), collection("principals")
        observed = invoke("hierarchy-json", args(worker["id"], directory_name=True, verb="list"))
        assert len(observed) == 2 and {v["direct"] for v in observed} == {True, False}
        by_id = {v["id"]: v for v in native}
        for value in observed:
            assert value["type"] == "team" and value["provisioned_by"] == "imported"
            assert value["team_manager"] == by_id[value["id"]]["team_manager"]
            assert (value["membership_id"] is not None) == value["direct"]
        inherited = next(v for v in observed if not v["direct"])
        assert invoke("inherited-detail", args(worker["id"], inherited["id"])) == inherited
        assert invoke("inherited-name", args(worker["id"], inherited["name"])) == inherited
        detail = invoke("inherited-text", args(worker["id"], inherited["id"]), output="text")
        assert "direct:" in detail and "false" in detail and "membership_id:" in detail
        assert "team_manager.id:" in detail and "team_manager.display_name:" in detail and "team_manager.email:" in detail
        yaml = invoke("hierarchy-yaml", args(worker["id"]), output="yaml")
        assert "direct: false" in yaml and "direct: true" in yaml and "display_name:" in yaml
        assert (collection("groups"), collection("principals")) == before
        assert all(value["managed_by"] == {} for value in collection("groups"))
        for label in ("ordinary-json", "hierarchy-json"):
            assert traces[label]["projection_pages"] == 2 and traces[label]["cursor_pages"] == 1
            assert traces[label]["bracketed_size_one"]
        assert request("GET", root) == original
    except Exception:
        raise RuntimeError("Principal-group inspection failed during " + phase + "; private values withheld") from None
    finally:
        try:
            discovered = imported() if import_started else {}
            known.update(v["id"] for v in discovered.values())
            assert all(v["id"] in known for v in collection("principals"))
            if import_started:
                upload(rows[:1])
                assert collection("groups") == []
            else:
                for group_id in ordinary:
                    for entry in collection("groups/" + group_id + "/membership"):
                        assert entry["member"]["id"] in known
                        request("DELETE", root + "/groups/" + group_id + "/membership/" + entry["id"])
                    request("DELETE", root + "/groups/" + group_id)
            for person in collection("principals"):
                assert person["id"] in known
                if person["provisioned_by"] == "imported":
                    request("PATCH", root + "/principals/" + person["id"], {"active": False})
                request("DELETE", root + "/principals/" + person["id"])
            assert collection("principals") == [] and collection("groups") == []
            assert request("GET", root) == original
        finally:
            scratch.cleanup()
    assert not private.exists()
    receipt = {"path": str(private), "removed": True, "private_sources_and_captures_copied": False,
        "private_hr_sources_and_outputs_copied": False, "import_reports_copied": False}
    for value in (json.dumps(traces).encode(), json.dumps(receipt).encode()):
        assert not any(needle in value for needle in needles) and not any(value_part in value for value_part in hidden)
    (artifacts / "public-page-traces.json").write_text(json.dumps(traces, indent=2) + "\n")
    (artifacts / "private-workspace.json").write_text(json.dumps(receipt) + "\n")
    return {"ordinary_direct_groups": True, "imported_direct_and_inherited": True,
        "group_and_entry_ids_distinct": True, "manager_and_provisioning_fields": True,
        "complete_two_page_sdk_reads": True, "scope_and_missing_guards": True,
        "reads_preserve_state_and_ownership": True, "output_formats": 3,
        "private_sources_and_outputs_not_copied": True, "private_workspace_removed": True, "fixtures_removed": True}


if __name__ == "__main__":
    try:
        print(json.dumps(main()))
    except Exception as error:
        print(str(error) if isinstance(error, RuntimeError) else "Principal-group scenario or cleanup failed", file=sys.stderr)
        sys.exit(1)
