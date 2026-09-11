# Turn the working project into a deployment pipeline

Keep kongctl declarative files as the desired state. Preserve the existing
gateway, namespace, certificate and model identities. Add automation around
that project; the local data plane remains on its current host.

Produce runnable workflow files and a short setup/handoff document for the
chosen CI system. For GitHub Actions, use the pattern below and check current
action inputs against their official repositories when authoring YAML.

## Enforce the plan and approval boundary

Use this sequence for each target:

1. Check out one explicit source revision and install one fixed kongctl
   version. Record both. Keep non-secret model IDs in reviewed configuration.
2. Plan against the intended Konnect org, region/base URL and namespace.
   Use a read-capable token for this job and `--mode apply` for additive
   changes. It needs remote state; calling the job "validate" is not an
   offline check.
3. Save `plan.json` and render `kongctl diff --plan plan.json` for review.
   Record target identity, CLI version, source revision and plan hash in
   the run summary and an artifact manifest. The summary must expose the
   actual proposed actions before approval.
4. Upload that plan and its audit metadata with a run/attempt-specific
   artifact name, failing on missing files. Set an explicit retention time.
   Use a dedicated artifact directory and check the upload action's file
   selection. A hidden directory such as `.artifacts/` can be excluded by
   default; a successful plan command does not prove the plan was uploaded.
5. Block the apply job on an enforceable approval rule. For GitHub, configure
   required reviewers on the named deployment environment in repository
   settings. Environment naming, `workflow_dispatch` and comments in YAML
   alone do not create that rule. Document the settings and confirm their
   availability for the repository's plan/visibility.
6. After approval, download the artifact from this run. Check its expected
   revision, target, CLI version and hash, then execute
   `kongctl apply --plan plan.json --auto-approve`. Supply write-capable
   Konnect and deferred provider/caller credentials only to this job.
7. Retain execution outcome with the same audit identity. Verify remote
   configuration, then hand off traffic checks to the actual data plane
   host. A hosted runner's `localhost` is not the presenter's gateway.

`--auto-approve` belongs behind the platform approval boundary. An apply job
must not regenerate the plan with `apply -f`, `sync -f` or another `plan`
after approval. New input requires a new artifact and a new review.

See [GitHub environments][environments] for required-reviewer settings. If
the repository cannot enforce that gate, state the missing prerequisite and
leave deployment disabled until an equivalent gate is available.

## Make artifact checks executable

For example, a plan job can hash the exact plan and expose the digest as a
job output as well as in the reviewer summary:

```sh
kongctl plan --mode apply -f ai-gateway.yaml \
  --require-namespace ai-demo --output-file plan.json
kongctl diff --plan plan.json > plan-diff.txt
plan_digest="$(sha256sum plan.json | cut -d ' ' -f 1)"
printf 'plan_sha256=%s\n' "$plan_digest" >> "$GITHUB_OUTPUT"
```

In the apply job, pass the plan job's output through an environment variable
named `EXPECTED_PLAN_SHA256` and enforce comparison before apply:

```sh
test -n "$EXPECTED_PLAN_SHA256"
printf '%s  plan.json\n' "$EXPECTED_PLAN_SHA256" | sha256sum --check -
kongctl apply --plan plan.json --auto-approve
```

Use a fail-fast shell. Add executable checks for the recorded revision,
target and tool version too; a metadata file nobody checks is only a note.
Keep the expected digest attached to the producer job and approval record,
rather than trusting only a checksum file stored beside a replaceable plan.
Keep this explicit check alongside the pinned action's
[artifact digest validation][artifacts]; mismatch behavior depends on the
action version and settings.

Bind the downloaded artifact to the current run and attempt. Fail when it
is missing, expired or mismatched; do not find the newest similarly named
artifact or silently generate a replacement. Pin action revisions and the
CLI version so review captures automation changes too.

## Bind deployment to one target and control overlap

