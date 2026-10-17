# Metering & Billing

Meters aggregate ingested events in Konnect Metering & Billing. Commands are
scoped under `metering-billing` (alias `mb`), with Konnect implied:

```sh
kongctl get metering-billing meters --page-size 100 -o json
kongctl get konnect metering-billing meter request_count -o yaml
kongctl create metering-billing meter --file create-meter.yaml
kongctl update metering-billing meter request_count --file update-meter.yaml
kongctl get metering-billing meters query request_count --file query.yaml
kongctl delete metering-billing meter request_count --auto-approve
```

Identifiers are the unique API `key` or an uppercase ULID. Keys contain 1–64
lowercase alphanumeric characters separated by single underscores. Display names and
local declarative references are not API identifiers. Lists retrieve every
API page. All commands support text, JSON, and YAML output. Query results are
operational output and never enter declarative desired state.

Request files accept JSON or YAML and contain API fields only. Create requires
`name`, `key`, `aggregation`, and `event_type`:

```yaml
name: Request Count
key: request_count
aggregation: count
event_type: request
dimensions:
  region: $.region
```

For aggregations other than count, supply `value_property` where required by
your event data, for example `$.tokens`. Aggregations are `sum`, `count`,
`unique_count`, `avg`, `min`, `max`, and `latest`. Optional `events_from` is an
RFC3339 timestamp; omission includes historical events.

Update files accept only `name`, `description`, `labels`, and `dimensions`:

```yaml
name: Requests
dimensions: {}
```

Explicit `{}` clears dimensions or user labels; omission leaves them
unmanaged. Imperative updates preserve existing `KONGCTL-*` management labels
and reject protected meters. Remove protection through declarative management
before performing imperative updates or deletion.

Queries accept the API's meter query fields, for example:

```yaml
from: '2026-10-01T00:00:00Z'
to: '2026-10-02T00:00:00Z'
group_by_dimensions:
  - region
```

## Declarative lifecycle

```yaml
_defaults:
  kongctl:
    namespace: billing
meters:
  - ref: local-request-meter
    name: Request Count
    key: request_count
    aggregation: count
    event_type: request
    dimensions:
      region: $.region
    kongctl:
      protected: true
```

```sh
kongctl explain meters
kongctl scaffold meters
kongctl plan -f meters.yaml
kongctl apply -f meters.yaml --auto-approve
kongctl dump declarative --resources meters
```

`ref` identifies the declaration and can be referenced by other declarative
resources. `key` matches remote meters and is unique across namespaces.
Optional `id` selects an existing remote ULID and must agree with its key;
missing explicit IDs fail rather than creating a replacement. A ULID-shaped
`ref` still remains a local reference. Dump exports the key, explicit ID, and
namespace so reapplying an owned dump is stable.

Existing unmanaged meters are never adopted implicitly. Adopt them explicitly
before applying a matching declaration or an unmanaged dump:

```sh
kongctl adopt metering-billing meter request_count --namespace billing
```

Adoption preserves user labels and rejects existing namespace ownership unless
`--overwrite-namespace` is supplied. Protected meters cannot be moved through
adoption. Planning rejects key collisions with unmanaged meters or other
namespaces. Execution checks namespace ownership and protection again, so a
stale saved plan cannot mutate a meter that has changed owners.

Name, description, user labels, and dimensions are mutable. Aggregation,
event type, value property, and events-from changes fail with migration
instructions. Create a meter with a new key, migrate its consumers, then delete
the old meter; kongctl does not automatically replace it. Optional omitted
fields remain unmanaged. Required fields have no derived defaults.

Declarative `labels` accept user labels only. Configure namespace and
protection through `kongctl.namespace` and `kongctl.protected`; supplying
`KONGCTL-*` management keys in `labels` is rejected during validation.

Sync only prunes meters with `KONGCTL-namespace` matching a selected namespace.
An omitted `meters` collection leaves meters untouched; explicit `meters: []`
removes owned, unprotected meters from that scope. A protected meter must be
unprotected in a separate apply before deletion or other field changes.

The published M&B label contract permits uppercase `KONGCTL-namespace` and
`KONGCTL-protected` keys. Label values, including namespace values, must be
1–63 characters, begin and end with an alphanumeric character, and contain
only alphanumerics, hyphens, dots, or underscores. Labels have at most 50
entries including management labels. Lowercase keys starting with `kong`,
`konnect`, `mesh`, `kic`, or `_` are reserved.

