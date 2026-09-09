#!/usr/bin/env python3
"""Experimental, explicit record/replay of one unchanged Konnect E2E scenario.

No production code or ordinary E2E routing depends on this module. The HTTP
engine is resource-independent; eligibility is intentionally a one-scenario
allowlist until recording fidelity and maintenance cost have been measured.
"""

from __future__ import annotations

import argparse
import hashlib
import http.client
import http.server
import json
import os
from pathlib import Path
import re
import socket
import ssl
import subprocess
import sys
import tempfile
import threading
import time
from urllib.parse import parse_qsl, urlsplit


ROOT = Path(__file__).resolve().parents[1]
SCENARIO = "control-plane/get"
HOSTS = {"us.api.konghq.com": "regional", "global.api.konghq.com": "global"}
DUMMY_PAT = "replay-dummy"
LIMIT = 8 * 1024 * 1024
UUID = re.compile(r"\b[0-9a-fA-F]{8}(?:-[0-9a-fA-F]{4}){3}-[0-9a-fA-F]{12}\b")
SENSITIVE_KEY = re.compile(r"password|secret|token|authorization|cookie|private.?key", re.I)
SENSITIVE_VALUE = re.compile(r"kpat_|spat_|Bearer\s|-----BEGIN|[\w.+-]+@[\w.-]+\.[a-z]{2,}", re.I)


def canonical(value):
    return json.dumps(value, sort_keys=True, separators=(",", ":"), allow_nan=False)


def parse_json(data):
    def unique(pairs):
        result = {}
        for key, value in pairs:
            if key in result:
                raise ValueError("duplicate JSON key")
            result[key] = value
        return result

    return json.loads(data, object_pairs_hook=unique,
                      parse_constant=lambda _: (_ for _ in ()).throw(ValueError("non-finite JSON number")))


def scenario_digest(directory):
    """Hash names and bytes, including additions/deletions; never follow symlinks."""
    digest = hashlib.sha256()
    for path in sorted(directory.rglob("*")):
        relative = path.relative_to(directory)
        if relative.parts[0] == "replay":
            continue
        if path.is_symlink():
            raise ValueError("symlinked scenario inputs are not replay-supported")
        if path.is_file():
            name, data = relative.as_posix().encode(), path.read_bytes()
            digest.update(len(name).to_bytes(8, "big") + name)
            digest.update(len(data).to_bytes(8, "big") + data)
    return "sha256:" + digest.hexdigest()


def check_eligibility(directory):
    # A deliberately conservative boundary for the single prototype scenario.
    # New external dependencies, custom commands or org pins require explicit
    # implementation review, not merely a refreshed fingerprint.
    definition = (directory / "scenario.yaml").read_text(encoding="utf-8")
    if re.search(r"(?m)^\s*(?:-\s*)?(exec|create|delete|resetOrgRegions|env|requiredEnvVars|assignedEnvironment):", definition):
        raise ValueError("scenario gained an unsupported command, environment dependency or organization pin")
    for path in directory.rglob("*"):
        if path.relative_to(directory).parts[0] == "replay":
            continue
        if path.is_symlink():
            raise ValueError("symlinked scenario inputs are not replay-supported")
        if not path.is_file():
            continue
        content = path.read_text(encoding="utf-8")
        if any(marker in content for marker in ("../", "repo_dir", "https://", "http://")):
            raise ValueError("scenario gained an external input; replay dependency review is required")


def check_safe(value):
    """Fail closed on sensitive fields; never persist arbitrary HTTP headers."""
    if isinstance(value, dict):
        for key, item in value.items():
            if SENSITIVE_KEY.search(key):
                raise ValueError("sensitive field: cassette requires explicit sanitizer review")
            check_safe(item)
    elif isinstance(value, list):
        for item in value:
            check_safe(item)
    elif isinstance(value, str) and SENSITIVE_VALUE.search(value):
        raise ValueError("sensitive value: refusing to publish cassette")


