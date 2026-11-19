package executor

import (
	"context"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/stretchr/testify/require"
)

func TestAIGatewayMCPServerAdapterMapCreateFieldsRequiresExactlyOneOfURLOrTargets(t *testing.T) {
	t.Parallel()

	adapter := NewAIGatewayMCPServerAdapter(nil)
	fields := map[string]any{
		planner.FieldType:        "upstream-server",
		planner.FieldName:        "upstream",
		planner.FieldDisplayName: "Upstream",
		planner.FieldConfig: map[string]any{
			"tools_cache_ttl_seconds": 60,
			"route":                   map[string]any{"paths": []string{"/mcp"}},
		},
	}

	var req kkComps.CreateAIGatewayMCPServerRequest
	err := adapter.MapCreateFields(context.Background(), nil, fields, &req)
	require.Error(t, err)
	require.Contains(t, err.Error(), "exactly one of config.url or config.targets")
}

func TestAIGatewayMCPServerAdapterMapCreateFieldsRejectsBothURLAndTargets(t *testing.T) {
	t.Parallel()

	adapter := NewAIGatewayMCPServerAdapter(nil)
	fields := map[string]any{
		planner.FieldType:        "upstream-server",
		planner.FieldName:        "upstream",
		planner.FieldDisplayName: "Upstream",
		planner.FieldConfig: map[string]any{
			"url":                     "https://upstream.example.com",
			"targets":                 []map[string]any{{"address": "upstream-1.internal.kongair.com:443"}},
			"tools_cache_ttl_seconds": 60,
			"route":                   map[string]any{"paths": []string{"/mcp"}},
		},
	}

	var req kkComps.CreateAIGatewayMCPServerRequest
	err := adapter.MapCreateFields(context.Background(), nil, fields, &req)
	require.Error(t, err)
	require.Contains(t, err.Error(), "exactly one of config.url or config.targets")
}

func TestAIGatewayMCPServerAdapterMapCreateFieldsAcceptsTargetsOnly(t *testing.T) {
	t.Parallel()

	adapter := NewAIGatewayMCPServerAdapter(nil)
	fields := map[string]any{
		planner.FieldType:        "upstream-server",
		planner.FieldName:        "upstream",
		planner.FieldDisplayName: "Upstream",
		planner.FieldConfig: map[string]any{
			"targets":                 []map[string]any{{"address": "upstream-1.internal.kongair.com:443"}},
			"tools_cache_ttl_seconds": 60,
			"route":                   map[string]any{"paths": []string{"/mcp"}},
		},
	}

	var req kkComps.CreateAIGatewayMCPServerRequest
	require.NoError(t, adapter.MapCreateFields(context.Background(), nil, fields, &req))
	require.NotNil(t, req.AIGatewayMCPServerUpstreamServer)
	require.Len(t, req.AIGatewayMCPServerUpstreamServer.Config.Targets, 1)
}

func TestAIGatewayMCPServerAdapterMapCreateFieldsListenerTypeUnaffected(t *testing.T) {
	t.Parallel()

	adapter := NewAIGatewayMCPServerAdapter(nil)
	fields := map[string]any{
		planner.FieldType:        "listener",
		planner.FieldName:        "support-listener",
		planner.FieldDisplayName: "Support Listener",
		"sources":                []string{"support-tools"},
		planner.FieldConfig: map[string]any{
			"route": map[string]any{"paths": []string{"/support-listener"}},
		},
	}

	var req kkComps.CreateAIGatewayMCPServerRequest
	require.NoError(t, adapter.MapCreateFields(context.Background(), nil, fields, &req))
	require.NotNil(t, req.AIGatewayMCPServerListener)
}
