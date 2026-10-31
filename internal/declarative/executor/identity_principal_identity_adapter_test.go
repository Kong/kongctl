package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/loader"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	logctx "github.com/kong/kongctl/internal/log"
	fixture "github.com/kong/kongctl/test/konnect"
	"github.com/stretchr/testify/require"
)

const identityLifecycleManifest = `_defaults:
  kongctl: {namespace: c-identity-team}
identity:
  directories:
    - ref: people
      name: people
      allow_all_control_planes: true
      kongctl: {namespace: a-directory-team}
      principals:
        - ref: person
          type: user
          kongctl: {namespace: b-principal-team}
          identities:
            - ref: employee
              type: custom
              key: employee
              value: '123'
              labels: {team: identity}
            - ref: external-login
              type: oidc
              issuer: https://issuer.example.test
              claim: {name: sub, value: employee-123}
  identities:
    - ref: oauth-worker
      principal: person
      type: auth_server_client
      auth_server_id: 731559d3-39bd-48c7-b33c-eb192847eb24
      client_id: opaque-external-client
    - ref: gateway-user
      principal: person
      type: control_plane_consumer
      control_plane_id: 731559d3-39bd-48c7-b33c-eb192847eb25
      consumer_id: 731559d3-39bd-48c7-b33c-eb192847eb26
`

func TestPrincipalIdentitySavedPlanLifecycleAcrossNamespaces(t *testing.T) {
	directories := &fixture.DirectoryAPI{
		Lifecycle: true,
		Current:   fixture.Directory("people", "a-directory-team", false),
	}
	principals := &fixture.PrincipalAPI{Directories: make(map[string][]kkComps.KongPrincipal)}
	identities := &fixture.PrincipalIdentitiesAPI{Parents: make(map[string][]kkComps.KongPrincipalIdentity)}
	client := state.NewClient(state.ClientConfig{
		IdentityDirectoriesAPI: directories, IdentityPrincipalsAPI: principals,
		IdentityPrincipalIdentitiesAPI: identities,
	})
	file := filepath.Join(t.TempDir(), "manifest.yaml")
	loadPlan := func(input string, mode planner.PlanMode) (*planner.Plan, error) {
		t.Helper()
		require.NoError(t, os.WriteFile(file, []byte(input), 0o600))
		rs, err := loader.New().LoadFile(file)
		require.NoError(t, err)
		plan, err := planner.NewPlanner(client, slog.Default()).
			GeneratePlan(t.Context(), rs, planner.Options{Mode: mode})
		if err != nil {
			return nil, err
		}
		data, err := json.Marshal(plan)
		require.NoError(t, err)
		var saved planner.Plan
		require.NoError(t, json.Unmarshal(data, &saved))
		return &saved, nil
	}
	execute := func(plan *planner.Plan) *ExecutionResult {
		ctx := context.WithValue(t.Context(), logctx.LoggerKey, slog.Default())
		return NewWithOptions(client, nil, false, Options{MaxConcurrency: 1}).Execute(ctx, plan)
	}
	plan, err := loadPlan(identityLifecycleManifest, planner.PlanModeApply)
	require.NoError(t, err)
	require.Len(t, plan.Changes, 6)
	result := execute(plan)
	require.Zero(t, result.FailureCount, "%+v", result.Errors)
	require.Len(t, directories.Directories, 1)
	directoryID := directories.Directories[0].ID
	require.Len(t, principals.Directories[directoryID], 1)
	principalID := principals.Directories[directoryID][0].ID
	require.Len(t, identities.Creates, 4)
	for _, created := range identities.Creates {
		require.Equal(t, directoryID, created.DirectoryID)
		require.Equal(t, principalID, created.PrincipalID)
		metadata, err := identityRequestMetadata(created.KongPrincipalIdentityBody)
		require.NoError(t, err)
		require.Equal(t, "c-identity-team", metadata[resources.ChildNamespaceKey])
		require.Equal(t, "kongctl", metadata[resources.ManagedByToolKey])
		require.Len(t, metadata, 5)
	}
	plan, err = loadPlan(identityLifecycleManifest, planner.PlanModeApply)
	require.NoError(t, err)
	require.Empty(t, plan.Changes)
	principalAlias := identityLifecycleManifest + fmt.Sprintf(`  principals:
    - ref: external-person
      directory: people
      _external: {id: %s}
`, principalID)
	_, err = loadPlan(principalAlias, planner.PlanModeApply)
	require.ErrorContains(t, err, "both managed and external")
	var employeeID string
	for _, value := range identities.Parents[directoryID+"/"+principalID] {
		if value.KongPrincipalIdentityCustomResponse != nil {
			employeeID = value.KongPrincipalIdentityCustomResponse.ID
		}
	}
	require.NotEmpty(t, employeeID)
	identityAlias := identityLifecycleManifest + fmt.Sprintf(`    - ref: external-employee
      principal: person
      _external: {id: %s}
`, employeeID)
	_, err = loadPlan(identityAlias, planner.PlanModeApply)
	require.ErrorContains(t, err, "both managed and external")
	updated := strings.ReplaceAll(identityLifecycleManifest, "value: '123'", "value: '456'")
	updated = strings.ReplaceAll(updated, "labels: {team: identity}", "labels: {}")
	plan, err = loadPlan(updated, planner.PlanModeApply)
	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)
	result = execute(plan)
	require.Zero(t, result.FailureCount, "%+v", result.Errors)
	require.Len(t, identities.Replaces, 1)
	require.Empty(t, identities.Replaces[0].KongPrincipalIdentityBody.KongPrincipalIdentityCustom.Labels)
	plan, err = loadPlan(updated, planner.PlanModeApply)
	require.NoError(t, err)
	require.Empty(t, plan.Changes)
	protected := strings.Replace(updated, "ref: employee", "ref: employee\n              kongctl: {protected: true}", 1)
	plan, err = loadPlan(protected, planner.PlanModeApply)
	require.NoError(t, err)
	result = execute(plan)
	require.Zero(t, result.FailureCount, "%+v", result.Errors)
	_, err = loadPlan(protected, planner.PlanModeDelete)
	require.ErrorContains(t, err, "protected")
	plan, err = loadPlan(updated, planner.PlanModeApply)
	require.NoError(t, err)
	result = execute(plan)
	require.Zero(t, result.FailureCount, "%+v", result.Errors)
	plan, err = loadPlan(updated, planner.PlanModeSync)
	require.NoError(t, err)
	require.Empty(t, plan.Changes)
	require.Equal(t, []string{"a-directory-team"}, plan.Metadata.SyncScope.IdentityDirectoryNamespaces)
	plan, err = loadPlan(updated, planner.PlanModeDelete)
	require.NoError(t, err)
	require.Len(t, plan.Changes, 6)
	result = execute(plan)
	require.Zero(t, result.FailureCount, "%+v", result.Errors)
	require.Len(t, identities.Deletes, 4)
	require.Equal(t, 1, principals.Deletes)
	require.Empty(t, directories.Directories)
}

