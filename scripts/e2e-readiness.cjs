// Load this module from the same trusted revision as the workflow definition.
const PREREQUISITES = ['CI Test', 'Checks'];
const REQUIRED = new RegExp(
  '^(go\\.mod|go\\.sum|[^/]+\\.go|internal/.*|test/e2e/.*|Makefile|' +
  '\\.goreleaser\\.yml|\\.github/workflows/e2e.*\\.ya?ml|scripts/e2e.*)$',
);
const GATE = 'E2E Required';

function requirement(pr, configuredEnvironment = '') {
  const labels = new Set(pr.labels.map(label => label.name));
  const environment = labels.has('konnect-env:tech') || configuredEnvironment === 'tech' ? 'tech' : 'com';
  const fork = pr.head.repo.full_name !== pr.base.repo.full_name;
  const mode = fork || environment === 'tech' || labels.has('e2e:force-live') ? 'live' : 'pr';
  const key = `${pr.number}/${pr.head.sha}/${environment}/${mode}`;
  return {key, environment, mode, fork, title: `readiness:${key}`,
    statusContext: `E2E Validation / ${pr.number} / ${environment} / ${mode}`};
}

function reviewReady(pr, reviews, decision, threads) {
  if (pr.state !== 'open' || pr.draft || threads.some(thread => !thread.isResolved)) return false;
  if (decision === 'REVIEW_REQUIRED' || decision === 'CHANGES_REQUESTED') return false;
  if (reviews.some(review => !review.user && review.state === 'CHANGES_REQUESTED')) return false;
  const latest = new Map();
  for (const review of reviews) {
    // Comments do not supersede an approval or a changes request.
    if (review.user?.login && ['APPROVED', 'CHANGES_REQUESTED', 'DISMISSED'].includes(review.state)) {
      latest.set(review.user.login, review);
    }
  }
  const active = [...latest.values()];
  return !active.some(review => review.state === 'CHANGES_REQUESTED') &&
    active.some(review => review.state === 'APPROVED' && review.commit_id === pr.head.sha &&
      review.user.type === 'User' && review.user.login !== pr.user.login &&
      ['OWNER', 'MEMBER', 'COLLABORATOR'].includes(review.author_association));
}

function ciReady(runs, pr) {
  return PREREQUISITES.every(name => {
    const latest = runs.filter(run => run.name === name && run.event === 'pull_request' &&
      run.head_sha === pr.head.sha && run.pull_requests?.some(pull => pull.number === pr.number))
      .sort((a, b) => b.id - a.id)[0];
    return latest?.status === 'completed' && latest.conclusion === 'success';
  });
}

function api(github, context) {
  const repo = context.repo;
  const paginate = (method, args) => github.paginate(method, {...repo, per_page: 100, ...args});
  const statusCache = new Map();
  async function snapshot(number) {
    const {data: pr} = await github.rest.pulls.get({...repo, pull_number: number});
    const files = await paginate(github.rest.pulls.listFiles, {pull_number: number});
    // The REST file endpoint is capped at 3000. Fail closed on truncated diffs.
    const required = pr.changed_files > files.length || files.some(file =>
      REQUIRED.test(file.filename) || REQUIRED.test(file.previous_filename || ''));
    const config = process.env.KONNECT_ENVIRONMENT || '';
    if (!['', 'com', 'production', 'prod', 'tech'].includes(config)) throw new Error('Invalid Konnect environment');
    const scope = {pr, required, ...requirement(pr, config)};
    if (pr.state !== 'open' || !required) return {...scope, ready: false};
    let cursor = null;
    const threads = [];
    let decision;
    do {
      const data = await github.graphql(`query($owner:String!, $repo:String!, $number:Int!, $cursor:String) {
        repository(owner:$owner, name:$repo) { pullRequest(number:$number) {
          reviewDecision reviewThreads(first:100, after:$cursor) {
            nodes { isResolved } pageInfo { hasNextPage endCursor }
          }
        } }
      }`, {...repo, number, cursor});
      const pull = data.repository.pullRequest;
      decision = pull.reviewDecision;
      threads.push(...pull.reviewThreads.nodes);
      cursor = pull.reviewThreads.pageInfo.hasNextPage ? pull.reviewThreads.pageInfo.endCursor : null;
    } while (cursor);
    const reviews = await paginate(github.rest.pulls.listReviews, {pull_number: number});
    const runs = await paginate(github.rest.actions.listWorkflowRunsForRepo, {head_sha: pr.head.sha});
    return {...scope, ready: reviewReady(pr, reviews, decision, threads) && ciReady(runs, pr)};
  }
  async function statuses(sha) {
    if (!statusCache.has(sha)) {
      const all = await paginate(github.rest.repos.listCommitStatusesForRef, {ref: sha});
      statusCache.set(sha, new Map(all.slice().reverse().map(status => [status.context, status])));
    }
    return statusCache.get(sha);
  }
  async function status(sha, name, state, description, url) {
    const old = (await statuses(sha)).get(name);
    if (old?.state === state && old.description === description && old.target_url === url) return;
    await github.rest.repos.createCommitStatus({...repo, sha, context: name, state, description, target_url: url});
    statusCache.get(sha).set(name, {state, description, target_url: url});
  }
  return {snapshot, statuses, status, paginate};
}

