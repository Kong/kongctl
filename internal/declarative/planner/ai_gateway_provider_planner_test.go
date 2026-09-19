package planner

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/stretchr/testify/require"
)

func TestShouldUpdateAIGatewayProviderRejectsTypeChange(t *testing.T) {
	t.Parallel()

	needsUpdate, fields, _, err := shouldUpdateAIGatewayProvider(
		state.AIGatewayProvider{
			Type:        "openai",
			DisplayName: "OpenAI Provider",
			Config:      map[string]any{"auth": map[string]any{"type": "basic"}},
		},
		resources.AIGatewayProviderResource{
			Type:        "anthropic",
			DisplayName: "Anthropic Provider",
			Config:      map[string]any{"auth": map[string]any{"type": "basic"}},
		},
	)

	require.Error(t, err)
	require.False(t, needsUpdate)
	require.Nil(t, fields)
	require.Contains(t, err.Error(), "changing AI Gateway Model Provider type")
}

func TestAIGatewayProviderConfigChangedIgnoresWriteOnlyValues(t *testing.T) {
	t.Parallel()

	current := map[string]any{
		"auth": map[string]any{
			"type": "basic",
			"headers": []any{
				map[string]any{"name": "Authorization"},
			},
		},
	}
	desired := map[string]any{
		"auth": map[string]any{
			"type": "basic",
			"headers": []any{
				map[string]any{"name": "Authorization", "value": "Bearer token"},
			},
		},
	}

	require.False(t, aiGatewayProviderConfigChanged(current, desired))
}

func TestAIGatewayProviderConfigChangedComparesPublicVaultReferences(t *testing.T) {
	t.Parallel()

	config := func(value string) map[string]any {
		return map[string]any{
			"auth": map[string]any{
				"type": "basic",
				"headers": []any{
					map[string]any{"name": "Authorization", "value": value},
				},
			},
		}
	}

	require.False(t, aiGatewayProviderConfigChanged(
		config("{vault://support-secrets/openai-token}"),
		config("{vault://support-secrets/openai-token}"),
	))
	require.True(t, aiGatewayProviderConfigChanged(
		config("{vault://support-secrets/old-openai-token}"),
		config("{vault://support-secrets/new-openai-token}"),
	))
	// Plain secret material remains excluded from comparison.
	require.False(t, aiGatewayProviderConfigChanged(
		config("old-secret-material"),
		config("new-secret-material"),
	))
}

func TestAIGatewayProviderConfigChangedIgnoresVaultReferenceMissingFromResponse(t *testing.T) {
	t.Parallel()

	current := map[string]any{
		"auth": map[string]any{
			"type": "basic",
			"headers": []any{
				map[string]any{"name": "Authorization"},
			},
		},
	}
	desired := map[string]any{
		"auth": map[string]any{
			"type": "basic",
			"headers": []any{
				map[string]any{
					"name":  "Authorization",
					"value": "{vault://support-secrets/openai-token}",
				},
			},
		},
	}

	require.False(t, aiGatewayProviderConfigChanged(current, desired))
}

func TestShouldUpdateAIGatewayProviderIncludesChangedPublicVaultReference(t *testing.T) {
	t.Parallel()

	oldReference := "{vault://support-secrets/old-openai-token}"
	newReference := "{vault://support-secrets/new-openai-token}"
	config := func(value string) map[string]any {
		return map[string]any{
			"auth": map[string]any{
				"type": "basic",
				"headers": []any{
					map[string]any{"name": "Authorization", "value": value},
				},
			},
		}
	}

	needsUpdate, fields, changedFields, err := shouldUpdateAIGatewayProvider(
		state.AIGatewayProvider{
			Type:        "openai",
			DisplayName: "OpenAI Provider",
			Config:      config(oldReference),
		},
		resources.AIGatewayProviderResource{
			Type:        "openai",
			DisplayName: "OpenAI Provider",
			Config:      config(newReference),
		},
	)

	require.NoError(t, err)
	require.True(t, needsUpdate)
	updateConfig := fields[FieldConfig].(map[string]any)
	updateHeader := updateConfig["auth"].(map[string]any)["headers"].([]any)[0].(map[string]any)
	require.Equal(t, newReference, updateHeader[FieldValue])
	diffConfig := changedFields[FieldConfig].New.(map[string]any)
	diffHeader := diffConfig["auth"].(map[string]any)["headers"].([]any)[0].(map[string]any)
	require.Equal(t, newReference, diffHeader[FieldValue])
}

