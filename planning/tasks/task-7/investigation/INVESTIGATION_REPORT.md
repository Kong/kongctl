# Investigation Report: CreatePortalCustomDomainRequest Structure

## Summary
This report provides a comprehensive analysis of the `CreatePortalCustomDomainRequest` structure in the Kong SDK, with a focus on understanding the SSL field structure and its usage.

## Key Findings

### 1. CreatePortalCustomDomainRequest Structure
Located in: `/vendor/github.com/Kong/sdk-konnect-go/models/components/createportalcustomdomainrequest.go`

```go
type CreatePortalCustomDomainRequest struct {
    Hostname string                      `json:"hostname"`
    Enabled  bool                        `json:"enabled"`
    Ssl      CreatePortalCustomDomainSSL `json:"ssl"`
}
```

**Fields:**
- `Hostname` (string): The custom domain hostname (e.g., "developer.example.com")
- `Enabled` (bool): Whether the custom domain is enabled
- `Ssl` (CreatePortalCustomDomainSSL): SSL configuration for the custom domain

### 2. SSL Field Structure
Located in: `/vendor/github.com/Kong/sdk-konnect-go/models/components/createportalcustomdomainssl.go`

```go
type CreatePortalCustomDomainSSL struct {
    DomainVerificationMethod PortalCustomDomainVerificationMethod `json:"domain_verification_method"`
}
```

The SSL structure contains only one field:
- `DomainVerificationMethod`: Specifies how the domain ownership will be verified

### 3. Domain Verification Method
Located in: `/vendor/github.com/Kong/sdk-konnect-go/models/components/portalcustomdomainverificationmethod.go`

```go
type PortalCustomDomainVerificationMethod string

const (
    PortalCustomDomainVerificationMethodHTTP PortalCustomDomainVerificationMethod = "http"
)
```

Currently, only one verification method is supported:
- `"http"`: HTTP-based domain verification

### 4. Usage in Kongctl

#### Resource Definition
In `/internal/declarative/resources/portal_custom_domain.go`:
- The `PortalCustomDomainResource` embeds `CreatePortalCustomDomainRequest`
- Adds `Ref` and `Portal` fields for resource management
- Validates hostname format but SSL validation is marked as TODO

#### Adapter Implementation
In `/internal/declarative/executor/portal_domain_adapter.go`:
- Maps user-provided fields to SDK structures
- Handles SSL configuration in `MapCreateFields`:
  ```go
  if sslData, ok := fields["ssl"].(map[string]interface{}); ok {
      ssl := kkComps.CreatePortalCustomDomainSSL{}
      if method, ok := sslData["domain_verification_method"].(string); ok {
          ssl.DomainVerificationMethod = kkComps.PortalCustomDomainVerificationMethod(method)
      }
      create.Ssl = ssl
  }
  ```

#### Test Example
In `/internal/declarative/planner/portal_child_test.go`:
```go
CreatePortalCustomDomainRequest: kkComps.CreatePortalCustomDomainRequest{
    Hostname: "developer.example.com",
    Enabled:  true,
    Ssl: kkComps.CreatePortalCustomDomainSSL{
        DomainVerificationMethod: kkComps.PortalCustomDomainVerificationMethodHTTP,
    },
}
```

### 5. YAML Configuration
Based on the example in `/docs/examples/declarative/namespace/single-team/portal.yaml`:
```yaml
custom_domain:
  ref: internal-domain
  hostname: "api.internal.example.com"
```

The example doesn't show SSL configuration, suggesting it might be optional or have defaults.

## Recommendations

1. **Documentation**: The SSL field structure should be documented in YAML examples:
   ```yaml
   custom_domain:
     ref: internal-domain
     hostname: "api.internal.example.com"
     enabled: true
     ssl:
       domain_verification_method: "http"
   ```

2. **Validation**: The TODO comment in `portal_custom_domain.go` (line 39) indicates SSL validation should be implemented.

3. **Defaults**: Consider if SSL configuration should have defaults when not specified.

4. **Required Fields**: The adapter lists only `hostname` and `enabled` as required, but the SDK structure suggests `ssl` is also required.

## Related Files
1. SDK Definition Files:
   - `/vendor/github.com/Kong/sdk-konnect-go/models/components/createportalcustomdomainrequest.go`
   - `/vendor/github.com/Kong/sdk-konnect-go/models/components/createportalcustomdomainssl.go`
   - `/vendor/github.com/Kong/sdk-konnect-go/models/components/portalcustomdomainverificationmethod.go`

2. Implementation Files:
   - `/internal/declarative/resources/portal_custom_domain.go`
   - `/internal/declarative/executor/portal_domain_adapter.go`
   - `/internal/declarative/executor/portal_child_operations.go`
   - `/internal/konnect/helpers/portal_custom_domains.go`

3. Test Files:
   - `/internal/declarative/planner/portal_child_test.go`

4. Example Files:
   - `/docs/examples/declarative/namespace/single-team/portal.yaml`

## Conclusion
The `CreatePortalCustomDomainRequest` structure is well-defined in the SDK with a simple SSL configuration that currently only supports HTTP-based domain verification. The implementation in kongctl properly handles this structure, though there are opportunities for improvement in validation and documentation.