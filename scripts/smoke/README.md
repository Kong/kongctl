# Installed-binary smoke test

Run from a checkout so `scripts/smoke-test.sh` can find its Python companion
files. Dependencies are Bash, Python 3 (standard library only), and an installed
or locally built `kongctl` binary.

```sh
scripts/smoke-test.sh --binary ./kongctl --profile smoke --quick
scripts/smoke-test.sh --binary ./kongctl --profile smoke --yes
scripts/smoke-test.sh --binary ./kongctl --profile smoke --yes \
  --resources ai_gateway
```

Quick mode checks version metadata, YAML tag preservation in `patch file`,
extended provider/vault schemas, root and API child scaffolds, fixture
generation, and authenticated list output. It also checks `KONGCTL_CONFIG_FILE`,
flag precedence, and missing-file errors using temporary configuration files.
It does not mutate remote state.

The full suite requires access to APIs, Portals, Gateway control planes, AI
Gateway, Event Gateway, Kong Identity directories, and Auth Servers. Each run
uses unique names and namespaces. It exercises:

- Saved create/delete plans, diffs, imperative detail/list output, full-width
  text tables, description/label updates, and delete dry runs.
- API versions and documents, a Portal snippet, an AI provider/model/attached
  policy, and an Event Gateway backend/virtual cluster.
- Adoption previews and saved declarative output for a Portal and AI Gateway
  models. A filtered model preview targets a different namespace, then bulk
  adoption retains the original namespace. Apply/sync plans verify that
  previews leave ownership unchanged and adoption preserves configuration;
  apply plans also check the exported declarations. These checks re-adopt the
  suite's managed fixtures using `--overwrite-namespace`; they don't cover
  bootstrapping unmanaged resources or every adoption family.
- Apply- and sync-mode no-change plans after updates and from exported files.
- Namespace `!env` resolution: create fixtures inherit a scalar-tagged
  `_defaults.kongctl.namespace`; update fixtures use a mapping-tagged
  resource-level `kongctl.namespace` without a file default. Both resolve to
  the same isolated namespace, checked by `--require-namespace` and plan
  assertions throughout the lifecycle. Saved create, sync, and delete plans
  execute with a different environment value to verify namespace ownership
  stays fixed at planning time.
- Provider replacement and attached-policy renaming, including execution-order
  and concurrency-group assertions before applying the saved sync plan.
- Exact child readbacks and name matching with a UUID-shaped model ref.
- Child deletion with the parent retained, followed by restoration and root
  cleanup. Event Gateway exports also check that children are excluded by
  default.

Kong Identity coverage (1.27.0) adds two selectable families, enabled by
default: `identity_principal` and `identity_auth_server`.

- A user principal and custom employee identity under an existing directory,
  referenced through `_external`. Discovery prefers `default`, otherwise the
  first directory by name. Updates check typed metadata, principal ownership,
  and identity values. Principal lookup uses a unique display name followed by
  UUID selection. Existing directory settings and ownership are preserved.
- If the inventory is empty, full mode bootstraps `default` after confirmation,
  using `allow_all_control_planes: true` for organizations with a one-directory
  quota. The saved plan must contain exactly one protected directory create.
  The new directory belongs to `kctl-smoke-identity-bootstrap` and is retained
  permanently, including after failures. Principal cleanup references it as an
  external parent and never deletes it. Inventory errors stop the run rather
  than triggering bootstrap. Quick mode remains read-only and skips principal
  listing when no directory exists.
- An Auth Server with a scope and a claim linked through `!ref`. Updates
  verify audience, scope description, claim value, and the resolved scope UUID.
- Saved create and delete plans check exact resource counts and dependency
  order, including separate execution groups. Both families check namespace
  isolation, apply/sync idempotency, and targeted cleanup. Auth Servers also
  check dump round trips. Shared-directory dumps are excluded because they
  can contain unrelated principals and directory settings.

```sh
scripts/smoke-test.sh --binary ./kongctl --profile smoke --yes \
  --resources identity_principal,identity_auth_server
```

