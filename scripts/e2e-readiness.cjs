// Load this module from the same trusted revision as the workflow definition.
const PREREQUISITES = [
  {name: 'CI Test', path: '.github/workflows/test.yaml'},
  {name: 'Checks', path: '.github/workflows/checks.yaml'},
];
const REQUIRED = new RegExp(
  '^(go\\.mod|go\\.sum|[^/]+\\.go|internal/.*|test/e2e/.*|Makefile|' +
  '\\.goreleaser\\.yml|\\.github/workflows/e2e.*\\.ya?ml|scripts/e2e.*)$',
);
const GATE = 'E2E Required';
const AUTHORIZATION_WORKFLOW = 'e2e-authorize.yaml';
const AUTHORIZATION_JOB = 'Authorize exact PR commit for E2E';
const AUTHORIZATION_STEP = 'Record authorization';
const RESERVATION_GRACE_MS = 10 * 60 * 1000;
const authorizationContext = number => `E2E Authorization / ${number}`;
const authorizationTitle = pr => `authorize-e2e:${pr.number}/${pr.head.sha}`;

async function maintainerAccess(github, context, user) {
  if (user?.type !== 'User' || !user.login) return false;
  const {data: access} = await github.rest.repos.getCollaboratorPermissionLevel({...context.repo,
    username: user.login});
  // The legacy permission field maps Maintain to Write. Use the actual role.
  return ['maintain', 'admin'].includes(access.role_name);
}

async function trustedRevision(github, context, defaultBranch, revision) {
  if (!/^[a-f0-9]{40}$/.test(revision || '')) return false;
  const comparison = await github.graphql(`query($owner:String!, $repo:String!, $base:String!, $head:String!) {
    repository(owner:$owner, name:$repo) {
      ref(qualifiedName:$base) { compare(headRef:$head) { status } }
    }
  }`, {...context.repo, base: `refs/heads/${defaultBranch}`, head: revision});
  return ['BEHIND', 'IDENTICAL'].includes(comparison.repository?.ref?.compare?.status);
}

async function trustedAuthorizationRun(github, context, pr, run, defaultBranch, completed = true) {
  const repository = `${context.repo.owner}/${context.repo.repo}`;
  if (run.head_repository?.full_name !== repository || run.display_title !== authorizationTitle(pr) ||
      run.run_attempt !== 1 || run.actor?.type !== 'User' ||
      run.triggering_actor?.id !== run.actor.id ||
      (completed && (run.status !== 'completed' || run.conclusion !== 'success'))) return false;
  if (run.event !== 'workflow_dispatch' || run.head_branch !== defaultBranch) return false;
  const revision = run.head_sha;
  const {data: workflow} = await github.rest.actions.getWorkflow({...context.repo,
    workflow_id: AUTHORIZATION_WORKFLOW});
  if (run.workflow_id !== workflow.id || !await trustedRevision(github, context, defaultBranch, revision)) return false;
  if (completed) {
    const jobs = await github.paginate(github.rest.actions.listJobsForWorkflowRun,
      {...context.repo, run_id: run.id, per_page: 100});
    if (!jobs.some(job => job.name === AUTHORIZATION_JOB && job.conclusion === 'success' &&
        job.steps?.some(step => step.name === AUTHORIZATION_STEP && step.conclusion === 'success'))) return false;
  }
  return maintainerAccess(github, context, run.actor);
}

