Issue 322 - Noisy Planner (Portal Assets and Publications)
===========================================================

Current understanding (no resolution planned)
---------------------------------------------

Summary
- Issue #322 reports noisy planner output on consecutive runs with no config changes.
- Affects portal assets (logo/favicon) and sometimes API publications.
- The plan shows UPDATEs on the second apply with no edits between runs.
- Regression timing is unknown.

Observed behavior (user reports)
- Running `kongctl plan --mode apply` twice on the same inputs produces 4 UPDATEs:
  - `portal_asset_logo`
  - `portal_asset_favicon`
  - `api_publication` (sms)
  - `api_publication` (voice)
- No files are modified between runs.
- Asset files are static on disk.
- A simplified example `docs/examples/declarative/basic/api-with-portal-pub.yaml` is idempotent
  until assets are added; with assets present, the plan becomes noisy even if only favicon is set.
- In HTTP dumps from `kongctl plan`, no GETs were observed for portal asset endpoints
  (e.g., `/v3/portals/{portalId}/assets/logo` or `/assets/favicon`).

Simplified repro input
- `docs/examples/declarative/basic/api-with-portal-pub.yaml`
  - Portal assets are declared under `portals[].assets`.
  - `logo` and/or `favicon` are file references, e.g. `!file ../portal/assets/favicon.png`.

API response observations (from prior dumps)
- `GET /v3/api-publications?filter[api_id][eq]=...` returns visibility `private` even though
  desired config sets `public`. This mismatch appears in the full portal example but not in the
  simplified assets-only repro (publication issue is not reproduced there yet).
- No GET calls for asset endpoints appear in the plan dump, suggesting no comparison step.

Code evidence (planner always schedules asset updates)
- `internal/declarative/planner/portal_child_planner.go`
  - `planPortalAssetLogosChanges` and `planPortalAssetFaviconsChanges` contain:
    "Assets are singletons - always plan UPDATE (no comparison needed)"
  - Both call `planPortalAssetLogoUpdate` / `planPortalAssetFaviconUpdate`, which set
    `Action: ActionUpdate` unconditionally and include the `data_url` in Fields.
  - References:
    - `internal/declarative/planner/portal_child_planner.go:895-973`
    - `internal/declarative/planner/portal_child_planner.go:977-1046`

Code evidence (asset GET exists but unused by planner)
- `internal/declarative/state/client.go`
  - `GetPortalAssetLogo` and `GetPortalAssetFavicon` call the assets API endpoints.
  - These functions are not invoked by the portal asset planning path.
  - References:
    - `internal/declarative/state/client.go:2416-2435`
    - `internal/declarative/state/client.go:2460-2479`

Working hypothesis
- The planner never compares desired asset content to the current server state, so it always
  schedules an UPDATE for portal assets. This aligns with missing asset GET requests in the
  plan HTTP dump and the explicit "always plan UPDATE" comments in the planner.

Open questions / data to collect (for a future session)
- Confirm whether Konnect returns portal asset data via GET and whether its `data_url` matches
  the local file (or is normalized/rewritten).
- Decide whether asset updates should be idempotent (compare content) or intentionally always
  update (by design).
- If comparison is desired, determine the best comparison basis (full data_url, hash, size)
  and acceptable normalization (e.g., base64 wrapping, content-type changes).
- For the publication issue, confirm a minimal repro where publications remain noisy after
  assets are removed and compare expected vs. returned `visibility`/`auth_strategy_ids`.
