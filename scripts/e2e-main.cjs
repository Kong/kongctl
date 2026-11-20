// Trusted control-plane code only; never load this module from a PR revision.
const {REQUIRED, readinessAPI, maintainerAccess} = require('./e2e-readiness.cjs');
const STATE_REF = 'heads/automation/e2e-main-state';
const NIGHT = '7 7 * * *';
const PREFIX = 'main · scenarios · overnight · ';
const GRACE_MS = 10 * 60 * 1000;
const AGE_MS = 36 * 60 * 60 * 1000;

function client(github, context) {
  const repo = context.repo;
  const paginate = (method, args = {}) => github.paginate(method, {...repo, per_page: 100, ...args});
  let parent;
  async function read() {
    try { parent = (await github.rest.git.getRef({...repo, ref: STATE_REF})).data.object.sha; }
    catch (error) { if (error.status === 404) return {}; throw error; }
    const {data: file} = await github.rest.repos.getContent({...repo, path: 'state.json', ref: parent});
    const state = JSON.parse(Buffer.from(file.content, 'base64').toString('utf8'));
    if (state.version !== 1) throw new Error('Unknown main E2E state version.');
    return state;
  }
  async function write(state) {
    // A dedicated data-only branch retains state and its history without
    // modifying main or relying on expiring Actions artifacts.
    const {data: tree} = await github.rest.git.createTree({...repo, tree: [
      {path: 'state.json', mode: '100644', type: 'blob', content: JSON.stringify({...state, version: 1})},
    ]});
    const {data: commit} = await github.rest.git.createCommit({...repo,
      message: 'ci: record main E2E scheduling state', tree: tree.sha, parents: parent ? [parent] : []});
    if (parent) await github.rest.git.updateRef({...repo, ref: STATE_REF, sha: commit.sha});
    else await github.rest.git.createRef({...repo, ref: `refs/${STATE_REF}`, sha: commit.sha});
    parent = commit.sha;
  }
  return {read, write, paginate};
}

async function blockers({github, context}, excludeRun) {
  const c = client(github, context);
  // Include manual, trusted fork and merge-queue uses of the existing pool.
  // Conservatively yield across both environment queues; never cancel live use.
  const runs = (await Promise.all(['queued', 'in_progress', 'waiting', 'pending', 'requested'].map(status =>
    c.paginate(github.rest.actions.listWorkflowRuns, {workflow_id: 'e2e.yaml', status})))).flat();
  const active = runs.filter(run => run.id !== excludeRun && run.status !== 'completed');
  if (active.length) return active.map(run => `Active/queued scenario run ${run.id}`);
  const readiness = readinessAPI(github, context);
  const pulls = await c.paginate(github.rest.pulls.list, {state: 'open'});
  const reasons = [];
  for (const pr of pulls) {
    if (pr.draft || pr.head.repo?.full_name !== pr.base.repo.full_name) continue;
    const s = await readiness.snapshot(pr.number);
    if (!s.required || s.fork) continue;
    if (s.waitingForCI) reasons.push(`PR #${pr.number}: ${s.reason}`);
    else if (s.ready) {
      const status = (await readiness.statuses(s.pr.head.sha)).get(s.statusContext);
      const previousRequest = /\brequest=(review-\d+)\b/.exec(status?.description || '')?.[1] || '';
      const freshRequest = s.request && s.request !== previousRequest;
      // A new exact-SHA /e2e request takes priority even after a terminal result.
      if (freshRequest || !['success', 'failure', 'error'].includes(status?.state)) {
        reasons.push(`PR #${pr.number}: ready for E2E`);
      }
    }
  }
  return reasons;
}

async function latest(github, context) {
  const {data} = await github.rest.git.getRef({...context.repo,
    ref: `heads/${context.payload.repository.default_branch}`});
  return data.object.sha;
}

async function relevant(github, context, base, head) {
  if (!base) return true; // Establish a full-live baseline at rollout.
  if (base === head) return false;
  const {data} = await github.rest.repos.compareCommitsWithBasehead({...context.repo, basehead: `${base}...${head}`});
  // GitHub caps comparison files at 300. Truncation/divergence requires validation.
  if (data.status !== 'ahead' || data.files.length >= 300 || data.total_commits > data.commits.length) return true;
  return data.files.some(file => REQUIRED.test(file.filename) || REQUIRED.test(file.previous_filename || ''));
}

async function settle(args, state) {
  const {github, context} = args;
  const reservation = state.reservation;
  if (!reservation) return;
  const c = client(github, context);
  let runs;
  if (reservation.run) {
    try { runs = [(await github.rest.actions.getWorkflowRun({...context.repo, run_id: reservation.run})).data]; }
    catch (error) {
      if (error.status !== 404) throw error;
      delete reservation.run;
      return;
    }
  } else {
    runs = await c.paginate(github.rest.actions.listWorkflowRuns,
      {workflow_id: 'e2e.yaml', event: 'workflow_dispatch', branch: context.payload.repository.default_branch,
        created: `>=${new Date(Date.parse(reservation.at) - 60000).toISOString()}`});
  }
  const run = runs.find(run => run.display_title === `${PREFIX}${reservation.sha} · ${reservation.ticket}`);
  if (!run) return; // Ambiguous dispatch: retain reservation until explicit recovery.
  reservation.run = run.id;
  reservation.url = run.html_url;
  if (run.status !== 'completed') return;
  const jobs = await c.paginate(github.rest.actions.listJobsForWorkflowRun, {run_id: run.id});
  const verified = jobs.some(job => job.name === 'Verify scenario results and coverage' && job.conclusion === 'success');
  const deferred = run.conclusion === 'success' &&
    jobs.some(job => job.name === 'Determine scenario requirement' && job.conclusion === 'success' &&
      job.steps?.some(step => step.name === 'Main validation deferred before live work' && step.conclusion === 'success')) &&
    jobs.some(job => job.name === 'Verify scenario results and coverage' && job.conclusion === 'skipped');
  const outcome = run.conclusion === 'success' && verified ? 'success' : deferred ? 'deferred' :
    run.conclusion === 'success' ? 'failure' : run.conclusion || 'failure';
  state.last = {sha: reservation.sha, run: run.id, url: run.html_url, outcome, at: run.updated_at};
  if (outcome === 'success') state.success = {...state.last};
  else if (outcome !== 'deferred') state.failure = {...state.last};
  if (outcome === 'deferred') state.opportunity = true;
  delete state.reservation;
}

