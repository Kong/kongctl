"""Verify Identity retirement against independent Registry references."""
import base64
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
    def redirect_request(self, *args):
        return None


def main():
    binary, name, output = sys.argv[1:4]
    target = os.environ.get("KONGCTL_E2E_KONNECT_ENV", "com").strip() or "com"
    assert target in ("com", "tech")
    base = os.environ.get("KONGCTL_E2E_KONNECT_BASE_URL", "").strip() or "https://us.api.konghq." + target
    host = urllib.parse.urlparse(base)
    assert host.scheme == "https" and host.hostname.endswith((".api.konghq.com", ".api.konghq.tech"))
    token = os.environ["KONGCTL_E2E_KONNECT_PAT"]
    needles = (token.encode(), base64.b64encode(token.encode()))
    opener = urllib.request.build_opener(NoRedirect())
    config = ["--config-file", str(Path(os.environ["XDG_CONFIG_HOME"]) / "kongctl/config.yaml")]
    scratch = tempfile.TemporaryDirectory(prefix="kongctl-registry-guard-", dir=os.environ.get("TMPDIR"))
    private = Path(scratch.name)
    directory = person = link = None
    agents, traces = [], {}
    phase = "setup"

    def request(method, path, body=None, expected=(200, 201, 204)):
        req = urllib.request.Request(base.rstrip("/") + path, method=method,
            headers={"Authorization": "Bearer " + token, "Content-Type": "application/json"},
            data=None if body is None else json.dumps(body).encode())
        try:
            response = opener.open(req, timeout=30)
        except urllib.error.HTTPError as error:
            response = error
        with response:
            raw = response.read()
            assert response.status in expected
            return json.loads(raw) if raw and response.status != 404 else None

    def registry_agents():
        values = []
        number = 1
        while True:
            body = request("GET", "/v1/agents?page[size]=100&page[number]=" + str(number))
            assert body["meta"]["page"]["number"] == number
            values.extend(body["data"])
            if not body["data"]:
                assert len(values) == body["meta"]["page"]["total"]
                return {value["id"] for value in values}
            number += 1

    def invoke(label, args, failure=False):
        nonlocal phase
        phase = label
        log = private / (label + ".log")
        output_args = [] if args[0] == "plan" else ["-o", "json"]
        result = subprocess.run([binary, *args, *output_args, "--log-level", "trace", "--log-file", str(log), *config],
            capture_output=True, timeout=120, stdin=subprocess.DEVNULL)
        text = log.read_text() if log.exists() else ""
        for value in (result.stdout, result.stderr, text.encode()):
            assert not any(needle in value for needle in needles)
        requests = [line for line in text.splitlines() if "log_type=http_request " in line]
        assert all("host=" + host.hostname in line for line in requests)
        registry = [line for line in requests if "route=/v1/agents" in line]
        assert all("method=GET " in line for line in registry)
        traces[label] = {"registry_gets": len(registry), "configured_host_only": True, "registry_get_only": True}
        if failure:
            assert result.returncode != 0
            assert "Registry Agent" in (result.stdout + result.stderr).decode()
            assert registry
            return None
        assert result.returncode == 0
        if "--output-file" in args:
            return json.loads(Path(args[args.index("--output-file") + 1]).read_text())
        return json.loads(result.stdout)

    def manifest(filename, document):
        path = private / filename
        path.write_text(json.dumps(document))
        return str(path)

    namespace = "identity-registry-guard-e2e"
    defaults = {"kongctl": {"namespace": namespace}}
    initial = manifest("initial.yaml", {"_defaults": defaults, "identity": {"directories": [{
        "ref": "registry-directory", "name": name, "allow_all_control_planes": True, "vault": {"enabled": True},
        "principals": [{"ref": "registry-principal", "display_name": "Registry retirement subject", "type": "agent"}],
    }]}})
    root_only = manifest("directory.yaml", {"_defaults": defaults, "identity": {"directories": [{
        "ref": "registry-directory", "name": name, "vault": {"enabled": True},
    }]}})
    principal_doc = {"_defaults": defaults, "identity": {"directories": [{
        "ref": "registry-directory", "_external": {"selector": {"matchFields": {"name": name}}},
        "principals": [{"ref": "registry-principal", "display_name": "Registry retirement subject", "type": "agent"}],
    }]}}
    principal_file = manifest("principal.yaml", principal_doc)
    sync_doc = json.loads(json.dumps(principal_doc))
    sync_doc["identity"]["directories"][0]["principals"] = []
    empty = manifest("empty-principals.yaml", sync_doc)
    principal_plan, directory_plan = str(private / "principal-delete.json"), str(private / "directory-delete.json")
    original_agents = registry_agents()
    try:
        assert invoke("create", ["apply", "-f", initial, "--auto-approve"])["summary"]["applied"] == 2
        directories = request("GET", "/v2/directories")["data"]
        matches = [value for value in directories if value["name"] == name]
        assert len(matches) == 1
        directory = matches[0]["id"]
        root = "/v2/directories/" + directory
        people = request("GET", root + "/principals")["data"]
        assert len(people) == 1 and people[0]["managed_by"]["kongctl-ref"] == "registry-principal"
        person = people[0]["id"]
        assert matches[0]["managed_by"]["kongctl-namespace"] == namespace
        assert invoke("save-principal-delete", ["plan", "--mode", "delete", "-f", principal_file,
            "--output-file", principal_plan])["summary"]["total_changes"] == 1
        assert invoke("save-directory-delete", ["plan", "--mode", "delete", "-f", root_only,
            "--output-file", directory_plan])["summary"]["total_changes"] == 1
        for _ in range(3):
            created = request("POST", "/v1/agents?create_version=false", {
                "slug": "identity-reference-" + secrets.token_hex(12), "display_name": "Independent Registry Agent"})
            agents.append(created["id"])
        route = "/v1/agents/" + agents[-1] + "/principals"
        link = request("POST", route, {"directory_id": directory, "principal_id": person})["id"]
        invoke("principal-delete-plan-blocked", ["plan", "--mode", "delete", "-f", principal_file], failure=True)
        invoke("principal-sync-plan-blocked", ["plan", "--mode", "sync", "-f", empty], failure=True)
        invoke("principal-sync-blocked", ["sync", "-f", empty, "--auto-approve"], failure=True)
        invoke("saved-principal-delete-blocked", ["delete", "--plan", principal_plan, "--auto-approve"], failure=True)
        invoke("saved-directory-delete-blocked", ["delete", "--plan", directory_plan, "--auto-approve"], failure=True)
        assert request("GET", root + "/principals/" + person)["id"] == person
        principal_doc["identity"]["directories"][0]["principals"][0]["display_name"] = "Updated retirement subject"
        manifest("principal.yaml", principal_doc)
        assert invoke("linked-principal-update", ["apply", "-f", principal_file, "--auto-approve"])["summary"]["applied"] == 1
        assert invoke("linked-principal-no-op", ["plan", "--mode", "apply", "-f", principal_file])["summary"]["total_changes"] == 0
        assert traces["linked-principal-update"]["registry_gets"] == 0
        assert traces["linked-principal-no-op"]["registry_gets"] == 0
        assert request("GET", route)["data"][0]["id"] == link
        # Q53: native principal DELETE succeeds and leaves the stored tuple.
        request("DELETE", root + "/principals/" + person)
        assert request("GET", root + "/principals/" + person, expected=(404,)) is None
        assert request("GET", route)["data"][0]["principal_id"] == person
        invoke("dangling-directory-plan-blocked", ["plan", "--mode", "delete", "-f", root_only], failure=True)
        invoke("dangling-directory-saved-delete-blocked", ["delete", "--plan", directory_plan, "--auto-approve"], failure=True)
        request("DELETE", route + "/" + link)
        link = None
        for agent in agents:
            request("DELETE", "/v1/agents/" + agent)
        agents.clear()
        assert invoke("unlinked-directory-delete", ["delete", "--plan", directory_plan, "--auto-approve"])["summary"]["applied"] == 1
        assert request("GET", root, expected=(404,)) is None
    except Exception:
        raise RuntimeError("Registry reference guard failed during " + phase + "; response values withheld") from None
    finally:
        try:
            if link:
                request("DELETE", "/v1/agents/" + agents[-1] + "/principals/" + link, expected=(204, 404))
            for agent in agents:
                request("DELETE", "/v1/agents/" + agent, expected=(204, 404))
            if directory:
                if person:
                    request("DELETE", "/v2/directories/" + directory + "/principals/" + person, expected=(204, 404))
                request("DELETE", "/v2/directories/" + directory + "?force=false", expected=(204, 404))
            assert registry_agents() == original_agents
        finally:
            scratch.cleanup()
    assert not private.exists()
    receipt = {"path": str(private), "removed": True, "private_sources_and_outputs_copied": False}
    for data in (json.dumps(receipt).encode(), json.dumps(traces).encode()):
        assert not any(needle in data for needle in needles)
    (Path(output) / "private-workspace.json").write_text(json.dumps(receipt) + "\n")
    (Path(output) / "public-reference-traces.json").write_text(json.dumps(traces, indent=2) + "\n")
    return {"managed_identity_fixture": True, "principal_delete_and_sync_blocked": True,
        "saved_execution_observes_late_links": True, "directory_dangling_target_blocked": True,
        "reads_and_updates_preserved": True, "configured_host_only": True, "registry_get_only": True,
        "unlink_allows_retirement": True, "private_workspace_removed": True, "fixtures_removed": True}


if __name__ == "__main__":
    try:
        print(json.dumps(main()))
    except Exception as error:
        print(str(error) if isinstance(error, RuntimeError) else "Registry guard scenario or cleanup failed", file=sys.stderr)
        sys.exit(1)
