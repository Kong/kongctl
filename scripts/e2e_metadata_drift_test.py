import contextlib
import io
import json
import os
from pathlib import Path
import runpy
import unittest
from unittest.mock import patch


HELPER = (
    Path(__file__).resolve().parents[1]
    / "test/e2e/scenarios/ai-gateway/federated-providers/testdata"
    / "inject_managed_by_drift.py"
)


class MetadataDriftTest(unittest.TestCase):
    def test_environment_resolution_for_all_drift_requests(self):
        for environment, override, expected in [
            (None, None, "https://us.api.konghq.com"),
            ("", "", "https://us.api.konghq.com"),
            ("com", "", "https://us.api.konghq.com"),
            ("production", "", "https://us.api.konghq.com"),
            ("prod", "", "https://us.api.konghq.com"),
            ("tech", None, "https://us.api.konghq.tech"),
            ("tech", "", "https://us.api.konghq.tech"),
            (" TECH ", "  ", "https://us.api.konghq.tech"),
            ("tech", " https://eu.api.konghq.tech/ ", "https://eu.api.konghq.tech"),
            ("com", "https://custom.example.test/", "https://custom.example.test"),
        ]:
            with self.subTest(environment=environment, override=override):
                env = {"KONGCTL_E2E_KONNECT_PAT": "synthetic"}
                if environment is not None:
                    env["KONGCTL_E2E_KONNECT_ENV"] = environment
                if override is not None:
                    env["KONGCTL_E2E_KONNECT_BASE_URL"] = override
                responses = [
                    {"data": [{"name": "federated-providers-e2e", "id": "gateway"}]},
                    {"data": [{
                        "name": "provider-a", "id": "provider",
                        "display_name": "Provider A", "type": "openai",
                        "config": {}, "labels": {"team": "a"},
                        "managed_by": {"tool": "kongctl", "namespace": "team-a"},
                    }]},
                    {},
                ]
                with (
                    patch.dict(os.environ, env, clear=True),
                    patch("urllib.request.urlopen", side_effect=[
                        io.BytesIO(json.dumps(response).encode()) for response in responses
                    ]) as urlopen,
                    contextlib.redirect_stdout(io.StringIO()),
                ):
                    runpy.run_path(str(HELPER), run_name="__main__")

                requests = [call.args[0] for call in urlopen.call_args_list]
                base = expected + "/v1/ai-gateways"
                self.assertEqual([request.full_url for request in requests], [
                    base, base + "/gateway/model-providers",
                    base + "/gateway/model-providers/provider",
                ])
                self.assertEqual([request.get_method() for request in requests], ["GET", "GET", "PUT"])
                body = json.loads(requests[-1].data)
                self.assertEqual(body["managed_by"], {
                    "tool": "kongctl", "namespace": "team-a", "outside": "remove",
                })

    def test_unknown_environment_fails_before_request(self):
        with (
            patch.dict(os.environ, {"KONGCTL_E2E_KONNECT_ENV": "unknown"}, clear=True),
            patch("urllib.request.urlopen") as urlopen,
            self.assertRaisesRegex(SystemExit, "Unsupported Konnect environment"),
        ):
            runpy.run_path(str(HELPER), run_name="__main__")
        urlopen.assert_not_called()


if __name__ == "__main__":
    unittest.main()
