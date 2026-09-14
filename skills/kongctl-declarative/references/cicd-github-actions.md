# Declarative CI/CD with GitHub Actions

Use this reference when creating GitHub Actions workflows for kongctl.
For a complete AI Gateway manifest and both workflows, start with the
[GitHub Actions quickstart][quickstart].

[quickstart]: https://developer.konghq.com/kongctl/ci-cd/github-actions/

## Basic GitOps: diffs on PRs, apply on main

For requests such as "create a GitOps repository for one AI Gateway; plan on
PRs and apply on main", use the quickstart's direct diff/apply pattern unless
the user explicitly asks for saved plan artifacts:

- Store the requested resources in a dedicated manifest or resource directory.
  Use stable names and refs and an explicit namespace.
- On `pull_request` targeting `main`, run
  `kongctl diff --mode apply -f <manifest> -o text`.
- On `push` to `main`, run
  `kongctl apply -f <manifest> --auto-approve -o text`.
- Display the diff in workflow logs or the job summary. No PR comment script
  or artifact transfer is required for this pattern.
- Apply plans against live state again before execution. Explain that the
  deployment can differ from an earlier PR preview.
- Apply creates and updates; it does not delete omitted resources. Use sync
  only when the user wants deletion-based reconciliation.

Creating the repository template does not require live credentials. Provide
the exact GitHub setup instructions and report which checks were performed
locally versus against Konnect.

## Installation, authentication, and triggers

- Install with `kong/setup-kongctl@v1` and set its `kongctl-version` input
  to the same tested release in both workflows.
- Map repository secret `KONNECT_TOKEN` to
  `KONGCTL_DEFAULT_KONNECT_PAT` in both workflows.
- Set repository variable `KONNECT_REGION` and pass it through an environment
  variable to `--region` in both commands. Planning and diff require
  authenticated access to live Konnect state.
- For an AI Gateway provider, use `!secret` with `!env OPENAI_API_KEY`,
  composing the public `Bearer ` prefix with `parts` where needed.
  Expose the GitHub `OPENAI_API_KEY` secret only to apply; diff does not need
  deferred secret values.
- An ordinary apply writes provider secrets on creation. Updating a GitHub
  secret alone does not rotate an existing credential; secret rotation needs
  explicit secret-write selection.
- Limit PR runs to trusted same-repository branches and exclude Dependabot
  runs that lack secrets. Do not use `pull_request_target` to execute
  untrusted PR content with credentials.
- Use `permissions: {contents: read}` for workflows that only show logs and
  summaries. Include both resource files and workflow files in path filters.
- Serialize main deployments with a shared concurrency group and
  `cancel-in-progress: false`.
- When piping diff output through `tee`, use `shell: bash` so GitHub's
  default Bash pipeline failure handling preserves command failures.
- Only install decK when `_deck` or other decK operations are actually used.

## Saved-plan workflows

Use this when the user requests plan files, artifact review, or promotion.
Generate the plan once and render the same saved artifact:

```bash
mkdir -p .konnect
kongctl plan --mode apply -f konnect/resources --recursive \
  --output-file .konnect/plan.json
kongctl diff --plan .konnect/plan.json -o text
```

After the workflow's review or approval step:

```bash
kongctl apply --plan .konnect/plan.json --auto-approve -o text
```

`plan` always outputs JSON and rejects `-o/--output`; use `--output-file`.
The execution command must match the saved plan's mode. For cross-workflow
promotion, define how the artifact is associated with the reviewed commit,
target account and region, and how stale plans are handled. Do not invent an
artifact URL or assume a previous PR plan automatically reaches the main job.
Keep generated plans out of Git and upload only intended review artifacts.

## decK and APIOps extension point

When OpenAPI specifications generate Gateway configuration, put the
`deck file openapi2kong`, `deck file patch`, `deck file add-plugins`, and
`deck file add-tags` calls in a supplied repository script. Run it before
kongctl so `_deck.files` points at generated decK state.

Use existing repository scripts when available. If a workflow calls a new
script, supply its implementation and document its environment variables.
