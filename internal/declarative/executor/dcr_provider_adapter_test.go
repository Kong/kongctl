package executor

import (
	"context"
	"encoding/json"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDCRProviderAdapterMapUpdateFieldsBuildsHTTPConfigUnion(t *testing.T) {
	adapter := NewDCRProviderAdapter(nil)
	execCtx := &ExecutionContext{Namespace: "test"}
	fields := map[string]any{
		planner.FieldName:                  "http-dcr",
		planner.FieldDCRProviderUpdateType: "http",
		planner.FieldDCRProviderConfig: map[string]any{
			"dcr_base_url": "https://example.com/v2/dcr",
			"api_key":      "test_api_key",
		},
	}

	var update kkComps.UpdateDcrProviderRequest
	err := adapter.MapUpdateFields(context.Background(), execCtx, fields, &update, nil)
	require.NoError(t, err)
	require.NotNil(t, update.DcrConfig)
	require.NotNil(t, update.DcrConfig.UpdateDcrConfigHTTPInRequest)
	assert.Nil(t, update.DcrConfig.UpdateDcrConfigAuth0InRequest)
	assert.Equal(t, "https://example.com/v2/dcr", *update.DcrConfig.UpdateDcrConfigHTTPInRequest.DcrBaseURL)
	assert.Equal(t, "test_api_key", *update.DcrConfig.UpdateDcrConfigHTTPInRequest.APIKey)
}

func TestDCRProviderAdapterMapUpdateFieldsKeepsHTTPPatchPayloadSparse(t *testing.T) {
	adapter := NewDCRProviderAdapter(nil)
	fields := map[string]any{
		planner.FieldName:                  "http-dcr",
		planner.FieldDCRProviderUpdateType: "http",
		planner.FieldDCRProviderConfig: map[string]any{
			"dcr_base_url": "https://example.com/v2/dcr",
		},
	}

	var update kkComps.UpdateDcrProviderRequest
	err := adapter.MapUpdateFields(t.Context(), &ExecutionContext{Namespace: "test"}, fields, &update, nil)
	require.NoError(t, err)
	require.NotNil(t, update.DcrConfig)
	require.NotNil(t, update.DcrConfig.UpdateDcrConfigHTTPInRequest)

	body, err := json.Marshal(update.DcrConfig.UpdateDcrConfigHTTPInRequest)
	require.NoError(t, err)
	assert.JSONEq(t, `{"dcr_base_url":"https://example.com/v2/dcr"}`, string(body))

	var payload map[string]any
	require.NoError(t, json.Unmarshal(body, &payload))
	assert.NotContains(t, payload, "allow_multiple_credentials")
}

func TestDCRProviderAdapterMapUpdateFieldsAppliesProtectionOnlyChange(t *testing.T) {
	adapter := NewDCRProviderAdapter(nil)
	execCtx := &ExecutionContext{
		Namespace:  "test",
		Protection: planner.ProtectionChange{Old: true, New: false},
	}
	currentLabels := map[string]string{
		labels.NamespaceKey: "test",
		labels.ProtectedKey: "true",
		"team":              "platform",
	}

	var update kkComps.UpdateDcrProviderRequest
	err := adapter.MapUpdateFields(t.Context(), execCtx, map[string]any{planner.FieldName: "http-dcr"}, &update,
		currentLabels)
	require.NoError(t, err)
	require.NotNil(t, update.Labels)
	assert.Nil(t, update.Labels[labels.ProtectedKey])
	require.NotNil(t, update.Labels[labels.NamespaceKey])
	assert.Equal(t, "test", *update.Labels[labels.NamespaceKey])
	require.NotNil(t, update.Labels["team"])
	assert.Equal(t, "platform", *update.Labels["team"])
}
