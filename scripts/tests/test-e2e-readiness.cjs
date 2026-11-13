const {test} = require('node:test');
const assert = require('node:assert/strict');
const {requirement, reviewReady, ciReady, reconcile, admit, REQUIRED, scheduledBatch} = require('../e2e-readiness.cjs');

function fixture() {
  const pr = {number: 12, state: 'open', draft: false, changed_files: 1, labels: [],
    user: {login: 'author'}, head: {sha: 'a'.repeat(40), repo: {full_name: 'Kong/kongctl'}},
    base: {repo: {full_name: 'Kong/kongctl'}}};
  const approval = {state: 'APPROVED', commit_id: pr.head.sha, user: {login: 'human', type: 'User'},
    author_association: 'MEMBER'};
  const ci = ['CI Test', 'Checks'].map((name, id) => ({name, id, head_sha: pr.head.sha,
    pull_requests: [{number: pr.number}], event: 'pull_request', status: 'completed', conclusion: 'success'}));
  return {pr, approval, ci};
}

function fake() {
  const {pr, approval, ci} = fixture();
  const state = {deployed: true, pr, reviews: [approval], ci, runs: [], statuses: [], dispatches: [],
    threads: [], decision: 'APPROVED', files: [{filename: 'main.go'}], jobs: [], outputs: {}};
  const method = name => Object.assign(() => {}, {method: name});
  const github = {rest: {
    pulls: {get: async () => ({data: structuredClone(state.pr)}), list: method('pulls'),
      listFiles: method('files'), listReviews: method('reviews')},
    repos: {getContent: async () => {
      if (!state.deployed) throw Object.assign(new Error('Not found'), {status: 404});
      return {data: {type: 'file'}};
    }, listCommitStatusesForRef: method('statuses'), createCommitStatus: async status => {
      state.statuses.unshift(status);
    }},
    actions: {getWorkflowRun: async ({run_id}) => {
      const run = state.runs.find(run => run.id === run_id);
      if (!run) throw Object.assign(new Error('Not found'), {status: 404});
      return {data: structuredClone(run)};
    }, listWorkflowRunsForRepo: method('ci'), listWorkflowRuns: method('runs'),
      listJobsForWorkflowRun: method('jobs'), createWorkflowDispatch: async dispatch => {
        state.dispatches.push(dispatch);
        if (state.dispatchError) throw new Error('dispatch interrupted');
        return {data: {workflow_run_id: 200}};
      }},
  }, paginate: async method => structuredClone(state[method.method]),
  graphql: async () => ({repository: {pullRequest: {reviewDecision: state.decision,
    reviewThreads: {nodes: state.threads, pageInfo: {hasNextPage: false}}}}})};
  const context = {repo: {owner: 'Kong', repo: 'kongctl'}, eventName: 'schedule',
    payload: {repository: {default_branch: 'main'}, inputs: {pr_number: '12'}}};
  const core = {warning(message) { (state.warnings ||= []).push(message); }, info() {},
    setOutput(name, value) { state.outputs[name] = value; }};
  process.env.GITHUB_RUN_ID = '100';
  process.env.KONNECT_ENVIRONMENT = '';
  return {state, github, context, core};
}

function gate(f) { return f.state.statuses.find(status => status.context === 'E2E Required'); }
function dispatchedRun(f, conclusion = null) {
  const inputs = f.state.dispatches.at(-1).inputs;
  return {id: 200, display_title: `${requirement(f.state.pr).title}/${inputs.readiness_ticket}`,
    html_url: 'https://github.com/Kong/kongctl/actions/runs/200',
    status: conclusion ? 'completed' : 'queued', conclusion};
}

