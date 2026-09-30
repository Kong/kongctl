# Get a working deployment workflow first

Choose the GitHub Actions workflow matching the requested approval model:

- **Manual deployment:** use the [manual-hash workflow][manual-template]
  when the operator should approve an exact locally generated plan by
  dispatching with its SHA-256. This is the minimal default when no other
  model is requested.
- **PR planning and deployment on main:** use the
  [PR-plan workflow][pr-template] when requested. It generates and commits
  the plan on the PR branch, updates the PR description with that saved
  plan's diff, and applies the committed file on a qualifying main push.
  It also includes an independent read-only manual planning job.

Copy the selected asset to `.github/workflows/deploy-ai-gateway.yaml`.
Keep the existing gateway, namespace, model and local data plane. Get one
deployment working before adding models or authorization, unless those
are the user's immediate goal. Neither deployment route replans during
apply. Both use one Konnect token and no custom audit scripts.

## Check prerequisites once, before building

Check `gh auth status`, the repository's default branch, and existing
secret **names** using `gh secret list`. Reuse an existing authorized CI
token; the example calls its repository secret `KONNECT_PAT`. If the repo
already has `KONNECT_APPLY_PAT`, change the workflow's secret reference.
Do not create two new tokens or require separate read/write credentials
for this small setup. Ask for a missing CI credential with an explicit
scope; do not mint broad tokens merely to finish setup.

Fetch the intended remote and check ancestry before preparing a publication
commit. For an existing `origin/main`, use
`git rev-list --left-right --count HEAD...origin/main` and
`git merge-base HEAD origin/main`. Use the actual remote/default branch.
A failed fetch is not an empty repository. Surface unrelated or diverged
history early; do not force-push or discard local work to bypass it.
If authorized recovery uses a temporary worktree, finish by reconciling the
original checkout while preserving local credentials, keys and skills, and
verify its ahead/behind counts against the published branch.

Use a working, released local kongctl version and the same fixed version
in the workflow. Edit its version, organization UUID, region or base URL,
namespace, manifest/certificate paths, default branch and concurrency group
to match the project. The bundled action revisions and inputs are already
selected; adapting this template does not require
researching or writing a new installer. Keep its existing checks; do not
add a bespoke one-update guard just for a description-change demonstration.
If a requested check uses optional plan summary counters, interpret omitted
zero counters as zero, for example `(.summary.secret_writes // 0) == 0`.

Repository writers can [manually dispatch][dispatch] the manual deployment
workflow. This is an explicit operator approval, not independent reviewer
enforcement.
Do not configure an environment reviewer gate unless the user requests
that policy or the repository already requires it. [Required reviewers]
have plan/visibility restrictions; check support before building that path.
Keep existing repository controls. If publishing requires a PR, create it
directly and surface the merge prerequisite early. GitHub documents a
default-branch prerequisite for manual dispatch. For a workflow already
available there, use `gh workflow run ... --ref BRANCH` to test an authorized
branch version and inspect the actual result. An existing workflow was
successfully branch-dispatched when its branch added `workflow_dispatch`;
do not assume every dispatch edit must merge before testing. This does not
establish support for brand-new branch-only workflow files.

## PR planning and apply on main

Adapt the [PR-plan workflow][pr-template] directly rather than rebuilding
its write-back steps. It checks out the same-repository PR head, generates
`ci/plan.json` in apply mode, and renders that exact file with
`kongctl diff --plan`. Planning uses only the Konnect credential. Keep
provider and caller credentials deferred, and retain the deployment plan
guard and organization, region and namespace checks.

