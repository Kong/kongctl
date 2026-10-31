# Basic-auth username and password inspection

A basic-auth credential has two levels: a username record attached to a
directory principal, and up to two password records under that username.
The username is unique within the directory. Passwords have generated IDs
and expiration metadata; they have no name. Two slots allow deliberate
rotation, but expired records still occupy a slot in the tested API.

The preview branch supports public metadata reads for both levels. Use a
principal that already has basic-auth credentials:

```sh
kongctl get identity principals --directory-name default
kongctl get identity principal-basic-auths \
  --directory-name default --principal-id PRINCIPAL_UUID --page-size 1
kongctl get identity principal-basic-auth \
  --directory-name default --principal-id PRINCIPAL_UUID \
  --username worker@example.test -o yaml
kongctl get identity principal-basic-auth-passwords \
  --directory-name default --principal-id PRINCIPAL_UUID \
  --username worker@example.test --page-size 1
kongctl get identity principal-basic-auth-password PASSWORD_UUID \
  --directory-name default --principal-id PRINCIPAL_UUID \
  --basic-auth-id BASIC_AUTH_UUID -o text
```

Positional arguments are resource UUIDs. `--username` always means the exact
username, even if that username itself looks like a UUID. Password commands
require exactly one parent `--basic-auth-id` or `--username`. An explicit
directory UUID can replace its name. `list` and explicit `konnect` forms work
as well. Principal display names are not selectors because they can repeat.

JSON/YAML show public management metadata and timestamps, plus the username
or password expiration. Text uses a table for both detail and list output.
Password values are never displayed. These reads neither create credentials
nor change their ownership. The live `.tech` scenario verifies user, agent
and unclassified parents, real page-size-one traversal, wrong-parent errors,
private captures and secret-free outputs/logs.

## Declarative lifecycle work

There are no basic-auth/password declarations or adoption commands yet.
Continue using the principal examples for parent management; adding a
credential through another API does not grant KongCTL ownership over it.

The verified username-record PUT path can preserve its management metadata
without supplying passwords. Shared username-record write helpers now verify
retained metadata and fresh readback; creation is single-attempt. Their
deletion helper refuses a nonempty or incompletely observed password
collection. These helpers do not yet add credential checks to declarative
principal deletion. Native parent deletion can remove access to attached
credentials, so the upcoming declarative lifecycle must enroll those checks.

Password PATCH ignores management-map changes and clears an omitted
expiration. Password PUT requires explicit secret material and expiration.
Metadata-only adoption cannot safely use either request today (Q35).
Separately, preview SDK password write models lack the expiration field;
read support does not establish complete write support. Secret sources,
generation/capture, rotation and slot retirement need explicit contracts
before declarative password management is enabled.

The [API question log](../../../kong-identity-api-questions.md),
[compatibility tracker](../../../contributor/kong-identity-compatibility.md)
and [metadata ledger](../../../contributor/kong-identity-metadata-storage.md)
record these boundaries. Runtime authentication needs a supported integration
context (Q30); metadata inspection does not establish authentication success.
