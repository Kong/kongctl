package resources

import (
	"fmt"

	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/kong/kongctl/internal/util"
)

func init() {
	registerResourceType(
		ResourceTypePortalAuditLogWebhook,
		func(rs *ResourceSet) *[]PortalAuditLogWebhookResource { return &rs.PortalAuditLogWebhooks },
		AutoExplain[PortalAuditLogWebhookResource](
			WithExplainRecommendedFields("ref", "portal", "enabled", "audit_log_destination_id"),
			WithExplainFieldHint("portal", ExplainFieldHint{RefKind: string(ResourceTypePortal)}),
			WithExplainFieldHint(
				"audit_log_destination_id",
				ExplainFieldHint{RefKind: string(ResourceTypeAuditLogWebhookDestination)},
			),
		),
	)
}

// PortalAuditLogWebhookResource represents the portal audit-log webhook
// singleton child resource.
type PortalAuditLogWebhookResource struct {
	Ref                   string `yaml:"ref"                                json:"ref"`
	Portal                string `yaml:"portal,omitempty"                   json:"portal,omitempty"`
	Enabled               *bool  `yaml:"enabled,omitempty"                  json:"enabled,omitempty"`
	AuditLogDestinationID string `yaml:"audit_log_destination_id,omitempty" json:"audit_log_destination_id,omitempty"`

	konnectID string `yaml:"-" json:"-"`
}

func (w PortalAuditLogWebhookResource) GetType() ResourceType {
	return ResourceTypePortalAuditLogWebhook
}

func (w PortalAuditLogWebhookResource) GetRef() string {
	return w.Ref
}

func (w PortalAuditLogWebhookResource) GetMoniker() string {
	return w.Ref
}

func (w PortalAuditLogWebhookResource) GetDependencies() []ResourceRef {
	deps := make([]ResourceRef, 0, 2)
	if w.Portal != "" {
		deps = append(deps, ResourceRef{Kind: ResourceTypePortal, Ref: w.Portal})
	}
	if ref := auditLogDestinationRef(w.AuditLogDestinationID); ref != "" {
		deps = append(deps, ResourceRef{Kind: ResourceTypeAuditLogWebhookDestination, Ref: ref})
	}
	return deps
}

func (w PortalAuditLogWebhookResource) Validate() error {
	if err := ValidateRef(w.Ref); err != nil {
		return fmt.Errorf("invalid portal_audit_log_webhook ref: %w", err)
	}

	if w.Enabled == nil && w.AuditLogDestinationID == "" {
		return fmt.Errorf("portal_audit_log_webhook %q requires enabled or audit_log_destination_id", w.Ref)
	}

	if w.Enabled != nil && *w.Enabled && w.AuditLogDestinationID == "" {
		return fmt.Errorf("portal_audit_log_webhook %q requires audit_log_destination_id when enabled is true", w.Ref)
	}

	return nil
}

func (w *PortalAuditLogWebhookResource) SetDefaults() {}

func (w PortalAuditLogWebhookResource) GetKonnectID() string {
	return w.konnectID
}

func (w PortalAuditLogWebhookResource) GetKonnectMonikerFilter() string {
	return ""
}

func (w *PortalAuditLogWebhookResource) TryMatchKonnectResource(konnectResource any) bool {
	portalID, ok := konnectResource.(string)
	if !ok || portalID == "" {
		return false
	}
	w.konnectID = portalID
	return true
}

func (w PortalAuditLogWebhookResource) GetParentRef() *ResourceRef {
	if w.Portal == "" {
		return nil
	}
	return &ResourceRef{Kind: ResourceTypePortal, Ref: w.Portal}
}

func (w PortalAuditLogWebhookResource) GetReferenceFieldMappings() map[string]string {
	mappings := map[string]string{
		SchemaFieldPortal: string(ResourceTypePortal),
	}
	if ref := auditLogDestinationRef(w.AuditLogDestinationID); ref != "" {
		mappings["audit_log_destination_id"] = string(ResourceTypeAuditLogWebhookDestination)
	}
	return mappings
}

func auditLogDestinationRef(value string) string {
	if value == "" || util.IsValidUUID(value) {
		return ""
	}
	if tags.IsRefPlaceholder(value) {
		ref, _, ok := tags.ParseRefPlaceholder(value)
		if ok {
			return ref
		}
		return ""
	}
	return value
}
