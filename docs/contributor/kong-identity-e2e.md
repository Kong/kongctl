# Kong Identity E2E directory isolation

The normal Identity suite uses the existing directory named `default`. It
preserves that directory's UUID and never requires a second directory slot.
This matches organizations whose directory quota is one.

Of the 32 Identity scenarios, 25 exercise directory children using this shared
fixture. Six auth-server scenarios do not need a directory. The directory CRUD
scenario requires additional directory capacity and is explicitly opt-in.

## Shared directory fixture

Directory cleanup specifications declare `useDefaultDirectory: true`. The
harness resets `default` before and after the scenario, including after an
interrupted command. Fixture setup updates that existing directory rather than
issuing another directory creation request. An empty organization gets its
first `default` directory automatically.

This is destructive test isolation: running these scenarios clears the shared
directory's users, groups, credentials and other children. Use dedicated test
organizations. The fixture is not a way to run tests alongside customer data.

Tests that require directory ownership explicitly adopt the fixture into their
test namespace. Other child-resource workflows use an external directory
lookup. Groups, principals, identities, keys and username records retain their
own ownership and protection assertions.

Directory-wide delete and bootstrap sections in the Vault and Trusted IdP
lifecycles run only with additional capacity. Their normal paths retain child
CRUD, scoped sync/delete, dump/reload, adoption/protection and saved-plan
dependency guards. Those guards may delete a declared child before rejecting
an unsafe parent deletion, but do not delete `default`. The native org-chart
import scenarios also reuse the shared directory.

## Reset behavior

`make reset-org` now resets Identity before gateway resources, retiring consumer
links before their control planes. It:

1. Enables Vault temporarily so disabled Vault contents are visible.
2. Removes group memberships and available group key records.
3. Removes principal Vault authorizations, API keys, identities, and basic-auth
   passwords and username records before deleting principals. Imported users
   are explicitly deactivated first to release their import ownership.
4. Deletes groups and provider credentials before providers, Trusted IdPs and
   Vault encryption keys.
5. Removes other directories during a full organization reset, then restores
   `default` or bootstraps it if absent. Scenario cleanup resets only `default`.

The retained directory has empty labels and `managed_by`, an empty description,
600-second positive/negative TTLs, all control planes allowed, no explicit
control-plane IDs, and Vault disabled. Existing `default` keeps its UUID.
Inventory or deletion errors stop Identity reset; reset does not use a forced
directory cascade to hide an incomplete child cleanup.

On deployments where group-key inventory returns HTTP 501, destructive reset
asks the API to delete the group after removing memberships and principals.
This does not assert that no keys exist. A rejected group deletion is reported
as a reset failure. Declarative deletion guards continue to fail closed.

## Additional capacity

Set `KONGCTL_E2E_IDENTITY_MULTI_DIRECTORY=1` only when the test organization can
hold another directory alongside `default`. In GitHub Actions, set the
repository variable of the same name. Leave it unset or `0` for the usual
single-directory organizations.

This restores generated temporary directory fixtures, enables the complete
directory CRUD scenario, and runs the directory deletion/bootstrap sections in
the child lifecycles. Cleanup of those generated directories retains the
existing ownership and dependency checks. The replay-enabled scenario registry
is unchanged; no Identity scenario currently has enabled replay coverage.

The two existing dump replays retain their reviewed reset scope, which does
not include Identity. Their isolated replay environment explicitly sets
`KONGCTL_E2E_RESET_IDENTITY=0`; all previously recorded reset calls and
assertions remain mandatory. Live resets and the before/after live recording
reset include Identity. This does not remove or relax existing replay coverage.

## Validation and remaining limits

Harness tests model the API's one-directory quota: an extra POST fails with
HTTP 403, while fixture setup reuses the existing directory with PUT. Additional
tests cover cursor pagination, credential-before-parent cleanup, repeated reset,
missing-directory bootstrap, interrupted scenarios and explicit capacity opt-in.
Planner tests load the actual group, membership and numeric-metadata scenario
inputs against an existing default directory.

Local mocks cannot confirm all deployed API permissions or group/team deletion
behavior. The next live CI run must validate the complete suite. A separate
confirmed failure in the [investigated CI run][quota-run] was the Vault provider
lifecycle exceeding the harness's
60-second subprocess limit; long-running lifecycle drivers now have a
five-minute budget so their own cleanup can finish. Individual HTTP and CLI
request timeouts remain bounded separately.

[quota-run]: https://github.com/Kong/kongctl/actions/runs/42290874671
