"""Prepare public identity manifests; generated OAuth secrets stay private."""

import json
from pathlib import Path
import re
import sys


def write(work, name, value):
    text = json.dumps(value, indent=2)
    text = re.sub(r'"REF:([^"]+)"', lambda m: "!ref " + m.group(1), text)
    Path(work, name + ".yaml").write_text(text + "\n")


def parent_scope(directory, principal):
    return {
        "_defaults": {"kongctl": {"namespace": "identity-association-e2e"}},
        "identity": {"directories": [{
            "ref": "people", "_external": {"id": directory}, "principals": [{
                "ref": "person", "_external": {"id": principal},
            }],
        }]},
    }


def main():
    action, work = sys.argv[1:3]
    if action == "initial":
        directory, server, control_plane, consumer = sys.argv[3:7]
        labels = {"e2e-fixture": "REF:people#id", "e2e-principal-id": "REF:person#id", "team": "identity"}
        associations = [
            {"ref": "employee", "type": "custom", "key": "employee_id", "value": directory},
            {"ref": "login", "type": "oidc", "issuer": "https://issuer.example.test",
             "claim": {"name": "sub", "value": directory}},
            {"ref": "worker", "type": "auth_server_client", "auth_server_id": "REF:issuer#id", "client_id": "REF:primary-client#id"},
            {"ref": "gateway-user", "type": "control_plane_consumer", "control_plane_id": "REF:gateway#id", "consumer_id": consumer},
        ]
        for association in associations:
            association["labels"] = dict(labels)
        clients = [{
            "ref": ref, "name": server + suffix, "grant_types": ["client_credentials"], "response_types": [],
            "token_endpoint_auth_method": "client_secret_post", "labels": {"e2e-fixture": "REF:issuer#id"},
            "kongctl": {"namespace": "identity-client-e2e"},
        } for ref, suffix in [("primary-client", "-primary"), ("replacement-client", "-replacement")]]
        value = {
            "_defaults": {"kongctl": {"namespace": "identity-association-e2e"}},
            "control_planes": [{"ref": "gateway", "_external": {"id": control_plane}}],
            "identity": {
                "auth_servers": [{"ref": "issuer", "name": server, "audience": "https://api.example.test",
                                  "kongctl": {"namespace": "identity-auth-server-e2e"}, "clients": clients}],
                "directories": [{
                    "ref": "people", "name": directory, "allow_all_control_planes": True, "vault_enabled": False,
                    "kongctl": {"namespace": "identity-directory-e2e"},
                    "principals": [{"ref": "person", "display_name": "Directory user", "type": "user",
                                    "labels": {"e2e-fixture": "REF:people#id"},
                                    "kongctl": {"namespace": "identity-principal-e2e", "protected": True},
                                    "identities": associations}],
                }],
            },
        }
        Path(work, "initial.json").write_text(json.dumps(value))
        write(work, "initial", value)
    elif action == "update":
        directory, principal, second_consumer = sys.argv[3:6]
        value = json.loads(Path(work, "initial.json").read_text())
        identities = value["identity"]["directories"][0]["principals"][0]["identities"]
        identities[0].update(key="employee_number", value=directory + "-updated")
        identities[1].update(issuer="https://updated.example.test", claim={"name": "email", "value": "updated@example.test"})
        identities[2]["client_id"] = "REF:replacement-client#id"
        identities[3]["consumer_id"] = second_consumer
        for identity in identities:
            identity["labels"] = {"e2e-fixture": directory, "e2e-principal-id": principal}
        write(work, "update", value)
    elif action == "scopes":
        directory, principal = sys.argv[3:5]
        for name in ["omitted", "empty"]:
            value = parent_scope(directory, principal)
            if name == "empty":
                value["identity"]["directories"][0]["principals"][0]["identities"] = []
            write(work, name, value)
        labels = {"e2e-fixture": directory, "e2e-principal-id": principal}
        unowned = {"type": "custom", "key": "external_id", "value": directory, "labels": labels}
        Path(work, "unowned.json").write_text(json.dumps(unowned))
        foreign = dict(unowned, key="foreign_id", managed_by={"tool": "kongctl", "version": "e2e",
            "kongctl-namespace": "other-identity-team", "kongctl-ref": "foreign", "kongctl-protected": "true"})
        Path(work, "foreign.json").write_text(json.dumps(foreign))
        for name, protected in [("adopted", False), ("protected", True)]:
            value = parent_scope(directory, principal)
            value["identity"]["identities"] = [{"ref": "adopted", "principal": "person", **unowned,
                                                 "kongctl": {"protected": protected}}]
            write(work, name, value)
        value = json.loads(Path(work, "initial.json").read_text())
        parent = value["identity"]["directories"][0]["principals"][0]
        parent["identities"] = []
        parent["kongctl"]["protected"] = False
        write(work, "teardown", value)
    else:
        raise ValueError("unknown action")
    print(json.dumps({"prepared": True}))


main()
