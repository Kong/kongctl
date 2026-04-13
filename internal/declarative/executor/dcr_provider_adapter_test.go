package executor

import (
	"context"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
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
