# Organization user lookup replay

The [live recording][record] captured 7 exchanges under the existing
primary acceptance organization lock with successful before/after resets.
The selected users are pre-registered lookup targets, not created users.
Their email inputs remain required for live runs. Replay supplies reserved
synthetic identities and needs neither credentials nor registered users.

## Coverage and ordering

All seven requests remain strictly ordered. Coverage includes both users by
email, the complete user collection, lookup by recorded email and ID, and
curated text columns. No ordering annotations are required.

Ten consecutive local replays passed. [Isolated validation][replay] runs
all assertions three times with only loopback networking. Candidate data
is sanitized before publication; no real user emails, profile names, or
real-to-synthetic mappings are committed. Full user collections, active
states, memberships, and roles remain represented.

## Measurements

The three isolated replays matched all 7 exchanges. Scenario durations
were 0.733, 0.735, 1.128 seconds; wrapper durations were
1.422, 1.368, 1.706 seconds. Recording timings include proxy overhead
and are not a normal live baseline. These numbers measure execution work,
not workflow wall-clock savings; setup and shared live-shard resets remain.

Only eligible PRs replay this scenario. Main and force-live runs remain
live and still require the registered users. Changes to inputs, commands,
or assertions require a new recording in the same PR. See the repository
replay policy for the identity and environment-input restrictions.

[record]: https://github.com/Kong/kongctl/actions/runs/35463073348
[replay]: https://github.com/Kong/kongctl/actions/runs/35463073348