async function scenarioDispatch({github, context, core}) {
  if (context.eventName !== 'workflow_dispatch') return;
  const {data: run} = await github.rest.actions.getWorkflowRun({...context.repo,
    run_id: Number(process.env.GITHUB_RUN_ID)});
  const client = api(github, context);
  const defaultBranch = await client.defaultBranch();
  const actors = new Map([run.actor, run.triggering_actor].map(actor =>
    [JSON.stringify([actor?.id, actor?.login, actor?.type]), actor]));
  for (const actor of actors.values()) {
    if (await maintainerAccess(github, context, actor)) continue;
    const inputs = context.payload.inputs || {};
    const internal = actor?.type === 'Bot' && actor.login === 'github-actions[bot]' &&
      ['readiness', 'main-debounce'].includes(inputs.dispatch_source) && !inputs.trusted_pr_number && !inputs.trusted_head_sha &&
      run.event === 'workflow_dispatch' && run.head_branch === defaultBranch &&
      run.head_repository?.full_name === `${context.repo.owner}/${context.repo.repo}` &&
      process.env.GITHUB_REF === `refs/heads/${defaultBranch}` &&
      await trustedRevision(github, context, defaultBranch, run.head_sha);
    if (!internal) throw new Error('E2E dispatch requires a human with repository Maintain or Admin access.');
  }
  const inputs = context.payload.inputs || {};
  if (inputs.trusted_pr_number || inputs.trusted_head_sha) {
    if (!/^[1-9][0-9]*$/.test(inputs.trusted_pr_number || '') ||
        !/^[a-f0-9]{40}$/.test(inputs.trusted_head_sha || '')) throw new Error('Trusted fork execution requires a PR and exact SHA.');
    const {data: pr} = await github.rest.pulls.get({...context.repo, pull_number: Number(inputs.trusted_pr_number)});
    if (pr.state !== 'open' || pr.draft || !pr.head.repo || pr.head.sha !== inputs.trusted_head_sha ||
        pr.head.repo.full_name === pr.base.repo.full_name) throw new Error('Trusted fork request is stale or ineligible.');
  }
  core.setOutput('attempt', process.env.GITHUB_RUN_ATTEMPT);
}

async function authorize({github, context, core}) {
  const client = api(github, context);
  const defaultBranch = await client.defaultBranch();
  if (context.eventName !== 'workflow_dispatch') throw new Error('Unsupported E2E authorization request.');
  const {pr_number: number, head_sha: sha} = context.payload.inputs;
  if (process.env.GITHUB_REF !== `refs/heads/${defaultBranch}` || !/^[1-9][0-9]*$/.test(number) ||
      !/^[a-f0-9]{40}$/.test(sha)) throw new Error('A PR number and full lowercase head SHA are required.');
  const {data: pr} = await github.rest.pulls.get({...context.repo, pull_number: Number(number)});
  if (pr.head.sha !== sha || pr.state !== 'open' || pr.draft) {
    throw new Error('Authorization requires an open, non-draft PR at the exact supplied head SHA.');
  }
  if (pr.head.repo?.full_name !== pr.base.repo.full_name) {
    throw new Error('Fork PRs require the separate exact-SHA trusted E2E workflow.');
  }
  const {data: run} = await github.rest.actions.getWorkflowRun({...context.repo,
    run_id: Number(process.env.GITHUB_RUN_ID)});
  if (!await trustedAuthorizationRun(github, context, pr, run, defaultBranch, false)) {
    throw new Error('Authorization requires a human with repository Maintain or Admin access on the default branch; ' +
      'dispatch a new run.');
  }
  // The status is only a pointer. Readers must validate the successful trusted
  // workflow run and its GitHub-owned title/actor metadata before accepting it.
  await github.rest.repos.createCommitStatus({...context.repo, sha, context: authorizationContext(pr.number),
    state: 'success', description: `Authorized by ${run.actor.login}`, target_url: run.html_url});
  core.info(`E2E authorized by ${run.actor.login} for PR #${pr.number} at ${sha}: ${run.html_url}`);
}

function requirement(pr, configuredEnvironment = '') {
  const labels = new Set(pr.labels.map(label => label.name));
  const environment = labels.has('konnect-env:tech') || configuredEnvironment === 'tech' ? 'tech' : 'com';
  const fork = pr.head.repo?.full_name !== pr.base.repo.full_name;
  const mode = fork || environment === 'tech' || labels.has('e2e:force-live') ? 'live' : 'pr';
  const key = `${pr.number}/${pr.head.sha}/${environment}/${mode}`;
  return {key, environment, mode, fork, title: `readiness:${key}`,
    statusContext: `E2E Validation / ${pr.number} / ${environment} / ${mode}`};
}

