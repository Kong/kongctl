# Plan: Shield declarative resources from SDK 0.11 JSON hooks

## Context
- SDK 0.11 introduced `MarshalJSON`/`UnmarshalJSON` on many generated structs.
- Our declarative resources anonymously embed those SDK structs (e.g., `kkComps.CreatePortal`), so the generated decode logic is now promoted onto our types.
- During YAML ingest the promoted `UnmarshalJSON` ignores kongctl-only fields (`ref`, `_kongctl`, etc.), causing validation to fail.
- Goal: keep anonymous embedding (field ergonomics) while preventing the SDK’s JSON methods from hijacking our decode path.

## Approach Overview
Wrap each embedded SDK struct in a new **defined type** (same underlying layout, no promoted methods) and embed that wrapper instead. Add helper conversions where needed.

## Inventory: structs currently embedding SDK types

| Resource struct                                  | Embedded SDK type                            | File |
|--------------------------------------------------|----------------------------------------------|------|
| `PortalResource`                                 | `kkComps.CreatePortal`                       | `internal/declarative/resources/portal.go` |
| `APIResource`                                    | `kkComps.CreateAPIRequest`                   | `internal/declarative/resources/api.go` |
| `ApplicationAuthStrategyResource`                | `kkComps.CreateAppAuthStrategyRequest`       | `internal/declarative/resources/auth_strategy.go` |
| `ControlPlaneResource`                           | `kkComps.CreateControlPlaneRequest`          | `internal/declarative/resources/control_plane.go` |
| `APIVersionResource`                             | `kkComps.CreateAPIVersionRequest`            | `internal/declarative/resources/api_version.go` |
| `APIPublicationResource`                         | `kkComps.APIPublication`                     | `internal/declarative/resources/api_publication.go` |
| `APIImplementationResource`                      | `kkComps.APIImplementation`                  | `internal/declarative/resources/api_implementation.go` |
| `APIDocumentResource`                            | `kkComps.CreateAPIDocumentRequest`           | `internal/declarative/resources/api_document.go` |
| `GatewayServiceResource`                         | `kkComps.Service`                            | `internal/declarative/resources/gateway_service.go` |
| `PortalCustomizationResource`                    | `kkComps.PortalCustomization`                | `internal/declarative/resources/portal_customization.go` |
| `PortalCustomDomainResource`                     | `kkComps.CreatePortalCustomDomainRequest`    | `internal/declarative/resources/portal_custom_domain.go` |
| `PortalPageResource`                             | `kkComps.CreatePortalPageRequest`            | `internal/declarative/resources/portal_page.go` |

> Confirm during implementation: run `rg 'yaml:",inline"' internal/declarative/resources` to catch any additions.

## Wrapper type pattern
For each embedded SDK struct:

```go
// Example: portal resource
type createPortalData kkComps.CreatePortal

type PortalResource struct {
    createPortalData `yaml:",inline" json:",inline"`
    Ref              string `yaml:"ref" json:"ref"`
    // ...
}
```

Companion helpers:

```go
func newCreatePortalData(src kkComps.CreatePortal) createPortalData {
    return createPortalData(src)
}

func (d createPortalData) toSDK() kkComps.CreatePortal {
    return kkComps.CreatePortal(d)
}
```

Guidelines:
- Place wrappers near the resource definition (same file) or in a new `internal/declarative/resources/sdk_wrappers.go` for reuse.
- Export wrappers only if needed outside the package; prefer unexported (`createPortalData`) given usage is internal.
- Provide helper functions for common conversions (e.g., `func (r *PortalResource) ToSDK() kkComps.CreatePortal`).
- If existing code relies on SDK getters (`GetName()`, etc.), add thin forwarding methods on the resource or adjust call sites to use `util.StringValue` on the pointer fields.

## Migration Steps
1. **Introduce wrappers**
   - For each embedded type, declare an unexported defined type.
   - Add conversion helpers (constructor from SDK, `toSDK`, optionally pointer helpers).

2. **Update resource structs**
   - Replace each `kkComps.X` embedded field with the new wrapper type.
   - Ensure `yaml/json` tags remain unchanged.

3. **Adjust resource methods**
   - Where we previously referenced promoted SDK methods, update:
     - Direct field access still works (`r.Name`).
     - Replace calls to SDK getters with helpers if necessary.
   - When returning SDK types (e.g., in executor/planner adapters), call `.toSDK()` or cast the wrapper.

4. **Update planner / executor / loader interactions**
   - Any place we build SDK requests from resources must convert via the helpers.
   - Validate pointer semantics remain intact (we keep the pointer fields since wrapper shares underlying struct).

5. **Revisit tests & fixtures**
   - Ensure `stringPtr(...)` assignments still compile with new wrapper type (fields remain `*string`).
   - Adjust assertions that relied on SDK getters if they no longer compile.

6. **Clean up temporary debug artifacts** (ensure none remain from investigative work).

## Additional considerations
- **Typed conversions**: prefer helper methods attached to the resource (e.g., `func (r *PortalResource) AsSDK() kkComps.CreatePortal`) to avoid repetitive casting logic.
- **Zero-value safety**: guard helpers against nil receivers; returning zero-valued SDK structs matches current behaviour.
- **Marshalling back out**: verify any code using `json.Marshal` / `yaml.Marshal` on resource structs still works (wrappers have default reflection path, so behaviour should match pre-0.11).
- **Non-create structs**: `APIPublicationResource` embeds `kkComps.APIPublication` (a response model). Confirm whether the SDK added custom JSON there too; if so, wrap likewise.

## Testing & validation
- Unit: `go test ./internal/declarative/...`.
- Integration: `make test` plus `make test-integration` (focus on declarative suite).
- Manual sanity: run `kongctl plan --mode apply -f docs/examples/declarative/portal/portal.yaml` (reproduced failure).

## Risks / open questions
- Forward compatibility: if the SDK adds methods we want, wrappers will need equivalent helpers.
- External packages: audit for other anonymous embeddings outside `internal/declarative/resources` (e.g., state client, executor) and decide whether wrappers are needed there too.
- Ensure no JSON tags rely on promoted methods (unlikely, but worth verifying).

This plan should unblock implementation by outlining exactly which structs need wrappers, how to introduce them, and what follow-up adjustments are required across the declarative engine.
