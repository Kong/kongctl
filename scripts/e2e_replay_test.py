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


SPEC = importlib.util.spec_from_file_location("e2e_replay", Path(__file__).with_name("e2e_replay.py"))
MODULE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(MODULE)


def interaction(method="GET", body=None):
    return {"request": MODULE.request_key("regional", method, "/v2/control-planes?a=1&b=2", body),
            "response": {"status": 200, "body": {"data": []}}}


class ReplayTest(unittest.TestCase):
    def test_inline_overlays_are_local_literal_and_fingerprinted(self):
        import yaml
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            (root / "testdata").mkdir()
            (root / "testdata/portal.yaml").write_text("portals: []\n")
            operation = {"file": "portal.yaml", "match": "portals[?ref=='portal'] | [0]",
                         "set": {"default_api_visibility": "private", "auto_approve_applications": True}}
            definition = {"baseInputsPath": "testdata", "steps": [{"inputOverlayOps": [operation]}]}
            path = root / "scenario.yaml"
            path.write_text(yaml.safe_dump(definition, indent=2))
            MODULE.check_eligibility(root)
            before = MODULE.scenario_digest(root)
            operation["set"]["default_api_visibility"] = "public"
            path.write_text(yaml.safe_dump(definition, indent=2))
            MODULE.check_eligibility(root)
            self.assertNotEqual(before, MODULE.scenario_digest(root))
            invalid = [
                {"file": "../portal.yaml"}, {"file": "/etc/passwd"}, {"file": "missing.yaml"},
                {"file": "https://example.com/portal.yaml"}, {"file": "{{ .vars.file }}"},
                {"match": "portals[?ref=='{{ .vars.ref }}'] | [0]"}, {"match": "portals[*]"},
                {"set": {"visibility": {"from": "file"}}}, {"set": {"visibility": "!file doc.md"}},
                {"set": {"visibility": "{{ .vars.visibility }}"}}, {"set": {"a.b": "private"}},
                {"set": {}}, {"append": {"visibility": "private"}},
            ]
            for change in invalid:
                definition["steps"][0]["inputOverlayOps"] = [{**operation, **change}]
                path.write_text(yaml.safe_dump(definition, indent=2))
                with self.subTest(change=change), self.assertRaises(ValueError):
                    MODULE.check_eligibility(root)

    def test_inline_overlay_declarations_cannot_hide_in_yaml(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            (root / "testdata").mkdir()
            (root / "testdata/portal.yaml").write_text("portals: []\n")
            operation = "      - file: portal.yaml\n        match: portals[?ref=='p'] | [0]\n        set: {visibility: private}\n"
            header = "baseInputsPath: testdata\nsteps:\n  - name: example\n"
            for text in [
                header + "    inputOverlayOps: []\n",
                header + "    inputOverlayOps: []\n    inputOverlayOps:\n" + operation,
                "baseInputsPath: testdata\ninputOverlayOps: []\n",
                header + "    commands:\n      - inputOverlayOps: []\n",
                header + "    inputOverlayOps: &ops []\n",
                header + "    inputOverlayOps: !custom []\n",
                header + "    inputOverlayOpsFiles: [operations.yaml]\n",
            ]:
                (root / "scenario.yaml").write_text(text)
                with self.subTest(text=text), self.assertRaises(ValueError):
                    MODULE.check_eligibility(root)

    def test_chunked_cassette_round_trip_and_tamper_detection(self):
        cassette = {"schema_version": 2, "interactions": [interaction(body=None)] * 3}
        cassette["interactions"][0]["response"]["body"] = "x" * 180000
        with tempfile.TemporaryDirectory() as temporary:
            directory = Path(temporary) / "packed"
            MODULE.pack_cassette(cassette, directory)
            path = directory / "cassette.json"
            self.assertEqual(MODULE.load_cassette(path), cassette)
            manifest = json.loads(path.read_text())
            self.assertGreater(len(manifest["interaction_chunks"]), 1)
            with self.assertRaises(FileExistsError):
                MODULE.pack_cassette(cassette, directory)
            chunk = next((directory / "interactions").iterdir())
            chunk.write_text("[]\n")
            with self.assertRaisesRegex(ValueError, "digest mismatch"):
                MODULE.load_cassette(path)
            manifest["interaction_chunks"] = ["../outside"]
            path.write_text(json.dumps(manifest))
            with self.assertRaisesRegex(ValueError, "invalid cassette chunk digest"):
                MODULE.load_cassette(path)

    def test_chunked_cassette_rejects_symlinks_and_oversized_interactions(self):
        cassette = {"schema_version": 2, "interactions": [interaction()]}
        with tempfile.TemporaryDirectory() as temporary:
            directory = Path(temporary) / "packed"
            MODULE.pack_cassette(cassette, directory)
            chunks = directory / "interactions"
            chunks.rename(directory / "elsewhere")
            chunks.symlink_to(directory / "elsewhere", target_is_directory=True)
            with self.assertRaisesRegex(ValueError, "symlinked"):
                MODULE.load_cassette(directory / "cassette.json")
            cassette["interactions"][0]["response"]["body"] = "x" * 400000
            with self.assertRaisesRegex(ValueError, "single interaction"):
                MODULE.pack_cassette(cassette, Path(temporary) / "oversized")

    def test_parallel_phase_preserves_ids_bodies_and_barriers(self):
        identifier = "00000000-0000-4000-8000-000000000001"
        def exchange(method, path, body=None, response=None):
            return {"request": MODULE.request_key("regional", method, path,
                                                  json.dumps(body).encode() if body else b""),
                    "response": {"status": 201 if method == "POST" else 200, "body": response}}
        items = [exchange("POST", "/parents", {"name": "a"}, {"id": identifier}),
                 exchange("POST", "/parents", {"name": "b"}, {"id": "00000000-0000-4000-8000-000000000002"}),
                 exchange("POST", "/parents/" + identifier + "/children", {"name": "child"}),
                 exchange("GET", "/parents")]
        cassette = {"interactions": items, "parallel_phases": [{"start": 1, "end": 3, "after": {}}]}
        engine = MODULE.Replay(cassette)
        def send(item):
            request = item["request"]
            return engine.exchange("us.api.konghq.com", request["method"], request["path"],
                                   json.dumps(request["body"]).encode() if request["body"] else b"")
        with self.assertRaises(ValueError):
            send(items[2])  # No child before its parent exists.
        changed = copy.deepcopy(items[1])
        changed["request"]["body"]["name"] = "different"
        with self.assertRaises(ValueError):
            send(changed)
        send(items[1])  # Independent parent creates can reverse.
        send(items[0])
        with self.assertRaises(ValueError):
            send(items[3])  # No crossing into the next phase.
        send(items[2])
        send(items[3])
        engine.verify()

    def test_parallel_phase_keeps_observations_before_mutations(self):
        items = [interaction(), interaction("DELETE"), interaction("POST", b'{"name":"new"}')]
        cassette = {"interactions": items, "parallel_phases": [{"start": 1, "end": 3, "after": {}}]}
        phase = MODULE.parallel_phases(cassette)[0]
        self.assertIn(0, phase[1])
        for invalid in [
            [{"start": 1, "end": 3, "after": {"1": [2]}}],
            [{"start": True, "end": 3, "after": {}}],
            [{"start": 1, "end": 3, "after": {}}, {"start": 2, "end": 3, "after": {}}],
        ]:
            with self.subTest(invalid=invalid), self.assertRaises(ValueError):
                MODULE.parallel_phases({**cassette, "parallel_phases": invalid})

    def test_failure_diagnostics_never_include_raw_cli_output(self):
        engine = MODULE.Replay({"interactions": []})
        directory = MODULE.ROOT / "test/e2e/scenarios/portal/sync"
        summary = MODULE.failure_summary(engine, "command 000-sync failed (exit=1): kpat_PRIVATE\n"
                                         "stderr: password=PRIVATE", directory)
        self.assertNotIn("PRIVATE", summary)
        self.assertEqual([{"name": "000-sync", "exit": 1}], json.loads(summary)["commands"])

    def test_supported_scenarios_have_local_dependencies(self):
        for scenario in MODULE.SCENARIOS:
            with self.subTest(scenario=scenario):
                MODULE.check_eligibility(MODULE.ROOT / "test/e2e/scenarios" / scenario)
                MODULE.fixture_strings(MODULE.ROOT / "test/e2e/scenarios" / scenario)

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
        response.getheader.return_value = "application/json"
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
        response.getheader.return_value = "application/json"
        response.read.return_value = b'{"message":"private-credential"}'
        connection = MagicMock()
        connection.getresponse.return_value = response
        with patch.object(MODULE.http.client, "HTTPSConnection", return_value=connection):
            engine = MODULE.Replay(token="private-credential", fixtures=frozenset(["private-credential"]))
            with self.assertRaisesRegex(ValueError, "credential echoed"):
                engine.exchange("us.api.konghq.com", "GET", "/v2/control-planes", b"")
        self.assertEqual([], engine.interactions)

    def test_repository_cassettes_are_current_even_without_replay_routing(self):
        root = MODULE.ROOT / "test/e2e/scenarios"
        for path in sorted(root.glob("**/replay/cassette.json")):
            directory = path.parent.parent
            with self.subTest(path=path):
                MODULE.validate_cassette(MODULE.load_cassette(path), directory,
                                        directory.relative_to(root).as_posix())

    def test_external_dependencies_require_explicit_review(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            for content in ["baseInputsPath: ../shared", "exec: [curl]", "env: {}", "assignedEnvironment: other-org"]:
                (root / "scenario.yaml").write_text(content)
                with self.subTest(content=content), self.assertRaises(ValueError):
                    MODULE.check_eligibility(root)

    def test_local_file_inputs_are_bounded_and_fingerprinted(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "scenario.yaml").write_text("baseInputsPath: testdata\nsteps: []\n")
            inputs = root / "testdata"
            inputs.mkdir()
            document = inputs / "doc.md"
            document.write_text("Public docs link: https://example.com\n")
            manifest = inputs / "portal.yaml"
            manifest.write_text("content: !file ./doc.md\n")
            MODULE.check_eligibility(root)
            before = MODULE.scenario_digest(root)
            document.write_text("Changed document\n")
            self.assertNotEqual(before, MODULE.scenario_digest(root))
            for value in ["../doc.md", "/etc/passwd", "https://example.com/doc.md",
                          "missing.md", '"doc.md"', "\n  path: doc.md"]:
                manifest.write_text("content: !file " + value + "\n")
                with self.subTest(value=value), self.assertRaises(ValueError):
                    MODULE.check_eligibility(root)
            manifest.write_text("content: !file doc.md\n")
            document.unlink()
            document.symlink_to(root / "scenario.yaml")
            with self.assertRaises(ValueError):
                MODULE.check_eligibility(root)

    def test_public_fixture_exemption_is_exact_and_preserves_literal_ids(self):
        public = "Example: Bearer EXAMPLE, author@example.com, aabbccdd-1234-4567-8901-aabbccddeeff\n"
        fixtures = frozenset([public])
        MODULE.check_safe({"content": public}, fixtures)
        self.assertEqual(public, MODULE.Sanitizer(fixtures).normalize(public))
        for value in [public + "extra", "author@example.com", {"token": public}]:
            with self.subTest(value=value), self.assertRaises(ValueError):
                MODULE.check_safe(value, fixtures)

    def test_yaml_spec_serialized_as_json_remains_a_public_fixture(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "testdata").mkdir()
            (root / "testdata/spec.yaml").write_text(
                'openapi: "3.0.0"\ninfo:\n  contact:\n    email: public@example.com\n'
                'example: 2026-09-10T12:00:00Z\n')
            fixtures = MODULE.fixture_strings(root)
            wire = ('{"info":{"contact":{"email":"public@example.com"}},"openapi":"3.0.0",'
                    '"example":"2026-09-10T12:00:00Z"}')
            MODULE.check_safe({"content": wire}, fixtures)
            self.assertEqual(wire, MODULE.Sanitizer(fixtures).normalize(wire))
            with self.assertRaises(ValueError):
                MODULE.check_safe({"content": wire.replace("public@", "private@")}, fixtures)

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
                     {"value": "kpat_abc"}, {"email": "person@example.com"},
                     {"value": "-----BEGIN PRIVATE KEY"}]:  # pragma: allowlist secret (rejection test, no key)
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
        handler.server.engine.fail.assert_called_once_with("TLS tunnel failed (TimeoutError)")

    def test_cancelled_idle_tls_dial_is_not_an_api_exchange(self):
        for error in (ConnectionResetError(), ssl.SSLEOFError()):
            handler = MODULE.ProxyHandler.__new__(MODULE.ProxyHandler)
            handler.path = "us.api.konghq.com:443"
            handler.connection = MagicMock()
            handler.server = MagicMock()
            handler.send_response = MagicMock()
            handler.end_headers = MagicMock()
            handler.server.tls.wrap_socket.side_effect = error
            handler.do_CONNECT()
            handler.server.engine.fail.assert_not_called()

    def test_cancelled_tls_dial_does_not_satisfy_a_required_request(self):
        handler = MODULE.ProxyHandler.__new__(MODULE.ProxyHandler)
        handler.path = "us.api.konghq.com:443"
        handler.connection = MagicMock()
        handler.server = MagicMock()
        handler.server.engine = MODULE.Replay({"interactions": [interaction()]})
        handler.send_response = MagicMock()
        handler.end_headers = MagicMock()
        handler.server.tls.wrap_socket.side_effect = ssl.SSLEOFError()
        handler.do_CONNECT()
        with self.assertRaisesRegex(ValueError, "required interactions unused"):
            handler.server.engine.verify()

    def test_other_tls_errors_remain_fatal(self):
        handler = MODULE.ProxyHandler.__new__(MODULE.ProxyHandler)
        handler.path = "us.api.konghq.com:443"
        handler.connection = MagicMock()
        handler.server = MagicMock()
        handler.server.engine = MODULE.Replay({"interactions": []})
        handler.send_response = MagicMock()
        handler.end_headers = MagicMock()
        handler.server.tls.wrap_socket.side_effect = ssl.SSLError("certificate failure")
        handler.do_CONNECT()
        with self.assertRaisesRegex(ValueError, "TLS tunnel failed"):
            handler.server.engine.verify()

    def test_reset_between_requests_is_idle_but_partial_request_is_not(self):
        handler = MODULE.InnerHandler.__new__(MODULE.InnerHandler)
        handler.rfile = MagicMock()
        handler.rfile.peek.side_effect = ConnectionResetError()
        handler.handle_one_request()
        self.assertTrue(handler.close_connection)
        handler.rfile.peek.side_effect = None
        handler.rfile.peek.return_value = b"G"
        with patch.object(MODULE.http.server.BaseHTTPRequestHandler, "handle_one_request",
                          side_effect=ConnectionResetError()):
            with self.assertRaises(ConnectionResetError):
                handler.handle_one_request()

    def test_partial_http_timeout_remains_fatal_even_after_all_exchanges(self):
        handler = MODULE.InnerHandler.__new__(MODULE.InnerHandler)
        handler.rfile = MagicMock()
        handler.rfile.peek.return_value = b"G"
        handler.rfile.readline.side_effect = TimeoutError()
        handler.server = MagicMock()
        handler.server.engine = MODULE.Replay({"interactions": []})
        handler.handle_one_request()
        with self.assertRaisesRegex(ValueError, "HTTP protocol error"):
            handler.server.engine.verify()

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
            typed = copy.deepcopy(cassette)
            typed["schema_version"] = 2
            typed["interactions"][0]["response"] = {
                "status": 404, "body": {"status": 404}, "content_type": "application/problem+json"}
            MODULE.validate_cassette(typed, root)
            for content_type in [None, "text/html", "application/json\r\nSet-Cookie: private", []]:
                invalid = copy.deepcopy(typed)
                invalid["interactions"][0]["response"]["content_type"] = content_type
                with self.subTest(content_type=content_type), self.assertRaises(ValueError):
                    MODULE.validate_cassette(invalid, root)
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

    def test_problem_json_content_type_survives_the_https_proxy(self):
        expected = interaction()
        expected["response"] = {"status": 404, "body": {"status": 404, "title": "Not Found"},
                                "content_type": "application/problem+json"}
        engine = MODULE.Replay({"interactions": [expected]})
        with tempfile.TemporaryDirectory() as directory:
            with MODULE.Server(engine, Path(directory)) as server:
                context = ssl.create_default_context(cafile=str(server.certificate))
                client = http.client.HTTPSConnection("127.0.0.1", server.httpd.server_port, context=context)
                client.set_tunnel("us.api.konghq.com", 443)
                client.request("GET", "/v2/control-planes?a=1&b=2")
                response = client.getresponse()
                self.assertEqual("application/problem+json", response.getheader("Content-Type"))
                self.assertEqual(404, response.status)
                response.read()
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
