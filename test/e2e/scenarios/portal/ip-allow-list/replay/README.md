# Portal IP allow-list replay

The unchanged scenario passed [live recording][record] with 136 exchanges,
under the existing acceptance-3 lock and with both before/after resets.
Scenario inputs and assertions are unchanged. Coverage includes nested and
root-level allow-list creation, updates, no-op plans, dump assertions and
portal cleanup. The IP addresses are the existing public example fixtures.

Strict replay stopped at exchange 6: the two independent allow-list creates
arrived in a different order. The candidate-bound `parallel-phases.json`
describes independent operations within individual commands using the existing
matcher. All method, path, query and body matching remains exact, once only.

| Exchanges | Scenario command |
| --- | --- |
| 4–7 | Apply independent portals and their allow lists |
| 8–39 | Dump portals and their children |
| 40–56 | No-op plan after creation |
| 57–73 | Plan allow-list updates |
| 74–81 | Apply the two independent allow-list updates |
| 82–98 | No-op plan after updates |
| 99–130 | Dump updated portals and their children |
| 131–136 | Cleanup inventory, lookups and portal deletion |

Initial inventory at 1–3 remains strict. Mandatory generated-ID dependencies
keep each allow-list create after its portal create. The update phase keeps
inventory and each allow-list GET before its PATCH; post-update assertions
cannot consume pre-update responses. Cleanup reads remain before deletion.
Identical requests retain stream order. No phase crosses a command boundary,
and no later-state response can satisfy an earlier-state command.

The annotated candidate passed [three loopback-only replays][replay], each
matching all 136 exchanges. Scenario durations were 1.732, 1.715 and 1.683
seconds; wrapper durations were 2.297, 2.388 and 2.399 seconds. The committed
chunks preserve the validated candidate exactly, including all response data,
ordering annotations and provenance. No matcher or sanitizer rules changed.

The ordinary [PR run at the recording's source commit][live] measured 13.01
seconds live. This suggests about 11 seconds less scenario work, not a
controlled benchmark or a measured end-to-end workflow saving. Live subtest
timing includes the scenario reset, so reset savings must not be added again.
Replay's scenario timer additionally includes test-process startup. Recording's
43.845-second scenario duration includes proxy overhead and is not a normal
live baseline.

Only eligible PRs use this cassette. Main and force-live runs stay live. Input
or assertion changes invalidate the fingerprint and require a new reviewed
recording or removal from replay policy.

[record]: https://github.com/Kong/kongctl/actions/runs/35165520273
[replay]: https://github.com/Kong/kongctl/actions/runs/35166388118
[live]: https://github.com/Kong/kongctl/actions/runs/35165525053
