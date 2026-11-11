# Event Gateway consume-policy replay

The expanded scenario passed [live recording][record] with 478 HTTP
exchanges and [three isolated replays][replay]. Recording used the existing
acceptance-3 organization lock and passed before/after live resets.

The committed cassette is identical to the validated cassette from each
isolated replay. Requests, responses, input fingerprint, and live source
provenance are unchanged from the recording; only reviewed parallel phases
were added. The annotation file binds those phases to the original
candidate's SHA-256. Generated UUIDs are normalized by the recorder. The
static key remains the public scenario fixture.

Existing policy CRUD, root declarations, references, schema validation,
decrypt-fields, mask-fields, transcode, registry, and cleanup assertions are
retained. Added coverage checks top-level and shared-parent sibling creation,
order-only correction, saved plans and text diff, insertion/deletion,
immutable recreation, moving a child to the top-level chain, repeat no-op
convergence, ordered dump/reload, and reconstruction after deleting the
original gateway.

Eight bounded phases permit independent work within specific commands:

- 169–171: static-key and schema-validation-parent creation. The static-key
  create follows its gateway lookup.
- 293–298: transcode-policy and registry cleanup. Each delete follows its
  lookup; registry deletion also waits for policy deletion.
- 320–325: parent-policy and static-key cleanup. Lookups precede deletes.
- 340–341: a top-level sibling and a child under the schema-validation parent
  are created independently. The next child remains behind the strict barrier.
- 343–346: read-only observations for the two independent sibling chains.
- 380–386: reorder observations and sibling moves. Both chain observations
  precede moves, and the two top-level moves retain their order. The child
  move is independent of those top-level moves.
- 420–426: insertion and independent immutable-policy deletions. Each delete
  retains its detail-read dependency; subsequent replacement creates stay
  behind the strict barrier.
- 467–468: independent top-level and child creation during reconstruction.

Planning, dump, convergence checks, parent creation, sibling sequencing, and
all other exchanges remain strict. Every request method, path, query, and
body still matches exactly once. Matcher tests exercise reversed independent
work and reject writes before required predecessors.

The three isolated runs took 28.238–28.699 seconds in the scenario and
28.754–29.272 seconds in the wrapper. Recording took 112.336 scenario
seconds, including proxy overhead, so it cannot serve as a live performance
baseline. These timings describe validation execution, without a speedup claim.

Only ordinary eligible PRs use this cassette. Main, manual all-live runs,
and `e2e:force-live` retain live validation. Input or assertion changes
require another live recording and three isolated replays before promotion.

[record]:
  https://github.com/Kong/kongctl/actions/runs/42966353556/job/130345970498
[replay]: https://github.com/Kong/kongctl/actions/runs/42967692887
