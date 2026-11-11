"""Exercise the group fixture's deployment-specific team fallback offline."""

import importlib.util
import io
import json
from pathlib import Path
import unittest
import urllib.error
from unittest.mock import patch


FIXTURE = Path(__file__).resolve().parents[1] / "test/e2e/scenarios/identity/groups/fixtures.py"
SPEC = importlib.util.spec_from_file_location("identity_group_fixtures", FIXTURE)
fixtures = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(fixtures)

ENUM = {"field": "type", "rule": "enum", "reason": "type must be one of: group", "source": "body"}
ALL_OF = {"field": "type", "rule": "allOf", "reason": 'doesn\'t match all schemas from "allOf"', "source": ""}


class GroupFixtureTest(unittest.TestCase):
    def create(self, failure=None, status=400, fail_type="team"):
        requests = []

        def respond(request, timeout):
            self.assertEqual(timeout, 30)
            body = json.loads(request.data)
            requests.append(body)
            if failure is not None and body.get("type") == fail_type:
                raise urllib.error.HTTPError(request.full_url, status, "rejected", {},
                                             io.BytesIO(json.dumps(failure).encode()))
            response = io.BytesIO(json.dumps({"id": f"fixture-{len(requests)}", **body}).encode())
            response.status = 201
            return response

        with patch.object(fixtures.sys, "argv", ["fixtures.py", "create", "directory", "user", "agent", "legacy"]), \
                patch.dict(fixtures.os.environ, {"KONGCTL_E2E_KONNECT_PAT": "test-only",
                                                "KONGCTL_E2E_KONNECT_BASE_URL": "https://us.api.konghq.com"}), \
                patch.object(fixtures.urllib.request, "urlopen", side_effect=respond):
            return fixtures.main(), requests

    def test_supported_teams(self):
        result, _ = self.create()
        self.assertTrue(result["team_available"])
        self.assertEqual((result["ordinary_count"], result["team_count"]), (3, 1))

    def test_known_team_rejections_fall_back_to_group(self):
        for parameter in (ENUM, ALL_OF):
            with self.subTest(parameter=parameter):
                result, requests = self.create({"invalid_parameters": [parameter]})
                self.assertFalse(result["team_available"])
                self.assertEqual(result["alternate_type"], "group")
                self.assertEqual((result["created"], result["ordinary_count"], result["team_count"]), (4, 4, 0))
                self.assertEqual(len([r for r in requests if "member" in r]), 4)
                self.assertTrue(any(r.get("type") == "group" and r["name"].endswith("TeamFallback")
                                    for r in requests))

    def test_unrelated_rejections_remain_fatal(self):
        for failure, status, fail_type in (
                ({"invalid_parameters": [dict(ALL_OF, field="name")]}, 400, "team"),
                ({"invalid_parameters": [ALL_OF, ENUM]}, 400, "team"),
                ({"invalid_parameters": [dict(ALL_OF, reason="different validation")]}, 400, "team"),
                ({"message": "unknown rejection"}, 400, "team"),
                ({"invalid_parameters": [ALL_OF]}, 500, "team"),
                ({"invalid_parameters": [ALL_OF]}, 400, "group")):
            with self.subTest(failure=failure, status=status, fail_type=fail_type):
                with self.assertRaises((RuntimeError, urllib.error.HTTPError)):
                    self.create(failure, status, fail_type)

    def test_failure_context_omits_response_body(self):
        with self.assertRaises(RuntimeError) as raised:
            self.create({"message": "private-response-content"}, 500)
        message = str(raised.exception)
        self.assertIn("POST /v2/directories/directory/groups", message)
        self.assertIn("HTTP 500", message)
        self.assertNotIn("private-response-content", message)
        self.assertNotIn("test-only", message)


if __name__ == "__main__":
    unittest.main()