func TestShouldUpdateAIGatewayProviderDisplaysUnpairedVaultReferenceOnObservableConfigChange(t *testing.T) {
	t.Parallel()

	const reference = "{vault://support-secrets/oauth-client-secret}"
	needsUpdate, fields, changedFields, err := shouldUpdateAIGatewayProvider(
		state.AIGatewayProvider{
			Type:        "openai",
			DisplayName: "OpenAI Provider",
			Config:      map[string]any{"auth": map[string]any{"type": "basic"}},
		},
		resources.AIGatewayProviderResource{
			Type:        "openai",
			DisplayName: "OpenAI Provider",
			Config: map[string]any{"auth": map[string]any{
				"type": "oauth2", FieldClientSecret: reference,
			}},
		},
	)

	require.NoError(t, err)
	require.True(t, needsUpdate)
	require.Equal(t, reference, fields[FieldConfig].(map[string]any)["auth"].(map[string]any)[FieldClientSecret])
	require.Equal(
		t,
		reference,
		changedFields[FieldConfig].New.(map[string]any)["auth"].(map[string]any)[FieldClientSecret],
	)
}

func TestAIGatewayProviderCreatePlanKeepsPublicVaultReferenceOutOfSecretWrites(t *testing.T) {
	t.Parallel()

	const reference = "{vault://support-secrets/openai-token}"
	plan := NewPlan("1.0", "test", PlanModeApply)
	(&Planner{}).planAIGatewayProviderCreate(
		DefaultNamespace,
		"gateway",
		"Gateway",
		"gateway-id",
		resources.AIGatewayProviderResource{
			BaseResource: resources.BaseResource{Ref: "provider"},
			Name:         "provider",
			Type:         "openai",
			DisplayName:  "Provider",
			Config: map[string]any{
				"auth": map[string]any{
					"type": "basic",
					"headers": []any{
						map[string]any{"name": "Authorization", "value": reference},
					},
				},
			},
		},
		nil,
		plan,
	)

	require.Len(t, plan.Changes, 1)
	require.Empty(t, plan.Changes[0].SecretWrites)
	data, err := json.Marshal(plan)
	require.NoError(t, err)
	require.Contains(t, string(data), reference)
}

func TestAIGatewayProviderConfigChangedIgnoresAzureFoundryDomainDefault(t *testing.T) {
	t.Parallel()

	current := map[string]any{
		FieldService: "azure-foundry",
		FieldFoundry: map[string]any{
			FieldResource: "support-foundry",
			FieldDomain:   "services.ai.azure.com",
		},
	}
	desired := map[string]any{
		FieldService: "azure-foundry",
		FieldFoundry: map[string]any{
			FieldResource: "support-foundry",
		},
	}

	require.False(t, aiGatewayProviderConfigChanged(current, desired))
}

func TestAIGatewayProviderChangedFieldsScrubWriteOnlyValues(t *testing.T) {
	t.Parallel()

	needsUpdate, fields, changedFields, err := shouldUpdateAIGatewayProvider(
		state.AIGatewayProvider{
			Type:        "openai",
			DisplayName: "OpenAI Provider",
			Config: map[string]any{
				"auth": map[string]any{
					"type": "basic",
					"headers": []any{
						map[string]any{"name": "Authorization"},
					},
				},
			},
		},
		resources.AIGatewayProviderResource{
			Type:        "openai",
			DisplayName: "OpenAI Provider",
			Config: map[string]any{
				"auth": map[string]any{
					"type": "basic",
					"headers": []any{
						map[string]any{"name": "Authorization", "value": "Bearer token"},
					},
				},
				"endpoint": "https://api.openai.test",
			},
		},
	)

	require.NoError(t, err)
	require.True(t, needsUpdate)
	updateConfig := fields[FieldConfig].(map[string]any)
	updateHeader := updateConfig["auth"].(map[string]any)["headers"].([]any)[0].(map[string]any)
	require.Equal(t, "Bearer token", updateHeader["value"])

	configChange := changedFields[FieldConfig]
	diffConfig := configChange.New.(map[string]any)
	diffHeader := diffConfig["auth"].(map[string]any)["headers"].([]any)[0].(map[string]any)
	require.NotContains(t, diffHeader, "value")
}

