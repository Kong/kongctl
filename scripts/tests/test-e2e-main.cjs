const {test} = require('node:test');
const assert = require('node:assert/strict');
const {readFileSync} = require('node:fs');
const {reconcile, admit, relevant, blockers, NIGHT, PREFIX, STATE_REF} = require('../e2e-main.cjs');
const HEAD = 'a'.repeat(40);
const BASE = 'b'.repeat(40);

function fake() {
  const s = {head: HEAD, stored: null, runs: [], pulls: [], files: [{filename: 'main.go'}],
    dispatches: [], outputs: {}, warnings: [], writes: 0, role: 'maintain', jobs: []};
  const method = name => Object.assign(() => {}, {method: name});
  const github = {rest: {
    git: {
      getRef: async ({ref}) => {
        if (ref !== STATE_REF) return {data: {object: {sha: s.head}}};
        if (!s.stored) throw Object.assign(new Error('missing'), {status: 404});
        return {data: {object: {sha: 'state-commit'}}};
      },
      createTree: async ({tree}) => {s.tree = JSON.parse(tree[0].content); return {data: {sha: 'tree'}};},
      createCommit: async () => ({data: {sha: 'next-state'}}),
      createRef: async () => {s.stored = structuredClone(s.tree); s.writes++;},
      updateRef: async () => {
        if (s.writeError) throw new Error('state write failed');
        s.stored = structuredClone(s.tree); s.writes++;
      },
    },
    repos: {
      getContent: async () => ({data: {content: Buffer.from(JSON.stringify(s.stored)).toString('base64')}}),
      compareCommitsWithBasehead: async () => ({data: {status: 'ahead', files: s.files,
        total_commits: 2, commits: [{sha: 'one'}, {sha: 'two'}]}}),
      getCollaboratorPermissionLevel: async () => ({data: {role_name: s.role}}),
      listCommitStatusesForRef: method('statuses'),
    },
    pulls: {list: method('pulls'), get: async () => ({data: s.pulls[0]}),
      listFiles: method('files'), listReviews: method('reviews')},
    actions: {listWorkflowRuns: method('runs'), listWorkflowRunsForRepo: method('ci'),
      listJobsForWorkflowRun: method('jobs'),
      getWorkflowRun: async ({run_id}) => {
        if (run_id !== context.runId) {
          const run = s.runs.find(run => run.id === run_id);
          if (!run) throw Object.assign(new Error('missing run'), {status: 404});
          return {data: structuredClone(run)};
        }
        return {data: {actor: {type: 'User', login: 'maintainer'},
          triggering_actor: {type: 'User', login: 'maintainer'}}};
      },
      createWorkflowDispatch: async dispatch => {
        assert.ok(s.stored.reservation, 'persist reservation before API dispatch');
        s.dispatches.push(dispatch);
        if (s.dispatchError) throw new Error('ambiguous dispatch');
      },
    },
  }, paginate: async (method, args) => structuredClone((s[method.method] || [])
    .filter(item => !args.status || item.status === args.status)),
  graphql: async query => query.includes('node(id:') ? {node: s.requestNode} :
    {repository: {pullRequest: {reviewDecision: 'APPROVED',
      reviewThreads: {nodes: s.threads || [], pageInfo: {hasNextPage: false}}}}}};
  const context = {repo: {owner: 'Kong', repo: 'kongctl'}, eventName: 'schedule', runId: 100,
    payload: {repository: {default_branch: 'main'}, schedule: NIGHT}};
  const core = {info() {}, warning: message => s.warnings.push(message),
    setOutput: (key, value) => {s.outputs[key] = value;},
    summary: {addHeading() {return this;}, addTable(rows) {s.summary = rows; return this;}, async write() {}}};
  process.env.GITHUB_REF = 'refs/heads/main';
  process.env.GITHUB_RUN_ATTEMPT = '1';
  process.env.KONNECT_ENVIRONMENT = '';
  return {s, github, context, core};
}

function completed(f, outcome = 'success') {
  const {sha, ticket} = f.s.stored.reservation;
  f.s.runs = [{id: 200, display_title: `${PREFIX}${sha} · ${ticket}`, status: 'completed',
    conclusion: outcome, updated_at: new Date().toISOString(), html_url: 'https://github.com/Kong/kongctl/actions/runs/200'}];
  f.s.jobs = [{name: 'Verify scenario results and coverage', conclusion: outcome},
    {name: 'Determine scenario requirement', conclusion: 'success'}];
  f.context.payload.schedule = '*/15 * * * *';
}

