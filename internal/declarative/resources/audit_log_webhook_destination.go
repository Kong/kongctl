package resources

import "fmt"

func init() {
	registerExternalResourceTypeWithSliceAccessors(
		ResourceTypeAuditLogWebhookDestination,
		auditLogWebhookDestinationSlice,
		ensureAuditLogWebhookDestinationSlice,
		AutoExplain[AuditLogWebhookDestinationResource](
			WithExplainAliases("audit-logs.destinations"),
			WithExplainRecommendedFields("ref", "_external"),
		),
		ExternalResolutionRegistration{Selectors: []string{SchemaFieldName}},
	)
}

func (d *AuditLogWebhookDestinationResource) GetExternalBlock() *ExternalBlock { return d.External }

func auditLogWebhookDestinationSlice(rs *ResourceSet) *[]AuditLogWebhookDestinationResource {
	if rs == nil || rs.AuditLogs == nil {
		return nil
	}
	return &rs.AuditLogs.Destinations
}

func ensureAuditLogWebhookDestinationSlice(rs *ResourceSet) *[]AuditLogWebhookDestinationResource {
	if rs.AuditLogs == nil {
		rs.AuditLogs = &AuditLogsResource{}
	}
	return &rs.AuditLogs.Destinations
}

// AuditLogsResource groups organization-scoped audit-log declarative resources.
type AuditLogsResource struct {
	Destinations []AuditLogWebhookDestinationResource `yaml:"destinations,omitempty" json:"destinations,omitempty"`
}

// AuditLogWebhookDestinationResource references an externally managed Konnect
// audit-log webhook destination.
type AuditLogWebhookDestinationResource struct {
	BaseResource
	External *ExternalBlock `yaml:"_external,omitempty" json:"_external,omitempty"`
}

func (d AuditLogWebhookDestinationResource) GetType() ResourceType {
	return ResourceTypeAuditLogWebhookDestination
}

func (d AuditLogWebhookDestinationResource) GetMoniker() string {
	if d.External != nil && d.External.Selector != nil {
		if name, ok := d.External.Selector.MatchFields["name"]; ok {
			return name
		}
	}
	return d.Ref
}

func (d AuditLogWebhookDestinationResource) GetDependencies() []ResourceRef {
	return nil
}

func (d AuditLogWebhookDestinationResource) Validate() error {
	if err := ValidateRef(d.Ref); err != nil {
		return fmt.Errorf("invalid audit_log_webhook_destination ref: %w", err)
	}

	if d.Kongctl != nil {
		return fmt.Errorf("audit_log_webhook_destination %q cannot use kongctl metadata", d.Ref)
	}

	if d.External == nil {
		return fmt.Errorf("audit_log_webhook_destination %q requires _external", d.Ref)
	}

	if err := d.External.Validate(); err != nil {
		return fmt.Errorf("invalid _external block: %w", err)
	}

	if d.External.Selector != nil {
		_, hasName := d.External.Selector.MatchFields["name"]
		if len(d.External.Selector.MatchFields) != 1 || !hasName {
			return fmt.Errorf("audit_log_webhook_destination %s: selector supports matchFields.name only", d.Ref)
		}
	}

	return nil
}

func (d *AuditLogWebhookDestinationResource) SetDefaults() {}

func (d AuditLogWebhookDestinationResource) GetKonnectMonikerFilter() string {
	return ""
}

func (d *AuditLogWebhookDestinationResource) TryMatchKonnectResource(konnectResource any) bool {
	id, ok := tryMatchByNameWithExternal(d.GetMoniker(), konnectResource, matchOptions{}, d.External)
	if ok {
		d.SetKonnectID(id)
	}
	return ok
}

func (d *AuditLogWebhookDestinationResource) IsExternal() bool {
	return d.External != nil && d.External.IsExternal()
}
