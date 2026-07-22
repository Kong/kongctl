package resources

import "strings"

// RelationshipKind distinguishes API schema foreign keys from kongctl-added
// root-level parent selectors while giving both the same reference semantics.
type RelationshipKind string

const (
	RelationshipKindAPIForeignKey         RelationshipKind = "api_foreign_key"
	RelationshipKindKongctlParentSelector RelationshipKind = "kongctl_parent_selector"
)

// RelationshipDescriptor is static schema metadata for a cross-resource field.
type RelationshipDescriptor struct {
	FieldPath      string
	TargetType     ResourceType
	Kind           RelationshipKind
	ScopeFieldPath string
	RootOnly       bool
}

var relationshipDescriptors = map[ResourceType][]RelationshipDescriptor{
	ResourceTypeAPIPublication: {
		{FieldPath: "portal_id", TargetType: ResourceTypePortal, Kind: RelationshipKindAPIForeignKey},
	},
	ResourceTypeAPIImplementation: {
		{
			FieldPath:  "service.control_plane_id",
			TargetType: ResourceTypeControlPlane,
			Kind:       RelationshipKindAPIForeignKey,
		},
		{
			FieldPath: "service.id", TargetType: ResourceTypeGatewayService, Kind: RelationshipKindAPIForeignKey,
			ScopeFieldPath: "service.control_plane_id",
		},
	},
	ResourceTypeGatewayService: {
		{
			FieldPath: "control_plane", TargetType: ResourceTypeControlPlane,
			Kind: RelationshipKindKongctlParentSelector, RootOnly: true,
		},
	},
	ResourceTypeControlPlaneDataPlaneCertificate: {
		{
			FieldPath: "control_plane", TargetType: ResourceTypeControlPlane,
			Kind: RelationshipKindKongctlParentSelector, RootOnly: true,
		},
	},
	ResourceTypePortalAuditLogWebhook: {
		{
			FieldPath: "portal", TargetType: ResourceTypePortal,
			Kind: RelationshipKindKongctlParentSelector, RootOnly: true,
		},
		{
			FieldPath: "audit_log_destination_id", TargetType: ResourceTypeAuditLogWebhookDestination,
			Kind: RelationshipKindAPIForeignKey,
		},
	},
	ResourceTypeOrganizationTeamRole: {
		{
			FieldPath: "team", TargetType: ResourceTypeOrganizationTeam,
			Kind: RelationshipKindKongctlParentSelector, RootOnly: true,
		},
	},
	ResourceTypeOrganizationUserTeamMembership: {
		{
			FieldPath: "team", TargetType: ResourceTypeOrganizationTeam,
			Kind: RelationshipKindKongctlParentSelector, RootOnly: true,
		},
	},
	ResourceTypeOrganizationSystemAccountTeamMembership: {
		{
			FieldPath: "team", TargetType: ResourceTypeOrganizationTeam,
			Kind: RelationshipKindKongctlParentSelector, RootOnly: true,
		},
	},
	ResourceTypeEventGatewayVirtualCluster: {
		{
			FieldPath: "event_gateway", TargetType: ResourceTypeEventGatewayControlPlane,
			Kind: RelationshipKindKongctlParentSelector, RootOnly: true,
		},
	},
	ResourceTypeEventGatewayClusterPolicy: {
		{
			FieldPath: "event_gateway", TargetType: ResourceTypeEventGatewayControlPlane,
			Kind: RelationshipKindKongctlParentSelector, RootOnly: true,
		},
		{
			FieldPath: "virtual_cluster", TargetType: ResourceTypeEventGatewayVirtualCluster,
			Kind: RelationshipKindKongctlParentSelector, ScopeFieldPath: "event_gateway", RootOnly: true,
		},
	},
	ResourceTypeEventGatewayProducePolicy: {
		{
			FieldPath: "event_gateway", TargetType: ResourceTypeEventGatewayControlPlane,
			Kind: RelationshipKindKongctlParentSelector, RootOnly: true,
		},
		{
			FieldPath: "virtual_cluster", TargetType: ResourceTypeEventGatewayVirtualCluster,
			Kind: RelationshipKindKongctlParentSelector, ScopeFieldPath: "event_gateway", RootOnly: true,
		},
	},
	ResourceTypeEventGatewayConsumePolicy: {
		{
			FieldPath: "event_gateway", TargetType: ResourceTypeEventGatewayControlPlane,
			Kind: RelationshipKindKongctlParentSelector, RootOnly: true,
		},
		{
			FieldPath: "virtual_cluster", TargetType: ResourceTypeEventGatewayVirtualCluster,
			Kind: RelationshipKindKongctlParentSelector, ScopeFieldPath: "event_gateway", RootOnly: true,
		},
	},
}

