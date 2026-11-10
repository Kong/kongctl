# Kong Identity examples

Choose an example for your use case; do not apply this entire directory.
Most examples look up the existing directory named `default` without managing
its settings. `identity` has the alias `id` in CLI commands.

| Example | Contents |
| --- | --- |
| [Bootstrap default][bootstrap] | Missing-directory setup |
| [People and automation][access] | Principals, identities, group, credentials |
| [Service authentication][service] | OAuth issuer, scopes, claims and client |
| [Consumer linking][consumer] | Principal linked to an existing Consumer |
| [Imported workforce][workforce] | Generated keys under external principals |
| [Declarative CSV][csv] | Managed principals and children from CSV |
| [Enable Vault][enable] | Directory setup before planning Vault children |
| [Token Vault][vault] | Workforce trust, encryption and shared upstream token |
| [OAuth integrations][oauth] | User/shared upstream OAuth providers |

These are starting points to edit. API field details belong in
`kongctl explain identity` and the [resource reference][reference]. The
[Learn guide][learn] provides a longer walkthrough, including AI Gateway.

## Principals and access

First check the directory:

```sh
kongctl get id directory -o json
```

If it is missing, apply `bootstrap-default.yaml` separately. For an existing
unowned directory, leave it external for the principal examples.

`principals-and-access.yaml` groups human/agent principals, a custom employee
identity, a basic-auth username, an ordinary group, memberships and generated
principal/group API keys. Basic-auth declarations manage username records;
password material is outside declarative support.

Principal/group key names can be changed without rotating the credential. Set
`name: null` to clear a name; omit it to preserve the current value. These
examples use generated keys; imported-key provisioning is outside the
supported declarative workflow.
Principal and group metadata preserve signed 64-bit integers, including values
above JavaScript's safe-integer range. Use strings for identifiers that are not
intended to be numbers. These behaviors require the updated Identity API.

```sh
kongctl apply -f principals-and-access.yaml \
  --secret-output-dir ./private/initial-keys
```

Review and confirm the proposed changes. Issued material is captured privately;
keep it outside source control. Recreating a deleted key requires a fresh
capture directory. Repeating an unchanged configuration should show no drift:

```sh
kongctl diff --mode apply -f principals-and-access.yaml
```

```sh
kongctl get id principals -o json
```

```sh
kongctl get id group memberships --group-name "FulfillmentWorkers"
```

To remove only this example's managed resources:

```sh
kongctl delete -f principals-and-access.yaml
```

Apply preserves undeclared resources. Sync treats explicitly declared child
collections as authoritative within their ownership scope; omission leaves a
collection alone. The directory remains external during cleanup.

## Service and consumer identities

`service-authentication.yaml` configures an Auth Server, scopes, claims, an
OAuth client and a principal linked to that client. Apply with a private
`--secret-output-dir` for the initial client secret. It configures the issuer;
it does not perform a runtime token request.

`consumer-linking.yaml` links a principal to an existing Gateway Consumer.
Set `KONGCTL_IDENTITY_CONTROL_PLANE_ID` and `KONGCTL_IDENTITY_CONSUMER_ID`
to the corresponding UUIDs before applying. The Gateway entities stay external.

## Workforce imports

The native org-chart import manages people and reporting relationships.
**Execution uses a complete roster and can deactivate previously imported
people omitted from the file.** Start with a preview:

```sh
kongctl import id org-chart --file org-chart.csv \
  --dry-run --report-output-dir ./private/import-reports
```

After reviewing its report, use the same command with `--execute` instead of
`--dry-run`. It does not add KongCTL ownership metadata.

```sh
kongctl get id principals -o json
```

Select the intended principal's UUID; display names are not unique:

```sh
export WORKFORCE_PRINCIPAL_ID='<verified-principal-uuid>'
```

```sh
kongctl apply -f imported-workforce.yaml \
  --secret-output-dir ./private/workforce-keys
```

The principal stays external and importer-controlled. Its generated key has
its own immutable namespace/ref markers. Duplicate matches cause a conflict;
foreign and other-namespace keys stay outside sync/delete scope. Key adoption,
ownership changes and key-level protection are unsupported in this mode.

`principal-csv.yaml` is a different workflow: `!csv` expands `principals.csv`
into ordinary managed declarations, including supported children. It does not
run the org-chart importer or construct its reporting hierarchy. Apply it with
`--secret-output-dir` for the example's generated key. Validate its CSV locally:

```sh
kongctl inspect id principal csv --file principals.csv \
  --metadata-mode merge -o json
```

## Token Vault

Vault keys, providers, static shared credentials and Trusted IdPs inherit
management authority from the directory. Adopt an unowned default first:

```sh
kongctl adopt id directory default --namespace identity-directory
```

```sh
kongctl apply -f enable-token-vault.yaml
```

Use the directory's existing namespace if it is already managed. Enablement
is a separate step because a disabled Vault hides its child collections.

For `token-vault.yaml`, set `IDENTITY_WORKFORCE_ISSUER` to your issuer URL and
`BILLING_SHARED_SECRET` to a test upstream token. Replace
`vault-public-key.pem` with your public encryption key and retain the matching
private key. The file groups trust configuration, an OIDC principal identity,
an encryption key, a static provider and its default shared credential.

```sh
kongctl apply -f token-vault.yaml
```

Secret sources are deferred until execution. Changing a stored secret requires
explicit selection with `--write-secret 'default-token#secret'`; changing the
environment value alone does not trigger rotation. Dump omits secret material.
Trust configuration alone does not demonstrate runtime authentication.

`oauth-integrations.yaml` shows upstream user/shared OAuth providers and a
custom endpoint configuration. Set `VAULT_GITHUB_CLIENT_ID` and
`VAULT_GITHUB_CLIENT_SECRET` from a registered upstream application, and edit
the sample endpoints. These declarations configure provider registration;
shared OAuth consent/enrollment and per-user authorizations are outside this
administrative declarative workflow.

[reference]: ../../../declarative-resource-reference.md
[learn]: https://kong.github.io/kongctl/kong-identity/

[bootstrap]: bootstrap-default.yaml
[access]: principals-and-access.yaml
[service]: service-authentication.yaml
[consumer]: consumer-linking.yaml
[workforce]: imported-workforce.yaml
[csv]: principal-csv.yaml
[enable]: enable-token-vault.yaml
[vault]: token-vault.yaml
[oauth]: oauth-integrations.yaml
