package executor

import (
	"bytes"
	"encoding/json"
	"io"
	"maps"
	"net/http"
	"testing"

	sdk "github.com/Kong/sdk-konnect-go"
	c "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/stretchr/testify/require"
)

func TestAIGatewayOIDCConfigPresenceThroughSavedPlanAndMetadataStamp(t *testing.T) {
	for _, tc := range []struct {
		name  string
		extra map[string]any
	}{
		{name: "omitted"},
		{name: "null", extra: map[string]any{"upstream_access_token_header": nil}},
		{name: "explicit-default", extra: map[string]any{"upstream_access_token_header": "authorization:bearer"}},
		{name: "custom", extra: map[string]any{"scopes": []any{}, "upstream_access_token_header": "x-custom-token"}},
	} {
		for _, verb := range []string{"create", "update"} {
			t.Run(verb+"/"+tc.name, func(t *testing.T) {
				config := map[string]any{
					"issuer": "https://example.com", "cache_tokens_salt": "test-salt",
					"auth_methods": []any{"bearer"}, "hide_credentials": false,
				}
				maps.Copy(config, tc.extra)
				change := planner.PlannedChange{
					ResourceType: planner.ResourceTypeAIGatewayAuthStrategy,
					Namespace:    "presence", ResourceID: "probe", Parent: &planner.ParentInfo{ID: "gateway"}, Fields: map[string]any{
						planner.FieldName: "probe", planner.FieldDisplayName: "Probe", planner.FieldType: "openid-connect",
						planner.FieldConfig: config, planner.FieldManagedBy: resources.BuildChildManagedBy("presence", false, "dev"),
					},
				}
				data, err := json.Marshal(change)
				require.NoError(t, err)
				var saved planner.PlannedChange
				require.NoError(t, json.Unmarshal(data, &saved))
				observed := maps.Clone(saved.Fields)
				observed[planner.FieldID] = "probe"
				observed[planner.FieldDisplayName] = "Previous name"
				if tc.name == "null" {
					currentConfig := maps.Clone(config)
					currentConfig["upstream_access_token_header"] = "authorization:bearer"
					observed[planner.FieldConfig] = currentConfig
				}
				observed["created_at"] = "2026-11-11T00:00:00Z"
				observed["updated_at"] = "2026-11-11T00:00:00Z"
				response, err := json.Marshal(observed)
				require.NoError(t, err)
				capture := &oidcExecutionCapture{response: response}
				client := sdk.New(sdk.WithClient(capture), sdk.WithServerURL("https://probe.invalid"))
				stateClient := state.NewClient(state.ClientConfig{AIGatewayAuthStrategiesAPI: client.AIGatewayAuthStrategies})
				adapter := NewAIGatewayAuthStrategyAdapter(stateClient)
				if verb == "create" {
					var request c.CreateAIGatewayAuthStrategyRequest
					require.NoError(t, adapter.MapCreateFields(t.Context(), nil, saved.Fields, &request))
					_, err = stateClient.CreateAIGatewayAuthStrategy(t.Context(), "gateway", request, "presence")
				} else {
					var request c.UpdateAIGatewayAuthStrategyRequest
					require.NoError(t, adapter.MapUpdateFields(t.Context(), NewExecutionContext(&saved), saved.Fields, &request, nil))
					_, err = adapter.Update(t.Context(), "probe", request, "presence", NewExecutionContext(&saved))
				}
				require.Error(t, err) // The capture client returns a synthetic HTTP 400 after recording the request.
				require.NotEmpty(t, capture.body)
				var body map[string]any
				require.NoError(t, json.Unmarshal(capture.body, &body))
				wireConfig := body[planner.FieldConfig].(map[string]any)
				header, present := wireConfig["upstream_access_token_header"]
				expectedHeader, expectedPresent := config["upstream_access_token_header"]
				require.Equal(t, expectedPresent, present)
				require.Equal(t, expectedHeader, header)
				require.Equal(t, false, wireConfig["hide_credentials"])
				require.Equal(t, []any{"bearer"}, wireConfig["auth_methods"])
				require.Equal(t, float64(3600), wireConfig["cache_ttl"])
				require.NotContains(t, wireConfig, "scopes") // Existing SDK omitempty behavior for empty arrays.
				require.Contains(t, body, planner.FieldManagedBy)
				if verb == "update" {
					require.Equal(t, 1, capture.reads)
				}
			})
		}
	}
}

type oidcExecutionCapture struct {
	realmsCapture
	response []byte
	reads    int
}

func (c *oidcExecutionCapture) Do(req *http.Request) (*http.Response, error) {
	if req.Method == http.MethodGet {
		c.reads++
		return &http.Response{
			StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/json"}},
			Body: io.NopCloser(bytes.NewReader(c.response)), Request: req,
		}, nil
	}
	return c.realmsCapture.Do(req)
}

func TestOIDCConfigValidationRejectsMalformedKnownFields(t *testing.T) {
	for _, field := range []string{"issuer", "cache_tokens_salt", "hide_credentials"} {
		t.Run(field, func(t *testing.T) {
			config := map[string]any{"issuer": "https://example.com", "cache_tokens_salt": "salt"}
			config[field] = map[string]any{"invalid": "sensitive-value-must-not-leak"}
			fields := map[string]any{
				planner.FieldName: "probe", planner.FieldDisplayName: "Probe",
				planner.FieldType: string(c.CreateAIGatewayAuthStrategyRequestTypeOpenidConnect), planner.FieldConfig: config,
			}
			adapter := NewAIGatewayAuthStrategyAdapter(nil)
			var create c.CreateAIGatewayAuthStrategyRequest
			var update c.UpdateAIGatewayAuthStrategyRequest
			for _, err := range []error{
				adapter.MapCreateFields(t.Context(), nil, fields, &create),
				adapter.MapUpdateFields(t.Context(), nil, fields, &update, nil),
			} {
				require.ErrorContains(t, err, "current SDK rejected")
				require.NotContains(t, err.Error(), "sensitive-value-must-not-leak")
			}
		})
	}
}