function pull(f) {
  const pr = {number: 12, state: 'open', draft: false, changed_files: 1, labels: [], user: {login: 'author'},
    head: {sha: BASE, repo: {full_name: 'Kong/kongctl'}}, base: {repo: {full_name: 'Kong/kongctl'}}};
  f.s.pulls = [pr];
  f.s.reviews = [{state: 'APPROVED', commit_id: BASE, user: {type: 'User', login: 'maintainer'},
    author_association: 'MEMBER'}];
  f.s.ci = ['test', 'checks'].map((name, id) => ({id, path: `.github/workflows/${name}.yaml`,
    event: 'pull_request', head_sha: BASE, pull_requests: [{number: 12}], status: 'completed', conclusion: 'success'}));
}

test('merge batch resolves newest main, reserves once even before run visibility, and pins checkout', async () => {
  const f = fake();
  f.s.head = 'c'.repeat(40);
  await reconcile(f);
  await reconcile(f);
  assert.equal(f.s.dispatches.length, 1);
  assert.equal(f.s.dispatches[0].inputs.main_sha, f.s.head);
  const workflow = readFileSync('.github/workflows/e2e.yaml', 'utf8');
  assert.match(workflow, /status_sha="\$\{MAIN_SHA\}"\s+checkout_ref="\$\{MAIN_SHA\}"/);
  assert.match(workflow, /id: main-admission/);
});

test('idle night skips unchanged or irrelevant main and avoids rewriting durable state', async () => {
  const f = fake();
  f.s.stored = {version: 1, success: {sha: HEAD, at: new Date().toISOString()}, opportunity: false};
  await reconcile(f);
  assert.equal(f.s.dispatches.length, 0);
  assert.equal(f.s.writes, 0);
  f.s.head = BASE;
  f.s.files = [{filename: 'docs/guide.md'}];
  await reconcile(f);
  assert.equal(f.s.dispatches.length, 0);
});

test('daytime pushes wait for an overnight opportunity; deferred night retries by day', async () => {
  const f = fake();
  f.context.payload.schedule = '*/15 * * * *';
  await reconcile(f);
  assert.equal(f.s.dispatches.length, 0);
  f.context.payload.schedule = NIGHT;
  f.s.runs = [{id: 1, status: 'queued'}];
  await reconcile(f);
  assert.equal(f.s.dispatches.length, 0);
  assert.equal(f.s.stored.opportunity, true);
  f.context.payload.schedule = '*/15 * * * *';
  f.s.runs = [];
  f.s.head = BASE;
  await reconcile(f);
  assert.equal(f.s.dispatches[0].inputs.main_sha, BASE);
});

for (const stage of ['queued', 'in_progress', 'waiting', 'pending']) {
  test(`busy night yields to ${stage} live use`, async () => {
    const f = fake();
    f.s.runs = [{id: 1, status: stage}];
    await reconcile(f);
    assert.equal(f.s.dispatches.length, 0);
  });
}

test('ready PR and prerequisite CI get priority, but drafts/review blockers/completed PRs do not', async () => {
  const f = fake();
  pull(f);
  assert.match((await blockers(f)).join(), /ready for E2E/);
  f.s.ci[0].status = 'in_progress';
  assert.match((await blockers(f)).join(), /prerequisite CI/);
  const reviews = f.s.reviews;
  f.s.reviews = [];
  assert.deepEqual(await blockers(f), [], 'unreviewed PR is not waiting only on CI');
  f.s.reviews = reviews;
  f.s.pulls[0].draft = true;
  assert.deepEqual(await blockers(f), []);
  f.s.pulls[0].draft = false;
  f.s.threads = [{isResolved: false}];
  assert.deepEqual(await blockers(f), []);
  f.s.threads = [];
  f.s.ci[0].status = 'completed';
  for (const state of ['success', 'failure', 'error']) {
    f.s.statuses = [{context: 'E2E Validation / 12 / com / pr', state}];
    assert.deepEqual(await blockers(f), []);
  }
  f.s.statuses = [];
  f.s.ci[0].conclusion = 'failure';
  assert.deepEqual(await blockers(f), []);
  f.s.ci[1].status = 'in_progress';
  assert.deepEqual(await blockers(f), [], 'failed prerequisite prevents near-ready priority');
});