function latestReviews(reviews) {
  const latest = new Map();
  for (const review of reviews) {
    // Comments do not supersede an approval or a changes request.
    if (review.user?.login && ['APPROVED', 'CHANGES_REQUESTED', 'DISMISSED'].includes(review.state)) {
      latest.set(review.user.login, review);
    }
  }
  return [...latest.values()];
}

function currentApproval(pr, review) {
  return review.state === 'APPROVED' && review.commit_id === pr.head.sha &&
    review.user?.type === 'User' && review.user.login !== pr.user.login &&
    ['OWNER', 'MEMBER', 'COLLABORATOR'].includes(review.author_association);
}

function reviewBlocker(pr, reviews, decision, threads) {
  if (pr.state !== 'open') return 'PR is closed.';
  if (pr.draft) return 'PR is a draft.';
  if (threads.some(thread => !thread.isResolved)) return 'Unresolved review threads.';
  if (decision === 'CHANGES_REQUESTED' ||
      reviews.some(review => !review.user && review.state === 'CHANGES_REQUESTED') ||
      latestReviews(reviews).some(review => review.state === 'CHANGES_REQUESTED')) return 'Active changes requests.';
  return '';
}

function ciBlockers(runs, pr) {
  return PREREQUISITES.filter(({path}) => {
    const latest = runs.filter(run => run.path?.split('@')[0] === path && run.event === 'pull_request' &&
      run.head_sha === pr.head.sha && run.pull_requests?.some(pull => pull.number === pr.number))
      .sort((a, b) => b.id - a.id)[0];
    return latest?.status !== 'completed' || latest.conclusion !== 'success';
  }).map(({name}) => name);
}

function ciReady(runs, pr) { return ciBlockers(runs, pr).length === 0; }

async function trustedReviewRequest(github, context, pr, review) {
  if (!review.node_id) return false;
  const {node} = await github.graphql(`query($reviewID:ID!) { node(id:$reviewID) {
    ... on PullRequestReview { fullDatabaseId body state submittedAt lastEditedAt
      author { login } editor { login } commit { oid }
      pullRequest { number repository { nameWithOwner } }
    }
  } }`, {reviewID: review.node_id});
  const login = review.user.login.toLowerCase();
  const ownEdit = (!node?.lastEditedAt && !node?.editor) || node?.editor?.login?.toLowerCase() === login;
  return node && String(node.fullDatabaseId) === String(review.id) && node.submittedAt &&
    node.body?.trim() === '/e2e' && ['COMMENTED', 'APPROVED'].includes(node.state) &&
    node.commit?.oid === pr.head.sha && node.author?.login?.toLowerCase() === login && ownEdit &&
    node.pullRequest?.number === pr.number &&
    node.pullRequest.repository.nameWithOwner === `${context.repo.owner}/${context.repo.repo}`;
}

