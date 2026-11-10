"""Local demo upstream; never logs or returns received credentials."""

import hmac
import json
import os
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer


expected = os.environ.get("SUPPORT_TICKETING_TOKEN")
if not expected:
    raise SystemExit("Set SUPPORT_TICKETING_TOKEN to the value stored in Token Vault.")


class Handler(BaseHTTPRequestHandler):
    def log_message(self, *_):
        pass

    def do_POST(self):
        if self.path != "/v1/chat/completions":
            self.send_error(404)
            return
        provided = self.headers.get("Authorization", "")
        verified = hmac.compare_digest(
            provided.encode(), ("Bearer " + expected).encode()
        )
        if verified:
            payload = {
                "id": "chatcmpl-identity-guide",
                "object": "chat.completion",
                "created": 0,
                "model": "gpt-4o",
                "choices": [{
                    "index": 0,
                    "finish_reason": "stop",
                    "message": {
                        "role": "assistant",
                        "content": "The upstream verified the ticketing credential.",
                    },
                }],
                "usage": {
                    "prompt_tokens": 0,
                    "completion_tokens": 0,
                    "total_tokens": 0,
                },
            }
        else:
            payload = {"error": {"message": "Upstream credential did not match."}}
        body = json.dumps(payload).encode()
        self.send_response(200 if verified else 401)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)
        self.close_connection = True
        print(f"upstream_credential_verified={verified}", flush=True)


address = (os.environ.get("SUPPORT_TICKETING_BIND", "127.0.0.1"), 18080)
print(f"Demo upstream listening on http://{address[0]}:{address[1]}", flush=True)
ThreadingHTTPServer(address, Handler).serve_forever()
