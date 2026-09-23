# Get a working deployment workflow first

For a small GitHub Actions setup, start with the bundled
[workflow](../assets/github-actions/deploy.yaml). Copy it to
`.github/workflows/deploy-ai-gateway.yaml`. Keep the existing gateway,
namespace, model and local data plane. Get one deployment working before
adding models or authorization, unless those are the user's immediate goal.

The first version plans locally and deploys in CI. The user reviews that
saved plan, commits it with the source, then approves execution by manually
dispatching the workflow with its exact SHA-256. CI verifies the bytes and
organization and applies that file. It never replans during deployment.
This needs one workflow, one Konnect token and no custom audit scripts.

## Check prerequisites once, before building

Check `gh auth status`, the repository's default branch, and existing
secret **names** using `gh secret list`. Reuse an existing authorized CI
token; the example calls its repository secret `KONNECT_PAT`. If the repo
already has `KONNECT_APPLY_PAT`, change the workflow's secret reference.
Do not create two new tokens or require separate read/write credentials
for this small setup. Ask for a missing CI credential with an explicit
scope; do not mint broad tokens merely to finish setup.

Use a working, released local kongctl version and the same fixed version
in the workflow. Edit its version, organization UUID, base URL, namespace
and concurrency group to match the project. The bundled action revisions
and inputs are already selected; adapting this template does not require
researching or writing a new installer.

Repository writers can [manually dispatch][dispatch] this workflow. This
is an explicit operator approval, not independent reviewer enforcement.
Do not configure an environment reviewer gate unless the user requests
that policy or the repository already requires it. [Required reviewers]
have plan/visibility restrictions; check support before building that path.
Keep existing repository controls. If publishing requires a PR, create it
directly and surface the merge prerequisite early. The workflow must exist
on the default branch before its first manual dispatch.

## Prepare one small change and its saved plan

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
mappings. Do not require unrelated secrets to execute an unchanged plan.

## Approve, run, and verify

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
syntax and plan inputs, then exercise the real workflow when authorized.

The minimal path assumes one trusted repository operator and a recently
reviewed plan. It does not enforce a second reviewer, plan expiry, or
detect all remote drift. Add those controls, CI-side planning, multi-target
promotion, separate tokens, or a full access matrix when requested. If an
existing policy needs a stronger gate, honor it instead of using this path.

For a speed-focused task, report time to the first successful run separately
from later feature expansion. Surface missing login, token, source merge
or runner availability immediately; never call authored YAML functional CI.

[dispatch]: https://docs.github.com/en/actions/how-tos/manage-workflow-runs/manually-run-a-workflow
[Required reviewers]: https://docs.github.com/en/actions/reference/workflows-and-actions/deployments-and-environments
