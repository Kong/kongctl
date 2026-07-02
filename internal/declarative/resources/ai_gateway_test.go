package resources

import (
	"encoding/json"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/stretchr/testify/require"
	sigsyaml "sigs.k8s.io/yaml"
)

func TestAIGatewayResourceMarshalIncludesName(t *testing.T) {
	t.Parallel()

	description := "AI Gateway description"
	resource := AIGatewayResource{
		BaseResource: BaseResource{Ref: "ai-gateway"},
		CreateAIGatewayRequest: kkComps.CreateAIGatewayRequest{
			DisplayName: "AI Gateway",
			Name:        "ai-gateway-name",
			Description: &description,
			ProxyUrls: []kkComps.AIGatewayProxyURL{
				{Host: "proxy.example.com", Port: 443, Protocol: "https"},
			},
			Labels: map[string]string{"owner": "platform"},
		},
	}

	jsonBytes, err := json.Marshal(resource)
	require.NoError(t, err)

	var jsonPayload map[string]any
	require.NoError(t, json.Unmarshal(jsonBytes, &jsonPayload))
	requireAIGatewaySerializedPayload(t, jsonPayload)

	yamlBytes, err := sigsyaml.Marshal(resource)
	require.NoError(t, err)

	var yamlPayload map[string]any
	require.NoError(t, sigsyaml.Unmarshal(yamlBytes, &yamlPayload))
	requireAIGatewaySerializedPayload(t, yamlPayload)
}

func requireAIGatewaySerializedPayload(t *testing.T, payload map[string]any) {
	t.Helper()

	require.Equal(t, "ai-gateway", payload["ref"])
	require.Equal(t, "ai-gateway-name", payload["name"])
	require.Equal(t, "AI Gateway", payload["display_name"])
	require.Equal(t, "AI Gateway description", payload["description"])
	require.Equal(t, map[string]any{"owner": "platform"}, payload["labels"])
	require.NotContains(t, payload, "additionalProperties")

	proxyURLs, ok := payload["proxy_urls"].([]any)
	require.True(t, ok, "expected proxy_urls array, got %T", payload["proxy_urls"])
	require.Len(t, proxyURLs, 1)
	proxyURL, ok := proxyURLs[0].(map[string]any)
	require.True(t, ok, "expected proxy_url object, got %T", proxyURLs[0])
	require.Equal(t, "proxy.example.com", proxyURL["host"])
	require.EqualValues(t, 443, proxyURL["port"])
	require.Equal(t, "https", proxyURL["protocol"])
}

func TestAIGatewayResourceSetDefaultsUsesRefOnlyWhenNameOmitted(t *testing.T) {
	t.Parallel()

	resource := AIGatewayResource{
		BaseResource: BaseResource{Ref: "local-ref"},
		CreateAIGatewayRequest: kkComps.CreateAIGatewayRequest{
			Name: "api-name",
		},
	}

	resource.SetDefaults()

	require.Equal(t, "api-name", resource.Name)
	require.Equal(t, "api-name", resource.DisplayName)

	resource = AIGatewayResource{BaseResource: BaseResource{Ref: "local-ref"}}
	resource.SetDefaults()

	require.Equal(t, "local-ref", resource.Name)
	require.Equal(t, "local-ref", resource.DisplayName)
}

func TestAIGatewayResourceAllowsExternalWithoutDisplayName(t *testing.T) {
	t.Parallel()

	resource := AIGatewayResource{
		BaseResource: BaseResource{Ref: "external-ai-gateway"},
		External: &ExternalBlock{
			Selector: &ExternalSelector{
				MatchFields: map[string]string{"display_name": "Shared AI Gateway"},
			},
		},
	}

	require.True(t, resource.IsExternal())
	require.NoError(t, resource.Validate())
	require.Empty(t, resource.GetKonnectMonikerFilter())
}
