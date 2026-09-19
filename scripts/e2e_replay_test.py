from __future__ import annotations

import copy
from concurrent.futures import ThreadPoolExecutor
import http.client
import importlib.util
import json
import os
import re
from pathlib import Path
import ssl
import tempfile
import threading
import unittest
from urllib.parse import urlencode
from unittest.mock import MagicMock, patch


SPEC = importlib.util.spec_from_file_location("e2e_replay", Path(__file__).with_name("e2e_replay.py"))
MODULE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(MODULE)


def interaction(method="GET", body=None):
    return {"request": MODULE.request_key("regional", method, "/v2/control-planes?a=1&b=2", body),
            "response": {"status": 200, "body": {"data": []}}}


def cassette_refresh_scenario(env):
    """Only the manual candidate workflow may defer one old input fingerprint."""
    scenario = env.get("KONGCTL_REPLAY_REFRESH_SCENARIO")
    if not scenario:
        return None
    mode, source = env.get("REPLAY_MODE"), env.get("SOURCE_RUN", "")
    if (env.get("GITHUB_EVENT_NAME") != "workflow_dispatch"
            or env.get("GITHUB_WORKFLOW") != "E2E replay experiment"
            or scenario not in MODULE.SCENARIOS
            or scenario != env.get("REPLAY_SCENARIO")
            or not (mode == "record" and not source
                    or mode == "replay" and re.fullmatch(r"[0-9]+", source))):
        raise ValueError("cassette refresh requires the selected manual recording or source-run replay")
    return scenario


