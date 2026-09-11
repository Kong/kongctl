---
name: kongctl-ai-gateway
description: >-
  Build and evolve native Konnect AI Gateway deployments with kongctl
  declarative configuration, from local provider-backed inference to model
  access controls and auditable CI/CD. Use for AI Gateway setup, extending
  an existing deployment, or diagnosing its configuration and traffic.
license: Apache-2.0
metadata:
  product: kongctl
  category: ai-gateway
---

# kongctl AI Gateway

Help the user reach a working AI Gateway through declarative files they can
review and operate again. This skill is self-contained; a kongctl source
checkout or another skill is unnecessary.

## Choose the next outcome

- For a new local OpenAI deployment, read
  [local OpenAI setup](references/local-openai.md). Adapt the bundled
  [manifest](assets/openai/ai-gateway.yaml) and
  [data plane helper](assets/openai/data-plane.sh) into the user's project.
- For more models or caller authorization, read
  [model access](references/model-access.md). Extend the existing gateway
  and preserve its namespace, refs, names, public certificate and model
  aliases unless the requested change requires otherwise.
- For a deployment pipeline, read [CI/CD](references/cicd.md). Combine it
  with model access only when that expansion is requested.
- For a failure, use the diagnostic table in the local setup reference;
  inspect the relevant installed schema before changing configuration.

Use the user's chosen provider, CI system and hosting model. The local
OpenAI example is a starting point, not a prerequisite for other use cases.
If account details or credentials are unavailable, still prepare concrete
files, local checks and the commands needed to finish. Ask only for choices
that block the next action; reuse existing authorization. A files-only task
does not authorize Konnect mutations, Docker startup or paid inference.

## Discover the installed contract

Start with `kongctl version --full` and targeted schema discovery:

```sh
kongctl explain ai_gateways
kongctl explain ai_gateways.model_providers
kongctl explain ai_gateways.models
kongctl explain ai_gateways.data_plane_certificates
```

Narrow large results to the field being authored, for example
`kongctl explain ai_gateways.models.access`. Use `kongctl scaffold --help`
when a different resource shape needs a starter. Read the schema's required
fields, union branches and relationship annotations; do not infer required
names from `ref`. If the installed CLI lacks these subjects, report that
capability gap and arrange a compatible CLI before attempting deployment.

Native AI Gateway uses `ai_gateways`, nested `model_providers`, `models`,
`data_plane_certificates`, `auth_strategies` and `consumers`. A Gateway
control plane with services, routes and AI plugins is a different workflow.
Use that architecture when the user explicitly requests it or is already
operating it. Native AI Gateway configuration does not require decK.

The bundled example follows kongctl 1.15.1 native declarative support and
the AI Gateway 2.0 quickstarts. Verify the installed schema when adapting it;
an example version is not a claim of compatibility with every environment.
For other providers or policies, use targeted discovery and the current
[AI Gateway documentation](https://developer.konghq.com/ai-gateway/).

## Keep the deployment contract explicit

- Set `_defaults.kongctl.namespace` to the project's ownership scope. Use
  explicit `ref`, `name` and required API fields. Preserve identity during
  expansion so the plan updates the existing deployment.
- Keep file inputs within the project. `!file` resolves relative to its
  containing YAML file, constrained by `--base-dir`. Ordinary `!env` values
  resolve during loading; deferred `!secret` values resolve during apply.
- Use `!secret` for write-only provider and caller credentials. Public
  `Bearer ` decoration belongs in `parts`; secret bytes do not. Do not print
  environment values or put private keys, credentials or response dumps in
  committed configuration.
- Separate the public model alias used in request routing from the upstream
  model ID. Give aliases unique values within the gateway. Select upstream
  models available to the user's provider account before planning.
- Prefer an explicit `--mode apply` plan for additive setup and expansion.
  `plan` defaults to sync mode, which can propose deletions. A saved plan's
  execution verb must match its mode. Use sync/delete only for the intended
  ownership scope and show their proposed removals.
- Plan and diff require remote state and Konnect authentication. Schema
  inspection and local file checks can run without them. Do not invent an
  offline validation flag, substitute a fake API endpoint, or claim generic
  YAML lint proves native schema, references or deployment success.

Once the target and execution are authorized, produce and inspect a saved
plan, then execute that same file:

```sh
mkdir -p .plans
kongctl plan --mode apply -f ai-gateway.yaml \
  --require-namespace ai-demo --output-file .plans/apply.json
kongctl diff --plan .plans/apply.json
kongctl apply --plan .plans/apply.json
```

Adapt the namespace and use the same explicit profile and region/base URL
for planning and execution. Changes to inputs or target require a new plan
and review. Secrets referenced in that plan must be supplied to execution.

## Finish with evidence and a usable handoff

Deliver the project files and concise instructions to configure required
inputs, plan/apply, run the data plane, verify traffic and clean up the
project's resources. Keep local data plane operations separate from remote
Konnect operations so the user can run each on its intended host.

Report what was actually checked:

- **Authored:** files and execution instructions exist.
- **Locally checked:** name the schema, file or script checks performed.
- **Provisioned/connected:** resources exist and the data plane connects.
- **Functional:** a matching request returns a real model completion;
  access changes also pass the intended allowed and denied cases.

A plan, container startup, node listing or HTTP status alone does not prove
inference. Record remaining steps when live execution is unavailable. For a
repeatable demo, inspect an unchanged follow-up plan and rehearse scoped
cleanup. Determinism here concerns deployment inputs and approved actions;
model replies and remote state can vary.
