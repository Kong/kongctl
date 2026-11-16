const {test} = require('node:test');
const assert = require('node:assert/strict');
const {requirement, reviewReady: reviewPolicy, ciReady, reconcile, admit, REQUIRED, scheduledBatch, authorize,
  AUTHORIZATION_JOB, AUTHORIZATION_STEP, scenarioDispatch} = require('../e2e-readiness.cjs');

function reviewReady(pr, reviews, decision, threads, authorized = false) {
  return reviewPolicy(pr, reviews, decision, threads, authorized, new Set(['author', 'human']));
}

function fixture() {
  const pr = {number: 12, state: 'open', draft: false, changed_files: 1, labels: [],
    user: {login: 'author'}, head: {sha: 'a'.repeat(40), repo: {full_name: 'Kong/kongctl'}},
    base: {repo: {full_name: 'Kong/kongctl'}}};
  const approval = {state: 'APPROVED', commit_id: pr.head.sha, user: {login: 'human', type: 'User'},
    author_association: 'MEMBER'};
  const ci = ['CI Test', 'Checks'].map((name, id) => ({name, id,
    path: `.github/workflows/${name === 'Checks' ? 'checks' : 'test'}.yaml`, head_sha: pr.head.sha,
    pull_requests: [{number: pr.number}], event: 'pull_request', status: 'completed', conclusion: 'success'}));
  return {pr, approval, ci};
}

