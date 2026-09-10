from __future__ import annotations

import copy
import http.client
import importlib.util
import json
import os
from pathlib import Path
import ssl
import tempfile
import unittest
from unittest.mock import MagicMock, patch


SPEC = importlib.util.spec_from_file_location("e2e_replay", Path(__file__).with_name("e2e-replay.py"))
MODULE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(MODULE)


def interaction(method="GET", body=None):
    return {"request": MODULE.request_key("regional", method, "/v2/control-planes?a=1&b=2", body),
            "response": {"status": 200, "body": {"data": []}}}


class ReplayTest(unittest.TestCase):
    def test_supported_scenarios_have_local_dependencies(self):
        for scenario in MODULE.SCENARIOS:
            with self.subTest(scenario=scenario):
                MODULE.check_eligibility(MODULE.ROOT / "test/e2e/scenarios" / scenario)

    def test_recording_workflow_shares_live_org_lock(self):
        live = (MODULE.ROOT / ".github/workflows/e2e.yaml").read_text()
        experiment = (MODULE.ROOT / ".github/workflows/e2e-replay.yaml").read_text()
        self.assertIn("group: konnect-e2e-${{ matrix.org_name }}", live)
        self.assertIn("group: konnect-e2e-kongctl-acceptance-3", experiment)
        self.assertIn("environment: kongctl-acceptance-3", experiment)
        self.assertIn("cancel-in-progress: false", experiment)
        self.assertIn("queue: max", experiment)

    def test_recording_forwards_with_private_pat_but_only_saves_sanitized_data(self):
        response = MagicMock(status=201)
        real_id = "aabbccdd-1234-4567-8901-aabbccddeeff"
        response.read.return_value = json.dumps({"id": real_id}).encode()
        connection = MagicMock()
        connection.getresponse.return_value = response
        with patch.object(MODULE.http.client, "HTTPSConnection", return_value=connection):
            engine = MODULE.Replay(token="private-credential")
            received = engine.exchange("us.api.konghq.com", "POST", "/v2/control-planes", b'{"name":"example"}')
        self.assertEqual(real_id, received["body"]["id"])
        self.assertNotIn(real_id, json.dumps(engine.interactions))
        self.assertNotIn("private-credential", json.dumps(engine.interactions))
        self.assertEqual("Bearer private-credential", connection.request.call_args.kwargs["headers"]["Authorization"])
        connection.close.assert_called_once()

    def test_recording_rejects_echoed_credentials(self):
        response = MagicMock(status=200)
        response.read.return_value = b'{"message":"private-credential"}'
        connection = MagicMock()
        connection.getresponse.return_value = response
        with patch.object(MODULE.http.client, "HTTPSConnection", return_value=connection):
            engine = MODULE.Replay(token="private-credential")
            with self.assertRaisesRegex(ValueError, "credential echoed"):
                engine.exchange("us.api.konghq.com", "GET", "/v2/control-planes", b"")
        self.assertEqual([], engine.interactions)

    def test_repository_cassettes_are_current_even_without_replay_routing(self):
        directory = MODULE.ROOT / "test/e2e/scenarios" / MODULE.SCENARIO
        for path in sorted((directory / "replay").glob("*.json")):
            with self.subTest(path=path):
                MODULE.validate_cassette(MODULE.parse_json(path.read_bytes()), directory)

    def test_external_dependencies_require_explicit_review(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            for content in ["baseInputsPath: ../shared", "exec: [curl]", "env: {}", "assignedEnvironment: other-org"]:
                (root / "scenario.yaml").write_text(content)
                with self.subTest(content=content), self.assertRaises(ValueError):
                    MODULE.check_eligibility(root)

    def test_matches_query_order_and_json_structurally(self):
        engine = MODULE.Replay({"interactions": [interaction("POST", b'{"a":1,"b":2}') ]})
        response = engine.exchange("us.api.konghq.com", "POST", "/v2/control-planes?b=2&a=1", b'{"b":2, "a":1}')
        self.assertEqual(200, response["status"])
        engine.verify()

    def test_rejects_semantic_changes_without_consuming_expectation(self):
        for method, target, body in [
            ("DELETE", "/v2/control-planes?a=1&b=2", b'{"a":1}'),
            ("POST", "/wrong?a=1&b=2", b'{"a":1}'),
            ("POST", "/v2/control-planes?a=1&b=3", b'{"a":1}'),
            ("POST", "/v2/control-planes?a=1&b=2", b'{"a":true}'),
            ("POST", "/v2/control-planes?a=1&b=2", b'{}'),
        ]:
            with self.subTest(method=method, target=target, body=body):
                engine = MODULE.Replay({"interactions": [interaction("POST", b'{"a":1}')]})
                with self.assertRaisesRegex(ValueError, "mismatch"):
                    engine.exchange("us.api.konghq.com", method, target, body)
                self.assertEqual(0, engine.position)

    def test_rejects_extra_and_unused_interactions(self):
        engine = MODULE.Replay({"interactions": [interaction()]})
        with self.assertRaisesRegex(ValueError, "unused"):
            engine.verify()
        engine.exchange("us.api.konghq.com", "GET", "/v2/control-planes?a=1&b=2", b"")
        with self.assertRaisesRegex(ValueError, "extra"):
            engine.exchange("us.api.konghq.com", "GET", "/v2/control-planes?a=1&b=2", b"")

    def test_no_body_and_json_null_are_not_equivalent(self):
        # The initial format uses null for an absent body: explicitly reject a
        # JSON null body rather than silently weakening the matching contract.
        with self.assertRaises(ValueError):
            MODULE.request_key("regional", "POST", "/v2/control-planes", b"null")

    def test_strict_json(self):
        for value in ['{"a":1,"a":2}', '{"n":NaN}', '{"n":Infinity}']:
            with self.assertRaises(ValueError):
                MODULE.parse_json(value)

    def test_sanitizer_preserves_identifier_relationships_not_wildcards(self):
        real = "aabbccdd-1234-4567-8901-aabbccddeeff"
        sanitizer = MODULE.Sanitizer()
        output = sanitizer.normalize({"id": real, "path": "/items/" + real})
        self.assertNotEqual(real, output["id"])
        self.assertEqual("/items/" + output["id"], output["path"])
        expected = interaction("POST", json.dumps(output).encode())
        engine = MODULE.Replay({"interactions": [expected]})
        with self.assertRaisesRegex(ValueError, "mismatch"):
            engine.exchange("us.api.konghq.com", "POST", "/v2/control-planes?a=1&b=2",
                            json.dumps({"id": real, "path": "/items/" + real}).encode())

    def test_sensitive_content_cannot_be_published(self):
        for data in [{"client_secret": "x"}, {"a": [{"password": "x"}]},
                     {"value": "kpat_abc"}, {"email": "person@example.com"}, {"value": "-----BEGIN PRIVATE KEY"}]:
            with self.subTest(data=data), self.assertRaises(ValueError):
                MODULE.check_safe(data)

    def test_generated_hostnames_are_sanitized_across_labels_and_regions(self):
        for host in ["org-123.us.cp.konghq.com", "extra.org-123.eu.tp.konghq.com",
                     "ORG-123.AP-SOUTHEAST-2.CP.KONGHQ.COM"]:
            with self.subTest(host=host):
                value = MODULE.Sanitizer().normalize("https://" + host + ":443/path")
                region, kind = host.lower().split(".")[-4:-2]
                self.assertEqual(f"https://replay.{region}.{kind}.konghq.com:443/path", value)
                MODULE.check_safe(value)
                with self.assertRaisesRegex(ValueError, "unsanitized Kong hostname"):
                    MODULE.check_safe(host)
        for host in ["org-123.unknown.konghq.com", "replay.org.us.cp.konghq.com"]:
            with self.subTest(host=host), self.assertRaisesRegex(ValueError, "unsanitized Kong hostname"):
                MODULE.check_safe(host)
        for host in MODULE.HOSTS:
            MODULE.check_safe(host)

    def test_tls_timeout_is_set_before_handshake(self):
        handler = MODULE.ProxyHandler.__new__(MODULE.ProxyHandler)
        handler.path = "us.api.konghq.com:443"
        handler.connection = MagicMock()
        handler.server = MagicMock()
        handler.send_response = MagicMock()
        handler.end_headers = MagicMock()

        def stalled_handshake(connection, **kwargs):
            connection.settimeout.assert_called_once_with(90)
            raise TimeoutError()

        handler.server.tls.wrap_socket.side_effect = stalled_handshake
        handler.do_CONNECT()
        handler.server.engine.fail.assert_called_once_with("TLS tunnel failed")

    def test_error_response_on_disconnected_socket_is_quiet_but_fatal(self):
        handler = MODULE.InnerHandler.__new__(MODULE.InnerHandler)
        handler.server = MagicMock()
        handler.server.engine = MODULE.Replay({"interactions": []})
        with patch.object(MODULE.http.server.BaseHTTPRequestHandler, "send_error", side_effect=BrokenPipeError):
            handler.send_error(400)
        self.assertTrue(handler.close_connection)
        with self.assertRaisesRegex(ValueError, "HTTP protocol rejected"):
            handler.server.engine.verify()

    def test_environment_does_not_inherit_credentials_or_skip_switches(self):
        with patch.dict(os.environ, {"KONGCTL_DEFAULT_KONNECT_PAT": "real-secret",
                                    "KONGCTL_E2E_SKIP_STEPS": "*", "NO_PROXY": "*", "GH_TOKEN": "secret"}):
            env = MODULE.clean_environment(Path("/private"), Path("/binary"))
        self.assertNotIn("GH_TOKEN", env)
        self.assertNotIn("KONGCTL_DEFAULT_KONNECT_PAT", env)
        self.assertNotIn("KONGCTL_E2E_SKIP_STEPS", env)
        self.assertNotIn("NO_PROXY", env)
        self.assertEqual(MODULE.DUMMY_PAT, env["KONGCTL_E2E_KONNECT_PAT"])

    def test_input_digest_covers_add_change_delete_but_not_cassette(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "scenario.yaml").write_text("baseInputsPath: testdata\nsteps: []")
            first = MODULE.scenario_digest(root)
            (root / "input.yaml").write_text("a: 1")
            second = MODULE.scenario_digest(root)
            self.assertNotEqual(first, second)
            (root / "input.yaml").write_text("a: 2")
            self.assertNotEqual(second, MODULE.scenario_digest(root))
            (root / "input.yaml").unlink()
            self.assertEqual(first, MODULE.scenario_digest(root))
            (root / "replay").mkdir()
            (root / "replay/cassette.json").write_text("{}")
            self.assertEqual(first, MODULE.scenario_digest(root))
            (root / "external.yaml").symlink_to(root / "scenario.yaml")
            with self.assertRaisesRegex(ValueError, "symlink"):
                MODULE.scenario_digest(root)

    def test_cassette_schema_and_staleness(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "scenario.yaml").write_text("baseInputsPath: testdata\nsteps: []")
            cassette = {"schema_version": 1, "scenario": MODULE.SCENARIO,
                        "inputs_sha256": MODULE.scenario_digest(root),
                        "source": {"kind": "recorded", "commit": "a" * 40,
                                   "run_url": "https://github.com/Kong/kongctl/actions/runs/123"},
                        "interactions": [interaction()]}
            MODULE.validate_cassette(cassette, root)
            for path in ["/x#frag", "/x#", "/x?q=1", "/x?", "//other/x", "/x\ny"]:
                invalid = copy.deepcopy(cassette)
                invalid["interactions"][0]["request"]["path"] = path
                with self.subTest(path=path), self.assertRaises(ValueError):
                    MODULE.validate_cassette(invalid, root)
            invalid = copy.deepcopy(cassette)
            invalid["interactions"][0]["request"]["query"].reverse()
            with self.assertRaisesRegex(ValueError, "canonical matching form"):
                MODULE.validate_cassette(invalid, root)
            bootstrap = copy.deepcopy(cassette)
            bootstrap["source"]["kind"] = "bootstrap"
            with self.assertRaisesRegex(ValueError, "must be a live recording"):
                MODULE.validate_cassette(bootstrap, root)
            for key, value in [("schema_version", 2), ("interactions", []), ("inputs_sha256", "stale")]:
                invalid = {**cassette, key: value}
                with self.subTest(key=key), self.assertRaises(ValueError):
                    MODULE.validate_cassette(invalid, root)
            for status in [301, 429, 500]:
                invalid = copy.deepcopy(cassette)
                invalid["interactions"][0]["response"]["status"] = status
                with self.subTest(status=status), self.assertRaises(ValueError):
                    MODULE.validate_cassette(invalid, root)
            (root / "scenario.yaml").write_text("baseInputsPath: testdata\nsteps: [changed]")
            with self.assertRaisesRegex(ValueError, "stale cassette"):
                MODULE.validate_cassette(cassette, root)

    def test_real_https_connect_and_certificate_validation(self):
        engine = MODULE.Replay({"interactions": [interaction()]})
        with tempfile.TemporaryDirectory() as directory:
            with MODULE.Server(engine, Path(directory)) as server:
                context = ssl.create_default_context(cafile=str(server.certificate))
                client = http.client.HTTPSConnection("127.0.0.1", server.httpd.server_port, context=context)
                client.set_tunnel("us.api.konghq.com", 443)
                client.request("GET", "/v2/control-planes?b=2&a=1")
                response = client.getresponse()
                self.assertEqual(200, response.status)
                self.assertEqual({"data": []}, json.loads(response.read()))
                client.close()
                engine.verify()

    def test_unknown_destination_is_blocked_without_forwarding(self):
        engine = MODULE.Replay({"interactions": []})
        with tempfile.TemporaryDirectory() as directory:
            with MODULE.Server(engine, Path(directory)) as server:
                client = http.client.HTTPConnection("127.0.0.1", server.httpd.server_port)
                client.request("CONNECT", "example.com:443")
                response = client.getresponse()
                self.assertEqual(403, response.status)
                response.read()
                client.close()
        with self.assertRaisesRegex(ValueError, "blocked CONNECT"):
            engine.verify()


if __name__ == "__main__":
    unittest.main()
