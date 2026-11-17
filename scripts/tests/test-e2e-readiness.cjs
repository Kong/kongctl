const {test} = require('node:test');
const assert = require('node:assert/strict');
const {requirement, ciReady, reconcile, admit, REQUIRED, scheduledBatch, authorize,
  AUTHORIZATION_JOB, AUTHORIZATION_STEP, scenarioDispatch, waitForCompletion, recheckRerun} = require('../e2e-readiness.cjs');

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
  const state = {deployed: true, pr, reviews: [approval], ci, runs: [], statuses: [], checks: [], dispatches: [],
    threads: [], decision: 'APPROVED', files: [{filename: 'main.go'}], jobs: [], outputs: {}};
  const method = name => Object.assign(() => {}, {method: name});
  const github = {rest: {
    pulls: {get: async () => ({data: structuredClone(state.pr)}), list: method('pulls'),
      listFiles: method('files'), listReviews: method('reviews')},
    repos: {getCollaboratorPermissionLevel: async args => {
      return {data: {permission: 'write', role_name: args.username === 'human' ? (state.reviewRole ?? 'maintain') : 'write'}};
    }, get: async () => ({data: {default_branch: state.defaultBranch || 'main'}}), getContent: async args => {
      if (!state.deployed) throw Object.assign(new Error('Not found'), {status: 404});
      return {data: {type: 'file'}};
    }, listCommitStatusesForRef: method('statuses'), createCommitStatus: async status => {
      state.statuses.unshift({...status, created_at: new Date().toISOString()});
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
    if (method.method === 'checks') data = data.filter(check => check.head_sha === args.ref && (!args.check_name || check.name === args.check_name));
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

test('deleted reviewers cannot approve or crash production readiness evaluation', async () => {
  for (const [reviews, allowed] of [
    [[{...fixture().approval, user: null}], false],
    [[fixture().approval, {...fixture().approval, user: null, state: 'CHANGES_REQUESTED'}], false],
    [[fixture().approval, {...fixture().approval, user: null}], true],
  ]) {
    const f = fake();
    f.state.reviews = reviews;
    await reconcile(f);
    assert.equal(f.state.dispatches.length, allowed ? 1 : 0);
  }
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
  f.state.role = 'maintain';
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
    return {data: {permission: 'write', role_name: args.username === (f.state.permissionSubject || run.actor.login) ? f.state.role : 'write'}};
  };
  f.state.reviews = [];
  f.state.decision = 'REVIEW_REQUIRED';
  f.state.statuses.unshift({context: 'E2E Authorization / 12', sha: f.state.pr.head.sha,
    state: 'success', target_url: run.html_url});
  return run;
}

for (const permission of ['maintain', 'admin']) {
  test(`a ${permission} human can authorize their own PR; reconciliation, queue admission and retry agree`, async () => {
    const f = fake();
    manualAuthorization(f);
    f.state.role = permission;
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
  write: (f) => {f.state.role = 'write';},
  'missing role': (f) => {f.state.role = undefined;},
  'custom role': (f) => {f.state.role = 'custom-maintainer';},
  read: (f) => {f.state.role = 'read';},
  triage: (f) => {f.state.role = 'triage';},
  none: (f) => {f.state.role = 'none';},
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
    return {data: {permission: 'write', role_name: args.username === 'author' ? (f.state.requestRole ?? 'maintain') : 'write'}};
  };
}

for (const role of ['maintain', 'admin', 'write', 'triage', 'read', 'custom-maintainer', undefined]) {
  test(`/e2e admission uses the current built-in role ${role}`, async () => {
    const f = fake();
    reviewRequest(f);
    f.github.rest.repos.getCollaboratorPermissionLevel = async () => ({data: {permission: 'admin', role_name: role}});
    const allowed = ['maintain', 'admin'].includes(role);
    await reconcile(f);
    assert.equal(f.state.dispatches.length, allowed ? 1 : 0);
    Object.assign(process.env, {READINESS_PR: '12', READINESS_SHA: f.state.pr.head.sha,
      READINESS_ENV: 'com', READINESS_MODE: 'pr'});
    await admit(f);
    assert.equal(f.state.outputs.accepted, String(allowed));
    if (allowed) {
      f.github.rest.repos.getCollaboratorPermissionLevel = async () => ({data: {permission: 'write', role_name: 'write'}});
      await admit(f);
      assert.equal(f.state.outputs.accepted, 'false', 'demotion revokes queued authorization');
    }
  });
}

for (const role of ['maintain', 'admin', 'write', 'read', 'triage', 'none', undefined]) {
  test(`ordinary approval uses the built-in repository role ${role}`, async () => {
    const f = fake();
    f.state.reviewRole = role;
    f.github.rest.repos.getCollaboratorPermissionLevel = async () => ({data: {permission: 'write', role_name: role}});
    await reconcile(f);
    assert.equal(f.state.dispatches.length, ['maintain', 'admin'].includes(role) ? 1 : 0);
  });
}

for (const source of ['', 'readiness', 'main-debounce']) {
  for (const role of ['maintain', 'admin', 'write']) {
    test(`human scenario dispatch ${source || 'manual'} checks repository role ${role}, including forks`, async () => {
      const f = fake();
      manualAuthorization(f);
      f.state.role = role;
      f.context.eventName = 'workflow_dispatch';
      f.context.payload.inputs = {dispatch_source: source, trusted_pr_number: '12', trusted_head_sha: f.state.pr.head.sha};
      f.state.pr.head.repo.full_name = 'outside/kongctl';
      Object.assign(process.env, {GITHUB_RUN_ID: '300', GITHUB_RUN_ATTEMPT: '2'});
      if (['maintain', 'admin'].includes(role)) {
        await scenarioDispatch(f);
        assert.equal(f.state.outputs.attempt, '2');
      } else await assert.rejects(scenarioDispatch(f), /Maintain or Admin/);
    });
  }
}

for (const [name, mutate] of Object.entries({
  'new head': f => {f.state.pr.head.sha = 'b'.repeat(40);},
  closed: f => {f.state.pr.state = 'closed';},
  draft: f => {f.state.pr.draft = true;},
  'same repository': f => {f.state.pr.head.repo.full_name = 'Kong/kongctl';},
  'deleted fork repository': f => {f.state.pr.head.repo = null;},
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
    else await assert.rejects(scenarioDispatch(f), /Maintain or Admin/);
  });
}

test('a non-maintainer cannot rerun a maintainer scenario dispatch', async () => {
  const f = fake();
  const run = manualAuthorization(f);
  run.triggering_actor = {type: 'User', login: 'outside'};
  f.context.eventName = 'workflow_dispatch';
  Object.assign(process.env, {GITHUB_RUN_ID: '300'});
  await assert.rejects(scenarioDispatch(f), /Maintain or Admin/);
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
      await assert.rejects(reconcile(f), /Maintain or Admin/);
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
    const job = scenario.split(`  ${name}:\n`)[1].split(/\n  \S/)[0];
    const condition = job.split('    runs-on:')[0];
    assert.ok(!condition.includes('admission_attempt'), name);
    const guard = job.indexOf('Recheck rerun readiness before expensive work');
    const checkout = job.search(/- name: Checkout(?:\n| trusted replay tools)/);
    assert.ok(guard >= 0 && checkout > guard, name);
    assert.match(job, /ref: \$\{\{ github.workflow_sha \}\}/);
    const retryStep = job.split('- name: Recheck rerun readiness before expensive work')[1].split(/\n      - /)[0];
    assert.match(retryStep, /if: github.event_name == 'workflow_dispatch' && github.run_attempt > 1\n/);
    assert.match(retryStep, /\.recheckRerun\(\{github, context, core\}\)/);
    assert.ok(!retryStep.includes("dispatch_source != 'readiness'"), name);
    for (const [key, value] of Object.entries({READINESS_PR: 'inputs.readiness_pr', READINESS_SHA: 'inputs.readiness_sha',
      READINESS_ENV: 'inputs.konnect_environment', READINESS_MODE: 'inputs.readiness_mode'})) {
      assert.ok(retryStep.includes(`${key}: \${{ ${value} }}`), `${name}: ${key}`);
    }
    assert.match(retryStep, /KONNECT_ENVIRONMENT:.*vars.KONGCTL_E2E_KONNECT_ENV/);
    assert.match(job, /actions: read/);
    assert.match(job, /pull-requests: read/);
    assert.match(job.split('    steps:')[0], /statuses: read/, `${name}: manual authorization evidence requires status access`);
  }
  assert.ok(!scenario.includes('Require a fresh readiness decision'));
});

test('reservation recheck observes a revoked repository role', async () => {
  const f = fake();
  manualRequest(f);
  const get = f.github.rest.pulls.get;
  let calls = 0;
  f.github.rest.pulls.get = async args => {
    if (++calls === 2) f.state.role = 'write';
    return get(args);
  };
  await reconcile(f);
  assert.equal(f.state.dispatches.length, 0);
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

for (const conclusion of ['success', 'failure', 'cancelled']) {
  test(`a command during active E2E requests one follow-up after ${conclusion}`, async () => {
    const f = fake();
    reviewRequest(f, 700);
    await reconcile(f);
    f.state.runs = [dispatchedRun(f)];
    reviewRequest(f, 701);
    await reconcile(f);
    await reconcile(f);
    assert.equal(f.state.dispatches.length, 1);
    assert.match(f.state.statuses.find(s => s.context === requirement(f.state.pr).statusContext).description,
      /request=review-700/);
    f.state.runs[0].status = 'completed';
    f.state.runs[0].conclusion = conclusion;
    f.state.jobs = [{name: 'Verify scenario results and coverage', conclusion}];
    process.env.GITHUB_RUN_ID = '101';
    await reconcile(f);
    await reconcile(f);
    assert.equal(f.state.dispatches.length, 2);
    assert.match(f.state.statuses.find(s => s.context === requirement(f.state.pr).statusContext).description,
      /request=review-701/);
  });
}

for (const [name, mutate] of Object.entries({
  'non-maintainer': f => {f.state.reviews[0].user.login = 'outside';},
  bot: f => {f.state.reviews[0].user.type = 'Bot';},
  'write-only role': f => {f.state.requestRole = 'write';},
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

for (const [name, mutate] of Object.entries({
  bot: f => {f.state.reviews[0].user.type = 'Bot';},
  author: f => {f.state.pr.user.login = 'human';},
  stale: f => {f.state.reviews[0].commit_id = 'b'.repeat(40);},
  outsider: f => {f.state.reviews[0].author_association = 'CONTRIBUTOR';},
  dismissed: f => {f.state.reviews[0].state = 'DISMISSED';},
  'changes requested': f => {f.state.reviews.push({...fixture().approval, state: 'CHANGES_REQUESTED'});},
  'review required': f => {f.state.decision = 'REVIEW_REQUIRED';},
  'unresolved thread': f => {f.state.threads = [{isResolved: false}];},
  draft: f => {f.state.pr.draft = true;},
  closed: f => {f.state.pr.state = 'closed';},
})) {
  test(`production ordinary approval policy rejects ${name}`, async () => {
    const f = fake();
    mutate(f);
    await reconcile(f);
    assert.equal(f.state.dispatches.length, 0);
  });
}

for (const missing of ['Reserved', 'Running']) {
  test(`a new command recovers missing ${missing} work after the visibility grace period`, async () => {
    const f = fake();
    reviewRequest(f, 700);
    await reconcile(f);
    const reserved = f.state.statuses.find(s => s.context === requirement(f.state.pr).statusContext);
    reserved.description = `${missing} request=review-700 100`;
    if (missing === 'Reserved') reserved.target_url = 'https://github.com/Kong/kongctl/pull/12/checks';
    reviewRequest(f, 701);
    await reconcile(f);
    assert.equal(f.state.dispatches.length, 1, 'wait for transient GitHub visibility delays');
    reserved.created_at = new Date(Date.now() - 11 * 60000).toISOString();
    process.env.GITHUB_RUN_ID = '101';
    await reconcile(f);
    await reconcile(f);
    assert.equal(f.state.dispatches.length, 2);
    assert.match(f.state.checks[0].output.summary, /retry=true/);
  });
}

test('a new command recovers an interrupted dispatch without the Actions form', async () => {
  const f = fake();
  reviewRequest(f, 700);
  f.state.dispatchError = true;
  await assert.rejects(reconcile(f), /interrupted/);
  f.state.dispatchError = false;
  const reserved = f.state.statuses.find(s => s.context === requirement(f.state.pr).statusContext);
  reserved.created_at = new Date(Date.now() - 11 * 60000).toISOString();
  reviewRequest(f, 701);
  await reconcile(f);
  assert.equal(f.state.dispatches.length, 2);
});

test('recovery discovers an active dispatch whose response was lost instead of duplicating it', async () => {
  const f = fake();
  reviewRequest(f, 700);
  await reconcile(f);
  const reserved = f.state.statuses.find(s => s.context === requirement(f.state.pr).statusContext);
  reserved.description = 'Reserved request=review-700 100';
  reserved.target_url = 'https://github.com/Kong/kongctl/pull/12/checks';
  reserved.created_at = new Date(Date.now() - 11 * 60000).toISOString();
  f.state.runs = [dispatchedRun(f)];
  reviewRequest(f, 701);
  await reconcile(f);
  assert.equal(f.state.dispatches.length, 1);
  assert.match(gate(f).target_url, /actions\/runs\/200$/);
});

test('recovery fails closed if GitHub cannot list the potentially active dispatch', async () => {
  const f = fake();
  reviewRequest(f, 700);
  await reconcile(f);
  const reserved = f.state.statuses.find(s => s.context === requirement(f.state.pr).statusContext);
  reserved.created_at = new Date(Date.now() - 11 * 60000).toISOString();
  reviewRequest(f, 701);
  const paginate = f.github.paginate;
  f.github.paginate = async (method, args) => {
    if (method.method === 'runs') throw Object.assign(new Error('Recovery API unavailable'), {status: 403});
    return paginate(method, args);
  };
  await assert.rejects(reconcile(f), /Recovery API unavailable/);
  assert.equal(f.state.dispatches.length, 1);
});

for (const label of ['e2e:force-live', 'konnect-env:tech']) {
  test(`changing ${label} completes obsolete pending PR checks`, async () => {
    const f = fake();
    f.state.reviews = [];
    await reconcile(f);
    const old = f.state.checks[0];
    // Include checks left by the earlier implementation's per-mode name.
    old.name += ' / com / pr';
    f.state.pr.labels = [{name: label}];
    await reconcile(f);
    assert.equal(old.status, 'completed');
    assert.equal(old.conclusion, 'neutral');
    assert.match(old.output.title, /Superseded/);
    assert.equal(f.state.checks.filter(check => check.status !== 'completed').length, 1);
    f.state.pr.labels = [];
    await reconcile(f);
    assert.equal(f.state.checks.filter(check => check.status !== 'completed').length, 1);
    assert.equal(f.state.checks[0].name, 'E2E / PR #12');
  });
}

for (const status of [403, 429]) {
  test(`a Checks API ${status} cannot prevent publishing protected gate success or failure`, async () => {
    const f = fake();
    await reconcile(f);
    f.state.runs = [dispatchedRun(f, 'success')];
    f.state.jobs = [{name: 'Verify scenario results and coverage', conclusion: 'success'}];
    const paginate = f.github.paginate;
    f.github.paginate = async (method, args) => {
      if (method.method === 'checks') throw Object.assign(new Error('Checks unavailable'), {status});
      return paginate(method, args);
    };
    await reconcile(f);
    assert.equal(gate(f).state, 'success');
    f.state.runs[0].run_attempt = 2;
    f.state.runs[0].conclusion = 'failure';
    await reconcile(f);
    assert.equal(gate(f).state, 'failure');
    assert.match(f.state.warnings[0], /could not update E2E execution check/);
  });
}

test('docs-only reconciliation publishes the merge gate without any Checks API calls', async () => {
  const f = fake();
  f.state.files = [{filename: 'README.md'}];
  const paginate = f.github.paginate;
  f.github.paginate = async (method, args) => {
    assert.notEqual(method.method, 'checks');
    return paginate(method, args);
  };
  await reconcile(f);
  await reconcile(f);
  assert.equal(gate(f).state, 'success');
  assert.equal(f.state.checks.length, 0);
});

test('a deleted fork remains ineligible in authorization, reconciliation and queued admission', async () => {
  const f = fake();
  manualRequest(f);
  f.state.pr.head.repo = null;
  await assert.rejects(authorize(f), /Fork PRs require/);
  f.context.eventName = 'schedule';
  await reconcile(f);
  assert.equal(f.state.dispatches.length, 0);
  assert.equal(gate(f).state, 'pending');
  Object.assign(process.env, {READINESS_PR: '12', READINESS_SHA: f.state.pr.head.sha,
    READINESS_ENV: 'com', READINESS_MODE: 'live'});
  await admit(f);
  assert.equal(f.state.outputs.accepted, 'false');
});

function completionRequest(f) {
  const run = {id: 200, workflow_id: 8, event: 'workflow_dispatch',
    head_repository: {full_name: 'Kong/kongctl'}, status: 'in_progress',
    display_title: `readiness:12/${f.state.pr.head.sha}/com/pr/100`};
  f.state.runs = [run];
  f.context.eventName = 'workflow_dispatch';
  f.context.payload.inputs = {pr_number: '12', completion_run_id: '200'};
  f.github.rest.actions.getWorkflow = async args => {
    assert.equal(args.workflow_id, 'e2e.yaml');
    return {data: {id: 8}};
  };
  return run;
}

test('scenario completion dispatches a targeted non-retry callback after every other job', async () => {
  const {readFileSync} = require('node:fs');
  const {resolve} = require('node:path');
  const workflow = readFileSync(resolve(__dirname, '../../.github/workflows/e2e.yaml'), 'utf8');
  const job = workflow.split('  e2e-completion:\n')[1];
  const dependencies = job.split('    if:')[0];
  for (const [, name] of workflow.split('jobs:\n')[1].matchAll(/^  ([a-z][a-z0-9-]*):$/gm)) {
    if (name !== 'e2e-completion') assert.ok(dependencies.includes(`      - ${name}\n`), name);
  }
  assert.match(job, /always\(\) && github.event_name == 'workflow_dispatch'/);
  assert.match(job, /inputs.dispatch_source == 'readiness'/);
  assert.match(job, /inputs.trusted_pr_number != ''/);
  assert.match(job, /continue-on-error: true/);
  assert.match(job, /actions: write/);
  assert.ok(!job.includes('secrets.'));
  const script = job.split('          script: |\n')[1];
  const AsyncFunction = Object.getPrototypeOf(async () => {}).constructor;
  const requests = [];
  const github = {rest: {repos: {get: async () => ({data: {default_branch: 'trunk'}})},
    actions: {createWorkflowDispatch: async args => requests.push(args)}}};
  await new AsyncFunction('github', 'context', 'process', script)(github,
    {repo: {owner: 'Kong', repo: 'kongctl'}, runId: 200}, {env: {COMPLETION_PR: '12'}});
  assert.deepEqual(requests, [{owner: 'Kong', repo: 'kongctl', workflow_id: 'e2e-required-status.yaml', ref: 'trunk',
    inputs: {pr_number: '12', completion_run_id: '200', retry: 'false'}}]);
});

test('completion callback waits for suite teardown before publishing the successful result', async () => {
  const f = fake();
  await reconcile(f);
  const executed = dispatchedRun(f, 'success');
  const run = completionRequest(f);
  Object.assign(run, executed, {status: 'in_progress'});
  f.state.jobs = [{name: 'Verify scenario results and coverage', conclusion: 'success'}];
  let waits = 0;
  f.pause = async milliseconds => {
    assert.equal(milliseconds, 5000);
    assert.equal(gate(f).state, 'pending');
    waits++;
    run.status = 'completed';
  };
  await reconcile(f);
  assert.equal(waits, 1);
  assert.equal(gate(f).state, 'success');
  assert.equal(f.state.checks[0].conclusion, 'success');
  assert.equal(f.state.dispatches.length, 1, 'completion must not request another E2E run');
  await reconcile(f);
  assert.equal(f.state.dispatches.length, 1);
});

for (const conclusion of ['failure', 'cancelled']) {
  test(`completion callback publishes ${conclusion} without retrying E2E`, async () => {
    const f = fake();
    await reconcile(f);
    const executed = dispatchedRun(f, conclusion);
    const run = completionRequest(f);
    Object.assign(run, executed);
    await reconcile(f);
    assert.equal(gate(f).state, 'failure');
    assert.equal(f.state.checks[0].conclusion, 'failure');
    assert.equal(f.state.dispatches.length, 1);
  });
}

test('completion wait tolerates temporary run invisibility and rereads the current attempt', async () => {
  const f = fake();
  const run = completionRequest(f);
  const get = f.github.rest.actions.getWorkflowRun;
  let calls = 0;
  f.github.rest.actions.getWorkflowRun = async args => {
    if (++calls === 1) throw Object.assign(new Error('Not yet visible'), {status: 404});
    return get(args);
  };
  let waits = 0;
  await waitForCompletion(f, async () => {
    if (++waits === 2) Object.assign(run, {run_attempt: 2, status: 'completed', conclusion: 'success'});
  });
  assert.equal(calls, 3);
  assert.equal(waits, 2);
  assert.equal(f.state.statuses.length, 0);
});

test('completion wait times out without declaring an unfinished run successful', async () => {
  const f = fake();
  completionRequest(f);
  let waits = 0;
  f.pause = async () => {waits++;};
  await assert.rejects(reconcile(f), /still not complete/);
  assert.equal(waits, 23);
  assert.equal(f.state.statuses.length, 0);
  assert.equal(f.state.dispatches.length, 0);
});

for (const [name, mutate] of Object.entries({
  'another PR': (f, run) => {run.display_title = run.display_title.replace('readiness:12/', 'readiness:13/');},
  'another workflow': (f, run) => {run.workflow_id = 99;},
  'another repository': (f, run) => {run.head_repository.full_name = 'fork/kongctl';},
  'another event': (f, run) => {run.event = 'pull_request';},
  'another run ID': (f, run) => {run.id = 201; f.github.rest.actions.getWorkflowRun = async () => ({data: run});},
  'manual non-PR run': (f, run) => {run.display_title = 'Manual · scenarios · com';},
  'invalid run input': f => {f.context.payload.inputs.completion_run_id = '../200';},
  'missing PR': f => {f.context.payload.inputs.pr_number = '';},
  retry: f => {f.context.payload.inputs.retry = 'true';},
})) {
  test(`completion callback rejects ${name} without writing or dispatching`, async () => {
    const f = fake();
    const run = completionRequest(f);
    mutate(f, run);
    await assert.rejects(reconcile(f), /completion/i);
    assert.equal(f.state.statuses.length, 0);
    assert.equal(f.state.dispatches.length, 0);
  });
}

for (const status of [403, 429]) {
  test(`completion API ${status} fails closed without a false pass`, async () => {
    const f = fake();
    completionRequest(f);
    f.github.rest.actions.getWorkflowRun = async () => {throw Object.assign(new Error('API unavailable'), {status});};
    await assert.rejects(reconcile(f), /API unavailable/);
    assert.equal(f.state.statuses.length, 0);
  });
}

test('an obsolete completion cannot pass a new PR head', async () => {
  const f = fake();
  await reconcile(f);
  const executed = dispatchedRun(f, 'success');
  Object.assign(completionRequest(f), executed);
  f.state.pr.head.sha = 'b'.repeat(40);
  await reconcile(f);
  assert.equal(gate(f).sha, f.state.pr.head.sha);
  assert.equal(gate(f).state, 'pending');
  assert.equal(f.state.dispatches.length, 1);
});

test('trusted-fork completion reevaluates existing evidence without authorizing fork code', async () => {
  const f = fake();
  const run = completionRequest(f);
  Object.assign(run, {status: 'completed', conclusion: 'success',
    display_title: `PR #12 · trusted scenarios · ${f.state.pr.head.sha}`});
  f.state.pr.head.repo.full_name = 'fork/kongctl';
  await reconcile(f);
  assert.equal(gate(f).state, 'pending', 'a wakeup alone is not fork validation evidence');
  assert.equal(f.state.dispatches.length, 0);
  f.state.statuses.unshift({sha: f.state.pr.head.sha, context: requirement(f.state.pr).statusContext,
    state: 'success', description: 'Trusted E2E passed', target_url: 'https://github.com/Kong/kongctl/actions/runs/200'});
  await reconcile(f);
  assert.equal(gate(f).state, 'success');
  assert.equal(f.state.dispatches.length, 0);
});

function readinessRerun(f) {
  const actor = {id: 43, login: 'human', type: 'User'};
  f.state.runs.push({id: 300, event: 'workflow_dispatch', actor, triggering_actor: actor});
  f.context.eventName = 'workflow_dispatch';
  f.context.payload.inputs = {dispatch_source: 'readiness'};
  Object.assign(process.env, {GITHUB_RUN_ID: '300', GITHUB_RUN_ATTEMPT: '2',
    READINESS_PR: '12', READINESS_SHA: f.state.pr.head.sha, READINESS_ENV: 'com', READINESS_MODE: 'pr'});
}

test('a failed shard can obtain fresh admission on the same commit without redispatching the suite', async () => {
  const f = fake();
  readinessRerun(f);
  await recheckRerun(f);
  assert.equal(f.state.outputs.accepted, 'true');
  assert.equal(f.state.outputs.attempt, '2');
  assert.equal(f.state.dispatches.length, 0);
  assert.equal(f.state.statuses.length, 0);
});

test('manual exact-head authorization supports failed-shard retry while a positive review is absent', async () => {
  const f = fake();
  const authorization = manualAuthorization(f);
  f.context.eventName = 'workflow_dispatch';
  f.context.payload.inputs = {dispatch_source: 'readiness'};
  f.state.runs.push({id: 400, event: 'workflow_dispatch', actor: authorization.actor,
    triggering_actor: authorization.actor});
  Object.assign(process.env, {GITHUB_RUN_ID: '400', GITHUB_RUN_ATTEMPT: '2',
    READINESS_PR: '12', READINESS_SHA: f.state.pr.head.sha, READINESS_ENV: 'com', READINESS_MODE: 'pr'});
  const paginate = f.github.paginate;
  let statusReads = 0;
  f.github.paginate = async (method, args) => {
    if (method.method === 'statuses') statusReads++;
    return paginate(method, args);
  };
  await recheckRerun(f);
  assert.equal(f.state.reviews.length, 0);
  assert.equal(f.state.decision, 'REVIEW_REQUIRED');
  assert.equal(statusReads, 1);
  assert.equal(f.state.outputs.accepted, 'true');
  assert.equal(f.state.outputs.attempt, '2');
  assert.equal(f.state.dispatches.length, 0);
});

test('bot-dispatched readiness runs allow an authorized maintainer to retry their failed shards', async () => {
  const f = fake();
  readinessRerun(f);
  Object.assign(f.state.runs[0], {actor: {id: 1, login: 'github-actions[bot]', type: 'Bot'},
    head_branch: 'main', head_sha: 'c'.repeat(40), head_repository: {full_name: 'Kong/kongctl'}});
  process.env.GITHUB_REF = 'refs/heads/main';
  const graphql = f.github.graphql;
  f.github.graphql = async (query, args) => args.head
    ? {repository: {ref: {compare: {status: 'BEHIND'}}}} : graphql(query, args);
  await recheckRerun(f);
  assert.equal(f.state.outputs.accepted, 'true');
  assert.equal(f.state.dispatches.length, 0);
});

test('an author can retry failed shards using their existing exact-head /e2e authorization', async () => {
  const f = fake();
  reviewRequest(f);
  readinessRerun(f);
  f.state.runs[0].actor = f.state.reviews[0].user;
  f.state.runs[0].triggering_actor = f.state.reviews[0].user;
  await recheckRerun(f);
  assert.equal(f.state.decision, 'REVIEW_REQUIRED');
  assert.equal(f.state.outputs.accepted, 'true');
  f.state.reviewProof = {body: 'Authorization removed'};
  await assert.rejects(recheckRerun(f), /E2E retry denied/);
  assert.equal(f.state.dispatches.length, 0);
});

for (const [name, mutate] of Object.entries({
  'new commit': f => {f.state.pr.head.sha = 'b'.repeat(40);},
  draft: f => {f.state.pr.draft = true;},
  closed: f => {f.state.pr.state = 'closed';},
  fork: f => {f.state.pr.head.repo.full_name = 'outside/kongctl';},
  'failed CI': f => {f.state.ci[0].conclusion = 'failure';},
  'unresolved thread': f => {f.state.threads = [{isResolved: false}];},
  'changes request': f => {f.state.reviews.push({state: 'CHANGES_REQUESTED', user: {login: 'reviewer'}});},
  'revoked approval': f => {f.state.reviews = [];},
  'target environment change': f => {f.state.pr.labels = [{name: 'konnect-env:tech'}];},
  'configured environment change': () => {process.env.KONNECT_ENVIRONMENT = 'tech';},
  'routing change': f => {f.state.pr.labels = [{name: 'e2e:force-live'}];},
  'revoked role': f => {f.state.reviewRole = 'write';},
  'unauthorized triggering actor': f => {f.state.runs[0].triggering_actor = {id: 99, login: 'outside', type: 'User'};},
  'unauthorized original actor': f => {f.state.runs[0].actor = {id: 99, login: 'outside', type: 'User'};},
})) {
  test(`failed-shard retry rejects ${name} before expensive work`, async () => {
    const f = fake();
    readinessRerun(f);
    mutate(f);
    await assert.rejects(recheckRerun(f));
    assert.equal(f.state.dispatches.length, 0);
    assert.equal(f.state.statuses.length, 0);
  });
}