Identity checks administer isolated resources; they do not exercise runtime
OAuth flows, external IdPs, generated credentials, or Token Vault enrollment.
Cleanup deletes only the smoke principal and its identity, then verifies the
selected directory remains accessible.

AI Gateway coverage requires runtime 2.3 whenever AI Gateway is selected.
It retains model costs and selector aliases, a policy condition, and MCP
version/cache settings, including a zero TTL. Runtime 2.3 coverage adds:

- MCP method and tool rate limits, server conditional ACLs, default tool ACLs,
  and individual tool ACLs, with updates and exact remote readbacks.
- A vector datastore referenced by the model using `!ref`, with list/detail
  reads, a display-name update, and dump round trips. The model uses a semantic
  balancer with OpenAI embeddings, pgvector search settings, and two targets
  with semantic descriptions, including a catch-all. Provider replacement
  updates both targets and the embeddings provider. Plan assertions require
  datastore creation before the model and deletion after the model, including
  separate execution groups when groups are present.

Quick mode generates the same 2.3 fixtures; full mode verifies their lifecycle
and remote values. These are control-plane checks; no inference calls, live
vector database, or running Kafka brokers are required. Token Vault enrollment
and outbound TLS are outside this suite's current coverage.

Namespace coverage requires the support from
[PR #2705](https://github.com/Kong/kongctl/pull/2705) (`bcd17a79`) on both
namespace locations. The runner exports `KONGCTL_SMOKE_NAMESPACE_<RESOURCE>`
(uppercase, such as `KONGCTL_SMOKE_NAMESPACE_AI_GATEWAY`) with each namespace,
overriding any inherited value for selected resources. To reuse an artifact
fixture manually, set its variable to the namespace recorded in `report.json`.
Quick mode generates these fixtures; full mode checks namespace resolution.

Plans are checked for unexpected actions and changes outside the run's
namespace. Create plans must contain exactly the expected root and child
resource types. A successful zero-change summary is insufficient if the plan
still contains changes. Failed detail reads after deletion must report
not-found; authentication and transport failures do not count as absence.

Use `--resources` to select a comma-separated subset when testing a particular
resource family or an environment with limited product access. Selection is
explicit and recorded through the report's resource list; unavailable services
are not silently skipped.

## AI Gateway live validation status

The original live run against `7971eda1` passed creation, sync updates, and
child readbacks, then stopped at `plan-zero-ai_gateway-updated-apply` because
of [datastore drift #2711](https://github.com/Kong/kongctl/issues/2711).
The fix landed in [PR #2713](https://github.com/Kong/kongctl/pull/2713) and is
included in this branch. The zero-change assertion remains a hard failure.

A live rerun with the fixed binary is still required. AI Gateway dump round
trips, provider replacement, pruning, restoration, and planned root deletion
were not reached in the original run; only the offline suite has verified
those later checks so far. Failure cleanup passed in the original live run.

## Observed regression on `4f1e759a`

The Event Gateway prune check detects a planner issue: backend and virtual
cluster deletions occupy the same execution group. Backend deletion cascades
to its virtual cluster, so the subsequent virtual-cluster delete can return
404. The smoke test requires virtual-cluster deletion to finish first and
fails before applying an unsafe prune plan. This is a hard failure, with
normal targeted cleanup; the script does not suppress it or retry the plan.

To exercise the other families independently while investigating this issue:

```sh
scripts/smoke-test.sh --binary ./kongctl --yes \
  --resources api,portal,control_plane,ai_gateway
```

Failures stop the suite and trigger targeted cleanup in reverse root order.
`--keep-on-failure` retains resources for investigation. Artifacts include
commands, stdout/stderr, fixtures, saved plans, dumps, `report.json`, and
`summary.txt`. Use `--artifacts-dir` to choose their parent directory.

The suite does not retry mutations automatically: a partially applied saved
plan must be investigated and replanned rather than blindly replayed.

Run `make test-smoke` to lint the shell scripts and exercise the offline fake
CLI suite. Its model covers child reconciliation and intentionally injected
failures; it cannot establish compatibility with a live Konnect environment.
