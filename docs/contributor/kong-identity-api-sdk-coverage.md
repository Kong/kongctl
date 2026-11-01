# Kong Identity API and SDK operation coverage

Assessed November 1, 2026, against `ki-triage` and US `.tech`.

This matrix distinguishes an SDK symbol from an implemented workflow and from
deployed evidence. It covers every directory/auth-server operation in the
selected public preview, plus the Credential Provider template catalog:
**112 operations, all with generated SDK methods; 97 methods called by the
current consumer and 15 deliberately unused methods.** The latter include one
unfinished principal Vault revocation workflow, alternate write routes and
explicitly separate follow-ons. A method reference alone proves no lifecycle.

Use public SDK preview `2f51ede0670e1e6a27b130e8d6e659f73f7f4bf0`, consumed as
`v0.79.1-0.20261101101317-2f51ede0670e`. It preserves the Identity operation
surface and all seventeen cursor corrections from 37580cf8cdf5. Two additional
Registry GETs supply the cross-service retirement dependency below; they are
outside the 112-operation Identity count. An AST comparison preserves all
61,406 pre-existing public declarations, signatures and struct wire tags.
This is a branch dependency, not an SDK release or a guarantee about future
versions. Canonical KongCTL patch provenance remains
`5680da0f5da0a6a26a0a6e8f758da5d888a64553`.

Sources: [pinned public SDK][sdk], [canonical SDK patches][patches],
[API compatibility/removal conditions][compatibility],
[ownership metadata ledger][metadata] and [API-only questions][questions].
The per-family issue scopes are #1399/#1400–#1404 and
#1413/#1414–#1419/#2656–#2665.

## Surfaces and evidence boundaries

| Surface | This implementation |
| --- | --- |
| Auth-server administration, Sliver `/v1/auth-servers` | Server/client/scope/claim configuration and explicit secret lifecycle. OAuth client-credentials issuance/rotation also has runtime tests. |
| Directory administration, `/v2/directories`, plus template catalog | Configuration, ownership, direct memberships, public credential observation and explicit enrollment/import workflows within their documented boundaries. |
| Identity runtime authentication and lookup | Not desired-state resources. Supported runtime mTLS context is missing (Q30); administrative reads do not prove credential authentication, enrollment or revocation. This is separate from working auth-server client-credentials tests. |
| Registry Agent incoming references, `/v1/agents` | Two read-only inventory methods guard Identity principal/directory retirement. No Registry declarations, ownership stamping or mutation authority are added. |

Still-gated realm/configuration, directory-key/quota/org-chart-root operations,
JWK/Connected-App follow-ons and runtime authentication/lookup are outside this
selected preview inventory. Their absence is different from an SDK-only method
listed below. Product-list inclusion or specification publication alone does
not establish deployed availability.

`used` means a production helper/state/executor/command calls that SDK method.
It does not mean every HTTP method is a separately exposed imperative command.
Configuration resources use the declarative planner/executor; secret/import/
enrollment operations follow the documented explicit workflows. `SDK only`
methods are dispositioned individually after the tables.

HTTP numbers below are response statuses actually captured in the successful
32-scenario US `.tech` run with SDK 37580cf8cdf5. Six operations without saved
HTTP trace lines have named live scenario assertions (`A`) instead.
Principal-group projection evidence combines direct native requests with the
successful focused CLI scenario using the updated SDK. Negative statuses
exercise error
or isolation behavior; they do not negate a successful status on the same row.
`—` means no live claim is made here for that particular operation.

Every one of the 97 consumer-call operations has positive live trace or named
scenario evidence. One unused alternate directory PATCH also has captured
success; the other SDK-only operations are not claimed as verified workflows.

The native/CLI evidence is configuration and administrative behavior only.
It does not establish token continuity for user-enrolled credentials or any
Q30-dependent runtime behavior. Shared OAuth pending creation is not consent.

## Per-operation matrix

### Registry incoming-reference dependency

These two operations extend the selected preview to 114 tracked operations:
99 consumer methods and 15 deliberately unused Identity methods. Their source
is the Agent Registry v1 specification at platform-api 79a35a9a281b. Only the
two reads and their transitive component dependencies have gating removed.
The bundled inputs, transform tests and provenance live under the SDK's
identity-preview directory. Existing paths/components remain unchanged.