function fake() {
  const {pr, approval, ci} = fixture();
  const state = {deployed: true, maintainers: ['author', 'human'], pr, reviews: [approval], ci, runs: [], statuses: [], checks: [], dispatches: [],
    threads: [], decision: 'APPROVED', files: [{filename: 'main.go'}], jobs: [], outputs: {}};
  const method = name => Object.assign(() => {}, {method: name});
  const github = {rest: {
    pulls: {get: async () => ({data: structuredClone(state.pr)}), list: method('pulls'),
      listFiles: method('files'), listReviews: method('reviews')},
    repos: {getCollaboratorPermissionLevel: async args => {
      assert.equal(args.username, 'human');
      return {data: {permission: state.reviewPermission || 'write'}};
    }, get: async () => ({data: {default_branch: state.defaultBranch || 'main'}}), getContent: async args => {
      if (args.path === 'scripts/e2e-authorizers.json') {
        assert.equal(args.ref, `refs/heads/${state.defaultBranch || 'main'}`);
        if (state.policyMissing) throw Object.assign(new Error('Policy not found'), {status: 404});
        return {data: {type: 'file', encoding: 'base64', content:
          Buffer.from(state.policyContent ?? JSON.stringify({maintainers: state.maintainers})).toString('base64')}};
      }
      if (!state.deployed) throw Object.assign(new Error('Not found'), {status: 404});
      return {data: {type: 'file'}};
    }, listCommitStatusesForRef: method('statuses'), createCommitStatus: async status => {
      state.statuses.unshift(status);
    }},
    checks: {listForRef: method('checks'), create: async check => {
      const data = {...check, id: state.checks.length + 1, app: {slug: 'github-actions'}};
      state.checks.unshift(data);
      return {data};
    }, update: async check => {
      (state.checkUpdates ||= []).push(check);
      Object.assign(state.checks.find(old => old.id === check.check_run_id), check);
    }},
    actions: {getWorkflowRun: async ({run_id}) => {
      const run = state.runs.find(run => run.id === run_id);
      if (!run && context.eventName === 'workflow_dispatch' && context.payload.inputs?.retry === 'true' &&
          run_id === Number(process.env.GITHUB_RUN_ID)) {
        const actor = state.controllerActor || {id: 43, login: 'human', type: 'User'};
        return {data: {actor, triggering_actor: state.controllerTrigger || actor, event: 'workflow_dispatch'}};
      }
      if (!run) throw Object.assign(new Error('Not found'), {status: 404});
      return {data: structuredClone(run)};
    }, listWorkflowRunsForRepo: method('ci'), listWorkflowRuns: method('runs'),
      listJobsForWorkflowRun: method('jobs'), createWorkflowDispatch: async dispatch => {
        state.dispatches.push(dispatch);
        if (state.dispatchError) throw new Error('dispatch interrupted');
        return {data: {workflow_run_id: 200}};
      }},
  }, paginate: async (method, args) => {
    let data = state[method.method];
    if (method.method === 'statuses') data = data.filter(status => !status.sha || status.sha === args.ref);
    if (method.method === 'checks') data = data.filter(check => check.head_sha === args.ref && check.name === args.check_name);
    return structuredClone(data);
  },
  graphql: async () => ({repository: {pullRequest: {reviewDecision: state.decision,
    reviewThreads: {nodes: state.threads, pageInfo: {hasNextPage: false}}}}})};
  const context = {repo: {owner: 'Kong', repo: 'kongctl'}, eventName: 'schedule',
    payload: {inputs: {pr_number: '12'}}};
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


function manualAuthorization(f) {
  const actor = {id: 42, login: 'author', type: 'User'};
  const run = {id: 300, workflow_id: 7, event: 'workflow_dispatch', head_branch: f.state.defaultBranch || 'main', head_sha: 'c'.repeat(40),
    head_repository: {full_name: 'Kong/kongctl'},
    display_title: `authorize-e2e:12/${f.state.pr.head.sha}`, actor, triggering_actor: actor,
    run_attempt: 1, status: 'completed', conclusion: 'success',
    html_url: 'https://github.com/Kong/kongctl/actions/runs/300'};
  f.state.runs.push(run);
  f.state.controllerActor = run.actor;
  f.state.permission = 'write';
  f.state.comparison = 'BEHIND';
  const graphql = f.github.graphql;
  f.github.graphql = async (query, args) => {
    if (args.head) {
      assert.equal(args.base, `refs/heads/${f.state.defaultBranch || 'main'}`);
      assert.equal(args.head, run.event === 'pull_request_target' ? run.pull_requests[0].base.sha : run.head_sha);
      return {repository: {ref: {compare: {status: f.state.comparison}}}};
    }
    return graphql(query, args);
  };
  const paginate = f.github.paginate;
  f.github.paginate = async (method, args) => {
    if (method.method === 'jobs' && args.run_id === 300) return f.state.authorizationJobs;
    return paginate(method, args);
  };
  f.state.authorizationJobs = [{name: 'Authorize exact PR commit for E2E', conclusion: 'success',
    steps: [{name: 'Record authorization', conclusion: 'success'}]}];
  f.github.rest.actions.getWorkflow = async args => {
    assert.equal(args.workflow_id, 'e2e-authorize.yaml');
    return {data: {id: 7}};
  };
  f.github.rest.repos.getCollaboratorPermissionLevel = async args => {
    assert.equal(args.username, f.state.permissionSubject || run.actor.login);
    return {data: {permission: f.state.permission}};
  };
  f.state.reviews = [];
  f.state.decision = 'REVIEW_REQUIRED';
  f.state.statuses.unshift({context: 'E2E Authorization / 12', sha: f.state.pr.head.sha,
    state: 'success', target_url: run.html_url});
  return run;
}

for (const permission of ['write', 'maintain', 'admin']) {
  test(`a ${permission} human can authorize their own PR; reconciliation, queue admission and retry agree`, async () => {
    const f = fake();
    manualAuthorization(f);
    f.state.permission = permission;
    await reconcile(f);
    assert.equal(f.state.dispatches.length, 1);
    Object.assign(process.env, {READINESS_PR: '12', READINESS_SHA: f.state.pr.head.sha,
      READINESS_ENV: 'com', READINESS_MODE: 'pr'});
    await admit(f);
    assert.equal(f.state.outputs.accepted, 'true');
    f.state.runs.push(dispatchedRun(f, 'failure'));
    await reconcile(f);
    f.context.eventName = 'workflow_dispatch';
    f.context.payload.inputs.retry = 'true';
    await reconcile(f);
    assert.equal(f.state.dispatches.length, 2);
  });
}

const invalidAuthorizations = {
  read: (f) => {f.state.permission = 'read';},
  triage: (f) => {f.state.permission = 'triage';},
  none: (f) => {f.state.permission = 'none';},
  bot: (f, run) => {run.actor.type = 'Bot';},
  'different PR': (f, run) => {run.display_title = run.display_title.replace(':12/', ':13/');},
  'stale SHA': (f, run) => {run.display_title = `authorize-e2e:12/${'b'.repeat(40)}`;},
  'branch code': (f, run) => {run.head_branch = 'untrusted';},
  'fork workflow': (f, run) => {run.head_repository.full_name = 'fork/kongctl';},
  'wrong workflow': (f, run) => {run.workflow_id = 99;},
  'untrusted tag revision': f => {f.state.comparison = 'DIVERGED';},
  'skipped authorization job': f => {f.state.authorizationJobs[0].conclusion = 'skipped';},
  'skipped authorization step': f => {f.state.authorizationJobs[0].steps[0].conclusion = 'skipped';},
  'wrong event': (f, run) => {run.event = 'pull_request';},
  'failed authorization': (f, run) => {run.conclusion = 'failure';},
  'unfinished authorization': (f, run) => {run.status = 'in_progress';},
  rerun: (f, run) => {run.run_attempt = 2;},
  'different rerun actor': (f, run) => {run.triggering_actor = {id: 99};},
  'forged status URL': f => {f.state.statuses[0].target_url += '/jobs/1';},
};
for (const [name, mutate] of Object.entries(invalidAuthorizations)) {
  test(`manual authorization fails closed for ${name}`, async () => {
    const f = fake();
    const run = manualAuthorization(f);
    mutate(f, run);
    await reconcile(f);
    assert.equal(f.state.dispatches.length, 0);
    assert.equal(gate(f).state, 'pending');
    Object.assign(process.env, {READINESS_PR: '12', READINESS_SHA: f.state.pr.head.sha,
      READINESS_ENV: 'com', READINESS_MODE: 'pr'});
    await admit(f);
    assert.equal(f.state.outputs.accepted, 'false');
  });
}

for (const [name, mutate] of Object.entries({
  'active changes request': f => {f.state.reviews = [{state: 'CHANGES_REQUESTED', user: {login: 'reviewer'}}];},
  'blocking decision': f => {f.state.decision = 'CHANGES_REQUESTED';},
  'unresolved thread': f => {f.state.threads = [{isResolved: false}];},
  'failed CI': f => {f.state.ci[0].conclusion = 'failure';},
  draft: f => {f.state.pr.draft = true;},
  closed: f => {f.state.pr.state = 'closed';},
  fork: f => {f.state.pr.head.repo.full_name = 'fork/kongctl';},
})) {
  test(`manual authorization preserves the ${name} restriction`, async () => {
    const f = fake();
    manualAuthorization(f);
    mutate(f);
    await reconcile(f);
    assert.equal(f.state.dispatches.length, 0);
    Object.assign(process.env, {READINESS_PR: '12', READINESS_SHA: f.state.pr.head.sha,
      READINESS_ENV: 'com', READINESS_MODE: 'pr'});
    await admit(f);
    assert.equal(f.state.outputs.accepted, 'false');
  });
}

test('a new head does not inherit authorization, even if status evidence is copied', async () => {
  const f = fake();
  manualAuthorization(f);
  f.state.pr.head.sha = 'b'.repeat(40);
  f.state.statuses[0].sha = f.state.pr.head.sha;
  f.state.ci = f.state.ci.map(run => ({...run, head_sha: f.state.pr.head.sha}));
  await reconcile(f);
  assert.equal(f.state.dispatches.length, 0);
});

test('authorization completion targets the PR for immediate reconciliation', async () => {
  const f = fake();
  const run = manualAuthorization(f);
  f.context.eventName = 'workflow_run';
  f.context.payload.inputs = {};
  f.context.payload.workflow_run = {...run, pull_requests: []};
  await reconcile(f);
  assert.equal(f.state.dispatches.length, 1);
});

test('recording authorization validates the actor, trusted run and exact current head', async () => {
  for (const invalid of [null, ...['read', 'triage', 'none', 'bot', 'different PR', 'stale SHA', 'branch code',
    'fork workflow', 'wrong workflow', 'untrusted tag revision', 'wrong event', 'rerun',
    'different rerun actor'].map(name => invalidAuthorizations[name]),
    f => {f.state.pr.head.sha = 'new';}, f => {f.state.pr.draft = true;},
    () => {process.env.GITHUB_REF = 'refs/tags/main';}]) {
    const f = fake();
    const run = manualAuthorization(f);
    f.state.statuses = [];
    f.context.eventName = 'workflow_dispatch';
    f.context.payload.inputs = {pr_number: '12', head_sha: f.state.pr.head.sha};
    process.env.GITHUB_RUN_ID = '300';
    process.env.GITHUB_REF = 'refs/heads/main';
    if (invalid) {
      invalid(f, run);
      await assert.rejects(authorize(f));
      assert.equal(f.state.statuses.length, 0);
    } else {
      await authorize(f);
      const evidence = f.state.statuses[0];
      assert.equal(evidence.context, 'E2E Authorization / 12');
      assert.equal(evidence.sha, f.state.pr.head.sha);
      assert.equal(evidence.description, 'Authorized by author');
      assert.equal(evidence.target_url, run.html_url);
    }
  }
});


test('deleted authorization evidence cannot admit E2E', async () => {
  const f = fake();
  manualAuthorization(f);
  f.state.runs = [];
  await reconcile(f);
  assert.equal(f.state.dispatches.length, 0);
});

test('manual authorization workflow is pinned to trusted code and admission can read evidence', () => {
  const {readFileSync} = require('node:fs');
  const {resolve} = require('node:path');
  const workflow = readFileSync(resolve(__dirname, '../../.github/workflows/e2e-authorize.yaml'), 'utf8');
  assert.ok(workflow.includes("github.ref == format('refs/heads/{0}', github.event.repository.default_branch)"));
  assert.ok(workflow.includes('ref: ${{ github.workflow_sha }}'));
  const lines = workflow.split('\n').map(line => line.trim());
  assert.ok(lines.includes(`name: ${AUTHORIZATION_JOB}`));
  assert.ok(lines.includes(`- name: ${AUTHORIZATION_STEP}`));
  assert.ok(!workflow.includes('pull_request_target:'));
  assert.ok(!workflow.includes('secrets.'));
  const scenario = readFileSync(resolve(__dirname, '../../.github/workflows/e2e.yaml'), 'utf8');
  assert.ok(scenario.split('  e2e-needed:')[1].split('    outputs:')[0].includes('statuses: read'));
});


test('make authorize-e2e discovers the current PR and default branch and fails before dispatch on lookup errors', () => {
  const {mkdtempSync, writeFileSync, readFileSync, existsSync, rmSync} = require('node:fs');
  const {tmpdir} = require('node:os');
  const {join, resolve} = require('node:path');
  const {spawnSync} = require('node:child_process');
  const dir = mkdtempSync(join(tmpdir(), 'kongctl-authorize-'));
  const log = join(dir, 'dispatch.json');
  try {
    writeFileSync(join(dir, 'gh'), `#!/usr/bin/env node
const fs = require('node:fs');
const args = process.argv.slice(2);
if (args[0] === 'pr') {
  if (args[1] !== 'view' || args[2] !== '--json' || args.includes('--repo')) process.exit(2);
  if (process.env.FAIL_LOOKUP === 'true') process.exit(1);
  process.stdout.write((process.env.WRONG_REPO === 'true' ? 'https://github.com/other/repo/pull/2792' :
    'https://github.com/Kong/kongctl/pull/2792') + ' ${'a'.repeat(40)}');
} else if (args[0] === 'repo') {
  process.stdout.write('default-branch');
} else if (args[0] === 'workflow') {
  fs.writeFileSync(process.env.DISPATCH_LOG, JSON.stringify(args));
} else process.exit(2);
`, {mode: 0o755});
    const env = {...process.env, PATH: `${dir}:${process.env.PATH}`, DISPATCH_LOG: log};
    const invoke = env => spawnSync('make', ['authorize-e2e'],
      {cwd: resolve(__dirname, '../..'), env, encoding: 'utf8'});
    const result = invoke(env);
    assert.equal(result.status, 0, result.stderr);
    assert.deepEqual(JSON.parse(readFileSync(log, 'utf8')), ['workflow', 'run', 'e2e-authorize.yaml',
      '--repo', 'Kong/kongctl', '--ref', 'default-branch', '-f', 'pr_number=2792', '-f', `head_sha=${'a'.repeat(40)}`]);
    rmSync(log);
    const failed = invoke({...env, FAIL_LOOKUP: 'true'});
    assert.notEqual(failed.status, 0);
    assert.equal(existsSync(log), false);
    assert.notEqual(invoke({...env, WRONG_REPO: 'true'}).status, 0);
    assert.equal(existsSync(log), false);
  } finally {
    rmSync(dir, {recursive: true, force: true});
  }
});


test('scheduled reconciliation without repository payload preserves a manually authorized success', async () => {
  const f = fake();
  manualAuthorization(f);
  await reconcile(f);
  f.state.runs.push(dispatchedRun(f, 'success'));
  f.state.jobs = [{name: 'Verify scenario results and coverage', conclusion: 'success'}];
  await reconcile(f);
  assert.equal(gate(f).state, 'success');
  f.context.payload = {};
  f.state.pulls = [{number: 12}];
  await reconcile(f);
  assert.equal(f.state.warnings?.length || 0, 0);
  assert.equal(gate(f).state, 'success');
});

test('reservation recheck observes revoked authorization statuses', async () => {
  const f = fake();
  manualAuthorization(f);
  const get = f.github.rest.pulls.get;
  let calls = 0;
  f.github.rest.pulls.get = async args => {
    if (++calls === 2) f.state.statuses[0] = {...f.state.statuses[0], state: 'failure'};
    return get(args);
  };
  await reconcile(f);
  assert.equal(f.state.dispatches.length, 0);
});

test('fork PRs cannot record misleading manual authorization', async () => {
  const f = fake();
  manualAuthorization(f);
  f.state.pr.head.repo.full_name = 'fork/kongctl';
  f.context.eventName = 'workflow_dispatch';
  f.context.payload.inputs = {pr_number: '12', head_sha: f.state.pr.head.sha};
  Object.assign(process.env, {GITHUB_RUN_ID: '300', GITHUB_REF: 'refs/heads/main'});
  f.state.statuses = [];
  await assert.rejects(authorize(f), /fork|same.repository/i);
  assert.equal(f.state.statuses.length, 0);
});

for (const blocker of ['approved', 'failed CI', 'unresolved thread', 'fork']) {
  test(`authorization APIs are unnecessary with ${blocker}`, async () => {
    const f = fake();
    manualAuthorization(f);
    if (blocker === 'approved') {
      f.state.reviews = [fixture().approval];
      f.state.decision = 'APPROVED';
      f.state.permissionSubject = 'human';
    }
    if (blocker === 'failed CI') f.state.ci[0].conclusion = 'failure';
    if (blocker === 'unresolved thread') f.state.threads = [{isResolved: false}];
    if (blocker === 'fork') f.state.pr.head.repo.full_name = 'fork/kongctl';
    const get = f.github.rest.actions.getWorkflowRun;
    f.github.rest.actions.getWorkflowRun = async args => {
      assert.notEqual(args.run_id, 300, 'unnecessary authorization validation');
      return get(args);
    };
    await reconcile(f);
  });
}


test('repository metadata overrides stale event metadata and is fetched once across snapshots', async () => {
  const f = fake();
  f.state.defaultBranch = 'trunk';
  manualAuthorization(f);
  f.context.payload.repository = {default_branch: 'stale'};
  const get = f.github.rest.repos.get;
  let calls = 0;
  f.github.rest.repos.get = async args => {calls++; return get(args);};
  await reconcile(f);
  assert.equal(f.state.dispatches.length, 1);
  assert.equal(f.state.dispatches[0].ref, 'trunk');
  assert.equal(calls, 1);
});

for (const status of ['BEHIND', 'IDENTICAL', 'AHEAD', 'DIVERGED', null]) {
  test(`authorization workflow ancestry comparison ${status} is interpreted against the default branch`, async () => {
    const f = fake();
    manualAuthorization(f);
    f.state.comparison = status;
    await reconcile(f);
    assert.equal(f.state.dispatches.length, ['BEHIND', 'IDENTICAL'].includes(status) ? 1 : 0);
  });
}


for (const status of [403, 429]) {
  for (const stage of ['run', 'permission']) {
    test(`authorization ${stage} API error ${status} is reported without dispatch or changing the pending gate`, async () => {
      const f = fake();
      manualAuthorization(f);
      f.context.payload = {};
      f.state.pulls = [{number: 12}];
      f.state.statuses.unshift({context: 'E2E Required', state: 'pending', description: 'Awaiting readiness.'});
      const count = f.state.statuses.length;
      const fail = async () => {throw Object.assign(new Error('API rate limit exceeded'), {status});};
      if (stage === 'run') f.github.rest.actions.getWorkflowRun = fail;
      else f.github.rest.repos.getCollaboratorPermissionLevel = fail;
      await reconcile(f);
      assert.equal(f.state.dispatches.length, 0);
      assert.equal(f.state.statuses.length, count);
      assert.equal(gate(f).state, 'pending');
      assert.equal(f.state.warnings.length, 1);
      assert.match(f.state.warnings[0], /API rate limit exceeded/);
      Object.assign(process.env, {READINESS_PR: '12', READINESS_SHA: f.state.pr.head.sha,
        READINESS_ENV: 'com', READINESS_MODE: 'pr'});
      await assert.rejects(admit(f), /API rate limit exceeded/);
    });
  }
}

test('authorization permission lookup checks the authorizing human rather than the PR author', async () => {
  const f = fake();
  const run = manualAuthorization(f);
  run.actor.login = 'gatekeeper';
  f.state.maintainers = ['gatekeeper'];
  assert.notEqual(run.actor.login, f.state.pr.user.login);
  await reconcile(f);
  assert.equal(f.state.dispatches.length, 1);
});


function manualRequest(f) {
  const run = manualAuthorization(f);
  f.context.eventName = 'workflow_dispatch';
  f.context.payload.inputs = {pr_number: '12', head_sha: f.state.pr.head.sha};
  Object.assign(process.env, {GITHUB_RUN_ID: '300', GITHUB_REF: 'refs/heads/main', GITHUB_RUN_ATTEMPT: '1'});
  return run;
}

function reviewRequest(f, id = 700) {
  f.state.reviews = [{id, node_id: `review-${id}`, state: 'COMMENTED', body: '/e2e', commit_id: f.state.pr.head.sha,
    user: {id: 42, login: 'author', type: 'User'},
    html_url: `https://github.com/Kong/kongctl/pull/12#pullrequestreview-${id}`}];
  f.state.decision = 'REVIEW_REQUIRED';
  f.state.controllerActor = f.state.reviews[0].user;
  const graphql = f.github.graphql;
  f.github.graphql = async (query, args) => {
    if (args.reviewID) {
      const review = f.state.reviews.find(review => review.node_id === args.reviewID);
      return {node: {fullDatabaseId: String(review.id), body: review.body, state: review.state,
        submittedAt: '2026-11-16T20:00:00Z', lastEditedAt: null, editor: null,
        author: {login: review.user.login}, commit: {oid: review.commit_id},
        pullRequest: {number: 12, repository: {nameWithOwner: 'Kong/kongctl'}}, ...f.state.reviewProof}};
    }
    return graphql(query, args);
  };
  f.github.rest.repos.getCollaboratorPermissionLevel = async args => {
    assert.equal(args.username, 'author');
    return {data: {permission: f.state.requestPermission || 'write'}};
  };
}

test('ordinary approval is restricted to allowlisted maintainers with current write access', async () => {
  for (const policy of ['not listed', 'no write', 'case insensitive']) {
    const f = fake();
    f.state.maintainers = policy === 'case insensitive' ? ['HUMAN'] : ['author'];
    if (policy === 'no write') {
      f.state.maintainers = ['human'];
      f.state.reviewPermission = 'read';
    }
    await reconcile(f);
    assert.equal(f.state.dispatches.length, policy === 'case insensitive' ? 1 : 0);
  }
});

for (const source of ['', 'readiness', 'main-debounce']) {
  test(`human scenario dispatch ${source || 'manual'} requires the allowlist, including fork execution`, async () => {
    for (const listed of [true, false]) {
      const f = fake();
      manualAuthorization(f);
      f.context.eventName = 'workflow_dispatch';
      f.context.payload.inputs = {dispatch_source: source, trusted_pr_number: '12', trusted_head_sha: f.state.pr.head.sha};
      f.state.pr.head.repo.full_name = 'outside/kongctl';
      Object.assign(process.env, {GITHUB_RUN_ID: '300', GITHUB_RUN_ATTEMPT: '2'});
      f.state.maintainers = listed ? ['author'] : ['human'];
      if (listed) {
        await scenarioDispatch(f);
        assert.equal(f.state.outputs.attempt, '2');
      } else await assert.rejects(scenarioDispatch(f), /allowlisted/);
    }
  });
}

for (const [name, mutate] of Object.entries({
  'new head': f => {f.state.pr.head.sha = 'b'.repeat(40);},
  closed: f => {f.state.pr.state = 'closed';},
  draft: f => {f.state.pr.draft = true;},
  'same repository': f => {f.state.pr.head.repo.full_name = 'Kong/kongctl';},
})) {
  test(`trusted fork retries reject ${name} before executing code`, async () => {
    const f = fake();
    manualAuthorization(f);
    f.context.eventName = 'workflow_dispatch';
    f.context.payload.inputs = {trusted_pr_number: '12', trusted_head_sha: f.state.pr.head.sha};
    f.state.pr.head.repo.full_name = 'outside/kongctl';
    Object.assign(process.env, {GITHUB_RUN_ID: '300', GITHUB_RUN_ATTEMPT: '2'});
    mutate(f);
    await assert.rejects(scenarioDispatch(f), /stale or ineligible/);
    assert.equal(f.state.outputs.attempt, undefined);
  });
}

for (const [name, mutate] of Object.entries({
  'trusted readiness': () => {},
  'trusted main debounce': f => {f.context.payload.inputs.dispatch_source = 'main-debounce';},
  'fork input': f => {f.context.payload.inputs.trusted_pr_number = '13';},
  'fork SHA without PR': f => {f.context.payload.inputs.trusted_head_sha = 'a'.repeat(40);},
  'untrusted branch': (f, run) => {run.head_branch = 'feature';},
  'untrusted revision': f => {f.state.comparison = 'DIVERGED';},
  'arbitrary source': f => {f.context.payload.inputs.dispatch_source = '';},
  'other bot': (f, run) => {run.actor.login = 'other[bot]';},
  'unauthorized rerun': (f, run) => {run.triggering_actor = {type: 'User', login: 'outside'};},
})) {
  test(`internal scenario dispatch validates ${name}`, async () => {
    const f = fake();
    const run = manualAuthorization(f);
    run.actor = {type: 'Bot', login: 'github-actions[bot]'};
    run.triggering_actor = run.actor;
    f.context.eventName = 'workflow_dispatch';
    f.context.payload.inputs = {dispatch_source: 'readiness'};
    Object.assign(process.env, {GITHUB_RUN_ID: '300', GITHUB_REF: 'refs/heads/main', GITHUB_RUN_ATTEMPT: '1'});
    mutate(f, run);
    if (name.startsWith('trusted ')) await scenarioDispatch(f);
    else await assert.rejects(scenarioDispatch(f), /allowlisted/);
  });
}

test('a non-maintainer cannot rerun a maintainer scenario dispatch', async () => {
  const f = fake();
  const run = manualAuthorization(f);
  run.triggering_actor = {type: 'User', login: 'outside'};
  f.context.eventName = 'workflow_dispatch';
  Object.assign(process.env, {GITHUB_RUN_ID: '300'});
  await assert.rejects(scenarioDispatch(f), /allowlisted/);
});

for (const actor of [{id: 77, login: 'outside', type: 'User'},
  {id: 78, login: 'github-actions[bot]', type: 'Bot'}]) {
  for (const role of ['controllerActor', 'controllerTrigger']) {
    test(`explicit retries reject unauthorized ${role} ${actor.login}`, async () => {
      const f = fake();
      await reconcile(f);
      f.state.runs = [dispatchedRun(f, 'failure')];
      f.context.eventName = 'workflow_dispatch';
      f.context.payload.inputs.retry = 'true';
      f.state[role] = actor;
      await assert.rejects(reconcile(f), /allowlisted/);
      assert.equal(f.state.dispatches.length, 1);
    });
  }
}

test('workflow contracts retain manual-only proof and admission guards', () => {
  const {readFileSync} = require('node:fs');
  const {resolve} = require('node:path');
  const auth = readFileSync(resolve(__dirname, '../../.github/workflows/e2e-authorize.yaml'), 'utf8');
  assert.ok(!auth.includes('pull_request_target:'));
  assert.ok(!auth.includes('issues: write'));
  const scenario = readFileSync(resolve(__dirname, '../../.github/workflows/e2e.yaml'), 'utf8');
  for (const name of ['e2e-harness', 'e2e-build', 'e2e', 'e2e-replay', 'e2e-replay-verify', 'e2e-verify']) {
    const job = scenario.split(`  ${name}:\n`)[1].split('    runs-on:')[0];
    assert.ok(job.includes('needs.e2e-needed.outputs.admission_attempt == github.run_attempt'), name);
  }
  for (const name of ['e2e-harness', 'e2e-build', 'e2e', 'e2e-verify']) {
    const job = scenario.split(`  ${name}:\n`)[1].split(/\n  \S/)[0];
    const guard = job.indexOf('Recheck rerun maintainer');
    const checkout = job.indexOf('- name: Checkout\n');
    assert.ok(guard >= 0 && checkout > guard, name);
    assert.match(job, /ref: \$\{\{ github.workflow_sha \}\}/);
  }
});

test('failed-jobs-only readiness retries fail explicitly instead of silently skipping validation', async () => {
  const {readFileSync} = require('node:fs');
  const {resolve} = require('node:path');
  const workflow = readFileSync(resolve(__dirname, '../../.github/workflows/e2e.yaml'), 'utf8');
  const script = workflow.split('- name: Require a fresh readiness decision')[1]
    .split('script: |\n')[1].split('\n\n')[0];
  const execute = new Function('process', 'core', script);
  for (const [admission, current, failed] of [['1', '1', false], ['1', '2', true], ['', '2', true]]) {
    const errors = [];
    execute({env: {ADMISSION_ATTEMPT: admission, GITHUB_RUN_ATTEMPT: current}},
      {setFailed: message => errors.push(message)});
    assert.equal(errors.length > 0, failed);
    if (failed) assert.match(errors[0], /new \/e2e Comment review.*Re-run all jobs/);
  }
});

test('PR policy edits and Actions variables cannot add an authorizer to the trusted roster', async () => {
  const f = fake();
  manualRequest(f);
  f.state.statuses = [];
  f.state.maintainers = ['human'];
  f.context.payload.inputs.maintainers = ['author'];
  const old = process.env.KONGCTL_E2E_AUTHORIZERS;
  process.env.KONGCTL_E2E_AUTHORIZERS = 'author';
  try {
    await assert.rejects(authorize(f), /allowlisted/);
    assert.equal(f.state.statuses.length, 0);
  } finally {
    if (old === undefined) delete process.env.KONGCTL_E2E_AUTHORIZERS;
    else process.env.KONGCTL_E2E_AUTHORIZERS = old;
  }
});

test('reservation recheck reloads a revoked default-branch maintainer policy', async () => {
  const f = fake();
  manualRequest(f);
  const get = f.github.rest.pulls.get;
  let calls = 0;
  f.github.rest.pulls.get = async args => {
    if (++calls === 2) f.state.maintainers = ['human'];
    return get(args);
  };
  await reconcile(f);
  assert.equal(f.state.dispatches.length, 0);
});

for (const policy of ['{}', 'null', '{"maintainers":null}', '{"maintainers":[42]}',
  '{"maintainers":["*"]}', 'not JSON', '{"maintainers":[]}']) {
  test(`maintainer policy ${policy} never authorizes execution`, async () => {
    const f = fake();
    manualRequest(f);
    f.state.statuses = [];
    f.state.policyContent = policy;
    await assert.rejects(authorize(f));
    assert.equal(f.state.statuses.length, 0);
  });
}

test('review policy without an explicit maintainer roster cannot authorize E2E', () => {
  const {pr, approval} = fixture();
  assert.equal(reviewPolicy(pr, [approval], 'APPROVED', []), false);
});


test('a forged target run associated with two PRs cannot establish authorization provenance', async () => {
  const f = fake();
  const run = manualAuthorization(f);
  run.event = 'pull_request_target';
  run.pull_requests = [{number: 12, head: {sha: f.state.pr.head.sha},
    base: {ref: 'main', sha: 'c'.repeat(40)}},
  {number: 13, head: {sha: f.state.pr.head.sha}, base: {ref: 'forged-workflow', sha: 'd'.repeat(40)}}];
  await reconcile(f);
  assert.equal(f.state.dispatches.length, 0);
});

test('scenario permission lookup runs once for an identical original and rerun actor', async () => {
  const f = fake();
  manualAuthorization(f);
  f.context.eventName = 'workflow_dispatch';
  Object.assign(process.env, {GITHUB_RUN_ID: '300', GITHUB_RUN_ATTEMPT: '1'});
  const get = f.github.rest.repos.getCollaboratorPermissionLevel;
  let calls = 0;
  f.github.rest.repos.getCollaboratorPermissionLevel = async args => {calls++; return get(args);};
  await scenarioDispatch(f);
  assert.equal(calls, 1);
});


test('maintainer /e2e reviews authorize their own head, retry failures, and request fresh validation after success', async () => {
  const f = fake();
  reviewRequest(f, 700);
  await reconcile(f);
  await reconcile(f);
  assert.equal(f.state.dispatches.length, 1);
  assert.match(f.state.statuses[0].target_url, /actions\/runs\/200$/);
  f.state.runs = [dispatchedRun(f, 'failure')];
  await reconcile(f);
  assert.equal(gate(f).state, 'failure');
  const failed = f.state.checks[0];
  assert.equal(failed.head_sha, f.state.pr.head.sha);
  assert.equal(failed.conclusion, 'failure');
  assert.equal(failed.details_url, f.state.runs[0].html_url);
  assert.match(failed.output.summary, /Start or retry/);
  assert.match(failed.output.summary, /\/e2e/);
  reviewRequest(f, 701);
  process.env.GITHUB_RUN_ID = '101';
  await reconcile(f);
  assert.equal(f.state.dispatches.length, 2);
  f.state.runs = [dispatchedRun(f, 'success')];
  f.state.jobs = [{name: 'Verify scenario results and coverage', conclusion: 'success'}];
  await reconcile(f);
  assert.equal(gate(f).state, 'success');
  reviewRequest(f, 702);
  process.env.GITHUB_RUN_ID = '102';
  await reconcile(f);
  assert.equal(f.state.dispatches.length, 3);
  assert.match(f.state.statuses.find(status => status.context === requirement(f.state.pr).statusContext).description,
    /request=review-702/);
});

test('repeating /e2e while E2E is active acknowledges the command without queuing duplicate work', async () => {
  const f = fake();
  reviewRequest(f, 700);
  await reconcile(f);
  f.state.runs = [dispatchedRun(f)];
  reviewRequest(f, 701);
  await reconcile(f);
  f.state.runs[0].status = 'completed';
  f.state.runs[0].conclusion = 'success';
  f.state.jobs = [{name: 'Verify scenario results and coverage', conclusion: 'success'}];
  await reconcile(f);
  assert.equal(f.state.dispatches.length, 1);
  assert.equal(gate(f).state, 'success');
});

for (const [name, mutate] of Object.entries({
  'non-maintainer': f => {f.state.reviews[0].user.login = 'outside';},
  bot: f => {f.state.reviews[0].user.type = 'Bot';},
  'no current write access': f => {f.state.requestPermission = 'triage';},
  'stale command': f => {f.state.reviews[0].commit_id = 'b'.repeat(40);},
  'quoted or embedded command': f => {f.state.reviews[0].body = 'please /e2e';},
  'dismissed review': f => {f.state.reviews[0].state = 'DISMISSED';},
  draft: f => {f.state.pr.draft = true;},
  fork: f => {f.state.pr.head.repo.full_name = 'outside/kongctl';},
  'failed CI': f => {f.state.ci[0].conclusion = 'failure';},
  'unresolved thread': f => {f.state.threads = [{isResolved: false}];},
  'active changes request': f => {f.state.reviews.push({state: 'CHANGES_REQUESTED', user: {login: 'outside'}});},
  'another editor': f => {f.state.reviewProof = {lastEditedAt: 'now', editor: {login: 'outside'}};},
  'unknown editor': f => {f.state.reviewProof = {lastEditedAt: 'now', editor: null};},
  'different PR proof': f => {f.state.reviewProof = {pullRequest: {number: 13, repository: {nameWithOwner: 'Kong/kongctl'}}};},
  'different repository proof': f => {f.state.reviewProof = {pullRequest: {number: 12, repository: {nameWithOwner: 'outside/kongctl'}}};},
  'different commit proof': f => {f.state.reviewProof = {commit: {oid: 'b'.repeat(40)}};},
  'different author proof': f => {f.state.reviewProof = {author: {login: 'outside'}};},
  'changed body proof': f => {f.state.reviewProof = {body: 'removed'};},
  'different ID proof': f => {f.state.reviewProof = {fullDatabaseId: '999'};},
  'pending proof': f => {f.state.reviewProof = {submittedAt: null};},
})) {
  test(`/e2e cannot authorize execution with ${name}`, async () => {
    const f = fake();
    reviewRequest(f);
    mutate(f);
    await reconcile(f);
    assert.equal(f.state.dispatches.length, 0);
    Object.assign(process.env, {READINESS_PR: '12', READINESS_SHA: f.state.pr.head.sha,
      READINESS_ENV: 'com', READINESS_MODE: 'pr'});
    await admit(f);
    assert.equal(f.state.outputs.accepted, 'false');
  });
}

test('a command does not authorize a new push and a new current-head command does', async () => {
  const f = fake();
  reviewRequest(f, 700);
  await reconcile(f);
  f.state.pr.head.sha = 'b'.repeat(40);
  f.state.ci = f.state.ci.map(run => ({...run, head_sha: f.state.pr.head.sha}));
  await reconcile(f);
  assert.equal(f.state.dispatches.length, 1);
  assert.match(gate(f).description, /Comment review/);
  reviewRequest(f, 701);
  process.env.GITHUB_RUN_ID = '101';
  await reconcile(f);
  assert.equal(f.state.dispatches.length, 2);
  assert.equal(f.state.dispatches[1].inputs.readiness_sha, f.state.pr.head.sha);
});

test('native PR checks explain waiting state and link failed executions without overwriting the protected gate', async () => {
  const f = fake();
  f.state.reviews = [];
  f.state.decision = 'REVIEW_REQUIRED';
  await reconcile(f);
  assert.equal(f.state.checks.length, 1);
  assert.equal(f.state.checks[0].head_sha, f.state.pr.head.sha);
  assert.equal(f.state.checks[0].status, 'queued');
  assert.match(f.state.checks[0].output.summary, /Comment/);
  assert.match(f.state.checks[0].output.summary, new RegExp(f.state.pr.head.sha));
  await reconcile(f);
  assert.equal(f.state.checks.length, 1);
  assert.equal(f.state.checkUpdates?.length || 0, 0);
  reviewRequest(f);
  await reconcile(f);
  f.state.runs = [dispatchedRun(f, 'failure')];
  await reconcile(f);
  f.state.threads = [{isResolved: false}];
  await reconcile(f);
  assert.equal(gate(f).state, 'pending');
  assert.equal(f.state.checks[0].conclusion, 'failure');
  assert.match(f.state.checks[0].output.summary, /Unresolved review threads/);
  assert.match(f.state.checks[0].details_url, /actions\/runs\/200$/);
});

test('CI matches immutable workflow paths rather than dynamic run display names', async () => {
  const f = fake();
  f.state.ci[1].name = 'Checks, branch:12/merge, triggered by @author';
  await reconcile(f);
  assert.equal(f.state.dispatches.length, 1);
  const blocked = fake();
  blocked.state.ci[1].path = '.github/workflows/forged-checks.yaml';
  await reconcile(blocked);
  assert.equal(blocked.state.dispatches.length, 0);
  assert.equal(gate(blocked).description, 'Awaiting prerequisite CI: Checks.');
});