test('review policy requires current human approval and no blocking findings', () => {
  const {pr, approval} = fixture();
  assert.equal(reviewReady(pr, [approval], 'APPROVED', []), true);
  for (const review of [{...approval, user: {login: 'bot', type: 'Bot'}},
    {...approval, commit_id: 'old'}, {...approval, author_association: 'CONTRIBUTOR'}]) {
    assert.equal(reviewReady(pr, [review], null, []), false);
  }
  for (const state of ['DISMISSED', 'CHANGES_REQUESTED']) {
    assert.equal(reviewReady(pr, [approval, {...approval, state}], 'APPROVED', []), false);
  }
  assert.equal(reviewReady(pr, [approval], 'REVIEW_REQUIRED', []), false);
  assert.equal(reviewReady(pr, [approval], 'APPROVED', [{isResolved: false}]), false);
  assert.equal(reviewReady({...pr, draft: true}, [approval], 'APPROVED', []), false);
  assert.equal(reviewReady({...pr, state: 'closed'}, [approval], 'APPROVED', []), false);
});

test('prerequisites require latest explicit ordinary workflows for this revision', () => {
  const {pr, ci} = fixture();
  assert.equal(ciReady(ci, pr), true);
  assert.equal(ciReady(ci.slice(1), pr), false);
  assert.equal(ciReady([...ci, {...ci[0], id: 10, conclusion: 'failure'}], pr), false);
  assert.equal(ciReady(ci, {...pr, head: {...pr.head, sha: 'new'}}), false);
});

for (const last of ['review', 'ci', 'thread']) {
  test(`${last} finishing last admits once despite duplicate evaluations and delayed run visibility`, async () => {
    const f = fake();
    if (last === 'review') f.state.reviews = [];
    if (last === 'ci') f.state.ci[0].conclusion = null;
    if (last === 'thread') f.state.threads = [{isResolved: false}];
    await reconcile(f);
    assert.equal(f.state.dispatches.length, 0);
    assert.equal(gate(f).state, 'pending');
    f.state.reviews = [fixture().approval];
    f.state.ci = fixture().ci;
    f.state.threads = [];
    await reconcile(f);
    await reconcile(f);
    assert.equal(f.state.dispatches.length, 1);
    assert.equal(f.state.dispatches[0].ref, 'main');
    f.state.runs = [dispatchedRun(f)];
    await reconcile(f);
    assert.equal(f.state.dispatches.length, 1);
  });
}

test('successful validation survives incidental events; labels and commits require different evidence', async () => {
  const f = fake();
  await reconcile(f);
  f.state.runs = [dispatchedRun(f, 'success')];
  f.state.jobs = [{name: 'Verify scenario results and coverage', conclusion: 'success'}];
  await reconcile(f);
  assert.equal(gate(f).state, 'success');
  const writes = f.state.statuses.length;
  f.state.pr.labels = [{name: 'unrelated'}];
  await reconcile(f);
  assert.equal(f.state.statuses.length, writes);
  for (const label of ['e2e:force-live', 'konnect-env:tech']) {
    f.state.pr.labels = [{name: label}];
    await reconcile(f);
    assert.equal(gate(f).state, 'pending');
  }
  assert.equal(f.state.dispatches.length, 3);
  f.state.pr.head.sha = 'b'.repeat(40);
  await reconcile(f);
  assert.equal(gate(f).state, 'pending');
  assert.equal(f.state.dispatches.length, 3);
});

for (const conclusion of ['failure', 'cancelled']) {
  test(`${conclusion} requires explicit retry, which cannot duplicate active work`, async () => {
    const f = fake();
    await reconcile(f);
    f.state.runs = [dispatchedRun(f, conclusion)];
    await reconcile(f);
    assert.equal(gate(f).state, 'failure');
    assert.equal(f.state.dispatches.length, 1);
    f.context.eventName = 'workflow_dispatch';
    f.context.payload.inputs.retry = 'true';
    process.env.GITHUB_RUN_ID = '101';
    await reconcile(f);
    f.state.runs.unshift(dispatchedRun(f));
    await reconcile(f);
    assert.equal(f.state.dispatches.length, 2);
  });
}

