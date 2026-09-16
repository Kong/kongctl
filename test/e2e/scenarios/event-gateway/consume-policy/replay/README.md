# Event Gateway consume-policy replay

The expanded scenario passed [live recording][record] with all 211 HTTP
exchanges and [three isolated replays][replay]. Recording used the existing
acceptance-3 organization lock and passed both before/after live resets.

The cassette is the validated candidate from the isolated replay job. Its
requests, responses and recording provenance are unchanged from the live
candidate; only the reviewed parallel phases were added. Generated UUIDs are
normalized by the existing recorder. The backend uses public example hosts
and anonymous authentication. The static key is the public test fixture
already present in the scenario, not a recording credential.

Coverage includes gateway/backend/virtual-cluster creation, consume-policy
lookup by name and ID, updates, no-op planning, dump/plan round-trip,
root-level declarations, decrypt, skip-record, schema-validation and chained
policies. The added decrypt_fields lifecycle verifies the exact configuration
and parent link, no-op replanning, child deletion while preserving the parent,
and final parent/static-key cleanup.

The original strict replay failed during independent prerequisite cleanup.
Two bounded phases describe the independent operations:

- Exchanges 158–160: create the static key and schema-validation parent.
  The static-key create still follows its gateway lookup.
- Exchanges 201–206: look up and delete the parent policy and static key.
  Repeated gateway reads retain their order; resource lookups precede their
  deletes, and ancestor reads cannot cross deletes.

All requests still match method, path, query and body exactly once. Planning,
child creation and deletion, and post-cleanup verification remain outside
these phases. No scenario assertions or matcher rules were relaxed.
The source-candidate SHA-256 in parallel-phases.json binds these annotations
to this recording for any later source_run validation.

The isolated scenario durations were 6.861, 6.971 and 6.855 seconds; wrapper
durations were 7.565, 7.744 and 7.607 seconds. Recording took 29.388 seconds
for the scenario and includes proxy overhead. These are validation timings,
not a controlled live-versus-replay comparison or a workflow savings claim.

Only ordinary eligible PRs use this cassette. Main, manual all-live runs and
e2e:force-live retain live validation. Input/assertion changes invalidate the
cassette fingerprint and require a new recording through the manual refresh
workflow before promotion.

[record]: https://github.com/Kong/kongctl/actions/runs/35115143565
[replay]: https://github.com/Kong/kongctl/actions/runs/35116004092