Use an explicit region/base URL and namespace guard in the plan job. Use
the same endpoint and intended organization credential in the apply job.
Profile names alone do not identify an organization; token ownership and
repository environment settings establish that binding.

Check each job's token against the intended organization. For example:

```sh
organization_id="$(kongctl get organization --base-url "$KONNECT_BASE_URL" \
  -o json --jq '.id' --jq-raw-output)"
test "$organization_id" = "$KONNECT_ORG_ID"
```

`--jq-raw-output` requires `-o json` when using `--jq`; a fresh CLI defaults
to text. Set the format on this query so other commands keep their intended
output, including version checks that parse text.

Serialize the entire plan/review/apply workflow per target with a shared
concurrency group, not just the apply step. Avoid cancelling a running
deployment midway through mutation. Other writers can still change Konnect
outside this workflow: a saved plan is not a remote-state lock.

Choose an explicit plan age bound for the team's review cadence and reject
expired plans before execution. Invalidate superseded revisions according
to the branch/release policy and check that policy at apply time. Re-run
planning and approval after an intervening deployment or relevant manual
change; do not claim kongctl automatically detects all stale remote state.

For a small demo, a protected branch and manually dispatched, serialized
workflow are sufficient starting choices. Document how reruns and old
artifacts are rejected. Keep untrusted pull-request execution away from
deployment credentials; lint/schema inspection can run separately.

## Supply inputs on the correct host

- **Public data plane certificate:** the plan job must resolve `!file` from
  its checkout. Deliberately commit the public `.crt` or provide it as a
  declared CI input. Ignoring the whole `certs/` directory without another
  handoff breaks planning. Track certificate expiry and replacement.
- **Private data plane key:** keep it on the laptop/runtime host with the
  matching public certificate. CI that only configures Konnect does not
  need this key. Do not regenerate it on every runner or deploy.
  Inspect the existing helper's actual key path. If the key lives outside
  the checkout, adapt the helper and runbook to use that location. The
  bundled helper supports `AIGW_DATA_PLANE_KEY=/absolute/runtime/key` and
  `bash data-plane.sh check`. Update an older project helper if it lacks
  this input; documentation alone cannot change its mount behavior.
  Read the [bundled helper](../assets/openai/data-plane.sh) when adapting
  an older copy. On Linux, preserve its `--group-add` using the key file's
  numeric group ID and owner/group read permissions. A host-side key-match
  check does not prove that the container user can read the mounted key.
- **Provider and caller keys:** store them in protected execution secrets.
  `!secret` keeps their bytes out of the saved plan. Prefer deferred
  environment sources in CI; deferred file secrets also require an explicit
  execution-host handoff relative to the saved plan location.
- **Konnect credentials:** use the intended organization's tokens with the
  permissions needed for each job. Verify the planning and applying tokens
  target the same organization; never copy a local browser login into CI.
- **Runtime:** discover endpoints after provisioning, keep the local
  container running and execute the model/access matrix from that host.

## Check the automation before live execution

Run credential-free checks on the generated scripts. Check each shell file
separately; `bash -n first.sh second.sh` checks only `first.sh`:

```sh
for script in data-plane.sh scripts/*.sh; do
  bash -n "$script" || exit 1
done
```

Exercise response assertions with synthetic bodies, including a rejected
provider error, as described in [model access](model-access.md). Verify
that the test job itself exits nonzero when a rejection assertion is broken.
Syntax and workflow lint alone do not establish these behaviors.

## Show the operational result

The finished project should demonstrate a configuration diff, an actual
saved plan, a human approval, execution of that artifact, and the expected
traffic outcomes. Inspect an unchanged follow-up apply plan to show whether
configuration converged. A successful CI apply alone proves neither data
plane connection nor user access to the models.

[environments]:
  https://docs.github.com/en/actions/reference/workflows-and-actions/deployments-and-environments
[artifacts]: https://docs.github.com/en/actions/tutorials/store-and-share-data
