// Trusted controller code: always load this module from the default branch.
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
  const latest = new Map();
  for (const review of reviews) {
    // Comments do not supersede an approval or a changes request.
    if (['APPROVED', 'CHANGES_REQUESTED', 'DISMISSED'].includes(review.state)) {
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
      run.head_sha === pr.head.sha && run.pull_requests.some(pull => pull.number === pr.number))
      .sort((a, b) => b.id - a.id)[0];
    return latest?.status === 'completed' && latest.conclusion === 'success';
  });
}

function api(github, context) {
  const repo = context.repo;
  const paginate = (method, args) => github.paginate(method, {...repo, per_page: 100, ...args});
  async function snapshot(number) {
    const {data: pr} = await github.rest.pulls.get({...repo, pull_number: number});
    const files = await paginate(github.rest.pulls.listFiles, {pull_number: number});
    // The REST file endpoint is capped at 3000. Fail closed on truncated diffs.
    const required = pr.changed_files > files.length || files.some(file =>
      REQUIRED.test(file.filename) || REQUIRED.test(file.previous_filename || ''));
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
    const config = process.env.KONNECT_ENVIRONMENT || '';
    if (!['', 'com', 'production', 'prod', 'tech'].includes(config)) throw new Error('Invalid Konnect environment');
    return {pr, required, ...requirement(pr, config),
      ready: reviewReady(pr, reviews, decision, threads) && ciReady(runs, pr)};
  }
  async function statuses(sha) {
    const all = await paginate(github.rest.repos.listCommitStatusesForRef, {ref: sha});
    return new Map(all.slice().reverse().map(status => [status.context, status]));
  }
  async function status(sha, name, state, description, url) {
    const old = (await statuses(sha)).get(name);
    if (old?.state === state && old.description === description && old.target_url === url) return;
    await github.rest.repos.createCommitStatus({...repo, sha, context: name, state, description, target_url: url});
  }
  return {snapshot, statuses, status, paginate};
}

async function admit({github, context, core}) {
  const client = api(github, context);
  const s = await client.snapshot(Number(process.env.READINESS_PR));
  const accepted = !s.fork && s.required && s.ready && s.pr.head.sha === process.env.READINESS_SHA &&
    s.environment === process.env.READINESS_ENV && s.mode === process.env.READINESS_MODE;
  core.setOutput('attempt', process.env.GITHUB_RUN_ATTEMPT);
  core.setOutput('accepted', String(accepted));
  core.setOutput('sha', s.pr.head.sha);
  core.setOutput('mode', s.mode);
  core.info(accepted ? `Admitted ${s.key}` : 'Readiness changed while queued; skipping expensive work.');
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
    const numbers = new Set(run.pull_requests.map(pr => pr.number));
    if (match) numbers.add(Number(match[1]));
    pulls = [...numbers].map(number => ({number}));
  } else {
    pulls = await client.paginate(github.rest.pulls.list, {state: 'open'});
  }
  if (!pulls.length) return;
  const runs = await client.paginate(github.rest.actions.listWorkflowRuns, {
    workflow_id: 'e2e.yaml', event: 'workflow_dispatch',
  });
  const base = `https://github.com/${context.repo.owner}/${context.repo.repo}`;
  for (const {number} of pulls) {
    let s = await client.snapshot(number);
    if (s.pr.state !== 'open') continue;
    const old = (await client.statuses(s.pr.head.sha)).get(s.statusContext);
    let state = 'pending';
    let description = s.fork
      ? 'Awaiting exact-SHA trusted maintainer E2E.' : 'Awaiting human review and prerequisite CI.';
    let url = `${base}/pull/${number}/checks`;
    const ticket = old?.description?.split(' ').at(-1);
    const matches = runs.filter(run => run.display_title === `${s.title}/${ticket}`).sort((a, b) => b.id - a.id);
    const run = matches[0];
    let skipped = old?.description?.startsWith('Skipped ') || false;
    if (old && (s.fork || old.state === 'success' || old.state === 'failure')) {
      state = old.state; description = old.description; url = old.target_url;
    }
    if (!s.fork && run) {
      url = run.html_url;
      if (run.status === 'completed') {
        const jobs = await client.paginate(github.rest.actions.listJobsForWorkflowRun, {run_id: run.id});
        const verify = jobs.find(job => job.name === 'Verify scenario results and coverage');
        // A successful workflow with skipped validation is not evidence of a pass.
        const needed = jobs.find(job => job.name === 'Determine scenario requirement');
        skipped = needed?.conclusion === 'success' && verify?.conclusion === 'skipped';
        state = run.conclusion === 'success' && verify?.conclusion === 'success' ? 'success' : 'failure';
        description = state === 'success'
          ? 'E2E validation passed.' : 'E2E failed or cancelled; retry via readiness controller.';
        await client.status(s.pr.head.sha, s.statusContext, state,
          `${skipped ? 'Skipped' : state === 'success' ? 'Passed' : 'Failed'} ${ticket}`, url);
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
      await client.status(s.pr.head.sha, s.statusContext, 'pending', `Reserved ${process.env.GITHUB_RUN_ID}`, url);
      await github.rest.actions.createWorkflowDispatch({...context.repo, workflow_id: 'e2e.yaml',
        ref: context.payload.repository.default_branch,
        inputs: {dispatch_source: 'readiness', readiness_pr: String(number), readiness_sha: s.pr.head.sha,
          readiness_mode: s.mode, readiness_ticket: process.env.GITHUB_RUN_ID, konnect_environment: s.environment}});
      state = 'pending';
      description = 'E2E validation scheduled.';
    }
    // This is the only writer of the aggregate PR gate. Scenario jobs only
    // publish keyed fork evidence; old automatic runs cannot overwrite it.
    const current = await client.snapshot(number);
    if (current.key !== s.key || current.pr.state !== 'open') continue;
    s = current;
    if (!s.required) { state = 'success'; description = 'E2E not required for this PR.'; }
    else if (!s.fork && !s.ready) { state = 'pending'; description = 'Awaiting human review and prerequisite CI.'; }
    await client.status(s.pr.head.sha, GATE, state, description, url);
    core.setOutput('konnect_environment', s.environment);
    core.setOutput('required', String(s.required));
    core.setOutput('fork_pull_request', String(s.fork));
    core.setOutput('pr_number', String(number));
    core.setOutput('pr_head_sha', s.pr.head.sha);
  }
}
module.exports = {PREREQUISITES, REQUIRED, requirement, reviewReady, ciReady, admit, reconcile};
