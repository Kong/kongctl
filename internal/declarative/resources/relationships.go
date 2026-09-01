package resources

import "slices"

// RelationshipKind distinguishes API schema foreign keys from kongctl-added
// root-level parent selectors while giving both the same reference semantics.
type RelationshipKind string

type RelationshipCardinality string

type RelationshipResultField string

const (
	RelationshipKindAPIForeignKey         RelationshipKind        = "api_foreign_key"
	RelationshipKindKongctlParentSelector RelationshipKind        = "kongctl_parent_selector"
	RelationshipCardinalityScalar         RelationshipCardinality = "scalar"
	RelationshipCardinalityList           RelationshipCardinality = "list"
	RelationshipResultFieldID             RelationshipResultField = "id"
	RelationshipResultFieldRef            RelationshipResultField = "ref"
	RelationshipResultFieldName           RelationshipResultField = "name"
)

// RelationshipDescriptor is static schema metadata for a cross-resource field.
type RelationshipDescriptor struct {
	FieldPath                    string
	TargetType                   ResourceType
	TargetTypes                  []ResourceType
	TargetDiscriminatorFieldPath string
	TargetTypeResolver           func(string) (ResourceType, bool)
	Kind                         RelationshipKind
	Cardinality                  RelationshipCardinality
	ResultField                  RelationshipResultField
	ScopeFieldPath               string
	RootOnly                     bool
}