func TestProtectedIdentityPayloadReferencesAreNoOpsBeforeNamespacePlanning(t *testing.T) {
	directory := fixture.Directory("people", "directory-team", false)
	const principalID = "731559d3-39bd-48c7-b33c-eb192847eb21"
	principal := kkComps.KongPrincipal{
		ID: principalID, Type: new(kkComps.KongPrincipalTypeUser),
		Labels: map[string]string{
			"directory": directory.ID,
		}, ManagedBy: resources.BuildPrincipalManagedBy("principal-team", "person", true, "old"),
	}
	identity := kkComps.CreateKongPrincipalIdentityCustom(kkComps.KongPrincipalIdentityCustomResponse{
		ID: "731559d3-39bd-48c7-b33c-eb192847eb22", Key: "employee", Value: "123",
		Labels: map[string]string{
			"principal": principalID,
		}, ManagedBy: resources.BuildPrincipalManagedBy("association-team", "employee", true, "old"),
	})
	client := state.NewClient(state.ClientConfig{
		IdentityDirectoriesAPI: &fixture.DirectoryAPI{Directories: []kkComps.KongDirectory{directory}},
		IdentityPrincipalsAPI: &fixture.PrincipalAPI{
			Directories: map[string][]kkComps.KongPrincipal{directory.ID: {principal}},
		},
		IdentityPrincipalIdentitiesAPI: &fixture.PrincipalIdentitiesAPI{
			Data: []kkComps.KongPrincipalIdentity{identity},
		},
	})
	input := `_defaults:
  kongctl: {namespace: association-team}
identity:
  directories:
    - ref: people
      name: people
      kongctl: {namespace: directory-team}
      principals:
        - ref: person
          type: user
          labels: {directory: !ref people#id}
          kongctl: {namespace: principal-team, protected: true}
          identities:
            - ref: employee
              type: custom
              key: employee
              value: '123'
              labels: {principal: !ref person#id}
              kongctl: {protected: true}
`
	file := filepath.Join(t.TempDir(), "protected.yaml")
	require.NoError(t, os.WriteFile(file, []byte(input), 0o600))
	rs, err := loader.New().LoadFile(file)
	require.NoError(t, err)
	plan, err := planner.NewPlanner(client, slog.Default()).
		GeneratePlan(t.Context(), rs, planner.Options{Mode: planner.PlanModeApply})
	require.NoError(t, err)
	require.Empty(t, plan.Changes)
}

