// Package secrets defines the reviewed declarative write-only field catalog.
package secrets

import (
	"strings"

	"github.com/kong/kongctl/internal/declarative/resources"
)

// Capability describes the supported lifecycle of a write-only field.
type Capability struct {
	ResourceType resources.ResourceType
	PathPattern  string
	Create       bool
	Update       bool
}

var capabilities = []Capability{
	{resources.ResourceTypePortalIdentityProvider, "/config/client_secret", true, true},
	{resources.ResourceTypeDCRProvider, "/dcr_config/initial_client_secret", true, true},
	{resources.ResourceTypeDCRProvider, "/dcr_config/dcr_token", true, true},
	{resources.ResourceTypeDCRProvider, "/dcr_config/api_key", true, true},
	{resources.ResourceTypeAIGatewayProvider, "/config/**/headers/*/value", true, true},
	{resources.ResourceTypeAIGatewayProvider, "/config/**/client_secret", true, true},
	{resources.ResourceTypeAIGatewayProvider, "/config/**/secret_access_key", true, true},
	{resources.ResourceTypeAIGatewayProvider, "/config/**/service_account_json", true, true},
	{resources.ResourceTypeAIGatewayIdentityProvider, "/config/client_secret/*", true, true},
	{resources.ResourceTypeAIGatewayIdentityProvider, "/config/client_secret", true, true},
	{resources.ResourceTypeAIGatewayVault, "/config/**/api_key", true, true},
	{resources.ResourceTypeAIGatewayVault, "/config/**/token", true, true},
	{resources.ResourceTypeAIGatewayVault, "/config/**/key", true, true},
	{resources.ResourceTypeAIGatewayVault, "/config/**/client_secret", true, true},
	{resources.ResourceTypeAIGatewayVault, "/config/**/secret_access_key", true, true},
	{resources.ResourceTypeAIGatewayVault, "/config/**/secret_id", true, true},
	{resources.ResourceTypeEventGatewaySchemaRegistry, "/config/authentication/password", true, true},
	{resources.ResourceTypeAIGatewayConsumerCredential, "/api_key", true, false},
}

// Match returns the reviewed capability for a concrete resource field path.
func Match(resourceType resources.ResourceType, fieldPath string) (Capability, bool) {
	for _, capability := range capabilities {
		if capability.ResourceType == resourceType && matchPath(capability.PathPattern, fieldPath) {
			return capability, true
		}
	}
	return Capability{}, false
}

// Capabilities returns a copy of the reviewed catalog.
func Capabilities() []Capability {
	return append([]Capability(nil), capabilities...)
}

func matchPath(pattern, value string) bool {
	patternSegments := splitPath(pattern)
	valueSegments := splitPath(value)
	return matchSegments(patternSegments, valueSegments)
}

func matchSegments(pattern, value []string) bool {
	if len(pattern) == 0 {
		return len(value) == 0
	}
	if pattern[0] == "**" {
		if matchSegments(pattern[1:], value) {
			return true
		}
		return len(value) > 0 && matchSegments(pattern, value[1:])
	}
	if len(value) == 0 || (pattern[0] != "*" && pattern[0] != value[0]) {
		return false
	}
	return matchSegments(pattern[1:], value[1:])
}

func splitPath(path string) []string {
	trimmed := strings.Trim(path, "/")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "/")
}
