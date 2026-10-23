# Control-plane apply replay

The complete expanded scenario passed live recording and three strict,
network-isolated replays of all 19 exchanges. The recording ran under the
acceptance-3 organization lock with successful before/after resets.

Coverage includes create, description/label addition, label value changes,
label removal/addition, preservation of the management namespace label,
readback, no-op planning, a filtered dump round trip, and deletion.
All requests remain strictly ordered; no parallel annotations are needed.

The isolated wrappers took 3.308, 3.238, and 3.453 seconds. These are scenario
execution measurements, not workflow speedup estimates. Recording includes
proxy and reset overhead and is not a normal live baseline.

Only eligible PRs replay this scenario. Main and force-live runs stay live.
Scenario input or assertion changes require a fresh recording in the same PR.

[Recording and three isolated validations](https://github.com/Kong/kongctl/actions/runs/39982974653)
