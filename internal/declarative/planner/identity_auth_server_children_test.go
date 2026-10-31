package planner

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/loader"
	"github.com/kong/kongctl/internal/declarative/state"
	fixture "github.com/kong/kongctl/test/konnect"
	"github.com/stretchr/testify/require"
)

func planAuthChildren(t *testing.T, input string, roots *fixture.AuthServerAPI,
	children *fixture.AuthServerChildrenAPI, mode PlanMode,
) (*Plan, error) {
	t.Helper()
	file := filepath.Join(t.TempDir(), "manifest.yaml")
	require.NoError(t, os.WriteFile(file, []byte(input), 0o600))
	rs, err := loader.New().LoadFile(file)
	if err != nil {
		return nil, err
	}
	client := state.NewClient(state.ClientConfig{
		IdentityAuthServersAPI:      roots,
		IdentityAuthServerScopesAPI: children, IdentityAuthServerClaimsAPI: children, IdentityAuthServerClientsAPI: children,
	})
	return NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: mode})
}

const authChildParent = `_defaults:
  kongctl: {namespace: auth-team}
identity:
  auth_servers:
    - ref: issuer
      name: issuer
      audience: https://api.example.test
`

func TestAuthChildrenSamePlanReferencesAndDeleteOrdering(t *testing.T) {
	roots := &fixture.AuthServerAPI{}
	children := &fixture.AuthServerChildrenAPI{}
	input := authChildParent + `      scopes:
        - {ref: read, name: read:data, enabled: false}
      claims:
        - ref: department
          name: department
          value: '${ uuidv4 }'
          include_in_scopes: [!ref read#id]
`
	plan, err := planAuthChildren(t, input, roots, children, PlanModeApply)
	require.NoError(t, err)
	require.Len(t, plan.Changes, 3)
	require.Equal(t, ResourceTypeIdentityAuthServer, plan.Changes[0].ResourceType)
	require.Equal(t, ResourceTypeIdentityAuthServerScope, plan.Changes[1].ResourceType)
	require.Equal(t, ResourceTypeIdentityAuthServerClaim, plan.Changes[2].ResourceType)
	require.Contains(t, plan.Changes[2].DependsOn, plan.Changes[1].ID)
	root := fixture.AuthServer("issuer", "auth-team", false)
	roots.Servers = []kkComps.AuthServer{root}
	children.Scopes = map[string][]kkComps.AuthServerScope{root.ID: {{ID: "scope-id", Name: "read:data"}}}
	children.Claims = map[string][]kkComps.Claim{root.ID: {{
		ID: "claim-id", Name: "department", Value: "value",
		IncludeInScopes: []string{"scope-id"},
	}}}
	plan, err = planAuthChildren(t, input, roots, children, PlanModeDelete)
	require.NoError(t, err)
	require.Len(t, plan.Changes, 3)
	var rootDelete, scopeDelete, claimDelete PlannedChange
	for _, change := range plan.Changes {
		switch change.ResourceType {
		case ResourceTypeIdentityAuthServer:
			rootDelete = change
		case ResourceTypeIdentityAuthServerScope:
			scopeDelete = change
		case ResourceTypeIdentityAuthServerClaim:
			claimDelete = change
		}
	}
	require.Contains(t, scopeDelete.DependsOn, claimDelete.ID)
	require.Contains(t, rootDelete.DependsOn, scopeDelete.ID)
}

func TestScopeSyncRequiresExplicitCollectionAndRejectsRetainedReferences(t *testing.T) {
	root := fixture.AuthServer("issuer", "auth-team", false)
	roots := &fixture.AuthServerAPI{Servers: []kkComps.AuthServer{root}}
	children := &fixture.AuthServerChildrenAPI{
		Scopes: map[string][]kkComps.AuthServerScope{root.ID: {{ID: "scope-id", Name: "read:data"}}},
	}
	plan, err := planAuthChildren(t, authChildParent, roots, children, PlanModeSync)
	require.NoError(t, err)
	require.Empty(t, plan.Changes)
	children.Claims = map[string][]kkComps.Claim{root.ID: {{
		ID: "foreign-claim", Name: "keep",
		IncludeInScopes: []string{"scope-id"},
	}}}
	_, err = planAuthChildren(t, authChildParent+"      scopes: []\n", roots, children, PlanModeSync)
	require.ErrorContains(t, err, "still referenced by claim")
	children.Claims = nil
	children.Clients = map[string][]kkComps.AuthServerClient{root.ID: {{
		ID: "opaque-client", Name: "keep",
		AllowScopes: []string{"scope-id"},
	}}}
	_, err = planAuthChildren(t, authChildParent+"      scopes: []\n", roots, children, PlanModeSync)
	require.ErrorContains(t, err, "still referenced by client")
	children.Clients = nil
	plan, err = planAuthChildren(t, authChildParent+"      scopes: []\n", roots, children, PlanModeSync)
	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)
	roots.Servers[0].Labels["KONGCTL-protected"] = "true"
	_, err = planAuthChildren(t, authChildParent+"      scopes: []\n", roots, children, PlanModeSync)
	require.ErrorContains(t, err, "protected")
}