async function summary(core, state, head, reason, now) {
  const age = state.success ? Math.max(0, (now - Date.parse(state.success.at)) / 3600000) : null;
  const deferredHours = state.pendingSince ? (now - Date.parse(state.pendingSince)) / 3600000 : 0;
  if (state.pendingSince && (age === null || age * 3600000 >= AGE_MS || deferredHours >= 24)) {
    core.warning(`Main validation needs attention: validation age ${age?.toFixed(1) ?? 'unknown'} hours; ` +
      `pending ${deferredHours.toFixed(1)} hours. ${reason}`);
  }
  if (state.failure && (!state.success || Date.parse(state.failure.at) > Date.parse(state.success.at))) {
    core.warning(`Last main full-live validation ${state.failure.outcome}: ${state.failure.url}`);
  }
  await core.summary.addHeading('Main full-live validation').addTable([
    [{data: 'Field', header: true}, {data: 'Value', header: true}],
    ['Schedule', '07:07 UTC daily; pending work rechecked every 15 minutes, all day'],
    ['Latest main SHA', head], ['Decision', reason],
    ['Last successful tested SHA', state.success?.sha || 'No baseline yet'],
    ['Last successful full-live validation', state.success?.at || 'Unknown'],
    ['Validation age (hours)', age?.toFixed(1) ?? 'Unknown'],
    ['Pending since', state.pendingSince || 'None'],
    ['Reservation', JSON.stringify(state.reservation || null)],
    ['Last result', JSON.stringify(state.last || null)],
    ['Last failure', JSON.stringify(state.failure || null)],
  ]).write();
}

async function reconcile(args) {
  const {github, context, core} = args;
  const branch = context.payload.repository.default_branch;
  if (process.env.GITHUB_REF !== `refs/heads/${branch}`) throw new Error('Main scheduler must run on the default branch.');
  if (context.eventName === 'workflow_dispatch') {
    const {data: run} = await github.rest.actions.getWorkflowRun({...context.repo, run_id: context.runId});
    for (const actor of [run.actor, run.triggering_actor]) {
      if (!await maintainerAccess(github, context, actor)) throw new Error('Run-now requires repository Maintain or Admin.');
    }
  }
  const now = Date.now();
  const c = client(github, context);
  const state = await c.read();
  let saved = JSON.stringify(state);
  await settle(args, state);
  const head = await latest(github, context);
  if (context.eventName === 'workflow_dispatch' || context.payload.schedule === NIGHT) state.opportunity = true;
  const needed = await relevant(github, context, state.success?.sha, head);
  if (needed) state.pendingSince ||= new Date(now).toISOString();
  else { delete state.pendingSince; state.opportunity = false; }
  let reason = !needed ? 'No relevant changes await validation.' : 'Awaiting overnight opportunity.';
  if (state.reservation && !state.reservation.run && context.payload.inputs?.recover_reservation === 'true') {
    if (now - Date.parse(state.reservation.at) < GRACE_MS) throw new Error('Reservation visibility grace period is ten minutes.');
    delete state.reservation;
  }
  if (state.reservation) reason = 'Existing dispatch reservation; no duplicate dispatch.';
  else if (needed && state.opportunity) {
    const busy = await blockers(args);
    if (busy.length) reason = `Deferred: ${busy.join('; ')}`;
    else {
      // Resolve again immediately before reserving to coalesce merges during API reads.
      const sha = await latest(github, context);
      state.reservation = {sha, ticket: `${context.runId}-${process.env.GITHUB_RUN_ATTEMPT || '1'}`,
        at: new Date(now).toISOString()};
      state.opportunity = false;
      await c.write(state); // Persist BEFORE dispatch; ambiguous failures fail closed.
      saved = JSON.stringify(state);
      await github.rest.actions.createWorkflowDispatch({...context.repo, workflow_id: 'e2e.yaml', ref: branch,
        inputs: {dispatch_source: 'main-overnight', main_sha: sha, main_ticket: state.reservation.ticket}});
      reason = `Reserved full-live validation of ${sha}.`;
    }
  }
  if (JSON.stringify(state) !== saved) await c.write(state);
  await summary(core, state, head, reason, now);
  return state;
}

async function admit(args) {
  const {github, context, core} = args;
  const inputs = context.payload.inputs;
  const state = await client(github, context).read();
  let reason = '';
  if (state.reservation?.sha !== inputs.main_sha || state.reservation?.ticket !== inputs.main_ticket) {
    reason = 'Dispatch does not own the current main reservation.';
  } else if (await latest(github, context) !== inputs.main_sha) {
    reason = 'Newer merges superseded the queued main revision.';
  } else {
    reason = (await blockers(args, context.runId)).join('; ');
  }
  core.setOutput('accepted', String(!reason));
  core.info(reason ? `Deferring main validation: ${reason}` : `Admitted full-live main SHA ${inputs.main_sha}`);
  return {accepted: !reason, reason};
}

module.exports = {reconcile, admit, blockers, relevant, client, settle, NIGHT, PREFIX, STATE_REF};
