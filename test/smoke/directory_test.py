#!/usr/bin/env python3
"""Verify directory discovery, first-use bootstrap, and retained ownership."""

import argparse
import json
import os
import pathlib
import subprocess
import sys
import tempfile
import unittest

TEMP_ROOT = None
ROOT = pathlib.Path(__file__).resolve().parents[2]
DEFAULT = {"id": "22222222-2222-4222-8222-222222222222", "name": "default"}


class DirectoryChecks(unittest.TestCase):
    def run_smoke(self, directories, *options, fault=""):
        scratch = tempfile.TemporaryDirectory(dir=TEMP_ROOT)
        self.addCleanup(scratch.cleanup)
        work = pathlib.Path(scratch.name)
        fake = work / "kongctl"
        fake.write_bytes((ROOT / "test/smoke/fake_kongctl.py").read_bytes())
        fake.chmod(0o700)
        state = work / "state.json"
        state.write_text("{}")
        inventory = state.with_suffix(".directories.json")
        inventory.write_text(json.dumps(directories))
        env = {key: value for key, value in os.environ.items()
               if not key.startswith("FAKE_") and key != "KONGCTL_SMOKE_YES"}
        env.update(FAKE_KONGCTL_LOG=str(work / "commands.log"), FAKE_KONGCTL_STATE=str(state), FAKE_FAULT=fault)
        result = subprocess.run(
            ["bash", str(ROOT / "scripts/smoke-test.sh"), "--binary", str(fake),
             "--artifacts-dir", str(work), "--resources", "identity_principal", *options],
            env=env, input="n\n", capture_output=True, text=True, check=False,
        )
        self.log = (work / "commands.log").read_text()
        self.directories = json.loads(inventory.read_text())
        self.state = json.loads(state.read_text())
        self.run_dir = next(work.glob("kongctl-smoke.*"))
        self.report = json.loads((self.run_dir / "report.json").read_text())
        return result

    def test_existing_default_is_reused(self):
        result = self.run_smoke([DEFAULT], "--yes")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertNotIn("plan -f " + str(self.run_dir / "fixtures/identity-directory-bootstrap.yaml"), self.log)
        self.assertEqual(self.directories, [DEFAULT])
        self.assertEqual(self.state, {})

    def test_existing_named_directory_is_reused(self):
        existing = dict(DEFAULT, name="employees")
        result = self.run_smoke([existing], "--yes")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("--directory-name employees", self.log)
        self.assertEqual(self.directories, [existing])
        self.assertEqual(self.state, {})

    def test_default_is_preferred_over_other_existing_directories(self):
        inventory = [dict(DEFAULT, name="employees", id="11111111-1111-4111-8111-111111111111"), DEFAULT]
        result = self.run_smoke(inventory, "--yes")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("--directory-name default", self.log)
        self.assertEqual(self.directories, inventory)

    def test_empty_organization_is_bootstrapped_and_retained(self):
        result = self.run_smoke([], "--yes")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("bootstrap-identity-directory", result.stdout)
        self.assertEqual(len(self.directories), 1)
        self.assertEqual(self.directories[0]["name"], "default")
        manifest = json.loads((self.run_dir / "fixtures/identity-directory-bootstrap.yaml").read_text())
        directory = manifest["identity"]["directories"][0]
        self.assertTrue(directory["allow_all_control_planes"])
        self.assertTrue(directory["kongctl"]["protected"])
        self.assertEqual(self.state, {})

    def test_quick_mode_does_not_bootstrap(self):
        result = self.run_smoke([], "--quick")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual(self.directories, [])
        self.assertNotIn("apply ", self.log)
        self.assertTrue(any(c["status"] == "skipped" for c in self.report["checks"]))

    def test_declined_confirmation_does_not_bootstrap(self):
        result = self.run_smoke([])
        self.assertEqual(result.returncode, 1, result.stdout + result.stderr)
        self.assertEqual(self.directories, [])
        self.assertNotIn("apply ", self.log)

    def test_failed_inventory_is_not_treated_as_empty(self):
        result = self.run_smoke([], "--yes", fault="directory-read-error")
        self.assertEqual(result.returncode, 1)
        self.assertNotIn("apply ", self.log)
        self.assertEqual(self.directories, [])

    def test_bootstrap_failure_stops_before_principal_creation(self):
        result = self.run_smoke([], "--yes", fault="bootstrap-failure")
        self.assertEqual(result.returncode, 1)
        self.assertEqual(self.directories, [])
        self.assertEqual(self.state, {})
        self.assertNotIn("apply-create-identity_principal", result.stdout)

    def test_principal_failure_retains_bootstrapped_directory(self):
        result = self.run_smoke([], "--yes", fault="stale-principal")
        self.assertEqual(result.returncode, 1)
        self.assertEqual(self.directories[0]["name"], "default")
        self.assertEqual(self.state, {})


if __name__ == "__main__":
    parser = argparse.ArgumentParser(add_help=False)
    parser.add_argument("--temp-root")
    options, unittest_args = parser.parse_known_args()
    TEMP_ROOT = options.temp_root
    unittest.main(argv=[sys.argv[0], *unittest_args])