class Sanitizer:
    """Recording-only normalization. Replay requests are NOT wildcard-normalized."""

    def __init__(self):
        self.ids = {}

    def normalize(self, value):
        if isinstance(value, dict):
            return {key: self.normalize(item) for key, item in value.items()}
        if isinstance(value, list):
            return [self.normalize(item) for item in value]
        if isinstance(value, str):
            def replace(match):
                key = match.group().lower()
                if key not in self.ids:
                    self.ids[key] = f"00000000-0000-4000-8000-{len(self.ids) + 1:012d}"
                return self.ids[key]
            value = UUID.sub(replace, value)
            return re.sub(r"https://[a-z0-9]+\.(us|eu|au)\.(cp|tp)\.konghq\.com",
                          r"https://replay.\1.\2.konghq.com", value)
        return value


def request_key(endpoint, method, target, data):
    url = urlsplit(target)
    if url.scheme or url.netloc or url.fragment or not target.startswith("/"):
        raise ValueError("only origin-form requests are supported")
    body = parse_json(data) if data else None
    if data and body is None:
        raise ValueError("explicit JSON null request bodies are unsupported")
    return {"endpoint": endpoint, "method": method, "path": url.path,
            "query": [list(pair) for pair in sorted(parse_qsl(url.query, keep_blank_values=True))], "body": body}


def validate_cassette(cassette, directory, allow_bootstrap=False):
    check_eligibility(directory)
    if not isinstance(cassette, dict) or set(cassette) != {"schema_version", "scenario", "inputs_sha256", "source", "interactions"}:
        raise ValueError("invalid cassette fields")
    if cassette["schema_version"] != 1 or cassette["scenario"] != SCENARIO:
        raise ValueError("unsupported cassette schema or scenario")
    if cassette["inputs_sha256"] != scenario_digest(directory):
        raise ValueError(f"stale cassette: {SCENARIO}; re-record and review, or remove replay eligibility")
    source = cassette["source"]
    if not isinstance(source, dict) or not re.fullmatch(r"[0-9a-f]{40}", source.get("commit", "")):
        raise ValueError("cassette must identify its recorded commit")
    if set(source) != {"kind", "commit", "run_url"}:
        raise ValueError("invalid provenance fields")
    if source.get("kind") not in {"recorded", "bootstrap"}:
        raise ValueError("cassette must distinguish live recording from bootstrap fixture")
    if source["kind"] == "bootstrap" and not allow_bootstrap:
        raise ValueError("bootstrap is not a live recording; use --allow-bootstrap only for transport evaluation")
    if not re.fullmatch(r"https://github.com/[Kk]ong/kongctl/actions/runs/[0-9]+", source.get("run_url", "")):
        raise ValueError("cassette must identify its successful live source run")
    interactions = cassette["interactions"]
    if not isinstance(interactions, list) or not 0 < len(interactions) <= 10000:
        raise ValueError("cassette requires a bounded, nonempty interaction list")
    for item in interactions:
        if not isinstance(item, dict) or set(item) != {"request", "response"}:
            raise ValueError("invalid interaction fields")
        request, response = item["request"], item["response"]
        if not isinstance(request, dict) or set(request) != {"endpoint", "method", "path", "query", "body"}:
            raise ValueError("invalid request fields")
        if request["endpoint"] not in HOSTS.values() or request["method"] not in {"GET", "POST", "PUT", "PATCH", "DELETE"}:
            raise ValueError("unsupported endpoint or HTTP method")
        if not isinstance(request["path"], str) or not request["path"].startswith("/"):
            raise ValueError("invalid request path")
        if not isinstance(request["query"], list) or any(
            not isinstance(pair, list) or len(pair) != 2 or any(not isinstance(x, str) for x in pair)
            for pair in request["query"]
        ):
            raise ValueError("invalid request query")
        if not isinstance(response, dict) or set(response) != {"status", "body"} or type(response["status"]) is not int:
            raise ValueError("invalid response fields")
        if not 200 <= response["status"] < 500 or response["status"] == 429 or 300 <= response["status"] < 400:
            raise ValueError("redirects and transient failures are not supported in cassettes")
        check_safe(item)
        canonical(item)