var relationshipDescriptors = map[ResourceType][]RelationshipDescriptor{
	ResourceTypeAPIVersion: {
		{FieldPath: "api", TargetType: ResourceTypeAPI, Kind: RelationshipKindKongctlParentSelector, RootOnly: true},
	},
	ResourceTypeAPIPublication: {
		{FieldPath: "api", TargetType: ResourceTypeAPI, Kind: RelationshipKindKongctlParentSelector, RootOnly: true},
		{FieldPath: "portal_id", TargetType: ResourceTypePortal, Kind: RelationshipKindAPIForeignKey},
		{
			FieldPath: "auth_strategy_ids", TargetType: ResourceTypeApplicationAuthStrategy,
			Kind: RelationshipKindAPIForeignKey, Cardinality: RelationshipCardinalityList,
		},
	},
	ResourceTypeAPIImplementation: {
		{FieldPath: "api", TargetType: ResourceTypeAPI, Kind: RelationshipKindKongctlParentSelector, RootOnly: true},
		{
			FieldPath:  "control_plane.control_plane_id",
			TargetType: ResourceTypeControlPlane,
			Kind:       RelationshipKindAPIForeignKey,
		},
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
	ResourceTypeAPIDocument: {
		{FieldPath: "api", TargetType: ResourceTypeAPI, Kind: RelationshipKindKongctlParentSelector, RootOnly: true},
		{
			FieldPath: "parent_document_ref", TargetType: ResourceTypeAPIDocument,
			Kind: RelationshipKindKongctlParentSelector, ScopeFieldPath: "api",
		},
	},
	ResourceTypePortal: {
		{
			FieldPath: "default_application_auth_strategy_id", TargetType: ResourceTypeApplicationAuthStrategy,
			Kind: RelationshipKindAPIForeignKey,
		},
	},
	ResourceTypeApplicationAuthStrategy: {
		{FieldPath: "dcr_provider_id", TargetType: ResourceTypeDCRProvider, Kind: RelationshipKindAPIForeignKey},
	},
	ResourceTypePortalPage: {
		{
			FieldPath: "parent_page_ref", TargetType: ResourceTypePortalPage,
			Kind: RelationshipKindKongctlParentSelector, ScopeFieldPath: "portal",
		},
	},
	ResourceTypePortalTeamGroupMapping: {
		{
			FieldPath: SchemaFieldTeam, TargetType: ResourceTypePortalTeam,
			Kind: RelationshipKindKongctlParentSelector, ScopeFieldPath: SchemaFieldPortal, RootOnly: true,
		},
	},
	ResourceTypePortalTeamRole: {
		{
			FieldPath: SchemaFieldTeam, TargetType: ResourceTypePortalTeam,
			Kind: RelationshipKindKongctlParentSelector, ScopeFieldPath: SchemaFieldPortal, RootOnly: true,
		},
		{
			FieldPath: "entity_id", TargetDiscriminatorFieldPath: "entity_type_name",
			TargetTypeResolver: RoleEntityResourceType,
			TargetTypes:        []ResourceType{ResourceTypeAPI, ResourceTypePortal, ResourceTypeControlPlane},
			Kind:               RelationshipKindAPIForeignKey,
		},
	},
	ResourceTypeAIGatewayConsumerCredential: {
		{
			FieldPath: SchemaFieldAIGatewayConsumer, TargetType: ResourceTypeAIGatewayConsumer,
			Kind: RelationshipKindKongctlParentSelector, RootOnly: true,
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
			FieldPath: SchemaFieldTeam, TargetType: ResourceTypeOrganizationTeam,
			Kind: RelationshipKindKongctlParentSelector, RootOnly: true,
		},
		{
			FieldPath: "entity_id", TargetDiscriminatorFieldPath: "entity_type_name",
			TargetTypeResolver: RoleEntityResourceType,
			TargetTypes:        []ResourceType{ResourceTypeAPI, ResourceTypePortal, ResourceTypeControlPlane},
			Kind:               RelationshipKindAPIForeignKey,
		},
	},
	ResourceTypeOrganizationUserRole: {
		{
			FieldPath: "entity_id", TargetDiscriminatorFieldPath: "entity_type_name",
			TargetTypeResolver: RoleEntityResourceType,
			TargetTypes:        []ResourceType{ResourceTypeAPI, ResourceTypePortal, ResourceTypeControlPlane},
			Kind:               RelationshipKindAPIForeignKey,
		},
	},
	ResourceTypeOrganizationSystemAccountRole: {
		{
			FieldPath: "entity_id", TargetDiscriminatorFieldPath: "entity_type_name",
			TargetTypeResolver: RoleEntityResourceType,
			TargetTypes:        []ResourceType{ResourceTypeAPI, ResourceTypePortal, ResourceTypeControlPlane},
			Kind:               RelationshipKindAPIForeignKey,
		},
	},
	ResourceTypeOrganizationUserTeamMembership: {
		{
			FieldPath: SchemaFieldTeam, TargetType: ResourceTypeOrganizationTeam,
			Kind: RelationshipKindKongctlParentSelector, RootOnly: true,
		},
	},
	ResourceTypeOrganizationSystemAccountTeamMembership: {
		{
			FieldPath: SchemaFieldTeam, TargetType: ResourceTypeOrganizationTeam,
			Kind: RelationshipKindKongctlParentSelector, RootOnly: true,
		},
	},
	ResourceTypeEventGatewayVirtualCluster: {
		{
			FieldPath: SchemaFieldEventGateway, TargetType: ResourceTypeEventGatewayControlPlane,
			Kind: RelationshipKindKongctlParentSelector, RootOnly: true,
		},
	},
	ResourceTypeEventGatewayClusterPolicy: {
		{
			FieldPath: SchemaFieldEventGateway, TargetType: ResourceTypeEventGatewayControlPlane,
			Kind: RelationshipKindKongctlParentSelector, RootOnly: true,
		},
		{
			FieldPath: "virtual_cluster", TargetType: ResourceTypeEventGatewayVirtualCluster,
			Kind: RelationshipKindKongctlParentSelector, ScopeFieldPath: SchemaFieldEventGateway, RootOnly: true,
		},
	},
	ResourceTypeEventGatewayProducePolicy: {
		{
			FieldPath: SchemaFieldEventGateway, TargetType: ResourceTypeEventGatewayControlPlane,
			Kind: RelationshipKindKongctlParentSelector, RootOnly: true,
		},
		{
			FieldPath: "virtual_cluster", TargetType: ResourceTypeEventGatewayVirtualCluster,
			Kind: RelationshipKindKongctlParentSelector, ScopeFieldPath: SchemaFieldEventGateway, RootOnly: true,
		},
	},
	ResourceTypeEventGatewayConsumePolicy: {
		{
			FieldPath: SchemaFieldEventGateway, TargetType: ResourceTypeEventGatewayControlPlane,
			Kind: RelationshipKindKongctlParentSelector, RootOnly: true,
		},
		{
			FieldPath: "virtual_cluster", TargetType: ResourceTypeEventGatewayVirtualCluster,
			Kind: RelationshipKindKongctlParentSelector, ScopeFieldPath: SchemaFieldEventGateway, RootOnly: true,
		},
	},
	ResourceTypeEventGatewayListenerPolicy: {
		{
			FieldPath: "listener", TargetType: ResourceTypeEventGatewayListener,
			Kind: RelationshipKindKongctlParentSelector, ScopeFieldPath: SchemaFieldEventGateway, RootOnly: true,
		},
	},
}

var aiGatewayChildTypes = []ResourceType{
	ResourceTypeAIGatewayProvider,
	ResourceTypeAIGatewayAuthStrategy,
	ResourceTypeAIGatewayPolicy,
	ResourceTypeAIGatewayAgent,
	ResourceTypeAIGatewayConsumer,
	ResourceTypeAIGatewayConsumerGroup,
	ResourceTypeAIGatewayModel,
	ResourceTypeAIGatewayMCPServer,
	ResourceTypeAIGatewayConfigStore,
	ResourceTypeAIGatewayVault,
	ResourceTypeAIGatewayDataPlaneCertificate,
	ResourceTypeAIGatewayCertificate,
	ResourceTypeAIGatewayCACertificate,
	ResourceTypeAIGatewaySNI,
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
		relationshipDescriptors[resourceType] = append(relationshipDescriptors[resourceType], RelationshipDescriptor{
			FieldPath: SchemaFieldAIGateway, TargetType: ResourceTypeAIGateway,
			Kind: RelationshipKindKongctlParentSelector, RootOnly: true,
		})
	}
	relationshipDescriptors[ResourceTypeAIGatewayConfigStoreSecret] = []RelationshipDescriptor{{
		FieldPath: SchemaFieldAIGatewayConfigStore, TargetType: ResourceTypeAIGatewayConfigStore,
		Kind: RelationshipKindKongctlParentSelector, RootOnly: true,
	}}
	relationshipDescriptors[ResourceTypeAIGatewayVault] = append(
		relationshipDescriptors[ResourceTypeAIGatewayVault],
		RelationshipDescriptor{
			FieldPath:  SchemaFieldConfig + "." + SchemaFieldConfigStoreID,
			TargetType: ResourceTypeAIGatewayConfigStore,
			Kind:       RelationshipKindAPIForeignKey,
		},
	)
	relationshipDescriptors[ResourceTypeAIGatewaySNI] = append(
		relationshipDescriptors[ResourceTypeAIGatewaySNI],
		RelationshipDescriptor{
			FieldPath: SchemaFieldCertificate, TargetType: ResourceTypeAIGatewayCertificate,
			Kind: RelationshipKindAPIForeignKey, ResultField: RelationshipResultFieldName,
			ScopeFieldPath: SchemaFieldAIGateway,
		},
	)
	for _, resourceType := range []ResourceType{
		ResourceTypeAIGatewayAgent,
		ResourceTypeAIGatewayModel,
		ResourceTypeAIGatewayMCPServer,
	} {
		relationshipDescriptors[resourceType] = append(
			relationshipDescriptors[resourceType],
			RelationshipDescriptor{
				FieldPath:      SchemaFieldAccess + "." + SchemaFieldAuthStrategies,
				TargetType:     ResourceTypeAIGatewayAuthStrategy,
				Kind:           RelationshipKindAPIForeignKey,
				Cardinality:    RelationshipCardinalityList,
				ScopeFieldPath: SchemaFieldAIGateway,
			},
		)
	}
	for _, resourceType := range portalChildTypes {
		relationshipDescriptors[resourceType] = append(relationshipDescriptors[resourceType], RelationshipDescriptor{
			FieldPath: SchemaFieldPortal, TargetType: ResourceTypePortal,
			Kind: RelationshipKindKongctlParentSelector, RootOnly: true,
		})
	}
	for _, resourceType := range eventGatewayChildTypes {
		relationshipDescriptors[resourceType] = append(relationshipDescriptors[resourceType], RelationshipDescriptor{
			FieldPath: SchemaFieldEventGateway, TargetType: ResourceTypeEventGatewayControlPlane,
			Kind: RelationshipKindKongctlParentSelector, RootOnly: true,
		})
	}
}