test('interrupted reservation fails closed and can be retried', async () => {
  const f = fake();
  f.state.dispatchError = true;
  await assert.rejects(reconcile(f), /interrupted/);
  f.state.dispatchError = false;
  await reconcile(f);
  assert.equal(f.state.dispatches.length, 1);
  f.context.eventName = 'workflow_dispatch';
  f.context.payload.inputs.retry = 'true';
  await reconcile(f);
  assert.equal(f.state.dispatches.length, 2);
});

test('skipped admission is never success and retries automatically when ready again', async () => {
  const f = fake();
  await reconcile(f);
  f.state.runs = [dispatchedRun(f, 'success')];
  f.state.jobs = [{name: 'Verify scenario results and coverage', conclusion: 'skipped'},
    {name: 'Determine scenario requirement', conclusion: 'success'}];
  f.state.pr.draft = true;
  await reconcile(f);
  assert.equal(gate(f).state, 'pending');
  f.state.pr.draft = false;
  process.env.GITHUB_RUN_ID = '101';
  await reconcile(f);
  await reconcile(f);
  assert.equal(f.state.dispatches.length, 2);
  assert.equal(gate(f).state, 'pending');
});

test('fork readiness never grants secrets; keyed trusted evidence is reused', async () => {
  const f = fake();
  f.state.pr.head.repo.full_name = 'contributor/kongctl';
  await reconcile(f);
  assert.equal(f.state.dispatches.length, 0);
  assert.equal(gate(f).state, 'pending');
  f.state.statuses.unshift({context: requirement(f.state.pr).statusContext,
    state: 'success', description: 'Trusted E2E passed', target_url: 'https://github.com/trusted'});
  await reconcile(f);
  assert.equal(gate(f).state, 'success');
  f.state.pr.labels = [{name: 'konnect-env:tech'}];
  await reconcile(f);
  assert.equal(gate(f).state, 'pending');
});

test('irrelevant diffs pass without review or dispatch; renamed relevant files still require E2E', async () => {
  const f = fake();
  f.state.files = [{filename: 'README.md'}];
  f.state.reviews = [];
  await reconcile(f);
  assert.equal(gate(f).state, 'success');
  assert.equal(f.state.dispatches.length, 0);
  f.state.files[0].previous_filename = 'internal/old.go';
  await reconcile(f);
  assert.equal(gate(f).state, 'pending');
  for (const file of ['scripts/e2e-readiness.cjs', '.github/workflows/e2e-required-status.yaml']) {
    assert.equal(REQUIRED.test(file), true);
  }
});

test('queued admission checks SHA, review, draft, environment, routing and fork status again', async () => {
  const f = fake();
  Object.assign(process.env, {READINESS_PR: '12', READINESS_SHA: f.state.pr.head.sha,
    READINESS_ENV: 'com', READINESS_MODE: 'pr'});
  await admit(f);
  assert.equal(f.state.outputs.accepted, 'true');
  for (const mutate of [s => {s.pr.draft = true;}, s => {s.reviews = [];},
    s => {s.pr.head.sha = 'new';}, s => {s.pr.labels = [{name: 'e2e:force-live'}];},
    s => {s.pr.labels = [{name: 'konnect-env:tech'}];},
    s => {s.pr.head.repo.full_name = 'fork/repo';}]) {
    const changed = fake();
    mutate(changed.state);
    await admit(changed);
    assert.equal(changed.state.outputs.accepted, 'false');
  }
});


test('completed evidence survives workflow history expiry and repeated unrelated events', async () => {
  const f = fake();
  await reconcile(f);
  f.state.runs = [dispatchedRun(f, 'success')];
  f.state.jobs = [{name: 'Verify scenario results and coverage', conclusion: 'success'}];
  await reconcile(f);
  f.state.runs = [];
  await reconcile(f);
  assert.equal(gate(f).state, 'success');
  assert.equal(f.state.dispatches.length, 1);
});

