package executor

import (
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/stretchr/testify/require"
)

func TestAIGatewayConsumerGroupAdapterRejectsAdditionalProperties(t *testing.T) {
	t.Parallel()

	fields := map[string]any{
		planner.FieldName:        "premium-users",
		planner.FieldDisplayName: "Premium Users",
		"future_group_field":     "group-value",
	}
	var request kkComps.CreateAIGatewayConsumerGroupRequest
	err := NewAIGatewayConsumerGroupAdapter(nil).MapCreateFields(t.Context(), nil, fields, &request)
	require.Error(t, err)
	require.Contains(t, err.Error(), "future_group_field")
}

func TestAIGatewayMCPServerAdapterRejectsAdditionalProperties(t *testing.T) {
	t.Parallel()

	fields := map[string]any{
		planner.FieldType:        "conversion-only",
		planner.FieldName:        "support-tools",
		planner.FieldDisplayName: "Support Tools",
		planner.FieldConfig: map[string]any{
			"url": "https://support-tools.example.com",
		},
		planner.FieldTools: []any{map[string]any{
			"name":        "lookup-customer",
			"description": "Look up a customer",
			"method":      "GET",
		}},
		"future_mcp_server_field": "mcp-server-value",
	}
	var request kkComps.CreateAIGatewayMCPServerRequest
	err := NewAIGatewayMCPServerAdapter(nil).MapCreateFields(t.Context(), nil, fields, &request)
	require.Error(t, err)
	require.Contains(t, err.Error(), "future_mcp_server_field")
}

func TestAIGatewayMCPServerAdapterCombinedACLs(t *testing.T) {
	for _, tc := range []struct {
		name, location string
		allow, deny    []string
		valid          bool
	}{
		{"server", "server", []string{"support"}, []string{"blocked"}, false},
		{"empty_server", "server", []string{}, []string{}, false},
		{"tool", "tool", []string{"support"}, []string{"blocked"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fields := map[string]any{
				planner.FieldType:        "passthrough-listener",
				planner.FieldName:        "support",
				planner.FieldDisplayName: "Support",
				planner.FieldConfig: map[string]any{
					"url": "https://example.com", "route": map[string]any{"paths": []string{"/mcp"}},
				},
			}
			access := map[string]any{"acls": map[string]any{"allow": tc.allow, "deny": tc.deny}}
			if tc.location == "server" {
				fields[planner.FieldAccess] = access
			} else {
				fields[planner.FieldTools] = []any{map[string]any{"name": "lookup", "access": access}}
			}
			adapter := NewAIGatewayMCPServerAdapter(nil)
			var create kkComps.CreateAIGatewayMCPServerRequest
			err := adapter.MapCreateFields(t.Context(), nil, fields, &create)
			if tc.valid {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, "acls must specify exactly one")
			}
			var update kkComps.UpdateAIGatewayMCPServerRequest
			err = adapter.MapUpdateFields(t.Context(), nil, fields, &update, nil)
			if tc.valid {
				require.NoError(t, err)
				for _, request := range []any{create, update} {
					payload, err := ownedChildPayload(request)
					require.NoError(t, err)
					tool := payload[planner.FieldTools].([]any)[0].(map[string]any)
					acls := tool["access"].(map[string]any)["acls"].(map[string]any)
					require.Equal(t, []any{"support"}, acls["allow"])
					require.Equal(t, []any{"blocked"}, acls["deny"])
				}
			} else {
				require.ErrorContains(t, err, "acls must specify exactly one")
			}
		})
	}
}
