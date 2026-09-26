package executor

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/loader"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestStoredEnvPlanExecutesApprovedValues(t *testing.T) {
	t.Setenv("STORED_EXEC_DESCRIPTION", "approved plaintext")
	t.Setenv("STORED_EXEC_ENABLED", "true")
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`portals:
  - ref: stored
    name: stored
    description: !env_store STORED_EXEC_DESCRIPTION
    auto_approve_developers: !env {var: STORED_EXEC_ENABLED, store: true}
`), 0o600))
	rs, err := loader.New().LoadFromSources([]loader.Source{{Path: path, Type: loader.SourceTypeFile}}, false)
	require.NoError(t, err)
	require.Empty(t, rs.EnvSources)
	api := new(MockPortalAPI)
	api.On("ListPortals", mock.Anything, mock.Anything).Return(&kkOps.ListPortalsResponse{
		ListPortalsResponse: &kkComps.ListPortalsResponse{
			Data: []kkComps.ListPortalsResponsePortal{},
			Meta: kkComps.PaginatedMeta{Page: kkComps.PageMeta{Total: 0}},
		},
	}, nil)
	client := state.NewClient(state.ClientConfig{PortalAPI: api})
	plan, err := planner.NewPlanner(client, slog.Default()).
		GeneratePlan(t.Context(), rs, planner.Options{Mode: planner.PlanModeApply})
	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)
	saved, err := json.Marshal(plan)
	require.NoError(t, err)
	require.Contains(t, string(saved), "approved plaintext")
	require.NotContains(t, string(saved), "__ENV__:")
	require.NotContains(t, string(saved), "STORED_EXEC")
	for _, absent := range []bool{false, true} {
		if absent {
			require.NoError(t, os.Unsetenv("STORED_EXEC_DESCRIPTION"))
			require.NoError(t, os.Unsetenv("STORED_EXEC_ENABLED"))
		} else {
			t.Setenv("STORED_EXEC_DESCRIPTION", "unapproved change")
			t.Setenv("STORED_EXEC_ENABLED", "false")
		}
		var restored planner.Plan
		require.NoError(t, json.Unmarshal(saved, &restored))
		exec := New(client, nil, false)
		require.NoError(t, exec.resolveDeferredEnvPlaceholders(&restored.Changes[0]))
		api.On("CreatePortal", mock.Anything, mock.MatchedBy(func(request kkComps.CreatePortal) bool {
			return request.Description != nil && *request.Description == "approved plaintext" &&
				request.AutoApproveDevelopers != nil && *request.AutoApproveDevelopers
		})).Return(&kkOps.CreatePortalResponse{PortalResponse: &kkComps.PortalResponse{ID: "stored-id"}}, nil).Once()
		_, err := exec.createPortal(testContextWithLogger(), restored.Changes[0])
		require.NoError(t, err)
	}
	api.AssertExpectations(t)
}

func TestStoredEnvStructuredPayloadRoundTrip(t *testing.T) {
	t.Setenv("STORED_EXEC_CONFIG", `{"redis":{"ssl":true,"ssl_verify":false},
"sync_rate":0.5,"values":[null,9007199254740991,"42",{},[]]}`)
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`ai_gateways:
  - ref: gateway
    name: gateway
    display_name: Gateway
    policies:
      - ref: rate
        name: rate
        display_name: Rate
        type: rate-limiting
        config: !env_store STORED_EXEC_CONFIG
`), 0o600))
	rs, err := loader.New().LoadFromSources([]loader.Source{{Path: path, Type: loader.SourceTypeFile}}, false)
	require.NoError(t, err)
	require.Empty(t, rs.EnvSources)
	policy := rs.AIGatewayPolicies[0]
	fields := map[string]any{
		planner.FieldName: policy.Name, planner.FieldDisplayName: policy.DisplayName,
		planner.FieldType: policy.Type, planner.FieldConfig: policy.Config,
	}
	change := planner.PlannedChange{Fields: fields, ChangedFields: map[string]planner.FieldChange{
		planner.FieldConfig: {Old: map[string]any{}, New: policy.Config},
	}}
	saved, err := json.Marshal(change)
	require.NoError(t, err)
	require.NoError(t, os.Unsetenv("STORED_EXEC_CONFIG"))
	var restored planner.PlannedChange
	require.NoError(t, json.Unmarshal(saved, &restored))
	require.NoError(t, New(nil, nil, false).resolveDeferredEnvPlaceholders(&restored))
	adapter := NewAIGatewayPolicyAdapter(nil)
	var create kkComps.CreateAIGatewayPolicyRequest
	require.NoError(t, adapter.MapCreateFields(t.Context(), &ExecutionContext{}, restored.Fields, &create))
	require.Equal(t, policy.Config, create.Config)
	var update kkComps.UpdateAIGatewayPolicyRequest
	require.NoError(t, adapter.MapUpdateFields(t.Context(), &ExecutionContext{}, restored.Fields, &update, nil))
	payload, err := json.Marshal(update)
	require.NoError(t, err)
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(payload, &decoded))
	require.Equal(t, policy.Config, decoded[planner.FieldConfig])
	require.Equal(t, policy.Config, restored.ChangedFields[planner.FieldConfig].New)
}
