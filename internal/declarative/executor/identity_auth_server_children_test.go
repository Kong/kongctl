package executor

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"

	"github.com/kong/kongctl/internal/declarative/loader"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
	logctx "github.com/kong/kongctl/internal/log"
	fixture "github.com/kong/kongctl/test/konnect"
	"github.com/stretchr/testify/require"
)

func TestAuthChildSavedPlanLifecycleAndFreshOwnershipChecks(t *testing.T) {
	roots := &fixture.AuthServerAPI{}
	children := &fixture.AuthServerChildrenAPI{}
	client := state.NewClient(state.ClientConfig{
		IdentityAuthServersAPI:      roots,
		IdentityAuthServerScopesAPI: children, IdentityAuthServerClaimsAPI: children, IdentityAuthServerClientsAPI: children,
	})
	manifest := `_defaults:
  kongctl: {namespace: auth-team}
identity:
  auth_servers:
    - ref: issuer
      name: issuer
      audience: https://api.example.test
      scopes:
        - ref: read
          name: read:data
          description: Keep this description
          default: true
          include_in_metadata: true
          enabled: false
      claims:
        - ref: department
          name: department
          value: '${ uuidv4 }'
          include_in_token: true
          include_in_all_scopes: false
          include_in_scopes: [!ref read#id]
          enabled: true
`
	file := filepath.Join(t.TempDir(), "manifest.yaml")
	loadPlan := func(input string, mode planner.PlanMode) *planner.Plan {
		t.Helper()
		require.NoError(t, os.WriteFile(file, []byte(input), 0o600))
		rs, err := loader.New().LoadFile(file)
		require.NoError(t, err)
		plan, err := planner.NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, planner.Options{Mode: mode})
		require.NoError(t, err)
		encoded, err := json.Marshal(plan)
		require.NoError(t, err)
		var saved planner.Plan
		require.NoError(t, json.Unmarshal(encoded, &saved))
		return &saved
	}
	execute := func(plan *planner.Plan) *ExecutionResult {
		t.Helper()
		ctx := context.WithValue(t.Context(), logctx.LoggerKey, slog.Default())
		return NewWithOptions(client, nil, false, Options{MaxConcurrency: 1}).Execute(ctx, plan)
	}
	plan := loadPlan(manifest, planner.PlanModeApply)
	require.Len(t, plan.Changes, 3)
	result := execute(plan)
	require.Zero(t, result.FailureCount, "%+v", result.Errors)
	parentID := roots.Servers[0].ID
	require.Len(t, children.Scopes[parentID], 1)
	require.Len(t, children.Claims[parentID], 1)
	require.Equal(t, children.Scopes[parentID][0].ID, children.Claims[parentID][0].IncludeInScopes[0])
	require.Equal(t, "${ uuidv4 }", children.Claims[parentID][0].Value)
	require.Empty(t, loadPlan(manifest, planner.PlanModeApply).Changes)
	updated := strings.Replace(manifest, "include_in_scopes: [!ref read#id]", "include_in_scopes: []", 1)
	updated = strings.Replace(updated, "value: '${ uuidv4 }'", "value: '{\"team\":\"platform\"}'", 1)
	result = execute(loadPlan(updated, planner.PlanModeApply))
	require.Zero(t, result.FailureCount, "%+v", result.Errors)
	require.Empty(t, children.Claims[parentID][0].IncludeInScopes)
	require.Equal(t, "Keep this description", children.Scopes[parentID][0].Description)
	require.Empty(t, loadPlan(updated, planner.PlanModeApply).Changes)
	deletePlan := loadPlan(manifest, planner.PlanModeDelete)
	oldWrites := children.Writes
	roots.Servers[0].Labels["KONGCTL-namespace"] = "foreign"
	result = execute(deletePlan)
	require.Positive(t, result.FailureCount)
	require.Equal(t, oldWrites, children.Writes, "saved writes must recheck parent ownership")
	roots.Servers[0].Labels["KONGCTL-namespace"] = "auth-team"
	children.Clients = map[string][]kkComps.AuthServerClient{parentID: {{
		ID: "opaque-client", Name: "late-client",
		AllowScopes: []string{children.Scopes[parentID][0].ID},
	}}}
	result = execute(deletePlan)
	require.Positive(t, result.FailureCount, "fresh client references must block a saved scope deletion")
	require.Len(t, children.Scopes[parentID], 1)
	require.Equal(t, 0, roots.Deletes)
	children.Clients = nil
	result = execute(deletePlan)
	require.Zero(t, result.FailureCount, "%+v", result.Errors)
	require.Empty(t, children.Claims[parentID])
	require.Empty(t, children.Scopes[parentID])
	require.Equal(t, 1, roots.Deletes)
}
