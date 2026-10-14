# Metering & Billing meters

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
kongctl dump --resources meters -o yaml
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
