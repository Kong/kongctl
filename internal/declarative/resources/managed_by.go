package resources

import "maps"

const (
	ManagedByToolKey    = "tool"
	ManagedByVersionKey = "version"
	ManagedByTool       = "kongctl"
)

// ManagedByWithProvenance stamps the writer without adopting the resource.
func ManagedByWithProvenance(metadata map[string]string, version string) map[string]string {
	result := maps.Clone(metadata)
	if result == nil {
		result = make(map[string]string)
	}
	if version == "" {
		version = "dev"
	}
	result[ManagedByToolKey] = ManagedByTool
	result[ManagedByVersionKey] = version
	return result
}

var managedByResourceTypes = map[ResourceType]struct{}{
	ResourceTypeAIGatewayProvider: {}, ResourceTypeAIGatewayAuthStrategy: {},
	ResourceTypeAIGatewayAgent: {}, ResourceTypeAIGatewayConsumer: {},
	ResourceTypeAIGatewayConsumerCredential: {}, ResourceTypeAIGatewayConsumerGroup: {},
	ResourceTypeAIGatewayModel: {}, ResourceTypeAIGatewayPolicy: {},
	ResourceTypeAIGatewayCustomPolicy: {}, ResourceTypeAIGatewayMCPServer: {},
	ResourceTypeAIGatewayVault: {}, ResourceTypeAIGatewayCertificate: {},
	ResourceTypeAIGatewayCACertificate: {}, ResourceTypeAIGatewaySNI: {},
}

// SupportsManagedBy describes API payload support, independently of ownership
// enrollment. Config stores and data plane certificates have no such field.
func SupportsManagedBy(kind ResourceType) bool {
	_, ok := managedByResourceTypes[kind]
	return ok
}