async function admit({github, context, core}) {
  const client = api(github, context);
  const s = await client.snapshot(Number(process.env.READINESS_PR));
  let reason = '';
  if (s.pr.head.sha !== process.env.READINESS_SHA) {
    reason = `Queued SHA ${process.env.READINESS_SHA}; current PR head ${s.pr.head.sha}.`;
  } else if (s.pr.state !== 'open') {
    reason = 'PR is no longer open.';
  } else if (s.fork) {
    reason = 'Fork PR requires exact-SHA trusted maintainer execution.';
  } else if (s.environment !== process.env.READINESS_ENV || s.mode !== process.env.READINESS_MODE) {
    reason = 'Target environment or live/replay requirement changed while queued.';
  } else if (!s.required || !s.ready) {
    reason = 'PR no longer requires E2E or its review/CI readiness changed while queued.';
  }
  const accepted = !reason;
  core.setOutput('attempt', process.env.GITHUB_RUN_ATTEMPT);
  core.setOutput('accepted', String(accepted));
  core.setOutput('reason', reason);
  core.setOutput('sha', s.pr.head.sha);
  core.setOutput('mode', s.mode);
  core.info(accepted ? `Admitted ${s.key}` : `Skipping expensive work: ${reason}`);
}

function scheduledBatch(pulls, now = Date.now()) {
  const ordered = pulls.toSorted((a, b) => a.number - b.number);
  const batches = Math.ceil(ordered.length / 5);
  const offset = batches ? (Math.floor(now / 300000) % batches) * 5 : 0;
  return ordered.slice(offset, offset + 5);
}

