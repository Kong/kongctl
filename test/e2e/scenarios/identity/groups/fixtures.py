"""Create credential-free manual group fixtures and inspect pagination traces."""

import json
import os
from pathlib import Path
import sys
import urllib.parse
import urllib.request


def main():
    action = sys.argv[1]
    if action == "create":
        directory, user, agent, legacy = sys.argv[2:6]
        target = os.environ.get("KONGCTL_E2E_KONNECT_ENV", "com").strip() or "com"
        assert target in ("com", "tech")
        base = os.environ.get("KONGCTL_E2E_KONNECT_BASE_URL", "").strip() or "https://us.api.konghq." + target
        parsed = urllib.parse.urlparse(base)
        assert parsed.scheme == "https" and parsed.hostname.endswith((".api.konghq.com", ".api.konghq.tech"))
        headers = {
            "Authorization": "Bearer " + os.environ["KONGCTL_E2E_KONNECT_PAT"],
            "Content-Type": "application/json",
        }

        def create(path, body):
            request = urllib.request.Request(base.rstrip("/") + path, method="POST", headers=headers,
                                             data=json.dumps(body).encode())
            with urllib.request.urlopen(request, timeout=30) as response:
                value = json.load(response)
            assert response.status == 201 and value.get("id")
            return value

        prefix = "Kongctl" + directory.replace("-", "")
        path = "/v2/directories/" + directory + "/groups"
        groups = {}
        for suffix, kind in [("Group", "group"), ("Team", "team"), ("Default", None), ("Empty", "group")]:
            body = {
                "name": prefix + suffix, "description": "Manual inspection fixture",
                "labels": {"e2e-fixture": directory, "team": "identity"},
                "metadata": {"count": 42, "enabled": True, "strings": ["a", "b"], "numbers": [1, 2]},
                "managed_by": {
                    "tool": "kongctl", "version": "e2e-fixture",
                    "kongctl-namespace": "identity-group-read-e2e", "kongctl-protected": "false",
                },
            }
            if suffix != "Empty":
                body["type"] = kind
            groups[suffix] = create(path, body)
        first = None
        for principal in [user, agent, legacy]:
            entry = create(path + "/" + groups["Group"]["id"] + "/membership",
                           {"member": {"type": "principal", "id": principal}})
            if first is None:
                first = entry["id"]
        create(path + "/" + groups["Team"]["id"] + "/membership",
               {"member": {"type": "principal", "id": user}})
        return {
            "created": len(groups), "group_id": groups["Group"]["id"], "group_name": groups["Group"]["name"],
            "team_id": groups["Team"]["id"], "team_name": groups["Team"]["name"],
            "empty_id": groups["Empty"]["id"], "other_id": groups["Default"]["id"], "entry_id": first,
        }
    if action == "verify-pagination":
        work = Path(sys.argv[2]).resolve()
        result = {}
        for command, suffix, pages in [("list-groups-json", "/groups ", 4),
                                        ("list-members-json", "/membership ", 3)]:
            trace = (work.parent / "commands" / command / "kongctl.log").read_text()
            requests = [line for line in trace.splitlines() if "log_type=http_request " in line and suffix in line]
            result[command] = (
                len(requests) >= pages
                and all("page[size]:1" in line for line in requests)
                and sum("page[after]:" in line for line in requests) >= pages - 1
            )
        return result
    raise ValueError("unknown action")


try:
    print(json.dumps(main()))
except Exception:
    print("Group fixture operation failed", file=sys.stderr)
    sys.exit(1)
