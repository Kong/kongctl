# Issue 225 – External Namespace Safety

## 2025-11-14

## Summary
- External resources currently inherit implicit `default` namespaces, which drags unmanaged namespaces 
    into sync planning and triggers unintended deletes (to be demonstrated via a new failing `test/e2e/scenarios/external` scenario).
- External resources should never carry `kongctl` metadata; they exist strictly to resolve IDs from Konnect.
- Namespace guardrails already exist via `--require-any-namespace` / `--require-namespace`; fixes must remain compatible with those flags.
- Documentation must clearly describe how namespaces interact with sync mode, children, and `_external` references.

## Implementation Plan
1. **Validation/Loader**
   - Reject any resource that defines both `_external` and `kongctl` metadata (parse-time error).
   - Skip assigning implicit namespaces/protected defaults to `_external` resources; they should not contribute to `ResourceSet.DefaultNamespace`.
2. **Planner**
   - Build namespace lists using only managed (non-external) parents so sync mode never deletes based on external references.
   - Ensure updated behavior works seamlessly with `--require-any-namespace` / `--require-namespace`
3. **Testing**
   - Author a new `test/e2e/scenarios/external/...` scenario that syncs a config containing only an external portal plus a team-owned 
        API/Publication and asserts that a portal DELETE is planned (current bug).
   - After implementing fixes, rerun the scenario to confirm the DELETE is eliminated. Add supplemental unit tests for 
        loader validation and namespace filtering.
4. **Documentation**
   - Expand `docs/declarative.md` namespace section to cover:
     * Only parent resources support namespaces (labels).
     * `_external` resources cannot declare namespaces or protection.
     * Sync mode only considers namespaces from managed parents.
     * Managing children of external parents relies on resolving the parent ID, not namespace labels.
     * Usage and behavior of the new `--allowed-namespace` flag.
