"""Exercise the workflow's artifact naming and partial-rerun verification."""

import fnmatch
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / '.e2e-artifacts/replay-python'))
import yaml

import e2e_ci_diagnose as diagnose


class ResultsTest(unittest.TestCase):
    def setUp(self):
        self.jobs = yaml.safe_load((ROOT / '.github/workflows/e2e.yaml').read_text())['jobs']

    def step(self, job, name):
        return next(step for step in self.jobs[job]['steps'] if step.get('name') == name)

    def test_setup_failure_reports_that_scenarios_did_not_run(self):
        step = self.step('e2e', 'Summarize scenario diagnostics')
        with tempfile.TemporaryDirectory() as directory:
            summary = Path(directory) / 'e2e-details.md'
            result = subprocess.run(['bash', '-e', '-c', step['run']], capture_output=True, text=True,
                                    env={**os.environ, 'SCENARIO_ARTIFACTS_DIR': '',
                                         'SCENARIOS_OUTCOME': 'skipped', 'SETUP_DECK_OUTCOME': 'failure',
                                         'GITHUB_STEP_SUMMARY': str(summary), 'RUNNER_TEMP': str(summary.parent)})
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertIn('Scenarios did not run: Setup deck failed.', summary.read_text())
            self.assertIn('Shard metrics unavailable', summary.read_text())
            self.assertNotIn('No terminal failures', summary.read_text())

    def test_metrics_require_started_scenarios_and_fail_nonfatally(self):
        step = self.step('e2e', 'Generate shard metrics')
        self.assertEqual(step['if'], "always() && steps.scenarios.outputs.artifacts_dir != ''")
        self.assertTrue(step['continue-on-error'])
        upload = self.step('e2e', 'Upload shard metrics')
        # outcome retains a collector failure even with continue-on-error.
        self.assertEqual(upload['if'], "always() && steps.metrics.outcome == 'success'")
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            summary = root / 'e2e-details.md'
            result = subprocess.run(['bash', '-e', '-c', step['run']], cwd=ROOT,
                                    capture_output=True, text=True,
                                    env={**os.environ, 'KONGCTL_E2E_ARTIFACTS_DIR': str(root / 'missing'),
                                         'GITHUB_STEP_SUMMARY': str(summary), 'RUNNER_TEMP': str(summary.parent)})
            self.assertEqual(result.returncode, 1)
            self.assertIn('::warning::Shard metrics unavailable', result.stdout)
            self.assertIn('collection failed', summary.read_text())
            self.assertFalse((root / 'missing/e2e-metrics.json').exists())

    def test_consolidated_report_survives_failure_and_keeps_evidence(self):
        job = self.jobs['e2e-summary']
        self.assertEqual(job['if'], 'always()')
        self.assertEqual(job['permissions'], {'actions': 'read', 'contents': 'read'})
        self.assertIn('e2e-verify', job['needs'])
        self.assertIn('e2e-required', job['needs'])
        publish = self.step('e2e-summary', 'Publish consolidated summary')
        self.assertEqual(publish['if'], 'always()')
        self.assertIn('INCOMPLETE', publish['run'])
        upload = self.step('e2e-summary', 'Upload consolidated report')
        self.assertEqual(upload['if'], 'always()')
        self.assertTrue(upload['with']['include-hidden-files'])
        routing = self.step('e2e-build', 'Upload routing plan')['with']['path'].split()
        self.assertEqual(routing, ['e2e-routing.json', 'e2e-build-details.md'])
        for name, other in self.jobs.items():
            if name == 'e2e-summary':
                continue
            for step in other.get('steps', []):
                self.assertNotIn('GITHUB_STEP_SUMMARY', step.get('run', ''))
                self.assertNotIn('core.summary', step.get('with', {}).get('script', ''))

    def test_org_lock_admission_guards_remote_scenarios(self):
        guard = self.step('e2e', 'Recheck PR head before scenarios')
        steps = self.jobs['e2e']['steps']
        self.assertLess(steps.index(guard), steps.index(self.step('e2e', 'Run assigned Konnect scenarios')))
        self.assertEqual("steps.revision.outputs.accepted != 'false'",
                         self.step('e2e', 'Run assigned Konnect scenarios')['if'])
        self.assertEqual('revision', guard['id'])
        self.assertEqual('read', self.jobs['e2e']['permissions']['pull-requests'])

    def test_org_lock_guard_checks_api_head_and_records_skip_evidence(self):
        step = self.step('e2e', 'Recheck PR head before scenarios')
        script = step['with']['script']
        harness = r"""
const AsyncFunction = Object.getPrototypeOf(async function(){}).constructor;
const outputs = {};
const notices = [];
const github = {rest: {pulls: {get: async args => {
  if (args.pull_number !== 12) throw new Error('wrong PR');
  if (process.env.API_FAILURE === 'true') throw new Error('API unavailable');
  return {data: {number: 12, state: process.env.PR_STATE, head: {sha: process.env.CURRENT_SHA}}};
}}}};
const core = {setOutput: (key, value) => {outputs[key] = value;}, notice: value => notices.push(value)};
new AsyncFunction('github', 'context', 'core', 'require', process.env.GUARD_SCRIPT)(
  github, {repo: {owner: 'Kong', repo: 'kongctl'}, runId: 99}, core, require
).then(() => console.log(JSON.stringify({outputs, notices}))).catch(error => {
  console.error(error.message); process.exitCode = 1;
});
"""
        for state, head, failure in [('open', 'old', False), ('open', 'new', False),
                                     ('closed', 'old', False), ('open', 'old', True)]:
            with self.subTest(state=state, head=head, failure=failure), tempfile.TemporaryDirectory() as temporary:
                result = subprocess.run(['node', '-e', harness], capture_output=True, text=True, env={
                    **os.environ, 'GUARD_SCRIPT': script, 'VALIDATION_PR': '12', 'VALIDATION_SHA': 'old',
                    'CURRENT_SHA': head, 'PR_STATE': state, 'API_FAILURE': str(failure).lower(),
                    'KONGCTL_E2E_ARTIFACTS_DIR': temporary, 'KONGCTL_E2E_MATRIX_ORG': 'org',
                    'GITHUB_RUN_ATTEMPT': '1',
                })
                if failure:
                    self.assertNotEqual(result.returncode, 0)
                    self.assertIn('API unavailable', result.stderr)
                    continue
                self.assertEqual(result.returncode, 0, result.stderr)
                data = json.loads(result.stdout)
                accepted = state == 'open' and head == 'old'
                self.assertEqual(str(accepted).lower(), data['outputs']['accepted'])
                evidence = Path(temporary) / 'e2e-admission.json'
                self.assertEqual(not accepted, evidence.exists())
                if not accepted:
                    record = json.loads(evidence.read_text())
                    self.assertEqual(99, record['run_id'])
                    self.assertIn(f'current head {head}', record['reason'])
                    self.assertIn('queued SHA old', data['notices'][0])

    def test_diagnostics_resolves_readiness_runs_on_default_branch(self):
        pr = diagnose.PRInfo(12, "Example", "", "feature", "reviewed", "main")
        ready = {"id": 200, "event": "workflow_dispatch", "head_sha": "default-branch",
                 "display_title": "readiness:12/reviewed/com/pr/100"}
        def response(repo, path):
            return {"workflow_runs": [ready]} if "event=workflow_dispatch" in path and "page=1" in path else {
                "workflow_runs": []}
        with patch.object(diagnose, 'gh_api_json', side_effect=response):
            runs = diagnose.resolve_pr_runs_from_workflow_runs('Kong/kongctl', 'e2e.yaml', pr, 2)
        self.assertEqual([200], [run.database_id for run in runs])
        self.assertFalse(diagnose.workflow_run_matches_pr({**ready, "display_title": "readiness:123/reviewed/com/pr/100"}, pr))
        self.assertFalse(diagnose.workflow_run_matches_pr({**ready, "display_title": "readiness:12/old/com/pr/100"}, pr))

    def test_merge_queue_and_controller_share_one_required_file_pattern(self):
        step = self.step('e2e-needed', 'Decide whether scenario suite is required')
        self.assertIn('node scripts/e2e-readiness.cjs --required-files', step['run'])
        for filename in ['scripts/e2e-readiness.cjs', '.github/workflows/e2e-required-status.yaml', 'README.md']:
            with tempfile.NamedTemporaryFile() as paths:
                paths.write(filename.encode() + b'\0')
                paths.flush()
                result = subprocess.run(['node', 'scripts/e2e-readiness.cjs', '--required-files', paths.name],
                                        cwd=ROOT, capture_output=True, text=True, check=True)
            self.assertEqual('false' if filename == 'README.md' else 'true', result.stdout)
        checkout = self.step('e2e-needed', 'Checkout trusted admission controller')
        self.assertEqual("inputs.dispatch_source == 'readiness'", checkout['if'])
        self.assertEqual(1, checkout['with']['fetch-depth'])

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
            summary = workspace / 'e2e-details.md'
            completed = subprocess.run(
                ['bash', '-c', self.step('e2e-verify', 'Verify scenario coverage and results')['run']],
                cwd=workspace, capture_output=True, text=True,
                env={**os.environ, 'GITHUB_WORKSPACE': str(workspace), 'GITHUB_RUN_ID': '123',
                     'GITHUB_EVENT_NAME': event, 'REPLAY_VERIFY_RESULT': 'success',
                     'READINESS_DISPATCH': 'true' if event == 'pull_request' else 'false',
                     'GITHUB_STEP_SUMMARY': str(summary), 'RUNNER_TEMP': str(summary.parent), 'TMPDIR': str(workspace)},
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
