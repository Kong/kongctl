"""Exercise the workflow's artifact naming and partial-rerun verification."""

import fnmatch
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / '.e2e-artifacts/replay-python'))
import yaml

import e2e_ci_diagnose as diagnose


class ResultsTest(unittest.TestCase):
    def setUp(self):
        self.jobs = yaml.safe_load((ROOT / '.github/workflows/e2e.yaml').read_text())['jobs']

    def step(self, job, name):
        return next(step for step in self.jobs[job]['steps'] if step.get('name') == name)

    def artifact_name(self, attempt, org):
        name = self.step('e2e', 'Upload scenario results and diagnostics')['with']['name']
        return (name.replace('${{ github.run_id }}', '123')
                .replace('${{ github.run_attempt }}', str(attempt))
                .replace('${{ matrix.org_name }}', org))

    def test_artifact_names_preserve_all_attempts_for_verification(self):
        names = [self.artifact_name(attempt, org)
                 for attempt in [1, 2, 10] for org in ['acceptance', 'acceptance-2']]
        self.assertEqual(len(set(names)), len(names),
                         'Each attempt needs a distinct name: artifact IDs do not order reruns reliably.')
        download = self.step('e2e-verify', 'Download shard results')['with']
        pattern = download['pattern'].replace('${{ github.run_id }}', '123')
        for name in names:
            self.assertTrue(fnmatch.fnmatchcase(name, pattern), name)
        self.assertFalse(download.get('merge-multiple', False))

    def run_verifier(self, event, newest_passes=True, omit_result=False):
        with tempfile.TemporaryDirectory() as temporary:
            workspace = Path(temporary)
            helper = workspace / 'test/e2e/harness/ci/results.sh'
            helper.parent.mkdir(parents=True)
            helper.symlink_to(ROOT / 'test/e2e/harness/ci/results.sh')
            scenarios = ['first/scenario.yaml', 'second/scenario.yaml']
            for scenario in scenarios:
                path = workspace / 'test/e2e/scenarios' / scenario
                path.parent.mkdir(parents=True)
                path.touch()
            (workspace / 'e2e-routing.json').write_text(json.dumps({'live': scenarios}))
            # Only shard zero reruns. Attempt 10 must supersede both 1 and 2,
            # while shard one's original successful result remains eligible.
            for attempt, index, passes in [(1, 0, not newest_passes), (1, 1, True),
                                          (2, 0, not newest_passes), (10, 0, newest_passes)]:
                org = f'acceptance-{index}'
                # Keep a legacy artifact for the shard that did not rerun.
                name = f'e2e-artifacts-123-{org}' if index == 1 else self.artifact_name(attempt, org)
                directory = workspace / '.downloaded-artifacts' / name / f'results-{attempt}'
                directory.mkdir(parents=True)
                scenario = scenarios[index]
                (directory / 'assigned-scenarios.txt').write_text(
                    f'shard_index={index}\nshard_total=2\n\n{scenario}\n')
                result = '' if omit_result and attempt == 10 else scenario
                (directory / 'scenario-results.txt').write_text(
                    f'org_name={org}\nrun_attempt={attempt}\nexit_code={int(not passes)}\n'
                    f'duration_seconds=1\npassed_count={int(passes)}\nfailed_count={int(not passes)}\n'
                    'skipped_count=0\nbeta_failed_count=0\nobserved_count=1\n\n'
                    f'[{"passed" if passes else "failed"}]\n{result}\n')
            summary = workspace / 'summary.md'
            completed = subprocess.run(
                ['bash', '-c', self.step('e2e-verify', 'Verify scenario coverage and results')['run']],
                cwd=workspace, capture_output=True, text=True,
                env={**os.environ, 'GITHUB_WORKSPACE': str(workspace), 'GITHUB_RUN_ID': '123',
                     'GITHUB_EVENT_NAME': event, 'REPLAY_VERIFY_RESULT': 'success',
                     'GITHUB_STEP_SUMMARY': str(summary), 'TMPDIR': str(workspace)},
            )
            return completed, summary.read_text()

    def test_partial_rerun_uses_latest_attempt_per_shard(self):
        for event in ['pull_request', 'push']:
            with self.subTest(event=event):
                result, summary = self.run_verifier(event)
                self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
                self.assertIn('- Failed: 0', summary)
                self.assertIn('| 0 | 10 | acceptance-0 |', summary)
                self.assertIn('| 1 | 1 | acceptance-1 |', summary)

    def test_latest_failed_attempt_still_blocks(self):
        result, _ = self.run_verifier('pull_request', newest_passes=False)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('1 scenario(s) failed; 1 shard(s) exited non-zero', result.stdout)

    def test_latest_attempt_cannot_omit_a_scenario_result(self):
        result, _ = self.run_verifier('pull_request', omit_result=True)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('Scenario result coverage does not match shard assignments', result.stdout)

    def test_diagnostic_org_filters_support_old_and_attempt_specific_names(self):
        org = 'kongctl-acceptance-4'
        for name in [f'e2e-artifacts-123-{org}', self.artifact_name(2, org)]:
            with self.subTest(name=name):
                artifact = diagnose.Artifact(42, name, False, 100, '', '')
                self.assertEqual(artifact.org_name, org)
                self.assertEqual(diagnose.infer_org_from_artifact_dir(Path(name) / 'results'), org)
                self.assertEqual(diagnose.select_artifacts([artifact], [], [org], [], False), [artifact])


if __name__ == '__main__':
    unittest.main()