The API contract is the
[M&B public specification](https://github.com/Kong/platform-api/blob/eaa6339dfa9c44d3df3a17b8b790d88a630fe159/build/services/konnect/metering-and-billing/v3/public.yaml).
Customers, subscriptions, invoices, credits, and AI Cost Management are outside
this resource's scope.

## Testing

The integration lifecycle uses a local HTTP fixture with the real SDK. Live
coverage is in `test/e2e/scenarios/metering-billing/meters`; enable it with
`KONGCTL_E2E_METERING_BILLING=1` in a test organization with M&B access. It
manages only the `mb-meter-e2e` namespace and clears that scope before and after
its lifecycle. Discovery coverage runs without credentials.

## Features

Features describe static capabilities or connect meters to billable usage.
Their optional `unit_cost` measures internal operational cost in USD; it is
separate from customer-facing plan prices. AI Cost Management and LLM price
overrides are outside this feature's scope.

```sh
kongctl get metering-billing features --page-size 100 -o json
kongctl get konnect mb feature requests -o yaml
kongctl create mb feature --file create-feature.yaml
kongctl update mb feature requests --file feature-cost.yaml
kongctl get mb features cost-query requests --file query.yaml
kongctl delete mb feature requests --auto-approve
kongctl explain features
kongctl scaffold features
kongctl dump declarative --resources features
```

Imperative identifiers are API keys or uppercase ULIDs, with the same
pagination and output conventions as meters. Create files require `name` and
`key`. They may contain `description`, user `labels`, `meter`, and `unit_cost`.
For imperative creation, `meter.id` must be a remote meter ULID. Omit `meter`
to create a static feature. Cost queries are beta, imperative operations and
use the meter query request shape described above.

Declarative creation supports both remote ULIDs and local meter references:

```yaml
_defaults:
  kongctl:
    namespace: billing
meters:
  - ref: local-requests
    name: Request Count
    key: request_count
    aggregation: count
    event_type: request
    dimensions:
      region: $.region
features:
  - ref: request-feature
    name: Requests
    key: requests
    meter:
      id: local-requests
      filters:
        region:
          eq: us
    unit_cost:
      type: manual
      amount: '0.001'
```

`meter.id: !ref local-requests#id` is also supported. Newly created meters
execute before features that reference them, including across namespaces and
through saved plans.
Dimension filters are preserved. For LLM cost lookup, use configuration such
as:

```yaml
unit_cost:
  type: llm
  provider: openai
  model_property: model
  token_type: input
```

Each LLM dimension accepts a static value (`provider`, `model`, `token_type`)
or its corresponding property selector (`provider_property`,
`model_property`, `token_type_property`), mutually exclusively. Resolved LLM
pricing is response data and is excluded from desired state and dump output.

PATCH supports only `unit_cost`. Omission leaves existing cost unchanged;
`unit_cost: null` clears it. This distinction survives plan serialization.
Manual costs require a decimal string `amount`. Required create fields have
no derived defaults.

Feature identity and sync ownership follow meters: keys are globally unique,
optional explicit IDs must match the key, and `ref` remains local. Creation
writes validated uppercase `KONGCTL-*` labels from the `kongctl` block.
Existing unmanaged features are never implicitly adopted. The API cannot
update labels, so feature adoption and namespace moves are unsupported.
Recreating a capability under a new key requires migrating its consumers.

Changes to name, description, meter, filters, user labels, or protection fail
with migration guidance. Omitted optional description and labels remain
unmanaged. Omitting `meter` declares a static feature, so it cannot silently
detach a metered feature. Protection can be set at creation; the current API
cannot clear or change it afterward. Protected features cannot be updated or
deleted by kongctl. Avoid setting protection unless this permanent API
limitation is appropriate for the resource.

An omitted `features` collection leaves it untouched. Explicit `features: []`
prunes only owned, unprotected features in the selected namespace. Meter
deletion inspects all features, including those outside sync scope, and fails
if a retained feature references the meter. When both are deleted in a plan,
feature deletion precedes meter deletion. Execution rechecks ownership and
protection before feature updates and deletion.

The feature integration lifecycle uses the real SDK with a local HTTP
fixture. Live coverage is in
`test/e2e/scenarios/metering-billing/features`, enabled by
`KONGCTL_E2E_METERING_BILLING=1`; it uses namespace `mb-feature-e2e`.