var aiGatewayChildTypes = []ResourceType{
	ResourceTypeAIGatewayProvider,
	ResourceTypeAIGatewayIdentityProvider,
	ResourceTypeAIGatewayPolicy,
	ResourceTypeAIGatewayAgent,
	ResourceTypeAIGatewayConsumer,
	ResourceTypeAIGatewayConsumerGroup,
	ResourceTypeAIGatewayModel,
	ResourceTypeAIGatewayMCPServer,
	ResourceTypeAIGatewayVault,
	ResourceTypeAIGatewayDataPlaneCertificate,
}

var portalChildTypes = []ResourceType{
	ResourceTypePortalCustomization,
	ResourceTypePortalCustomDomain,
	ResourceTypePortalAuthSettings,
	ResourceTypePortalIPAllowList,
	ResourceTypePortalIntegration,
	ResourceTypePortalIdentityProvider,
	ResourceTypePortalTeamGroupMapping,
	ResourceTypePortalPage,
	ResourceTypePortalSnippet,
	ResourceTypePortalTeam,
	ResourceTypePortalTeamRole,
	ResourceTypePortalAssetLogo,
	ResourceTypePortalAssetFavicon,
	ResourceTypePortalEmailConfig,
	ResourceTypePortalEmailTemplate,
}

var eventGatewayChildTypes = []ResourceType{
	ResourceTypeEventGatewayBackendCluster,
	ResourceTypeEventGatewayListener,
	ResourceTypeEventGatewayDataPlaneCertificate,
	ResourceTypeEventGatewaySchemaRegistry,
	ResourceTypeEventGatewayStaticKey,
	ResourceTypeEventGatewayTLSTrustBundle,
}

func init() {
	for _, resourceType := range aiGatewayChildTypes {
		relationshipDescriptors[resourceType] = []RelationshipDescriptor{{
			FieldPath: SchemaFieldAIGateway, TargetType: ResourceTypeAIGateway,
			Kind: RelationshipKindKongctlParentSelector, RootOnly: true,
		}}
	}
	for _, resourceType := range portalChildTypes {
		relationshipDescriptors[resourceType] = []RelationshipDescriptor{{
			FieldPath: SchemaFieldPortal, TargetType: ResourceTypePortal,
			Kind: RelationshipKindKongctlParentSelector, RootOnly: true,
		}}
	}
	for _, resourceType := range eventGatewayChildTypes {
		relationshipDescriptors[resourceType] = []RelationshipDescriptor{{
			FieldPath: "event_gateway", TargetType: ResourceTypeEventGatewayControlPlane,
			Kind: RelationshipKindKongctlParentSelector, RootOnly: true,
		}}
	}
}

// RelationshipDescriptorsForType returns static relationship schema metadata.
func RelationshipDescriptorsForType(resourceType ResourceType) []RelationshipDescriptor {
	return append([]RelationshipDescriptor(nil), relationshipDescriptors[resourceType]...)
}

// RelationshipDescriptorsFor returns static descriptors plus compatibility
// mappings for fields not yet backfilled into the static schema registry.
func RelationshipDescriptorsFor(resource Resource) []RelationshipDescriptor {
	result := RelationshipDescriptorsForType(resource.GetType())
	seen := make(map[string]struct{}, len(result))
	for _, descriptor := range result {
		seen[descriptor.FieldPath] = struct{}{}
	}
	mapping, ok := resource.(ReferenceMapping)
	if !ok {
		return result
	}
	for fieldPath, target := range mapping.GetReferenceFieldMappings() {
		if _, exists := seen[fieldPath]; exists {
			continue
		}
		kind := RelationshipKindKongctlParentSelector
		if strings.HasSuffix(fieldPath, "_id") || strings.Contains(fieldPath, ".id") {
			kind = RelationshipKindAPIForeignKey
		}
		result = append(result, RelationshipDescriptor{
			FieldPath:  fieldPath,
			TargetType: ResourceType(target),
			Kind:       kind,
		})
	}
	return result
}