This route trusts repository writers with the configured credentials.
Same-repository PRs can change the workflow and run it before review; the
fork guard does not protect secrets from repository writers. When the
project requires narrower access, use a credential restricted to the reads
needed for planning, or move credentials into an environment with required
reviewers. Retain the shared-token default for trusted writers unless a
different policy is requested. See
[GitHub's secret trust boundary][secret-trust].

The planning job alone has `contents: write` and `pull-requests: write`.
It retains the committed plan bytes when regeneration differs only in
`metadata.generated_at`, then computes the diff and hash from that retained
file. Other plan changes are committed as `ci/plan.json`. It verifies the
expected PR head, pushes without force, and polls briefly for the plan
commit to appear in PR metadata.
Unresolved mismatches fail with expected/actual SHA diagnostics. The marked
description section is replaced through a JSON API payload, preserving
surrounding prose and handling embedded Markdown backticks.

Review the generated plan commit and diff through the authorized repository
process. A push to `main` changing `ci/plan.json` applies the committed file;
a config-only main push does not deploy. Direct main plan pushes also
qualify. A push trigger does not prove PR review: preserve existing branch
controls and add stronger gating only when required by the user's policy.
Do not add the manual-hash route's dispatch gate to explicitly requested
automatic deployment.

Bot exclusions prevent recursive planning jobs but do not prevent GitHub
from creating approval-required runs for `GITHUB_TOKEN`-generated
`pull_request` events of types `opened`, `synchronize`, or `reopened`.
The token-generated `push` event itself does not start a push workflow.
Inspect the resulting run state and any existing required-check policy;
repository writers can approve those runs through GitHub. Report any
remaining prerequisite rather than claiming seamless automation. Do not
silently add a privileged token or change repository controls. See
[GitHub's token-trigger behavior][token-triggers].

## Read-only manual planning

The PR-plan workflow's `workflow_dispatch` runs only `check-drift`. It plans
the selected ref into `evidence/plan.json`, renders that saved file, and
uploads artifacts even if the no-op assertion fails. It never commits
`ci/plan.json` or applies changes; retain repository read permissions and
only the Konnect planning credential. This job can also be adapted into a
separate workflow. A planning-only request needs no inference check.

```sh
gh workflow run deploy-ai-gateway.yaml --ref main -f expect_no_changes=true
```

`expect_no_changes` defaults to true. Matching configuration passes;
resource changes or secret writes fail with the diff and plan retained.
Set it to false to inspect proposed changes without the no-op assertion.
Interpret omitted zero counters as zero. Apply-mode no-op establishes
convergence for declared resources, not absence of extra remote resources.

## Manual hash-approved deployment

The following steps apply to the manual-hash workflow. For PR automation,
CI generates the saved plan instead; use the same input and secret-source
constraints.

### Prepare one small change and its saved plan

Start with the current working manifest. For a visible first deployment,
make a user-requested small change such as updating the gateway description.
An unchanged plan is also a valid automation smoke test; label it as such.
Do not add a new model or caller key solely to prove CI works.

Use the project's known profile/target and namespace in these commands:

```sh
mkdir -p ci
kongctl plan --mode apply -f ai-gateway.yaml \
  --profile default --base-url https://us.api.konghq.com \
  --require-namespace ai-demo --output-file ci/plan.json
kongctl diff --plan ci/plan.json
shasum -a 256 ci/plan.json
```

Inspect the plan before committing it: provider and caller credentials
must be deferred `!secret` environment sources, never literal bytes.
Commit `ci/plan.json`, the manifest, the public certificate and the workflow.
Keep `.env` and private keys ignored. Plans can contain operational IDs and
configuration; use a repository appropriate for that content. The public
certificate resolves `!file` for future planning; the private key stays on
the data plane host. Do not regenerate or move it when enabling CI.

The initial workflow maps `OPENAI_API_KEY` for plans that write that secret.
Existing resources with no secret write need no new provider credential.
When later adding caller credentials, add only their required secret/env
mappings and extend the plan guard's environment-source allowlist with those
same deployment credential names. Never allow management credentials such
as `KONGCTL_DEFAULT_KONNECT_PAT` as a secret-write source. The template
allows public literal parts and file sources; environment sources are
limited to `OPENAI_API_KEY` until explicitly extended. Do not require
unrelated secrets to execute an unchanged plan.

### Approve, run, and verify

After reviewing the exact plan and publishing through the repository's
existing source-review process, the authorized operator runs:

```sh
gh workflow run deploy-ai-gateway.yaml --ref main \
  -f plan_sha256=THE_REVIEWED_SHA256
```

Use the actual default branch. An agent may dispatch after the user has
authorized that exact plan; do not treat the hash as approval by itself.
Open the resulting run and confirm the execution report succeeded. The
workflow records source revision, actor, target, plan hash and diff, and
retains its execution report. `-o json` also avoids CLI versions that skip
the report file in text output.

Run one inference check from the existing data plane host. A GitHub-hosted
runner's `localhost` is not the user's gateway. Generate a fresh local
apply-mode plan afterwards to check convergence; do not blindly rerun an
old create plan. For later changes, replace `ci/plan.json`, review the new
diff and hash, commit, then dispatch again.

## Keep the first delivery small

One working run and one short README section are the first milestone.
Use available lint tools; do not install a Go toolchain or build a bespoke
test/audit framework to validate this copied workflow. Check adapted shell
syntax and plan inputs. For PR write-back, check prose preservation and
replacement of an existing marked section. For manual planning, check both
no-op and changed-plan assertions. Exercise the real selected workflow
when authorized and verify event isolation and saved-plan evidence.

The manual-hash path assumes one trusted repository operator and a recently
reviewed plan. It does not enforce a second reviewer, plan expiry, or
detect all remote drift. Add those controls, multi-target
promotion, separate tokens, or a full access matrix when requested. If an
existing policy needs a stronger gate, honor it instead of using this path.

For a speed-focused task, report time to the first successful run separately
from later feature expansion. Surface missing login, token, source merge
or runner availability immediately; never call authored YAML functional CI.

[dispatch]: https://docs.github.com/en/actions/how-tos/manage-workflow-runs/manually-run-a-workflow
[Required reviewers]: https://docs.github.com/en/actions/reference/workflows-and-actions/deployments-and-environments
[token-triggers]: https://docs.github.com/en/actions/how-tos/write-workflows/choose-when-workflows-run/trigger-a-workflow
[secret-trust]: https://docs.github.com/en/actions/reference/security/secure-use
[manual-template]: ../assets/github-actions/deploy.yaml
[pr-template]: ../assets/github-actions/pr-plan-deploy.yaml
