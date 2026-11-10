"""Prepare public fixture payloads; credential capture remains private."""

import json
from pathlib import Path
import sys


def write(path, value):
    Path(path).write_text(json.dumps(value, indent=2) + "\n")


def main():
    action, workdir = sys.argv[1:3]
    if action == "client":
        server = sys.argv[3]
        write(Path(workdir, "client.yaml"), {
            "_defaults": {"kongctl": {"namespace": "identity-id-e2e"}},
            "identity": {"auth_servers": [{
                "ref": "issuer", "_external": {"id": server},
                "clients": [{
                    "ref": "identity-worker", "name": "kongctl-e2e-" + server + "-identity-worker",
                    "grant_types": ["client_credentials"], "response_types": [],
                    "token_endpoint_auth_method": "client_secret_basic",
                    "labels": {"e2e-fixture": server},
                }],
            }]},
        })
    elif action == "parent-delete":
        directory = sys.argv[3]
        for name, principals in [("parent-delete", [{"ref": "owner"}]), ("parent-empty", [])]:
            write(Path(workdir, name + ".yaml"), {
                "_defaults": {"kongctl": {"namespace": "identity-id-e2e"}},
                "identity": {"directories": [{
                    "ref": "directory", "_external": {"id": directory}, "principals": principals,
                }]},
            })
    elif action == "identities":
        directory, principal, server, client, control_plane, consumer = sys.argv[3:9]
        metadata = {"tool": "kongctl", "version": "e2e", "kongctl-namespace": "identity-id-e2e", "kongctl-protected": "false"}
        labels = {"e2e-fixture": directory, "e2e-principal-id": principal, "team": "identity"}
        bodies = {"custom-" + str(i): {"type": "custom", "key": "employee_id", "value": directory + "-" + str(i)} for i in range(7)}
        bodies["oidc"] = {"type": "oidc", "issuer": "https://issuer.example.test", "claim": {"name": "sub", "value": directory}}
        bodies["oauth"] = {"type": "auth_server_client", "auth_server_id": server, "client_id": client}
        bodies["consumer"] = {"type": "control_plane_consumer", "control_plane_id": control_plane, "consumer_id": consumer}
        for ref, body in bodies.items():
            write(Path(workdir, ref + ".json"), dict(body, labels=labels, managed_by=dict(metadata, **{"kongctl-ref": ref})))
    elif action == "verify-pagination":
        work = Path(workdir).resolve()
        candidates = [parent / "commands" / "list-all-pages" / "kongctl.log" for parent in [work, *work.parents]]
        trace = next(path for path in candidates if path.is_file()).read_text()
        requests = [line for line in trace.splitlines() if "log_type=http_request " in line and "/identities " in line]
        print(json.dumps({"traversed_pages": len(requests) >= 10,
                          "followed_cursors": sum("page[after]:" in line for line in requests) >= 9,
                          "bracketed_size": all("page[size]:1" in line for line in requests)}))
        return
    else:
        raise ValueError("unknown action")
    print(json.dumps({"prepared": True}))


main()
