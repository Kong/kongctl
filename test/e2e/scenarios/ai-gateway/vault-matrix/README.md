# AI Gateway vault conformance scenarios

`vault-matrix` exercises live Konnect creation, saved-plan apply, sync
updates, readback, no-change diff, dump/reload, and cleanup. It covers all
seven vault types and nine HashiCorp authentication methods. The inputs
exercise every declared config field in those variants. Expected files
compare observable config values; secret fields are excluded, Config Store
IDs are checked separately, and certificate material is generated at runtime
with the existing runtime TLS scenario helper.

The update changes the AWS role ARN, session name, Secrets Manager endpoint,
and STS endpoint. It also changes caching options across cloud vaults and
HashiCorp methods, and explicitly selects the certificate private key for
writing through `--write-secret`.

An additional AWS vault reproduces #2316 using only a region and role ARN.
Its create plan must omit `base64_decode`, `endpoint_url`, and
`sts_endpoint_url`; readback verifies the server-injected defaults, and a
subsequent diff must report no changes. This adds four CLI commands to the
scenario and uses the existing gateway cleanup.

The scenario refuses to use an existing gateway with its test name. It uses
the `ai-gateway-vault-matrix-e2e` namespace and deletes its gateway at the end.
It contains no organization reset command. To run it without the harness's
automatic organization reset:

```sh
KONGCTL_E2E_RESET=0 make test-e2e-scenarios SCENARIO=ai-gateway/vault-matrix
```

The adjacent `vault-validation` scenario requires no credentials. It checks
`explain`/`scaffold`, unknown and wrong-branch fields, missing Azure location,
invalid HashiCorp authentication methods, and literal private-key rejection
through `diff`, `apply`, and `sync`.

## AppRole API discrepancy

As observed on 2026-09-15, Konnect rejects a HashiCorp AppRole request that
contains the SDK/public specification's `role_id` and `secret_id_file`
fields. Its validation error instead asks for `approle_role_id` or
`approle_secret_id_file`. These prefixed fields are absent from the SDK and
the [public specification][spec].

AppRole is therefore excluded from the live matrix until that discrepancy
is resolved. `vault-validation` verifies that its supported fields load
successfully before an intentional namespace mismatch stops execution;
unit tests cover planning and SDK request construction for AppRole. Discovery
assertions still require all ten HashiCorp authentication branches.

[spec]: https://developer.konghq.com/api/konnect/ai-gateway/v1/#/operations/create-ai-gateway-vault