function api(github, context) {
  const repo = context.repo;
  const paginate = (method, args) => github.paginate(method, {...repo, per_page: 100, ...args});
  const statusCache = new Map();
  let branch;
  async function defaultBranch() {
    if (!branch) {
      const {data: repository} = await github.rest.repos.get(repo);
      if (!repository.default_branch) throw new Error('Repository default branch is unavailable.');
      branch = repository.default_branch;
    }
    return branch;
  }
  async function snapshot(number, {fresh = false} = {}) {
    const {data: pr} = await github.rest.pulls.get({...repo, pull_number: number});
    if (fresh) {
      statusCache.delete(pr.head.sha);
    }
    const files = await paginate(github.rest.pulls.listFiles, {pull_number: number});
    // The REST file endpoint is capped at 3000. Fail closed on truncated diffs.
    const required = pr.changed_files > files.length || files.some(file =>
      REQUIRED.test(file.filename) || REQUIRED.test(file.previous_filename || ''));
    const config = process.env.KONNECT_ENVIRONMENT || '';
    if (!['', 'com', 'production', 'prod', 'tech'].includes(config)) throw new Error('Invalid Konnect environment');
    const scope = {pr, required, ...requirement(pr, config)};
    if (pr.state !== 'open') return {...scope, ready: false, reason: 'PR is closed.'};
    if (pr.draft) return {...scope, ready: false, reason: 'PR is a draft.'};
    if (scope.fork) return {...scope, ready: false, reason: 'Trusted fork execution required.'};
    if (!required) return {...scope, ready: false};
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
    const blocker = reviewBlocker(pr, reviews, decision, threads);
    if (blocker) return {...scope, ready: false, reason: blocker};
    const runs = await paginate(github.rest.actions.listWorkflowRunsForRepo, {head_sha: pr.head.sha});
    const waiting = ciBlockers(runs, pr);
    if (waiting.length) return {...scope, ready: false, reason: `Awaiting prerequisite CI: ${waiting.join(', ')}.`};
    const permission = new Map();
    const allowed = async user => {
      if (!permission.has(user.login)) permission.set(user.login, await maintainerAccess(github, context, user));
      return permission.get(user.login);
    };
    const requests = reviews.filter(review => ['COMMENTED', 'APPROVED'].includes(review.state) &&
      review.body?.trim() === '/e2e' && review.commit_id === pr.head.sha &&
      Number.isSafeInteger(review.id) && review.user?.type === 'User').sort((a, b) => b.id - a.id);
    for (const review of requests) {
      if (await allowed(review.user) && await trustedReviewRequest(github, context, pr, review)) {
        return {...scope, ready: true, request: `review-${review.id}`,
          authorizer: review.user.login, authorizationURL: review.html_url};
      }
    }
    if (decision !== 'REVIEW_REQUIRED') {
      for (const review of latestReviews(reviews).filter(review => currentApproval(pr, review))) {
        if (await allowed(review.user)) return {...scope, ready: true, authorizer: review.user.login,
          authorizationURL: review.html_url};
      }
    }
    let authorized = false;
    const evidence = (await statuses(pr.head.sha)).get(authorizationContext(number));
    const prefix = `https://github.com/${repo.owner}/${repo.repo}/actions/runs/`;
    const runID = evidence?.target_url?.startsWith(prefix) && evidence.target_url.slice(prefix.length);
    if (evidence?.state === 'success' && /^[1-9][0-9]*$/.test(runID || '')) {
      try {
        const {data: run} = await github.rest.actions.getWorkflowRun({...repo, run_id: Number(runID)});
        authorized = await trustedAuthorizationRun(github, context, pr, run, await defaultBranch());
      } catch (error) {
        if (error.status !== 404) throw error;
      }
    }
    return {...scope, ready: authorized, authorizationURL: evidence?.target_url,
      reason: authorized ? '' : 'Awaiting maintainer Comment review: /e2e.'};
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
    statusCache.get(sha).set(name, {state, description, target_url: url, created_at: new Date().toISOString()});
  }
  async function executionCheck(s, executionState, url) {
    if (s.fork) return;
    const base = `https://github.com/${repo.owner}/${repo.repo}`;
    const runURL = url?.startsWith(`${base}/actions/runs/`) ? url : '';
    const name = `E2E / PR #${s.pr.number}`;
    const summary = [
      `PR #${s.pr.number} · commit \`${s.pr.head.sha}\` · ${s.environment}/${s.mode}`,
      s.required ? (s.reason || 'Readiness requirements satisfied.') : 'E2E is not required for this commit.',
      s.authorizationURL ? `Authorization: [view evidence](${s.authorizationURL}).` : 'No accepted authorization yet.',
      runURL ? `Latest E2E execution: [open run](${runURL}) — **${executionState}**.` : 'No E2E execution for this commit yet.',
      `**Start or retry:** [Files changed → Review changes → Comment](${base}/pull/${s.pr.number}/files), ` +
        'submit a new review containing only \`/e2e\`. Only humans with repository Maintain or Admin access may do this.',
      'Each submitted review is bound by GitHub to its commit. Repeat after a push; older commands do not authorize new commits.',
      runURL ? 'For transient failures, open the run and choose **Re-run failed jobs** to retry failed shards on this commit. ' +
        'Each retried job rechecks current permissions and readiness; successful shards are reused.' : '',
      runURL ? 'Reruns retain their original workflow revision. Runs predating failed-shard retry support need a fresh ' +
        '`/e2e` run on the same commit to adopt the fix.' : '',
      'A new command during active validation requests one follow-up run after it finishes.',
      'If dispatch was interrupted or its run disappeared, a new command recovers after a ten-minute visibility grace period.',
      `For immediate recovery, a human with repository Maintain or Admin access can run: \`gh workflow run e2e-required-status.yaml ` +
        `--repo ${repo.owner}/${repo.repo} --ref ${await defaultBranch()} -f pr_number=${s.pr.number} -f retry=true\`.`,
      'This check links execution to the PR; **E2E Required** remains the protected merge gate.',
    ].filter(Boolean).join('\n\n');
    const status = !s.required || ['success', 'failure'].includes(executionState) ? 'completed' : runURL ? 'in_progress' : 'queued';
    const conclusion = !s.required ? 'skipped' : executionState === 'success' ? 'success' : 'failure';
    const body = {name, head_sha: s.pr.head.sha, external_id: s.key, status,
      details_url: runURL || `${base}/pull/${s.pr.number}/files`,
      output: {title: !s.required ? 'E2E not required' : `E2E ${executionState}`, summary},
      ...(status === 'completed' ? {conclusion} : {})};
    const checks = (await paginate(github.rest.checks.listForRef, {ref: s.pr.head.sha})).filter(check =>
      check.app?.slug === 'github-actions' && (check.name === name || check.name.startsWith(`${name} / `)));
    for (const check of checks.filter(check => (check.external_id !== s.key || check.name !== name) && check.status !== 'completed')) {
      await github.rest.checks.update({...repo, check_run_id: check.id, status: 'completed', conclusion: 'neutral',
        output: {title: 'Superseded E2E requirement',
          summary: `The current requirement is ${s.environment}/${s.mode}. See the latest ${name} check. This does not cancel an active run.`}});
    }
    const old = checks.find(check => check.name === name && check.external_id === s.key);
    if (!old || (old.status === 'completed' && status !== 'completed')) {
      await github.rest.checks.create({...repo, ...body});
    }
    else if (old.status !== body.status || (old.conclusion || undefined) !== body.conclusion || old.details_url !== body.details_url ||
        old.output?.title !== body.output.title || old.output?.summary !== summary) {
      const {head_sha: _, ...update} = body;
      await github.rest.checks.update({...repo, check_run_id: old.id, ...update});
    }
  }
  return {snapshot, statuses, status, paginate, defaultBranch, executionCheck};
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
  return {accepted, reason};
}

