#!/usr/bin/env python3
"""Explicit record/replay of unchanged Konnect E2E scenarios.

No production code depends on this module. The HTTP engine is resource-independent;
PR eligibility is an explicit reviewed subset in test/e2e/replay-scenarios.json.
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
# CI installs the pinned parser here before entering network isolation. Local
# users can install the same requirements in their active Python environment.
sys.path.insert(0, str(ROOT / ".e2e-artifacts/replay-python"))
SCENARIO = "control-plane/get"
SCENARIOS = (
    "control-plane/apply", "control-plane/delete-groups", "control-plane/get",
    "control-plane/groups", "control-plane/plan/apply-workflow",
    "control-plane/sync", "control-plane/sync-groups", "portal/sync",
)
HOSTS = {"us.api.konghq.com": "regional", "global.api.konghq.com": "global"}
DUMMY_PAT = "replay-dummy"
LIMIT = 8 * 1024 * 1024
UUID = re.compile(r"\b[0-9a-fA-F]{8}(?:-[0-9a-fA-F]{4}){3}-[0-9a-fA-F]{12}\b")
SENSITIVE_KEY = re.compile(r"password|secret|token|authorization|cookie|private.?key", re.I)
SENSITIVE_VALUE = re.compile(r"kpat_|spat_|Bearer\s|-----BEGIN|[\w.+-]+@[\w.-]+\.[a-z]{2,}", re.I)
KONG_HOST = re.compile(r"\b(?:[a-z0-9-]+\.)*konghq\.com\b", re.I)
GENERATED_HOST = re.compile(r"\b(?:[a-z0-9-]+\.)+([a-z0-9-]+)\.(cp|tp)\.konghq\.com\b", re.I)


def canonical(value):
    return json.dumps(value, sort_keys=True, separators=(",", ":"), allow_nan=False)


def parse_json(data):
    def reject_constant(_):
        raise ValueError("non-finite JSON number")

    def unique(pairs):
        result = {}
        for key, value in pairs:
            if key in result:
                raise ValueError("duplicate JSON key")
            result[key] = value
        return result

    return json.loads(data, object_pairs_hook=unique, parse_constant=reject_constant)


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
    # A deliberately conservative boundary for the reviewed local-input scenarios.
    # New external dependencies, custom commands or org pins require explicit
    # implementation review, not merely a refreshed fingerprint.
    definition = (directory / "scenario.yaml").read_text(encoding="utf-8")
    definition = definition.replace("env:\n  KONGCTL_LOG_LEVEL: info\n", "")
    unsupported = (
        "exec|create|delete|resetOrgRegions|env|requiredEnvVars|assignedEnvironment|"
        "inputOverlayOpsFiles|inputOverlayOps|stdinFile|workdir"
    )
    if re.search(r"(?m)^\s*(?:-\s*)?(" + unsupported + r"):", definition):
        raise ValueError("scenario gained an unsupported command, environment dependency or organization pin")
    if not re.search(r"(?m)^baseInputsPath: testdata\s*$", definition):
        raise ValueError("prototype requires scenario-local testdata inputs")
    for match in re.finditer(r"(?m)^\s*stdoutFile:\s*(.+)$", definition):
        if not re.fullmatch(r'"\{\{ \.workdir \}\}/[a-zA-Z0-9_-]+\.(json|yaml)"', match[1]):
            raise ValueError("replay requires workdir-local stdout files")
    for match in re.finditer(r"(?m)^( +)inputOverlayDirs:\n((?:\1  - [^\n]+\n)+)", definition):
        for line in match[2].splitlines():
            relative = line.strip().removeprefix("- ")
            if not re.fullmatch(r"overlays/[a-zA-Z0-9_-]+", relative) or not (directory / relative).is_dir():
                raise ValueError("replay requires scenario-local overlay directories")
    if len(re.findall(r"(?m)^\s*inputOverlayDirs:", definition)) != len(list(re.finditer(
        r"(?m)^( +)inputOverlayDirs:\n((?:\1  - [^\n]+\n)+)", definition
    ))):
        raise ValueError("unsupported overlay declaration")
    for path in directory.rglob("*"):
        if path.relative_to(directory).parts[0] == "replay":
            continue
        if path.is_symlink():
            raise ValueError("symlinked scenario inputs are not replay-supported")
        if not path.is_file():
            continue
        content = path.read_text(encoding="utf-8")
        if any(marker in content for marker in ("../", "repo_dir", "!env", "!include", "!<", "%TAG")):
            raise ValueError("scenario gained an external input; replay dependency review is required")
        # Only plain scalar local !file references are supported. Overlays are
        # copied onto testdata, so references resolve in that merged tree.
        references = list(re.finditer(r"!file[ \t]+([a-zA-Z0-9_./-]+)(?:#[a-zA-Z0-9_.-]+)?[ \t]*(?:\n|$)", content))
        if content.count("!file") != len(references):
            raise ValueError("replay requires plain scenario-local !file references")
        for reference in references:
            relative = Path(reference[1])
            if relative.is_absolute() or ".." in relative.parts:
                raise ValueError("replay requires scenario-local !file references")
            parts = path.relative_to(directory).parts
            source = path.parent
            if parts[0] == "overlays":
                source = directory / "testdata" / Path(*parts[2:-1])
            target = source / relative
            if not target.is_file() or not target.resolve().is_relative_to((directory / "testdata").resolve()):
                raise ValueError("replay !file target must exist within scenario testdata")
        if path.name == "scenario.yaml" and any(marker in content for marker in ("https://", "http://")):
            raise ValueError("scenario gained an external command input")


def fixture_strings(directory):
    """Known public document/spec bytes, not arbitrary response substrings.

    These files are fingerprinted with the scenario. Preserve exact payloads
    containing example credentials/emails; never exempt a modified payload.
    """
    strings, specs = set(), set()
    for path in (directory / "testdata").rglob("*"):
        if not path.is_file() or path.suffix not in (".md", ".yaml", ".json"):
            continue
        content = path.read_text(encoding="utf-8")
        strings.add(content)
        if re.search(r'(?m)^openapi:|^swagger:', content):
            import yaml
            class FixtureLoader(yaml.SafeLoader):
                pass
            # sigs.k8s.io/yaml leaves timestamp scalars as strings for JSON.
            # PyYAML otherwise creates datetime objects before serialization.
            FixtureLoader.add_constructor("tag:yaml.org,2002:timestamp", FixtureLoader.construct_scalar)
            try:
                spec = yaml.load(content, Loader=FixtureLoader)
                specs.add(canonical(spec))
            except (yaml.YAMLError, TypeError, ValueError):
                raise ValueError("public OpenAPI fixture must be valid JSON-compatible YAML") from None
        elif path.suffix == ".json":
            spec = parse_json(content)
            if isinstance(spec, dict) and ("openapi" in spec or "swagger" in spec):
                specs.add(canonical(spec))
    return PublicFixtures(strings, specs)


class PublicFixtures:
    """Exact whole strings or whole OpenAPI objects; never individual fields."""

    def __init__(self, strings, specs):
        self.strings, self.specs = strings, specs

    def __contains__(self, value):
        if value in self.strings:
            return True
        if self.specs and value.lstrip().startswith("{"):
            try:
                return canonical(parse_json(value)) in self.specs
            except ValueError:
                pass
        return False


def check_safe(value, fixtures=frozenset(), location="exchange"):
    """Fail closed on sensitive fields; never persist arbitrary HTTP headers."""
    if isinstance(value, dict):
        for key, item in value.items():
            if SENSITIVE_KEY.search(key):
                raise ValueError(f"sensitive field at {location}: cassette requires explicit sanitizer review")
            # Only known structural names enter diagnostics, never arbitrary
            # upstream keys, values, hashes or raw HTTP payloads.
            field = key if key in {"request", "response", "body", "content", "spec", "description", "data"} else "field"
            check_safe(item, fixtures, location + "." + field)
    elif isinstance(value, list):
        for item in value:
            check_safe(item, fixtures, location + "[]")
    elif isinstance(value, str):
        if value in fixtures:
            return
        if SENSITIVE_VALUE.search(value):
            raise ValueError(f"sensitive value at {location}: refusing to publish cassette")
        for match in KONG_HOST.finditer(value):
            host = match.group().lower()
            if host not in HOSTS and not re.fullmatch(r"replay\.[a-z0-9-]+\.(cp|tp)\.konghq\.com", host):
                raise ValueError("unsanitized Kong hostname: refusing to publish cassette")


class Sanitizer:
    """Recording-only normalization. Replay requests are NOT wildcard-normalized."""

    def __init__(self, fixtures=frozenset()):
        self.ids = {}
        self.fixtures = fixtures

    def normalize(self, value):
        if isinstance(value, dict):
            return {key: self.normalize(item) for key, item in value.items()}
        if isinstance(value, list):
            return [self.normalize(item) for item in value]
        if isinstance(value, str):
            if value in self.fixtures:
                return value
            def replace(match):
                key = match.group().lower()
                if key not in self.ids:
                    self.ids[key] = f"00000000-0000-4000-8000-{len(self.ids) + 1:012d}"
                return self.ids[key]
            value = UUID.sub(replace, value)
            return GENERATED_HOST.sub(lambda match: f"replay.{match[1].lower()}.{match[2].lower()}.konghq.com", value)
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


def validate_cassette(cassette, directory, scenario=SCENARIO):
    check_eligibility(directory)
    fixtures = fixture_strings(directory)
    if not isinstance(cassette, dict) or set(cassette) != {"schema_version", "scenario", "inputs_sha256", "source", "interactions"}:
        raise ValueError("invalid cassette fields")
    if type(cassette["schema_version"]) is not int or cassette["schema_version"] != 1 or cassette["scenario"] != scenario:
        raise ValueError("unsupported cassette schema or scenario")
    if cassette["inputs_sha256"] != scenario_digest(directory):
        raise ValueError(f"stale cassette: {scenario}; re-record and review, or remove replay eligibility")
    source = cassette["source"]
    if not isinstance(source, dict) or not isinstance(source.get("commit"), str) or not re.fullmatch(r"[0-9a-f]{40}", source["commit"]):
        raise ValueError("cassette must identify its recorded commit")
    if set(source) != {"kind", "commit", "run_url"}:
        raise ValueError("invalid provenance fields")
    if source.get("kind") != "recorded":
        raise ValueError("cassette must be a live recording, not a bootstrap fixture")
    if not isinstance(source.get("run_url"), str) or not re.fullmatch(r"https://github.com/[Kk]ong/kongctl/actions/runs/[0-9]+", source["run_url"]):
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
        if request["endpoint"] not in HOSTS.values() or request["method"] not in ("GET", "POST", "PUT", "PATCH", "DELETE"):
            raise ValueError("unsupported endpoint or HTTP method")
        if not isinstance(request["path"], str) or not request["path"].startswith("/"):
            raise ValueError("invalid request path")
        if not isinstance(request["query"], list) or any(
            not isinstance(pair, list) or len(pair) != 2 or any(not isinstance(x, str) for x in pair)
            for pair in request["query"]
        ):
            raise ValueError("invalid request query")
        normalized = request_key(request["endpoint"], request["method"], request["path"],
                                 canonical(request["body"]).encode() if request["body"] is not None else b"")
        if normalized["path"] != request["path"] or "?" in request["path"] or "#" in request["path"]:
            raise ValueError("invalid request path")
        normalized["query"] = sorted(request["query"])
        if canonical(normalized) != canonical(request):
            raise ValueError("request is not in canonical matching form")
        if not isinstance(response, dict) or set(response) != {"status", "body"} or type(response["status"]) is not int:
            raise ValueError("invalid response fields")
        if not 200 <= response["status"] < 500 or response["status"] == 429 or 300 <= response["status"] < 400:
            raise ValueError("redirects and transient failures are not supported in cassettes")
        check_safe(item, fixtures)
        canonical(item)


class Replay:
    def __init__(self, cassette=None, token=None, fixtures=frozenset()):
        self.interactions = cassette["interactions"] if cassette else []
        self.token = token
        self.position = 0
        self.errors = []
        self.lock = threading.Lock()
        self.fixtures = fixtures
        self.sanitizer = Sanitizer(fixtures)

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
                if content and response["body"] is None:
                    raise ValueError("explicit JSON null responses are unsupported")
            finally:
                connection.close()
            item = self.sanitizer.normalize({"request": request, "response": response})
            if self.token in canonical(item):
                raise ValueError("credential echoed in response; refusing recording")
            check_safe(item, self.fixtures)
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
        try:
            super().send_error(code, "replay request rejected", "See replay diagnostics")
        except OSError:
            self.close_connection = True


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
            self.connection.settimeout(90)
            with self.server.tls.wrap_socket(self.connection, server_side=True) as connection:
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


def clean_environment(directory, binary, scenario=SCENARIO):
    # Do not inherit profiles, credentials, proxy bypasses, filters, skip flags,
    # shard configuration, debug capture switches or user CLI settings.
    env = {key: os.environ[key] for key in ("PATH", "TMPDIR", "SYSTEMROOT") if key in os.environ}
    env.update({
        "DO_NOT_TRACK": "1", "XDG_CONFIG_HOME": str(directory / "config"),
        "KONGCTL_E2E_BIN": str(binary), "KONGCTL_E2E_ARTIFACTS_DIR": str(directory / "artifacts"),
        "KONGCTL_E2E_SCENARIO": scenario, "KONGCTL_E2E_KONNECT_ENV": "com",
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
    parser.add_argument("--scenario", choices=SCENARIOS, default=SCENARIO)
    parser.add_argument("--cassette", type=Path)
    parser.add_argument("--binary", type=Path, default=ROOT / "kongctl")
    parser.add_argument("--test-binary", type=Path, default=ROOT / "e2e.test")
    parser.add_argument("--reset-binary", type=Path, default=ROOT / "reset-org.test")
    parser.add_argument("--output-dir", type=Path, default=ROOT / ".e2e-artifacts/replay")
    parser.add_argument("--require-isolated", action="store_true", help="require a Linux loopback-only network namespace")
    args = parser.parse_args()
    directory = ROOT / "test/e2e/scenarios" / args.scenario
    args.cassette = args.cassette or directory / "replay/cassette.json"
    if args.require_isolated and (sys.platform != "linux" or {name for _, name in socket.if_nameindex()} != {"lo"}):
        raise ValueError("replay requires a network namespace containing only loopback")
    if args.mode == "record" and args.require_isolated:
        raise ValueError("recording requires live networking, not replay isolation")
    cassette = None
    if args.mode != "record":
        cassette = parse_json(args.cassette.read_bytes())
        validate_cassette(cassette, directory, args.scenario)
        if args.mode == "check":
            print(f"Cassette inputs and schema valid: {args.scenario}")
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
    engine = Replay(cassette, token, fixture_strings(directory))
    with tempfile.TemporaryDirectory(prefix="kongctl-replay-") as temporary:
        private = Path(temporary)
        env = clean_environment(private, args.binary.resolve(), args.scenario)
        reset_env = {**env, "KONGCTL_E2E_KONNECT_PAT": token or DUMMY_PAT, "KONGCTL_E2E_RESET": "1"}
        started = time.monotonic()
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
            if result.returncode or f"--- PASS: Test_Scenarios/test/e2e/scenarios/{args.scenario}/scenario.yaml" not in output:
                raise ValueError("scenario did not pass; candidate not published (raw live logs are not uploaded)")
        finally:
            if token:
                subprocess.run([str(args.reset_binary.resolve()), "--stage", "after-replay-record"],
                               env=reset_env, check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, timeout=600)
        elapsed = time.monotonic() - started
    if token:
        candidate = {"schema_version": 1, "scenario": args.scenario, "inputs_sha256": scenario_digest(directory),
                     "source": {"kind": "recorded", "commit": os.environ.get("GITHUB_SHA", ""),
                                "run_url": "https://github.com/Kong/kongctl/actions/runs/" + os.environ.get("GITHUB_RUN_ID", "")},
                     "interactions": engine.interactions}
        validate_cassette(candidate, directory, args.scenario)
        # Never overwrite the reviewed cassette, even when recording succeeded.
        write_json(args.output_dir / "candidate-cassette.json", candidate)
    summary = {"mode": args.mode, "scenario": args.scenario, "interactions": engine.position,
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
