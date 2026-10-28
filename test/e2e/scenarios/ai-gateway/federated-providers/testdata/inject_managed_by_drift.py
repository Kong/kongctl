"""Simulate outside tool metadata without printing credentials or API payloads."""

import json
import os
import urllib.error
import urllib.request


base = os.environ.get(
    "KONGCTL_E2E_KONNECT_BASE_URL", "https://us.api.konghq.com"
).rstrip("/") + "/v1/ai-gateways"
token = os.environ["KONGCTL_E2E_KONNECT_PAT"]


def request(path="", method="GET", body=None):
    req = urllib.request.Request(
        base + path,
        method=method,
        headers={"Authorization": "Bearer " + token, "Content-Type": "application/json"},
        data=json.dumps(body).encode() if body is not None else None,
    )
    try:
        with urllib.request.urlopen(req, timeout=30) as response:
            return json.load(response)
    except urllib.error.HTTPError as error:
        raise SystemExit(f"Metadata drift setup returned HTTP {error.code}") from None
    except urllib.error.URLError:
        raise SystemExit("Metadata drift setup failed to reach Konnect") from None


gateway = next(
    item for item in request()["data"] if item["name"] == "federated-providers-e2e"
)
parent = "/" + gateway["id"] + "/model-providers"
provider = next(item for item in request(parent)["data"] if item["name"] == "provider-a")
body = {key: provider[key] for key in ("name", "display_name", "type", "config", "labels") if key in provider}
body["managed_by"] = dict(provider["managed_by"], outside="remove")
request(parent + "/" + provider["id"], "PUT", body)
print(json.dumps({"injected": True}))