| HTTP method and route | SDK method | Consumer | Live evidence |
| --- | --- | --- | --- |
| `GET /v1/agents` | `RegistryAgentReferences.ListAgents` | used | 200, native three-Agent pagination and Registry guard scenario |
| `GET /v1/agents/{agentId}/principals` | `RegistryAgentReferences.ListAgentPrincipals` | used | 200, native linked/empty/terminal pages and Registry guard scenario |

The focused US .tech scenario passes with 13 CLI invocations, 45 Registry GETs
and ten outcomes. It verifies late saved-plan links, scoped sync, dangling
directory targets, explicit unlinking, preserved updates and configured hosts.
CLI logs remain in a removed private workspace; only count/host/method receipts
are saved. Source-defined operation servers require an explicit per-operation
URL override; the consumer supplies its configured regional origin for both
reads. Missing permissions or unavailable Registry APIs block target retirement.
All 33 Identity live scenarios subsequently pass together on this pin in
633.61s, including the Registry scenario in 12.16s. Repository gates pass.
Evidence is registry-guards-all/20261101-042019 and its verification receipts
under the local cache; default settings are restored and inventories are empty.

### AuthServer

| HTTP method and route | SDK method | Consumer | Live evidence |
| --- | --- | --- | --- |
| `DELETE /v1/auth-servers/{authServerId}` | `DeleteAuthServer` | used | 204 |
| `GET /v1/auth-servers/{authServerId}` | `GetAuthServer` | used | 200 |
| `PATCH /v1/auth-servers/{authServerId}` | `UpdateAuthServer` | used | A |
| `GET /v1/auth-servers` | `ListAuthServers` | used | 200 |
| `POST /v1/auth-servers` | `CreateAuthServer` | used | 201 |

### AuthServerClaims

| HTTP method and route | SDK method | Consumer | Live evidence |
| --- | --- | --- | --- |
| `DELETE /v1/auth-servers/{authServerId}/claims/{claimId}` | `DeleteAuthServerClaim` | used | A |
| `GET /v1/auth-servers/{authServerId}/claims/{claimId}` | `GetAuthServerClaim` | used | A |
| `PATCH /v1/auth-servers/{authServerId}/claims/{claimId}` | `UpdateAuthServerClaim` | used | A |
| `GET /v1/auth-servers/{authServerId}/claims` | `ListAuthServerClaims` | used | 200 |
| `POST /v1/auth-servers/{authServerId}/claims` | `CreateAuthServerClaim` | used | A |

### AuthServerClients

| HTTP method and route | SDK method | Consumer | Live evidence |
| --- | --- | --- | --- |
| `DELETE /v1/auth-servers/{authServerId}/clients/{clientId}/secrets/{clientSecretId}` | `DeleteClientSecret` | used | 204 |
| `GET /v1/auth-servers/{authServerId}/clients/{clientId}/secrets/{clientSecretId}` | `GetClientSecret` | used | 200, 404 |
| `PATCH /v1/auth-servers/{authServerId}/clients/{clientId}/secrets/{clientSecretId}` | `UpdateClientSecret` | used | 200, 404 |
| `GET /v1/auth-servers/{authServerId}/clients/{clientId}/secrets` | `ListClientSecrets` | used | 200 |
| `POST /v1/auth-servers/{authServerId}/clients/{clientId}/secrets` | `CreateClientSecret` | used | 201, 403 |
| `POST /v1/auth-servers/{authServerId}/clients/{clientId}/test-claim` | `TestClaimForClient` | SDK only | — |
| `DELETE /v1/auth-servers/{authServerId}/clients/{clientId}/tokens/{tokenId}` | `DeleteAuthServerClientToken` | SDK only | — |
| `GET /v1/auth-servers/{authServerId}/clients/{clientId}/tokens/{tokenId}` | `GetAuthServerClientToken` | SDK only | — |
| `DELETE /v1/auth-servers/{authServerId}/clients/{clientId}/tokens` | `DeleteAllAuthServerClientTokens` | SDK only | — |
| `GET /v1/auth-servers/{authServerId}/clients/{clientId}/tokens` | `ListAuthServerClientTokens` | SDK only | — |
| `DELETE /v1/auth-servers/{authServerId}/clients/{clientId}` | `DeleteAuthServerClient` | used | 204 |
| `GET /v1/auth-servers/{authServerId}/clients/{clientId}` | `GetAuthServerClient` | used | 200 |
| `PATCH /v1/auth-servers/{authServerId}/clients/{clientId}` | `UpdateAuthServerClient` | used | 200 |
| `PUT /v1/auth-servers/{authServerId}/clients/{clientId}` | `ReplaceAuthServerClient` | SDK only | — |
| `GET /v1/auth-servers/{authServerId}/clients` | `ListAuthServerClients` | used | 200 |
| `POST /v1/auth-servers/{authServerId}/clients` | `CreateAuthServerClient` | used | 201 |