class Replay:
    def __init__(self, cassette=None, token=None):
        self.interactions = cassette["interactions"] if cassette else []
        self.token = token
        self.position = 0
        self.errors = []
        self.lock = threading.Lock()
        self.sanitizer = Sanitizer()

    def fail(self, message):
        with self.lock:
            self.errors.append(message)

    def exchange(self, host, method, target, data):
        # Serialize complete exchanges. The prototype intentionally requires a
        # deterministic request order; future unordered groups must be explicit.
        with self.lock:
            request = request_key(HOSTS[host], method, target, data)
            if self.token is None:
                if self.position >= len(self.interactions):
                    raise ValueError("unexpected extra request")
                item = self.interactions[self.position]
                if canonical(request) != canonical(item["request"]):
                    raise ValueError(f"request mismatch at interaction {self.position + 1} (method/path/query/body)")
                self.position += 1
                return item["response"]

            # HTTPSConnection never uses proxy environment variables. The host
            # is from the fixed CONNECT allowlist, not from a cassette or URL.
            connection = http.client.HTTPSConnection(host, timeout=60)
            try:
                connection.request(method, target, body=data or None, headers={
                    "Authorization": "Bearer " + self.token,
                    "Accept": "application/json", "Content-Type": "application/json",
                })
                raw = connection.getresponse()
                content = raw.read(LIMIT + 1)
                if len(content) > LIMIT:
                    raise ValueError("response exceeds recording limit")
                response = {"status": raw.status, "body": parse_json(content) if content else None}
            finally:
                connection.close()
            item = self.sanitizer.normalize({"request": request, "response": response})
            if self.token in canonical(item):
                raise ValueError("credential echoed in response; refusing recording")
            check_safe(item)
            self.interactions.append(item)
            self.position += 1
            # Live CLI sees the real IDs, so its subsequent requests remain
            # valid upstream. Only the candidate cassette contains replacements.
            return response

    def verify(self):
        if self.errors:
            raise ValueError("; ".join(self.errors[:5]))
        if self.position != len(self.interactions):
            raise ValueError(f"{len(self.interactions) - self.position} required interactions unused")


class QuietHandler(http.server.BaseHTTPRequestHandler):
    def log_message(self, *_):
        pass  # Never put request bodies, paths or credentials in server logs.

    def send_error(self, code, message=None, explain=None):
        self.server.engine.fail("HTTP protocol rejected a request")
        super().send_error(code, "replay request rejected", "See replay diagnostics")


class ProxyHandler(QuietHandler):
    def do_CONNECT(self):
        host = self.path.removesuffix(":443")
        if self.path != host + ":443" or host not in HOSTS:
            self.server.engine.fail("blocked CONNECT destination")
            self.send_error(403)
            return
        self.send_response(200)
        self.end_headers()
        self.close_connection = True
        try:
            with self.server.tls.wrap_socket(self.connection, server_side=True) as connection:
                connection.settimeout(90)
                InnerHandler(connection, self.client_address, self.server, tunnel_host=host)
        except (OSError, ValueError):
            self.server.engine.fail("TLS tunnel failed")

    def do_GET(self):
        self.server.engine.fail("only HTTPS CONNECT is supported")
        self.send_error(403)


class InnerHandler(QuietHandler):
    protocol_version = "HTTP/1.1"

    def __init__(self, *args, tunnel_host):
        self.tunnel_host = tunnel_host
        super().__init__(*args)

    def exchange(self):
        try:
            if self.headers.get("Host") not in {self.tunnel_host, self.tunnel_host + ":443"}:
                raise ValueError("Host does not match CONNECT destination")
            if self.headers.get("Transfer-Encoding"):
                raise ValueError("chunked requests are unsupported")
            length = int(self.headers.get("Content-Length", "0"))
            if not 0 <= length <= LIMIT:
                raise ValueError("request exceeds recording limit")
            data = self.rfile.read(length)
            response = self.server.engine.exchange(self.tunnel_host, self.command, self.path, data)
            body = canonical(response["body"]).encode() if response["body"] is not None else b""
            self.send_response(response["status"])
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)
        except Exception as error:
            # Never stringify upstream HTTP/TLS errors (may contain live data).
            message = str(error) if isinstance(error, ValueError) and not isinstance(error, json.JSONDecodeError) else "proxy exchange failed"
            self.server.engine.fail(message)
            self.close_connection = True
            self.send_error(400, "replay exchange failed")

    do_GET = exchange
    do_POST = exchange
    do_PUT = exchange
    do_PATCH = exchange
    do_DELETE = exchange
    do_HEAD = exchange
    do_OPTIONS = exchange