func TestAIGatewayProviderPlannerNameIdentity(t *testing.T) {
	t.Parallel()
	const (
		cachedID  = "11111111-1111-4111-8111-111111111111"
		refID     = "22222222-2222-4222-8222-222222222222"
		nameID    = "33333333-3333-4333-8333-333333333333"
		missingID = "44444444-4444-4444-8444-444444444444"
	)
	for _, tt := range []struct {
		name, ref, cachedID, desiredName                      string
		action                                                ActionType
		apply, missingDetail, readError, protected, unchanged bool
	}{
		{name: "name wins over UUID ref", ref: refID, desiredName: "desired-name", action: ActionUpdate},
		{
			name: "name wins over cached ID", ref: "local-ref", cachedID: cachedID,
			desiredName: "desired-name", action: ActionUpdate,
		},
		{
			name: "missing IDs do not hide name", ref: missingID, cachedID: missingID,
			desiredName: "desired-name", action: ActionUpdate,
		},
		{name: "UUID ref does not rename or retain", ref: refID, desiredName: "new-name", action: ActionCreate},
		{
			name: "cached ID does not rename or retain", ref: "local-ref", cachedID: cachedID,
			desiredName: "new-name", action: ActionCreate,
		},
		{
			name: "absent detail recreates and retains declared name", ref: refID,
			desiredName: "desired-name", action: ActionCreate, missingDetail: true,
		},
		{name: "detail error stops before sync", ref: refID, desiredName: "desired-name", readError: true},
		{
			name: "protection stops sync at first deletion", ref: refID,
			desiredName: "desired-name", action: ActionUpdate, protected: true,
		},
		{name: "apply does not prune", ref: refID, desiredName: "new-name", action: ActionCreate, apply: true},
		{name: "unchanged detail retains name", ref: refID, desiredName: "desired-name", unchanged: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			api := &nameIdentityProviderAPI{missingDetail: tt.missingDetail, readError: tt.readError}
			for _, identity := range []struct{ id, name string }{
				{cachedID, "cached-name"}, {refID, "ref-name"}, {nameID, "desired-name"},
			} {
				raw := map[string]any{
					"id": identity.id, "name": identity.name, "type": "openai",
					"display_name": "Original child",
					"config":       map[string]any{"auth": map[string]any{"type": "basic"}},
					"created_at":   "2026-01-01T00:00:00Z", "updated_at": "2026-01-01T00:00:00Z",
				}
				if tt.protected && identity.id == cachedID {
					raw["labels"] = map[string]string{labels.ProtectedKey: labels.TrueValue}
				}
				data, err := json.Marshal(raw)
				require.NoError(t, err)
				var child kkComps.AIGatewayModelProvider
				require.NoError(t, json.Unmarshal(data, &child))
				api.children = append(api.children, child)
			}
			desired := resources.AIGatewayProviderResource{
				BaseResource: resources.BaseResource{Ref: tt.ref},
				Name:         tt.desiredName, Type: "openai", DisplayName: "Updated child",
				Config: map[string]any{"auth": map[string]any{"type": "basic"}},
			}
			if tt.unchanged {
				desired.DisplayName = "Original child"
			}
			desired.SetKonnectID(tt.cachedID)
			p := NewPlanner(state.NewClient(state.ClientConfig{AIGatewayProvidersAPI: api}), slog.Default())
			mode := PlanModeSync
			if tt.apply {
				mode = PlanModeApply
			}
			plan := NewPlan(CurrentPlanVersion, "test", mode)
			err := p.planAIGatewayProviderChanges(t.Context(), nil,
				"test-namespace", "Gateway", "gateway-id", "gateway-ref", "",
				[]resources.AIGatewayProviderResource{desired}, plan)
			require.Equal(t, []string{"gateway-id"}, api.lists)
			if tt.desiredName == "desired-name" {
				require.Equal(t, [][2]string{{"gateway-id", nameID}}, api.reads)
			} else {
				require.Empty(t, api.reads)
			}
			if tt.readError {
				require.ErrorContains(t, err, "failed to get AI Gateway Model Provider "+nameID)
				require.ErrorContains(t, err, "detail unavailable")
				require.Empty(t, plan.Changes)
				return
			}
			if tt.protected {
				require.ErrorContains(t, err, "protected")
			} else {
				require.NoError(t, err)
			}
			wantDeleteIDs := []string{cachedID, refID}
			if tt.desiredName == "new-name" {
				wantDeleteIDs = append(wantDeleteIDs, nameID)
			}
			if tt.apply || tt.protected {
				wantDeleteIDs = nil
			}
			offset := 0
			if !tt.unchanged {
				offset = 1
			}
			require.Len(t, plan.Changes, offset+len(wantDeleteIDs))
			if !tt.unchanged {
				change := plan.Changes[0]
				require.Equal(t, tt.action, change.Action)
				wantID := ""
				if tt.action == ActionUpdate {
					wantID = nameID
					require.Equal(t, FieldChange{Old: "Original child", New: "Updated child"},
						change.ChangedFields[FieldDisplayName])
				}
				require.Equal(t, wantID, change.ResourceID)
				require.Equal(t, tt.ref, change.ResourceRef)
				require.Equal(t, tt.desiredName, change.Fields[FieldName])
				require.Equal(t, "Updated child", change.Fields[FieldDisplayName])
			}
			for i, id := range wantDeleteIDs {
				require.Equal(t, ActionDelete, plan.Changes[offset+i].Action)
				require.Equal(t, id, plan.Changes[offset+i].ResourceID)
			}
			for _, change := range plan.Changes {
				require.Equal(t, ResourceTypeAIGatewayProvider, change.ResourceType)
				require.Equal(t, "test-namespace", change.Namespace)
				require.Equal(t, &ParentInfo{Ref: "gateway-ref", ID: "gateway-id"}, change.Parent)
			}
		})
	}
}

