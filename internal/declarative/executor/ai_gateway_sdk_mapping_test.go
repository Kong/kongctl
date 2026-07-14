package executor

import (
	"errors"
	"testing"

	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/stretchr/testify/require"
)

var errAIGatewaySDKDecodeWithSecret = errors.New("SDK rejected secret-value-must-not-be-reported")

type rejectingAIGatewaySDKRequest struct{}

func (*rejectingAIGatewaySDKRequest) UnmarshalJSON([]byte) error {
	return errAIGatewaySDKDecodeWithSecret
}

func TestMapAIGatewaySDKRequestRejectsDiscardedNestedFields(t *testing.T) {
	t.Parallel()

	type auth struct {
		Type string `json:"type"`
	}
	type request struct {
		Config struct {
			Auth auth `json:"auth"`
		} `json:"config"`
	}

	payload := map[string]any{
		"config": map[string]any{
			"auth": map[string]any{
				"type":         "basic",
				"header_name":  "Authorization",
				"header_value": "secret-value-must-not-be-reported",
			},
		},
	}

	var destination request
	err := mapAIGatewaySDKRequest("AI Gateway test resource", payload, &destination)
	require.Error(t, err)
	require.ErrorContains(t, err, "config.auth.header_name")
	require.ErrorContains(t, err, "config.auth.header_value")
	require.NotContains(t, err.Error(), "secret-value-must-not-be-reported")
}

func TestMapAIGatewaySDKRequestAcceptsPreservedNestedFields(t *testing.T) {
	t.Parallel()

	type header struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	}
	type auth struct {
		Type    string   `json:"type"`
		Headers []header `json:"headers,omitempty"`
	}
	type request struct {
		Config struct {
			Auth auth `json:"auth"`
		} `json:"config"`
	}

	payload := map[string]any{
		"config": map[string]any{
			"auth": map[string]any{
				"type": "basic",
				"headers": []any{
					map[string]any{"name": "Authorization", "value": "Bearer token"},
				},
			},
		},
	}

	var destination request
	require.NoError(t, mapAIGatewaySDKRequest("AI Gateway test resource", payload, &destination))
	require.Equal(t, "Authorization", destination.Config.Auth.Headers[0].Name)
	require.Equal(t, "Bearer token", destination.Config.Auth.Headers[0].Value)
}

func TestMapAIGatewaySDKRequestExcludesResolvedPathFields(t *testing.T) {
	t.Parallel()

	type request struct {
		Name string `json:"name"`
	}

	payload := map[string]any{
		"name":                           "resource",
		planner.FieldAIGatewayID:         "gateway-id",
		planner.FieldAIGatewayConsumerID: "consumer-id",
	}

	var destination request
	require.NoError(t, mapAIGatewaySDKRequest("AI Gateway test resource", payload, &destination))
	require.Equal(t, "resource", destination.Name)
	require.Contains(t, payload, planner.FieldAIGatewayID)
	require.Contains(t, payload, planner.FieldAIGatewayConsumerID)
}

func TestMapAIGatewaySDKRequestIgnoresOmittedNilAndEmptyContainers(t *testing.T) {
	t.Parallel()

	type request struct {
		Name string `json:"name"`
	}

	payload := map[string]any{
		"name":         "resource",
		"optional":     nil,
		"empty_object": map[string]any{},
		"empty_array":  []any{},
	}

	var destination request
	require.NoError(t, mapAIGatewaySDKRequest("AI Gateway test resource", payload, &destination))
}

func TestMapAIGatewaySDKRequestDoesNotExposePayloadFromSDKDecodeErrors(t *testing.T) {
	t.Parallel()

	var destination rejectingAIGatewaySDKRequest
	err := mapAIGatewaySDKRequest(
		"AI Gateway test resource",
		map[string]any{"credential": "secret-value-must-not-be-reported"},
		&destination,
	)
	require.Error(t, err)
	require.ErrorIs(t, err, errAIGatewaySDKDecodeWithSecret)
	require.ErrorContains(t, err, "verify the resource fields with kongctl explain")
	require.NotContains(t, err.Error(), "secret-value-must-not-be-reported")
}
