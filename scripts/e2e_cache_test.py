"""Exercise the build workflow's fallback partition and reporting contract."""

import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

WORKFLOW = Path(__file__).resolve().parents[1] / '.github/workflows/e2e.yaml'
# Reuse the parser already installed by make test-e2e-metrics.
sys.path.insert(0, str(WORKFLOW.parents[2] / '.e2e-artifacts/replay-python'))
import yaml


class CacheTest(unittest.TestCase):
    def setUp(self):
        self.steps = yaml.safe_load(WORKFLOW.read_text())['jobs']['e2e-build']['steps']

    def step(self, name):
        return next(step for step in self.steps if step['name'] == name)

    def run_step(self, name, **environment):
        with tempfile.TemporaryDirectory() as temporary:
            directory = Path(temporary)
            go = directory / 'go'
            go.write_text('#!/bin/sh\ncase "$2" in GOMODCACHE) echo /cache/modules;; '
                          'GOCACHE) echo /cache/build;; *) exit 1;; esac\n')
            go.chmod(0o755)
            output, summary = directory / 'output', directory / 'summary'
            result = subprocess.run(
                ['bash', '-c', self.step(name)['run']], capture_output=True, text=True,
                env={**os.environ, 'PATH': str(directory) + os.pathsep + os.environ['PATH'],
                     'GITHUB_OUTPUT': str(output), 'GITHUB_STEP_SUMMARY': str(summary), **environment},
            )
            return result, output.read_text() if output.exists() else '', summary.read_text() if summary.exists() else ''

    def test_setup_go_cache_contract_requires_review_on_action_update(self):
        # Reviewed v7 source: src/cache-restore.ts and src/package-managers.ts
        # https://github.com/actions/setup-go/tree/b7ad1dad31e06c5925ef5d2fc7ad053ef454303e/src
        self.assertEqual(
            self.step('Setup Go')['uses'],
            'actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e',
            'Review the new setup-go cache key format and ordered cache paths, '
            'then update this reviewed pin and the documented namespace expectations.',
        )

    def test_fallback_partition_uses_documented_namespace(self):
        # Pinned setup-go's Linux key uses RUNNER_OS, process.arch, ImageOS,
        # exact Go version, then the dependency hash. Paths are GOMODCACHE,
        # GOCACHE, in that order. This is an offline contract snapshot, not
        # a live comparison with upstream; the revision guard forces review.
        for arch, image, version in [('X64', 'ubuntu24', '1.26.0'), ('ARM64', 'ubuntu24', '1.26.0'),
                                     ('X64', 'ubuntu26', '1.26.0'), ('X64', 'ubuntu24', '1.26.1')]:
            with self.subTest(arch=arch, image=image, version=version):
                result, output, _ = self.run_step('Resolve Go fallback cache', RUNNER_OS='Linux',
                                                RUNNER_ARCH=arch, ImageOS=image, GO_VERSION=version)
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertEqual(output, 'mod=/cache/modules\nbuild=/cache/build\n'
                                 f'prefix=setup-go-Linux-{arch.lower()}-{image}-go-{version}-\n')
        result, _, _ = self.run_step('Resolve Go fallback cache', GO_VERSION='', ImageOS='ubuntu24')
        self.assertNotEqual(result.returncode, 0)

    def test_fallback_is_optional_and_does_not_skip_compilation(self):
        setup = self.step('Setup Go')
        paths = self.step('Resolve Go fallback cache')
        restore = self.step('Restore Go dependency fallback')
        self.assertTrue(setup['with']['cache'])
        self.assertEqual(setup['with']['cache-dependency-path'].split(), ['go.mod', 'go.sum'])
        self.assertEqual(paths['if'], "steps.setup-go.outputs.cache-hit != 'true' && runner.os == 'Linux'")
        self.assertEqual(restore['if'], "steps.go-cache-paths.outcome == 'success'")
        self.assertTrue(paths['continue-on-error'])
        self.assertTrue(restore['continue-on-error'])
        self.assertEqual(restore['timeout-minutes'], 2)
        self.assertIn('actions/cache/restore@', restore['uses'])
        self.assertEqual(restore['with']['restore-keys'], '${{ steps.go-cache-paths.outputs.prefix }}')
        self.assertEqual(restore['with']['key'],
                         "${{ steps.go-cache-paths.outputs.prefix }}${{ hashFiles('go.mod', 'go.sum') }}")
        self.assertEqual(restore['with']['path'].splitlines(),
                         ['${{ steps.go-cache-paths.outputs.mod }}', '${{ steps.go-cache-paths.outputs.build }}'])
        for name in ['Build kongctl', 'Build scenario test binary', 'Validate scenario allocation offline']:
            self.assertNotIn('if', self.step(name))
            self.assertGreater(self.steps.index(self.step(name)), self.steps.index(restore))

    def test_report_distinguishes_exact_fallback_cold_and_failed_restore(self):
        for hit, key, outcome, expected in [('true', '', 'skipped', 'exact'),
                                          ('false', 'previous-key', 'success', 'fallback'),
                                          ('false', '', 'success', 'cold'),
                                          ('false', 'previous-key', 'failure', 'cold'),
                                          ('', '', 'skipped', 'cold')]:
            with self.subTest(hit=hit, key=key, outcome=outcome):
                result, _, summary = self.run_step('Report Go cache status', CACHE_HIT=hit,
                                                  FALLBACK_KEY=key, FALLBACK_OUTCOME=outcome)
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertIn(f'Go build cache result (dependency-fallback-v1): {expected}', summary)
                self.assertIn(f'Go build cache primary-key hit: `{hit}`', summary)


if __name__ == '__main__':
    unittest.main()
