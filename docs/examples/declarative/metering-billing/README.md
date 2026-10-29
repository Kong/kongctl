# Metering & Billing Examples

These examples use the `metering-billing:` declarative grouping. All seven
collections are represented: meters, features, currencies, tax codes, apps,
billing profiles, and plans. See the
[resource guide](../../../metering-billing.md) for supported fields, ownership,
adoption, and lifecycle restrictions.

Run commands from this directory with a profile authenticated to a Konnect
organization with Metering & Billing enabled. Apps and custom currencies use
beta APIs. Each walkthrough has its own namespace and resource keys so the
examples can coexist. Keys and currency codes are organization-wide identities;
adjust the example identities if they already exist outside your ownership.

## Files and coverage

- [Usage](usage/config.yaml): meter, filtered feature, manual unit cost, and
  static feature.
- [Catalog](catalog/config.yaml): connected seven-resource suite, trial phase,
  pricing, and entitlements.
- [Integrations](integrations/config.yaml): Stripe credential indirection and
  external invoicing hooks.
- Federation: [platform](federated/platform.yaml) and
  [product](federated/product.yaml) resources with references across files and
  namespaces.

Pass the specific configuration files shown below. Cleanup files select empty
collections for deletion and must be loaded separately from desired resources.

## Start with usage

`usage/config.yaml` declares a count meter for `api_request` events, a feature
filtered to the `us` region, and a static support feature without a meter.
The meter extracts its `region` dimension from the event's `region` property.
The metered feature uses `!ref request-meter#id`, so kongctl creates the meter
before the feature. The manual unit cost is a USD cost per unit, independent
of a plan's selling price or billing currency.

```sh
kongctl diff -f usage/config.yaml
kongctl apply -f usage/config.yaml
kongctl plan -f usage/config.yaml
kongctl get mb meters
kongctl get mb features
```

The repeated plan should contain no changes. These manifests configure usage
aggregation and billing metadata; they do not ingest events or create customers
or subscriptions.

To inspect the generated schema or create another starter manifest:

```sh
kongctl explain metering-billing.features.meter
kongctl scaffold metering-billing.meters
```

## Connect a product catalog

`catalog/config.yaml` connects all seven collections in one namespace:

- A request meter supplies the billable feature.
- A custom credits currency supplies the plan's currency through
  `!ref catalog-credits#code`. Currency references resolve to codes, not IDs.
- A tax code supplies the standard phase's tax configuration.
- A sandbox app supplies the billing profile's tax, invoicing, and payment
  integrations. The app explicitly disables implicit profile creation.
- A separate non-default profile specifies the supplier and workflow.
- A plan has a 14-day free trial, followed by a monthly standard phase with
  per-request pricing and a flat platform fee. The metered entitlement uses
  a monthly usage period.

The profile and plan are independent configuration resources. A subscription
would select them when linking a customer to this configuration; creating that
subscription is outside this example's scope.

```sh
kongctl plan -f catalog/config.yaml --output-file catalog-plan.json
kongctl diff --plan catalog-plan.json
kongctl apply --plan catalog-plan.json
kongctl plan -f catalog/config.yaml
kongctl get mb plans kongctl_docs_catalog_plan
```

Required values such as `precision`, `app_mappings`, `default`, and
`create_billing_profile` are explicit, including zero, empty, or false values.
Decimal amounts are strings. `ref` is a local declaration identifier, while
`key`, `code`, and remote ULIDs select API resources. Both plain local refs and
tagged refs appear in this example.

The plan remains a draft. Applying it never publishes it or creates another
version. To publish deliberately, inspect the plan and use its returned ULID:

```sh
kongctl update mb plans PLAN_ULID --publish
```

Published versions cannot be edited as drafts or deleted by cleanup. Leave the
example unpublished to exercise the draft cleanup below. Profile app
associations and plan currency/cadence are immutable; create another resource
when those associations must change.

## Configure provider integrations

`integrations/config.yaml` declares two apps without creating profiles as an
installation side effect. The Stripe app stores only the name
`KONGCTL_DOCS_STRIPE_API_KEY` in the manifest and saved plan. Make that
environment variable available with a valid Stripe credential when applying the
plan; planning does not need its value. Do not replace `api_key_env` with
`!env`, `!secret`, or a literal credential: this field names a variable to read
at execution, rather than holding the credential itself.

```sh
kongctl plan -f integrations/config.yaml --output-file integrations-plan.json
kongctl diff --plan integrations-plan.json
kongctl apply --plan integrations-plan.json
```

The external invoicing app explicitly disables draft hooks and enables issuing
hooks. Both hook flags must be specified for managed metadata updates. Provider
credential rotation remains an imperative operation; see the
[integration lifecycle][integrations].
The sandbox catalog walkthrough needs no Stripe credential.

## Share resources across namespaces

The federated example separates platform and product ownership:

- `federated/platform.yaml` owns a shared meter, currency, tax code, sandbox
  app, and billing profile in `mb-platform-example`.
- `federated/product.yaml` owns a feature and draft plan in
  `mb-product-example`. The feature references the platform's meter; the plan
  references its currency and tax code.

Load both files together so references can resolve, including on the first
apply. Each file's `_defaults.kongctl.namespace` applies to its own resources.
References establish dependency order without changing namespace ownership.

```sh
kongctl plan -f federated/platform.yaml -f federated/product.yaml
kongctl apply -f federated/platform.yaml -f federated/product.yaml
kongctl plan -f federated/platform.yaml -f federated/product.yaml
```

For independent team runs against existing shared resources, replace a local
reference with an explicit remote ULID or an existing currency code. External
name-lookup declarations are not supported for these resources. Deleting the
shared meter or tax code while another namespace still references it fails.

## Dump and cleanup

Dump uses the same grouped collection paths. Inspect exported ownership before
applying a dump: resources created outside kongctl need explicit adoption.

```sh
kongctl dump declarative \
  --resources=metering-billing.meters,metering-billing.features \
  --filter-name='Example*' --output-file usage-dump.yaml
```

Preview cleanup in sync mode, then sync the empty collections. Each cleanup
file targets only its walkthrough's namespace and listed collections:

```sh
kongctl plan --mode sync -f usage/cleanup.yaml
kongctl sync -f usage/cleanup.yaml
kongctl plan --mode sync -f catalog/cleanup.yaml
kongctl sync -f catalog/cleanup.yaml
kongctl sync -f integrations/cleanup.yaml
kongctl plan --mode sync \
  -f federated/product-cleanup.yaml -f federated/platform-cleanup.yaml
kongctl sync \
  -f federated/product-cleanup.yaml -f federated/platform-cleanup.yaml
```

Omitted collections remain untouched. The explicit empty lists select owned,
unprotected resources for pruning; dependent resources are deleted first.
Unprotect resources in a separate apply if you added protection. A profile
made default must have another profile designated as default before deletion.
Subscriptions or other remote dependencies can also prevent deletion.

Custom currencies have no delete API. `KONGCTL_DOCS_CREDITS` and
`KONGCTL_FED_CREDITS` remain after cleanup. Rerunning the examples
observes them without claiming ownership. Published or archived plans are also
retained by sync. Cost-basis scheduling, plan publication, and provider secret
rotation are explicit operations rather than replayable desired state.

[integrations]: ../../../metering-billing.md#integration-apps-and-profiles