func TestIdentitySavedMutationRejectsLateOwnershipProtectionAndDuplicates(t *testing.T) {
	for _, action := range []planner.ActionType{planner.ActionUpdate, planner.ActionDelete} {
		for _, mode := range []string{
			resources.ManagedByToolKey, resources.ChildNamespaceKey, resources.ManagedByRefKey,
			resources.ChildProtectedKey, "duplicate", "failed-observation",
		} {
			t.Run(string(action)+"/"+mode, func(t *testing.T) {
				const directory = "731559d3-39bd-48c7-b33c-eb192847eb20"
				const principal = "731559d3-39bd-48c7-b33c-eb192847eb21"
				const id = "731559d3-39bd-48c7-b33c-eb192847eb22"
				metadata := resources.BuildPrincipalManagedBy("identity-team", "employee", false, "old")
				current := kkComps.KongPrincipalIdentityCustomResponse{
					ID:        id,
					Key:       "employee",
					Value:     "123",
					ManagedBy: metadata,
				}
				api := &fixture.PrincipalIdentitiesAPI{Parents: map[string][]kkComps.KongPrincipalIdentity{
					directory + "/" + principal: {kkComps.CreateKongPrincipalIdentityCustom(current)},
				}}
				switch mode {
				case "duplicate":
					duplicate := current
					duplicate.ID = "731559d3-39bd-48c7-b33c-eb192847eb23"
					duplicate.Value = "different-external-id"
					api.Parents[directory+"/"+principal] = append(
						api.Parents[directory+"/"+principal],
						kkComps.CreateKongPrincipalIdentityCustom(duplicate),
					)
				case "failed-observation":
					api.Error = fmt.Errorf("incomplete observation")
				case resources.ChildProtectedKey:
					metadata[mode] = "true"
				default:
					metadata[mode] = "other"
				}
				client := state.NewClient(state.ClientConfig{
					IdentityPrincipalIdentitiesAPI: api,
					IdentityPrincipalsAPI: &fixture.PrincipalAPI{
						Directories: map[string][]kkComps.KongPrincipal{directory: {{ID: principal}}},
					},
				})
				adapter := &IdentityPrincipalIdentityAdapter{client: client}
				change := planner.PlannedChange{
					ResourceType: planner.ResourceTypeIdentityPrincipalIdentity, ResourceRef: "employee",
					ResourceID: id, Namespace: "identity-team", Protection: false, Action: action,
					Parent:     &planner.ParentInfo{ID: principal},
					References: map[string]planner.ReferenceInfo{planner.FieldDirectoryID: {ID: directory}},
					Fields: map[string]any{
						planner.FieldType: "custom", planner.FieldKey: "employee", planner.FieldValue: "456",
						planner.FieldManagedBy: resources.BuildPrincipalManagedBy(
							"identity-team",
							"employee",
							false,
							"",
						),
					},
				}
				data, err := json.Marshal(change)
				require.NoError(t, err)
				var saved planner.PlannedChange
				require.NoError(t, json.Unmarshal(data, &saved))
				execCtx := NewExecutionContext(&saved)
				if action == planner.ActionDelete {
					err = adapter.Delete(t.Context(), id, execCtx)
				} else {
					var body kkComps.KongPrincipalIdentityBody
					require.NoError(t, adapter.MapUpdateFields(t.Context(), execCtx, saved.Fields, &body, nil))
					_, err = adapter.Update(t.Context(), id, body, execCtx.Namespace, execCtx)
				}
				require.Error(t, err)
				require.Empty(t, api.Replaces)
				require.Empty(t, api.Deletes)
			})
		}
	}
}