test('stale evaluation cannot publish or dispatch after the requirement changes', async () => {
  const f = fake();
  const get = f.github.rest.pulls.get;
  let calls = 0;
  f.github.rest.pulls.get = async () => {
    if (++calls === 2) f.state.pr.labels = [{name: 'e2e:force-live'}];
    return get();
  };
  await reconcile(f);
  assert.equal(f.state.dispatches.length, 0);
  assert.equal(gate(f), undefined);
});


test('workflow completion targets its PR and manual/main completion relies on reconciliation', async () => {
  for (const run of [
    {pull_requests: [{number: 12}], display_title: 'CI Test'},
    {pull_requests: [], display_title: `readiness:12/${'a'.repeat(40)}/com/pr/100`},
    {pull_requests: [], display_title: `PR #12 · trusted scenarios · ${'a'.repeat(40)}`},
  ]) {
    const f = fake();
    f.context.eventName = 'workflow_run';
    f.context.payload.inputs = {};
    f.context.payload.workflow_run = run;
    await reconcile(f);
    assert.equal(f.state.dispatches.length, 1);
  }
  const f = fake();
  f.context.eventName = 'workflow_run';
  f.context.payload.inputs = {};
  f.context.payload.workflow_run = {pull_requests: [], display_title: 'main · scenarios · debounced · head'};
  await reconcile(f);
  assert.equal(f.state.dispatches.length, 0);
});


test('expensive automatic jobs and verification require admission in the current run attempt', () => {
  const {readFileSync} = require('node:fs');
  const {resolve} = require('node:path');
  const source = readFileSync(resolve(__dirname, '../../.github/workflows/e2e.yaml'), 'utf8');
  for (const name of ['e2e-harness', 'e2e-build', 'e2e', 'e2e-replay', 'e2e-replay-verify', 'e2e-verify']) {
    const job = source.split(`  ${name}:\n`)[1].split('    runs-on:')[0];
    assert.ok(job.includes('needs.e2e-needed.outputs.admission_attempt == github.run_attempt'), name);
  }
  assert.ok(!source.split('jobs:')[0].includes('  pull_request:'));
});

test('rerunning a successful run temporarily invalidates its evidence', async () => {
  const f = fake();
  await reconcile(f);
  f.state.runs = [dispatchedRun(f, 'success')];
  f.state.jobs = [{name: 'Verify scenario results and coverage', conclusion: 'success'}];
  await reconcile(f);
  f.state.runs[0].status = 'in_progress';
  f.state.runs[0].conclusion = null;
  await reconcile(f);
  assert.equal(gate(f).state, 'pending');
  assert.equal(f.state.dispatches.length, 1);
});


test('controller checkout matches its workflow revision before the module exists on main', () => {
  const {readFileSync} = require('node:fs');
  const {resolve} = require('node:path');
  const workflow = readFileSync(resolve(__dirname, '../../.github/workflows/e2e-required-status.yaml'), 'utf8');
  const checkout = workflow.split('      - name: Checkout trusted controller')[1].split('      - name:')[0];
  assert.ok(checkout.includes('ref: ${{ github.workflow_sha }}'));
});

test('undeployed default branch defers dispatch without reserving or falsely passing validation', async () => {
  const f = fake();
  f.state.deployed = false;
  await reconcile(f);
  assert.equal(f.state.dispatches.length, 0);
  assert.equal(gate(f).state, 'pending');
  assert.match(gate(f).description, /default.branch deployment/);
  assert.ok(!f.state.statuses.some(status => status.context === requirement(f.state.pr).statusContext));
  f.state.deployed = true;
  await reconcile(f);
  assert.equal(f.state.dispatches.length, 1);
});


test('a queue of revisions admits only the current head and explains each stale skip', async () => {
  const f = fake();
  const current = f.state.pr.head.sha;
  Object.assign(process.env, {READINESS_PR: '12', READINESS_ENV: 'com', READINESS_MODE: 'pr'});
  for (const sha of ['b'.repeat(40), 'c'.repeat(40), current]) {
    process.env.READINESS_SHA = sha;
    await admit(f);
    assert.equal(f.state.outputs.accepted, String(sha === current));
    if (sha !== current) {
      assert.ok(f.state.outputs.reason.includes(sha));
      assert.ok(f.state.outputs.reason.includes(current));
    }
  }
  assert.equal(f.state.statuses.length, 0);
});