### AuthServerScopes

| HTTP method and route | SDK method | Consumer | Live evidence |
| --- | --- | --- | --- |
| `DELETE /v1/auth-servers/{authServerId}/scopes/{scopeId}` | `DeleteAuthServerScope` | used | 204 |
| `GET /v1/auth-servers/{authServerId}/scopes/{scopeId}` | `GetAuthServerScope` | used | 200 |
| `PATCH /v1/auth-servers/{authServerId}/scopes/{scopeId}` | `UpdateAuthServerScope` | used | A |
| `GET /v1/auth-servers/{authServerId}/scopes` | `ListAuthServerScopes` | used | 200 |
| `POST /v1/auth-servers/{authServerId}/scopes` | `CreateAuthServerScope` | used | 201 |

### KongIdentityDirectories

| HTTP method and route | SDK method | Consumer | Live evidence |
| --- | --- | --- | --- |
| `DELETE /v2/directories/{directoryId}` | `DeleteDirectory` | used | 204 |
| `GET /v2/directories/{directoryId}` | `GetDirectory` | used | 200, 404 |
| `PATCH /v2/directories/{directoryId}` | `UpdateDirectory` | SDK only | 200 |
| `PUT /v2/directories/{directoryId}` | `ReplaceDirectory` | used | 200 |
| `GET /v2/directories` | `ListKongDirectories` | used | 200 |
| `POST /v2/directories` | `CreateDirectory` | used | 201 |

### KongPrincipal

| HTTP method and route | SDK method | Consumer | Live evidence |
| --- | --- | --- | --- |
| `POST /v2/directories/{directoryId}/org-chart/import` | `ImportOrgChart` | used | 200, 400 |
| `GET /v2/directories/{directoryId}/principals/{principalId}/groups` | `ListPrincipalGroups` | used | 200, 404 |
| `DELETE /v2/directories/{directoryId}/principals/{principalId}` | `DeletePrincipal` | used | 204 |
| `GET /v2/directories/{directoryId}/principals/{principalId}` | `GetPrincipal` | used | 200, 404 |
| `PATCH /v2/directories/{directoryId}/principals/{principalId}` | `UpdatePrincipal` | SDK only | — |
| `PUT /v2/directories/{directoryId}/principals/{principalId}` | `ReplacePrincipal` | used | 200 |
| `GET /v2/directories/{directoryId}/principals` | `ListPrincipals` | used | 200 |
| `POST /v2/directories/{directoryId}/principals` | `CreatePrincipal` | used | 201 |

### KongPrincipalCredential

| HTTP method and route | SDK method | Consumer | Live evidence |
| --- | --- | --- | --- |
| `DELETE /v2/directories/{directoryId}/principals/{principalId}/api-keys/{apiKeyId}` | `DeletePrincipalKey` | used | 204 |
| `GET /v2/directories/{directoryId}/principals/{principalId}/api-keys/{apiKeyId}` | `GetIdentityKey` | used | 200, 404 |
| `PATCH /v2/directories/{directoryId}/principals/{principalId}/api-keys/{apiKeyId}` | `UpdatePrincipalKey` | used | 200 |
| `GET /v2/directories/{directoryId}/principals/{principalId}/api-keys` | `ListPrincipalKeys` | used | 200 |
| `POST /v2/directories/{directoryId}/principals/{principalId}/api-keys` | `CreatePrincipalKey` | used | 201 |
| `DELETE /v2/directories/{directoryId}/principals/{principalId}/basic-auths/{basicAuthId}/passwords/{passwordId}` | `DeletePassword` | used | 204 |
| `GET /v2/directories/{directoryId}/principals/{principalId}/basic-auths/{basicAuthId}/passwords/{passwordId}` | `GetPassword` | used | 200, 404 |
| `PATCH /v2/directories/{directoryId}/principals/{principalId}/basic-auths/{basicAuthId}/passwords/{passwordId}` | `UpdatePassword` | used | 200 |
| `PUT /v2/directories/{directoryId}/principals/{principalId}/basic-auths/{basicAuthId}/passwords/{passwordId}` | `ReplacePassword` | used | 200 |
| `DELETE /v2/directories/{directoryId}/principals/{principalId}/basic-auths/{basicAuthId}/passwords` | `DeleteAllPasswords` | used | 204 |
| `GET /v2/directories/{directoryId}/principals/{principalId}/basic-auths/{basicAuthId}/passwords` | `ListPasswords` | used | 200 |
| `POST /v2/directories/{directoryId}/principals/{principalId}/basic-auths/{basicAuthId}/passwords` | `CreatePassword` | used | 201, 403 |
| `DELETE /v2/directories/{directoryId}/principals/{principalId}/basic-auths/{basicAuthId}` | `DeletePrincipalBasicAuth` | used | 204 |
| `GET /v2/directories/{directoryId}/principals/{principalId}/basic-auths/{basicAuthId}` | `GetPrincipalBasicAuth` | used | 200, 404 |
| `PATCH /v2/directories/{directoryId}/principals/{principalId}/basic-auths/{basicAuthId}` | `UpdatePrincipalBasicAuth` | SDK only | — |
| `PUT /v2/directories/{directoryId}/principals/{principalId}/basic-auths/{basicAuthId}` | `ReplacePrincipalBasicAuth` | used | 200 |
| `GET /v2/directories/{directoryId}/principals/{principalId}/basic-auths` | `ListPrincipalBasicAuths` | used | 200, 404 |
| `POST /v2/directories/{directoryId}/principals/{principalId}/basic-auths` | `CreatePrincipalBasicAuth` | used | 201 |

