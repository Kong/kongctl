# Metering & Billing

The initial Metering & Billing suite manages meters, features, plans, tax
codes, currencies, integration apps, and billing profiles. Commands are
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

All seven collections live under the `metering-billing:` grouping, alongside
`organization:` and `analytics:`. Its children are `meters`, `features`,
`tax_codes`, `currencies`, `apps`, `profiles`, and `plans`. Flat root collections
are not accepted. The grouping does not change ownership or dependency order.

```yaml
_defaults:
  kongctl:
    namespace: billing
metering-billing:
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
kongctl explain metering-billing.meters
kongctl scaffold metering-billing.meters
kongctl plan -f meters.yaml
kongctl apply -f meters.yaml --auto-approve
kongctl dump declarative --resources metering-billing.meters
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
kongctl explain metering-billing.features
kongctl scaffold metering-billing.features
kongctl dump declarative --resources metering-billing.features
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
metering-billing:
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

## Product catalog and billing configuration

The remaining resources share the `mb` namespace. Request files use native
API JSON field names and accept JSON or YAML. Lists follow every API page;
`--page-size` controls each request, not the total number of results.

| Child of `metering-billing` | Command | Lifecycle |
| --- | --- | --- |
| `tax_codes` | `tax-codes` | Create, PUT, delete |
| `currencies` | `currencies` | Create and observe |
| `apps` | `apps` | Install, update, uninstall |
| `profiles` | `profiles` | Create, PUT, delete |
| `plans` | `plans` | Draft CRUD; observe published versions |

Reads accept unambiguous names for profiles/apps, currency codes, tax keys,
and `key@version` for plans. Profile, app, and plan mutations require ULIDs.
Display names are not assumed unique. Specify `id` before renaming an app or
profile, and whenever name or plan-version matching would be ambiguous.
Explicit missing IDs fail; they never create a replacement. A plan ID must
agree with its declared key and optional version.

```sh
kongctl get mb tax-codes --include-deleted -o json
kongctl get mb currencies --type custom --expand-cost-basis -o yaml
kongctl get mb apps catalog
kongctl get mb apps catalog stripe
kongctl create mb tax-code --file tax-code.yaml
kongctl create mb currency --file currency.yaml
kongctl create mb app --file app.yaml
kongctl create mb profile --file profile.yaml
kongctl create mb plan --file plan.yaml
kongctl get mb plans starter@1
kongctl update mb plan PLAN_ULID --file draft-update.yaml
kongctl update mb plan PLAN_ULID --publish
kongctl update mb plan PLAN_ULID --archive
```

The following manifest connects the catalog resources. Add meters/features
from the earlier examples to attach usage to rate cards through
`feature: {id: request-feature}`.

```yaml
_defaults:
  kongctl:
    namespace: billing
metering-billing:
  currencies:
    - ref: credits
      name: Credits
      code: CREDITS
      precision: 0
      decimal_mark: '.'
      thousand_separator: ','
  tax_codes:
    - ref: service-tax
      name: Service Tax
      key: service_tax
      app_mappings: []
  apps:
    - ref: sandbox
      name: Billing Sandbox
      type: sandbox
      create_billing_profile: false
  profiles:
    - ref: profile
      name: Billing Profile
      supplier:
        name: Example Supplier
      workflow: {}
      apps:
        tax: {id: sandbox}
        invoicing: {id: sandbox}
        payment: {id: sandbox}
      default: false
  plans:
    - ref: starter
      name: Starter
      key: starter
      currency: credits
      billing_cadence: P1M
      phases:
        - name: Standard
          key: standard
          rate_cards:
            - name: Service
              key: service
              price: {type: flat, amount: '10'}
              tax_config:
                code: {id: service-tax}
```

Required API fields must be explicit, including `precision: 0`,
`app_mappings: []`, `default: false`, and `create_billing_profile: false`.
Names and keys are never derived from local refs. Native optional fields,
including supplier contacts/addresses, workflow collection/invoicing/payment/
tax settings, rate-card pricing, entitlements, discounts, commitments,
settlement mode, and provider tax mappings, retain their API shapes. Use
`kongctl explain metering-billing.plans` and the other grouped paths for
complete schemas.
Pricing supports free, flat, unit, graduated, and volume variants.

Plan rate-card feature IDs, tax-code IDs, and profile app IDs accept local
refs, `!ref resource#id`, or remote uppercase ULIDs. These refs are typed:
apps cannot be substituted for tax codes. Currency fields accept an existing
currency code or a declared custom-currency ref, including rate-card currency
overrides. Currency refs resolve to codes, never ULIDs. New dependencies run
before consumers, including across namespaces and through saved plans.

### Ownership, deletion, and adoption

Tax codes, apps, profiles, and draft plans use namespace/protection labels.
Existing unmanaged resources require explicit adoption:

