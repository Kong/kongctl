import base64
import importlib.util
import os
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch


script = Path(__file__).parents[1] / "homebrew/rehearse-app-publication.py"
spec = importlib.util.spec_from_file_location("rehearsal", script)
rehearsal = importlib.util.module_from_spec(spec)
spec.loader.exec_module(rehearsal)


class RehearsalTest(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory()
        self.addCleanup(self.directory.cleanup)
        self.root = Path(self.directory.name)
        self.addCleanup(patch.stopall)
        patch.dict(os.environ, {"GITHUB_RUN_ID": "123"}).start()
        self.git("init", "-q")
        for target in rehearsal.TARGETS.values():
            path = self.root / target
            path.parent.mkdir(exist_ok=True)
            path.write_text('version "1.24.0"\n')
        (self.root / "README.md").write_text("tap\n")
        self.git("add", ".")
        self.git("-c", "user.name=Test", "-c", "user.email=test@example.com",
                 "-c", "commit.gpgsign=false", "-c", "core.hooksPath=/dev/null", "commit", "-qm", "baseline")

    def git(self, *args):
        subprocess.run(["git", *args], cwd=self.root, check=True, capture_output=True)

    def guard(self):
        return subprocess.run(["python3", str(script.resolve()), "guard"], cwd=self.root,
                              capture_output=True, text=True)

    def test_marker_only_commit_passes(self):
        path = self.root / "Formula/kongctl.rb"
        path.write_text(rehearsal.marker() + path.read_text())
        self.git("add", ".")
        self.assertEqual(self.guard().returncode, 0)

    def test_version_change_is_rejected(self):
        path = self.root / "Casks/kongctl.rb"
        path.write_text(rehearsal.marker() + 'version "1.25.0"\n')
        self.git("add", ".")
        self.assertNotEqual(self.guard().returncode, 0)

    def test_unrelated_file_is_rejected(self):
        (self.root / "README.md").write_text(rehearsal.marker() + "tap\n")
        self.git("add", ".")
        self.assertNotEqual(self.guard().returncode, 0)

    def test_cleanup_preserves_intervening_release_changes(self):
        written = []

        def fake_gh(*args, payload=None):
            if payload is None:
                return {"sha": "latest-sha", "content": base64.b64encode(
                    (rehearsal.marker() + 'version "1.25.0"\n').encode()).decode()}
            written.append(payload)
            return {}

        with patch.object(rehearsal, "gh", side_effect=fake_gh):
            rehearsal.cleanup()
        self.assertEqual(len(written), 2)
        for payload in written:
            self.assertEqual(payload["sha"], "latest-sha")
            self.assertEqual(base64.b64decode(payload["content"]), b'version "1.25.0"\n')


if __name__ == "__main__":
    unittest.main()