class ReplayTest(unittest.TestCase):
    def portal_lookup_cassette(self):
        portal = "00000000-0000-4000-8000-000000000001"
        lookup = {"request": MODULE.request_key("regional", "GET", "/v3/portals?page[number]=1&page[size]=100", b""),
                  "response": {"status": 200, "body": {"data": [{"id": portal, "name": "portal",
                      "updated_at": "2026-09-19T00:00:00Z"}], "meta": {"page": {"total": 1}}}}}
        update = {"request": MODULE.request_key("regional", "PATCH", f"/v3/portals/{portal}/customization",
                                              b'{"layout":"topnav"}'),
                  "response": {"status": 200, "body": {"layout": "topnav"}}}
        after = copy.deepcopy(lookup)
        after["response"]["body"]["data"][0]["updated_at"] = "2026-09-19T00:00:01Z"
        return {"interactions": [lookup, update, after], "parallel_phases": [{"start": 1, "end": 3, "after": {}}]}

    def test_portal_lookup_can_precede_child_update_without_a_dependency_cycle(self):
        cassette = self.portal_lookup_cassette()
        engine = MODULE.Replay(cassette)
        lookup = "/v3/portals?page[number]=1&page[size]=100"
        engine.exchange("us.api.konghq.com", "GET", lookup, b"")
        with patch.object(MODULE, "DEPENDENCY_WAIT_SECONDS", 0):
            response = engine.exchange("us.api.konghq.com", "GET", lookup, b"")
        self.assertEqual(cassette["interactions"][2]["response"], response)
        update = cassette["interactions"][1]["request"]
        engine.exchange("us.api.konghq.com", "PATCH", update["path"], json.dumps(update["body"]).encode())
        engine.verify()
        self.assertIn(0, engine.phases[0][2])  # Repeated lookup stream stays ordered.
        cassette["parallel_phases"][0]["after"] = {"3": [2]}
        self.assertIn(1, MODULE.parallel_phases(cassette)[0][2])  # Explicit constraints are never removed.

    def test_portal_lookup_exception_requires_unchanged_reviewed_inventory(self):
        for mutation in ["field", "metadata", "timestamp", "missing-portal", "duplicate-portal", "query",
                         "body", "read-status", "write-status", "method", "path", "endpoint", "write-query",
                         "write-body"]:
            cassette = self.portal_lookup_cassette()
            before, update, after = cassette["interactions"]
            if mutation == "field":
                after["response"]["body"]["data"][0]["name"] = "changed"
            elif mutation == "metadata":
                after["response"]["body"]["meta"]["page"]["total"] = 2
            elif mutation == "timestamp":
                after["response"]["body"]["data"][0]["updated_at"] = "unreviewed"
            elif mutation == "missing-portal":
                before["response"]["body"]["data"] = after["response"]["body"]["data"] = []
            elif mutation == "duplicate-portal":
                after["response"]["body"]["data"] *= 2
            elif mutation == "query":
                before["request"]["query"].append(["filter", "changed"])
                after["request"]["query"].append(["filter", "changed"])
            elif mutation == "body":
                before["request"]["body"] = after["request"]["body"] = {}
            elif mutation == "read-status":
                after["response"]["status"] = 404
            elif mutation == "write-status":
                update["response"]["status"] = 400
            elif mutation == "method":
                update["request"]["method"] = "DELETE"
            elif mutation == "path":
                update["request"]["path"] = update["request"]["path"].replace("customization", "unreviewed")
            elif mutation == "endpoint":
                for item in cassette["interactions"]:
                    item["request"]["endpoint"] = "global"
            elif mutation == "write-body":
                update["request"]["body"] = None
            else:
                update["request"]["query"] = [["extra", "value"]]
            with self.subTest(mutation=mutation):
                self.assertIn(1, MODULE.parallel_phases(cassette)[0][2])
        cassette = self.portal_lookup_cassette()
        cassette["parallel_phases"][0]["start"] = 2
        self.assertIn(1, MODULE.parallel_phases(cassette)[1][2])  # No before/after evidence within this phase.

    def test_portal_owned_round_trip_replays_resets_without_user_or_create_permissions(self):
        directory = MODULE.ROOT / "test/e2e/scenarios/dump/portal-owned"
        MODULE.check_eligibility(directory)
        scenario = MODULE.parse_scenario((directory / "scenario.yaml").read_text())
        self.assertEqual(2, sum(command.get("resetOrg") is True
                                for step in scenario["steps"] for command in step["commands"]))
        with self.assertRaisesRegex(ValueError, "mid-scenario"):
            MODULE.check_scenario_controls(scenario)
        MODULE.check_scenario_controls(scenario, replay_resets=True)
        scenario["steps"][0]["commands"].append({"name": "unreviewed-create", "create": {
            "resource": "system-account", "payload": {"inline": {
                "name": "{{ .vars.systemAccountName }}",
                "description": "System account for organization teams dump E2E coverage"}}}})
        with self.assertRaisesRegex(ValueError, "unsupported command"):
            MODULE.check_scenario_controls(scenario, replay_resets=True)
        with patch.object(MODULE, "ROOT", Path("/separate-verifier-checkout")):
            MODULE.check_eligibility(directory)
        env = MODULE.clean_environment(Path("/private"), Path("/kongctl"), "dump/portal-owned")
        self.assertEqual("1", env["KONGCTL_E2E_RESET"])
        self.assertFalse(any("ORG_USER_EMAIL" in key for key in env))
        self.assertEqual("kongctl-acceptance-2", MODULE.recording_org("dump/portal-owned"))

    def test_round_trip_cassette_phases_preserve_dependencies_and_exact_matching(self):
        hosts = {endpoint: host for host, endpoint in MODULE.HOSTS.items()}
        for scenario in (*MODULE.USER_SCENARIOS, "dump/portal-owned"):
            with self.subTest(scenario=scenario):
                cassette = MODULE.load_cassette(MODULE.ROOT / "test/e2e/scenarios" / scenario / "replay/cassette.json")
                engine = MODULE.Replay(cassette)

                def send(index):
                    request = cassette["interactions"][index]["request"]
                    query = urlencode([tuple(pair) for pair in request["query"]])
                    target = request["path"] + ("?" + query if query else "")
                    body = json.dumps(request["body"]).encode() if request["body"] is not None else b""
                    return engine.exchange(hosts[request["endpoint"]], request["method"], target, body)

                position = 0
                for phase in cassette.get("parallel_phases", []):
                    start, end = phase["start"] - 1, phase["end"]
                    for index in range(position, start):
                        send(index)
                    dependencies = engine.phases[start]
                    for index in range(start, end):
                        if (cassette["interactions"][index]["request"]["method"] != "GET"
                                and dependencies[index] - engine.completed):
                            with patch.object(MODULE, "DEPENDENCY_WAIT_SECONDS", 0):
                                with self.assertRaisesRegex(ValueError, "dependency wait timed out"):
                                    send(index)
                            self.assertEqual(start, engine.position)
                    pending = set(range(start, end))
                    while pending:
                        index = max(i for i in pending if dependencies[i] <= engine.completed)
                        self.assertEqual(send(index), cassette["interactions"][index]["response"])
                        pending.remove(index)
                    position = end
                for index in range(position, len(cassette["interactions"])):
                    send(index)
                engine.verify()

    def test_distinct_team_membership_creates_can_reorder_but_duplicates_cannot(self):
        team = "00000000-0000-4000-8000-000000000001"
        first_user = "00000000-0000-4000-8000-000000000002"
        second_user = "00000000-0000-4000-8000-000000000003"
        path = f"/v3/teams/{team}/users"
        first = {"request": MODULE.request_key("global", "POST", path, json.dumps({"id": first_user}).encode()),
                 "response": {"status": 201, "body": None}}
        second = copy.deepcopy(first)
        second["request"]["body"]["id"] = second_user
        cassette = {"interactions": [first, second], "parallel_phases": [{"start": 1, "end": 2, "after": {}}]}
        engine = MODULE.Replay(cassette)
        engine.exchange("global.api.konghq.com", "POST", path, json.dumps({"id": second_user}).encode())
        engine.exchange("global.api.konghq.com", "POST", path, json.dumps({"id": first_user}).encode())
        engine.verify()
        # Memberships still depend on their team's creation. A phase is not
        # permission to attach users before that parent exists.
        create_team = {"request": MODULE.request_key("global", "POST", "/v3/teams", b'{"name":"team"}'),
                       "response": {"status": 201, "body": {"id": team}}}
        parent = MODULE.Replay({"interactions": [create_team, first, second],
                               "parallel_phases": [{"start": 1, "end": 3, "after": {}}]})
        with patch.object(MODULE, "DEPENDENCY_WAIT_SECONDS", 0):
            with self.assertRaisesRegex(ValueError, "dependency wait timed out"):
                parent.exchange("global.api.konghq.com", "POST", path, json.dumps({"id": second_user}).encode())
        self.assertEqual(0, parent.position)
        parent.exchange("global.api.konghq.com", "POST", "/v3/teams", b'{"name":"team"}')
        parent.exchange("global.api.konghq.com", "POST", path, json.dumps({"id": second_user}).encode())
        parent.exchange("global.api.konghq.com", "POST", path, json.dumps({"id": first_user}).encode())
        parent.verify()
        for mutation in ["duplicate", "status", "endpoint", "query", "body", "response", "path"]:
            changed = copy.deepcopy(cassette)
            items = changed["interactions"]
            if mutation == "duplicate":
                items[1] = copy.deepcopy(items[0])
            elif mutation == "status":
                items[1]["response"]["status"] = 200
            elif mutation == "endpoint":
                for item in items:
                    item["request"]["endpoint"] = "regional"
            elif mutation == "query":
                items[1]["request"]["query"] = [["extra", "value"]]
            elif mutation == "body":
                items[1]["request"]["body"]["extra"] = True
            elif mutation == "response":
                items[1]["response"]["body"] = {"data": []}
            else:
                for item in items:
                    item["request"]["path"] = "/v3/unreviewed"
            with self.subTest(mutation=mutation):
                self.assertIn(0, MODULE.parallel_phases(changed)[0][1])

    def test_portal_email_template_phases_preserve_read_before_write(self):
        cassette = MODULE.load_cassette(MODULE.ROOT /
            "test/e2e/scenarios/portal/email-templates/replay/cassette.json")
        engine = MODULE.Replay(cassette)

        def send(index):
            request = cassette["interactions"][index]["request"]
            query = urlencode([tuple(pair) for pair in request["query"]])
            target = request["path"] + ("?" + query if query else "")
            body = json.dumps(request["body"]).encode() if request["body"] is not None else b""
            return engine.exchange("us.api.konghq.com", request["method"], target, body)

        position = 0
        for phase in cassette["parallel_phases"]:
            start, end = phase["start"] - 1, phase["end"]
            for index in range(position, start):
                send(index)
            # The final portal deletion cannot cross any template phase.
            with self.assertRaisesRegex(ValueError, "mismatch"):
                send(len(cassette["interactions"]) - 1)
            dependencies = engine.phases[start]
            for index in range(start, end):
                if (cassette["interactions"][index]["request"]["method"] in {"PATCH", "DELETE"}
                        and dependencies[index]):
                    with patch.object(MODULE, "DEPENDENCY_WAIT_SECONDS", 0):
                        with self.assertRaisesRegex(ValueError, "dependency wait timed out"):
                            send(index)
                    self.assertEqual(start, engine.position)
            pending = set(range(start, end))
            while pending:
                # Prefer the opposite order while retaining mandatory dependencies.
                index = max(i for i in pending if dependencies[i] <= engine.completed)
                self.assertEqual(send(index), cassette["interactions"][index]["response"])
                pending.remove(index)
            position = end
        for index in range(position, len(cassette["interactions"])):
            send(index)
        engine.verify()

    def test_portal_teams_phases_reorder_independent_work_without_crossing_barriers(self):
        cassette = MODULE.load_cassette(MODULE.ROOT /
            "test/e2e/scenarios/portal/teams/replay/cassette.json")
        engine = MODULE.Replay(cassette)

        def send(index):
            request = cassette["interactions"][index]["request"]
            query = urlencode([tuple(pair) for pair in request["query"]])
            target = request["path"] + ("?" + query if query else "")
            body = json.dumps(request["body"]).encode() if request["body"] is not None else b""
            return engine.exchange("us.api.konghq.com", request["method"], target, body)

        position = 0
        for phase in cassette["parallel_phases"]:
            start, end = phase["start"] - 1, phase["end"]
            for index in range(position, start):
                send(index)
            # No phase may consume the request from the following strict segment.
            with self.assertRaisesRegex(ValueError, "mismatch"):
                send(end)
            self.assertEqual(start, engine.position)
            self.assertLessEqual(end - start, 3)
            methods = {item["request"]["method"] for item in cassette["interactions"][start:end]}
            self.assertIn(methods, ({"GET"}, {"POST"}))
            for index in reversed(range(start, end)):
                self.assertEqual(send(index), cassette["interactions"][index]["response"])
            position = end
        for index in range(position, len(cassette["interactions"])):
            send(index)
        engine.verify()

    def test_portal_document_patch_waits_for_delayed_api_inventory(self):
        cassette = MODULE.load_cassette(MODULE.ROOT /
            "test/e2e/scenarios/portal/api_docs_with_children/replay/cassette.json")
        engine = MODULE.Replay(cassette)

        def send(index):
            request = cassette["interactions"][index]["request"]
            query = urlencode([tuple(pair) for pair in request["query"]])
            target = request["path"] + ("?" + query if query else "")
            host = next(host for host, endpoint in MODULE.HOSTS.items() if endpoint == request["endpoint"])
            body = json.dumps(request["body"]).encode() if request["body"] is not None else b""
            return engine.exchange(host, request["method"], target, body)

        # Reproduce #2229: PATCH 132 arrives while GET 127 is still in flight.
        # The request matches exactly; only its ancestor-read dependency lags.
        for index in [*range(124), 124, 125, 127, 129]:
            send(index)
        waiting = threading.Event()
        original_wait = engine.changed.wait

        def observed_wait(timeout):
            waiting.set()
            return original_wait(timeout)

        with patch.object(engine.changed, "wait", side_effect=observed_wait), ThreadPoolExecutor(max_workers=1) as pool:
            pending = pool.submit(send, 131)
            try:
                self.assertTrue(waiting.wait(2))
                self.assertFalse(pending.done())
                self.assertEqual(engine.position, 128)
            finally:
                send(126)
            self.assertEqual(pending.result(timeout=1), cassette["interactions"][131]["response"])
        for index in [128, 130, *range(132, len(cassette["interactions"]))]:
            send(index)
        engine.verify()

    def test_cassette_refresh_is_scoped_to_manual_candidate_workflow(self):
        env = {"GITHUB_EVENT_NAME": "workflow_dispatch", "GITHUB_WORKFLOW": "E2E replay experiment",
               "REPLAY_MODE": "record", "REPLAY_SCENARIO": MODULE.SCENARIO,
               "KONGCTL_REPLAY_REFRESH_SCENARIO": MODULE.SCENARIO}
        self.assertIsNone(cassette_refresh_scenario({}))
        self.assertEqual(MODULE.SCENARIO, cassette_refresh_scenario(env))
        self.assertEqual(MODULE.SCENARIO, cassette_refresh_scenario(
            {**env, "REPLAY_MODE": "replay", "SOURCE_RUN": "123"}))
        for changes in [{"GITHUB_EVENT_NAME": "pull_request"}, {"GITHUB_WORKFLOW": "CI Test"},
                        {"REPLAY_SCENARIO": "portal/sync"}, {"KONGCTL_REPLAY_REFRESH_SCENARIO": "unknown"},
                        {"REPLAY_MODE": "replay"}, {"SOURCE_RUN": "123"},
                        {"REPLAY_MODE": "replay", "SOURCE_RUN": "not-a-run"}]:
            with self.subTest(changes=changes), self.assertRaises(ValueError):
                cassette_refresh_scenario({**env, **changes})

    def test_assertion_fields_are_data_but_environment_controls_stay_restricted(self):
        import yaml
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            path = root / "scenario.yaml"
            command = {"run": ["get", "event-gateways"], "assertions": [
                {"expect": {"fields": {"headers": {"env": "production"}, "exec": "example",
                                       "resetOrg": True}}}
            ]}
            scenario = {"baseInputsPath": "testdata", "env": {"KONGCTL_LOG_LEVEL": "info"},
                        "steps": [{"commands": [command]}]}
            path.write_text(yaml.safe_dump(scenario))
            MODULE.check_eligibility(root)
            # Both block and flow/quoted YAML forms must be checked structurally.
            for flow in [False, True]:
                for level in ["root", "step", "command"]:
                    changed = copy.deepcopy(scenario)
                    target = {"root": changed, "step": changed["steps"][0],
                              "command": changed["steps"][0]["commands"][0]}[level]
                    target["env"] = {"HTTPS_PROXY": "other-proxy"}
                    path.write_text(yaml.safe_dump(changed, default_flow_style=flow))
                    with self.subTest(level=level, flow=flow), self.assertRaisesRegex(ValueError, "environment"):
                        MODULE.check_eligibility(root)
            for key in ["exec", "create", "delete", "resetOrgRegions", "requiredEnvVars", "assignedEnvironment",
                        "inputOverlayOpsFiles", "stdinFile", "workdir"]:
                changed = copy.deepcopy(scenario)
                changed["steps"][0]["commands"][0][key] = "unsupported"
                path.write_text(yaml.safe_dump(changed))
                with self.subTest(key=key), self.assertRaisesRegex(ValueError, "unsupported"):
                    MODULE.check_eligibility(root)

    def test_only_standalone_initial_reset_is_supported(self):
        import yaml
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            path = root / "scenario.yaml"
            reset = {"name": "reset", "resetOrg": True}
            get = {"run": ["get", "event-gateways"]}
            def check(commands, later_steps=None):
                path.write_text(yaml.safe_dump({"baseInputsPath": "testdata", "steps": [
                    {"commands": commands}, *(later_steps or [])]}))
                MODULE.check_eligibility(root)
            check([reset, get])
            for commands, later in [
                ([get, reset], []), ([reset, get, dict(reset)], []),
                ([reset, get], [{"commands": [dict(reset)]}]),
                ([{**reset, **get}], []), ([{"resetOrg": False}], []),
                ([{"resetOrg": "true"}], []),
            ]:
                with self.subTest(commands=commands, later=later), self.assertRaisesRegex(ValueError, "initial reset"):
                    check(commands, later)

    def test_scenario_controls_reject_ambiguous_yaml(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            header = "baseInputsPath: testdata\n"
            for definition in [
                header + "env: {}\nenv: {KONGCTL_LOG_LEVEL: info}\nsteps: []\n",
                header + "steps: [{commands: [{run: [get], env: {}, env: {}}]}]\n",
                header + "vars: &vars {value: data}\nsteps: []\n",
                header + "steps: [{commands: [{env: !custom {}}]}]\n",
                header + "steps: [{commands: [{assertions: null}]}]\n",
            ]:
                (root / "scenario.yaml").write_text(definition)
                with self.subTest(definition=definition), self.assertRaises(ValueError):
                    MODULE.check_eligibility(root)

    def test_overlay_only_documents_are_local_exact_and_fingerprinted(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            (root / "testdata").mkdir()
            overlay = root / "overlays/update"
            overlay.mkdir(parents=True)
            (root / "scenario.yaml").write_text(
                "baseInputsPath: testdata\nsteps:\n  - inputOverlayDirs:\n      - overlays/update\n")
            (overlay / "apis.yaml").write_text("content: !file updated.md\n")
            document = overlay / "updated.md"
            public = "Public example: author@example.com\n"
            document.write_text(public)
            MODULE.check_eligibility(root)
            fixtures = MODULE.fixture_strings(root)
            MODULE.check_safe({"content": public}, fixtures)
            with self.assertRaises(ValueError):
                MODULE.check_safe({"content": public + "private extra"}, fixtures)
            before = MODULE.scenario_digest(root)
            document.write_text("Changed\n")
            self.assertNotEqual(before, MODULE.scenario_digest(root))
            document.unlink()
            other = root / "overlays/other"
            other.mkdir()
            (other / "updated.md").write_text(public)
            with self.assertRaisesRegex(ValueError, "own overlay"):
                MODULE.check_eligibility(root)

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
            operation = ("      - file: portal.yaml\n        match: portals[?ref=='p'] | [0]\n"
                         "        set: {visibility: private}\n")
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
        with patch.object(MODULE, "DEPENDENCY_WAIT_SECONDS", 0.01), self.assertRaisesRegex(ValueError, "dependency wait"):
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

    def test_visibility_update_cannot_be_skipped_or_changed_between_phases(self):
        before = interaction()
        before["response"]["body"] = {"visibility": "public"}
        after = copy.deepcopy(before)
        after["response"]["body"] = {"visibility": "private"}
        update = interaction("PUT", b'{"visibility":"private"}')
        engine = MODULE.Replay({"interactions": [before, before, update, after, after],
                               "parallel_phases": [{"start": 1, "end": 2, "after": {}},
                                                   {"start": 4, "end": 5, "after": {}}]})
        def send(method="GET", body=b""):
            return engine.exchange("us.api.konghq.com", method, "/v2/control-planes?a=1&b=2", body)
        self.assertEqual(send()["body"], {"visibility": "public"})
        self.assertEqual(send()["body"], {"visibility": "public"})
        with self.assertRaises(ValueError):
            send()  # Later-state reads cannot skip the visibility mutation.
        with self.assertRaises(ValueError):
            send("PUT", b'{"visibility":"public"}')
        send("PUT", b'{"visibility":"private"}')
        self.assertEqual(send()["body"], {"visibility": "private"})
        self.assertEqual(send()["body"], {"visibility": "private"})
        engine.verify()

    def test_read_only_phase_matches_distinct_queries_without_wildcards(self):
        def get(api, value):
            return {"request": MODULE.request_key("regional", "GET", "/v3/api-publications?api=" + api, b""),
                    "response": {"status": 200, "body": {"value": value}}}
        items = [get("a", "first-a"), get("b", "b"), get("a", "second-a")]
        phase = {"start": 1, "end": 3, "after": {}}
        engine = MODULE.Replay({"interactions": items, "parallel_phases": [phase]})
        def send(api):
            return engine.exchange("us.api.konghq.com", "GET", "/v3/api-publications?api=" + api, b"")
        with self.assertRaises(ValueError):
            send("unknown")
        self.assertEqual(send("b")["body"], {"value": "b"})
        self.assertEqual(send("a")["body"], {"value": "first-a"})
        self.assertEqual(send("a")["body"], {"value": "second-a"})
        engine.verify()
        constrained = MODULE.parallel_phases({"interactions": items, "parallel_phases": [
            {**phase, "after": {"2": [1]}}
        ]})
        self.assertIn(0, constrained[0][1])
        # A phase containing writes must not gain this read-only relaxation.
        items[2] = interaction("PUT", b'{"visibility":"private"}')
        mixed = MODULE.parallel_phases({"interactions": items, "parallel_phases": [phase]})
        self.assertIn(0, mixed[0][1])

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
        self.assertIn("group: konnect-e2e-${{ needs.build.outputs.recording_org }}", experiment)
        self.assertIn("environment: ${{ needs.build.outputs.recording_org }}", experiment)
        self.assertIn("KONGCTL_E2E_MATRIX_ORG: ${{ needs.build.outputs.recording_org }}", experiment)
        for scenario in MODULE.SCENARIOS:
            expected = "kongctl-acceptance" if scenario in MODULE.USER_SCENARIOS else "kongctl-acceptance-3"
            if scenario == "dump/portal-owned":
                expected = "kongctl-acceptance-2"
            self.assertEqual(expected, MODULE.recording_org(scenario))
        self.assertIn("cancel-in-progress: false", experiment)
        self.assertIn("queue: max", experiment)

    def test_user_replay_inputs_are_synthetic_and_do_not_inherit_identity(self):
        with patch.dict(os.environ, {"KONGCTL_E2E_ORG_USER_EMAIL_1": "private@example.test"}):
            env = MODULE.clean_environment(Path("/private"), Path("/kongctl"), "org/users/get")
        self.assertEqual("replay-user-1@example.invalid", env["KONGCTL_E2E_ORG_USER_EMAIL_1"])
        self.assertEqual("replay-user-2@example.invalid", env["KONGCTL_E2E_ORG_USER_EMAIL_2"])
        self.assertNotIn("private@example.test", str(env))
        self.assertEqual("0", env["KONGCTL_E2E_RESET"])
        dump = MODULE.clean_environment(Path("/private"), Path("/kongctl"), "dump/organization-teams")
        self.assertEqual("1", dump["KONGCTL_E2E_RESET"])

    def test_user_recording_inputs_fail_closed(self):
        for env in [{}, {"KONGCTL_E2E_ORG_USER_EMAIL_1": "replay-user-1@example.invalid"},
                    {"KONGCTL_E2E_ORG_USER_EMAIL_1": "person@example.test",
                     "KONGCTL_E2E_ORG_USER_EMAIL_2": "PERSON@example.test"}]:
            with self.subTest(env=env), self.assertRaisesRegex(ValueError, "distinct existing"):
                MODULE.recording_inputs("org/users/get", env)

    def test_user_identity_sanitization_preserves_collections_and_states(self):
        identities = MODULE.UserIdentities({"KONGCTL_E2E_ORG_USER_EMAIL_1": "fixture@example.test"})
        users = {"data": [
            {"id": "first", "email": "other@example.test", "full_name": "Other Person", "active": False},
            {"id": "second", "email": "fixture@example.test", "full_name": "Fixture Person",
             "preferred_name": "Fixture", "active": True, "created_at": "2025-07-12T01:02:03Z"},
            {"id": "third", "email": "inactive@example.test", "full_name": "n/a", "preferred_name": None},
        ]}
        normalized = identities.normalize(users)
        self.assertEqual(3, len(normalized["data"]))
        self.assertEqual(["first", "second", "third"], [u["id"] for u in normalized["data"]])
        self.assertFalse(normalized["data"][0]["active"])
        self.assertEqual("replay-user-1@example.invalid", normalized["data"][1]["email"])
        self.assertEqual("replay-user-1", normalized["data"][1]["full_name"])
        self.assertEqual("n/a", normalized["data"][2]["full_name"])
        self.assertIsNone(normalized["data"][2]["preferred_name"])
        self.assertEqual(normalized, identities.normalize(users))
        self.assertNotIn("Person", str(normalized))
        self.assertNotIn("@example.test", str(normalized))
        fixtures = MODULE.fixture_strings(MODULE.ROOT / "test/e2e/scenarios/org/users/get")
        MODULE.check_safe(normalized, fixtures)
        MODULE.check_user_profiles(normalized)
        for key, value in [("full_name", "Real Name"), ("created_at", "2025-02-03T00:00:00Z"),
                           ("phone", "555-0101")]:
            edited = copy.deepcopy(normalized)
            edited["data"][0][key] = value
            with self.assertRaises(ValueError):
                MODULE.check_user_profiles(edited)
        with self.assertRaises(ValueError):
            MODULE.check_user_profiles({"full_name": "Real Name"})
        with self.assertRaises(ValueError):
            MODULE.check_safe(normalized)  # Only the reviewed user scenarios allow synthetic identities.
        for value in [{"email": "fixture@example.test", "phone": "sensitive"},
                      {"email": "fixture@example.test", "secret": "sensitive"}]:  # pragma: allowlist secret
            with self.assertRaisesRegex(ValueError, "profile schema"):
                identities.normalize(value)
        for value in ["real@example.test", "prefix replay-user-1@example.invalid", "Bearer private"]:
            with self.assertRaises(ValueError):
                MODULE.check_safe(value, fixtures)

    def test_user_metadata_exceptions_are_bounded(self):
        directory = MODULE.ROOT / "test/e2e/scenarios/org/users/get"
        scenario = MODULE.parse_scenario((directory / "scenario.yaml").read_text())
        variables = MODULE.USER_SCENARIOS["org/users/get"]
        MODULE.check_scenario_controls(scenario, variables)
        with self.assertRaises(ValueError):
            MODULE.check_scenario_controls(scenario)
        for key, value in [("assignedEnvironment", "another-org"),
                           ("requiredEnvVars", ["PRIVATE_PAT"])]:
            changed = copy.deepcopy(scenario)
            changed["test"][key] = value
            with self.assertRaises(ValueError):
                MODULE.check_scenario_controls(changed, variables)
        changed = copy.deepcopy(scenario)
        changed["steps"][-1]["commands"] = [{"resetOrg": True}]
        with self.assertRaisesRegex(ValueError, "mid-scenario"):
            MODULE.check_scenario_controls(changed, variables)
        MODULE.check_scenario_controls(changed, variables, replay_resets=True)

    def test_user_scenario_data_can_be_separate_from_verifier_code(self):
        directory = MODULE.ROOT / "test/e2e/scenarios/org/users/get"
        with patch.object(MODULE, "ROOT", Path("/separate-verifier-checkout")):
            MODULE.check_eligibility(directory)
            MODULE.check_safe("replay-user-1@example.invalid", MODULE.fixture_strings(directory))
        self.assertIsNone(MODULE.user_scenario(Path("/unrelated/org/users/get")))

    def test_user_recording_does_not_hide_echoed_credentials(self):
        response = MagicMock(status=200)
        response.getheader.return_value = "application/json"
        response.read.return_value = json.dumps({"id": "user-id", "email": "fixture@example.test",
                                                "full_name": "private-credential"}).encode()
        connection = MagicMock()
        connection.getresponse.return_value = response
        fixtures = MODULE.fixture_strings(MODULE.ROOT / "test/e2e/scenarios/org/users/get")
        with patch.object(MODULE.http.client, "HTTPSConnection", return_value=connection):
            engine = MODULE.Replay(token="private-credential", fixtures=fixtures,
                                   user_inputs={"KONGCTL_E2E_ORG_USER_EMAIL_1": "fixture@example.test"})
            with self.assertRaisesRegex(ValueError, "credential echoed"):
                engine.exchange("global.api.konghq.com", "GET", "/v3/users", b"")
        self.assertEqual([], engine.interactions)

    def test_email_paths_fail_closed_before_recording_and_during_validation(self):
        directory = MODULE.ROOT / "test/e2e/scenarios/org/users/get"
        cassette = MODULE.load_cassette(directory / "replay/cassette.json")
        for path in ["/v3/users/fixture@example.test", "/v3/users/fixture%40example.test",
                     "/v3/users/fixture%2540example.test", "/v3/users/%66ixture@%65xample%2etest",
                     "/v3/users/fixture%40example%2Etest", "/v3/users/replay-user-1%40example.invalid"]:
            with self.subTest(path=path):
                engine = MODULE.Replay(token="private-credential", fixtures=MODULE.fixture_strings(directory),
                                       user_inputs={"KONGCTL_E2E_ORG_USER_EMAIL_1": "fixture@example.test"})
                with patch.object(MODULE.http.client, "HTTPSConnection") as connection:
                    with self.assertRaisesRegex(ValueError, "email address in request path") as error:
                        engine.exchange("global.api.konghq.com", "GET", path, b"")
                    connection.assert_not_called()
                self.assertNotIn(path, str(error.exception))
                self.assertEqual([], engine.interactions)
                edited = copy.deepcopy(cassette)
                edited["interactions"][0]["request"]["path"] = path
                with self.assertRaisesRegex(ValueError, "email address in request path"):
                    MODULE.validate_cassette(edited, directory, "org/users/get")

    def test_safe_encoded_paths_retain_exact_matching(self):
        for path in ["/v3/teams/team%20one", "/v3/teams/team%2Fchild", "/v3/teams/team%252Fchild"]:
            with self.subTest(path=path):
                request = MODULE.request_key("global", "GET", path, b"")
                self.assertEqual(path, request["path"])
        request = MODULE.request_key("global", "GET", "/v3/users?email=fixture%40example.test", b"")
        self.assertEqual([["email", "fixture@example.test"]], request["query"])

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
        refresh = cassette_refresh_scenario(os.environ)
        for path in sorted(root.glob("**/replay/cassette.json")):
            directory = path.parent.parent
            scenario = directory.relative_to(root).as_posix()
            with self.subTest(path=path):
                MODULE.validate_cassette(MODULE.load_cassette(path), directory,
                                        scenario, allow_stale=scenario == refresh)

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

    def test_opaque_documents_and_binary_assets_are_fingerprinted_not_parsed(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            (root / "scenario.yaml").write_text("baseInputsPath: testdata\nsteps: []\n")
            inputs = root / "testdata"
            inputs.mkdir()
            (inputs / "portal.yaml").write_text("logo: !file logo.png\ncontent: !file guide.md\n")
            (inputs / "logo.png").write_bytes(b"\x89PNG\r\n\x1a\n")
            (inputs / "guide.md").write_text("The `!file` and `!env` tags are documented here.\n")
            MODULE.check_eligibility(root)
            digest = MODULE.scenario_digest(root)
            (inputs / "logo.png").write_bytes(b"\x89PNG\r\n\x1a\nchanged")
            self.assertNotEqual(digest, MODULE.scenario_digest(root))
            digest = MODULE.scenario_digest(root)
            (inputs / "guide.md").write_text("Changed documentation\n")
            self.assertNotEqual(digest, MODULE.scenario_digest(root))
            (inputs / "portal.yaml").write_text("email: !env UNREVIEWED\n")
            with self.assertRaisesRegex(ValueError, "external input"):
                MODULE.check_eligibility(root)

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
            (root / "scenario.yaml").write_text("baseInputsPath: testdata\nsteps: [{name: changed}]")
            with self.assertRaisesRegex(ValueError, "stale cassette"):
                MODULE.validate_cassette(cassette, root)
            MODULE.validate_cassette(cassette, root, allow_stale=True)
            invalid = copy.deepcopy(cassette)
            invalid["interactions"][0]["response"]["body"] = {"token": "private"}
            with self.assertRaisesRegex(ValueError, "sensitive field"):
                MODULE.validate_cassette(invalid, root, allow_stale=True)
            with self.assertRaisesRegex(ValueError, "must be a live recording"):
                MODULE.validate_cassette(bootstrap, root, allow_stale=True)
            (root / "scenario.yaml").write_text("baseInputsPath: testdata\nenv: {}\nsteps: []")
            with self.assertRaisesRegex(ValueError, "environment"):
                MODULE.validate_cassette(cassette, root, allow_stale=True)

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


class ReplayDependencyWaitTest(unittest.TestCase):
    def setUp(self):
        self.items = [
            {"request": MODULE.request_key("regional", "GET", "/parents", b""),
             "response": {"status": 200, "body": {"state": "before"}}},
            {"request": MODULE.request_key("regional", "PATCH", "/parents/child", b'{"value":"updated"}'),
             "response": {"status": 200, "body": {"state": "after"}}},
            {"request": MODULE.request_key("regional", "GET", "/later", b""),
             "response": {"status": 200, "body": {"state": "verified"}}},
        ]
        self.engine = MODULE.Replay({"interactions": self.items,
                                    "parallel_phases": [{"start": 1, "end": 2, "after": {}}]})
        self.addCleanup(self.engine.close)

    def send(self, index):
        request = self.items[index]["request"]
        body = json.dumps(request["body"]).encode() if request["body"] is not None else b""
        return self.engine.exchange("us.api.konghq.com", request["method"], request["path"], body)

    def observe_wait(self):
        waiting = threading.Event()
        original = self.engine.changed.wait

        def wait(timeout):
            waiting.set()
            return original(timeout)

        self.enterContext(patch.object(self.engine.changed, "wait", side_effect=wait))
        return waiting

    def test_wait_reserves_exchange_and_rejects_mismatches_and_later_phases(self):
        waiting = self.observe_wait()
        with ThreadPoolExecutor(max_workers=1) as pool:
            pending = pool.submit(self.send, 1)
            try:
                self.assertTrue(waiting.wait(2))
                self.assertEqual(self.engine.pending, {1})
                # A duplicate cannot steal a waiting request's response.
                with self.assertRaisesRegex(ValueError, "request mismatch"):
                    self.send(1)
                with self.assertRaisesRegex(ValueError, "request mismatch"):
                    self.send(2)
                for method, target, body in [
                    ("PUT", "/parents/child", b'{"value":"updated"}'),
                    ("PATCH", "/parents/other", b'{"value":"updated"}'),
                    ("PATCH", "/parents/child?unexpected=1", b'{"value":"updated"}'),
                    ("PATCH", "/parents/child", b'{"value":"different"}'),
                ]:
                    with self.subTest(method=method, target=target, body=body), self.assertRaisesRegex(
                            ValueError, "request mismatch"):
                        self.engine.exchange("us.api.konghq.com", method, target, body)
            finally:
                self.send(0)
            self.assertEqual(pending.result(timeout=2), self.items[1]["response"])
        self.assertEqual(self.engine.pending, set())
        with self.assertRaisesRegex(ValueError, "request mismatch"):
            self.send(1)
        self.send(2)
        self.engine.verify()
        with self.assertRaisesRegex(ValueError, "unexpected extra request"):
            self.send(2)

    def test_spurious_wakeups_do_not_extend_deadline_or_consume_exchange(self):
        with patch.object(MODULE, "DEPENDENCY_WAIT_SECONDS", 5), \
                patch.object(MODULE.time, "monotonic", side_effect=[0, 0, 1, 5]), \
                patch.object(self.engine.changed, "wait", return_value=True) as wait:
            with self.assertRaisesRegex(ValueError,
                    r"dependency wait timed out for interaction 2; waiting for interactions \[1\]"):
                self.send(1)
        self.assertEqual([call.args[0] for call in wait.call_args_list], [5, 4])
        self.assertEqual(self.engine.pending, set())
        self.assertEqual(self.engine.completed, set())
        self.assertEqual(self.engine.position, 0)
        with self.assertRaisesRegex(ValueError, "required interactions unused"):
            self.engine.verify()

    def test_missing_dependency_times_out_in_http_handler_and_fails_verification(self):
        with patch.object(MODULE, "DEPENDENCY_WAIT_SECONDS", 0.02), tempfile.TemporaryDirectory() as directory:
            with MODULE.Server(self.engine, Path(directory)) as server:
                context = ssl.create_default_context(cafile=str(server.certificate))
                client = http.client.HTTPSConnection("127.0.0.1", server.httpd.server_port, context=context, timeout=2)
                client.set_tunnel("us.api.konghq.com", 443)
                try:
                    client.request("PATCH", "/parents/child", b'{"value":"updated"}')
                    response = client.getresponse()
                    self.assertEqual(response.status, 400)
                    response.read()
                finally:
                    client.close()
            with self.assertRaisesRegex(ValueError, "dependency wait timed out for interaction 2"):
                self.engine.verify()

    def test_identical_waiting_requests_keep_their_own_responses_in_order(self):
        second_patch = copy.deepcopy(self.items[1])
        second_patch["response"]["body"] = {"state": "second"}
        self.items = [self.items[0], self.items[1], second_patch]
        self.engine = MODULE.Replay({"interactions": self.items,
                                    "parallel_phases": [{"start": 1, "end": 3, "after": {}}]})
        self.addCleanup(self.engine.close)
        waiting = self.observe_wait()
        with ThreadPoolExecutor(max_workers=2) as pool:
            first = pool.submit(self.send, 1)
            try:
                self.assertTrue(waiting.wait(2))
                waiting.clear()
                second = pool.submit(self.send, 2)
                self.assertTrue(waiting.wait(2))
                with self.engine.lock:
                    self.assertEqual(self.engine.pending, {1, 2})
                with self.assertRaisesRegex(ValueError, "request mismatch"):
                    self.send(1)
            finally:
                self.send(0)
            self.assertEqual(first.result(timeout=2), self.items[1]["response"])
            self.assertEqual(second.result(timeout=2), self.items[2]["response"])
        self.engine.verify()

    def test_proxy_failure_wakes_waiter_without_consuming_response(self):
        self.assert_waiter_stops(lambda: self.engine.fail("protocol failure"))
        with self.assertRaisesRegex(ValueError, "protocol failure"):
            self.engine.verify()

    def test_proxy_shutdown_wakes_waiter_without_consuming_response(self):
        self.assert_waiter_stops(self.engine.close)
        with self.assertRaisesRegex(ValueError, "replay stopped with pending interactions"):
            self.engine.verify()

    def assert_waiter_stops(self, stop):
        waiting = self.observe_wait()
        with ThreadPoolExecutor(max_workers=1) as pool:
            pending = pool.submit(self.send, 1)
            try:
                self.assertTrue(waiting.wait(2))
            finally:
                stop()
            with self.assertRaisesRegex(ValueError, "stopped or previously failed"):
                pending.result(timeout=2)
        self.assertEqual(self.engine.pending, set())
        self.assertEqual(self.engine.position, 0)


if __name__ == "__main__":
    unittest.main()