test('a visible active reservation is retained through completion and cannot be manually recovered', async () => {
  const f = fake();
  await reconcile(f);
  completed(f);
  f.s.runs[0].status = 'in_progress';
  f.s.runs[0].conclusion = null;
  await reconcile(f);
  assert.equal(f.s.stored.reservation.run, 200);
  f.context.eventName = 'workflow_dispatch';
  f.context.payload.inputs = {recover_reservation: 'true'};
  await reconcile(f);
  assert.equal(f.s.dispatches.length, 1);
  f.context.eventName = 'schedule';
  f.context.payload.inputs = {};
  completed(f);
  await reconcile(f);
  assert.equal(f.s.stored.success.sha, HEAD);
});

test('a fresh exact-head /e2e request takes priority over old terminal validation evidence', async () => {
  const f = fake();
  pull(f);
  f.s.reviews = [{id: 42, node_id: 'review', state: 'COMMENTED', body: '/e2e', commit_id: BASE,
    user: {type: 'User', login: 'maintainer'}}];
  f.s.requestNode = {fullDatabaseId: '42', body: '/e2e', state: 'COMMENTED', submittedAt: '2026-11-20T00:00:00Z',
    author: {login: 'maintainer'}, commit: {oid: BASE},
    pullRequest: {number: 12, repository: {nameWithOwner: 'Kong/kongctl'}}};
  f.s.statuses = [{context: 'E2E Validation / 12 / com / pr', state: 'failure',
    description: 'request=review-41 E2E failed.'}];
  assert.match((await blockers(f)).join(), /ready for E2E/);
  f.s.statuses[0].description = 'request=review-42 E2E failed.';
  assert.deepEqual(await blockers(f), []);
});

test('a deleted reserved run requires explicit recovery and queue admission requires the exact ticket', async () => {
  const f = fake();
  await reconcile(f);
  f.context.payload.inputs = {...f.s.dispatches[0].inputs, main_ticket: 'wrong'};
  assert.equal((await admit(f)).accepted, false);
  f.s.stored.reservation.run = 200;
  f.s.stored.reservation.at = new Date(Date.now() - 11 * 60000).toISOString();
  await reconcile(f);
  assert.equal(f.s.dispatches.length, 1);
  f.context.eventName = 'workflow_dispatch';
  f.context.payload.inputs = {recover_reservation: 'true'};
  await reconcile(f);
  assert.equal(f.s.dispatches.length, 2);
});

test('late-arriving PR and newer merge revoke queued admission, then retry latest main', async () => {
  const f = fake();
  await reconcile(f);
  f.context.payload.inputs = f.s.dispatches[0].inputs;
  f.context.runId = 200;
  f.s.runs = [{id: 200, status: 'in_progress'}];
  assert.equal((await admit(f)).accepted, true, 'ignore our own queued run');
  pull(f);
  assert.equal((await admit(f)).accepted, false);
  f.s.pulls = [];
  f.s.head = BASE;
  assert.match((await admit(f)).reason, /superseded/);
  completed(f);
  f.s.jobs[0].conclusion = 'skipped';
  f.s.jobs[1].steps = [{name: 'Main validation deferred before live work', conclusion: 'success'}];
  await reconcile(f);
  assert.equal(f.s.dispatches.length, 2);
  assert.equal(f.s.dispatches[1].inputs.main_sha, BASE);
  assert.equal(f.s.stored.success, undefined);
});

test('success records exact tested SHA; a newer merge during live execution stays eligible', async () => {
  const f = fake();
  await reconcile(f);
  f.s.head = BASE;
  completed(f);
  await reconcile(f);
  assert.equal(f.s.stored.success.sha, HEAD);
  assert.equal(f.s.dispatches.length, 1);
  f.context.payload.schedule = NIGHT;
  await reconcile(f);
  assert.equal(f.s.dispatches[1].inputs.main_sha, BASE);
});

for (const outcome of ['failure', 'cancelled', 'timed_out']) {
  test(`${outcome} remains visible without automatic daytime redispatch`, async () => {
    const f = fake();
    await reconcile(f);
    completed(f, outcome);
    await reconcile(f);
    await reconcile(f);
    assert.equal(f.s.stored.success, undefined);
    assert.equal(f.s.stored.last.outcome, outcome);
    assert.equal(f.s.dispatches.length, 1);
    assert.ok(f.s.warnings.some(w => w.includes(outcome)));
    f.context.payload.schedule = NIGHT;
    await reconcile(f);
    assert.equal(f.s.dispatches.length, 2);
  });
}

