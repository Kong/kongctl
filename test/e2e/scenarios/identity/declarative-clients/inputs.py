"""Prepare public manifests and inspect private initial-secret captures."""

import base64
import json
from pathlib import Path
import sys


def main():
    action = sys.argv[1]
    if action == "prepare":
        workdir, parent, name = sys.argv[2:5]
        root = """_defaults:
  kongctl: {namespace: identity-client-parent-e2e}
identity:
  auth_servers:
    - ref: issuer
      name: NAME
      audience: https://api.example.test
""".replace("NAME", json.dumps(name))
        client = """      clients:
        - ref: worker
          name: CLIENT
          kongctl: {namespace: identity-client-e2e}
          grant_types: [client_credentials]
          response_types: []
          redirect_uris: [https://client.example.test/callback]
          login_uri: https://client.example.test/login
          access_token_duration: 600
          id_token_duration: 900
          allow_all_scopes: false
          allow_scopes: [!ref read#id]
          token_endpoint_auth_method: client_secret_basic
          labels:
            e2e-fixture: PARENT
            team: identity
""".replace("CLIENT", json.dumps("kongctl-e2e-" + parent + "-worker")).replace("PARENT", json.dumps(parent))
        scope = """      scopes:
        - ref: read
          name: SCOPE
          enabled: true
""".replace("SCOPE", json.dumps("kongctl-e2e-" + parent + "-read"))
        initial = root + scope + client
        updated = (root + "      scopes: []\n" + client).replace("allow_scopes: [!ref read#id]", "allow_scopes: []").replace("redirect_uris: [https://client.example.test/callback]", "redirect_uris: []").replace("access_token_duration: 600", "access_token_duration: 1200").replace("id_token_duration: 900", "id_token_duration: 1800").replace("          token_endpoint_auth_method: client_secret_basic\n", "").replace("team: identity", "team: updated").replace("login_uri: https://client.example.test/login", 'login_uri: ""')
        external_root = "identity:\n  auth_servers:\n    - ref: issuer\n      _external: {id: " + parent + "}\n"
        external = external_root + updated[updated.index("      clients:"):]
        foreign = external.replace("ref: worker", "ref: foreign-worker").replace("-worker\"", "-foreign-worker\"").replace("namespace: identity-client-e2e", "namespace: identity-other-client-e2e")
        empty = "_defaults:\n  kongctl: {namespace: identity-client-e2e}\n" + external_root + "      clients: []\n"
        for filename, content in {"initial": initial, "updated": updated, "protected": updated.replace("namespace: identity-client-e2e}", "namespace: identity-client-e2e, protected: true}"), "external": external, "foreign": foreign, "empty": empty}.items():
            Path(workdir, filename + ".yaml").write_text(content)
        return {"prepared": True}
    if action == "verify":
        capture = json.loads(Path(sys.argv[2]).read_text())
        return {"captured": capture["status"] == "captured", "private": Path(sys.argv[2]).stat().st_mode & 0o077 == 0, "has_value": bool(capture.get("secret")), "has_ids": bool(capture.get("auth_server_id") and capture.get("client_id")), "has_ref": capture.get("resource_ref") == "worker", "namespace": capture.get("namespace")}
    if action == "verify-artifacts":
        captures = [json.loads(Path(path).read_text()) for path in sys.argv[2:4]]
        root = Path(sys.argv[4]).resolve().parents[2]
        needles = []
        for capture in captures:
            needles.extend([capture["secret"].encode(), base64.b64encode((capture["client_id"] + ":" + capture["secret"]).encode())])
        clean = all(not any(value in file.read_bytes() for value in needles) for file in root.rglob("*") if file.is_file())
        return {"clean": clean, "captures_outside_artifacts": all(not Path(path).resolve().is_relative_to(root) for path in sys.argv[2:4])}
    raise ValueError("unknown action")


try:
    print(json.dumps(main()))
except Exception:
    print("private initial credential verification failed", file=sys.stderr)
    sys.exit(1)
