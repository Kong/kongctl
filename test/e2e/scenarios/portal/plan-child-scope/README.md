# Portal child planning scope

This live CLI scenario reproduces issue #2706 without relying on the caller's
Portal role. It creates an existing managed portal with a page, snippet, team,
and IP allow list, then supplies a parent-only manifest.

The plan must update only the portal and capture exactly one HTTP request:
`GET /v3/portals?...`. Requiring both the request count and its path proves
that HTTP capture is active and no child endpoint was read. The same check
covers parent-only idempotency and an external parent resolved by name.

After applying the parent update, a dump verifies that all seeded children
retain their values. Further plans require the team read for a nested team
declaration and the IP allow-list read for a root-level declaration. Finally,
sync with `teams: []` must read teams and plan their deletion while leaving
the omitted child collections outside scope.

HTTP assertions use the existing harness's SDK request-dump artifacts. This
scenario runs live; existing replay eligibility and recordings are unchanged.

The [focused live validation][validation] ran this scenario with both CLI
versions under the same organization lock. The pre-fix CLI (`251b51c2`)
failed the parent-only request assertion with nine requests instead of one.
The fixed CLI passed all 25 assertions, including retained child state,
declared-child reads, sync scope, and cleanup.

Run with `make test-e2e-scenarios SCENARIO=portal/plan-child-scope`.

[validation]: https://github.com/Kong/kongctl/actions/runs/41921723592
