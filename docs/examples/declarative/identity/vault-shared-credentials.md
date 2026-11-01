# Manage static shared Vault credentials

A static shared credential stores a pre-issued value used to access an upstream
service, such as a billing API token. The Credential Provider identifies that
service; a directory public encryption key protects stored credential data.
These credentials do not authenticate principals to Kong Identity.

This example manages static shared values. OAuth consent, individual user
authorizations and runtime token retrieval are separate workflows.

## Prepare the default directory

The [example](vault-shared-credentials.yaml) explicitly manages `default` in
namespace `identity-directory`. Credentials and providers have no `labels` or
`managed_by`; their mutation authority and protection come from that directory.
An external directory lookup grants no credential write authority.

For an existing unmanaged default, adopt it and enable Vault first:

```sh
./kongctl adopt identity directory default --namespace identity-directory
./kongctl apply -f docs/examples/declarative/identity/vault-directory.yaml
```

If default is missing, the example can create the directory, public key,
provider and credentials together. An already managed default must use its
existing namespace. A protected directory needs a separately reviewed
unprotection before updates or retirement. Creation under a protected directory
follows the existing inherited-child policy.

Replace the sample public key with your own and retain its matching private
key. Provide `BILLING_SHARED_SECRET` through your execution environment and
create the private source file named in the example. Sources are deferred:
planning does not read their values. Both source files and the saved plan must
remain within the plan directory's permitted file boundary. See the
[write-only secret guide](../secrets/README.md) for source handling.

## Create and inspect

```sh
./kongctl plan --mode apply \
  -f docs/examples/declarative/identity/vault-shared-credentials.yaml \
  --output-file billing-plan.json
./kongctl diff --plan billing-plan.json
./kongctl apply --plan billing-plan.json --auto-approve
./kongctl get identity vault-provider-credentials \
  --directory-name default --provider-name billing-service -o yaml
```

Planning orders directory, key and provider creation before credential writes.
Saved execution rechecks directory identity, namespace, ownership, protection,
Vault enablement, provider name/type, routing and encryption-key availability.
Missing sources fail preflight before mutations. Writes make one attempt;
inspect current state and generate another plan after an uncertain outcome.

The API exposes one default slot and distinct nonnegative priority slots per
provider. A selector requires a priority; lower priorities are considered first.
The example's literal `true` selector matches every caller, so that preferred
credential is considered ahead of the default. Other CEL context and runtime
behavior require validation against your service's supported integration.

Matching uses the provider and default/priority slot. Selectors can repeat at
different priorities and are not identifiers. Ref is local and is not stamped
onto the API record. Duplicate remote slots stop reconciliation. Ordinary
reapplication produces no secret writes or changes.

## Replace material or change a selector

Changing a source's value does not implicitly replace a stored credential.
Select an explicit write, then execute the saved plan:

```sh
./kongctl plan --mode apply \
  -f docs/examples/declarative/identity/vault-shared-credentials.yaml \
  --write-secret 'preferred-token#secret' --output-file billing-rotation.json
./kongctl apply --plan billing-rotation.json --auto-approve
```

Secret-only replacement sends the current routing pair so PUT cannot reset a
rule to the default slot. Changing a selector within the same priority also
requires this explicit selection: the API requires material on every static
PUT. No automatic comparison of unreadable values is possible. Public readback
verifies identity/routing/status, not the stored secret or runtime usability.

Changing priority means a different declared resource. Apply creates that slot
and preserves the old one. Explicitly scoped sync creates the replacement before
retiring omitted slots. Moving to/from default has the same identity behavior.

## Scope, export and retirement

Omitting `shared_credentials` preserves existing credentials. An explicit
collection scopes sync to the declared static provider; `shared_credentials:
[]` retires its shared credentials. Provider and directory configuration alone
do not authorize credential retirement. User/OAuth credentials are not adopted
or pruned through this declaration.

Grouped declarations use `identity.vault_provider_credentials` with a
`provider` selector referencing a declared static/shared provider. The same
managed directory must be declared. Grouped empty lists have no provider
selector; use a nested empty collection for provider-specific retirement.

```sh
./kongctl dump declarative --resources identity.directories \
  --include-child-resources --output-file billing-dump.yaml
./kongctl plan -f billing-dump.yaml
```

Dump includes public static routing configuration and no secret values or
source bindings. It can reconcile existing slots without sources. Recreating a
missing slot requires adding a deferred source; dump is not a credential backup.

`delete -f vault-shared-credentials.yaml` selects the directory and provider as
well as its declared credentials and key. Credential deletions precede provider
retirement, followed by fresh actual-empty checks. Undeclared or newly attached
credentials block the parent. To keep the directory/provider, sync a manifest
that retains them and explicitly empties their credential collection instead.
Deletion checks cannot close the final concurrent attachment/API-delete race.

Q45/Q46 in the [engineering question log](../../../kong-identity-api-questions.md)
track formal priority/default uniqueness and material-preserving update
contracts. The current subset follows direct US `.tech` observations.
