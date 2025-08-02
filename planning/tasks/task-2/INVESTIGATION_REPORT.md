# Investigation Report: Missing API Versions and Publications in Sync Plan

## Problem Statement

When running the sync command with portal and API YAML files, the plan summary shows APIs being created but no API versions or publications are planned. This is a gap in the API resource behavior.

Example command:
```bash
k sync -f docs/examples/declarative/portal/getting-started/portal.yaml -f docs/examples/declarative/portal/getting-started/apis.yaml
```

The plan summary shows:
- api: 2 (resources to create)
- But no api_version or api_publication resources

## Investigation Summary

### 1. YAML Structure Analysis

The example YAML files show that APIs are defined with nested child resources:

**apis.yaml structure:**
```yaml
apis:
  - ref: sms
    name: "SMS API"
    versions:
      - ref: sms-v1
        version: "1.0.0"
        spec: !file apis/sms/openapi.yaml
    publications:
      - ref: sms-api-to-getting-started
        portal_id: getting-started
        visibility: public
```

### 2. Loader Processing

The loader (`internal/declarative/loader/loader.go`) correctly handles nested resources:

1. **Parsing**: The loader parses the YAML files and captures nested resources within API objects
2. **Extraction**: The `extractNestedResources` function (line 892) properly extracts nested child resources:
   - Extracts versions from `api.Versions` and appends to `rs.APIVersions`
   - Extracts publications from `api.Publications` and appends to `rs.APIPublications`
   - Sets the parent reference (e.g., `version.API = api.Ref`)
   - Clears the nested arrays from the parent API

### 3. Planner Processing

The planner receives the extracted resources but filters them out incorrectly:

1. **Resource Assignment**: In `GeneratePlan` (line 145-146), the planner assigns:
   ```go
   namespacePlanner.desiredAPIVersions = namespaceResources.APIVersions
   namespacePlanner.desiredAPIPublications = namespaceResources.APIPublications
   ```

2. **Filtering Issue**: The `filterResourcesByNamespace` function (line 516) has a critical bug:
   ```go
   // Building filter map using API names
   apiNames := make(map[string]bool)
   for _, api := range filtered.APIs {
       apiNames[api.Name] = true  // Stores "SMS API", "Voice API"
   }
   
   // Filtering child resources using parent refs
   for _, version := range rs.APIVersions {
       if apiNames[version.API] {  // version.API is "sms", not "SMS API"!
           filtered.APIVersions = append(filtered.APIVersions, version)
       }
   }
   ```

## Root Cause

The bug is in `filterResourcesByNamespace` at lines 545-560. The function builds a map of API **names** but then tries to match against API **refs** when filtering child resources.

- `apiNames` map contains: `"SMS API"`, `"Voice API"` (the name field)
- `version.API` contains: `"sms"`, `"voice"` (the ref field)
- Since `"sms" != "SMS API"`, the condition fails and child resources are excluded

## Impact

This bug causes all API child resources (versions, publications, implementations, documents) that are defined at the root level with parent references to be filtered out during namespace processing, preventing them from being included in the plan.

## Fix Location

The fix needs to be applied in `/internal/declarative/planner/planner.go` at line 545:

Change from:
```go
apiNames := make(map[string]bool)
for _, api := range filtered.APIs {
    apiNames[api.Name] = true
}
```

To:
```go
apiRefs := make(map[string]bool)
for _, api := range filtered.APIs {
    apiRefs[api.Ref] = true
}
```

And update all references from `apiNames` to `apiRefs` in the subsequent filtering logic (lines 552, 558, 564, 570).

## Additional Notes

1. The same pattern is correctly implemented for portals (using `portalRefs` at line 540)
2. The API planner does have logic to handle child resources, but they never reach it due to being filtered out
3. This issue only affects child resources defined at the root level; nested resources that remain within the API object would still be processed via `planAPIChildResourcesCreate`