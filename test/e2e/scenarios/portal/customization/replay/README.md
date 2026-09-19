# Portal customization replay

The scenario passed [live recording and three isolated replays][record]
with 71 HTTP exchanges. Recording used the existing acceptance-3 lock and
successful before/after organization resets. The only scenario change sets
diagnostic logging from `debug` to the replay-supported `info` level.
Inputs, overlays, commands, and assertions are otherwise unchanged.

## Coverage and ordering

Coverage includes portal creation, initial customization and no-op planning,
updates to theme, layout, renderer settings, robots and menus, and another
no-op plan. The scenario then seeds all three menu lists through the API,
clears them declaratively, verifies the empty lists through the API, checks
idempotency, and deletes the portal.

Every exchange remains strictly ordered. Portal creation is exchange 3;
initial customization and its update are PATCHes at exchanges 5 and 29.
The direct API PATCH at 41 seeds all three menu lists, and GET 42 verifies
them. PATCH 56 sends explicit empty arrays for `main`, `footer_sections`,
and `footer_bottom`; GET 57 verifies the resulting empty lists. The final
portal DELETE is exchange 71. Later-state reads cannot satisfy earlier
requests, and cleanup cannot cross an update or verification boundary.

No parallel-phase annotations, matcher relaxations, or sanitizer changes
are needed. UUIDs use existing deterministic placeholders. Methods, paths,
queries, bodies, and response media types retain exact matching. Expected
email-config and custom-domain 404s retain `application/problem+json`.
The cassette is byte-for-byte the successful candidate and is below the
per-file size limit; no chunk packing is needed.

## Measurements

The three loopback-only replays passed all assertions and matched all 71
exchanges. Scenario durations were 2.132, 2.138, and 2.124 seconds; wrapper
durations were 2.911, 3.005, and 2.682 seconds.

The frozen weighted-v1 baseline has 20 successful live observations with
a 14.795-second median. Replay uses about 12 seconds less execution time
relative to that history, but this is not a same-source benchmark or a
measured workflow speedup. Live scenario timing already includes its reset;
do not add the avoided reset twice. Shared shard resets, setup, replay job
startup, and time on other shards remain.

Recording's 13.639-second scenario duration includes proxy overhead and is
not the normal live baseline. Its 26.886-second wrapper additionally includes
the recording-specific before/after resets.

Only eligible PRs replay this scenario. Main and force-live runs remain
live. Replay skips the initial scenario reset entirely rather than replaying
reset transactions. Input or assertion changes require a new reviewed
recording in the same PR; follow the repository replay refresh procedure.

[record]: https://github.com/Kong/kongctl/actions/runs/35453473003
