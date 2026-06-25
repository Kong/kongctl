package resources

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/stretchr/testify/require"
)

const aiGatewayModelJSON = `{
  "ref": "support-gpt",
  "ai_gateway": "support-gateway",
  "type": "model",
  "name": "support-gpt",
  "display_name": "Support GPT",
  "enabled": true,
  "config": {
    "route": {},
    "model": {}
  },
  "formats": [{"type": "openai"}],
  "target_models": [{
    "name": "gpt-4o",
    "provider": "support-openai",
    "config": {"type": "openai"}
  }],
  "policies": [],
  "capabilities": ["generate"]
}`

const aiGatewayAPIModelJSON = `{
  "ref": "support-files",
  "ai_gateway": "support-gateway",
  "type": "api",
  "name": "support-files",
  "display_name": "Support Files",
  "enabled": true,
  "config": {
    "route": {},
    "model": {}
  },
  "formats": [{"type": "openai"}],
  "target_models": [{
    "name": "gpt-4o",
    "provider": "support-openai",
    "config": {"type": "openai"}
  }],
  "policies": [],
  "capabilities": ["files"]
}`

func TestAIGatewayModelResourceUnmarshalModelVariant(t *testing.T) {
	var model AIGatewayModelResource
	require.NoError(t, json.Unmarshal([]byte(aiGatewayModelJSON), &model))

	require.Equal(t, "support-gpt", model.Ref)
	require.Equal(t, "support-gateway", model.AIGateway)
	require.Equal(t, "model", model.ModelType())
	require.Equal(t, "support-gpt", model.Name())
	require.NotNil(t, model.AIGatewayModelModel)
	require.Nil(t, model.AIGatewayModelAPI)
	require.NoError(t, model.Validate())
}

func TestAIGatewayModelResourceUnmarshalAPIVariant(t *testing.T) {
	var model AIGatewayModelResource
	require.NoError(t, json.Unmarshal([]byte(aiGatewayAPIModelJSON), &model))

	require.Equal(t, "support-files", model.Ref)
	require.Equal(t, "support-gateway", model.AIGateway)
	require.Equal(t, "api", model.ModelType())
	require.Equal(t, "support-files", model.Name())
	require.NotNil(t, model.AIGatewayModelAPI)
	require.Nil(t, model.AIGatewayModelModel)
	require.NoError(t, model.Validate())
}

func TestAIGatewayModelResourceRejectsKongctlMetadata(t *testing.T) {
	payload := strings.Replace(aiGatewayModelJSON, `"ai_gateway": "support-gateway",`,
		`"ai_gateway": "support-gateway", "kongctl": {"namespace": "default"},`, 1)

	var model AIGatewayModelResource
	err := json.Unmarshal([]byte(payload), &model)
	require.Error(t, err)
	require.Contains(t, err.Error(), "kongctl metadata")
}

func TestAIGatewayModelResourceMarshalPreservesParentAndPayload(t *testing.T) {
	var model AIGatewayModelResource
	require.NoError(t, json.Unmarshal([]byte(aiGatewayModelJSON), &model))

	data, err := json.Marshal(model)
	require.NoError(t, err)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(data, &payload))
	require.Equal(t, "support-gpt", payload["ref"])
	require.Equal(t, "support-gateway", payload["ai_gateway"])
	require.Equal(t, "model", payload["type"])
	require.Equal(t, "support-gpt", payload["name"])
	require.NotContains(t, payload, "id")
}

func TestAIGatewayModelResourceParentRefNormalizesDeferredRef(t *testing.T) {
	var model AIGatewayModelResource
	require.NoError(t, json.Unmarshal([]byte(aiGatewayModelJSON), &model))
	model.AIGateway = tags.RefPlaceholderPrefix + "support-gateway#id"

	parent := model.GetParentRef()
	require.NotNil(t, parent)
	require.Equal(t, ResourceTypeAIGateway, parent.Kind)
	require.Equal(t, "support-gateway", parent.Ref)

	deps := model.GetDependencies()
	require.Len(t, deps, 1)
	require.Equal(t, "support-gateway", deps[0].Ref)
}
