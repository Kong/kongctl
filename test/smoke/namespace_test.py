#!/usr/bin/env python3
"""Exercise ownership metadata parsing in the smoke CLI fake."""

import argparse
import json
import os
import pathlib
import subprocess
import sys
import tempfile
import unittest

TEMP_ROOT = None
FAKE = pathlib.Path(__file__).with_name("fake_kongctl.py")


class NamespaceChecks(unittest.TestCase):
    def setUp(self):
        directory = tempfile.TemporaryDirectory(dir=TEMP_ROOT)
        self.addCleanup(directory.cleanup)
        self.directory = pathlib.Path(directory.name)
        self.fixture = self.directory / "api" / "initial.yaml"
        self.fixture.parent.mkdir()
        self.plan = self.directory / "plan.json"
        self.environment = {key: value for key, value in os.environ.items()
                            if not key.startswith("FAKE_")}
        self.environment.pop("SMOKE_NAMESPACE_TEST", None)
        self.environment.update(FAKE_KONGCTL_LOG=str(self.directory / "commands.log"),
                                FAKE_KONGCTL_STATE=str(self.directory / "state.json"))

    def run_plan(self, namespace, prefix="", expected_error=None):
        metadata = f"    kongctl:\n      namespace: {namespace}\n" if namespace else ""
        self.fixture.write_text(prefix + 'apis:\n  - ref: "api"\n    name: "api"\n'
                                '    labels:\n      smoke-phase: "initial"\n' + metadata)
        result = subprocess.run(
            [sys.executable, str(FAKE), "plan", "-f", str(self.fixture),
             "--require-namespace", "smoke-test", "--output-file", str(self.plan)],
            env=self.environment, capture_output=True, text=True, check=False,
        )
        if expected_error:
            self.assertNotEqual(result.returncode, 0)
            self.assertIn(expected_error, result.stderr)
            self.assertNotIn("Traceback", result.stderr)
        else:
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual(json.loads(self.plan.read_text())["_desired"]["_namespace"], "smoke-test")

    def test_nested_namespace_does_not_select_ownership(self):
        prefix = 'metadata:\n  namespace: "unrelated"\n'
        self.run_plan('"smoke-test"', prefix=prefix)

    def test_explicit_namespace_takes_precedence_over_defaults(self):
        self.run_plan('"smoke-test"', prefix='_defaults:\n  kongctl:\n    namespace: "other"\n')

    def test_default_environment_namespace(self):
        self.environment["SMOKE_NAMESPACE_TEST"] = "smoke-test"
        self.run_plan(None, prefix='_defaults:\n  kongctl:\n    namespace: !env SMOKE_NAMESPACE_TEST\n')

    def test_explicit_environment_namespace(self):
        self.environment["SMOKE_NAMESPACE_TEST"] = "smoke-test"
        self.run_plan("!env {var: SMOKE_NAMESPACE_TEST}")

    def test_missing_metadata_has_a_clear_error(self):
        self.run_plan(None, prefix='metadata:\n  namespace: "smoke-test"\n',
                      expected_error="fixture is missing kongctl namespace metadata")

    def test_missing_or_empty_environment_has_a_clear_error(self):
        for value in (None, ""):
            with self.subTest(value=value):
                if value is not None:
                    self.environment["SMOKE_NAMESPACE_TEST"] = value
                self.run_plan("!env SMOKE_NAMESPACE_TEST",
                              expected_error="namespace environment variable SMOKE_NAMESPACE_TEST is unset or empty")


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--temp-root")
    arguments, remaining = parser.parse_known_args()
    TEMP_ROOT = arguments.temp_root
    unittest.main(argv=[sys.argv[0], *remaining])
