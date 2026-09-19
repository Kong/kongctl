# Portal-owned dump replay

The [live recording][record] captured 453 exchanges on acceptance-2 under
its existing organization lock, including successful before/after cleanup.
All portal pages, snippets, teams/roles, customization, PNG assets, API
versions/documents/publications, and authentication-strategy assertions stay
intact. Normal and skip-defaults dumps both produce no-op plans. The scenario
then resets the organization, recreates resources from its dump, verifies
that dump, checks another no-op plan, and deletes the recreated resources.

## Reset and command boundaries

Both in-scenario resets run through the proxy; neither is skipped. Their
HTTP exchanges must be consumed before the following commands can run.

| Exchanges | Command |
| --- | --- |
| 1-13 | Initial reset inventory (already empty after workflow cleanup) |
| 14-64 | Initial apply |
| 65-118 | Full dump |
| 119-166 | No-op plan from full dump |
| 167-220 | Skip-defaults dump |
| 221-268 | No-op plan from skip-defaults dump |
| 269-286 | Mid-round-trip reset |
| 287-339 | Reconstruction from full dump |
| 340-393 | Dump after reconstruction |
| 394-441 | No-op plan after reconstruction |
| 442-453 | Final declarative deletion |

The annotation is bound to the exact source cassette SHA-256. Reset
inventory phases 1-13 and 269-281 permit independent category reads. Reset
deletions 282-286 remain strict: both APIs, the portal's absent custom domain,
the portal, and the authentication strategy are removed before reconstruction.

Initial planning reads (14-21), creation (22-64), reconstruction planning
(287-294), and reconstruction (295-339) stay within their respective CLI
commands. Generated-ID dependencies, repeated requests, and read/write
dependencies remain mandatory. Child creation cannot precede its parents.
Each of the six read-only dump/no-op-plan commands has its own phase matching
the table above. Team-role and API-version enumeration can vary in order;
every read still matches exactly once within that command's unchanged state.
Repeated portal ID lookups within creation can cross customization and
authentication-settings PATCHes only under the verifier's unchanged-inventory
rule: the recorded lookup responses differ solely in the target portal's
`updated_at` timestamp. Actual response bytes and repeated-request ordering
remain unchanged. This avoids assigning a concurrent child's lookup to an
update that is waiting for that very lookup.
Final deletion permits independent lookups (442-448) and API/portal deletes
(449-451). Authentication-strategy lookup/deletion (452-453) remains a strict
barrier after those deletes. No annotation crosses a command or reset boundary.

## Environment and measurement limitations

Both attempts of the [initial acceptance-3 recording][initial] failed to
recreate a portal whose dumped default auth-strategy ID no longer existed
after reset. No candidate was published. The same code's [normal live
scenario][live] passed on acceptance-2 with no default-ID field in its dump.
Recording therefore selects acceptance-2, without permanently pinning the
live scenario or changing its assertions. The acceptance-3 portability
problem is not claimed fixed by this replay addition.

The frozen 20-run live median is 32.925 seconds, including scenario resets.
Recording took 140.394 scenario seconds and includes serialized proxy
overhead; it is not the live performance baseline. Compare isolated replay
wrapper time, including setup, with the historical median as execution work,
not as a causal estimate of parallel workflow wall-clock savings.

[record]: https://github.com/Kong/kongctl/actions/runs/35470184997
[initial]: https://github.com/Kong/kongctl/actions/runs/35469445499
[live]: https://github.com/Kong/kongctl/actions/runs/35469673919
