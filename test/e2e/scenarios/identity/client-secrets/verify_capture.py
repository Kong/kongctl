"""Inspect private captures or exercise credentials without printing material."""

import base64
import json
import os
from pathlib import Path
import sys
import urllib.error
import urllib.parse
import urllib.request


def read_capture(path):
    return json.loads(Path(path).read_text())


def main():
    action, path = sys.argv[1:3]
    if action == "prepare-source":
        with os.fdopen(os.open(path, os.O_CREAT | os.O_EXCL | os.O_WRONLY, 0o600), "w") as output:
            output.write(base64.urlsafe_b64encode(os.urandom(48)).decode())
        return {"prepared": True}
    if action == "verify-artifacts":
        captures = [read_capture(path), read_capture(sys.argv[3])]
        root = Path(sys.argv[4]).resolve().parents[2]
        needles = []
        for item in captures:
            needles.append(item["secret"].encode())
            needles.append(base64.b64encode((item["client_id"] + ":" + item["secret"]).encode()))
        clean = True
        for file in root.rglob("*"):
            if file.is_file():
                data = file.read_bytes()
                clean = clean and not any(value in data for value in needles)
        return {"clean": clean, "captures_outside_artifacts":
            not Path(path).resolve().is_relative_to(root) and not Path(sys.argv[3]).resolve().is_relative_to(root)}
    capture = read_capture(path)
    if action == "verify":
        source = sys.argv[3] if len(sys.argv) > 3 else None
        return {
            "captured": capture["status"] == "captured",
            "private": Path(path).stat().st_mode & 0o077 == 0,
            "has_value": bool(capture.get("secret")),
            "has_ids": bool(capture.get("auth_server_id") and capture.get("client_id") and capture.get("secret_id")),
            "matches_source": source is None or capture.get("secret") == Path(source).read_text(),
        }
    if action == "verify-pending":
        return {"unconfirmed": capture["status"] == "creation_not_confirmed", "has_value": bool(capture.get("secret"))}
    if action == "authenticate":
        metadata_uri = sys.argv[3]
        host = urllib.parse.urlparse(metadata_uri)
        assert host.scheme == "https" and host.hostname.endswith((".identity.konghq.com", ".identity.konghq.tech"))
        with urllib.request.urlopen(metadata_uri, timeout=30) as response:
            endpoint = json.load(response)["token_endpoint"]
        parsed = urllib.parse.urlparse(endpoint)
        assert parsed.scheme == "https" and parsed.hostname == host.hostname
        auth = base64.b64encode((capture["client_id"] + ":" + capture["secret"]).encode()).decode()
        request = urllib.request.Request(endpoint, method="POST",
            headers={"Content-Type": "application/x-www-form-urlencoded", "Authorization": "Basic " + auth},
            data=urllib.parse.urlencode({"grant_type": "client_credentials"}).encode())
        try:
            with urllib.request.urlopen(request, timeout=30) as response:
                body = json.load(response)
                return {"status": response.status, "has_token": bool(body.get("access_token"))}
        except urllib.error.HTTPError as error:
            error.read()
            return {"status": error.code, "has_token": False}
    raise ValueError("unknown action")


try:
    print(json.dumps(main()))
except Exception:
    # Tracebacks and parser errors can contain sensitive response/file content.
    print("private credential verification failed", file=sys.stderr)
    sys.exit(1)
