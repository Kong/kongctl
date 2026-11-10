package planner

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"testing"
	"time"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/konnect/helpers"
	fixture "github.com/kong/kongctl/test/konnect"
	"github.com/stretchr/testify/require"
)

func apiKeyPlanFixture() (*Planner, *resources.ResourceSet, *fixture.GroupAPI, *fixture.APIKeyHTTP) {
	_, rs, groups := membershipPlanFixture()
	directory := rs.Identity.Directories[0].External.ID
	keys := &fixture.APIKeyHTTP{PageSize: 1}
	sdk := keys.SDK()
	client := state.NewClient(state.ClientConfig{
		IdentityDirectoriesAPI: &fixture.DirectoryAPI{
			Directories: []kkComps.KongDirectory{{ID: directory, Name: "people"}},
		},
		IdentityPrincipalsAPI: &fixture.PrincipalAPI{Directories: map[string][]kkComps.KongPrincipal{directory: {{
			ID:        "5078f559-063b-4f10-9098-9e6553b21d9b",
			ManagedBy: resources.BuildPrincipalManagedBy("principal-team", "alice", false, "old"),
		}}}},
		IdentityGroupsAPI: groups, IdentityPrincipalIdentitiesAPI: &fixture.PrincipalIdentitiesAPI{},
		IdentityBasicAuthAPI: &fixture.BasicAuthAPI{}, IdentityPrincipalKeysReader: sdk.GetIdentityPrincipalKeysAPI(),
		IdentityPrincipalKeysAPI: sdk.GetIdentityPrincipalKeysAPI(), IdentityGroupKeysAPI: sdk.GetIdentityGroupKeysAPI(),
	})
	rs.Identity.Memberships = nil
	rs.Identity.PrincipalAPIKeys = []resources.IdentityPrincipalAPIKeyResource{{
		IdentityAPIKeyFields: resources.IdentityAPIKeyFields{
			Ref:  "alice-key",
			Type: "v1",
			Name: new("Alice key"),
		}, Principal: "alice",
	}}
	rs.Identity.GroupAPIKeys = []resources.IdentityGroupAPIKeyResource{{
		IdentityAPIKeyFields: resources.IdentityAPIKeyFields{
			Ref:  "operators-key",
			Type: "v1",
			Name: new("Operators key"),
		}, Group: "operators",
	}}
	return NewPlanner(client, slog.Default()), rs, groups, keys
}

func seedAPIKeyPlan(t *testing.T, p *Planner, rs *resources.ResourceSet, api *fixture.APIKeyHTTP) *Plan {
	t.Helper()
	plan, err := p.GeneratePlan(t.Context(), rs, Options{Mode: PlanModeApply})
	require.NoError(t, err)
	require.Len(t, plan.Changes, 2)
	api.Keys = make(map[string][]helpers.IdentityAPIKey)
	for i, change := range plan.Changes {
		var key helpers.IdentityAPIKey
		data, err := json.Marshal(change.Fields)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(data, &key))
		key.ID = fmt.Sprintf("6078f559-063b-4f10-9098-9e6553b21d%02d", i)
		key.CreatedAt, key.UpdatedAt = time.Now().UTC(), time.Now().UTC()
		parent := "principals"
		if change.ResourceType == ResourceTypeIdentityGroupAPIKey {
			parent = "groups"
		}
		path := "/v2/directories/" + change.References[FieldDirectoryID].ID + "/" +
			parent + "/" + change.Parent.ID + "/api-keys"
		api.Keys[path] = []helpers.IdentityAPIKey{key}
	}
	return plan
}

func TestGeneratedAPIKeyPlanningUsesCurrentParentAuthorityAndStableRefs(t *testing.T) {
	p, rs, groups, api := apiKeyPlanFixture()
	initial := seedAPIKeyPlan(t, p, rs, api)
	for _, change := range initial.Changes {
		metadata := change.Fields[FieldManagedBy].(map[string]string)
		require.Len(t, metadata, 4)
		require.Equal(t, "v1", metadata[resources.IdentityAPIKeyCreateTypeKey])
		if change.ResourceType == ResourceTypeIdentityPrincipalAPIKey {
			require.Equal(t, "principal-team", change.Namespace)
		} else {
			require.Equal(t, "group-team", change.Namespace)
		}
	}
	for path, keys := range api.Keys {
		keys[0].ManagedBy[resources.ManagedByVersionKey] = "different-creator"
		api.Keys[path] = keys
	}
	plan, err := p.GeneratePlan(t.Context(), rs, Options{Mode: PlanModeApply})
	require.NoError(t, err)
	require.Empty(t, plan.Changes, "immutable creator version must not produce reconciliation")
	rs.Identity.PrincipalAPIKeys[0].Name = new("Renamed")
	plan, err = p.GeneratePlan(t.Context(), rs, Options{Mode: PlanModeApply})
	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)
	require.Equal(t, "Renamed", plan.Changes[0].Fields[FieldName])
	require.NotContains(t, plan.Changes[0].Fields, FieldManagedBy)
	rs.Identity.PrincipalAPIKeys[0].Name = new("Alice key")
	rs.Identity.GroupAPIKeys[0].Name = new("Renamed group key")
	plan, err = p.GeneratePlan(t.Context(), rs, Options{Mode: PlanModeApply})
	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)
	require.Equal(t, "Renamed group key", plan.Changes[0].Fields[FieldName])
	rs.Identity.GroupAPIKeys[0].Name = new("Operators key")
	directory := rs.Identity.Directories[0].External.ID
	groups.Directories[directory][0].ManagedBy[resources.ChildProtectedKey] = "true"
	rs.Identity.GroupAPIKeys[0].DeclareNoExpiration()
	for path, keys := range api.Keys {
		if keys[0].ManagedBy[resources.ManagedByRefKey] == "operators-key" {
			keys[0].ExpiresAt = new(time.Now().UTC())
			api.Keys[path] = keys
		}
	}
	_, err = p.GeneratePlan(t.Context(), rs, Options{Mode: PlanModeApply})
	require.ErrorContains(t, err, "protected")
}

