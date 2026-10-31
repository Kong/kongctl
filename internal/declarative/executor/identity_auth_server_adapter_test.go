package executor

import (
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
	fixture "github.com/kong/kongctl/test/konnect"
	"github.com/stretchr/testify/require"
)

func authServerExecutorFixture() (*IdentityAuthServerAdapter, *fixture.AuthServerAPI, *ExecutionContext) {
	api := &fixture.AuthServerAPI{Servers: []kkComps.AuthServer{fixture.AuthServer("issuer", "auth-team", false)}}
	change := planner.PlannedChange{
		ResourceType: planner.ResourceTypeIdentityAuthServer, ResourceRef: "issuer", ResourceID: api.Servers[0].ID,
		Action: planner.ActionUpdate, Namespace: "auth-team", Protection: false,
		Fields: map[string]any{planner.FieldName: "issuer", planner.FieldDescription: "Updated"},
	}
	adapter := &IdentityAuthServerAdapter{client: state.NewClient(state.ClientConfig{IdentityAuthServersAPI: api})}
	return adapter, api, NewExecutionContext(&change)
}

func TestAuthServerPatchPreservesOmissionsAndClearsArrays(t *testing.T) {
	a, api, execCtx := authServerExecutorFixture()
	var request kkComps.UpdateAuthServer
	require.NoError(t, a.MapUpdateFields(t.Context(), execCtx, execCtx.PlannedChange.Fields, &request, nil))
	_, err := a.Update(t.Context(), api.Servers[0].ID, request, execCtx.Namespace, execCtx)
	require.NoError(t, err)
	require.Nil(t, api.Writes[0].Audience)
	require.Nil(t, api.Writes[0].Dcr)
	require.Nil(t, api.Writes[0].TrustedOrigins)
	require.Equal(t, "test", *api.Writes[0].Labels["owner"])
	execCtx.PlannedChange.Fields[planner.FieldTrustedOrigins] = []any{}
	require.NoError(t, a.MapUpdateFields(t.Context(), execCtx, execCtx.PlannedChange.Fields, &request, nil))
	_, err = a.Update(t.Context(), api.Servers[0].ID, request, execCtx.Namespace, execCtx)
	require.NoError(t, err)
	require.NotNil(t, api.Writes[1].TrustedOrigins)
	require.Empty(t, api.Servers[0].TrustedOrigins)
}

func TestAuthServerSavedWriteRechecksIdentityOwnershipAndProtection(t *testing.T) {
	for _, field := range []string{planner.FieldName, labels.NamespaceKey, labels.ProtectedKey} {
		a, api, execCtx := authServerExecutorFixture()
		switch field {
		case planner.FieldName:
			api.Servers[0].Name = "renamed"
		case labels.ProtectedKey:
			api.Servers[0].Labels[field] = "true"
		default:
			api.Servers[0].Labels[field] = "foreign"
		}
		var request kkComps.UpdateAuthServer
		require.NoError(t, a.MapUpdateFields(t.Context(), execCtx, execCtx.PlannedChange.Fields, &request, nil))
		_, err := a.Update(t.Context(), api.Servers[0].ID, request, execCtx.Namespace, execCtx)
		require.Error(t, err)
		require.Empty(t, api.Writes)
		require.Error(t, a.Delete(t.Context(), api.Servers[0].ID, execCtx))
		require.Zero(t, api.Deletes)
	}
	a, api, execCtx := authServerExecutorFixture()
	api.Servers[0].Labels[labels.ProtectedKey] = "true"
	execCtx.PlannedChange.Protection = planner.ProtectionChange{Old: true, New: false}
	var request kkComps.UpdateAuthServer
	require.NoError(t, a.MapUpdateFields(t.Context(), execCtx, execCtx.PlannedChange.Fields, &request, nil))
	_, err := a.Update(t.Context(), api.Servers[0].ID, request, execCtx.Namespace, execCtx)
	require.ErrorContains(t, err, "unprotect")
	require.Empty(t, api.Writes)
}
