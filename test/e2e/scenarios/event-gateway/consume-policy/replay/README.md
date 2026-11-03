# Event Gateway consume-policy replay

The expanded scenario passed [live recording][record] with all 307 HTTP
exchanges and [three isolated replays][replay]. Recording used the existing
acceptance-3 organization lock and passed both before/after live resets.

The committed cassette is byte-for-byte identical to the validated cassette
from each isolated replay. Requests, responses, input fingerprint, and source
provenance are unchanged from the live candidate; only reviewed parallel
phases were added. The annotation file binds those phases to the original
candidate's SHA-256. Generated UUIDs use the recorder's normalization. The
static key remains the public scenario fixture, not a recording credential.

Coverage retains the existing create/update, root-level declarations, lookup,
dump/plan round-trip, decrypt, skip-record, schema-validation, chained-policy,
and decrypt-fields checks. The mask-fields lifecycle verifies the replacement
strategy and field path. The transcode lifecycle creates an Avro registry,
checks reference-based schema configuration and its resolved registry ID,
then deletes the policy and registry. Both lifecycles check idempotency and
preserve the existing schema-validation parent before final cleanup.

Three bounded phases describe concurrent operations:

- Exchanges 158–160 create the static key and schema-validation parent. The
  static-key create must follow its gateway lookup.
- Exchanges 271–276 look up and delete the transcode policy and registry.
  Each delete follows its lookup; registry deletion also waits for policy
  deletion, preserving the reference relationship.
- Exchanges 297–302 look up and delete the parent policy and static key.
  Repeated gateway reads retain their order and lookups precede deletes.

Planning, policy/registry creation, verification, and all other operations
remain outside those phases. Every method, path, query, and body must still
match exactly once. The replay dependency test also exercises reordered
requests and rejects writes before their required predecessors.

Only ordinary eligible PRs use this cassette. Main, manual all-live runs and
`e2e:force-live` retain live validation. Input or assertion changes require
another live recording and three isolated replays before promotion.

[record]: https://github.com/Kong/kongctl/actions/runs/41658862645
[replay]: https://github.com/Kong/kongctl/actions/runs/41661424390