### KongPrincipalIdentity

| HTTP method and route | SDK method | Consumer | Live evidence |
| --- | --- | --- | --- |
| `DELETE /v2/directories/{directoryId}/principals/{principalId}/identities/{identityId}` | `DeleteIdentity` | used | 204 |
| `GET /v2/directories/{directoryId}/principals/{principalId}/identities/{identityId}` | `GetIdentity` | used | 200 |
| `PATCH /v2/directories/{directoryId}/principals/{principalId}/identities/{identityId}` | `UpdateIdentity` | SDK only | — |
| `PUT /v2/directories/{directoryId}/principals/{principalId}/identities/{identityId}` | `ReplaceIdentity` | used | 200 |
| `GET /v2/directories/{directoryId}/principals/{principalId}/identities` | `ListIdentities` | used | 200, 404 |
| `POST /v2/directories/{directoryId}/principals/{principalId}/identities` | `CreateIdentity` | used | 201 |

### PrincipalGroup

| HTTP method and route | SDK method | Consumer | Live evidence |
| --- | --- | --- | --- |
| `DELETE /v2/directories/{directoryId}/groups/{groupId}/api-keys/{apiKeyId}` | `DeleteGroupKey` | used | 204 |
| `GET /v2/directories/{directoryId}/groups/{groupId}/api-keys/{apiKeyId}` | `GetGroupKey` | used | 200, 404 |
| `PATCH /v2/directories/{directoryId}/groups/{groupId}/api-keys/{apiKeyId}` | `UpdateGroupKey` | used | 200 |
| `GET /v2/directories/{directoryId}/groups/{groupId}/api-keys` | `ListGroupKeys` | used | 200 |
| `POST /v2/directories/{directoryId}/groups/{groupId}/api-keys` | `CreateGroupKey` | used | 201 |
| `POST /v2/directories/{directoryId}/groups/{groupId}/membership/batch` | `BatchCreateGroupMembershipEntries` | SDK only | — |
| `DELETE /v2/directories/{directoryId}/groups/{groupId}/membership/{groupMembershipId}` | `DeleteGroupMembershipEntry` | used | 204 |
| `GET /v2/directories/{directoryId}/groups/{groupId}/membership/{groupMembershipId}` | `GetGroupMembershipEntry` | used | 200 |
| `GET /v2/directories/{directoryId}/groups/{groupId}/membership` | `ListGroupMembership` | used | 200 |
| `POST /v2/directories/{directoryId}/groups/{groupId}/membership` | `CreateGroupMembershipEntry` | used | 201 |
| `DELETE /v2/directories/{directoryId}/groups/{groupId}` | `DeleteGroup` | used | 204 |
| `GET /v2/directories/{directoryId}/groups/{groupId}` | `GetGroup` | used | 200 |
| `PUT /v2/directories/{directoryId}/groups/{groupId}` | `ReplaceGroup` | used | 200 |
| `GET /v2/directories/{directoryId}/groups` | `ListGroups` | used | 200 |
| `POST /v2/directories/{directoryId}/groups` | `CreateGroup` | used | 201 |

