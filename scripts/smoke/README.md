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
Gateway, and Event Gateway. Each run uses unique names and namespaces. It
exercises:

- Saved create/delete plans, diffs, imperative detail/list output, full-width
  text tables, description/label updates, and delete dry runs.
- API versions and documents, a Portal snippet, an AI provider/model/attached
  policy, and an Event Gateway backend/virtual cluster.
- Apply- and sync-mode no-change plans after updates and from exported files.
- Provider replacement and attached-policy renaming, including execution-order
  and concurrency-group assertions before applying the saved sync plan.
- Exact child readbacks and name matching with a UUID-shaped model ref.
- Child deletion with the parent retained, followed by restoration and root
  cleanup. Event Gateway exports also check that children are excluded by
  default.

AI Gateway 2.1 coverage is mandatory whenever AI Gateway is selected. It tests
model costs and selector aliases, a policy condition, and MCP version/cache
settings, including a zero TTL. Quick mode generates the same 2.1 fixtures;
full mode verifies their lifecycle and remote values. These are control-plane
checks; no inference calls or running Kafka brokers are required.

Plans are checked for unexpected actions and changes outside the run's
namespace. Create plans must contain exactly the expected root and child
resource types. A successful zero-change summary is insufficient if the plan
still contains changes. Failed detail reads after deletion must report
not-found; authentication and transport failures do not count as absence.

Use `--resources` to select a comma-separated subset when testing a particular
resource family or an environment with limited product access. Selection is
explicit and recorded through the report's resource list; unavailable services
are not silently skipped.

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