async function recheckRerun(args) {
  await scenarioDispatch(args);
  if (args.context.payload.inputs?.dispatch_source === 'readiness') {
    const {accepted, reason} = await admit(args);
    if (!accepted) throw new Error(`E2E retry denied: ${reason}`);
  }
}

function scheduledBatch(pulls, now = Date.now()) {
  const ordered = pulls.toSorted((a, b) => a.number - b.number);
  const batches = Math.ceil(ordered.length / 5);
  const offset = batches ? (Math.floor(now / 300000) % batches) * 5 : 0;
  return ordered.slice(offset, offset + 5);
}

async function waitForCompletion({github, context, core}, pause = ms => new Promise(resolve => setTimeout(resolve, ms))) {
  const {completion_run_id: id, pr_number: number, retry} = context.payload.inputs || {};
  if (!/^[1-9][0-9]*$/.test(id || '') || !Number.isSafeInteger(Number(id)) ||
      !/^[1-9][0-9]*$/.test(number || '') || !Number.isSafeInteger(Number(number)) || retry === 'true') {
    throw new Error('Completion requires a valid run ID and PR number, without retry.');
  }
  const {data: workflow} = await github.rest.actions.getWorkflow({...context.repo, workflow_id: 'e2e.yaml'});
  for (let attempt = 0; attempt < 24; attempt++) {
    let run;
    try {
      ({data: run} = await github.rest.actions.getWorkflowRun({...context.repo, run_id: Number(id)}));
    } catch (error) {
      if (error.status !== 404) throw error;
    }
    if (run) {
      const match = /^readiness:([1-9][0-9]*)\/[a-f0-9]{40}\/(?:com|tech)\/(?:pr|live)\/[1-9][0-9]*$/.exec(run.display_title) ||
        /^PR #([1-9][0-9]*) · trusted scenarios · [a-f0-9]{40}$/.exec(run.display_title);
      if (run.id !== Number(id) || run.workflow_id !== workflow.id || run.event !== 'workflow_dispatch' ||
          run.head_repository?.full_name !== `${context.repo.owner}/${context.repo.repo}` || match?.[1] !== number) {
        throw new Error('Completion run does not match the scenario workflow and PR.');
      }
      if (run.status === 'completed') return;
    }
    if (attempt < 23) {
      core.info(`Waiting for E2E run ${id} to finish before reconciliation.`);
      await pause(5000);
    }
  }
  throw new Error(`Completion run ${id} is still not complete; scheduled reconciliation remains available.`);
}