test('success without successful coverage verification never advances baseline', async () => {
  const f = fake();
  await reconcile(f);
  completed(f);
  f.s.jobs = [];
  await reconcile(f);
  assert.equal(f.s.stored.success, undefined);
});

test('cancellation after admission and skipped verification is a failure, not a deferred opportunity', async () => {
  const f = fake();
  await reconcile(f);
  completed(f, 'cancelled');
  f.s.jobs[0].conclusion = 'skipped';
  await reconcile(f);
  assert.equal(f.s.dispatches.length, 1);
  assert.equal(f.s.stored.failure.outcome, 'cancelled');
});

test('a later queue deferral preserves the previous failed validation and last success', async () => {
  const f = fake();
  f.s.stored = {version: 1, success: {sha: BASE, at: '2026-01-01T00:00:00Z'}};
  await reconcile(f);
  completed(f, 'failure');
  await reconcile(f);
  const failure = f.s.stored.failure;
  f.context.payload.schedule = NIGHT;
  await reconcile(f);
  completed(f);
  f.s.jobs[0].conclusion = 'skipped';
  f.s.jobs[1].steps = [{name: 'Main validation deferred before live work', conclusion: 'success'}];
  pull(f);
  await reconcile(f);
  assert.deepEqual(f.s.stored.failure, failure);
  assert.equal(f.s.stored.success.sha, BASE);
});

test('manual run-now is maintainer-only, respects PRs and cannot duplicate reservations', async () => {
  const f = fake();
  f.context.eventName = 'workflow_dispatch';
  f.context.payload.inputs = {};
  f.s.role = 'write';
  await assert.rejects(reconcile(f), /Maintain or Admin/);
  f.s.role = 'maintain';
  pull(f);
  await reconcile(f);
  assert.equal(f.s.dispatches.length, 0);
  f.s.pulls = [];
  await reconcile(f);
  await reconcile(f);
  assert.equal(f.s.dispatches.length, 1);
});

test('ambiguous API failure keeps reservation and explicit recovery enforces visibility grace', async () => {
  const f = fake();
  f.s.dispatchError = true;
  await assert.rejects(reconcile(f), /ambiguous/);
  f.s.dispatchError = false;
  await reconcile(f);
  assert.equal(f.s.dispatches.length, 1);
  f.context.eventName = 'workflow_dispatch';
  f.context.payload.inputs = {recover_reservation: 'true'};
  await assert.rejects(reconcile(f), /ten minutes/);
  f.s.stored.reservation.at = new Date(Date.now() - 11 * 60000).toISOString();
  await reconcile(f);
  assert.equal(f.s.dispatches.length, 2);
});

test('durable state errors prevent dispatch and unknown state formats fail closed', async () => {
  const f = fake();
  f.s.stored = {version: 2};
  await assert.rejects(reconcile(f), /version/);
  f.s.stored = {version: 1};
  f.s.writeError = true;
  await assert.rejects(reconcile(f), /state write/);
  assert.equal(f.s.dispatches.length, 0);
});

test('prolonged deferral warns with pending time and validation age', async () => {
  const f = fake();
  f.s.stored = {version: 1, success: {sha: BASE, at: '2026-01-01T00:00:00Z'},
    pendingSince: '2026-01-01T00:00:00Z', opportunity: true};
  f.s.runs = [{id: 1, status: 'queued'}];
  await reconcile(f);
  assert.ok(f.s.warnings.some(w => /pending .* hours/.test(w)));
  assert.ok(f.s.summary.some(row => row[0] === 'Validation age (hours)'));
});

test('renames, comparison limits and missing baseline preserve relevant validation', async () => {
  const f = fake();
  assert.equal(await relevant(f.github, f.context, null, HEAD), true);
  assert.equal(await relevant(f.github, f.context, HEAD, HEAD), false);
  f.s.files = [{filename: 'docs/moved', previous_filename: 'main.go'}];
  assert.equal(await relevant(f.github, f.context, BASE, HEAD), true);
  f.s.files = Array.from({length: 300}, () => ({filename: 'docs/guide.md'}));
  assert.equal(await relevant(f.github, f.context, BASE, HEAD), true);
});