async function reconcile({github, context, core}) {
  const client = api(github, context);
  const number = Number(context.payload.inputs?.pr_number || context.payload.pull_request?.number || 0);
  if (context.payload.inputs?.retry === 'true' && !number) throw new Error('Retry requires pr_number');
  let pulls;
  if (number) {
    pulls = [{number}];
  } else if (context.eventName === 'workflow_run') {
    const run = context.payload.workflow_run;
    const match = /^(?:readiness:|PR #)([0-9]+)(?:\/| · trusted scenarios · )/.exec(run.display_title);
    const numbers = new Set((run.pull_requests || []).map(pr => pr.number));
    if (match) numbers.add(Number(match[1]));
    pulls = [...numbers].map(number => ({number}));
  } else {
    pulls = await client.paginate(github.rest.pulls.list, {state: 'open'});
    if (context.eventName === 'schedule') pulls = scheduledBatch(pulls);
  }
  if (!pulls.length) return;
  let deployed;
  const base = `https://github.com/${context.repo.owner}/${context.repo.repo}`;
  for (const {number} of pulls) {
    try {
      const s = await client.snapshot(number);
      if (s.pr.state !== 'open') continue;
      const old = (await client.statuses(s.pr.head.sha)).get(s.statusContext);
      let state = 'pending';
      let description = s.fork
        ? 'Awaiting exact-SHA trusted maintainer E2E.' : 'Awaiting human review and prerequisite CI.';
      let url = `${base}/pull/${number}/checks`;
      const ticket = old?.description?.split(' ').at(-1);
      const runID = /^https:\/\/github\.com\/[^/]+\/[^/]+\/actions\/runs\/(\d+)$/.exec(old?.target_url || '')?.[1];
      const eventRun = context.payload.workflow_run;
      const candidateID = runID || (eventRun?.display_title === `${s.title}/${ticket}` ? eventRun.id : null);
      let run;
      if (!s.fork && candidateID) {
        try {
          const response = await github.rest.actions.getWorkflowRun({...context.repo, run_id: Number(candidateID)});
          if (response.data.display_title === `${s.title}/${ticket}`) run = response.data;
        } catch (error) {
          if (error.status !== 404) throw error;
        }
      }
      let skipped = /^Skipped(?::| )/.test(old?.description || '');
      if (old && (s.fork || old.state === 'success' || old.state === 'failure')) {
        state = old.state;
        description = s.fork ? old.description : state === 'success'
          ? 'E2E validation passed.' : 'E2E failed or cancelled; retry via readiness controller.';
        url = old.target_url;
      }
      if (!s.fork && run) {
        url = run.html_url;
        if (run.status === 'completed') {
          const attempt = run.run_attempt || 1;
          const storedAttempt = /^(?:Passed|Failed|Skipped):(\d+) /.exec(old?.description || '')?.[1];
          if (Number(storedAttempt) !== attempt) {
            const jobs = await client.paginate(github.rest.actions.listJobsForWorkflowRun, {run_id: run.id});
            const verify = jobs.find(job => job.name === 'Verify scenario results and coverage');
            // A successful workflow with skipped validation is not evidence of a pass.
            const needed = jobs.find(job => job.name === 'Determine scenario requirement');
            const revisionSkipped = jobs.some(job => job.conclusion === 'success' &&
              job.steps?.some(step =>
                step.name === 'Recheck PR head before scenarios' && step.conclusion === 'success') &&
              job.steps?.some(step => step.name === 'Run assigned Konnect scenarios' && step.conclusion === 'skipped'));
            skipped = run.conclusion !== 'cancelled' && needed?.conclusion === 'success' &&
              (needed.run_attempt || 1) === attempt &&
              (verify?.conclusion === 'skipped' || revisionSkipped);
            state = run.conclusion === 'success' && verify?.conclusion === 'success' ? 'success' : 'failure';
            description = state === 'success'
              ? 'E2E validation passed.' : 'E2E failed or cancelled; retry via readiness controller.';
            await client.status(s.pr.head.sha, s.statusContext, state,
              `${skipped ? 'Skipped' : state === 'success' ? 'Passed' : 'Failed'}:${attempt} ${ticket}`, url);
          }
        } else {
          state = 'pending';
          description = 'E2E validation scheduled or running.';
          await client.status(s.pr.head.sha, s.statusContext, 'pending', `Running ${ticket}`, url);
        }
      }
      const retry = context.eventName === 'workflow_dispatch' && context.payload.inputs?.retry === 'true';
      const active = run && run.status !== 'completed';
      if (!s.fork && s.required && s.ready && !active && state !== 'success' &&
          ((!run && !old) || skipped || retry)) {
        // Re-fetch before reserving. All controller executions share one queue.
        const current = await client.snapshot(number);
        if (current.key !== s.key || !current.ready || !current.required) continue;
        if (deployed === undefined) {
          try {
            await github.rest.repos.getContent({...context.repo, path: 'scripts/e2e-readiness.cjs',
              ref: context.payload.repository.default_branch});
            deployed = true;
          } catch (error) {
            if (error.status !== 404) throw error;
            deployed = false;
          }
        }
        if (!deployed) {
          state = 'pending';
          description = 'Awaiting readiness controller default-branch deployment.';
          core.info(description);
        } else {
          await client.status(s.pr.head.sha, s.statusContext, 'pending', `Reserved ${process.env.GITHUB_RUN_ID}`, url);
          const {data: dispatched} = await github.rest.actions.createWorkflowDispatch({...context.repo,
            workflow_id: 'e2e.yaml', headers: {'X-GitHub-Api-Version': '2026-03-10'},
            ref: context.payload.repository.default_branch,
            inputs: {dispatch_source: 'readiness', readiness_pr: String(number), readiness_sha: s.pr.head.sha,
              readiness_mode: s.mode, readiness_ticket: process.env.GITHUB_RUN_ID,
              konnect_environment: s.environment}});
          if (!dispatched?.workflow_run_id) throw new Error('Dispatch returned no run ID; retry via controller.');
          url = `${base}/actions/runs/${dispatched.workflow_run_id}`;
          await client.status(s.pr.head.sha, s.statusContext, 'pending', `Running ${process.env.GITHUB_RUN_ID}`, url);
          state = 'pending';
          description = 'E2E validation scheduled.';
        }
      }
      // This is the only writer of the aggregate PR gate. Scenario jobs only
      // publish keyed fork evidence; old automatic runs cannot overwrite it.
      const {data: currentPR} = await github.rest.pulls.get({...context.repo, pull_number: number});
      if (requirement(currentPR, process.env.KONNECT_ENVIRONMENT).key !== s.key ||
          currentPR.state !== 'open' || currentPR.draft !== s.pr.draft) continue;
      if (!s.required) { state = 'success'; description = 'E2E not required for this PR.'; }
      else if (!s.fork && !s.ready) { state = 'pending'; description = 'Awaiting human review and prerequisite CI.'; }
      await client.status(s.pr.head.sha, GATE, state, description, url);
      core.setOutput('konnect_environment', s.environment);
      core.setOutput('validation_state', old?.state || '');
      core.setOutput('required', String(s.required));
      core.setOutput('fork_pull_request', String(s.fork));
      core.setOutput('pr_number', String(number));
      core.setOutput('pr_head_sha', s.pr.head.sha);
    } catch (error) {
      core.warning(`PR #${number}: readiness reconciliation failed: ${error.message}`);
      if (number === Number(context.payload.inputs?.pr_number || context.payload.pull_request?.number)) throw error;
    }
  }
}
module.exports = {PREREQUISITES, REQUIRED, requirement, reviewReady, ciReady, admit, reconcile, scheduledBatch};


if (require.main === module) {
  if (process.argv[2] !== '--required-files' || !process.argv[3]) {
    throw new Error('Usage: e2e-readiness.cjs --required-files NUL_SEPARATED_FILE');
  }
  const files = require('fs').readFileSync(process.argv[3], 'utf8').split('\0');
  process.stdout.write(String(files.some(file => REQUIRED.test(file))));
}