class Server:
    def __init__(self, engine, directory):
        certificate, key = directory / "ca.pem", directory / "key.pem"
        subprocess.run([
            "openssl", "req", "-x509", "-newkey", "rsa:2048", "-nodes", "-days", "1",
            "-subj", "/CN=kongctl-replay", "-addext", "subjectAltName=" + ",".join("DNS:" + h for h in HOSTS),
            "-keyout", str(key), "-out", str(certificate),
        ], check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        tls = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
        tls.minimum_version = ssl.TLSVersion.TLSv1_2
        tls.load_cert_chain(certificate, key)
        self.httpd = http.server.ThreadingHTTPServer(("127.0.0.1", 0), ProxyHandler)
        self.httpd.engine, self.httpd.tls = engine, tls
        self.url = f"http://127.0.0.1:{self.httpd.server_port}"
        self.certificate = certificate
        self.thread = threading.Thread(target=self.httpd.serve_forever, daemon=True)

    def __enter__(self):
        self.thread.start()
        return self

    def __exit__(self, *_):
        self.httpd.shutdown()
        self.httpd.server_close()
        self.thread.join()


def clean_environment(directory, binary):
    # Do not inherit profiles, credentials, proxy bypasses, filters, skip flags,
    # shard configuration, debug capture switches or user CLI settings.
    env = {key: os.environ[key] for key in ("PATH", "TMPDIR", "SYSTEMROOT") if key in os.environ}
    env.update({
        "DO_NOT_TRACK": "1", "XDG_CONFIG_HOME": str(directory / "config"),
        "KONGCTL_E2E_BIN": str(binary), "KONGCTL_E2E_ARTIFACTS_DIR": str(directory / "artifacts"),
        "KONGCTL_E2E_SCENARIO": SCENARIO, "KONGCTL_E2E_KONNECT_ENV": "com",
        "KONGCTL_E2E_KONNECT_BASE_URL": "https://us.api.konghq.com",
        "KONGCTL_E2E_KONNECT_BASE_AUTH_URL": "https://global.api.konghq.com",
        "KONGCTL_E2E_KONNECT_PAT": DUMMY_PAT, "KONGCTL_E2E_RESET": "0",
        "KONGCTL_E2E_CONSOLE_LOG_LEVEL": "warn", "KONGCTL_E2E_BETA_MODE": "fail",
    })
    return env


def write_json(path, value):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, indent=2, allow_nan=False) + "\n", encoding="utf-8")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("mode", choices=("check", "replay", "record"))
    parser.add_argument("--cassette", type=Path, default=ROOT / "test/e2e/scenarios" / SCENARIO / "replay/cassette.json")
    parser.add_argument("--binary", type=Path, default=ROOT / "kongctl")
    parser.add_argument("--test-binary", type=Path, default=ROOT / "e2e.test")
    parser.add_argument("--reset-binary", type=Path, default=ROOT / "reset-org.test")
    parser.add_argument("--output-dir", type=Path, default=ROOT / ".e2e-artifacts/replay")
    parser.add_argument("--allow-bootstrap", action="store_true", help="explicitly allow the unverified transport fixture")
    parser.add_argument("--require-isolated", action="store_true", help="require a Linux loopback-only network namespace")
    args = parser.parse_args()
    directory = ROOT / "test/e2e/scenarios" / SCENARIO
    if args.require_isolated and (sys.platform != "linux" or {name for _, name in socket.if_nameindex()} != {"lo"}):
        raise ValueError("replay requires a network namespace containing only loopback")
    if args.mode == "record" and args.require_isolated:
        raise ValueError("recording requires live networking, not replay isolation")
    cassette = None
    if args.mode != "record":
        cassette = parse_json(args.cassette.read_bytes())
        validate_cassette(cassette, directory, args.allow_bootstrap)
        if args.mode == "check":
            print(f"Cassette inputs and schema valid: {SCENARIO}")
            return
    token = None
    if args.mode == "record":
        check_eligibility(directory)
        # Recording is deliberately CI-only: manual trusted workflow + the
        # existing acceptance-3 environment and exact live shard lock.
        if os.environ.get("GITHUB_ACTIONS") != "true" or os.environ.get("GITHUB_EVENT_NAME") != "workflow_dispatch":
            raise ValueError("record only through the manual E2E replay workflow (organization lock required)")
        if os.environ.get("KONGCTL_E2E_MATRIX_ORG") != "kongctl-acceptance-3":
            raise ValueError("recording requires the locked kongctl-acceptance-3 environment")
        token = os.environ.get("KONGCTL_E2E_KONNECT_PAT")
        if not token:
            raise ValueError("recording PAT is missing")
    for binary in (args.binary, args.test_binary, *([args.reset_binary] if token else [])):
        if not binary.is_file():
            raise ValueError(f"missing executable: {binary}; run make build-e2e-replay")
    args.output_dir.mkdir(parents=True, exist_ok=True)
    engine = Replay(cassette, token)
    with tempfile.TemporaryDirectory(prefix="kongctl-replay-") as temporary:
        private = Path(temporary)
        env = clean_environment(private, args.binary.resolve())
        reset_env = {**env, "KONGCTL_E2E_KONNECT_PAT": token or DUMMY_PAT, "KONGCTL_E2E_RESET": "1"}
        started = time.monotonic()
        scenario_elapsed = 0.0
        try:
            if token:
                subprocess.run([str(args.reset_binary.resolve()), "--stage", "before-replay-record"],
                               env=reset_env, check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, timeout=600)
            with Server(engine, private) as server:
                env.update({"HTTPS_PROXY": server.url, "HTTP_PROXY": server.url, "NO_PROXY": "",
                            "SSL_CERT_FILE": str(server.certificate), "SSL_CERT_DIR": str(private / "empty-certs")})
                scenario_started = time.monotonic()
                result = subprocess.run([str(args.test_binary.resolve()), "-test.run", "^Test_Scenarios$", "-test.v",
                                         "-test.count=1", "-test.timeout=10m"], cwd=ROOT, env=env,
                                        stdout=subprocess.PIPE, stderr=subprocess.STDOUT, timeout=660)
                scenario_elapsed = time.monotonic() - scenario_started
            engine.verify()
            # A skipped test must never become a passing replay/recording.
            output = result.stdout.decode(errors="replace")
            if result.returncode or f"--- PASS: Test_Scenarios/test/e2e/scenarios/{SCENARIO}/scenario.yaml" not in output:
                raise ValueError("scenario did not pass; candidate not published (raw live logs are not uploaded)")
        finally:
            if token:
                subprocess.run([str(args.reset_binary.resolve()), "--stage", "after-replay-record"],
                               env=reset_env, check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, timeout=600)
        elapsed = time.monotonic() - started
    if token:
        candidate = {"schema_version": 1, "scenario": SCENARIO, "inputs_sha256": scenario_digest(directory),
                     "source": {"kind": "recorded", "commit": os.environ.get("GITHUB_SHA", ""),
                                "run_url": "https://github.com/Kong/kongctl/actions/runs/" + os.environ.get("GITHUB_RUN_ID", "")},
                     "interactions": engine.interactions}
        validate_cassette(candidate, directory)
        # Never overwrite the reviewed cassette, even when recording succeeded.
        write_json(args.output_dir / "candidate-cassette.json", candidate)
    summary = {"mode": args.mode, "scenario": SCENARIO, "interactions": engine.position,
               "elapsed_seconds": round(elapsed, 3), "status": "pass",
               "scenario_seconds": round(scenario_elapsed, 3),
               "network_isolated": args.require_isolated,
               "source_kind": "recorded" if token else cassette["source"]["kind"]}
    write_json(args.output_dir / "summary.json", summary)
    print(json.dumps(summary))


if __name__ == "__main__":
    try:
        main()
    except (ValueError, OSError, subprocess.SubprocessError) as error:
        print(f"E2E replay failed: {error}", file=sys.stderr)
        sys.exit(1)