```sh
kongctl adopt mb tax-code TAX_CODE_ULID --namespace billing
kongctl adopt mb app APP_ULID --namespace billing
kongctl adopt mb profile PROFILE_ULID --namespace billing
kongctl adopt mb plan DRAFT_PLAN_ULID --namespace billing
```

`--overwrite-namespace` explicitly moves an already managed resource.
Protected resources cannot be adopted or changed. Apply metadata-only
unprotection separately before a public-field change or deletion.

Sync prunes only the selected collections and owned namespaces. It never
prunes custom currencies or published/archived plan versions. Retained plan
versions produce a warning; archive them deliberately. Explicit declarative
plan deletion accepts drafts only. Default profiles cannot be deleted; first
designate another default. The server also enforces subscription, invoice,
and other billing dependencies outside the initial configuration suite.

Deletion checks the whole organization's plans/profiles, including outside
the current sync scope. Dependent deletions or proven reference detachment
run before target deletion. Fresh reads recheck references, ownership,
protection, default-profile status, and draft status before execution.

Tax-code keys are immutable. List normally excludes deleted tax codes;
`--include-deleted` supports inspection. Declarative matching also observes deleted tax codes and rejects reuse of
their keys. A missing explicit ID or an API Gone response fails rather than
implicitly recreating/adopting it. Provider values
in `app_mappings` are provider tax codes, not installed app ULIDs.

### Plan versions and publication

Declarative apply creates a draft only when the key does not exist. It
selects an explicit ID/version, the unique draft for that key, or an
unambiguous existing version. Ambiguity requires an explicit selector.
Repeated apply never creates another version or publishes/archives a plan.
Unchanged published/archived versions can be observed; changed public fields
or protection require an explicit draft. Key, currency, and billing cadence
are immutable. Publication/archive are explicit `update mb plans` flags;
they cannot be combined with a request file. The pinned API assigns effective
times through these actions; effective dates are response data rather than
declarative inputs. No subscription migration is performed.

To create another draft version, deliberately use `create mb plans` with the
API request for the same key, then pin the returned ID/version in your
manifest. Update files use `UpsertPlanRequest`: name and phases are required;
key, currency, and billing cadence belong only in create files.

### Currencies and cost bases

Custom currencies are beta and have no update, delete, labels, or protection
API. Matching an existing custom code observes its configuration without
claiming ownership or changing it. Configuration drift fails with an explicit
unsupported-update error. Built-in fiat currencies are read-only and cannot
be declared as custom currencies. Omission stops observation; it never
schedules deletion.

Cost bases are beta, scheduled imperative operations. They are not mutable
exchange-rate desired state and are never replayed by apply:

```sh
kongctl get mb currencies cost-bases CUSTOM_CURRENCY_ULID
kongctl create mb currencies cost-bases CUSTOM_CURRENCY_ULID --file basis.yaml
```

```yaml
fiat_code: USD
rate: '0.01'
effective_from: '2027-01-01T00:00:00Z'
```

### Integration apps and profiles

App catalog/install/update/uninstall operations are beta. Sandbox, Stripe,
and external invoicing variants use their native contracts. Every install
must explicitly choose `create_billing_profile`. Imperative `true` permits
the API's profile/default side effects. Inspect and explicitly adopt any
side-effect-created profile before declaring it; name matching never silently
adopts or duplicates that profile.

Declarative installs require `create_billing_profile: false` and separate
`profiles` declarations. The install API cannot set labels, so
kongctl immediately updates installed-app metadata. If that step fails, the
error reports the installed ULID: adopt that app before retrying. Creation
rechecks identity to reject stale plans that could install duplicates.
External invoicing declarations must explicitly set both
`enable_draft_sync_hook` and `enable_issuing_sync_hook` because metadata PUT
requires these values.

For declarative Stripe installs, use `api_key_env: STRIPE_API_KEY`. Only the
environment variable's name enters plans; its value is read at execution.
Literal API keys are rejected by the declarative schema. Dumps contain no
credentials. Existing explicitly identified Stripe apps do not need a key to
observe or update public metadata. Credential rotation is imperative:
`update mb apps APP_ULID --file stripe-update.yaml` accepts native
`secret_api_key`; omission preserves the current credential. SDK union
validation and app API failures never echo the credential-bearing request.
Do not use a redacted/masked response value as an update credential.

Profile app associations are immutable. Create another profile to change
them; updates retain native supplier, workflow, tax settings, labels, and
explicit default behavior. Apps installed outside kongctl can be used through
explicit remote ULIDs. Profiles created as installation side effects must be
adopted by ULID before declarative management.

The local catalog integration tests use the real pinned SDK against loopback
HTTP fixtures. The opt-in `metering-billing/suite` E2E scenario covers the
connected suite. It leaves its create/observe-only custom currency in place
and cleans up mutable resources in namespace `mb-suite-e2e`.