type nameIdentityProviderAPI struct {
	helpers.AIGatewayProvidersAPI
	children                 []kkComps.AIGatewayModelProvider
	lists                    []string
	reads                    [][2]string
	missingDetail, readError bool
}

func (a *nameIdentityProviderAPI) ListAiGatewayProviders(
	_ context.Context, req kkOps.ListAiGatewayModelProvidersRequest, _ ...kkOps.Option,
) (*kkOps.ListAiGatewayModelProvidersResponse, error) {
	a.lists = append(a.lists, req.GatewayID)
	return &kkOps.ListAiGatewayModelProvidersResponse{
		ListAIGatewayModelProvidersResponse: &kkComps.ListAIGatewayModelProvidersResponse{Data: a.children},
	}, nil
}

func (a *nameIdentityProviderAPI) GetAiGatewayProvider(
	_ context.Context, gatewayID, childID string, _ ...kkOps.Option,
) (*kkOps.GetAiGatewayModelProviderResponse, error) {
	a.reads = append(a.reads, [2]string{gatewayID, childID})
	if a.readError {
		return nil, errors.New("detail unavailable")
	}
	if a.missingDetail {
		return &kkOps.GetAiGatewayModelProviderResponse{}, nil
	}
	for _, child := range a.children {
		current, err := state.NormalizeAIGatewayProvider(child)
		if err != nil {
			return nil, err
		}
		if current.ID == childID {
			return &kkOps.GetAiGatewayModelProviderResponse{AIGatewayModelProvider: &child}, nil
		}
	}
	return &kkOps.GetAiGatewayModelProviderResponse{}, nil
}
