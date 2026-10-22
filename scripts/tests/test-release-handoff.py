#!/usr/bin/env python3
"""Exercise source/hash rejection and a real GoReleaser prebuilt dry run."""

import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import textwrap
import unittest
import zipfile

REPO = Path(__file__).resolve().parents[2]


def workflow_script(name):
    workflow = (REPO / ".github/workflows/release.md").read_text()
    step = workflow.split(f"      - name: {name}\n", 1)[1].split("      - name:", 1)[0]
    lines = []
    for line in step.split("        run: |\n", 1)[1].splitlines():
        if line.strip() and not line.startswith("          "):
            break
        lines.append(line)
    return textwrap.dedent("\n".join(lines))


class HandoffTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        self.env = {**os.environ, "GITHUB_REPOSITORY": "Kong/kongctl",
                    "GITHUB_REF": "refs/heads/main", "RELEASE_TAG": "v9.9.9",
                    "RELEASE_BUILD_MODE": "full"}
        self.run_command("git", "init", "-b", "main")
        self.run_command("git", "config", "user.name", "Release test")
        self.run_command("git", "config", "user.email", "release@example.invalid")
        shutil.copy(REPO / ".goreleaser.yml", self.root)
        shutil.copytree(REPO / "scripts", self.root / "scripts")
        (self.root / "main.go").write_text("package main\nfunc main() {}\n")
        (self.root / "go.mod").write_text("module example.invalid/release-test\n\ngo 1.26\n")
        (self.root / "README.md").write_text("Release fixture\n")
        self.run_command("git", "add", ".")
        self.run_command("git", "commit", "-m", "fixture")
        self.run_command("git", "tag", "v9.9.8")
        for message in ("docs: hidden documentation", "test: hidden tests",
                        "fix: zeta change", "feat: alpha change"):
            self.run_command("git", "commit", "--allow-empty", "-m", message)
        commit = self.run_command("git", "rev-parse", "HEAD").stdout.strip()
        self.env["RELEASE_COMMIT"] = commit
        self.run_command("git", "tag", "v9.9.9")
        self.run_command("git", "update-ref", "refs/remotes/origin/main", commit)
        self.run_command("git", "remote", "add", "origin", "https://github.com/Kong/kongctl.git")
        self.input = self.root / "handoff"
        for target in (f"{os}_{arch}" for os in ("darwin", "linux", "windows")
                       for arch in ("amd64", "arm64")):
            name = "kongctl.exe" if target.startswith("windows") else "kongctl"
            directory = self.input / "binaries" / target
            directory.mkdir(parents=True)
            (directory / name).write_bytes(f"never execute: {target}".encode())
        self.handoff("record")

    def run_command(self, *args, check=True):
        return subprocess.run(args, cwd=self.root, env=self.env, text=True,
                              capture_output=True, check=check)

    def handoff(self, operation, check=True):
        return self.run_command("python3", str(REPO / "scripts/release-handoff.py"),
                                operation, "handoff", check=check)

    def test_hash_and_source_rejections(self):
        self.handoff("verify")
        binary = self.input / "binaries/linux_amd64/kongctl"
        original = binary.read_bytes()
        binary.write_bytes(b"tampered")
        self.assertNotEqual(self.handoff("verify", check=False).returncode, 0)
        binary.write_bytes(original)
        extra = self.input / "extra"
        extra.write_text("unexpected")
        self.assertNotEqual(self.handoff("verify", check=False).returncode, 0)
        extra.unlink()
        binary.unlink()
        self.assertNotEqual(self.handoff("verify", check=False).returncode, 0)
        binary.symlink_to(self.root / "main.go")
        self.assertNotEqual(self.handoff("record", check=False).returncode, 0)
        binary.unlink()
        binary.write_bytes(original)
        for name, value in (("RELEASE_COMMIT", "0" * 40),
                            ("GITHUB_REPOSITORY", "other/kongctl"),
                            ("GITHUB_REF", "refs/heads/other"),
                            ("RELEASE_TAG", "v1.0.0")):
            previous = self.env[name]
            self.env[name] = value
            self.assertNotEqual(self.handoff("verify", check=False).returncode, 0, name)
            self.env[name] = previous
        self.run_command("git", "update-ref", "-d", "refs/remotes/origin/main")
        self.assertNotEqual(self.handoff("verify", check=False).returncode, 0)

    def test_real_goreleaser_packages_without_go(self):
        # A compiler, generator, or dependency invocation fails this dry run.
        sentinel = self.root / "deny-go"
        sentinel.mkdir()
        go = sentinel / "go"
        go.write_text("#!/bin/sh\necho 'Go invoked in packaging job' >&2\nexit 99\n")
        go.chmod(0o755)
        self.env.update({"PATH": f"{self.root / 'scripts/release-tools'}:{sentinel}:{self.env['PATH']}",
                         "RELEASE_INPUT": str(self.input), "GITHUB_TOKEN": "",
                         "TAP_GITHUB_TOKEN": ""})
        (self.root / ".git/info/exclude").write_text("/handoff/\n/deny-go/\n")
        self.handoff("verify")
        result = self.run_command("goreleaser", "release", "--clean",
                                  "--skip=before,publish,docker,notarize", check=False)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        for archive in (self.root / "dist").glob("kongctl_*.zip"):
            target = archive.stem.removeprefix("kongctl_")
            name = "kongctl.exe" if target.startswith("windows") else "kongctl"
            with zipfile.ZipFile(archive) as package:
                self.assertEqual(package.read(name),
                                 (self.input / "binaries" / target / name).read_bytes())
        self.assertEqual(len(list((self.root / "dist").glob("kongctl_*.zip"))), 6)
        self.assertTrue((self.root / "dist/homebrew/Casks/kongctl.rb").is_file())
        metadata = json.loads((self.root / "dist/metadata.json").read_text())
        self.assertEqual(metadata["version"], "9.9.9")
        self.assertEqual(metadata["commit"], self.env["RELEASE_COMMIT"])

        # Exercise the production staging block against GoReleaser's actual
        # per-target output directories, not a separately assumed fixture layout.
        original = self.root / "original-input"
        self.input.rename(original)
        self.run_command("bash", "-c", workflow_script("Stage unsigned binaries"))
        staged = sorted(path.relative_to(self.input).as_posix()
                        for path in self.input.rglob("*") if path.is_file())
        expected = sorted(path.relative_to(original).as_posix()
                          for path in original.rglob("*")
                          if path.is_file() and path.name != "handoff.json")
        self.assertEqual(staged, expected)
        for name in staged:
            self.assertEqual((self.input / name).read_bytes(), (original / name).read_bytes())
        self.handoff("record")

        # Carry GoReleaser's configured notes through the signed hash manifest
        # and the real publication command, with only the GitHub API replaced.
        changelog = (self.root / "dist/CHANGELOG.md").read_text()
        self.assertNotIn("hidden documentation", changelog)
        self.assertNotIn("hidden tests", changelog)
        self.assertLess(changelog.index("feat: alpha change"), changelog.index("fix: zeta change"))
        self.run_command("bash", "-c", workflow_script("Record final signed package hashes"))
        self.run_command("python3", "scripts/release-handoff.py", "verify", "signed")
        notes = self.root / "signed/CHANGELOG.md"
        notes.write_text("tampered notes")
        rejected = self.run_command("python3", "scripts/release-handoff.py", "verify", "signed", check=False)
        self.assertNotEqual(rejected.returncode, 0)
        notes.write_text(changelog)
        # A fresh publisher has no distribution output from the packaging job.
        shutil.rmtree(self.root / "dist/homebrew")
        self.run_command("bash", "-c", workflow_script("Verify every handoff before publication"))
        gh = sentinel / "gh"
        gh.write_text("#!/usr/bin/env python3\n"
                      "import json, pathlib, sys\n"
                      "args = sys.argv[1:]\n"
                      "notes = pathlib.Path(args[args.index('--notes-file') + 1]).read_text()\n"
                      "pathlib.Path('publication.json').write_text(json.dumps({'args': args, 'notes': notes}))\n")
        gh.chmod(0o755)
        self.run_command("bash", "-c", workflow_script("Create signed draft from verified assets"))
        publication = json.loads((self.root / "publication.json").read_text())
        self.assertEqual(publication["notes"], changelog)
        self.assertNotIn("--generate-notes", publication["args"])

        # A changed GoReleaser directory convention must fail with the missing
        # source path and an actionable diagnostic before any handoff upload.
        output = self.root / "dist/kongctl_darwin_amd64_v1/kongctl"
        self.assertTrue(output.is_file())
        output.rename(output.with_name("moved-kongctl"))
        rejected = self.run_command("bash", "-c", workflow_script("Stage unsigned binaries"), check=False)
        self.assertNotEqual(rejected.returncode, 0)
        self.assertIn("Missing GoReleaser build output: dist/kongctl_darwin_amd64_v1/kongctl", rejected.stdout)


if __name__ == "__main__":
    unittest.main()
