# Event Gateway consume-policy replay

The unchanged scenario passed [live recording and three isolated replays][run]
with all 150 HTTP exchanges. The recording used the existing acceptance-3
organization lock and passed both before/after live resets.

The cassette is the sanitized candidate from that run, unchanged. All
requests match in strict recorded order; no parallel phase annotations or
request-matching exceptions are needed. Generated UUIDs are normalized by
the existing recorder. The backend uses public example hosts and anonymous
authentication; decrypt policy configuration contains no key material.

Coverage includes gateway/backend/virtual-cluster creation, consume-policy
lookup by name and ID, updates, no-op planning, dump/plan round-trip,
root-level declarations, decrypt, skip-record, schema-validation and chained
policies, and policy deletion. No scenario inputs or assertions were changed.

The isolated scenario durations were 4.217, 4.219 and 4.220 seconds; wrapper
durations were 4.820, 4.826 and 4.791 seconds. Recent reduced-live PR runs had
a 28.61-second live median. This suggests about 24 seconds less scenario
execution work, plus one avoided live scenario reset, but is not a controlled
same-source comparison or a measured end-to-end workflow saving. Recording's
37.642-second scenario duration includes proxy overhead and is not a live
baseline.

Only ordinary eligible PRs use this cassette. Main, manual all-live runs and
`e2e:force-live` retain live validation. Input/assertion changes invalidate the
cassette fingerprint and require re-recording or removal from replay policy.

[run]: https://github.com/Kong/kongctl/actions/runs/35048315678
