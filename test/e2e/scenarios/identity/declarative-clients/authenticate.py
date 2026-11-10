"""Exercise a captured initial client secret without printing credentials."""

import base64
import json
from pathlib import Path
import sys
import urllib.error
import urllib.parse
import urllib.request


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None


def main():
    capture_path, metadata_uri = sys.argv[1:3]
    capture = json.loads(Path(capture_path).read_text())
    host = urllib.parse.urlparse(metadata_uri)
    assert host.scheme == "https" and host.hostname.endswith((".identity.konghq.com", ".identity.konghq.tech"))
    opener = urllib.request.build_opener(NoRedirect())
    with opener.open(metadata_uri, timeout=30) as response:
        endpoint = json.load(response)["token_endpoint"]
    parsed = urllib.parse.urlparse(endpoint)
    assert parsed.scheme == "https" and parsed.hostname == host.hostname
    auth = base64.b64encode((capture["client_id"] + ":" + capture["secret"]).encode()).decode()
    request = urllib.request.Request(endpoint, method="POST",
        headers={"Content-Type": "application/x-www-form-urlencoded", "Authorization": "Basic " + auth},
        data=urllib.parse.urlencode({"grant_type": "client_credentials"}).encode())
    try:
        with opener.open(request, timeout=30) as response:
            body = json.load(response)
            return {"status": response.status, "has_token": bool(body.get("access_token"))}
    except urllib.error.HTTPError as error:
        error.read()
        return {"status": error.code, "has_token": False}


try:
    print(json.dumps(main()))
except Exception:
    print("private credential verification failed", file=sys.stderr)
    sys.exit(1)