func TestGeneratedAPIKeySyncPreservesForeignAndOmittedCollections(t *testing.T) {
	p, rs, _, api := apiKeyPlanFixture()
	seedAPIKeyPlan(t, p, rs, api)
	for path, keys := range api.Keys {
		foreign := keys[0]
		foreign.ID = "7078f559-063b-4f10-9098-9e6553b21d9b"
		foreign.ManagedBy = map[string]string{"tool": "another"}
		legacy := keys[0]
		legacy.ID = "8078f559-063b-4f10-9098-9e6553b21d9b"
		legacy.ManagedBy = map[string]string{"tool": "kongctl", resources.ManagedByRefKey: "legacy"}
		api.Keys[path] = append(keys, foreign, legacy)
	}
	rs.Identity.PrincipalAPIKeys, rs.Identity.GroupAPIKeys = nil, nil
	plan, err := p.GeneratePlan(t.Context(), rs, Options{Mode: PlanModeSync})
	require.NoError(t, err)
	require.Empty(t, plan.Changes)
	rs.SyncScope.AddChild(
		resources.ResourceTypeIdentityPrincipal,
		"alice",
		resources.ResourceTypeIdentityPrincipalAPIKey,
	)
	rs.SyncScope.AddChild(resources.ResourceTypeIdentityGroup, "operators", resources.ResourceTypeIdentityGroupAPIKey)
	plan, err = p.GeneratePlan(t.Context(), rs, Options{Mode: PlanModeSync})
	require.NoError(t, err)
	require.Len(t, plan.Changes, 2)
	for _, change := range plan.Changes {
		require.Equal(t, ActionDelete, change.Action)
		require.Contains(t, []string{"alice-key", "operators-key"}, change.ResourceRef)
	}
}

func TestGeneratedAPIKeyReplacementAndParentRetirementDependencies(t *testing.T) {
	p, rs, _, api := apiKeyPlanFixture()
	seedAPIKeyPlan(t, p, rs, api)
	rs.SyncScope.AddChild(
		resources.ResourceTypeIdentityPrincipal,
		"alice",
		resources.ResourceTypeIdentityPrincipalAPIKey,
	)
	rs.SyncScope.AddChild(resources.ResourceTypeIdentityGroup, "operators", resources.ResourceTypeIdentityGroupAPIKey)
	rs.Identity.PrincipalAPIKeys[0].Ref, rs.Identity.GroupAPIKeys[0].Ref = "alice-next", "operators-next"
	plan, err := p.GeneratePlan(t.Context(), rs, Options{Mode: PlanModeSync})
	require.NoError(t, err)
	require.Len(t, plan.Changes, 4)
	for _, deletion := range plan.Changes {
		if deletion.Action != ActionDelete {
			continue
		}
		for _, creation := range plan.Changes {
			if creation.Action == ActionCreate && creation.ResourceType == deletion.ResourceType {
				require.Contains(t, deletion.DependsOn, creation.ID)
			}
		}
	}
	rs.Identity.PrincipalAPIKeys[0].Ref, rs.Identity.GroupAPIKeys[0].Ref = "alice-key", "operators-key"
	plan, err = p.GeneratePlan(t.Context(), rs, Options{Mode: PlanModeDelete})
	require.NoError(t, err)
	require.Len(t, plan.Changes, 4)
	for _, parent := range plan.Changes {
		if parent.ResourceType != ResourceTypeIdentityPrincipal && parent.ResourceType != ResourceTypeIdentityGroup {
			continue
		}
		for _, child := range plan.Changes {
			if child.Parent != nil && child.Parent.ID == parent.ResourceID {
				require.Contains(t, parent.DependsOn, child.ID)
			}
		}
	}
	for path, keys := range api.Keys {
		foreign := keys[0]
		foreign.ID, foreign.ManagedBy = "7078f559-063b-4f10-9098-9e6553b21d9b", map[string]string{}
		api.Keys[path] = append(keys, foreign)
		break
	}
	_, err = p.GeneratePlan(t.Context(), rs, Options{Mode: PlanModeDelete})
	require.ErrorContains(t, err, "authorize each key retirement")
}
