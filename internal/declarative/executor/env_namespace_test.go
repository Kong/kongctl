package executor

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/loader"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestEnvNamespaceSavedPlanPreservesOwnership(t *testing.T) {
	t.Setenv("PLAN_NAMESPACE", "team-prod")
	t.Setenv("PLAN_DESCRIPTION", "reviewed description")
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`_defaults:
  kongctl:
    namespace: !env PLAN_NAMESPACE
portals:
  - ref: portal
    name: prod-portal
    description: !env PLAN_DESCRIPTION
`), 0o600))
	rs, err := loader.New().LoadFromSources([]loader.Source{{Path: path, Type: loader.SourceTypeFile}}, false)
	require.NoError(t, err)
	api := new(MockPortalAPI)
	devNamespace := "team-dev"
	api.On("ListPortals", mock.Anything, mock.Anything).Return(&kkOps.ListPortalsResponse{
		ListPortalsResponse: &kkComps.ListPortalsResponse{
			Data: []kkComps.ListPortalsResponsePortal{{
				ID: "dev-id", Name: "dev-portal", Labels: map[string]string{labels.NamespaceKey: devNamespace},
			}},
			Meta: kkComps.PaginatedMeta{Page: kkComps.PageMeta{Total: 1}},
		},
	}, nil)
	client := state.NewClient(state.ClientConfig{PortalAPI: api})
	plan, err := planner.NewPlanner(client, slog.Default()).
		GeneratePlan(t.Context(), rs, planner.Options{Mode: planner.PlanModeSync})
	require.NoError(t, err)
	require.Len(t, plan.Changes, 1, "prod sync must not propose deleting the existing dev portal")
	require.Equal(t, planner.ActionCreate, plan.Changes[0].Action)
	require.Equal(t, "team-prod", plan.Changes[0].Namespace)
	saved, err := json.Marshal(plan)
	require.NoError(t, err)
	require.NotContains(t, string(saved), "PLAN_NAMESPACE")
	require.Contains(t, string(saved), "__ENV__:PLAN_DESCRIPTION")
	t.Setenv("PLAN_NAMESPACE", "team-dev")
	t.Setenv("PLAN_DESCRIPTION", "execution description")
	var restored planner.Plan
	require.NoError(t, json.Unmarshal(saved, &restored))
	exec := New(client, nil, false)
	require.NoError(t, exec.resolveDeferredEnvPlaceholders(&restored.Changes[0]))
	require.Equal(t, "team-prod", restored.Changes[0].Namespace)
	api.On("CreatePortal", mock.Anything, mock.MatchedBy(func(request kkComps.CreatePortal) bool {
		return request.Labels[labels.NamespaceKey] != nil && *request.Labels[labels.NamespaceKey] == "team-prod" &&
			request.Description != nil && *request.Description == "execution description"
	})).Return(&kkOps.CreatePortalResponse{PortalResponse: &kkComps.PortalResponse{ID: "prod-id"}}, nil).Once()
	_, err = exec.createResource(testContextWithLogger(), &restored.Changes[0])
	require.NoError(t, err)
	api.AssertExpectations(t)
}
