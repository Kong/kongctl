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

The refreshed recording passed [isolated validation][replay] three times
with only loopback networking. Candidate data is sanitized before
publication; no real user emails, profile names, or
real-to-synthetic mappings are committed. Full user collections, active
states, memberships, and roles remain represented.

## Measurements

The three isolated replays matched all 7 exchanges. Scenario durations
were 1.414, 1.427, 1.420 seconds; wrapper durations were
2.067, 2.331, 2.124 seconds. Recording timings include proxy overhead
and are not a normal live baseline. These numbers measure execution work,
not workflow wall-clock savings; setup and shared live-shard resets remain.

Only eligible PRs replay this scenario. Main and force-live runs remain
live and still require the registered users. Changes to inputs, commands,
or assertions require a new recording in the same PR. See the repository
replay policy for the identity and environment-input restrictions.

[record]: https://github.com/Kong/kongctl/actions/runs/35553891533
[replay]: https://github.com/Kong/kongctl/actions/runs/35553891533