### TokenVaultCredentials

| HTTP method and route | SDK method | Consumer | Live evidence |
| --- | --- | --- | --- |
| `DELETE /v2/directories/{directoryId}/principals/{principalId}/vault-credentials/{vaultCredentialId}` | `DeleteVaultCredential` | SDK only | — |
| `GET /v2/directories/{directoryId}/principals/{principalId}/vault-credentials/{vaultCredentialId}` | `GetVaultCredential` | SDK only | — |
| `GET /v2/directories/{directoryId}/principals/{principalId}/vault-credentials` | `ListVaultCredentials` | used | 200 |

### TokenVaultKeys

| HTTP method and route | SDK method | Consumer | Live evidence |
| --- | --- | --- | --- |
| `DELETE /v2/directories/{directoryId}/vault/keys/{keyId}` | `DeleteVaultKey` | used | 204 |
| `GET /v2/directories/{directoryId}/vault/keys/{keyId}` | `GetVaultKey` | used | 200, 404 |
| `PATCH /v2/directories/{directoryId}/vault/keys/{keyId}` | `UpdateVaultKey` | used | 200 |
| `GET /v2/directories/{directoryId}/vault/keys` | `ListVaultKeys` | used | 200, 403 |
| `POST /v2/directories/{directoryId}/vault/keys` | `CreateVaultKey` | used | 201 |

### TokenVaultProviders

| HTTP method and route | SDK method | Consumer | Live evidence |
| --- | --- | --- | --- |
| `GET /v2/credential-provider-templates/{credentialProviderTemplateName}` | `GetCredentialProviderTemplate` | used | 200, 404 |
| `GET /v2/credential-provider-templates` | `ListCredentialProviderTemplates` | used | 200 |
| `DELETE /v2/directories/{directoryId}/vault/providers/{providerId}/credentials/{credentialId}` | `DeleteVaultProviderCredential` | used | 204 |
| `GET /v2/directories/{directoryId}/vault/providers/{providerId}/credentials/{credentialId}` | `GetVaultProviderCredential` | used | 200 |
| `PUT /v2/directories/{directoryId}/vault/providers/{providerId}/credentials/{credentialId}` | `UpdateVaultProviderCredential` | used | 200 |
| `GET /v2/directories/{directoryId}/vault/providers/{providerId}/credentials` | `ListVaultProviderCredentials` | used | 200, 404 |
| `POST /v2/directories/{directoryId}/vault/providers/{providerId}/credentials` | `CreateVaultProviderCredential` | used | 201 |
| `DELETE /v2/directories/{directoryId}/vault/providers/{providerId}` | `DeleteVaultProvider` | used | 204 |
| `GET /v2/directories/{directoryId}/vault/providers/{providerId}` | `GetVaultProvider` | used | 200, 404 |
| `PATCH /v2/directories/{directoryId}/vault/providers/{providerId}` | `UpdateVaultProvider` | used | 200 |
| `PUT /v2/directories/{directoryId}/vault/providers/{providerId}` | `ReplaceVaultProvider` | SDK only | — |
| `GET /v2/directories/{directoryId}/vault/providers` | `ListVaultProviders` | used | 200, 403 |
| `POST /v2/directories/{directoryId}/vault/providers` | `CreateVaultProvider` | used | 201 |

### TrustedIDPs

| HTTP method and route | SDK method | Consumer | Live evidence |
| --- | --- | --- | --- |
| `GET /v2/directories/{directoryId}/trusted-idps/{trustedIdpId}/principals` | `ListTrustedIdpPrincipals` | used | 200 |
| `DELETE /v2/directories/{directoryId}/trusted-idps/{trustedIdpId}` | `DeleteTrustedIdp` | used | 204 |
| `GET /v2/directories/{directoryId}/trusted-idps/{trustedIdpId}` | `GetTrustedIdp` | used | 200, 404 |
| `PATCH /v2/directories/{directoryId}/trusted-idps/{trustedIdpId}` | `PatchTrustedIdp` | SDK only | — |
| `PUT /v2/directories/{directoryId}/trusted-idps/{trustedIdpId}` | `UpdateTrustedIdp` | used | 200 |
| `GET /v2/directories/{directoryId}/trusted-idps` | `ListTrustedIdps` | used | 200, 403, 404 |
| `POST /v2/directories/{directoryId}/trusted-idps` | `CreateTrustedIdp` | used | 201 |

## Live assertion evidence without saved HTTP lines

