# Rehearsal reset checklist

Use this before each complete first-stage rehearsal. Preserve the working
deployment between the basic setup and CI/CD expansion stages. A fresh
conversation, stopping a container and deleting Konnect resources are
three different operations.

Keep a completed copy of this checklist outside the next agent's project.
Record the date, operator, target organization ID, profile, region,
namespace, gateway ID/name, Docker context, container ID/name, host ports
and project directory. Inventory every leftover rehearsal separately;
resetting the newest project does not clean up an older one.

## 1. Inventory and preserve

- [ ] Stop any active rehearsal agent or CI deployment job from making
      changes during reset.
- [ ] Archive the project's manifest, public certificate, reviewed plans,
      execution reports and sanitized verification evidence. Keep private
      keys and credentials in their existing secure local storage; exclude
      them from shared transcript bundles.
- [ ] Load the project's trusted environment, then verify the organization
      ID using the same explicit profile and region as deployment.
- [ ] List Konnect gateways and Docker containers. Identify only the
      rehearsal resources to remove. Check container endpoints, mounts and
      project labels where available; do not infer ownership from a port.

Useful read-only commands, adapted to the recorded project:

```sh
kongctl get organization --profile "$DEMO_PROFILE" --region "$DEMO_REGION" \
  -o json --jq '.id' --jq-raw-output
kongctl get ai-gateways --profile "$DEMO_PROFILE" --region "$DEMO_REGION"
docker context show
docker ps -a --format 'table {{.ID}}\t{{.Names}}\t{{.Status}}\t{{.Ports}}'
```

Set `DEMO_PROFILE`, `DEMO_REGION` and `DEMO_NAMESPACE` from the inventory.
Use the recorded base URL instead of region flags if that was the target.
If an older project's manifest or certificate is missing, recover its
non-secret inputs from the archive before preparing its deletion plan;
do not invent replacement identities or certificates.

## 2. Prepare and review scoped remote cleanup

Run in each inventoried project's directory, with its original inputs:

```sh
mkdir -p .plans .artifacts
kongctl plan --mode delete -f ai-gateway.yaml --base-dir . \
  --profile "$DEMO_PROFILE" --region "$DEMO_REGION" \
  --require-namespace "$DEMO_NAMESPACE" --output-file .plans/reset.json
kongctl diff --plan .plans/reset.json
```

- [ ] Confirm the plan targets the recorded organization and namespace.
- [ ] Review every deletion, including children removed with the gateway.
- [ ] Obtain authorization for these concrete removals if it has not
      already been provided. A request to start over is not authorization
      to remove unrelated deployments.

## 3. Execute the reviewed reset

- [ ] Stop only the inventoried rehearsal container by its inspected ID:
      `docker stop "$DEMO_CONTAINER_ID"`. The bundled helper uses `--rm`,
      so stopping normally removes it. If it remains stopped, inspect it
      before removing that specific ID. Never use global Docker prune.
- [ ] Execute the same approved deletion plan:

```sh
kongctl delete --plan .plans/reset.json --auto-approve -o json \
  --profile "$DEMO_PROFILE" --region "$DEMO_REGION" \
  --execution-report-file .artifacts/reset-report.json
test -s .artifacts/reset-report.json
jq -e '.summary.status == "success"' .artifacts/reset-report.json
```

- [ ] Read back the gateway inventory and confirm each intended gateway
      is absent. A failed lookup caused by DNS or authorization is not
      deletion evidence. If deletion partially fails, inspect current
      state and prepare a new remaining-work plan before retrying.
- [ ] Confirm the inventoried containers are absent and run the next
      project's `data-plane.sh preflight` with its intended name and ports.
      Investigate any remaining host listeners; do not kill arbitrary owners.

## 4. Prepare the demonstration start

- [ ] Archive the completed reset evidence outside the next project.
- [ ] Use a fresh project directory with the current installed skills,
      intended credentials and the documented prompt. Keep old manifests,
      plans and transcripts out of that agent's workspace.
- [ ] Verify Docker access, selected context, Konnect target and network
      access from the actual agent execution environment.
- [ ] Record **ready** only when remote absence and local preflight both
      pass. Leave partial cleanup marked incomplete.
- [ ] Start a fresh agent session. After successful stage one, keep the
      same project, certificate, deployment and agent session for stage two.

For a resume rehearsal, keep the existing files and resources instead and
state that intent explicitly. For a parallel rehearsal, select a distinct
namespace, gateway/container identity and host ports before planning.