async function reconcile({github, context, core, pause}) {
  if (context.eventName === 'workflow_dispatch' && context.payload.inputs?.completion_run_id) {
    // This is only a wakeup. Reconciliation independently validates all evidence.
    await waitForCompletion({github, context, core}, pause);
  }
  const client = api(github, context);
  const number = Number(context.payload.inputs?.pr_number || context.payload.pull_request?.number || 0);
  if (context.payload.inputs?.retry === 'true' && !number) throw new Error('Retry requires pr_number');
  if (context.eventName === 'workflow_dispatch' && context.payload.inputs?.retry === 'true') {
    await scenarioDispatch({github, context, core});
  }
  let pulls;
  if (number) {
    pulls = [{number}];
  } else if (context.eventName === 'workflow_run') {
    const run = context.payload.workflow_run;
    const match = /^(?:readiness:|authorize-e2e:|PR #)([0-9]+)(?:\/| · trusted scenarios · )/.exec(run.display_title);
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
      let description = old?.state === 'pending' ? 'E2E validation reserved or running.' : s.fork
        ? 'Awaiting exact-SHA trusted maintainer E2E.' : 'Awaiting human review and prerequisite CI.';
      let url = old?.target_url || `${base}/pull/${number}/checks`;
      const ticket = old?.description?.split(' ').at(-1);
      const previousRequest = /\brequest=(review-\d+)\b/.exec(old?.description || '')?.[1] || '';
      const freshRequest = Boolean(s.request && s.request !== previousRequest);
      const tag = request => request ? `request=${request} ` : '';
      const runID = /^https:\/\/github\.com\/[^/]+\/[^/]+\/actions\/runs\/(\d+)$/.exec(old?.target_url || '')?.[1];
      const eventRun = context.payload.workflow_run;
      const candidateID = runID || (eventRun?.display_title === `${s.title}/${ticket}` ? eventRun.id : null);
      const retry = context.eventName === 'workflow_dispatch' && context.payload.inputs?.retry === 'true';
      const reserved = old?.state === 'pending' && /^(Reserved|Running) /.test(old.description);
      const reservedAt = Date.parse(old?.created_at || '1970-01-01T00:00:00Z');
      const expired = Date.now() - reservedAt >= RESERVATION_GRACE_MS;
      let run;
      if (!s.fork && candidateID) {
        try {
          const response = await github.rest.actions.getWorkflowRun({...context.repo, run_id: Number(candidateID)});
          if (response.data.display_title === `${s.title}/${ticket}`) run = response.data;
        } catch (error) {
          if (error.status !== 404) throw error;
        }
      }
      // An interrupted dispatch can have created a run without returning its ID.
      // Look for that ticket before allowing an explicit request to recover it.
      if (!s.fork && !run && reserved && (retry || (freshRequest && expired))) {
        const runs = await client.paginate(github.rest.actions.listWorkflowRuns, {
          workflow_id: 'e2e.yaml', event: 'workflow_dispatch',
          ...(old.created_at ? {created: `>=${new Date(reservedAt - 60000).toISOString()}`} : {}),
        });
        run = runs.find(candidate => candidate.display_title === `${s.title}/${ticket}`);
        if (!run && runs.length >= 1000) throw new Error('Recovery history is truncated; inspect the reserved E2E run.');
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
              `${skipped ? 'Skipped' : state === 'success' ? 'Passed' : 'Failed'}:${attempt} ${tag(previousRequest)}${ticket}`, url);
          }
        } else {
          state = 'pending';
          description = 'E2E validation scheduled or running.';
          await client.status(s.pr.head.sha, s.statusContext, 'pending', `Running ${tag(previousRequest)}${ticket}`, url);
        }
      }
      const active = (run && run.status !== 'completed') ||
        (!run && reserved && !retry && !(freshRequest && expired));
      if (!s.fork && s.required && s.ready && !active && (state !== 'success' || freshRequest) &&
          ((!run && !old) || skipped || retry || freshRequest)) {
        // Re-fetch before reserving. All controller executions share one queue.
        const current = await client.snapshot(number, {fresh: true});
        if (current.key !== s.key || !current.ready || !current.required || current.request !== s.request) continue;
        const defaultBranch = await client.defaultBranch();
        if (deployed === undefined) {
          try {
            await github.rest.repos.getContent({...context.repo, path: 'scripts/e2e-readiness.cjs',
              ref: defaultBranch});
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
          await client.status(s.pr.head.sha, s.statusContext, 'pending', `Reserved ${tag(s.request)}${process.env.GITHUB_RUN_ID}`, url);
          const {data: dispatched} = await github.rest.actions.createWorkflowDispatch({...context.repo,
            workflow_id: 'e2e.yaml', headers: {'X-GitHub-Api-Version': '2026-03-10'},
            ref: defaultBranch,
            inputs: {dispatch_source: 'readiness', readiness_pr: String(number), readiness_sha: s.pr.head.sha,
              readiness_mode: s.mode, readiness_ticket: process.env.GITHUB_RUN_ID,
              konnect_environment: s.environment}});
          if (!dispatched?.workflow_run_id) throw new Error('Dispatch returned no run ID; retry via controller.');
          url = `${base}/actions/runs/${dispatched.workflow_run_id}`;
          await client.status(s.pr.head.sha, s.statusContext, 'pending', `Running ${tag(s.request)}${process.env.GITHUB_RUN_ID}`, url);
          state = 'pending';
          description = 'E2E validation scheduled.';
        }
      }
      // This is the only writer of the aggregate PR gate. Scenario jobs only
      // publish keyed fork evidence; old automatic runs cannot overwrite it.
      const {data: currentPR} = await github.rest.pulls.get({...context.repo, pull_number: number});
      if (requirement(currentPR, process.env.KONNECT_ENVIRONMENT).key !== s.key ||
          currentPR.state !== 'open' || currentPR.draft !== s.pr.draft) continue;
      const executionState = state;
      if (!s.required) { state = 'success'; description = 'E2E not required for this PR.'; }
      else if (!s.fork && !s.ready) { state = 'pending'; description = s.reason || 'Awaiting readiness.'; }
      await client.status(s.pr.head.sha, GATE, state, description, url);
      // Execution checks are navigation aids; their API must not block the gate.
      if (s.required || old) {
        try { await client.executionCheck(s, executionState, url); }
        catch (error) { core.warning(`PR #${number}: could not update E2E execution check: ${error.message}`); }
      }
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
module.exports = {PREREQUISITES, REQUIRED, requirement, ciReady, admit, reconcile,
  scheduledBatch, authorize, scenarioDispatch, waitForCompletion, recheckRerun,
  AUTHORIZATION_JOB, AUTHORIZATION_STEP};


if (require.main === module) {
  if (process.argv[2] !== '--required-files' || !process.argv[3]) {
    throw new Error('Usage: e2e-readiness.cjs --required-files NUL_SEPARATED_FILE');
  }
  const files = require('fs').readFileSync(process.argv[3], 'utf8').split('\0');
  process.stdout.write(String(files.some(file => REQUIRED.test(file))));
}
