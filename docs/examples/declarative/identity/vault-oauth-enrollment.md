# Enroll a shared OAuth credential

A shared OAuth Credential Provider describes an application registered with an
upstream service, such as GitHub. Its `client_id` and `client_secret` belong to
that application. A provider credential holds the authorization obtained when
an operator completes consent for an upstream account. The resulting access
and refresh tokens differ from the application's client secret.

Provider configuration and encryption keys are declarative. Consent is an
explicit workflow: creating a credential produces a pending record and a
private enrollment URL. Saving that URL does not complete consent. Individual
user authorizations remain a separate runtime workflow.

## Prepare the default directory and provider

The [provider example](vault-oauth-enrollment.yaml) uses managed `default` in
namespace `identity-directory`. For an existing unmanaged directory, adopt it
and enable Vault before planning hidden Vault collections:

```sh
./kongctl adopt identity directory default --namespace identity-directory
./kongctl apply -f docs/examples/declarative/identity/vault-directory.yaml
```

If default is missing, the example can create it with the key and provider.
For an already managed directory, use its existing namespace and protection
policy. Replace the sample public key with your own and retain the matching
private key. Register a real upstream OAuth application and provide its ID and
secret through the named environment variables:

```sh
./kongctl plan --mode apply \
  -f docs/examples/declarative/identity/vault-oauth-enrollment.yaml \
  --output-file shared-provider-plan.json
./kongctl apply --plan shared-provider-plan.json --auto-approve
```

The provider's application secret uses normal deferred, explicitly selected
writes. It is independent of the enrollment URL and subsequent account tokens.

## Create the pending shared credential

Choose a new capture filename in an existing private directory:

```sh
./kongctl create identity vault provider credential \
  --directory-name default --provider-name github-shared \
  --secret-output-file /path/to/private/github-enrollment.json
```

Normal output contains public metadata, including `id`, `credential_type` and
`status: pending`. The new 0600 JSON file contains `directory_id`,
`provider_id`, `credential_id`, capture `status` and `enrollment_url`.
File status `captured` means the URL was saved; credential status `pending`
means consent is incomplete. The command never overwrites an existing file
and does not follow the URL.

Use the enrollment URL with your supported gateway/consent integration. This
administrative feature does not perform that integration or fabricate runtime
mTLS context. The API accepts pending credential creation before a public key
is registered; that does not establish that consent or runtime token retrieval
can complete without the required encryption/runtime configuration.

Omitting routing flags selects the provider's default slot. To create a routed
credential, supply both a CEL selector and a distinct nonnegative priority:

```sh
./kongctl create identity vault provider credential \
  --directory-name default --provider-name github-shared \
  --selector true --priority 0 \
  --secret-output-file /path/to/private/github-priority-enrollment.json
```

The literal `true` matches every caller ahead of default. Other runtime CEL
contexts require verification in your integration. Duplicate default/priority
slots fail before creation. These commands accept shared OAuth providers;
[static shared credentials](vault-shared-credentials.md) use declarations.

## Inspect or deliberately issue another URL

Ordinary reads use the collection and never mint enrollment URLs:

```sh
./kongctl get identity vault provider credentials \
  --directory-name default --provider-name github-shared -o yaml
./kongctl get identity vault provider credential CREDENTIAL_UUID \
  --directory-name default --provider-name github-shared -o json
```

To issue another URL for an existing credential, explicitly create an enrollment
capture with a new output filename:

```sh
./kongctl create identity vault provider enrollment CREDENTIAL_UUID \
  --directory-name default --provider-name github-shared \
  --secret-output-file /path/to/private/github-new-enrollment.json
```

This invokes the capability-producing detail GET once. Even that GET disables
automatic retries. The specification permits pending or active credentials;
live verification covers pending records. URL expiry, reuse and invalidation
rules are tracked in Q47. A new URL does not prove older URLs remain usable.

Request/capture failures retain the reserved file for inspection and never
automatically delete the credential. Inspect public metadata before repeating
an uncertain create. If a record exists but capture was lost, deliberate URL
issuance by its UUID avoids creating a second credential.

## Retire a shared record

```sh
./kongctl delete identity vault provider credential CREDENTIAL_UUID \
  --directory-name default --provider-name github-shared
```

Deletion uses normal confirmation or explicit `--auto-approve`. It proves
directory/provider membership through complete collections, rejects user
authorizations and verifies the shared UUID disappears. It does not mint a URL
or prove upstream token invalidation. The explicit deletion command also
supports shared static records.

Like other explicit secret commands, these operations use API authorization and
do not enforce declarative namespace/protection or stamp ownership metadata.
OAuth credential records and URLs are not exported as declarations or pruned
through static credential sync. Provider/directory retirement still requires
their credential collections to be actually empty.

OAuth routing PUT is not exposed here: its effect on an active authorization
needs the runtime fixture in Q30/Q46. These and the URL lifecycle question Q47
are in the [engineering question log](../../../kong-identity-api-questions.md).