All referenced scenarios passed in the combined US `.tech` run. These rows are
not inferred from compilation or the existence of a test file: the successful
commands, applied-change counts and subsequent live resource reads establish
the actual behavior. The local per-operation evidence JSON retains trace
scenario names; no credential or employee values are copied into this matrix.

| Operation ID | Successful scenario commands/assertions |
| --- | --- |
| `createAuthServerClaim` | `auth-server-declarative: apply-saved` |
| `getAuthServerClaim` | `auth-server-children: get-claim-by-name/id` |
| `updateAuthServerClaim` | `auth-server-declarative: apply-saved-update/verify-claim-clear` |
| `deleteAuthServerClaim` | `auth-server-declarative: claims-pruned/delete-all-in-order` |
| `updateAuthServerScope` | `auth-server-declarative: verify-scope-flags/verify-scope-omission` |
| `updateAuthServer` | `auth-servers: apply-saved-update/verify-all-fields/verify-omissions` |

## SDK-only dispositions

These operations exist in the selected SDK but are not automatically promised
as supported workflows. Alternate methods do not require adding redundant
commands when the chosen method already supports the scoped configuration.

| Operation ID | Disposition |
| --- | --- |
| `testClaimForClient` | Operational claim evaluation is separate from the requested claim configuration lifecycle. |
| `deleteAuthServerClientToken` | Client token/session administration is an explicit follow-on in #1399/#1133. |
| `getAuthServerClientToken` | Client token/session administration is an explicit follow-on in #1399/#1133. |
| `deleteAllAuthServerClientTokens` | Client token/session administration is an explicit follow-on in #1399/#1133. |
| `listAuthServerClientTokens` | Client token/session administration is an explicit follow-on in #1399/#1133. |
| `replaceAuthServerClient` | Normal updates use PATCH; PUT requires secret material and is unnecessary for configuration updates. |
| `batchCreateGroupMembershipEntries` | Optional batch API is not used; explicit individual entry operations cover the supported direct membership lifecycle. |
| `update-principal-basic-auth` | The supported username lifecycle uses hydrated PUT instead of this alternate PATCH route. |
| `updateIdentity` | The supported identity lifecycle uses hydrated PUT instead of this alternate PATCH route. |
| `deleteVaultCredential` | Pending #2663: a populated user-enrolled authorization and supported runtime fixture are required (Q30). No revocation command is exposed. |
| `getVaultCredential` | Public principal credential detail selects from the complete collection; this separate detail operation is not required by that projection. |
| `updatePrincipal` | Hydrated PUT preserves ownership; the observed PATCH ownership gap is Q11. |
| `patchTrustedIdp` | Full PUT provides the supported clearing/preservation behavior; nullable PATCH behavior is Q38. |
| `replaceVaultProvider` | The supported provider lifecycle uses PATCH; replacement/mode changes retain the Q39/Q40 boundary. |
| `updateDirectory` | The supported directory lifecycle uses hydrated PUT instead of this alternate PATCH route. |

## Reproducibility and remaining verification

The operation list comes from the pinned SDK `openapi.yaml`; each operation ID
is matched to the actual generated service method's `OperationID` constant.
Production call sites are inspected separately. SDK package/model/internal/pkg
tests and root/internal race tests pass. All seventeen cursor-wire cases pass
without skips; the previous preview fails only the newly added principal-group
case. Patch hashes, installed test bytes and idempotent application are
verified.

Captured evidence is under the local cache
`/home/rspurgeon/.cache/kong-identity-public-sdk/`: the
`identity-operation-coverage.json` index,
`principal-groups-all/20261101-025558`,
`principal-group-native-contracts.json`, `principal-groups-first`,
`principal-groups-sdk-wire.log`, `principal-groups-sdk-tests.log` and
`principal-groups-sdk-race.log`. Private CSV rows, generated material, employee
outputs and import reports are not part of this matrix or its evidence index.

The remaining populated principal Vault lookup/revocation evidence is absent;
do not count empty collections as that proof. API constraints, temporary client
handling and removal triggers stay in the compatibility tracker and Q1–Q53.
Issues remain open/In Progress until merge to main. No PR or SDK release is
created by this report.

[sdk]: https://github.com/Kong/sdk-konnect-go/tree/2f51ede0670e1e6a27b130e8d6e659f73f7f4bf0
[patches]: ../../tools/sdk-patches/README.md
[compatibility]: kong-identity-compatibility.md
[metadata]: kong-identity-metadata-storage.md
[questions]: ../kong-identity-api-questions.md
