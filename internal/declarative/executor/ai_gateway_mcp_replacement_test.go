package executor

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	kkSDK "github.com/Kong/sdk-konnect-go"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/stretchr/testify/require"
)

func TestSavedMCPReplacementCreateFailureKeepsOldServer(t *testing.T) {
	var methods []string
	httpClient := customPolicyHTTPClient(func(r *http.Request) (*http.Response, error) {
		methods = append(methods, r.Method)
		return &http.Response{
			StatusCode: http.StatusBadRequest,
			Header:     http.Header{"Content-Type": {"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"message":"replacement rejected"}`)), Request: r,
		}, nil
	})
	sdk := &helpers.KonnectSDK{SDK: kkSDK.New(kkSDK.WithServerURL("https://example.test"), kkSDK.WithClient(httpClient))}
	client := state.NewClient(state.ClientConfig{AIGatewayMCPServersAPI: sdk.GetAIGatewayMCPServersAPI()})
	plan := planner.NewPlan(planner.CurrentPlanVersion, "test", planner.PlanModeSync)
	plan.AddChange(planner.PlannedChange{
		ID: "create-new", ResourceType: planner.ResourceTypeAIGatewayMCPServer,
		ResourceRef: "new", Action: planner.ActionCreate, Parent: &planner.ParentInfo{ID: "gateway-id"},
		Fields: map[string]any{
			planner.FieldType: "passthrough-listener", planner.FieldName: "new",
			planner.FieldDisplayName: "New", planner.FieldConfig: map[string]any{
				"url": "https://mcp.example.test", planner.FieldRoute: map[string]any{"paths": []any{"/mcp"}},
			},
		},
	})
	plan.AddChange(planner.PlannedChange{
		ID: "delete-old", ResourceType: planner.ResourceTypeAIGatewayMCPServer,
		ResourceRef: "old", ResourceID: "old-id", Action: planner.ActionDelete,
		Parent: &planner.ParentInfo{ID: "gateway-id"}, DependsOn: []string{"create-new"},
	})
	resolved, err := planner.NewDependencyResolver().ResolveDependenciesWithGroups(plan.Changes)
	require.NoError(t, err)
	plan.ExecutionOrder = resolved.ExecutionOrder
	plan.SetExecutionGroups(resolved.ExecutionGroups)
	data, err := json.Marshal(plan)
	require.NoError(t, err)
	var saved planner.Plan
	require.NoError(t, json.Unmarshal(data, &saved))
	result := New(client, nil, false).Execute(testContextWithLogger(), &saved)
	require.Equal(t, 1, result.FailureCount)
	require.Equal(t, 1, result.SkippedCount)
	require.Equal(t, []string{http.MethodPost}, methods, "old server must not be read or deleted after create fails")
	require.Len(t, result.Errors, 1)
	require.Contains(t, result.Errors[0].Error, "replacement rejected")
}
