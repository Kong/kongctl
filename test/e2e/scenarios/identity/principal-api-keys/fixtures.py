"""Create marked generated-key fixtures; material stays in private captures."""

import base64
import datetime
import json
import os
from pathlib import Path
import sys
import urllib.parse
import urllib.request


def main():
    action, private_dir = sys.argv[1:3]
    private = Path(private_dir).resolve()
    if action == "create":
        directory, agent, legacy = sys.argv[3:6]
        target = os.environ.get("KONGCTL_E2E_KONNECT_ENV", "com").strip() or "com"
        assert target in ("com", "tech")
        base = os.environ.get("KONGCTL_E2E_KONNECT_BASE_URL", "").strip() or "https://us.api.konghq." + target
        parsed = urllib.parse.urlparse(base)
        assert parsed.scheme == "https" and parsed.hostname.endswith((".api.konghq.com", ".api.konghq.tech"))
        headers = {"Authorization": "Bearer " + os.environ["KONGCTL_E2E_KONNECT_PAT"], "Content-Type": "application/json"}
        expires = (datetime.datetime.now(datetime.timezone.utc) + datetime.timedelta(days=3)).isoformat()
        ids = []
        for index, principal in enumerate([agent, agent, agent, legacy]):
            body = {"type": "v1", "managed_by": {
                "tool": "kongctl", "version": "e2e-fixture", "kongctl-namespace": "identity-key-read-e2e",
                "kongctl-protected": "false", "kongctl-ref": "fixture-" + directory + "-" + str(index),
            }}
            if index == 0:
                body["expires_at"] = expires
            path = "/v2/directories/" + directory + "/principals/" + principal + "/api-keys"
            metadata = {"directory_id": directory, "principal_id": principal, "status": "creation_not_confirmed"}
            capture = private / ("key-" + str(index) + ".json")
            with os.fdopen(os.open(capture, os.O_CREAT | os.O_EXCL | os.O_WRONLY, 0o600), "w+") as output:
                json.dump(metadata, output)
                output.flush()
                os.fsync(output.fileno())
                request = urllib.request.Request(base.rstrip("/") + path, method="POST", headers=headers,
                                                 data=json.dumps(body).encode())
                with urllib.request.urlopen(request, timeout=30) as response:
                    value = json.load(response)
                assert response.status == 201 and value.get("id") and value.get("secret")
                metadata.update(status="captured", id=value["id"], secret=value["secret"])
                output.seek(0)
                output.truncate()
                json.dump(metadata, output)
                output.flush()
                os.fsync(output.fileno())
            ids.append(value["id"])
        return {"created": len(ids), "primary_id": ids[0], "legacy_key_id": ids[-1]}
    if action == "verify":
        work = Path(sys.argv[3]).resolve()
        root = work.parents[2]
        captures = list(private.glob("key-*.json"))
        needles = []
        for path in captures:
            value = json.loads(path.read_text())
            assert value["status"] == "captured" and value.get("secret")
            assert path.stat().st_mode & 0o077 == 0
            needles.extend([value["secret"].encode(), base64.b64encode(value["secret"].encode())])
        clean = all(not any(needle in path.read_bytes() for needle in needles)
                    for path in root.rglob("*") if path.is_file())
        trace = (work.parent / "commands" / "list-agent-page-one" / "kongctl.log").read_text()
        requests = [line for line in trace.splitlines() if "log_type=http_request " in line and "/api-keys " in line]
        return {"captures": len(captures), "private": True, "clean": clean,
                "captures_outside_artifacts": not private.is_relative_to(root),
                "traversed_pages": len(requests) >= 3,
                "bracketed_size": all("page[size]:1" in line for line in requests),
                "followed_cursors": sum("page[after]:" in line for line in requests) >= 2}
    raise ValueError("unknown action")


try:
    print(json.dumps(main()))
except Exception:
    print("private API-key fixture operation failed", file=sys.stderr)
    sys.exit(1)