// RelationshipDescriptorsForType returns static relationship schema metadata.
func RelationshipDescriptorsForType(resourceType ResourceType) []RelationshipDescriptor {
	result := append([]RelationshipDescriptor(nil), relationshipDescriptors[resourceType]...)
	for i := range result {
		if result[i].Cardinality == "" {
			result[i].Cardinality = RelationshipCardinalityScalar
		}
		if result[i].ResultField == "" {
			result[i].ResultField = RelationshipResultFieldID
			if result[i].Kind == RelationshipKindKongctlParentSelector {
				result[i].ResultField = RelationshipResultFieldRef
			}
		}
	}
	return result
}

// RelationshipDescriptorsFor returns authoritative relationship metadata for a resource.
func RelationshipDescriptorsFor(resource Resource) []RelationshipDescriptor {
	return RelationshipDescriptorsForType(resource.GetType())
}

// RelationshipTargetTypes returns every target declared by relationship metadata.
func RelationshipTargetTypes() []ResourceType {
	seen := make(map[ResourceType]struct{})
	for _, descriptors := range relationshipDescriptors {
		for _, descriptor := range descriptors {
			if descriptor.TargetType != "" {
				seen[descriptor.TargetType] = struct{}{}
			}
			for _, targetType := range descriptor.TargetTypes {
				seen[targetType] = struct{}{}
			}
		}
	}
	result := make([]ResourceType, 0, len(seen))
	for resourceType := range seen {
		result = append(result, resourceType)
	}
	slices.Sort(result)
	return result
}