test('an org-lock revision skip is retried when current readiness returns', async () => {
  const f = fake();
  await reconcile(f);
  f.state.runs = [dispatchedRun(f, 'failure')];
  f.state.jobs = [
    {name: 'Determine scenario requirement', conclusion: 'success'},
    {name: 'Verify scenario results and coverage', conclusion: 'failure'},
    {name: 'Run scenarios — org', conclusion: 'success', steps: [
      {name: 'Recheck PR head before scenarios', conclusion: 'success'},
      {name: 'Run assigned Konnect scenarios', conclusion: 'skipped'},
    ]},
  ];
  process.env.GITHUB_RUN_ID = '101';
  await reconcile(f);
  assert.equal(f.state.dispatches.length, 2);
  assert.equal(gate(f).state, 'pending');
});


for (const conclusion of ['failure', 'cancelled']) {
  test(`${conclusion} with a reused earlier admission never triggers an automatic retry`, async () => {
    const f = fake();
    await reconcile(f);
    f.state.runs = [{...dispatchedRun(f, conclusion), run_attempt: 2}];
    f.state.jobs = [
      {name: 'Determine scenario requirement', conclusion: 'success', run_attempt: 1},
      {name: 'Verify scenario results and coverage', conclusion: 'skipped', run_attempt: 2},
    ];
    await reconcile(f);
    await reconcile(f);
    assert.equal(f.state.dispatches.length, 1);
    assert.equal(gate(f).state, 'failure');
  });
}

test('deleted reviewers cannot approve or crash readiness evaluation', () => {
  const {pr, approval} = fixture();
  assert.equal(reviewReady(pr, [{...approval, user: null}], null, []), false);
  assert.equal(reviewReady(pr, [approval, {...approval, user: null, state: 'CHANGES_REQUESTED'}], null, []), false);
  assert.equal(reviewReady(pr, [approval, {...approval, user: null}], null, []), true);
});

test('a failed PR does not abort later PRs in a scheduled sweep', async () => {
  const f = fake();
  f.context.payload.inputs = {};
  f.state.pulls = [{number: 12}, {number: 13}];
  f.github.rest.pulls.get = async ({pull_number}) => {
    if (pull_number === 12) throw new Error('PR temporarily unavailable');
    return {data: {...structuredClone(f.state.pr), number: pull_number}};
  };
  await reconcile(f);
  assert.equal(f.state.warnings.length, 1);
  assert.match(f.state.warnings[0], /PR #12/);
  assert.equal(f.state.outputs.pr_number, '13');
});

test('scheduled sweeps rotate bounded batches without starving a stable open PR set', () => {
  const pulls = Array.from({length: 20}, (_, index) => ({number: index + 1}));
  const visited = [];
  for (let slot = 0; slot < 4; slot++) {
    const batch = scheduledBatch(pulls, slot * 300000);
    assert.equal(batch.length, 5);
    visited.push(...batch.map(pr => pr.number));
  }
  assert.deepEqual(visited, pulls.map(pr => pr.number));
});

test('stored run IDs avoid history and repeated terminal job/status listings', async () => {
  const f = fake();
  await reconcile(f);
  assert.equal(f.state.dispatches[0].headers['X-GitHub-Api-Version'], '2026-03-10');
  const evidence = f.state.statuses.find(status => status.context === requirement(f.state.pr).statusContext);
  assert.equal(evidence.target_url, 'https://github.com/Kong/kongctl/actions/runs/200');
  f.state.runs = [dispatchedRun(f, 'success')];
  f.state.jobs = [{name: 'Verify scenario results and coverage', conclusion: 'success'}];
  await reconcile(f);
  const calls = [];
  const paginate = f.github.paginate;
  f.github.paginate = async (method, args) => { calls.push(method.method); return paginate(method, args); };
  await reconcile(f);
  assert.equal(gate(f).state, 'success');
  assert.equal(calls.filter(name => name === 'statuses').length, 1);
  assert.ok(!calls.includes('jobs'));
  assert.ok(!calls.includes('runs'));
});

function workflowScript(file, name) {
  const {readFileSync} = require('node:fs');
  const {resolve} = require('node:path');
  const source = readFileSync(resolve(__dirname, '../../.github/workflows', file), 'utf8');
  return source.split(`      - name: ${name}\n`)[1].split('          script: |\n')[1]
    .split(/\n(?:      -|  [a-z])/)[0].split('\n').map(line => line.replace(/^ {12}/, '')).join('\n');
}

async function runCommentScript(file, name, env, pr, body = '') {
  Object.assign(process.env, env);
  const writes = [];
  const outputs = {};
  const summary = {addHeading() {return this;}, addList() {return this;}, async write() {}};
  const core = {setOutput(key, value) {outputs[key] = value;}, info() {}, warning() {},
    setFailed(message) {throw new Error(message);}, summary};
  const github = {paginate: async () => body ? [{id: 1, body, user: {login: 'github-actions[bot]'}}] : [],
    rest: {pulls: {get: async () => ({data: pr})}, issues: {
      listComments() {}, updateComment: async args => writes.push(args), createComment: async args => writes.push(args),
    }, repos: {createCommitStatus: async args => writes.push(args)}}};
  const AsyncFunction = Object.getPrototypeOf(async function() {}).constructor;
  await new AsyncFunction('github', 'context', 'core', 'require', workflowScript(file, name))(
    github, {repo: {owner: 'Kong', repo: 'kongctl'}, payload: {repository: {default_branch: 'main'}}}, core, require);
  return {writes, outputs};
}

test('fork events preserve equivalent results and environment changes replace instructions', async () => {
  const env = {PR_NUMBER: '12', PR_HEAD_SHA: 'reviewed', KONNECT_ENVIRONMENT: 'com', VALIDATION_STATE: 'success'};
  const body = '<!-- kongctl-trusted-e2e-status -->\n- Reviewed SHA: `reviewed`\n' +
    '- Konnect environment: `com`\nTrusted E2E passed';
  const pr = fixture().pr;
  const unchanged = await runCommentScript('e2e-required-status.yaml', 'Post trusted-run instructions', env, pr, body);
  assert.equal(unchanged.writes.length, 0);
  const legacy = await runCommentScript('e2e-required-status.yaml', 'Post trusted-run instructions',
    env, pr, '<!-- kongctl-trusted-e2e-status -->\n- Reviewed SHA: `reviewed`\nTrusted E2E passed');
  assert.equal(legacy.writes.length, 0);
  const changed = await runCommentScript('e2e-required-status.yaml', 'Post trusted-run instructions',
    {...env, KONNECT_ENVIRONMENT: 'tech', VALIDATION_STATE: ''}, pr, body);
  assert.equal(changed.writes.length, 1);
  assert.match(changed.writes[0].body, /konnect_environment=tech/);
});

test('a moved fork head sets superseded outputs and never posts a false failure result', async () => {
  const pr = {...fixture().pr, head: {sha: 'new'}, labels: []};
  const report = await runCommentScript('e2e.yaml', 'Publish “E2E Required” status',
    {TRUSTED_DISPATCH: 'true', TRUSTED_PR_NUMBER: '12', STATUS_SHA: 'old'}, pr);
  assert.equal(report.outputs.superseded, 'true');
  assert.equal(report.outputs.state, 'superseded');
  assert.equal(report.writes.length, 0);
  const comment = await runCommentScript('e2e.yaml', 'Update trusted-run PR comment',
    {TRUSTED_PR_NUMBER: '12', STATUS_SHA: 'old', VALIDATION_ENV: 'com', CONFIGURED_ENV: ''}, pr);
  assert.equal(comment.writes.length, 0);
});
